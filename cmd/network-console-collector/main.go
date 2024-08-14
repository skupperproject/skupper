package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"time"

	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/sync/errgroup"

	"github.com/skupperproject/skupper/cmd/network-console-collector/internal/api"
	"github.com/skupperproject/skupper/pkg/version"
)

func run(cfg Config) error {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	logger := slog.New(slog.Default().Handler())

	// Startup message
	logger.Info("Network Console Collector starting", slog.String("skupper_version", version.Version))

	reg := prometheus.NewRegistry()
	promAPI, err := parsePrometheusAPI(cfg.PrometheusAPI)
	if err != nil {
		return fmt.Errorf("error parsing prometheus-api as URL: %s", err)
	}

	specFS, err := getSpecFS()
	if err != nil {
		return fmt.Errorf("could not load spec filesystem: %s", err)
	}

	collectorAPI := &struct {
		api.ServerInterface
	}{}

	var apiMiddlewares []api.MiddlewareFunc
	if cfg.CORSAllowAll {
		apiMiddlewares = append(apiMiddlewares, handlers.CORS())
	}

	var mux = mux.NewRouter().StrictSlash(true)
	mux.Handle("/metrics", handleMetrics(reg))
	mux.PathPrefix("/swagger").Handler(handleSwagger("/swagger", specFS))
	api.HandlerWithOptions(collectorAPI, api.GorillaServerOptions{
		BaseRouter:  mux,
		Middlewares: apiMiddlewares,
	})

	if cfg.EnableConsole {
		// add unspec'd api routes
		mux.Path("/api/v1alpha1/user").Handler(handleNoContent(apiMiddlewares))
		mux.Path("/api/v1alpha1/logout").Handler(handleNoContent(apiMiddlewares))
		mux.PathPrefix("/api/v1alpha1/internal/prom").Handler(handleProxyPrometheusAPI("/api/v1alpha1/internal/prom", promAPI))

		mux.PathPrefix("/").Handler(handleConsoleAssets(cfg.ConsoleLocation))
	}

	if !cfg.APIDisableAccessLogs {
		mux.Use(func(next http.Handler) http.Handler {
			return handlers.LoggingHandler(os.Stdout, next)
		})
	}

	s := &http.Server{
		Addr:         cfg.APIListenAddress,
		Handler:      handlers.CompressHandler(mux),
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
	}
	tlsEnabled := cfg.APITLS.hasCert()
	if tlsEnabled {
		s.TLSConfig, err = cfg.APITLS.config()
		if err != nil {
			return fmt.Errorf("could not set up certs for api server: %s", err)
		}
	}

	g, runCtx := errgroup.WithContext(ctx)
	g.Go(func() error {
		logger.Info("Starting Network Console API Server",
			slog.String("address", cfg.APIListenAddress),
			slog.Bool("tls", tlsEnabled),
			slog.Bool("console", cfg.EnableConsole))
		var err error
		if tlsEnabled {
			err = s.ListenAndServeTLS("", "")
		} else {
			err = s.ListenAndServe()
		}
		if err != nil {
			return fmt.Errorf("server error running api server: %s", err)
		}
		return nil
	})
	g.Go(func() error {
		<-runCtx.Done()
		logger.Debug("Shutting down Network Console API Server")
		shutdownCtx, sCancel := context.WithTimeout(context.Background(), time.Second)
		defer sCancel()
		if err := s.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("api server shutdown did not complete gracefully: %s", err)
		}
		logger.Debug("Network Console API Server shutdown clean")
		return nil
	})

	if cfg.EnableProfile {
		// serve only over localhost loopback
		const pprofAddr = "localhost:9970"
		pprofSrv := &http.Server{
			Addr: pprofAddr,
		}
		g.Go(func() error {
			logger.Info("Starting Network Console Profiling Server",
				slog.String("address", pprofAddr))

			err := pprofSrv.ListenAndServe()
			if err != nil && !errors.Is(err, http.ErrServerClosed) {
				return fmt.Errorf("server error running profiler server: %s", err)
			}
			return nil
		})
		g.Go(func() error {
			<-runCtx.Done()
			logger.Debug("Shutting down Network Console Profiling Server")
			shutdownCtx, sCancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer sCancel()
			if err := pprofSrv.Shutdown(shutdownCtx); err != nil {
				return fmt.Errorf("pprof server shutdown did not complete gracefully: %s", err)
			}
			logger.Debug("Network Console Profiling Server shutdown clean")
			return nil
		})
	}

	if err := g.Wait(); err != nil && !errors.Is(err, ctx.Err()) {
		return err
	}
	return nil
}

func main() {
	var cfg Config
	flags := flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	// if -version used, report and exit
	isVersion := flags.Bool("version", false, "Report the version of skupper the Network Console Collector was built against")

	flags.StringVar(&cfg.RouterURL, "router-endpoint", "amqps://skupper-router-local", "URL to the skupper router amqp(s) endpoint")
	flags.StringVar(&cfg.RouterTLS.Cert, "router-tls-cert", "", "Path to the client certificate for the router endpoint")
	flags.StringVar(&cfg.RouterTLS.Key, "router-tls-key", "", "Path to the client key for the router endpoint")
	flags.StringVar(&cfg.RouterTLS.CA, "router-tls-ca", "", "Path to the CA certificate file for the router endpoint")
	flags.BoolVar(&cfg.RouterTLS.SkipVerify, "router-tls-insecure", false, "Set to skip verification of the router certificate and host name")

	flags.StringVar(&cfg.APIListenAddress, "listen", ":8080", "The address that the API Server will listen on")
	flags.BoolVar(&cfg.APIDisableAccessLogs, "disable-access-logs", false, "Disables access logging for the API Server")
	flags.StringVar(&cfg.APITLS.Cert, "tls-cert", "", "Path to the API Server certificate file")
	flags.StringVar(&cfg.APITLS.Key, "tls-key", "", "Path to the API Server certificate key file matching tls-cert")

	flags.BoolVar(&cfg.EnableConsole, "enable-console", true, "Enables the web console")
	flags.StringVar(&cfg.ConsoleLocation, "console-location", "/app/console", "Location where the console assets are installed")
	flags.StringVar(&cfg.PrometheusAPI, "prometheus-api", "http://network-console-prometheus:9090", "Prometheus API HTTP endpoint for console")

	flags.DurationVar(&cfg.FlowRecordTTL, "flow-record-ttl", 15*time.Minute, "How long to retain flow records in memory")
	flags.BoolVar(&cfg.CORSAllowAll, "cors-allow-all", false, "Development option to allow all origins")
	flags.BoolVar(&cfg.EnableProfile, "profile", false, "Exposes the runtime profiling facilities from net/http/pprof on http://localhost:9970")

	flags.Parse(os.Args[1:])
	if *isVersion {
		fmt.Println(version.Version)
		os.Exit(0)
	}

	if err := run(cfg); err != nil {
		slog.Error("network console collector run error", slog.Any("error", err))
		os.Exit(1)
	}
}

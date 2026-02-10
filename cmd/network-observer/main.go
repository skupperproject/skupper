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

	"github.com/skupperproject/skupper/cmd/network-observer/internal/api"
	"github.com/skupperproject/skupper/cmd/network-observer/internal/cmd"
	"github.com/skupperproject/skupper/cmd/network-observer/internal/collector"
	"github.com/skupperproject/skupper/cmd/network-observer/internal/flowlog"
	"github.com/skupperproject/skupper/cmd/network-observer/internal/server"
	"github.com/skupperproject/skupper/internal/version"
	"github.com/skupperproject/skupper/pkg/vanflow"
	"github.com/skupperproject/skupper/pkg/vanflow/session"
)

func run(cfg Config) error {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	logger := slog.New(slog.Default().Handler())

	// Startup message
	logger.Info("Network Observer starting", slog.String("skupper_version", version.Version))

	reg := prometheus.NewRegistry()

	specFS, err := getSpecFS()
	if err != nil {
		return fmt.Errorf("could not load spec filesystem: %s", err)
	}

	sessionConfig, err := configureSession(cfg.RouterTLS)
	if err != nil {
		return fmt.Errorf("failed to load router tls configuration: %s", err)
	}

	flowLogger := func(vanflow.RecordMessage) {}
	vanflowSLog := logger.With(slog.String("component", "vanflow"))
	switch cfg.VanflowLoggingProfile {
	case "silent":
	case "minimal":
		flowLogger = flowlog.New(ctx, vanflowSLog.Info, loggingProfileMinimal)
	case "moderate":
		flowLogger = flowlog.New(ctx, vanflowSLog.Info, loggingProfileModerate)
	case "all":
		flowLogger = flowlog.New(ctx, vanflowSLog.Info, loggingProfileAll)
	default:
		return fmt.Errorf("unknown logging profile: %s", cfg.VanflowLoggingProfile)
	}

	collector := collector.New(
		logger.With(slog.String("component", "collector")),
		session.NewContainerFactory(cfg.RouterURL, sessionConfig),
		reg,
		cfg.FlowRecordTTL,
		flowLogger,
	)

	collectorAPI := server.New(
		logger.With(slog.String("component", "api")),
		collector.Records,
		collector.GetGraph(),
	)

	var mux = mux.NewRouter().StrictSlash(true)
	promSubrouter := mux.PathPrefix("/api/v2alpha1/internal/prom")
	mux.Handle("/metrics", handleMetrics(reg))
	mux.PathPrefix("/swagger").Handler(handleSwagger("/swagger", specFS))
	apiMux := mux.PathPrefix("/").Subrouter()
	if cfg.CORSAllowAll {
		apiMux.Use(handlers.CORS())
	}
	api.HandlerWithOptions(collectorAPI, api.GorillaServerOptions{
		BaseRouter: apiMux,
	})

	if cfg.EnableConsole {
		promAPI, err := parsePrometheusAPI(cfg.PrometheusAPI)
		if err != nil {
			return fmt.Errorf("error parsing prometheus-api as URL: %s", err)
		}
		// add unspec'd api routes
		apiMux.Path("/api/v2alpha1/user").Handler(handleGetUser())
		apiMux.Path("/api/v2alpha1/logout").Handler(handleUserLogout())
		promSubrouter.Handler(handleProxyPrometheusAPI("/api/v2alpha1/internal/prom", promAPI))

		apiMux.PathPrefix("/").Handler(handleSecuredConsoleAssets(cfg.ConsoleLocation))
	}

	if cfg.APIEnableAccessLogs {
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
			logger.Info("Starting Network Observer Profiling Server",
				slog.String("address", pprofAddr))

			err := pprofSrv.ListenAndServe()
			if err != nil && !errors.Is(err, http.ErrServerClosed) {
				return fmt.Errorf("server error running profiler server: %s", err)
			}
			return nil
		})
		g.Go(func() error {
			<-runCtx.Done()
			logger.Debug("Shutting down Network Observer Profiling Server")
			shutdownCtx, sCancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer sCancel()
			if err := pprofSrv.Shutdown(shutdownCtx); err != nil {
				return fmt.Errorf("pprof server shutdown did not complete gracefully: %s", err)
			}
			logger.Debug("Network Observer Profiling Server shutdown clean")
			return nil
		})
	}

	// serve metrics on a separate server if listen-metrics is set
	if cfg.MetricsListenAddress != "" {
		metricsMux := http.NewServeMux()
		metricsMux.Handle("/metrics", handleMetrics(reg))
		metricsSrv := &http.Server{
			Addr:         cfg.MetricsListenAddress,
			Handler:      metricsMux,
			ReadTimeout:  5 * time.Second,
			WriteTimeout: 10 * time.Second,
		}
		g.Go(func() error {
			logger.Info("Starting Network Observer Metrics Server",
				slog.String("address", cfg.MetricsListenAddress))

			err := metricsSrv.ListenAndServe()
			if err != nil && !errors.Is(err, http.ErrServerClosed) {
				return fmt.Errorf("server error running metrics server: %s", err)
			}
			return nil
		})
		g.Go(func() error {
			<-runCtx.Done()
			logger.Debug("Shutting down Network Observer Metrics Server")
			shutdownCtx, sCancel := context.WithTimeout(context.Background(), time.Second)
			defer sCancel()
			if err := metricsSrv.Shutdown(shutdownCtx); err != nil {
				return fmt.Errorf("metrics server shutdown did not complete gracefully: %s", err)
			}
			logger.Debug("Network Observer Metrics Server shutdown clean")
			return nil
		})
	}

	g.Go(func() error {
		logger.Debug("Starting Network Observer Collector")
		if err := collector.Run(runCtx); err != nil {
			return fmt.Errorf("collector error: %w", err)
		}
		return nil
	})

	if err := g.Wait(); err != nil && !errors.Is(err, ctx.Err()) {
		return err
	}
	return nil
}

func main() {
	var cfg Config
	flags := flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	// if -version used, report and exit
	isVersion := flags.Bool("version", false, "Report the version of skupper the Network Observer was built against")

	flags.StringVar(&cfg.RouterURL, "router-endpoint", "amqps://skupper-router-local", "URL to the skupper router amqp(s) endpoint")
	flags.StringVar(&cfg.RouterTLS.Cert, "router-tls-cert", "", "Path to the client certificate for the router endpoint")
	flags.StringVar(&cfg.RouterTLS.Key, "router-tls-key", "", "Path to the client key for the router endpoint")
	flags.StringVar(&cfg.RouterTLS.CA, "router-tls-ca", "", "Path to the CA certificate file for the router endpoint")
	flags.BoolVar(&cfg.RouterTLS.SkipVerify, "router-tls-insecure", false, "Set to skip verification of the router certificate and host name")

	flags.StringVar(&cfg.APIListenAddress, "listen", ":8080", "The address that the API Server will listen on")
	flags.BoolVar(&cfg.APIEnableAccessLogs, "enable-access-logs", false, "Enable access logging for the API Server")
	flags.StringVar(&cfg.APITLS.Cert, "tls-cert", "", "Path to the API Server certificate file")
	flags.StringVar(&cfg.APITLS.Key, "tls-key", "", "Path to the API Server certificate key file matching tls-cert")

	flags.BoolVar(&cfg.EnableConsole, "enable-console", true, "Enables the web console")
	flags.StringVar(&cfg.ConsoleLocation, "console-location", "/app/console", "Location where the console assets are installed")
	flags.StringVar(&cfg.PrometheusAPI, "prometheus-api", "http://127.0.0.1:9090", "Prometheus API HTTP endpoint for console")

	flags.DurationVar(&cfg.FlowRecordTTL, "flow-record-ttl", 15*time.Minute, "How long to retain flow records in memory")
	flags.BoolVar(&cfg.CORSAllowAll, "cors-allow-all", false, "Development option to allow all origins")
	flags.BoolVar(&cfg.EnableProfile, "profile", false, "Exposes the runtime profiling facilities from net/http/pprof on http://localhost:9970")

	flags.StringVar(&cfg.VanflowLoggingProfile, "vanflow-logging-profile", "silent", "Controls low level vanflow record logging. Options are silent, minimal, moderate and all")

	flags.StringVar(&cfg.MetricsListenAddress, "listen-metrics", "", "The address that the Metrics Server will listen on.")

	flags.Parse(os.Args[1:])
	if *isVersion {
		fmt.Println(version.Version)
		os.Exit(0)
	}

	args := flags.Args()
	if len(args) > 0 {
		// Handle internal subcommands
		if args[0] == "help" {
			flags.Usage()
			os.Exit(0)
		}
		cmd.Run(args)
		return
	}

	if err := run(cfg); err != nil {
		slog.Error("network observer run error", slog.Any("error", err))
		os.Exit(1)
	}
}

func configureSession(tlsCfg TLSSpec) (ctrCfg session.ContainerConfig, err error) {
	ctrCfg.TLSConfig, err = tlsCfg.config()
	if err != nil {
		return
	}
	if tlsCfg.hasCert() {
		ctrCfg.SASLType = session.SASLTypeExternal
	}

	return ctrCfg, err
}

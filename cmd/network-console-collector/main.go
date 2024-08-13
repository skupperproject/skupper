package main

import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	_ "net/http/pprof"
	"net/url"
	"os"
	"os/signal"
	"time"

	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/skupperproject/skupper/pkg/qdr"

	"github.com/skupperproject/skupper/pkg/certs"
	"github.com/skupperproject/skupper/pkg/version"
)

func run(cfg Config) error {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	// Startup message
	slog.Info("Network Console Collector starting", slog.String("skupper_version", version.Version))

	var tlsConfig qdr.TlsConfigRetriever
	if cfg.RouterTLS.Key != "" || cfg.RouterTLS.CA != "" {
		tlsConfig = certs.GetTlsConfigRetriever(true, cfg.RouterTLS.Cert, cfg.RouterTLS.Key, cfg.RouterTLS.CA)
	}

	reg := prometheus.NewRegistry()
	c, err := NewController("", reg, cfg.RouterURL, tlsConfig, cfg.FlowRecordTTL)
	if err != nil {
		return fmt.Errorf("error initializing flow collector %s", err.Error())
	}

	promURL, err := url.JoinPath(cfg.PrometheusAPI, "/api/v1/")
	if err != nil {
		return fmt.Errorf("error parsing prometheus api endpoint: %s", err.Error())
	}
	c.FlowCollector.Collector.PrometheusUrl = promURL

	var mux = mux.NewRouter().StrictSlash(true)

	specFS, err := getSpecFS()
	if err != nil {
		return fmt.Errorf("could not load spec filesystem: %s", err)
	}
	mux.PathPrefix("/swagger").Handler(http.StripPrefix("/swagger", http.FileServer(http.FS(specFS))))

	var api = mux.PathPrefix("/api").Subrouter()
	api.NotFoundHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	if cfg.CORSAllowAll {
		api.Use(handlers.CORS())
	}

	var api1 = api.PathPrefix("/v1alpha1").Subrouter()
	api1.NotFoundHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	if !cfg.APIDisableAccessLogs {
		api1.Use(func(next http.Handler) http.Handler {
			return handlers.LoggingHandler(os.Stdout, next)
		})
	}

	var api1Internal = api1.PathPrefix("/internal").Subrouter()
	api1Internal.NotFoundHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	var promApi = api1Internal.PathPrefix("/prom").Subrouter()
	promApi.StrictSlash(true)
	promApi.NotFoundHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	var promqueryApi = promApi.PathPrefix("/query").Subrouter()
	promqueryApi.StrictSlash(true)
	promqueryApi.HandleFunc("/", http.HandlerFunc(c.promqueryHandler))
	promqueryApi.NotFoundHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	var promqueryrangeApi = promApi.PathPrefix("/rangequery").Subrouter()
	promqueryrangeApi.StrictSlash(true)
	promqueryrangeApi.HandleFunc("/", (http.HandlerFunc(c.promqueryrangeHandler)))
	promqueryrangeApi.NotFoundHandler = (http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))

	metricsHandler := promhttp.HandlerFor(reg, promhttp.HandlerOpts{Registry: reg})
	mux.Path("/metrics").Handler(metricsHandler)
	var metricsApi = api1.PathPrefix("/metrics").Subrouter()
	metricsApi.StrictSlash(true)
	metricsApi.Handle("/", metricsHandler)

	var eventsourceApi = api1.PathPrefix("/eventsources").Subrouter()
	eventsourceApi.StrictSlash(true)
	eventsourceApi.HandleFunc("/", http.HandlerFunc(c.eventsourceHandler)).Name("list")
	eventsourceApi.HandleFunc("/{id}", http.HandlerFunc(c.eventsourceHandler)).Name("item")
	eventsourceApi.NotFoundHandler = (http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))

	var userApi = api1.PathPrefix("/user").Subrouter()
	userApi.StrictSlash(true)
	userApi.HandleFunc("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	var userLogout = api1.PathPrefix("/logout").Subrouter()
	userLogout.StrictSlash(true)
	userLogout.HandleFunc("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	}))

	var siteApi = api1.PathPrefix("/sites").Subrouter()
	siteApi.StrictSlash(true)
	siteApi.HandleFunc("/", (http.HandlerFunc(c.siteHandler))).Name("list")
	siteApi.HandleFunc("/{id}", (http.HandlerFunc(c.siteHandler))).Name("item")
	siteApi.HandleFunc("/{id}/processes", (http.HandlerFunc(c.siteHandler))).Name("processes")
	siteApi.HandleFunc("/{id}/routers", (http.HandlerFunc(c.siteHandler))).Name("routers")
	siteApi.HandleFunc("/{id}/links", (http.HandlerFunc(c.siteHandler))).Name("links")
	siteApi.HandleFunc("/{id}/hosts", (http.HandlerFunc(c.siteHandler))).Name("hosts")
	siteApi.NotFoundHandler = (http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))

	var hostApi = api1.PathPrefix("/hosts").Subrouter()
	hostApi.StrictSlash(true)
	hostApi.HandleFunc("/", (http.HandlerFunc(c.hostHandler))).Name("list")
	hostApi.HandleFunc("/{id}", (http.HandlerFunc(c.hostHandler))).Name("item")
	hostApi.NotFoundHandler = (http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))

	var routerApi = api1.PathPrefix("/routers").Subrouter()
	routerApi.StrictSlash(true)
	routerApi.HandleFunc("/", (http.HandlerFunc(c.routerHandler))).Name("list")
	routerApi.HandleFunc("/{id}", (http.HandlerFunc(c.routerHandler))).Name("item")
	routerApi.HandleFunc("/{id}/flows", (http.HandlerFunc(c.routerHandler))).Name("flows")
	routerApi.HandleFunc("/{id}/links", (http.HandlerFunc(c.routerHandler))).Name("links")
	routerApi.HandleFunc("/{id}/listeners", (http.HandlerFunc(c.routerHandler))).Name("listeners")
	routerApi.HandleFunc("/{id}/connectors", (http.HandlerFunc(c.routerHandler))).Name("connectors")
	routerApi.NotFoundHandler = (http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))

	var linkApi = api1.PathPrefix("/links").Subrouter()
	linkApi.StrictSlash(true)
	linkApi.HandleFunc("/", (http.HandlerFunc(c.linkHandler))).Name("list")
	linkApi.HandleFunc("/{id}", (http.HandlerFunc(c.linkHandler))).Name("item")
	linkApi.NotFoundHandler = (http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))

	var listenerApi = api1.PathPrefix("/listeners").Subrouter()
	listenerApi.StrictSlash(true)
	listenerApi.HandleFunc("/", (http.HandlerFunc(c.listenerHandler))).Name("list")
	listenerApi.HandleFunc("/{id}", (http.HandlerFunc(c.listenerHandler))).Name("item")
	listenerApi.HandleFunc("/{id}/flows", (http.HandlerFunc(c.listenerHandler))).Name("flows")
	listenerApi.NotFoundHandler = (http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))

	var connectorApi = api1.PathPrefix("/connectors").Subrouter()
	connectorApi.StrictSlash(true)
	connectorApi.HandleFunc("/", (http.HandlerFunc(c.connectorHandler))).Name("list")
	connectorApi.HandleFunc("/{id}", (http.HandlerFunc(c.connectorHandler))).Name("item")
	connectorApi.HandleFunc("/{id}/flows", (http.HandlerFunc(c.connectorHandler))).Name("flows")
	connectorApi.HandleFunc("/{id}/process", (http.HandlerFunc(c.connectorHandler))).Name("process")
	connectorApi.NotFoundHandler = (http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))

	var addressApi = api1.PathPrefix("/addresses").Subrouter()
	addressApi.StrictSlash(true)
	addressApi.HandleFunc("/", (http.HandlerFunc(c.addressHandler))).Name("list")
	addressApi.HandleFunc("/{id}", (http.HandlerFunc(c.addressHandler))).Name("item")
	addressApi.HandleFunc("/{id}/processes", (http.HandlerFunc(c.addressHandler))).Name("processes")
	addressApi.HandleFunc("/{id}/processpairs", (http.HandlerFunc(c.addressHandler))).Name("processpairs")
	addressApi.HandleFunc("/{id}/flows", (http.HandlerFunc(c.addressHandler))).Name("flows")
	addressApi.HandleFunc("/{id}/flowpairs", (http.HandlerFunc(c.addressHandler))).Name("flowpairs")
	addressApi.HandleFunc("/{id}/listeners", (http.HandlerFunc(c.addressHandler))).Name("listeners")
	addressApi.HandleFunc("/{id}/connectors", (http.HandlerFunc(c.addressHandler))).Name("connectors")
	addressApi.NotFoundHandler = (http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))

	var processApi = api1.PathPrefix("/processes").Subrouter()
	processApi.StrictSlash(true)
	processApi.HandleFunc("/", (http.HandlerFunc(c.processHandler))).Name("list")
	processApi.HandleFunc("/{id}", (http.HandlerFunc(c.processHandler))).Name("item")
	processApi.HandleFunc("/{id}/flows", (http.HandlerFunc(c.processHandler))).Name("flows")
	processApi.HandleFunc("/{id}/addresses", (http.HandlerFunc(c.processHandler))).Name("addresses")
	processApi.HandleFunc("/{id}/connector", (http.HandlerFunc(c.processHandler))).Name("connector")
	processApi.NotFoundHandler = (http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))

	var processGroupApi = api1.PathPrefix("/processgroups").Subrouter()
	processGroupApi.StrictSlash(true)
	processGroupApi.HandleFunc("/", (http.HandlerFunc(c.processGroupHandler))).Name("list")
	processGroupApi.HandleFunc("/{id}", (http.HandlerFunc(c.processGroupHandler))).Name("item")
	processGroupApi.HandleFunc("/{id}/processes", (http.HandlerFunc(c.processGroupHandler))).Name("processes")
	processGroupApi.NotFoundHandler = (http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))

	var flowApi = api1.PathPrefix("/flows").Subrouter()
	flowApi.StrictSlash(true)
	flowApi.HandleFunc("/", (http.HandlerFunc(c.flowHandler))).Name("list")
	flowApi.HandleFunc("/{id}", (http.HandlerFunc(c.flowHandler))).Name("item")
	flowApi.HandleFunc("/{id}/process", (http.HandlerFunc(c.flowHandler))).Name("process")
	flowApi.NotFoundHandler = (http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))

	var flowpairApi = api1.PathPrefix("/flowpairs").Subrouter()
	flowpairApi.StrictSlash(true)
	flowpairApi.HandleFunc("/", (http.HandlerFunc(c.flowPairHandler))).Name("list")
	flowpairApi.HandleFunc("/{id}", (http.HandlerFunc(c.flowPairHandler))).Name("item")
	flowpairApi.NotFoundHandler = (http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))

	var sitepairApi = api1.PathPrefix("/sitepairs").Subrouter()
	sitepairApi.StrictSlash(true)
	sitepairApi.HandleFunc("/", (http.HandlerFunc(c.sitePairHandler))).Name("list")
	sitepairApi.HandleFunc("/{id}", (http.HandlerFunc(c.sitePairHandler))).Name("item")
	sitepairApi.NotFoundHandler = (http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))

	var processgrouppairApi = api1.PathPrefix("/processgrouppairs").Subrouter()
	processgrouppairApi.StrictSlash(true)
	processgrouppairApi.HandleFunc("/", (http.HandlerFunc(c.processGroupPairHandler))).Name("list")
	processgrouppairApi.HandleFunc("/{id}", (http.HandlerFunc(c.processGroupPairHandler))).Name("item")
	processgrouppairApi.NotFoundHandler = (http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))

	var processpairApi = api1.PathPrefix("/processpairs").Subrouter()
	processpairApi.StrictSlash(true)
	processpairApi.HandleFunc("/", (http.HandlerFunc(c.processPairHandler))).Name("list")
	processpairApi.HandleFunc("/{id}", (http.HandlerFunc(c.processPairHandler))).Name("item")
	processpairApi.NotFoundHandler = (http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))

	if cfg.EnableConsole {
		mux.PathPrefix("/").Handler(http.FileServer(http.Dir(cfg.ConsoleLocation)))
	}

	var collectorApi = api1.PathPrefix("/collectors").Subrouter()
	collectorApi.StrictSlash(true)
	collectorApi.HandleFunc("/", (http.HandlerFunc(c.collectorHandler))).Name("list")
	collectorApi.HandleFunc("/{id}", (http.HandlerFunc(c.collectorHandler))).Name("item")
	collectorApi.HandleFunc("/{id}/connectors-to-process", (http.HandlerFunc(c.collectorHandler))).Name("connectors-to-process")
	collectorApi.HandleFunc("/{id}/flows-to-pair", (http.HandlerFunc(c.collectorHandler))).Name("flows-to-pair")
	collectorApi.HandleFunc("/{id}/flows-to-process", (http.HandlerFunc(c.collectorHandler))).Name("flows-to-process")
	collectorApi.HandleFunc("/{id}/pair-to-aggregate", (http.HandlerFunc(c.collectorHandler))).Name("pair-to-aggregate")
	collectorApi.NotFoundHandler = (http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))

	s := &http.Server{
		Addr:         cfg.APIListenAddress,
		Handler:      handlers.CompressHandler(mux),
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
	}
	tlsEnabled := cfg.APITLS.Cert != ""
	if tlsEnabled {
		cert, err := tls.LoadX509KeyPair(cfg.APITLS.Cert, cfg.APITLS.Key)
		if err != nil {
			return fmt.Errorf("could not set up certs for api server: %s", err)
		}
		s.TLSConfig = &tls.Config{
			MinVersion:   tls.VersionTLS13,
			Certificates: []tls.Certificate{cert},
		}
	}

	runErrors := make(chan error, 1)
	go func() {
		slog.Info("Starting Network Console API Server",
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
			runErrors <- fmt.Errorf("server error running api server: %s", err)
		}
	}()

	if cfg.EnableProfile {
		// serve only over localhost loopback
		const pprofAddr = "localhost:9970"
		go func() {
			slog.Info("Starting Network Console Profiling Server",
				slog.String("address", pprofAddr))
			if err := http.ListenAndServe(pprofAddr, nil); err != nil {
				runErrors <- fmt.Errorf("server error running profiler server: %s", err)
			}
		}()
	}

	stopController := make(chan struct{})
	defer close(stopController)
	go func() {
		if err = c.Run(stopController); err != nil {
			runErrors <- fmt.Errorf("collector error: %s", err)
		}
	}()

	select {
	case err := <-runErrors:
		return err
	case <-ctx.Done():
	}

	shutdownCtx, sCancel := context.WithTimeout(context.Background(), time.Second)
	defer sCancel()
	if err := s.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown did not complete gracefully: %s", err)
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
	flags.BoolVar(&cfg.RouterTLS.Verify, "router-tls-verify", true, "Set to false to skip verifying the router tls ca")

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

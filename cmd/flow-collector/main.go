package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path"
	"strconv"
	"syscall"
	"time"

	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	clientpodman "github.com/skupperproject/skupper/client/podman"
	"github.com/skupperproject/skupper/pkg/config"
	"github.com/skupperproject/skupper/pkg/domain/podman"
	"github.com/skupperproject/skupper/pkg/utils"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/client"
	"github.com/skupperproject/skupper/pkg/certs"
	"github.com/skupperproject/skupper/pkg/kube"
	"github.com/skupperproject/skupper/pkg/version"
)

// should this be in utils?
type tlsConfig struct {
	Ca     string `json:"ca,omitempty"`
	Cert   string `json:"cert,omitempty"`
	Key    string `json:"key,omitempty"`
	Verify bool   `json:"recType,omitempty"`
}

type connectJson struct {
	Scheme string    `json:"scheme,omitempty"`
	Host   string    `json:"host,omitempty"`
	Port   string    `json:"port,omitempty"`
	Tls    tlsConfig `json:"tls,omitempty"`
}

var onlyOneSignalHandler = make(chan struct{})
var shutdownSignals = []os.Signal{os.Interrupt, syscall.SIGTERM}

func getConnectInfo(file string) (connectJson, error) {
	cj := connectJson{}

	jsonFile, err := ioutil.ReadFile(file)
	if err != nil {
		return cj, err
	}

	err = json.Unmarshal(jsonFile, &cj)
	if err != nil {
		return cj, err
	}

	return cj, nil
}

func SetupSignalHandler() (stopCh <-chan struct{}) {
	close(onlyOneSignalHandler) // panics when called twice

	stop := make(chan struct{})
	c := make(chan os.Signal, 2)
	signal.Notify(c, shutdownSignals...)
	go func() {
		<-c
		close(stop)
		<-c
		os.Exit(1) // second signal. Exit directly.
	}()

	return stop
}

func cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin == "" {
			origin = "*"
		}
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Allow-Methods", "GET,POST,DELETE")
		next.ServeHTTP(w, r)
	})
}

func authenticate(dir string, user string, password string) bool {
	filename := path.Join(dir, user)
	file, err := os.Open(filename)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			log.Printf("Failed to authenticate %s, no such user exists", user)
		} else {
			log.Printf("Failed to authenticate %s: %s", user, err)
		}
		return false
	}
	defer file.Close()

	bytes, err := ioutil.ReadAll(file)
	if err != nil {
		log.Printf("Failed to authenticate %s: %s", user, err)
		return false
	}
	return string(bytes) == password
}

func authenticated(h http.HandlerFunc) http.HandlerFunc {
	dir := os.Getenv("FLOW_USERS")

	if dir != "" {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user, password, ok := r.BasicAuth()

			if ok && authenticate(dir, user, password) {
				h.ServeHTTP(w, r)
			} else {
				w.Header().Set("WWW-Authenticate", "Basic realm=skupper")
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
			}
		})
	} else {
		return h
	}
}

func main() {
	// if -version used, report and exit
	isVersion := flag.Bool("version", false, "Report the version of the Skupper Flow Collector")
	flag.Parse()
	if *isVersion {
		fmt.Println(version.Version)
		os.Exit(0)
	}

	// Startup message
	log.Printf("Skupper Flow collector controller")
	log.Printf("Version: %s", version.Version)

	origin := os.Getenv("SKUPPER_SITE_ID")
	namespace := os.Getenv("SKUPPER_NAMESPACE")

	// set up signals so we handle the first shutdown signal gracefully
	stopCh := SetupSignalHandler()

	platform := config.GetPlatform()
	var flowRecordTtl time.Duration
	var enableConsole bool
	var prometheusHost string
	var prometheusAuthMethod string
	var prometheusUser string
	var prometheusPassword string

	// waiting on skupper-router to be available
	if platform == "" || platform == types.PlatformKubernetes {
		cli, err := client.NewClient(namespace, "", "")
		if err != nil {
			log.Fatal("Error getting van client", err.Error())
		}

		log.Println("Waiting for Skupper router component to start")
		_, err = kube.WaitDeploymentReady(types.TransportDeploymentName, namespace, cli.KubeClient, time.Second*180, time.Second*5)
		if err != nil {
			log.Fatal("Error waiting for transport deployment to be ready: ", err.Error())
		}

		siteConfig, err := cli.SiteConfigInspect(context.Background(), nil)
		if err != nil {
			log.Fatal("Error getting site config", err.Error())
		}

		flowRecordTtl = siteConfig.Spec.FlowCollector.FlowRecordTtl
		enableConsole = siteConfig.Spec.EnableConsole
		prometheusAuthMethod = siteConfig.Spec.PrometheusServer.AuthMode
		prometheusUser = siteConfig.Spec.PrometheusServer.User
		prometheusPassword = siteConfig.Spec.PrometheusServer.Password
		promProtocol := "https://"
		if prometheusAuthMethod == "basic" {
			promProtocol = "http://"
		}
		if siteConfig.Spec.PrometheusServer.ExternalServer != "" {
			prometheusHost = promProtocol + siteConfig.Spec.PrometheusServer.ExternalServer + ":9090" + "/api/v1"
			//	} else if siteConfig.Spec.Ingress == types.IngressRouteString {
		} else if siteConfig.Spec.IsIngressRoute() {
			route, err := kube.GetRoute(types.PrometheusRouteName, namespace, cli.RouteClient)
			if err != nil {
				log.Fatal("Error getting prometheus host", err.Error())
			}
			prometheusHost = promProtocol + route.Spec.Host + "/api/v1"
		} else if siteConfig.Spec.IsIngressLoadBalancer() {
			service, err := kube.GetService(types.PrometheusServiceName, namespace, cli.KubeClient)
			if err == nil {
				host := kube.GetLoadBalancerHostOrIP(service)
				prometheusHost = promProtocol + host + ":9090" + "/api/v1"
			}
		} else if siteConfig.Spec.IsIngressNginxIngress() || siteConfig.Spec.IsIngressKubernetes() {
			routes, err := kube.GetIngressRoutes("skupper-prometheus", namespace, cli)
			if err == nil {
				host := routes[0].Host
				port := strconv.Itoa(routes[0].ServicePort)
				prometheusHost = promProtocol + host + ":" + port + "/api/v1"
			}
		}
	} else {
		cfg, err := podman.NewPodmanConfigFileHandler().GetConfig()
		if err != nil {
			log.Fatal("Error reading podman site config", err)
		}
		podmanCli, err := clientpodman.NewPodmanClient(cfg.Endpoint, "")
		if err != nil {
			log.Fatal("Error creating podman client", err)
		}
		err = utils.Retry(time.Second, 120, func() (bool, error) {
			router, err := podmanCli.ContainerInspect(types.TransportDeploymentName)
			if err != nil {
				return false, fmt.Errorf("error retrieving %s container state - %w", types.TransportDeploymentName, err)
			}
			if !router.Running {
				return false, nil
			}
			return true, nil
		})
		if err != nil {
			log.Fatalf("unable to determine if %s container is running - %s", types.TransportDeploymentName, err)
		}
		flowRecordTtl, _ = time.ParseDuration(os.Getenv("FLOW_RECORD_TTL"))
		enableConsole, _ = strconv.ParseBool(os.Getenv("ENABLE_CONSOLE"))
		prometheusHost = os.Getenv("PROMETHEUS_HOST")
		prometheusAuthMethod = os.Getenv("PROMETHEUS_AUTH_METHOD")
		prometheusUser = os.Getenv("PROMETHEUS_USER")
		prometheusPassword = os.Getenv("PROMETHEUS_PASSWORD")
	}

	tlsConfig, err := certs.GetTlsConfig(true, types.ControllerConfigPath+"tls.crt", types.ControllerConfigPath+"tls.key", types.ControllerConfigPath+"ca.crt")
	if err != nil {
		log.Fatal("Error getting tls config", err.Error())
	}

	conn, err := getConnectInfo(types.ControllerConfigPath + "connect.json")
	if err != nil {
		log.Fatal("Error getting connect.json", err.Error())
	}

	reg := prometheus.NewRegistry()
	c, err := NewController(origin, reg, conn.Scheme, conn.Host, conn.Port, tlsConfig, flowRecordTtl)
	if err != nil {
		log.Fatal("Error getting new flow collector ", err.Error())
	}

	c.FlowCollector.Collector.PrometheusHost = prometheusHost
	c.FlowCollector.Collector.PrometheusAuthMethod = prometheusAuthMethod
	c.FlowCollector.Collector.PrometheusUser = prometheusUser
	c.FlowCollector.Collector.PrometheusPassword = prometheusPassword

	var mux = mux.NewRouter().StrictSlash(true)

	var api = mux.PathPrefix("/api").Subrouter()
	api.NotFoundHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	if os.Getenv("USE_CORS") != "" {
		api.Use(cors)
	}

	var api1 = api.PathPrefix("/v1alpha1").Subrouter()
	api1.NotFoundHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	var logUri = os.Getenv("LOG_REQ_URI")
	if logUri == "true" {
		api1.Use(func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				log.Println(r.RequestURI)
				next.ServeHTTP(w, r)
			})
		})
	}

	var metricsApi = api1.PathPrefix("/metrics").Subrouter()
	metricsApi.StrictSlash(true)
	metricsApi.Handle("/", promhttp.HandlerFor(reg, promhttp.HandlerOpts{Registry: reg}))

	var eventsourceApi = api1.PathPrefix("/eventsources").Subrouter()
	eventsourceApi.StrictSlash(true)
	eventsourceApi.HandleFunc("/", http.HandlerFunc(c.eventsourceHandler)).Name("list")
	eventsourceApi.HandleFunc("/{id}", http.HandlerFunc(c.eventsourceHandler)).Name("item")
	eventsourceApi.NotFoundHandler = authenticated(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))

	var siteApi = api1.PathPrefix("/sites").Subrouter()
	siteApi.StrictSlash(true)
	siteApi.HandleFunc("/", authenticated(http.HandlerFunc(c.siteHandler))).Name("list")
	siteApi.HandleFunc("/{id}", authenticated(http.HandlerFunc(c.siteHandler))).Name("item")
	siteApi.HandleFunc("/{id}/processes", authenticated(http.HandlerFunc(c.siteHandler))).Name("processes")
	siteApi.HandleFunc("/{id}/routers", authenticated(http.HandlerFunc(c.siteHandler))).Name("routers")
	siteApi.HandleFunc("/{id}/links", authenticated(http.HandlerFunc(c.siteHandler))).Name("links")
	siteApi.HandleFunc("/{id}/hosts", authenticated(http.HandlerFunc(c.siteHandler))).Name("hosts")
	siteApi.NotFoundHandler = authenticated(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))

	var hostApi = api1.PathPrefix("/hosts").Subrouter()
	hostApi.StrictSlash(true)
	hostApi.HandleFunc("/", authenticated(http.HandlerFunc(c.hostHandler))).Name("list")
	hostApi.HandleFunc("/{id}", authenticated(http.HandlerFunc(c.hostHandler))).Name("item")
	hostApi.NotFoundHandler = authenticated(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))

	var routerApi = api1.PathPrefix("/routers").Subrouter()
	routerApi.StrictSlash(true)
	routerApi.HandleFunc("/", authenticated(http.HandlerFunc(c.routerHandler))).Name("list")
	routerApi.HandleFunc("/{id}", authenticated(http.HandlerFunc(c.routerHandler))).Name("item")
	routerApi.HandleFunc("/{id}/flows", authenticated(http.HandlerFunc(c.routerHandler))).Name("flows")
	routerApi.HandleFunc("/{id}/links", authenticated(http.HandlerFunc(c.routerHandler))).Name("links")
	routerApi.HandleFunc("/{id}/listeners", authenticated(http.HandlerFunc(c.routerHandler))).Name("listeners")
	routerApi.HandleFunc("/{id}/connectors", authenticated(http.HandlerFunc(c.routerHandler))).Name("connectors")
	routerApi.NotFoundHandler = authenticated(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))

	var linkApi = api1.PathPrefix("/links").Subrouter()
	linkApi.StrictSlash(true)
	linkApi.HandleFunc("/", authenticated(http.HandlerFunc(c.linkHandler))).Name("list")
	linkApi.HandleFunc("/{id}", authenticated(http.HandlerFunc(c.linkHandler))).Name("item")
	linkApi.NotFoundHandler = authenticated(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))

	var listenerApi = api1.PathPrefix("/listeners").Subrouter()
	listenerApi.StrictSlash(true)
	listenerApi.HandleFunc("/", authenticated(http.HandlerFunc(c.listenerHandler))).Name("list")
	listenerApi.HandleFunc("/{id}", authenticated(http.HandlerFunc(c.listenerHandler))).Name("item")
	listenerApi.HandleFunc("/{id}/flows", authenticated(http.HandlerFunc(c.listenerHandler))).Name("flows")
	listenerApi.NotFoundHandler = authenticated(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))

	var connectorApi = api1.PathPrefix("/connectors").Subrouter()
	connectorApi.StrictSlash(true)
	connectorApi.HandleFunc("/", authenticated(http.HandlerFunc(c.connectorHandler))).Name("list")
	connectorApi.HandleFunc("/{id}", authenticated(http.HandlerFunc(c.connectorHandler))).Name("item")
	connectorApi.HandleFunc("/{id}/flows", authenticated(http.HandlerFunc(c.connectorHandler))).Name("flows")
	connectorApi.HandleFunc("/{id}/process", authenticated(http.HandlerFunc(c.connectorHandler))).Name("process")
	connectorApi.NotFoundHandler = authenticated(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))

	var addressApi = api1.PathPrefix("/addresses").Subrouter()
	addressApi.StrictSlash(true)
	addressApi.HandleFunc("/", authenticated(http.HandlerFunc(c.addressHandler))).Name("list")
	addressApi.HandleFunc("/{id}", authenticated(http.HandlerFunc(c.addressHandler))).Name("item")
	addressApi.HandleFunc("/{id}/processes", authenticated(http.HandlerFunc(c.addressHandler))).Name("processes")
	addressApi.HandleFunc("/{id}/flows", authenticated(http.HandlerFunc(c.addressHandler))).Name("flows")
	addressApi.HandleFunc("/{id}/flowpairs", authenticated(http.HandlerFunc(c.addressHandler))).Name("flowpairs")
	addressApi.HandleFunc("/{id}/listeners", authenticated(http.HandlerFunc(c.addressHandler))).Name("listeners")
	addressApi.HandleFunc("/{id}/connectors", authenticated(http.HandlerFunc(c.addressHandler))).Name("connectors")
	addressApi.NotFoundHandler = authenticated(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))

	var processApi = api1.PathPrefix("/processes").Subrouter()
	processApi.StrictSlash(true)
	processApi.HandleFunc("/", authenticated(http.HandlerFunc(c.processHandler))).Name("list")
	processApi.HandleFunc("/{id}", authenticated(http.HandlerFunc(c.processHandler))).Name("item")
	processApi.HandleFunc("/{id}/flows", authenticated(http.HandlerFunc(c.processHandler))).Name("flows")
	processApi.HandleFunc("/{id}/addresses", authenticated(http.HandlerFunc(c.processHandler))).Name("addresses")
	processApi.HandleFunc("/{id}/connector", authenticated(http.HandlerFunc(c.processHandler))).Name("connector")
	processApi.NotFoundHandler = authenticated(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))

	var processGroupApi = api1.PathPrefix("/processgroups").Subrouter()
	processGroupApi.StrictSlash(true)
	processGroupApi.HandleFunc("/", authenticated(http.HandlerFunc(c.processGroupHandler))).Name("list")
	processGroupApi.HandleFunc("/{id}", authenticated(http.HandlerFunc(c.processGroupHandler))).Name("item")
	processGroupApi.HandleFunc("/{id}/processes", authenticated(http.HandlerFunc(c.processGroupHandler))).Name("processes")
	processGroupApi.NotFoundHandler = authenticated(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))

	var flowApi = api1.PathPrefix("/flows").Subrouter()
	flowApi.StrictSlash(true)
	flowApi.HandleFunc("/", authenticated(http.HandlerFunc(c.flowHandler))).Name("list")
	flowApi.HandleFunc("/{id}", authenticated(http.HandlerFunc(c.flowHandler))).Name("item")
	flowApi.HandleFunc("/{id}/process", authenticated(http.HandlerFunc(c.flowHandler))).Name("process")
	flowApi.NotFoundHandler = authenticated(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))

	var flowpairApi = api1.PathPrefix("/flowpairs").Subrouter()
	flowpairApi.StrictSlash(true)
	flowpairApi.HandleFunc("/", authenticated(http.HandlerFunc(c.flowPairHandler))).Name("list")
	flowpairApi.HandleFunc("/{id}", authenticated(http.HandlerFunc(c.flowPairHandler))).Name("item")
	flowpairApi.NotFoundHandler = authenticated(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))

	var sitepairApi = api1.PathPrefix("/sitepairs").Subrouter()
	sitepairApi.StrictSlash(true)
	sitepairApi.HandleFunc("/", authenticated(http.HandlerFunc(c.sitePairHandler))).Name("list")
	sitepairApi.HandleFunc("/{id}", authenticated(http.HandlerFunc(c.sitePairHandler))).Name("item")
	sitepairApi.NotFoundHandler = authenticated(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))

	var processgrouppairApi = api1.PathPrefix("/processgrouppairs").Subrouter()
	processgrouppairApi.StrictSlash(true)
	processgrouppairApi.HandleFunc("/", authenticated(http.HandlerFunc(c.processGroupPairHandler))).Name("list")
	processgrouppairApi.HandleFunc("/{id}", authenticated(http.HandlerFunc(c.processGroupPairHandler))).Name("item")
	processgrouppairApi.NotFoundHandler = authenticated(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))

	var processpairApi = api1.PathPrefix("/processpairs").Subrouter()
	processpairApi.StrictSlash(true)
	processpairApi.HandleFunc("/", authenticated(http.HandlerFunc(c.processPairHandler))).Name("list")
	processpairApi.HandleFunc("/{id}", authenticated(http.HandlerFunc(c.processPairHandler))).Name("item")
	processpairApi.NotFoundHandler = authenticated(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))

	if enableConsole {
		mux.PathPrefix("/").Handler(http.FileServer(http.Dir("/app/console/")))
	} else {
		log.Println("Skupper console is disabled")
	}

	var collectorApi = api1.PathPrefix("/collectors").Subrouter()
	collectorApi.StrictSlash(true)
	collectorApi.HandleFunc("/", authenticated(http.HandlerFunc(c.collectorHandler))).Name("list")
	collectorApi.HandleFunc("/{id}", authenticated(http.HandlerFunc(c.collectorHandler))).Name("item")
	collectorApi.NotFoundHandler = authenticated(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))

	addr := ":8010"
	if os.Getenv("FLOW_PORT") != "" {
		addr = ":" + os.Getenv("FLOW_PORT")
	}
	if os.Getenv("FLOW_HOST") != "" {
		addr = os.Getenv("FLOW_HOST") + addr
	}
	log.Printf("Flow collector server listing on %s", addr)
	s := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	go func() {
		_, err := os.Stat("/etc/service-controller/console/tls.crt")
		if err == nil {
			err := s.ListenAndServeTLS("/etc/service-controller/console/tls.crt", "/etc/service-controller/console/tls.key")
			if err != nil {
				fmt.Println(err)
			}
		} else {
			err := s.ListenAndServe()
			if err != nil {
				fmt.Println(err)
			}
		}
	}()

	if err = c.Run(stopCh); err != nil {
		log.Fatal("Error running Flow collector: ", err.Error())
	}

}

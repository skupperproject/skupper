package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/prometheus/client_golang/prometheus"
	iflag "github.com/skupperproject/skupper/internal/flag"
	"github.com/skupperproject/skupper/internal/kube/adaptor"
	internalclient "github.com/skupperproject/skupper/internal/kube/client"
	"github.com/skupperproject/skupper/internal/kube/metrics"
	"github.com/skupperproject/skupper/internal/qdr"
	"github.com/skupperproject/skupper/internal/version"
)

var onlyOneSignalHandler = make(chan struct{})
var shutdownSignals = []os.Signal{os.Interrupt, syscall.SIGTERM}

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

func main() {
	flags := flag.NewFlagSet("", flag.ExitOnError)

	var namespace string
	var kubeconfig string
	iflag.StringVar(flags, &namespace, "namespace", "NAMESPACE", "", "The Kubernetes namespace scope for the controller")
	iflag.StringVar(flags, &kubeconfig, "kubeconfig", "KUBECONFIG", "", "A path to the kubeconfig file to use")

	var configDir string
	var configMapName string
	iflag.StringVar(flags, &configDir, "config-dir", "SKUPPER_CONFIG_DIR", "/etc/skupper-router-certs", "The directory to which configuration should be saved")
	iflag.StringVar(flags, &configMapName, "router-config", "SKUPPER_ROUTER_CONFIG", "skupper-router", "The name of the ConfigMap containing the router config")

	// if -version used, report and exit
	isVersion := flags.Bool("version", false, "Report the version of Config Sync")
	isInit := flags.Bool("init", false, "Downloads configuration and ssl profile artefacts")

	metricsConfig, err := metrics.BoundConfig(flags)
	if err != nil {
		log.Fatalf("Error reading metrics configuration: %s", err)
	}
	flags.Parse(os.Args[1:])
	if *isVersion {
		fmt.Println(version.Version)
		os.Exit(0)
	}

	// Startup message
	log.Printf("Version: %s", version.Version)

	// set up signals so we handle the first shutdown signal gracefully
	stopCh := SetupSignalHandler()

	cli, err := internalclient.NewClient(namespace, "", kubeconfig)
	if err != nil {
		log.Fatal("Error getting van client: ", err.Error())
	}

	if *isInit {
		if err := adaptor.InitialiseConfig(cli, cli.GetNamespace(), configDir, configMapName); err != nil {
			log.Fatal("Error initialising config ", err.Error())
		}
		os.Exit(0)
	}

	if !metricsConfig.Disabled {
		reg := prometheus.NewRegistry()
		metrics.MustRegisterClientGoMetrics(reg)
		srv := metrics.NewServer(metricsConfig, reg)
		if err := srv.Start(stopCh); err != nil {
			log.Fatalf("Error starting metrics server: %s", err)
		}
	}

	log.Println("Waiting for Skupper router to be ready")
	if err := waitForAMQPConnection("amqp://localhost:5672", time.Second*180, time.Second*5); err != nil {
		log.Fatal("Error waiting for router ", err.Error())
	}

	log.Println("Starting collector...")
	go adaptor.StartCollector(cli)

	//start health check
	http.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte("ok"))
	})
	go http.ListenAndServe(":9191", nil)

	configSync := adaptor.NewConfigSync(cli, cli.GetNamespace(), configDir, configMapName)
	log.Println("Starting controller loop...")
	configSync.Start(stopCh)

	<-stopCh
	log.Println("Shutting down...")
	configSync.Stop()
}

func waitForAMQPConnection(address string, timeout, interval time.Duration) error {
	b := backoff.NewExponentialBackOff(backoff.WithMaxElapsedTime(timeout), backoff.WithMaxInterval(interval))
	pool := qdr.NewAgentPool(address, nil)
	pool.SetConnectionTimeout(interval)
	return backoff.Retry(
		func() error {
			agent, err := pool.Get()
			if err != nil {
				return err
			}
			agent.Close()
			log.Printf("Connected to router at %q", address)
			return nil
		}, b)
}

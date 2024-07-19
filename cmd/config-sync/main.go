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

	corev1 "k8s.io/api/core/v1"

	internalclient "github.com/skupperproject/skupper/internal/kube/client"
	"github.com/skupperproject/skupper/pkg/kube"
	"github.com/skupperproject/skupper/pkg/version"
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

const SHARED_TLS_DIRECTORY = "/etc/skupper-router-certs"

func main() {
	// if -version used, report and exit
	isVersion := flag.Bool("version", false, "Report the version of Config Sync")
	flag.Parse()
	if *isVersion {
		fmt.Println(version.Version)
		os.Exit(0)
	}

	// Startup message
	log.Printf("CONFIG_SYNC: Version: %s", version.Version)

	// set up signals so we handle the first shutdown signal gracefully
	stopCh := SetupSignalHandler()

	namespace, err := kube.CurrentNamespace()
	if err != nil {
		log.Fatal("Error determining namespace: ", err.Error())
	}

	cli, err := internalclient.NewClient(namespace, "", "")
	if err != nil {
		log.Fatal("Error getting van client: ", err.Error())
	}

	log.Println("CONFIG_SYNC: Waiting for Skupper router to be ready")
	_, err = kube.WaitForPodsSelectorStatus(namespace, cli.Kube, "skupper.io/component=router", corev1.PodRunning, time.Second*180, time.Second*5)
	if err != nil {
		log.Fatal("Error waiting for router pods to be ready ", err.Error())
	}

	log.Println("CONFIG_SYNC: Starting collector...")
	go startCollector(cli)

	//start health check
	http.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte("ok"))
	})
	go http.ListenAndServe(":9191", nil)

	routerConfigMap := os.Getenv("SKUPPER_CONFIG")
	if routerConfigMap == "" {
		routerConfigMap = "skupper-internal" // change defult?
	}

	configSync := newConfigSync(cli, namespace, SHARED_TLS_DIRECTORY, routerConfigMap)
	log.Println("CONFIG_SYNC: Starting controller loop...")
	configSync.start(stopCh)

	<-stopCh
	log.Println("CONFIG_SYNC: Shutting down...")
	configSync.stop()
}

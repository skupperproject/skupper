package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/client"
	"github.com/skupperproject/skupper/pkg/certs"
	"github.com/skupperproject/skupper/pkg/event"
	"github.com/skupperproject/skupper/pkg/kube"
	"github.com/skupperproject/skupper/pkg/version"
)

func describe(i interface{}) {
	fmt.Printf("(%v, %T)\n", i, i)
	fmt.Println()
}

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
	// if -version used, report and exit
	isVersion := flag.Bool("version", false, "Report the version of the Skupper Service Controller")
	flag.Parse()
	if *isVersion {
		fmt.Println(version.Version)
		os.Exit(0)
	}

	// Startup message
	log.Printf("Skupper service controller")
	log.Printf("Version: %s", version.Version)

	origin := os.Getenv("SKUPPER_SITE_ID")
	namespace := os.Getenv("SKUPPER_NAMESPACE")
	disableServiceSync := os.Getenv("SKUPPER_DISABLE_SERVICE_SYNC")

	// set up signals so we handle the first shutdown signal gracefully
	stopCh := SetupSignalHandler()

	// todo, get context from env?
	cli, err := client.NewClient(namespace, "", "")
	if err != nil {
		log.Fatal("Error getting van client", err.Error())
	}

	tlsConfig := certs.GetTlsConfigRetriever(true, types.ControllerConfigPath+"tls.crt", types.ControllerConfigPath+"tls.key", types.ControllerConfigPath+"ca.crt")

	event.StartDefaultEventStore(stopCh)

	controller, err := NewController(cli, origin, tlsConfig, disableServiceSync == "true")
	if err != nil {
		log.Fatal("Error getting new controller", err.Error())
	}

	log.Println("Waiting for Skupper router component to start")
	_, err = kube.WaitDeploymentReady(types.TransportDeploymentName, namespace, cli.KubeClient, time.Second*180, time.Second)
	if err != nil {
		log.Fatal("Error waiting for transport deployment to be ready: ", err.Error())
	}

	// start the controller workers
	if err = controller.Run(stopCh); err != nil {
		log.Fatal("Error running controller: ", err.Error())
	}
}

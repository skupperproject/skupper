package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/pkg/certs"
	podmancontroller "github.com/skupperproject/skupper/pkg/domain/podman/controller"
	"github.com/skupperproject/skupper/pkg/version"
)

var onlyOneSignalHandler = make(chan struct{})
var shutdownSignals = []os.Signal{os.Interrupt, syscall.SIGTERM}

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

	// set up signals so we handle the first shutdown signal gracefully
	stopCh := SetupSignalHandler()

	tlsConfig, err := certs.GetTlsConfig(true, types.ControllerConfigPath+"tls.crt", types.ControllerConfigPath+"tls.key", types.ControllerConfigPath+"ca.crt")
	if err != nil {
		log.Fatal("Error getting tls config", err.Error())
	}

	controller, err := podmancontroller.NewControllerPodman(origin, tlsConfig)
	if err != nil {
		log.Fatal("Error getting new controller", err.Error())
	}
	if err = controller.Run(stopCh); err != nil {
		log.Fatal("Error running controller:", err.Error())
	}
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

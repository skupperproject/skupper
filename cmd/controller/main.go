package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	internalclient "github.com/skupperproject/skupper/internal/kube/client"
	"github.com/skupperproject/skupper/internal/kube/controller"
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
	flags := flag.NewFlagSet("", flag.ExitOnError)
	config, err := controller.BoundConfig(flags)
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}

	// if -version used, report and exit
	isVersion := flags.Bool("version", false, "Report the version of the Skupper Controller")
	flags.Parse(os.Args[1:])
	if *isVersion {
		fmt.Println(version.Version)
		os.Exit(0)
	}
	log.Printf("Version: %s", version.Version)
	if config.WatchNamespace == metav1.NamespaceAll {
		log.Println("Skupper controller watching all namespaces")
	} else {
		log.Println("Skupper controller watching namespace", config.WatchNamespace)
	}

	// set up signals so we handle the first shutdown signal gracefully
	stopCh := SetupSignalHandler()

	cli, err := internalclient.NewClient(config.Namespace, "", config.Kubeconfig)
	if err != nil {
		log.Fatal("Error getting van client ", err.Error())
	}

	controller, err := controller.NewController(cli, config)
	if err != nil {
		log.Fatal("Error getting new site controller ", err.Error())
	}

	if err = controller.Run(stopCh); err != nil {
		log.Fatal("Error running site controller: ", err.Error())
	}
}

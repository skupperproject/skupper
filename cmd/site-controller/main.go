package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/skupperproject/skupper/client"
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
	isVersion := flag.Bool("version", false, "Report the version of the Skupper Site Controller")
	flag.Parse()
	if *isVersion {
		fmt.Println(version.Version)
		os.Exit(0)
	}

	namespace := os.Getenv("NAMESPACE")
	kubeconfig := os.Getenv("KUBECONFIG")

	// set up signals so we handle the first shutdown signal gracefully
	stopCh := SetupSignalHandler()

	// todo, get context from env?
	cli, err := client.NewClient(namespace, "", kubeconfig)
	if err != nil {
		log.Fatal("Error getting van client ", err.Error())
	}

	controller, err := NewSiteController(cli)
	if err != nil {
		log.Fatal("Error getting new site controller ", err.Error())
	}

	if err = controller.Run(stopCh); err != nil {
		log.Fatal("Error running site controller: ", err.Error())
	}
}

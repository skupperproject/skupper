package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/skupperproject/skupper/internal/nonkube/controller"
	"github.com/skupperproject/skupper/internal/version"
	"github.com/skupperproject/skupper/pkg/nonkube/api"
)

func main() {
	parseFlags()

	log.Printf("Version: %s", version.Version)
	namespacesPath := api.GetHostNamespacesPath()
	log.Printf("Skupper User Controller watching %s", namespacesPath)
	if err := os.MkdirAll(namespacesPath, 0755); err != nil {
		log.Fatalf("Error creating skupper namespaces directory %q: %v", namespacesPath, err)
	}

	c, err := controller.NewController()
	if err != nil {
		log.Fatalf("Error creating controller: %v", err)
	}
	stop, wait := c.Start()

	handleShutdown(stop, wait)
}

func handleShutdown(stop chan struct{}, wait *sync.WaitGroup) {
	sigs := make(chan os.Signal, 2)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	<-sigs
	log.Println("Shutting down user controller")
	close(stop)

	graceful := make(chan struct{})
	go func() {
		wait.Wait()
		close(graceful)
	}()

	force := time.NewTicker(6 * time.Second)
	for {
		select {
		case <-sigs:
			log.Println("Second interrupt, forcing user controller shutdown")
			os.Exit(1)
		case <-force.C:
			log.Println("Graceful shutdown timed out, exiting now")
			os.Exit(1)
		case <-graceful:
			log.Println("Exiting user controller gracefully")
			os.Exit(0)
		}
	}
}

func parseFlags() {
	flags := flag.NewFlagSet("", flag.ExitOnError)
	isVersion := flags.Bool("version", false, "Report the version of the Skupper User Controller")
	flags.Parse(os.Args[1:])
	if *isVersion {
		fmt.Println(version.Version)
		os.Exit(0)
	}
}

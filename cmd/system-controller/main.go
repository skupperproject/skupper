package main

import (
	"flag"
	"fmt"
	"log/slog"
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
	slog.Info("Version info:", slog.String("version", version.Version))
	namespacesPath := api.GetDefaultOutputNamespacesPath()
	slog.Info("Skupper System Controller watching:", slog.String("path", namespacesPath))
	if api.IsRunningInContainer() {
		slog.Info("Host path info:", slog.String("path", api.GetHostNamespacesPath()))
	}
	if err := os.MkdirAll(namespacesPath, 0755); err != nil {
		slog.Error("Error creating skupper namespaces directory", slog.String("path", namespacesPath), slog.Any("error", err))
		os.Exit(1)
	}

	c, err := controller.NewController()
	if err != nil {
		slog.Error("Error creating controller", slog.Any("error", err))
		os.Exit(1)
	}
	stop, wait := c.Start()

	handleShutdown(stop, wait)
}

func handleShutdown(stop chan struct{}, wait *sync.WaitGroup) {
	sigs := make(chan os.Signal, 2)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	<-sigs
	slog.Info("Shutting down system controller")

	close(stop)

	graceful := make(chan struct{})
	go func() {
		wait.Wait()
		close(graceful)
	}()

	gracefulTimeout := time.NewTicker(10 * time.Second)
	for {
		select {
		case <-sigs:
			slog.Info("Second interrupt, forcing system controller shutdown")
			os.Exit(1)
		case <-gracefulTimeout.C:
			slog.Info("Graceful shutdown timed out, exiting now")
			os.Exit(1)
		case <-graceful:
			slog.Info("System controller shutdown completed")
			os.Exit(0)
		}
	}
}

func parseFlags() {
	flags := flag.NewFlagSet("", flag.ExitOnError)
	isVersion := flags.Bool("version", false, "Report the version of the Skupper System Controller")
	flags.Parse(os.Args[1:])
	if *isVersion {
		fmt.Println(version.Version)
		os.Exit(0)
	}
}

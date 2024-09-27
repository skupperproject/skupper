package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"time"

	"github.com/c-kruse/vanflow/session"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	flags           *flag.FlagSet = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	MessagingConfig string
	ExtensionName   string
	MetricsEnabled  bool

	Debug bool
)

func init() {
	flags.Usage = func() {
		fmt.Printf(`Usage of %s:
`, os.Args[0])
		flags.PrintDefaults()
	}
	flags.BoolVar(&Debug, "debug", false, "enable debug logging")
	flags.BoolVar(&MetricsEnabled, "metrics", true, "enables kiali metrics")
	flags.StringVar(&ExtensionName, "extension", "skupper", "sets the extension name")
	flags.StringVar(&MessagingConfig, "messaging-config", "", "path to a skupper connect.json")

	flags.Parse(os.Args[1:])
}

func main() {
	if Debug {
		logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
		slog.SetDefault(logger)
	}

	connectionFactory, err := parseMessagingConfig(MessagingConfig)
	if err != nil {
		fmt.Println(err.Error())
		flags.Usage()
		os.Exit(1)
	}

	if err := run(connectionFactory); err != nil {
		os.Exit(1)
	}
}

func run(factory session.ContainerFactory) error {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	var wg sync.WaitGroup
	collector, err := newFlowCollector(factory, ExtensionName)
	if err != nil {
		return err
	}
	metricsMux := http.NewServeMux()
	metricsMux.Handle("/metrics", promhttp.Handler())
	srv := http.Server{
		Addr:    ":9000",
		Handler: metricsMux,
	}
	if MetricsEnabled {
		wg.Add(2)
		go func() {
			defer wg.Done()
			slog.Debug("starting metrics server")
			if err := srv.ListenAndServe(); err != http.ErrServerClosed {
				slog.Error("metrics server error", slog.Any("error", err))
			}
		}()
		go func() {
			defer wg.Done()
			slog.Debug("starting flow collector")
			if err := collector.Run(ctx); err != nil {
				slog.Error("flow collector error", slog.Any("error", err))
			}
		}()
	}

	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	srv.Shutdown(shutdownCtx)
	wg.Wait()
	return nil
}

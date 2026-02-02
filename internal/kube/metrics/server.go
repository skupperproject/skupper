package metrics

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	iflag "github.com/skupperproject/skupper/internal/flag"
)

const metricsPath = "/metrics"

type Config struct {
	Disabled bool
	Address  string
}

func BoundConfig(flags *flag.FlagSet) (*Config, error) {
	cfg := &Config{}
	err := iflag.BoolVar(flags, &cfg.Disabled, "disable-metrics", "SKUPPER_METRICS_DISABLE", false, "Set to disable metrics.")
	iflag.StringVar(flags, &cfg.Address, "metrics-address", "SKUPPER_METRICS_ADDRESS", ":9000", "The address for the metrics http server to listen on.")
	return cfg, err
}

func NewServer(cfg *Config, registry *prometheus.Registry) *Server {
	mux := http.NewServeMux()
	mux.Handle(metricsPath, promhttp.HandlerFor(registry, promhttp.HandlerOpts{}))
	srv := &http.Server{
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
		IdleTimeout:  120 * time.Second,
		Handler:      mux,
	}
	return &Server{
		config: *cfg,
		server: srv,
		logger: slog.New(slog.Default().Handler()).With(slog.String("component", "kube.metrics.server")),
	}
}

type Server struct {
	config Config
	server *http.Server
	logger *slog.Logger
}

func (s *Server) Start(stopCh <-chan struct{}) error {
	listenCtx, cancel := context.WithCancel(context.Background())
	go func() {
		select {
		case <-stopCh:
			cancel()
		case <-listenCtx.Done():
		}
	}()
	defer cancel()

	var lc net.ListenConfig
	ln, err := lc.Listen(listenCtx, "tcp", s.config.Address)
	if err != nil {
		return fmt.Errorf("failed to start listener: %s", err)
	}
	go func() {
		if err := s.server.Serve(ln); err != nil {
			s.logger.Error("metrics server error", slog.Any("error", err))
		}
	}()
	s.logger.Info("Started metrics server", slog.String("address", s.config.Address))
	return nil
}

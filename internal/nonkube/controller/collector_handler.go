package controller

import (
	"context"
	"log/slog"

	"github.com/skupperproject/skupper/internal/nonkube/flow"
)

func NewCollectorLifecycleHandler(namespace string) *CollectorLifecycleHandler {
	c := &CollectorLifecycleHandler{
		namespace: namespace,
	}
	c.logger = slog.New(slog.Default().Handler()).
		With("component", "collector.lifecycle.handler").
		With("namespace", namespace)
	return c
}

type CollectorLifecycleHandler struct {
	ctx       context.Context
	cancel    context.CancelFunc
	namespace string
	logger    *slog.Logger
	running   bool
}

func (c *CollectorLifecycleHandler) Start(stopCh <-chan struct{}) {
	if c.running {
		return
	}
	c.ctx, c.cancel = context.WithCancel(context.Background())
	c.running = true
	go c.startCollectorLite()
	go c.handleShutdown(stopCh)
}

func (c *CollectorLifecycleHandler) handleShutdown(stopCh <-chan struct{}) {
	<-stopCh
	c.Stop()
}

func (c *CollectorLifecycleHandler) Stop() {
	if !c.running {
		return
	}
	c.running = false
	if c.cancel != nil {
		c.logger.Info("Stopping collector")
		c.cancel()
	}
}

func (c *CollectorLifecycleHandler) Id() string {
	return "collector.handler"
}

func (c *CollectorLifecycleHandler) startCollectorLite() {
	//c.logger.Info("waiting for router local port to be available")
	//err := utils.RetryErrorWithContext(c.ctx, time.Second*5, func() error {
	//	port, err := runtime.GetLocalRouterPort(c.namespace)
	//	if err != nil {
	//		c.logger.Error("unable to determine local router port", slog.Any("error", err.Error()))
	//		return fmt.Errorf("unable to determine local router port: %w", err)
	//	}
	//	address := fmt.Sprintf("127.0.0.1:%d", port)
	//	conn, err := net.DialTimeout("tcp", address, time.Second)
	//	if err != nil {
	//		c.logger.Error("router is not yet available...",
	//			slog.Any("address", address),
	//			slog.Any("error", err.Error()))
	//		return fmt.Errorf("router is not yet available: %w", err)
	//	}
	//	_ = conn.Close()
	//	return nil
	//})
	//// context has been closed
	//if err != nil {
	//	return
	//}
	err := flow.StartCollector(c.ctx, c.namespace)
	if err != nil {
		c.logger.Error("error starting collector", slog.Any("error", err.Error()))
		return
	}
}

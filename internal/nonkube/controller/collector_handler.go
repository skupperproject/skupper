package controller

import (
	"context"
	"log/slog"
	"sync"

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
	ctx         context.Context
	cancel      context.CancelFunc
	namespace   string
	logger      *slog.Logger
	running     bool
	mux         sync.Mutex
	startMethod func()
}

func (c *CollectorLifecycleHandler) Start(stopCh <-chan struct{}) {
	c.mux.Lock()
	defer c.mux.Unlock()
	if c.running {
		return
	}
	c.logger.Info("Starting collector lifecycle handler")
	c.ctx, c.cancel = context.WithCancel(context.Background())
	c.running = true
	if c.startMethod == nil {
		c.startMethod = c.startCollectorLite
	}
	go c.startMethod()
	go c.handleShutdown(stopCh)
}

func (c *CollectorLifecycleHandler) handleShutdown(stopCh <-chan struct{}) {
	const stopMsg = "Stopping collector lifecycle handler"
	select {
	case <-stopCh:
		c.logger.Info(stopMsg)
		c.Stop()
	case <-c.ctx.Done():
		c.logger.Info(stopMsg)
	}
}

func (c *CollectorLifecycleHandler) Stop() {
	c.mux.Lock()
	defer c.mux.Unlock()
	if !c.running {
		return
	}
	c.running = false
	if c.cancel != nil {
		c.logger.Info("Stopping collector lite")
		c.cancel()
	}
}

func (c *CollectorLifecycleHandler) Id() string {
	return "collector.handler"
}

func (c *CollectorLifecycleHandler) startCollectorLite() {
	c.logger.Info("Starting collector lite")
	err := flow.StartCollector(c.ctx, c.namespace)
	if err != nil {
		c.logger.Error("error starting collector", slog.Any("error", err.Error()))
		return
	}
}

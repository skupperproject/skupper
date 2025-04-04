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
		With("namespace", namespace).
		With("component", "collector.lifecycle.handler")
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
	err := flow.StartCollector(c.ctx, c.namespace)
	if err != nil {
		c.logger.Error("error starting collector", slog.Any("error", err.Error()))
		return
	}
}

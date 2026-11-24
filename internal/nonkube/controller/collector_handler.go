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
	startMethod func(ctx context.Context)
	// lastDone is closed once the most recently started generation has
	// returned from startMethod. The next generation waits on it so that two
	// collectors never overlap on the shared status ConfigMap.
	lastDone <-chan struct{}
}

// Start launches a collector generation unless one is already running. The
// new generation waits for the previous one to finish before it starts, so
// two collectors never share the status ConfigMap at the same time. Start
// itself returns immediately. Closing stopCh stops the running generation.
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
	// Capture ctx by value while the lock is held. Goroutines must not read
	// c.ctx through the struct pointer once the lock is dropped: a later
	// Start() call writes a new c.ctx under the lock, which would race with
	// a goroutine reading the old value via c.ctx.Done().
	ctx := c.ctx
	start := c.startMethod
	prev := c.lastDone
	done := make(chan struct{})
	c.lastDone = done
	go func() {
		defer close(done)
		if prev != nil {
			<-prev
		}
		start(ctx)
	}()
	go c.handleShutdown(ctx, stopCh)
}

// handleShutdown stops the handler when stopCh closes, and returns without
// stopping anything when ctx is canceled first (for example by Stop).
func (c *CollectorLifecycleHandler) handleShutdown(ctx context.Context, stopCh <-chan struct{}) {
	const stopMsg = "Stopping collector lifecycle handler"
	select {
	case <-stopCh:
		c.logger.Info(stopMsg)
		c.Stop()
	case <-ctx.Done():
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

// startCollectorLite runs the collector for one generation and blocks until
// ctx is canceled. It returns immediately if ctx is already canceled, so a
// generation that was stopped before it was scheduled never touches the
// shared status ConfigMap.
func (c *CollectorLifecycleHandler) startCollectorLite(ctx context.Context) {
	if err := ctx.Err(); err != nil {
		c.logger.Info("Skipping stale collector startup", slog.Any("reason", err))
		return
	}
	c.logger.Info("Starting collector lite")
	err := flow.StartCollector(ctx, c.namespace)
	if err != nil {
		c.logger.Error("error starting collector", slog.Any("error", err.Error()))
		return
	}
}

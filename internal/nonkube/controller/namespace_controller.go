package controller

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/skupperproject/skupper/internal/nonkube/flow"
	"github.com/skupperproject/skupper/pkg/fs"
	"github.com/skupperproject/skupper/pkg/nonkube/api"
)

type NamespaceController struct {
	ns             string
	stopCh         chan struct{}
	logger         *slog.Logger
	flowController *flow.Controller
	watcher        *fs.FileWatcher
}

func NewNamespaceController(namespace string) (*NamespaceController, error) {
	nsw := &NamespaceController{
		ns:     namespace,
		stopCh: make(chan struct{}),
	}
	watcher, err := fs.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("error creating watcher on namespace %s: %v", namespace, err)
	}
	nsw.watcher = watcher
	nsw.logger = slog.New(slog.Default().Handler()).
		With("namespace", namespace).
		With("component", "namespace.watcher")
	return nsw, nil
}

func (w *NamespaceController) Start() {
	routerConfigHandler := NewRouterConfigHandler(w.stopCh, w.ns)
	routerStateHandler := NewRouterStateHandler(w.ns)
	routerConfigHandler.AddCallback(routerStateHandler)
	collectorLifecycleHandler := NewCollectorLifecycleHandler(w.ns)
	routerStateHandler.SetCallback(collectorLifecycleHandler)
	w.watcher.Add(api.GetInternalOutputPath(w.ns, api.RouterConfigPath), routerConfigHandler)
	w.watcher.Add(api.GetInternalOutputPath(w.ns, api.RuntimeSiteStatePath), NewNetworkStatusHandler(w.ns))
	w.watcher.Start(w.stopCh)
	go w.run()
}

func (w *NamespaceController) run() {
	<-w.stopCh
	w.logger.Info("stopped namespace watcher")
	return
}

// collectorControl controls the lifecycle of the collector based on
// where a given namespace is actually initialized or not
func (w *NamespaceController) collectorControl() {
	ctx, cn := context.WithCancel(context.Background())
	defer cn()
	err := flow.StartCollector(ctx, w.ns)
	if err != nil {
		w.logger.Error("error starting flow collector", slog.Any("error", err.Error()))
		return
	}
}
func (w *NamespaceController) Stop() {
	close(w.stopCh)
}

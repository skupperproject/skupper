package controller

import (
	"fmt"
	"log/slog"

	"github.com/skupperproject/skupper/internal/filesystem"
	"github.com/skupperproject/skupper/pkg/nonkube/api"
)

type NamespaceController struct {
	ns      string
	stopCh  chan struct{}
	logger  *slog.Logger
	watcher *filesystem.FileWatcher
	prepare func()
}

func NewNamespaceController(namespace string) (*NamespaceController, error) {
	nsw := &NamespaceController{
		ns:     namespace,
		stopCh: make(chan struct{}),
	}
	watcher, err := filesystem.NewWatcher(slog.String("namespace", namespace))
	if err != nil {
		return nil, fmt.Errorf("error creating watcher on namespace %s: %v", namespace, err)
	}
	nsw.watcher = watcher
	nsw.logger = slog.New(slog.Default().Handler()).
		With("component", "namespace.watcher").
		With("namespace", namespace)
	return nsw, nil
}

func (w *NamespaceController) Start() {
	if w.prepare == nil {
		routerConfigHandler := NewRouterConfigHandler(w.stopCh, w.ns)
		routerStateHandler := NewRouterStateHandler(w.ns)
		routerConfigHandler.AddCallback(routerStateHandler)
		collectorLifecycleHandler := NewCollectorLifecycleHandler(w.ns)
		routerStateHandler.SetCallback(collectorLifecycleHandler)
		w.watcher.Add(api.GetInternalOutputPath(w.ns, api.RouterConfigPath), routerConfigHandler)
		w.watcher.Add(api.GetInternalOutputPath(w.ns, api.RuntimeSiteStatePath), NewNetworkStatusHandler(w.ns))
	} else {
		w.prepare()
	}
	w.watcher.Start(w.stopCh)
	go w.run()
}

func (w *NamespaceController) run() {
	<-w.stopCh
	w.logger.Info("stopped namespace watcher")
	return
}

func (w *NamespaceController) Stop() {
	close(w.stopCh)
}

package controller

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/skupperproject/skupper/internal/nonkube/flow"
	"github.com/skupperproject/skupper/pkg/fs"
	"github.com/skupperproject/skupper/pkg/nonkube/api"
)

type NamespaceWatcher struct {
	ns             string
	stopCh         chan struct{}
	logger         *slog.Logger
	flowController *flow.Controller
	watcher        *fs.FileWatcher
}

func NewNamespaceWatcher(namespace string) (*NamespaceWatcher, error) {
	nsw := &NamespaceWatcher{
		ns:     namespace,
		stopCh: make(chan struct{}),
	}
	watcher, err := fs.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("error creating watcher on namespace %s: %v", namespace, err)
	}
	nsw.watcher = watcher
	nsw.logger = slog.New(slog.Default().Handler()).
		With("component", "namespace.watcher").
		With("namespace", namespace)
	return nsw, nil
}

func (w *NamespaceWatcher) Start() {
	w.watcher.Add(api.GetInternalOutputPath(w.ns, api.InternalBasePath), NewCollectorLifecycleHandler(w.ns))
	w.watcher.Start(w.stopCh)
	go w.run()
}

func (w *NamespaceWatcher) run() {
	<-w.stopCh
	w.logger.Info("stopped namespace watcher")
	return
}

// collectorControl controls the lifecycle of the collector based on
// where a given namespace is actually initialized or not
func (w *NamespaceWatcher) collectorControl() {
	ctx, cn := context.WithCancel(context.Background())
	defer cn()
	err := flow.StartCollector(ctx, w.ns)
	if err != nil {
		w.logger.Error("error starting flow collector: %w", err.Error())
		return
	}
}
func (w *NamespaceWatcher) Stop() {
	close(w.stopCh)
}

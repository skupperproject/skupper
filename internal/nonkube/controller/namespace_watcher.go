package controller

import (
	"log/slog"
	"time"
)

type NamespaceWatcher struct {
	ns     string
	stopCh chan struct{}
	logger *slog.Logger
}

func NewNamespaceWatcher(namespace string) *NamespaceWatcher {
	nsw := &NamespaceWatcher{
		ns:     namespace,
		stopCh: make(chan struct{}),
	}
	nsw.logger = slog.New(slog.Default().Handler()).With("namespace", namespace)
	return nsw
}

func (w *NamespaceWatcher) Start() {
	go w.start()
}

func (w *NamespaceWatcher) start() {
	t := time.NewTicker(2 * time.Second)
	for {
		select {
		case <-t.C:
			w.logger.Info("Tick")
		case <-w.stopCh:
			w.logger.Info("stopped namespace watcher")
			return
		}
	}
}

func (w *NamespaceWatcher) Stop() {
	close(w.stopCh)
}

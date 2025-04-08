package controller

import (
	"fmt"
	"log/slog"
	"os"
	"path"
	"strings"
	"sync"

	"github.com/skupperproject/skupper/pkg/fs"
	"github.com/skupperproject/skupper/pkg/nonkube/api"
)

type NamespacesHandler struct {
	logger     *slog.Logger
	basePath   string
	watcher    *fs.FileWatcher
	namespaces map[string]*NamespaceController
	mutex      sync.Mutex
}

func (n *NamespacesHandler) OnBasePathAdded(basePath string) {
	slog.Info("Adding namespace", slog.String("path", basePath))
}

func NewNamespacesHandler() (*NamespacesHandler, error) {
	var err error
	basePath := api.GetDefaultOutputNamespacesPath()
	basePath = strings.TrimRight(basePath, string(os.PathSeparator))
	nsh := &NamespacesHandler{
		basePath:   basePath,
		namespaces: make(map[string]*NamespaceController),
		logger: slog.Default().
			With("component", "namespaces.handler"),
	}
	nsh.watcher, err = fs.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("failed to create namespaces watcher: %v", err)
	}
	return nsh, nil
}

func (n *NamespacesHandler) Start(stop chan struct{}, wg *sync.WaitGroup) error {
	w := n.watcher
	w.Add(n.basePath, n)
	err := n.loadExistingNamespaces()
	if err != nil {
		return err
	}
	w.Start(stop)
	go n.wait(stop, wg)
	return nil
}

func (n *NamespacesHandler) wait(stop chan struct{}, wg *sync.WaitGroup) {
	<-stop
	slog.Info("Stopping namespaces watcher")
	for _, nsh := range n.namespaces {
		nsh.Stop()
	}
	wg.Done()
}

func (n *NamespacesHandler) loadExistingNamespaces() error {
	entries, err := os.ReadDir(n.basePath)
	if err != nil {
		return fmt.Errorf("failed to read namespaces directory: %w", err)
	}
	for _, entry := range entries {
		if entry.IsDir() {
			n.OnCreate(path.Join(n.basePath, entry.Name()))
		}
	}
	return nil
}

func (n *NamespacesHandler) namespace(name string) (string, bool) {
	ns := name[len(n.basePath):]
	ns = strings.Trim(ns, string(os.PathSeparator))
	stat, err := os.Stat(name)
	if err != nil {
		return ns, false
	}
	return ns, stat.IsDir()
}

func (n *NamespacesHandler) OnCreate(name string) {
	n.mutex.Lock()
	defer n.mutex.Unlock()
	if name == n.basePath {
		slog.Info("Base path created, starting namespaces watcher")
		if err := n.loadExistingNamespaces(); err != nil {
			slog.Info("failed to create watchers for existing namespaces", slog.Any("error", err))
		}
		return
	}
	ns, isDir := n.namespace(name)
	if !isDir {
		slog.Debug("ignoring non-namespace file", slog.Any("name", name))
		return
	}
	if _, ok := n.namespaces[ns]; !ok {
		slog.Info("Starting namespace controller", slog.String("namespace", ns))
		nsc, err := NewNamespaceController(ns)
		if err != nil {
			slog.Error("Unable to start namespace controller",
				slog.String("namespace", ns),
				slog.Any("error", err))
		}
		n.namespaces[ns] = nsc
		nsc.Start()
	}

}

func (n *NamespacesHandler) OnRemove(name string) {
	n.mutex.Lock()
	defer n.mutex.Unlock()
	if n.basePath == name {
		slog.Info("Base namespace path removed, stopping all namespace watchers", slog.String("path", n.basePath))
		return
	}
	ns, _ := n.namespace(name)
	if ns == "" {
		return
	}
	if nsw, ok := n.namespaces[ns]; ok {
		slog.Info("Stopping namespace watcher", slog.Any("namespace", ns))
		nsw.Stop()
		delete(n.namespaces, ns)
	}
}

func (n *NamespacesHandler) Filter(name string) bool {
	stat, err := os.Stat(name)
	if err == nil && stat.IsDir() {
		return true
	}
	return false
}

func (n *NamespacesHandler) OnUpdate(name string) {
}

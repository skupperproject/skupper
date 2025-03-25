package controller

import (
	"fmt"
	"log"
	"os"
	"path"
	"regexp"
	"strings"

	"github.com/skupperproject/skupper/pkg/fs"
	"github.com/skupperproject/skupper/pkg/nonkube/api"
)

type NamespacesHandler struct {
	basePath   string
	watcher    *fs.FileWatcher
	namespaces map[string]*NamespaceWatcher
}

func NewNamespacesHandler() (*NamespacesHandler, error) {
	var err error
	basePath := api.GetHostNamespacesPath()
	basePath = strings.TrimRight(basePath, string(os.PathSeparator))
	nsh := &NamespacesHandler{
		basePath:   basePath,
		namespaces: make(map[string]*NamespaceWatcher),
	}
	nsh.watcher, err = fs.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("failed to create namespaces watcher: %v", err)
	}
	return nsh, nil
}

func (n *NamespacesHandler) Start(stop chan struct{}) error {
	w := n.watcher
	w.Add(n.basePath, n, regexp.MustCompile(`.*`))
	err := n.loadExistingNamespaces()
	if err != nil {
		return err
	}
	w.Start(stop)
	go n.start(stop)
	return nil
}

func (n *NamespacesHandler) start(stop chan struct{}) {
	<-stop
	log.Println("Stopping namespaces watcher")
	for _, nsh := range n.namespaces {
		nsh.Stop()
	}
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
	if name == n.basePath {
		log.Println("Base path created, starting namespaces watcher")
		if err := n.loadExistingNamespaces(); err != nil {
			log.Printf("failed to create watchers for existing namespaces: %v", err)
		}
		return
	}
	ns, isDir := n.namespace(name)
	if !isDir {
		return
	}
	if _, ok := n.namespaces[ns]; !ok {
		log.Println("Start watching namespace", ns)
		nsw := NewNamespaceWatcher(ns)
		n.namespaces[ns] = nsw
		nsw.Start()
	}

}

func (n *NamespacesHandler) OnRemove(name string) {
	if n.basePath == name {
		log.Printf("Base namespace path removed, stopping all namespace watchers: %q", n.basePath)
		return
	}
	ns, _ := n.namespace(name)
	if ns == "" {
		return
	}
	if nsw, ok := n.namespaces[ns]; ok {
		log.Println("Stopping namespace watcher", ns)
		nsw.Stop()
		delete(n.namespaces, ns)
	}
}

func (n *NamespacesHandler) OnUpdate(name string) {
}

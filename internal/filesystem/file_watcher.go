package filesystem

import (
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// FSChangeHandler provides a callback mechanism used by the FileWatcher
// to notify about changes to monitored directory or file.
type FSChangeHandler interface {
	OnBasePathAdded(basePath string)
	OnCreate(string)
	OnUpdate(string)
	OnRemove(string)
	Filter(string) bool
}

type eventTrigger struct {
	operation func(string)
	name      string
}

// FileWatcher uses fsnotify to watch file system changes done to
// files or directories, notifying the respective handlers. It is
// recommended to watch directories over files (you can add filters
// to limit the scope of files to be observed by your handler).
type FileWatcher struct {
	runningLock sync.Mutex
	handlerLock sync.RWMutex
	watcherLock sync.Mutex
	logger      *slog.Logger
	started     bool
	watcher     *fsnotify.Watcher
	refresh     chan bool
	triggerCh   chan eventTrigger
	handlerMap  map[string][]FSChangeHandler
}

func NewWatcher(attrs ...slog.Attr) (*FileWatcher, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	logger := slog.Default().With(slog.String("component", "pkg.fs.FileWatcher"))
	for _, attr := range attrs {
		logger = logger.With(slog.Any(attr.Key, attr.Value))
	}
	return &FileWatcher{
		watcher:    watcher,
		logger:     logger,
		refresh:    make(chan bool),
		triggerCh:  make(chan eventTrigger),
		handlerMap: map[string][]FSChangeHandler{},
	}, nil
}

func (w *FileWatcher) filterHandlers(name string) []FSChangeHandler {
	w.handlerLock.RLock()
	defer w.handlerLock.RUnlock()
	var filteredHandlers []FSChangeHandler

	for baseName, handlers := range w.handlerMap {
		if !strings.HasPrefix(name, baseName) {
			continue
		}
		for _, handler := range handlers {
			if handler.Filter(name) {
				filteredHandlers = append(filteredHandlers, handler)
			}
		}
	}
	return filteredHandlers
}

func (w *FileWatcher) Start(stopCh <-chan struct{}) {
	w.runningLock.Lock()
	defer w.runningLock.Unlock()
	if w.started {
		return
	}
	w.started = true
	go w.monitorPaths(stopCh)
	go w.processEvents(stopCh)
	go w.dispatchTriggers(stopCh)
}

func (w *FileWatcher) processEvents(stopCh <-chan struct{}) {
	for {
		select {
		case event := <-w.watcher.Events:
			handlers := w.filterHandlers(event.Name)
			switch {
			case event.Has(fsnotify.Create):
				for _, handler := range handlers {
					//go handler.OnCreate(event.Name)
					w.logger.Info("OnCreate", slog.String("name", event.Name))
					w.triggerCh <- eventTrigger{
						operation: handler.OnCreate,
						name:      event.Name,
					}
				}
			case event.Has(fsnotify.Write):
				for _, handler := range handlers {
					w.logger.Info("OnUpdate", slog.String("name", event.Name))
					//go handler.OnUpdate(event.Name)
					w.triggerCh <- eventTrigger{
						operation: handler.OnUpdate,
						name:      event.Name,
					}
				}
			case event.Has(fsnotify.Remove):
				for _, handler := range handlers {
					w.logger.Info("OnRemove", slog.String("name", event.Name))
					//go handler.OnRemove(event.Name)
					w.triggerCh <- eventTrigger{
						operation: handler.OnRemove,
						name:      event.Name,
					}
				}
				// if object being watched is removed, watch for it to show up again
				w.handlerLock.RLock()
				if _, ok := w.handlerMap[event.Name]; ok {
					w.refresh <- true
				}
				w.handlerLock.RUnlock()
			}
		case <-stopCh:
			_ = w.watcher.Close()
			w.started = false
			return
		}
	}
}

func (w *FileWatcher) dispatchTriggers(stopCh <-chan struct{}) {
	triggerTimeout := time.Millisecond * 100
	var timeoutTicker *time.Ticker

	for {
		select {
		case event := <-w.triggerCh:
			done := make(chan bool)
			go func() {
				event.operation(event.name)
				close(done)
			}()
			timeoutTicker = time.NewTicker(triggerTimeout)
			select {
			case <-done:
				timeoutTicker.Stop()
				continue
			case <-timeoutTicker.C:
				w.logger.Warn("event trigger timed out",
					slog.String("name", event.name),
					slog.Any("handler", event.operation))
			}
		case <-stopCh:
			return
		}
	}
}

// monitorPaths monitors paths added to the handlers map, adding watchers
// when those paths exist (fsNotify does not accept non-existing paths)
// and removing them from fsNotify, if they no longer exist.
func (w *FileWatcher) monitorPaths(stopCh <-chan struct{}) {
	w.logger.Info("Start monitoring paths")
	interval := time.Second
	ticker := time.NewTicker(interval)
	w.handlerLock.RLock()
	if len(w.handlerMap) > 0 {
		w.manageWatchers()
	}
	w.handlerLock.RUnlock()
	for {
		select {
		case <-w.refresh:
			w.manageWatchers()
		case <-ticker.C:
			w.manageWatchers()
		case <-stopCh:
			w.logger.Info("Stop monitoring paths")
			return
		}
	}
}

func (w *FileWatcher) manageWatchers() {
	w.watcherLock.Lock()
	defer w.watcherLock.Unlock()
	w.handlerLock.RLock()
	defer w.handlerLock.RUnlock()
	w.logger.Debug("entering manageWatchers()")
	for path, handlers := range w.handlerMap {
		stat, err := os.Stat(path)
		if err != nil {
			if !os.IsNotExist(err) {
				w.logger.Error("error verifying monitored path",
					slog.String("path", path),
					slog.String("error", err.Error()))
				continue
			}
			if slices.Contains(w.watcher.WatchList(), path) {
				if err := w.watcher.Remove(path); err != nil {
					w.logger.Error("error removing monitored path",
						slog.String("path", path),
						slog.String("error", err.Error()))
				}
				w.logger.Debug("Monitored path removed",
					slog.String("path", path))
			}
			continue
		}
		if slices.Contains(w.watcher.WatchList(), path) {
			continue
		}
		w.logger.Debug("Monitored path added",
			slog.String("path", path))
		if err = w.watcher.Add(path); err != nil {
			w.logger.Error("error adding monitored path",
				slog.String("path", path),
				slog.String("error", err.Error()))
			continue
		}
		var existingFilesAndDirectories []string
		if stat.IsDir() {
			pathEntries, err := os.ReadDir(path)
			if err != nil {
				w.logger.Error("error reading monitored path",
					slog.String("path", path),
					slog.String("error", err.Error()))
			}
			for _, entry := range pathEntries {
				entryName := filepath.Join(path, entry.Name())
				existingFilesAndDirectories = append(existingFilesAndDirectories, entryName)
			}
		} else {
			existingFilesAndDirectories = append(existingFilesAndDirectories, path)
		}
		for _, handler := range handlers {
			w.triggerCh <- eventTrigger{
				operation: handler.OnBasePathAdded,
				name:      path,
			}
			for _, existingPath := range existingFilesAndDirectories {
				if handler.Filter(existingPath) {
					w.triggerCh <- eventTrigger{
						operation: handler.OnCreate,
						name:      existingPath,
					}
				}
			}
		}
	}
}

func (w *FileWatcher) Add(name string, handler FSChangeHandler) {
	w.runningLock.Lock()
	defer w.runningLock.Unlock()
	w.handlerLock.Lock()
	defer w.handlerLock.Unlock()
	handlers, ok := w.handlerMap[name]
	if !ok {
		w.handlerMap[name] = []FSChangeHandler{handler}
	}
	w.logger.Info("Adding new handler",
		slog.String("path", name))

	w.handlerMap[name] = append(handlers, handler)
	if w.started {
		w.refresh <- true
	}
}

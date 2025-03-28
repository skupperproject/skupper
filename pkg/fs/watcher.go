package fs

import (
	"os"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
)

// FSChangeHandler provides a callback mechanism used by the FileWatcher
// to notify about changes to monitored directory or file.
type FSChangeHandler interface {
	OnAdd(basePath string)
	OnCreate(string)
	OnUpdate(string)
	OnRemove(string)
	Filter(string) bool
}

// FileWatcher uses fsnotify to watch file system changes done to
// files or directories, notifying the respective handlers. It is
// recommended to watch directories over files (you can add filters
// to limit the scope of files to be observed by your handler).
type FileWatcher struct {
	started    bool
	watcher    *fsnotify.Watcher
	handlerMap map[string][]FSChangeHandler
}

func NewWatcher() (*FileWatcher, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	return &FileWatcher{
		watcher:    watcher,
		handlerMap: map[string][]FSChangeHandler{},
	}, nil
}

func (w *FileWatcher) filterHandlers(name string) []FSChangeHandler {
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

func (w *FileWatcher) prepareRemoved(event fsnotify.Event) {
	if !event.Has(fsnotify.Remove) {
		return
	}
	// When the specific event name being watched is removed,
	// stop fsnotify and wait for it to show up again
	if _, ok := w.handlerMap[event.Name]; ok {
		_ = w.watcher.Remove(event.Name)
		w.watchCreated(event.Name)
	}
}

func (w *FileWatcher) Start(stopCh <-chan struct{}) {
	if w.started {
		return
	}
	w.started = true
	go func() {
		for {
			select {
			case event := <-w.watcher.Events:
				handlers := w.filterHandlers(event.Name)
				if len(handlers) == 0 {
					w.prepareRemoved(event)
					continue
				}
				switch {
				case event.Has(fsnotify.Create):
					for _, handler := range handlers {
						handler.OnCreate(event.Name)
					}
				case event.Has(fsnotify.Write):
					for _, handler := range handlers {
						handler.OnUpdate(event.Name)
					}
				case event.Has(fsnotify.Remove):
					for _, handler := range handlers {
						handler.OnRemove(event.Name)
					}
					// if object being watched is removed, watch for it to show up again
					w.prepareRemoved(event)
				}
			case <-stopCh:
				_ = w.watcher.Close()
				return
			}
		}
	}()
}

// watchCreated waits for a file or directory to exist, then it
// start watching the respective resource. It is recommended to
// watch directories and filter the desired files, as watching
// non-existing files directly might lead to missing events.
func (w *FileWatcher) watchCreated(name string) {
	go func() {
		ticker := time.Tick(time.Second)
		for {
			select {
			case <-ticker:
				if err := w.watcher.Add(name); err == nil {
					for _, handler := range w.handlerMap[name] {
						handler.OnAdd(name)
						if handler.Filter(name) {
							handler.OnCreate(name)
						}
					}
					return
				}
			}
		}
	}()
	return
}

func (w *FileWatcher) Add(name string, handler FSChangeHandler) {
	handlers, ok := w.handlerMap[name]
	if !ok {
		w.handlerMap[name] = []FSChangeHandler{handler}
	}
	w.handlerMap[name] = append(handlers, handler)
	if _, err := os.Stat(name); err != nil && os.IsNotExist(err) {
		w.watchCreated(name)
	} else {
		_ = w.watcher.Add(name)
		handler.OnAdd(name)
	}
}

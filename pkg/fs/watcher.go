package fs

import (
	"os"
	"regexp"
	"time"

	"github.com/fsnotify/fsnotify"
)

// FSChangeHandler provides a callback mechanism used by the FileWatcher
// to notify about changes to monitored directory or file.
type FSChangeHandler interface {
	OnCreate(string)
	OnUpdate(string)
	OnRemove(string)
}

// FileWatcher uses fsnotify to watch file system changes done to
// files or directories, notifying the respective handlers. It is
// recommended to watch directories over files (you can add filters
// to limit the scope of files to be observed).
type FileWatcher struct {
	watcher    *fsnotify.Watcher
	handlerMap map[string]FSChangeHandler
	filterMap  map[FSChangeHandler][]*regexp.Regexp
}

func NewWatcher() (*FileWatcher, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	return &FileWatcher{
		watcher:    watcher,
		handlerMap: map[string]FSChangeHandler{},
		filterMap:  map[FSChangeHandler][]*regexp.Regexp{},
	}, nil
}

func (w *FileWatcher) filterHandlers(name string) []FSChangeHandler {
	var handlers []FSChangeHandler

	for _, handler := range w.handlerMap {
		if filters, ok := w.filterMap[handler]; !ok || len(filters) == 0 {
			handlers = append(handlers, handler)
		} else {
			for _, f := range filters {
				if f.MatchString(name) {
					handlers = append(handlers, handler)
					break
				}
			}
		}
	}
	return handlers
}

func (w *FileWatcher) Start(stopCh <-chan struct{}) {
	go func() {
		for {
			select {
			case event := <-w.watcher.Events:
				handlers := w.filterHandlers(event.Name)
				if len(handlers) == 0 {
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
					// object being watched removed, watch for it to show up again
					if handler, ok := w.handlerMap[event.Name]; ok {
						_ = w.watcher.Remove(event.Name)
						w.watchCreated(event.Name, handler)
					}
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
func (w *FileWatcher) watchCreated(name string, handler FSChangeHandler) {
	go func() {
		ticker := time.Tick(time.Second)
		for {
			select {
			case <-ticker:
				if err := w.watcher.Add(name); err == nil {
					handler.OnCreate(name)
					return
				}
			}
		}
	}()
	return
}

func (w *FileWatcher) Add(name string, handler FSChangeHandler, filters ...*regexp.Regexp) {
	w.handlerMap[name] = handler
	w.filterMap[handler] = filters
	if _, err := os.Stat(name); err != nil && os.IsNotExist(err) {
		w.watchCreated(name, handler)
	} else {
		_ = w.watcher.Add(name)
	}
}

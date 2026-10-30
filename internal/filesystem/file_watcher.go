package filesystem

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
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
	scannedLock sync.Mutex
	logger      *slog.Logger
	started     bool
	resyncing   atomic.Bool
	watcher     *fsnotify.Watcher
	errCh       chan error
	refresh     chan bool
	triggerCh   chan eventTrigger
	handlerMap  map[string][]FSChangeHandler
	// scannedPaths tracks files for which manageWatchers() has already
	// dispatched an OnCreate via the initial directory scan, mapped to the
	// file identity (as seen by the scan) used to recognise that same file when
	// the fsnotify Create event for it arrives.
	//
	// addAndScan calls watcher.Add(path) and then os.ReadDir(path). A file
	// created between those two calls is both listed by ReadDir (manageWatchers
	// dispatches OnCreate) and reported by inotify (processEvents would
	// dispatch a second OnCreate). processEvents uses this map to suppress
	// the second one.
	//
	// The decision deliberately does not depend on wall-clock time: any fixed
	// window can be exceeded by a CPU-throttled container or inotify queue
	// pressure, which would reintroduce the duplicate. Instead, a Create event
	// is suppressed only if the path still refers to the same file (same
	// device and inode) that the scan reported. Files that existed before
	// Add() never produce a Create event, so their entries stay until a
	// Remove/Rename clears them; if such a file is replaced by renaming
	// another file over it (an atomic write), only a Create is delivered, and
	// the identity check lets that genuine Create through.
	//
	// scannedLock must be held for the whole Add()+ReadDir()+registration
	// sequence in addAndScan, so processEvents() cannot evaluate a Create
	// event for a path before its scan entry has been registered.
	scannedPaths map[string]os.FileInfo
	// afterWatchAdd, when set, runs between watcher.Add() and the directory
	// scan. It exists so tests can deterministically create the file that
	// triggers the race described on scannedPaths.
	afterWatchAdd func()
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
		watcher: watcher,
		// The watcher's own error channel. Kept as a field so tests can
		// substitute a channel they control, since fsnotify closes this one
		// from a goroutine that shutdown does not synchronize with.
		errCh:        watcher.Errors,
		logger:       logger,
		refresh:      make(chan bool),
		triggerCh:    make(chan eventTrigger),
		handlerMap:   map[string][]FSChangeHandler{},
		scannedPaths: map[string]os.FileInfo{},
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
				// Suppress the fsnotify Create event if manageWatchers already
				// dispatched an OnCreate for this path from its initial
				// directory scan (file created between Add() and ReadDir()).
				if w.consumeScanned(event.Name) {
					w.logger.Debug("suppressing duplicate OnCreate from fsnotify (already scanned)",
						slog.String("name", event.Name))
					break
				}
				for _, handler := range handlers {
					//go handler.OnCreate(event.Name)
					w.logger.Info("OnCreate", slog.String("name", event.Name))
					select {
					case w.triggerCh <- eventTrigger{operation: handler.OnCreate, name: event.Name}:
					case <-stopCh:
						return
					}
				}
			case event.Has(fsnotify.Write):
				for _, handler := range handlers {
					w.logger.Info("OnUpdate", slog.String("name", event.Name))
					//go handler.OnUpdate(event.Name)
					select {
					case w.triggerCh <- eventTrigger{operation: handler.OnUpdate, name: event.Name}:
					case <-stopCh:
						return
					}
				}
			case event.Has(fsnotify.Rename):
				// The path was renamed away, so a later Create for it (e.g. an
				// atomic replace) is a genuine new file and must not be
				// suppressed by a stale scan entry.
				w.scannedLock.Lock()
				delete(w.scannedPaths, event.Name)
				w.scannedLock.Unlock()
			case event.Has(fsnotify.Remove):
				// Forget the scanned entry so a subsequent re-creation of this
				// path is not suppressed.
				w.scannedLock.Lock()
				delete(w.scannedPaths, event.Name)
				w.scannedLock.Unlock()
				for _, handler := range handlers {
					w.logger.Info("OnRemove", slog.String("name", event.Name))
					//go handler.OnRemove(event.Name)
					select {
					case w.triggerCh <- eventTrigger{operation: handler.OnRemove, name: event.Name}:
					case <-stopCh:
						return
					}
				}
				// if object being watched is removed, watch for it to show up again
				w.handlerLock.RLock()
				if _, ok := w.handlerMap[event.Name]; ok {
					select {
					case w.refresh <- true:
					default:
					}
				}
				w.handlerLock.RUnlock()
			}
		case err, ok := <-w.errCh:
			// Drain and log watcher errors so the watcher does not get stuck.
			if !ok {
				// Errors channel closed: the watcher has been closed.
				w.setStarted(false)
				return
			}
			w.logger.Error("file watcher error", slog.String("error", err.Error()))
			if errors.Is(err, fsnotify.ErrEventOverflow) {
				// Events were dropped; resync to recover the missed changes.
				// Run it off the event loop so we keep servicing events, and
				// coalesce overlapping overflows into a single resync.
				if w.resyncing.CompareAndSwap(false, true) {
					go func() {
						defer w.resyncing.Store(false)
						w.resync(stopCh)
					}()
				}
			}
		case <-stopCh:
			w.runningLock.Lock()
			_ = w.watcher.Close()
			w.started = false
			w.runningLock.Unlock()
			return
		}
	}
}

func (w *FileWatcher) setStarted(started bool) {
	w.runningLock.Lock()
	defer w.runningLock.Unlock()
	w.started = started
}

// consumeScanned reports whether an fsnotify Create event for name is a
// duplicate of an OnCreate already dispatched by the initial directory scan,
// removing the scan entry. It is a duplicate only if name still refers to the
// file the scan reported; a different file at the same path (for example one
// renamed over the original) is a genuine new file.
//
// scannedLock is released before the Lstat call. processEvents is the only
// reader of the inotify event channel, so holding the lock across filesystem
// I/O would stall it (and addAndScan) and risk inotify queue overflow.
func (w *FileWatcher) consumeScanned(name string) bool {
	w.scannedLock.Lock()
	scanned, ok := w.scannedPaths[name]
	if ok {
		delete(w.scannedPaths, name)
	}
	w.scannedLock.Unlock()

	if !ok || scanned == nil {
		return ok
	}
	current, err := os.Lstat(name)
	if err != nil {
		// The file is already gone; a Remove event follows.
		return true
	}
	return os.SameFile(scanned, current)
}

// resync re-emits OnCreate for the files currently present under each watched
// path, so handlers can recover after dropped events. It returns early if
// stopCh is closed so it cannot leak on shutdown.
func (w *FileWatcher) resync(stopCh <-chan struct{}) {
	w.handlerLock.RLock()
	defer w.handlerLock.RUnlock()
	for path, handlers := range w.handlerMap {
		stat, err := os.Stat(path)
		if err != nil {
			continue
		}
		var existingFilesAndDirectories []string
		if stat.IsDir() {
			pathEntries, err := os.ReadDir(path)
			if err != nil {
				w.logger.Error("error reading monitored path during resync",
					slog.String("path", path),
					slog.String("error", err.Error()))
				continue
			}
			for _, entry := range pathEntries {
				existingFilesAndDirectories = append(existingFilesAndDirectories, filepath.Join(path, entry.Name()))
			}
		} else {
			existingFilesAndDirectories = append(existingFilesAndDirectories, path)
		}
		for _, handler := range handlers {
			for _, existingPath := range existingFilesAndDirectories {
				if handler.Filter(existingPath) {
					select {
					case w.triggerCh <- eventTrigger{
						operation: handler.OnCreate,
						name:      existingPath,
					}:
					case <-stopCh:
						return
					}
				}
			}
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
	handlersCount := len(w.handlerMap)
	w.handlerLock.RUnlock()
	if handlersCount > 0 {
		w.manageWatchers(stopCh)
	}
	for {
		select {
		case <-w.refresh:
			w.manageWatchers(stopCh)
		case <-ticker.C:
			w.manageWatchers(stopCh)
		case <-stopCh:
			w.logger.Info("Stop monitoring paths")
			return
		}
	}
}

// addAndScan registers path with fsnotify and lists the entries that already
// exist in it. scannedLock is held for the whole sequence and the entries that
// match a handler filter are registered in scannedPaths before it is released,
// so processEvents() either sees them registered or has no pending event for
// them (see scannedPaths).
func (w *FileWatcher) addAndScan(path string, isDir bool, handlers []FSChangeHandler) ([]string, error) {
	w.scannedLock.Lock()
	defer w.scannedLock.Unlock()
	if err := w.watcher.Add(path); err != nil {
		w.logger.Error("error adding monitored path",
			slog.String("path", path),
			slog.String("error", err.Error()))
		return nil, err
	}
	if w.afterWatchAdd != nil {
		w.afterWatchAdd()
	}
	var existing []string
	infos := map[string]os.FileInfo{}
	if isDir {
		pathEntries, err := os.ReadDir(path)
		if err != nil {
			w.logger.Error("error reading monitored path",
				slog.String("path", path),
				slog.String("error", err.Error()))
			return nil, fmt.Errorf("reading directory %s: %w", path, err)
		}
		for _, entry := range pathEntries {
			name := filepath.Join(path, entry.Name())
			existing = append(existing, name)
			// A nil info (entry vanished) is still tracked; see consumeScanned.
			infos[name], _ = entry.Info()
		}
	} else {
		existing = append(existing, path)
		infos[path], _ = os.Lstat(path)
	}
	for _, name := range existing {
		for _, handler := range handlers {
			if handler.Filter(name) {
				w.scannedPaths[name] = infos[name]
				break
			}
		}
	}
	return existing, nil
}

func (w *FileWatcher) manageWatchers(stopCh <-chan struct{}) {
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
		existingFilesAndDirectories, err := w.addAndScan(path, stat.IsDir(), handlers)
		if err != nil {
			continue
		}
		for _, handler := range handlers {
			select {
			case w.triggerCh <- eventTrigger{operation: handler.OnBasePathAdded, name: path}:
			case <-stopCh:
				return
			}
			for _, existingPath := range existingFilesAndDirectories {
				if handler.Filter(existingPath) {
					select {
					case w.triggerCh <- eventTrigger{operation: handler.OnCreate, name: existingPath}:
					case <-stopCh:
						return
					}
				}
			}
		}
	}
}

func (w *FileWatcher) Add(name string, handler FSChangeHandler) {
	w.handlerLock.Lock()
	handlers, ok := w.handlerMap[name]
	if !ok {
		w.handlerMap[name] = []FSChangeHandler{handler}
	}
	w.logger.Info("Adding new handler",
		slog.String("path", name))

	w.handlerMap[name] = append(handlers, handler)
	w.handlerLock.Unlock()
	w.runningLock.Lock()
	started := w.started
	w.runningLock.Unlock()
	if started {
		select {
		case w.refresh <- true:
		default:
		}
	}
}

package filesystem

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"
)

type ovHandler struct {
	w              *fsnotify.Watcher
	reTriggerCount atomic.Int32
	n              atomic.Int32
}

func (h *ovHandler) OnBasePathAdded(string) {}
func (h *ovHandler) OnCreate(name string) {
	if strings.Contains(name, "b-0010.") {
		count := h.reTriggerCount.Add(1)
		if count == 1 {
			// force an overflow error
			h.w.Errors <- fsnotify.ErrEventOverflow
		}
	}
	h.n.Add(1)
	time.Sleep(120 * time.Millisecond)
}
func (h *ovHandler) OnUpdate(string)      {}
func (h *ovHandler) OnRemove(string)      {}
func (h *ovHandler) Filter(s string) bool { return filepath.Ext(s) == ".yaml" }

// The test floods a watched directory with 2000 files while using a deliberately slow handler,
// which overflows the kernel's inotify queue and triggers an fsnotify error that nothing consumes.
// It then writes a single "canary" file and waits 8 seconds, if that file is never reported,
// the watcher has stopped delivering events permanently, and the test fails.
func TestOverflowKillsWatcher(t *testing.T) {
	if testing.Short() {
		t.Skip("floods the kernel event queue and takes ~20s; run without -short")
	}
	dir := t.TempDir()
	w, err := NewWatcher()
	if err != nil {
		t.Fatal(err)
	}
	h := &ovHandler{
		w: w.watcher,
	}
	w.Add(dir, h)
	stop := make(chan struct{})
	w.Start(stop)
	defer close(stop)
	time.Sleep(1500 * time.Millisecond)

	const burst = 2000
	for i := 0; i < burst; i++ {
		if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("b-%04d.yaml", i)), []byte("x"), 0644); err != nil {
			t.Fatalf("writing burst file %d: %v", i, err)
		}
	}
	time.Sleep(10 * time.Second)
	afterBurst := h.n.Load()
	t.Logf("phase 1: OnCreate fired %d / %d", afterBurst, burst)

	before := h.n.Load()
	if err := os.WriteFile(filepath.Join(dir, "canary.yaml"), []byte("x"), 0644); err != nil {
		t.Fatalf("writing canary file: %v", err)
	}
	time.Sleep(8 * time.Second)
	delta := h.n.Load() - before

	t.Logf("phase 2: canary file -> OnCreate delta = %d", delta)
	if delta == 0 {
		t.Errorf("WATCHER IS DEAD: canary create was never reported (phase1 delivered %d/%d)",
			afterBurst, burst)
	}

	if got := h.reTriggerCount.Load(); got < 2 {
		t.Errorf("WATCHER IS DEAD: expected at least 2 events on file b-0010.yaml, got: %d", got)
	}
}

// countHandler records how many OnCreate callbacks each file received.
type countHandler struct {
	mu     sync.Mutex
	counts map[string]int
}

func newCountHandler() *countHandler {
	return &countHandler{counts: map[string]int{}}
}

func (h *countHandler) OnBasePathAdded(string) {}
func (h *countHandler) OnCreate(name string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.counts[filepath.Base(name)]++
}
func (h *countHandler) OnUpdate(string)      {}
func (h *countHandler) OnRemove(string)      {}
func (h *countHandler) Filter(s string) bool { return filepath.Ext(s) == ".yaml" }

func (h *countHandler) count(base string) int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.counts[base]
}

func TestOverflowTriggersResync(t *testing.T) {
	dir := t.TempDir()
	w, err := NewWatcher()
	if err != nil {
		t.Fatal(err)
	}
	h := newCountHandler()
	w.Add(dir, h)

	errs := make(chan error)
	w.errCh = errs

	stop := make(chan struct{})
	w.Start(stop)
	defer close(stop)

	waitFor(t, 5*time.Second, "watch on "+dir+" to be registered", func() bool {
		return slices.Contains(w.watcher.WatchList(), dir)
	})

	const name = "a.yaml"
	if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0644); err != nil {
		t.Fatalf("writing %s: %v", name, err)
	}
	waitFor(t, 5*time.Second, "initial OnCreate for "+name, func() bool {
		return h.count(name) >= 1
	})

	select {
	case errs <- fsnotify.ErrEventOverflow:
	case <-time.After(2 * time.Second):
		t.Fatal("nothing is draining the watcher error channel: the watcher would stall here")
	}

	waitFor(t, 5*time.Second, "resync OnCreate for "+name, func() bool {
		return h.count(name) >= 2
	})
}

func waitFor(t *testing.T, timeout time.Duration, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out after %v waiting for %s", timeout, what)
}

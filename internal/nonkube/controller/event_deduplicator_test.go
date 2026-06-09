package controller

import (
	"log/slog"
	"sync"
	"testing"
	"testing/synctest"
	"time"
)

func TestEventDeduplicator_deduplicates_multiple_events_same_file(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var processedCount int
		var mu sync.Mutex
		var processedFiles []string

		handler := func(filename string) {
			mu.Lock()
			defer mu.Unlock()
			processedCount++
			processedFiles = append(processedFiles, filename)
		}

		logger := slog.Default()
		stopCh := make(chan struct{})
		deduplicator := NewEventDeduplicator(stopCh, handler, logger)
		defer close(stopCh)

		deduplicator.QueueEvent("test.yaml")
		deduplicator.QueueEvent("test.yaml")
		deduplicator.QueueEvent("test.yaml")

		time.Sleep(200 * time.Millisecond)

		mu.Lock()
		defer mu.Unlock()

		if processedCount != 1 {
			t.Errorf("Expected 1 processing, got %d", processedCount)
		}
	})
}

func TestEventDeduplicator_processes_events_for_diferent_files(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {

		var processedCount int
		var mu sync.Mutex
		var processedFiles []string

		handler := func(filename string) {
			mu.Lock()
			defer mu.Unlock()
			processedCount++
			processedFiles = append(processedFiles, filename)
		}

		logger := slog.Default()
		stopCh := make(chan struct{})
		deduplicator := NewEventDeduplicator(stopCh, handler, logger)
		defer close(stopCh)

		deduplicator.QueueEvent("file1.yaml")
		deduplicator.QueueEvent("file2.yaml")
		deduplicator.QueueEvent("file3.yaml")

		time.Sleep(200 * time.Millisecond)

		mu.Lock()
		defer mu.Unlock()

		if processedCount != 3 {
			t.Errorf("Expected 3 processing, got %d", processedCount)
		}

	})
}

func TestEventDeduplicator_resets_timer_new_event_for_same_file(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var processedCount int
		var mu sync.Mutex

		handler := func(filename string) {
			mu.Lock()
			defer mu.Unlock()
			processedCount++
		}

		logger := slog.Default()
		stopCh := make(chan struct{})
		deduplicator := NewEventDeduplicator(stopCh, handler, logger)
		defer close(stopCh)

		deduplicator.QueueEvent("test.yaml")
		time.Sleep(100 * time.Millisecond)

		deduplicator.QueueEvent("test.yaml")
		time.Sleep(100 * time.Millisecond)

		mu.Lock()
		result := processedCount
		mu.Unlock()

		if result != 0 {
			t.Errorf("Expected 0 processing at 200ms, got %d", result)
		}

		time.Sleep(100 * time.Millisecond)

		mu.Lock()
		result = processedCount
		mu.Unlock()

		if result != 1 {
			t.Errorf("Expected 1 processing after full debounce, got %d", result)
		}

	})
}

func TestEventDeduplicator_closes_eventCh_when_stopCh_closed(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var processedCount int
		var mu sync.Mutex

		handler := func(filename string) {
			mu.Lock()
			defer mu.Unlock()
			processedCount++
		}

		logger := slog.Default()
		stopCh := make(chan struct{})
		deduplicator := NewEventDeduplicator(stopCh, handler, logger)

		deduplicator.QueueEvent("test.yaml")

		// trigger namespace controller shutdown
		close(stopCh)

		time.Sleep(50 * time.Millisecond)

		select {
		case _, ok := <-deduplicator.eventCh:
			if ok {
				t.Error("Expected eventCh to be closed, but it's still open")
			}
		case <-time.After(100 * time.Millisecond):
			t.Error("Timeout waiting for eventCh to be closed")
		}
	})
}

func TestEventDeduplicator_no_double_close_panic(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		handler := func(filename string) {}

		logger := slog.Default()
		stopCh := make(chan struct{})
		deduplicator := NewEventDeduplicator(stopCh, handler, logger)

		// Close stopCh (simulating NamespaceController.Stop())
		close(stopCh)
		time.Sleep(50 * time.Millisecond)

		defer func() {
			if r := recover(); r != nil {
				t.Errorf("Stop() caused panic: %v", r)
			}
		}()
		deduplicator.Stop()
	})
}

package controller

import (
	"log/slog"
	"sync"
	"testing"
	"time"
)

func TestEventDeduplicator(t *testing.T) {
	t.Run("deduplicates multiple events for same file", func(t *testing.T) {
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
		defer deduplicator.Stop()

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

	t.Run("processes events for different files independently", func(t *testing.T) {
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
		defer deduplicator.Stop()

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

	t.Run("resets timer on new event for same file", func(t *testing.T) {
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
		defer deduplicator.Stop()

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

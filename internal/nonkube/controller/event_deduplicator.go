package controller

import (
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// EventDeduplicator handles deduplication of OnCreate and OnUpdate.
// It collects events in a time window and processes only the final event after a quiet period.
type EventDeduplicator struct {
	eventCh       chan string
	stopCh        chan struct{}
	done          chan struct{}
	mutex         sync.Mutex
	pendingEvents map[string]*time.Timer
	handler       func(string)
	logger        *slog.Logger
	closeOnce     sync.Once
}

func NewEventDeduplicator(stopCh chan struct{}, handler func(string), logger *slog.Logger) *EventDeduplicator {
	ed := &EventDeduplicator{
		eventCh:       make(chan string, 10),
		stopCh:        stopCh,
		done:          make(chan struct{}),
		pendingEvents: make(map[string]*time.Timer),
		handler:       handler,
		logger:        logger,
	}

	go ed.processEvents()

	return ed
}

func (ed *EventDeduplicator) processEvents() {
	const delay = 150 * time.Millisecond

	for {
		select {
		case <-ed.stopCh:
			ed.logger.Info("Parent channel is closed")
			ed.Stop()
			return

		case <-ed.done:
			return

		case filename := <-ed.eventCh:
			ed.mutex.Lock()

			if timer, exists := ed.pendingEvents[filename]; exists {
				timer.Stop()
			}

			ed.pendingEvents[filename] = time.AfterFunc(delay, func() {
				ed.handleDebouncedEvent(filename)
			})

			ed.mutex.Unlock()
		}
	}
}

// handleDebouncedEvent processes an event after the delay period has passed.
func (ed *EventDeduplicator) handleDebouncedEvent(filename string) {
	ed.mutex.Lock()
	delete(ed.pendingEvents, filename)
	ed.mutex.Unlock()
	ed.handler(filename)
}

func (ed *EventDeduplicator) QueueEvent(filename string) {
	select {
	case ed.eventCh <- filename:
	default:
		ed.logger.Warn(fmt.Sprintf("Event channel full, dropping event for: %s", filename))
	}
}

func (ed *EventDeduplicator) Stop() {
	ed.closeOnce.Do(func() {
		ed.mutex.Lock()
		for _, timer := range ed.pendingEvents {
			timer.Stop()
		}
		ed.mutex.Unlock()

		close(ed.done)
	})
}

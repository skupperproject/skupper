package event

import (
	"fmt"
	"sort"
	"time"
)

const (
	MaxMessagesPerEventType int = 5
)

type Event struct {
	Name          string
	Detail        string
	Occurrence    time.Time
}

type EventCount struct {
	Key            string    `json:"message"`
	Count          int       `json:"count"`
	LastOccurrence time.Time `json:"last_occurrence"`
}

type EventGroup struct {
	Name           string       `json:"name"`
	Total          int          `json:"total"`
	Counts         []EventCount `json:"counts"`
	LastOccurrence time.Time    `json:"last_occurrence"`
}

type EventStore struct {
	events        map[string]EventGroup
	incoming      chan Event
	queries       chan []EventGroup
}

func NewEventStore() *EventStore {
	return &EventStore {
		events: map[string]EventGroup{},
		incoming: make(chan Event),
		queries: make(chan []EventGroup),
	}
}

func (store *EventStore) latest() []EventGroup {
	result := []EventGroup{}
	for _, v:= range store.events {
		result = append(result, v)
	}
	sort.Slice(result, func (i, j int) bool { return result[i].LastOccurrence.After(result[j].LastOccurrence) })
	return result
}

func (store *EventStore) Start(stopCh <-chan struct{}) {
	go store.run(stopCh)
}

func (store *EventStore) run(stopCh <-chan struct{}) {
	for {
		select {
		case e := <-store.incoming:
			current := store.events[e.Name]
			current.Merge(e)
			store.events[e.Name] = current
		case store.queries <- store.latest():
		case <-stopCh:
			return
		}
	}
}

func (store *EventStore) Record(name string, detail string) {
	store.incoming <- Event {
		Name:       name,
		Detail:     detail,
		Occurrence: time.Now(),
	}
}

func (store *EventStore) Recordf(name string, format string, args ...interface{}) {
	store.Record(name, fmt.Sprintf(format, args...))
}

func (store *EventStore) Query() []EventGroup {
	response := <- store.queries
	return response
}

func (e *EventGroup) updateCounts(key string, t time.Time) bool {
	for i, _ := range e.Counts {
		c := &e.Counts[i]
		if c.Key == key {
			c.Count++
			c.LastOccurrence = t
			return true
		}
	}
	return false
}

func (e *EventGroup) Merge(event Event) {
	if e.Name == "" {
		e.Name = event.Name
	}
	e.Total++
	e.LastOccurrence = event.Occurrence
	if !e.updateCounts(event.Detail, event.Occurrence) {
		e.Counts = append(e.Counts, EventCount {
			Key: event.Detail,
			Count: 1,
			LastOccurrence: event.Occurrence,
		})
	}
	sort.Slice(e.Counts, func (i, j int) bool { return e.Counts[i].LastOccurrence.After(e.Counts[j].LastOccurrence) })
	if len(e.Counts) > MaxMessagesPerEventType {
		e.Counts = e.Counts[:MaxMessagesPerEventType]
	}
}

var DefaultStore *EventStore

func StartDefaultEventStore(stopCh <-chan struct{}) {
	DefaultStore = NewEventStore()
	DefaultStore.Start(stopCh)
}

func Record(name string, detail string) {
	DefaultStore.Record(name, detail)
}

func Recordf(name string, format string, args ...interface{}) {
	DefaultStore.Recordf(name, format, args...)
}

func Query() []EventGroup {
	return DefaultStore.Query()
}

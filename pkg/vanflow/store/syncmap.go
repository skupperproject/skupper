package store

import (
	"fmt"
	"sync"
	"time"

	"github.com/skupperproject/skupper/pkg/vanflow"
	"github.com/skupperproject/skupper/pkg/vanflow/encoding"
)

type EventHandlerFuncs struct {
	OnAdd    func(entry Entry)
	OnChange func(prev, curr Entry)
	OnDelete func(entry Entry)
}

type syncMapStore struct {
	mu    sync.RWMutex
	items map[string]Entry

	indexers      map[string]Indexer
	indices       map[string]map[string]keySet
	eventHandlers EventHandlerFuncs
}

type SyncMapStoreConfig struct {
	Indexers map[string]Indexer
	Handlers EventHandlerFuncs
}

func NewSyncMapStore(cfg SyncMapStoreConfig) Interface {
	if cfg.Indexers == nil {
		cfg.Indexers = defaultIndexers()
	}
	return &syncMapStore{
		indexers:      cfg.Indexers,
		eventHandlers: cfg.Handlers,

		items:   make(map[string]Entry),
		indices: make(map[string]map[string]keySet),
	}
}

func (m *syncMapStore) Add(record vanflow.Record, source SourceRef) bool {
	var entry Entry
	var added bool
	var prev Entry
	var sourceAdded bool

	key := record.Identity()
	entry = Entry{
		Metadata: newMetadata(source),
		Record:   record,
	}

	m.mu.Lock()
	if curr, exists := m.items[key]; exists {
		entry = curr
		if curr.Metadata.AddSource(source) {
			prev = curr
			curr.LastUpdate = time.Now()
			m.items[key] = curr
			m.reindex(key, &prev, curr)
			entry = curr
			sourceAdded = true
		}
	} else {
		m.items[key] = entry
		m.reindex(key, nil, entry)
		added = true
	}
	m.mu.Unlock()

	if added && m.eventHandlers.OnAdd != nil {
		m.eventHandlers.OnAdd(entry)
	}
	if sourceAdded && m.eventHandlers.OnChange != nil {
		m.eventHandlers.OnChange(prev, entry)
	}
	return added
}

func (m *syncMapStore) Update(record vanflow.Record) bool {
	prev, next, ok := func() (Entry, Entry, bool) {
		var prev, next Entry
		key := record.Identity()

		m.mu.Lock()
		defer m.mu.Unlock()
		prev, exists := m.items[key]
		if !exists {
			return prev, next, false
		}
		next = prev
		next.LastUpdate = time.Now()
		next.Record = record
		m.items[key] = next
		m.reindex(key, &prev, next)
		return prev, next, true
	}()

	if ok && m.eventHandlers.OnChange != nil {
		m.eventHandlers.OnChange(prev, next)
	}

	return ok
}

func (m *syncMapStore) Get(id string) (Entry, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	entry, ok := m.items[id]
	return entry, ok
}

func (m *syncMapStore) Delete(id string) (Entry, bool) {
	prev, ok := func() (Entry, bool) {
		m.mu.Lock()
		defer m.mu.Unlock()
		curr, exists := m.items[id]
		if !exists {
			return curr, false
		}
		delete(m.items, id)
		m.unindex(id, curr)
		return curr, true
	}()
	if ok && m.eventHandlers.OnDelete != nil {
		m.eventHandlers.OnDelete(prev)
	}
	return prev, ok
}

func (m *syncMapStore) Patch(record vanflow.Record, source SourceRef) {

	key := record.Identity()
	const (
		ok       = 0
		dne      = 1
		noChange = 2
	)
	prev, next, status, err := func() (prev Entry, next Entry, status int, err error) {
		m.mu.Lock()
		defer m.mu.Unlock()
		curr, exists := m.items[key]
		if !exists {
			return prev, next, dne, nil
		}

		currAttrs, err := encoding.Encode(curr.Record)
		if err != nil {
			err = fmt.Errorf("error encoding current record for comparison: %w", err)
			return
		}
		nextAttrs, err := encoding.Encode(record)
		if err != nil {
			err = fmt.Errorf("error encoding incoming record for comparison: %w", err)
			return
		}
		var changed bool
		for nK, nV := range nextAttrs {
			cV, ok := currAttrs[nK]
			if !ok || cV != nV {
				changed = true
				currAttrs[nK] = nV
			}
		}
		if !changed {
			if curr.Metadata.AddSource(source) {
				prev = curr
				next = curr
				next.LastUpdate = time.Now()
				m.items[key] = next
				m.reindex(key, &prev, next)
				return prev, next, ok, nil
			}
			return prev, next, noChange, nil
		}

		curr.Metadata.AddSource(source)
		prev = curr
		next = curr
		patched, err := encoding.Decode(currAttrs)
		next.Record = patched.(vanflow.Record)
		next.LastUpdate = time.Now()
		m.items[key] = next
		m.reindex(key, &prev, next)
		return
	}()

	if err != nil {
		panic(err) // TODO Decide on error handing - shouldn't happen
	}

	switch status {
	case dne:
		m.Add(record, source)
		return
	case noChange:
		return
	}

	if m.eventHandlers.OnChange != nil {
		m.eventHandlers.OnChange(prev, next)
	}
}

func (m *syncMapStore) List() []Entry {
	m.mu.RLock()
	defer m.mu.RUnlock()
	entries := make([]Entry, 0, len(m.items))
	for _, entry := range m.items {
		entries = append(entries, entry)
	}
	return entries
}

func (m *syncMapStore) Index(index string, exemplar Entry) []Entry {
	m.mu.RLock()
	defer m.mu.RUnlock()
	indexer, ok := m.indexers[index]
	if !ok {
		return nil
	}
	idx := m.indices[index]
	indexVals := indexer(exemplar)

	keys := make(keySet)
	for _, indexVal := range indexVals {
		for key := range idx[indexVal] {
			keys.Add(key)
		}
	}
	entries := make([]Entry, 0, len(keys))
	for key := range keys {
		entries = append(entries, m.items[key])
	}
	return entries
}
func (m *syncMapStore) IndexValues(index string) []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	idx := m.indices[index]
	if len(idx) == 0 {
		return nil
	}
	values := make([]string, 0, len(idx))
	for val := range idx {
		values = append(values, val)
	}
	return values
}

func (m *syncMapStore) RemoveSource(source SourceRef) int {
	var deleted []Entry
	var changed []struct {
		prev Entry
		next Entry
	}
	count := 0

	m.mu.Lock()
	indexer := m.indexers[SourceIndex]
	if indexer != nil {
		idx := m.indices[SourceIndex]
		if idx != nil {
			keys := make(keySet)
			for _, indexVal := range indexer(Entry{Metadata: Metadata{Source: source, Sources: []SourceRef{source}}}) {
				for key := range idx[indexVal] {
					keys.Add(key)
				}
			}
			for key := range keys {
				curr, exists := m.items[key]
				if !exists || !curr.Metadata.RemoveSource(source) {
					continue
				}
				count++
				prev := curr
				if len(curr.Metadata.Sources) == 0 {
					delete(m.items, key)
					m.unindex(key, prev)
					deleted = append(deleted, prev)
					continue
				}
				curr.LastUpdate = time.Now()
				m.items[key] = curr
				m.reindex(key, &prev, curr)
				changed = append(changed, struct {
					prev Entry
					next Entry
				}{prev: prev, next: curr})
			}
		}
	}
	m.mu.Unlock()

	for _, entry := range deleted {
		if m.eventHandlers.OnDelete != nil {
			m.eventHandlers.OnDelete(entry)
		}
	}
	for _, update := range changed {
		if m.eventHandlers.OnChange != nil {
			m.eventHandlers.OnChange(update.prev, update.next)
		}
	}
	return count
}

func (m *syncMapStore) Replace(items []Entry) {
	m.mu.Lock()
	defer m.mu.Unlock()

	entries := make(map[string]Entry, len(items))
	for _, item := range items {
		item.Metadata.ensureSources()
		entries[item.Record.Identity()] = item
	}
	m.items = entries

	m.indices = make(map[string]map[string]keySet)
	for key, entry := range m.items {
		m.reindex(key, nil, entry)
	}
}

func (m *syncMapStore) unindex(key string, entry Entry) {
	for name, indexer := range m.indexers {
		indexVals := indexer(entry)

		index := m.indices[name]
		if index == nil {
			continue
		}
		for _, val := range indexVals {
			if set := index[val]; set != nil {
				set.Remove(key)
				if len(set) == 0 {
					delete(index, val)
				}
			}
		}
	}
}

func (m *syncMapStore) reindex(key string, prev *Entry, next Entry) {
	if prev != nil {
		m.unindex(key, *prev)
	}
	for name, indexer := range m.indexers {
		indexVals := indexer(next)

		index := m.indices[name]
		if index == nil {
			index = map[string]keySet{}
			m.indices[name] = index
		}
		for _, indexVal := range indexVals {
			set := index[indexVal]
			if set == nil {
				set = keySet{}
				index[indexVal] = set
			}
			set.Add(key)
		}
	}
}

type Indexer func(Entry) []string
type keySet map[string]struct{}

func (s keySet) Add(id string) {
	s[id] = struct{}{}
}
func (s keySet) Remove(id string) bool {
	_, present := s[id]
	if present {
		delete(s, id)
	}
	return present
}

const (
	SourceIndex = "BySource"
	TypeIndex   = "ByType"
)

func sourceIndexKey(source SourceRef) string {
	return fmt.Sprintf("%s/%s", source.Version, source.ID)
}

func SourceIndexer(e Entry) []string {
	e.Metadata.ensureSources()
	keys := make([]string, 0, len(e.Metadata.Sources))
	for _, source := range e.Metadata.Sources {
		keys = append(keys, sourceIndexKey(source))
	}
	return keys
}
func TypeIndexer(e Entry) []string {
	return []string{e.Record.GetTypeMeta().String()}
}

func defaultIndexers() map[string]Indexer {
	return map[string]Indexer{
		SourceIndex: SourceIndexer,
		TypeIndex:   TypeIndexer,
	}
}

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
	indices      map[string]map[string]keySet
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

		items:    make(map[string]Entry),
		indices: make(map[string]map[string]keySet),
	}
}

func (m *syncMapStore) Add(record vanflow.Record, source SourceRef) bool {

	entry, ok := func() (Entry, bool) {
		key := record.Identity()

		entry := Entry{
			Metadata: Metadata{LastUpdate: time.Now(), Source: source},
			Record:   record,
		}

		m.mu.Lock()
		defer m.mu.Unlock()
		if _, exists := m.items[key]; exists {
			return entry, false
		}
		m.items[key] = entry
		m.reindex(key, nil, entry)
		return entry, true
	}()

	if ok && m.eventHandlers.OnAdd != nil {
		m.eventHandlers.OnAdd(entry)
	}
	return ok
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
			return prev, next, noChange, nil
		}

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

func (m *syncMapStore) Replace(items []Entry) {
	m.mu.Lock()
	defer m.mu.Unlock()

	entries := make(map[string]Entry, len(items))
	for _, item := range items {
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

func SourceIndexer(e Entry) []string {
	return []string{fmt.Sprintf("%s/%s", e.Source.Version, e.Metadata.Source.ID)}
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

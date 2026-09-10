package store

import (
	"time"

	"github.com/skupperproject/skupper/pkg/vanflow"
)

// Entry is the store record container
type Entry struct {
	Metadata
	Record vanflow.Record
}

// Metadata about a record
type Metadata struct {
	LastUpdate time.Time

	// Source is the first source that asserted this record.
	Source SourceRef
	// Sources is the set of sources that have asserted this record.
	Sources []SourceRef
}

func sourceRefEqual(a, b SourceRef) bool {
	return a.ID == b.ID && a.Version == b.Version
}

func (m *Metadata) ensureSources() {
	if len(m.Sources) == 0 && m.Source.ID != "" {
		m.Sources = []SourceRef{m.Source}
	}
}

func (m *Metadata) AddSource(source SourceRef) bool {
	m.ensureSources()
	for _, existing := range m.Sources {
		if sourceRefEqual(existing, source) {
			return false
		}
	}
	m.Sources = append(m.Sources, source)
	if m.Source.ID == "" {
		m.Source = source
	}
	return true
}

func (m *Metadata) RemoveSource(source SourceRef) bool {
	m.ensureSources()
	for i, existing := range m.Sources {
		if sourceRefEqual(existing, source) {
			m.Sources = append(m.Sources[:i], m.Sources[i+1:]...)
			if sourceRefEqual(m.Source, source) {
				if len(m.Sources) > 0 {
					m.Source = m.Sources[0]
				} else {
					m.Source = SourceRef{}
				}
			}
			return true
		}
	}
	return false
}

func newMetadata(source SourceRef) Metadata {
	return Metadata{
		LastUpdate: time.Now(),
		Source:     source,
		Sources:    []SourceRef{source},
	}
}

// SourceRef identifies a record source
type SourceRef struct {
	ID      string
	Version string
}

// Interface to a vanflow record store
type Interface interface {
	Add(record vanflow.Record, source SourceRef) bool
	Update(vanflow.Record) bool
	Get(id string) (Entry, bool)
	Delete(id string) (Entry, bool)

	// Patch will either create a new record or will merge the partial state
	// contained in the record with its stored state
	Patch(record vanflow.Record, source SourceRef)

	List() []Entry
	Index(index string, exemplar Entry) []Entry
	IndexValues(index string) []string

	Replace([]Entry)

	// RemoveSource detaches all records from the given source, deleting any
	// that are no longer asserted by any source.
	RemoveSource(source SourceRef) int
}

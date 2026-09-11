package store

import (
	"slices"
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

	// Sources is the set of sources that have asserted this record.
	Sources []SourceRef
}

func sourceRefEqual(a, b SourceRef) bool {
	return a.ID == b.ID && a.Version == b.Version
}

func (m Metadata) HasSource(source SourceRef) bool {
	for _, existing := range m.Sources {
		if sourceRefEqual(existing, source) {
			return true
		}
	}
	return false
}

func (m *Metadata) AddSource(source SourceRef) bool {
	for _, existing := range m.Sources {
		if sourceRefEqual(existing, source) {
			return false
		}
	}
	m.Sources = append(slices.Clone(m.Sources), source)
	return true
}

func (m *Metadata) RemoveSource(source SourceRef) bool {
	i := slices.IndexFunc(m.Sources, func(existing SourceRef) bool {
		return sourceRefEqual(existing, source)
	})
	if i < 0 {
		return false
	}
	m.Sources = slices.Delete(slices.Clone(m.Sources), i, i+1)
	return true
}

func newMetadata(source SourceRef) Metadata {
	return Metadata{
		LastUpdate: time.Now(),
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

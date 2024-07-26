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

	Source SourceRef
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
}

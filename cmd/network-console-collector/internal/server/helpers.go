package server

import (
	"sort"
	"strings"

	"github.com/skupperproject/skupper/pkg/vanflow"
	"github.com/skupperproject/skupper/pkg/vanflow/store"
)

func fetchAndMap[V vanflow.Record, R any](stor store.Interface, mapping func(V) R, id string) func() (R, bool) {
	return func() (R, bool) {
		var record R
		entry, ok := stor.Get(id)
		if !ok {
			return record, false
		}
		flowRecord, ok := entry.Record.(V)
		if !ok {
			return record, false
		}
		return mapping(flowRecord), true
	}
}

func fetchAndConditionalMap[V vanflow.Record, R any](stor store.Interface, mapping func(V) (R, bool), id string) func() (R, bool) {
	return func() (R, bool) {
		var record R
		entry, ok := stor.Get(id)
		if !ok {
			return record, false
		}
		flowRecord, ok := entry.Record.(V)
		if !ok {
			return record, false
		}
		return mapping(flowRecord)
	}
}

func listByType[T vanflow.Record](stor store.Interface) []store.Entry {
	var r T
	return ordered(stor.Index(store.TypeIndex, store.Entry{Record: r}))
}

func ordered(entries []store.Entry) []store.Entry {
	sort.Slice(entries, func(i, j int) bool {
		return strings.Compare(entries[i].Record.Identity(), entries[j].Record.Identity()) < 0
	})
	return entries
}

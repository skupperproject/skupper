package store

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/skupperproject/skupper/pkg/vanflow"
)

func TestSyncMapStoreDelete(t *testing.T) {
	var deleteEvents []Entry
	stor := NewSyncMapStore(SyncMapStoreConfig{Handlers: EventHandlerFuncs{
		OnAdd:    func(Entry) { t.Errorf("unexpected call to OnAdd") },
		OnChange: func(_, _ Entry) { t.Errorf("unexpected call to OnChange") },
		OnDelete: func(e Entry) { deleteEvents = append(deleteEvents, e) },
	}})

	source := SourceRef{ID: "test", Version: "0"}
	r0 := vanflow.LogRecord{BaseRecord: vanflow.NewBase("0", time.Now()), LogText: ptrTo("txt")}
	initialState := Entry{
		Record: r0,
		Metadata: Metadata{
			Source:     source,
			LastUpdate: time.Now().Add(-1 * time.Minute),
		},
	}
	stor.(*syncMapStore).Replace([]Entry{initialState})

	deleted, ok := stor.Delete("0")
	if !ok {
		t.Error("expected to delete record")
	}
	if !cmp.Equal(deleted, initialState) {
		t.Errorf("expected delete to return previous value: %s", cmp.Diff(deleted, initialState))
	}
}

func TestSyncMapStoreAdd(t *testing.T) {
	var addEvents []Entry
	stor := NewSyncMapStore(SyncMapStoreConfig{Handlers: EventHandlerFuncs{
		OnAdd:    func(e Entry) { addEvents = append(addEvents, e) },
		OnChange: func(_, _ Entry) { t.Errorf("unexpected call to OnChange") },
		OnDelete: func(Entry) { t.Errorf("unexpected call to OnDelete") },
	}})

	recordsToAdd := []vanflow.Record{
		vanflow.LogRecord{BaseRecord: vanflow.NewBase("1")},
		vanflow.LogRecord{BaseRecord: vanflow.NewBase("2")},
		vanflow.LogRecord{BaseRecord: vanflow.NewBase("3")},
	}
	source := SourceRef{ID: "test", Version: "0"}
	for _, record := range recordsToAdd {
		if ok := stor.Add(record, source); !ok {
			t.Errorf("unexpected failure to add record %v", record)
		}
	}
	for _, record := range recordsToAdd {
		if ok := stor.Add(record, source); ok {
			t.Errorf("add record returned ok, but should have failed to re-add record %v", record)
		}
	}

	actual := stor.List()
	expected := make([]Entry, len(recordsToAdd))
	for i := range recordsToAdd {
		expected[i] = Entry{Record: recordsToAdd[i], Metadata: Metadata{Source: source}}
	}

	if !cmp.Equal(actual, expected, ignoreLastUpdateAndOrder...) {
		t.Errorf("store contents do not match expected: %s", cmp.Diff(actual, expected, ignoreLastUpdateAndOrder...))
	}
	if !cmp.Equal(addEvents, expected, ignoreLastUpdateAndOrder...) {
		t.Errorf("entries from OnAdd handler do not match expected: %s", cmp.Diff(addEvents, expected, ignoreLastUpdateAndOrder...))
	}

}

func TestSyncMapStoreUpdate(t *testing.T) {
	source := SourceRef{ID: "test", Version: "0"}
	var updateEvents []Entry
	stor := NewSyncMapStore(SyncMapStoreConfig{Handlers: EventHandlerFuncs{
		OnAdd:    func(Entry) { t.Errorf("unexpected call to OnAdd") },
		OnChange: func(p, n Entry) { updateEvents = append(updateEvents, p, n) },
		OnDelete: func(Entry) { t.Errorf("unexpected call to OnDelete") },
	}})

	r0 := vanflow.LogRecord{BaseRecord: vanflow.NewBase("0")}
	if ok := stor.Update(r0); ok {
		t.Errorf("update record returned ok but should have failed to update new record")
	}

	initialState := Entry{
		Record: r0,
		Metadata: Metadata{
			Source:     source,
			LastUpdate: time.Now().Add(-1 * time.Minute),
		},
	}
	stor.(*syncMapStore).Replace([]Entry{initialState})

	r0.LogText = ptrTo("test")
	if ok := stor.Update(r0); !ok {
		t.Errorf("update record should have returned ok")
	}

	actual, ok := stor.Get("0")
	if !ok {
		t.Fatalf("record not in store")
	}
	if !actual.LastUpdate.After(initialState.LastUpdate) {
		t.Errorf("expected entry.LastUpdate to be updated: delta was %q", actual.LastUpdate.Sub(initialState.LastUpdate))
	}
	if !cmp.Equal(r0, actual.Record) {
		t.Errorf("store contents do not match expected: %s", cmp.Diff(actual.Record, r0))
	}

	expectedChanges := []Entry{
		initialState, // prev
		{Record: r0, Metadata: Metadata{Source: source}}, // next
	}
	if !cmp.Equal(updateEvents, expectedChanges, ignoreLastUpdateAndOrder...) {
		t.Errorf("entries from OnChange handler do not match expected: %s", cmp.Diff(updateEvents, expectedChanges, ignoreLastUpdateAndOrder...))
	}
}

func TestSyncMapStorePatch(t *testing.T) {
	source := SourceRef{ID: "test", Version: "0"}
	var addEvents []Entry
	var updateEvents []Entry
	stor := NewSyncMapStore(SyncMapStoreConfig{Handlers: EventHandlerFuncs{
		OnAdd:    func(e Entry) { addEvents = append(addEvents, e) },
		OnChange: func(p, n Entry) { updateEvents = append(updateEvents, p, n) },
		OnDelete: func(Entry) { t.Errorf("unexpected call to OnDelete") },
	}})

	startTime := time.Now().Truncate(time.Millisecond)
	r0 := vanflow.RouterRecord{BaseRecord: vanflow.NewBase("0", startTime), Parent: ptrTo("site")}
	r0Delta := vanflow.RouterRecord{BaseRecord: vanflow.NewBase("0"), Namespace: ptrTo("ns")}
	r0Expected := vanflow.RouterRecord{BaseRecord: vanflow.NewBase("0", startTime), Parent: ptrTo("site"), Namespace: ptrTo("ns")}

	initialState := Entry{
		Record: r0,
		Metadata: Metadata{
			Source:     source,
			LastUpdate: time.Now().Add(-1 * time.Minute),
		},
	}
	stor.(*syncMapStore).Replace([]Entry{initialState})

	stor.Patch(r0, source)      // no change
	stor.Patch(r0Delta, source) // update r0
	r1 := vanflow.LogRecord{BaseRecord: vanflow.NewBase("1"), LogText: ptrTo("testI")}
	stor.Patch(r1, source) // add r1
	stor.Patch(r1, source) // no change

	actual := stor.List()
	expected := []Entry{
		{Record: r0Expected, Metadata: Metadata{Source: source}},
		{Record: r1, Metadata: Metadata{Source: source}},
	}
	if !cmp.Equal(actual, expected, ignoreLastUpdateAndOrder...) {
		t.Errorf("store contents do not match expected: %s", cmp.Diff(actual, expected, ignoreLastUpdateAndOrder...))
	}

	expectedAdds := []Entry{
		{Record: r1, Metadata: Metadata{Source: source}},
	}
	if !cmp.Equal(addEvents, expectedAdds, ignoreLastUpdateAndOrder...) {
		t.Errorf("entries from OnAdd handler do not match expected: %s", cmp.Diff(addEvents, expectedAdds, ignoreLastUpdateAndOrder...))
	}
	expectedChanges := []Entry{
		{Record: r0, Metadata: Metadata{Source: source}},         // prev
		{Record: r0Expected, Metadata: Metadata{Source: source}}, // next
	}
	if !cmp.Equal(updateEvents, expectedChanges, ignoreLastUpdateAndOrder...) {
		t.Errorf("entries from OnChange handler do not match expected: %s", cmp.Diff(updateEvents, expectedChanges, ignoreLastUpdateAndOrder...))
	}
}
func TestSyncMapStoreIndex(t *testing.T) {
	stor := NewSyncMapStore(SyncMapStoreConfig{})

	var initialState []Entry
	for i := 0; i < 8; i++ {
		source := SourceRef{ID: fmt.Sprint(i)}
		for c := 0; c < 128; c++ {
			initialState = append(initialState, Entry{
				Metadata: Metadata{Source: source},
				Record:   vanflow.LogRecord{BaseRecord: vanflow.NewBase(fmt.Sprintf("%d-%d", i, c))},
			})
		}
	}

	stor.(*syncMapStore).Replace(initialState)

	items := stor.Index(SourceIndex, Entry{Metadata: Metadata{Source: SourceRef{ID: "dne"}}})
	if expected, actual := 0, len(items); expected != actual {
		t.Errorf("expected %d entries for source 'dne' but got %d", expected, actual)
	}

	items = stor.Index(TypeIndex, Entry{Record: vanflow.LogRecord{}})
	if expected, actual := initialState, items; !cmp.Equal(expected, actual, ignoreLastUpdateAndOrder...) {
		t.Errorf("expected type index to return all log records: %s", cmp.Diff(expected, actual, ignoreLastUpdateAndOrder...))
	}

	items = stor.Index("IndexDoesNotExist", Entry{Metadata: Metadata{Source: SourceRef{ID: "1"}}})
	if expected, actual := 0, len(items); expected != actual {
		t.Errorf("expected %d entries for source '1' but got %d", expected, actual)
	}

	items = stor.Index(SourceIndex, Entry{Metadata: Metadata{Source: SourceRef{ID: "7"}}})
	if expected, actual := 128, len(items); expected != actual {
		t.Errorf("expected %d entries for source '7' but got %d", expected, actual)
	}

	_, ok := stor.Delete(items[0].Record.Identity())
	if !ok {
		t.Fatalf("expected delete to succeed")
	}

	items = stor.Index(SourceIndex, Entry{Metadata: Metadata{Source: SourceRef{ID: "7"}}})
	if expected, actual := 127, len(items); expected != actual {
		t.Errorf("expected %d entries for source '7' after deleting one but got %d", expected, actual)
	}

}

func ptrTo[T any](e T) *T {
	return &e
}

var ignoreLastUpdateAndOrder = []cmp.Option{
	cmpopts.IgnoreFields(Metadata{}, "LastUpdate"),
	cmpopts.SortSlices(func(a, b Entry) bool {
		return strings.Compare(a.Record.Identity(), b.Record.Identity()) < 0
	}),
}

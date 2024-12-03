package collector

import (
	"context"
	"io"
	"log/slog"
	"sort"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/skupperproject/skupper/pkg/vanflow"
	"github.com/skupperproject/skupper/pkg/vanflow/store"
	"gotest.tools/v3/poll"
)

func TestAddressManager(t *testing.T) {
	tlog := slog.New(slog.NewTextHandler(io.Discard, nil))

	type testPhase struct {
		Input             []vanflow.Record
		ExpectedAddresses []vanflow.Record
	}
	testCases := []struct {
		Name   string
		Phases []testPhase
	}{
		{
			Name: "simple",
			Phases: []testPhase{
				{
					Input: []vanflow.Record{ // incomplete
						vanflow.ConnectorRecord{
							BaseRecord: vanflow.NewBase("c1"), Address: nil,
						},
					},
					ExpectedAddresses: []vanflow.Record{}, //none
				}, {
					Input: []vanflow.Record{ // now complete
						vanflow.ConnectorRecord{
							BaseRecord: vanflow.NewBase("c1"),
							Address:    ptrTo("bingo"), Protocol: ptrTo("tcp"),
						},
						vanflow.ConnectorRecord{ // multiple
							BaseRecord: vanflow.NewBase("c2"),
							Address:    ptrTo("bingo"), Protocol: ptrTo("tcp"),
						},
					},
					ExpectedAddresses: []vanflow.Record{
						AddressRecord{Name: "bingo", Protocol: "tcp"},
					},
				}, {
					Input:             []vanflow.Record{}, // empty
					ExpectedAddresses: []vanflow.Record{}, // empty
				},
			},
		},
		{
			Name: "listeners",
			Phases: []testPhase{
				{
					Input: []vanflow.Record{ // now complete
						vanflow.ListenerRecord{
							BaseRecord: vanflow.NewBase("l1"),
							Address:    ptrTo("bingo"), Protocol: ptrTo("tcp"),
						},
						vanflow.ListenerRecord{
							BaseRecord: vanflow.NewBase("l2"),
							Address:    ptrTo("bingo"), Protocol: ptrTo("yodel"),
						},
						vanflow.ListenerRecord{
							BaseRecord: vanflow.NewBase("l3"),
							Address:    ptrTo("poker"), Protocol: ptrTo("tcp"),
						},
						vanflow.ListenerRecord{
							BaseRecord: vanflow.NewBase("l4"),
							Address:    ptrTo("chess"), Protocol: ptrTo("tcp"),
						},
					},
					ExpectedAddresses: []vanflow.Record{
						AddressRecord{Name: "bingo", Protocol: "tcp"},
						AddressRecord{Name: "bingo", Protocol: "yodel"},
						AddressRecord{Name: "poker", Protocol: "tcp"},
						AddressRecord{Name: "chess", Protocol: "tcp"},
					},
				}, {
					Input:             []vanflow.Record{}, // empty
					ExpectedAddresses: []vanflow.Record{}, // empty
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			stor := store.NewSyncMapStore(store.SyncMapStoreConfig{
				Indexers: RecordIndexers(),
			})

			testCtx, cancel := context.WithCancel(context.Background())
			defer cancel()
			manager := newAddressManager(tlog, stor)
			go manager.run(testCtx)()

			for _, phase := range tc.Phases {
				stor.Replace(wrapRecords(phase.Input...))
				for _, e := range stor.List() {
					manager.handleChangeEvent(addEvent{Record: e.Record}, stor)
				}
				ordered := func(addresses []store.Entry) {
					sort.Slice(addresses, func(i, j int) bool {
						left := addresses[i].Record.(AddressRecord)
						right := addresses[j].Record.(AddressRecord)
						return left.Name+left.Protocol < right.Name+right.Protocol
					})

				}
				expected := wrapRecords(phase.ExpectedAddresses...)
				ordered(expected)
				poll.WaitOn(t, func(t poll.LogT) poll.Result {
					addresses := stor.Index(store.TypeIndex, store.Entry{Record: AddressRecord{}})
					ordered(addresses)
					diff := cmp.Diff(
						addresses,
						expected,
						cmpopts.IgnoreFields(AddressRecord{}, "ID"),
						cmpopts.IgnoreFields(AddressRecord{}, "Start"),
						cmpopts.IgnoreFields(store.Entry{}, "Metadata"),
					)
					if diff == "" {
						return poll.Success()
					}
					return poll.Continue("diff: %s", diff)
				})

				for _, r := range phase.Input {
					stor.Delete(r.Identity())
					manager.handleChangeEvent(deleteEvent{Record: r}, stor)
				}
			}
		})
	}

}

func wrapRecords(records ...vanflow.Record) []store.Entry {
	entries := make([]store.Entry, len(records))
	for i := range records {
		entries[i].Record = records[i]
	}
	return entries
}

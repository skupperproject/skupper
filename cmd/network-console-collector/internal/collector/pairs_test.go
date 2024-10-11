package collector

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/skupperproject/skupper/pkg/vanflow"
	"github.com/skupperproject/skupper/pkg/vanflow/store"
	"gotest.tools/poll"
)

func TestPairManagerSitePairs(t *testing.T) {
	tlog := slog.New(slog.NewTextHandler(io.Discard, nil))
	stor := store.NewSyncMapStore(store.SyncMapStoreConfig{
		Indexers: RecordIndexers(),
	})
	graph := NewGraph(stor).(*graph)
	metrics := register(prometheus.NewRegistry())

	source := store.SourceRef{ID: "test"}
	sites := []vanflow.SiteRecord{
		{BaseRecord: vanflow.NewBase("s1")},
		{BaseRecord: vanflow.NewBase("s2")},
		{BaseRecord: vanflow.NewBase("s3")},
	}
	processes := []vanflow.ProcessRecord{
		{BaseRecord: vanflow.NewBase("p1"), Parent: ptrTo("s1")},

		{BaseRecord: vanflow.NewBase("p2"), Parent: ptrTo("s2")},
		{BaseRecord: vanflow.NewBase("p3"), Parent: ptrTo("s2")},

		{BaseRecord: vanflow.NewBase("p4"), Parent: ptrTo("s3")},
		{BaseRecord: vanflow.NewBase("p5"), Parent: ptrTo("s3")},
	}
	procPairs := []struct {
		Pair           ProcPairRecord
		ExpectedSource string
		ExpectedDest   string
	}{
		{
			Pair:           ProcPairRecord{ID: "pp1", Source: "p1", Dest: "p2", Protocol: "test"},
			ExpectedSource: "s1", ExpectedDest: "s2",
		}, {
			Pair:           ProcPairRecord{ID: "pp2", Source: "p2", Dest: "p3", Protocol: "test"},
			ExpectedSource: "s2", ExpectedDest: "s2",
		}, {
			Pair:           ProcPairRecord{ID: "pp3", Source: "p2", Dest: "p1", Protocol: "test"},
			ExpectedSource: "s2", ExpectedDest: "s1",
		}, {
			Pair:           ProcPairRecord{ID: "pp1", Source: "p1", Dest: "p2", Protocol: "test"},
			ExpectedSource: "s1", ExpectedDest: "s2",
		}, {
			Pair:           ProcPairRecord{ID: "pp4", Source: "p2", Dest: "p5", Protocol: "test"},
			ExpectedSource: "s2", ExpectedDest: "s3",
		},
	}
	for _, site := range sites {
		stor.Add(site, source)
	}
	for _, proc := range processes {
		stor.Add(proc, source)
	}
	for _, pp := range procPairs {
		stor.Add(pp.Pair, source)
	}

	graph.Reset()

	manager := newPairManager(tlog, stor, graph, metrics)
	go manager.run(context.TODO())()
	for _, procPair := range procPairs {
		manager.handleChangeEvent(addEvent{Record: procPair.Pair}, stor)
		poll.WaitOn(t, func(t poll.LogT) poll.Result {
			for _, entry := range stor.Index(store.TypeIndex, store.Entry{Record: SitePairRecord{}}) {
				actual := entry.Record.(SitePairRecord)
				if actual.Protocol == "test" &&
					actual.Source == procPair.ExpectedSource &&
					actual.Dest == procPair.ExpectedDest {
					return poll.Success()
				}
			}
			return poll.Continue("missing site pair")
		}, poll.WithDelay(time.Microsecond*500))
	}
	// re-fire events many times - should be a noop
	for i := 0; i < 10_000; i++ {
		for _, procPair := range procPairs {
			manager.handleChangeEvent(addEvent{Record: procPair.Pair}, stor)
		}
	}
}

func ptrTo[T any](obj T) *T { return &obj }

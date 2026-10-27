package server

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/skupperproject/skupper/cmd/network-observer/internal/collector"
	"github.com/skupperproject/skupper/pkg/vanflow"
	"github.com/skupperproject/skupper/pkg/vanflow/store"
	"gotest.tools/v3/assert"
)

func TestListenerRoutingKeyCounts(t *testing.T) {
	stor := store.NewSyncMapStore(store.SyncMapStoreConfig{Indexers: collector.RecordIndexers()})
	graph := collector.NewGraph(stor)
	srv, client := requireTestClient(t, New(slog.Default(), stor, graph))
	defer srv.Close()
	now := time.Now()
	listener := func(id, router, name, key string) vanflow.ListenerRecord {
		return vanflow.ListenerRecord{
			BaseRecord:    vanflow.NewBase(id, now),
			Parent:        ptrTo(router),
			Name:          ptrTo(name),
			Address:       ptrTo(key),
			Protocol:      ptrTo("tcp"),
			StrategyName:  ptrTo("weighted"),
			StrategyValue: ptrTo(uint64(0)),
		}
	}
	second := listener("second", "r1", "alpha", "b")
	second.EndTime = ptrTo(vanflow.Time{Time: time.Unix(0, 0)})
	removed := listener("removed", "r1", "alpha", "c")
	removed.EndTime = ptrTo(vanflow.Time{Time: now})
	ended := listener("ended", "r1", "gone", "d")
	ended.EndTime = ptrTo(vanflow.Time{Time: now})
	single := listener("single", "r1", "alpha", "e")
	single.StrategyName, single.StrategyValue = nil, nil
	stor.Replace(wrapRecords(
		vanflow.RouterRecord{BaseRecord: vanflow.NewBase("r1", now)},
		vanflow.RouterRecord{BaseRecord: vanflow.NewBase("r2", now)},
		listener("first", "r1", "alpha", "a"),
		second,
		listener("duplicate", "r1", "alpha", "a"),
		removed,
		listener("other-name", "r1", "beta", "f"),
		listener("other-router", "r2", "alpha", "g"),
		ended,
		single,
	))
	graph.(reset).Reset()

	for _, phase := range []struct {
		name  string
		count int
	}{
		{name: "initial", count: 2},
		{name: "after removal", count: 1},
	} {
		t.Run(phase.name, func(t *testing.T) {
			if phase.name == "after removal" {
				second.EndTime = ptrTo(vanflow.Time{Time: now})
				assert.Assert(t, stor.Update(second))
			}
			expected := map[string]int{
				"first": phase.count, "second": phase.count, "duplicate": phase.count,
				"removed": phase.count, "other-name": 1, "other-router": 1, "ended": 0, "single": 1,
			}
			response, err := client.ListenersWithResponse(context.Background())
			assert.NilError(t, err)
			assert.Equal(t, response.StatusCode(), 200)
			assert.Assert(t, response.JSON200 != nil)
			assert.Equal(t, len(response.JSON200.Results), len(expected))
			counts := map[string]int{}
			for _, result := range response.JSON200.Results {
				counts[result.Identity] = result.RoutingKeyCount
			}
			assert.DeepEqual(t, counts, expected)
			for _, id := range []string{"first", "ended"} {
				detail, err := client.ListenerByIDWithResponse(context.Background(), id)
				assert.NilError(t, err)
				assert.Equal(t, detail.StatusCode(), 200)
				assert.Assert(t, detail.JSON200 != nil)
				assert.Equal(t, detail.JSON200.Results.RoutingKeyCount, expected[id])
			}
		})
	}
}

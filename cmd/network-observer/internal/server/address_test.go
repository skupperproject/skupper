package server

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/skupperproject/skupper/cmd/network-observer/internal/api"
	"github.com/skupperproject/skupper/cmd/network-observer/internal/collector"
	"github.com/skupperproject/skupper/pkg/vanflow"
	"github.com/skupperproject/skupper/pkg/vanflow/store"
	"gotest.tools/v3/assert"
)

func TestAddresses(t *testing.T) {
	tlog := slog.Default()
	stor := store.NewSyncMapStore(store.SyncMapStoreConfig{Indexers: collector.RecordIndexers()})
	graph := collector.NewGraph(stor)
	srv, c := requireTestClient(t, New(tlog, stor, graph))
	defer srv.Close()

	begin := time.Now()
	testcases := []struct {
		Records              []store.Entry
		Parameters           map[string][]string
		ExpectOK             bool
		ExpectCount          int
		ExpectTimeRangeCount int
		ExpectResults        func(t *testing.T, results []api.ServiceRecord)
	}{
		{ExpectOK: true},
		{
			Records: wrapRecords(
				collector.AddressRecord{ID: "addr-1", Name: "pizza", Protocol: "tcp", Start: begin},
				collector.AddressRecord{ID: "addr-2", Name: "icecream", Protocol: "tcp", Start: begin},
				collector.AddressRecord{ID: "addr-3", Name: "waffles", Protocol: "tcp", Start: begin},
				vanflow.ConnectorRecord{BaseRecord: vanflow.NewBase("c1"), Address: ptrTo("pizza"), Protocol: ptrTo("tcp")},
				vanflow.ConnectorRecord{BaseRecord: vanflow.NewBase("c2"), Address: ptrTo("pizza"), Protocol: ptrTo("tcp")},
				vanflow.ListenerRecord{BaseRecord: vanflow.NewBase("l1"), Address: ptrTo("pizza"), Protocol: ptrTo("tcp")},
				vanflow.ListenerRecord{BaseRecord: vanflow.NewBase("l2"), Address: ptrTo("pizza"), Protocol: ptrTo("tcp")},
				collector.RequestRecord{RoutingKey: "pizza", ID: "r1", Protocol: "yodel"},
				collector.RequestRecord{RoutingKey: "pizza", ID: "r2", Protocol: "yodel"},
			),
			ExpectOK:    true,
			ExpectCount: 3,
			ExpectResults: func(t *testing.T, results []api.ServiceRecord) {
				for _, result := range results {
					if result.Identity == "addr-1" {
						assert.DeepEqual(t, result, api.ServiceRecord{
							Identity:                     "addr-1",
							Name:                         "pizza",
							ConnectorCount:               2,
							ListenerCount:                2,
							HasListener:                  true,
							IsBound:                      true,
							Protocol:                     "tcp",
							ObservedApplicationProtocols: []string{"yodel"},
							StartTime:                    uint64(begin.UnixMicro()),
						})
						return
					}
				}
				t.Error("expected to find address record for addr-1")
			},
		},
		{
			Records: wrapRecords(
				collector.AddressRecord{ID: "addr-1", Name: "pizza", Protocol: "tcp", Start: begin},
				collector.AddressRecord{ID: "addr-2", Name: "icecream", Protocol: "tcp", Start: begin},
				collector.AddressRecord{ID: "addr-3", Name: "waffles", Protocol: "tcp", Start: begin},
				vanflow.ConnectorRecord{BaseRecord: vanflow.NewBase("c1"), Address: ptrTo("pizza"), Protocol: ptrTo("tcp")},
				vanflow.ListenerRecord{BaseRecord: vanflow.NewBase("l1"), Address: ptrTo("pizza"), Protocol: ptrTo("tcp")},
			),
			Parameters: map[string][]string{
				"sortBy": {"isBound.asc"},
			},
			ExpectOK:    true,
			ExpectCount: 3,
			ExpectResults: func(t *testing.T, results []api.ServiceRecord) {
				assert.Equal(t, results[2].Identity, "addr-1", "Expected addr-1 to be last with sortBy isBound.asc")
				assert.Equal(t, results[2].IsBound, true, "Expected addr-1 to be bound")
			},
		},
		{
			Records: wrapRecords(
				collector.AddressRecord{ID: "addr-1", Name: "pizza", Protocol: "tcp", Start: begin},
				collector.AddressRecord{ID: "addr-2", Name: "icecream", Protocol: "tcp", Start: begin},
				collector.AddressRecord{ID: "addr-3", Name: "waffles", Protocol: "tcp", Start: begin},
				vanflow.ConnectorRecord{BaseRecord: vanflow.NewBase("c1"), Address: ptrTo("pizza"), Protocol: ptrTo("tcp")},
				vanflow.ListenerRecord{BaseRecord: vanflow.NewBase("l1"), Address: ptrTo("pizza"), Protocol: ptrTo("tcp")},
			),
			Parameters: map[string][]string{
				"sortBy": {"isBound.desc"},
			},
			ExpectOK:    true,
			ExpectCount: 3,
			ExpectResults: func(t *testing.T, results []api.ServiceRecord) {
				assert.Equal(t, results[0].Identity, "addr-1", "Expected addr-1 to be first with sortBy isBound.asc")
				assert.Equal(t, results[0].IsBound, true, "Expected addr-1 to be bound")
			},
		},
	}

	for _, tc := range testcases {
		t.Run("", func(t *testing.T) {
			stor.Replace(tc.Records)
			graph.(reset).Reset()
			for _, r := range tc.Records {
				graph.(reset).Reindex(r.Record)
			}
			resp, err := c.ServicesWithResponse(context.TODO(), withParameters(tc.Parameters))
			assert.Check(t, err)
			if tc.ExpectOK {
				assert.Equal(t, resp.StatusCode(), 200)
				assert.Equal(t, resp.JSON200.Count, int64(tc.ExpectCount))
				assert.Assert(t, resp.JSON200.Results != nil)
				assert.Equal(t, len(resp.JSON200.Results), tc.ExpectCount)
				if tc.ExpectTimeRangeCount != 0 {
					assert.Equal(t, resp.JSON200.TimeRangeCount, int64(tc.ExpectTimeRangeCount))
				}
				if tc.ExpectResults != nil {
					tc.ExpectResults(t, resp.JSON200.Results)
				}
			} else {
				assert.Check(t, resp.JSON400 != nil)
			}
		})
	}
}

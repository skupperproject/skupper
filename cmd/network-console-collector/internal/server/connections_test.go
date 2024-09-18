package server

import (
	"context"
	"fmt"
	"log/slog"
	"testing"
	"time"

	"github.com/skupperproject/skupper/cmd/network-console-collector/internal/api"
	"github.com/skupperproject/skupper/cmd/network-console-collector/internal/collector"
	"github.com/skupperproject/skupper/pkg/vanflow"
	"github.com/skupperproject/skupper/pkg/vanflow/store"
	"gotest.tools/assert"
)

func TestConnections(t *testing.T) {
	tlog := slog.Default()
	stor := store.NewSyncMapStore(store.SyncMapStoreConfig{Indexers: collector.RecordIndexers()})
	flowStor := store.NewSyncMapStore(store.SyncMapStoreConfig{Indexers: collector.RecordIndexers()})
	graph := collector.NewGraph(stor)
	srv, c := requireTestClient(t, New(tlog, stor, graph))
	defer srv.Close()

	van := []vanflow.Record{
		vanflow.SiteRecord{BaseRecord: vanflow.NewBase("site-a"), Name: ptrTo("site a")},
		vanflow.RouterRecord{BaseRecord: vanflow.NewBase("router-a-1"), Name: ptrTo("router a.1"), Parent: ptrTo("site-a")},

		vanflow.SiteRecord{BaseRecord: vanflow.NewBase("site-b"), Name: ptrTo("site b")},
		vanflow.RouterRecord{BaseRecord: vanflow.NewBase("router-b-1"), Name: ptrTo("router b.1"), Parent: ptrTo("site-b")},
		vanflow.RouterRecord{BaseRecord: vanflow.NewBase("router-b-2"), Name: ptrTo("router b.2"), Parent: ptrTo("site-b")},
	}

	t0 := time.Now().Add(-2 * time.Hour)
	timeline := [5]time.Time{
		t0,
		t0.Add(1 * time.Minute),
		t0.Add(2 * time.Minute),
		t0.Add(3 * time.Minute),
		t0.Add(4 * time.Minute),
	}

	testcases := []struct {
		Records              []store.Entry
		Flows                []store.Entry
		Parameters           map[string][]string
		ExpectOK             bool
		ExpectCount          int
		ExpectTimeRangeCount int
		ExpectResults        func(t *testing.T, results []api.ConnectionRecord)
	}{
		{ExpectOK: true},
		{
			Records: wrapRecords(
				append(van,
					collector.ConnectionRecord{
						ID:           "flow:1",
						Source:       collector.NamedReference{ID: "p1", Name: "p:1"},
						Dest:         collector.NamedReference{ID: "p2", Name: "p:2"},
						SourceSite:   collector.NamedReference{ID: "site-a", Name: "site a"},
						DestSite:     collector.NamedReference{ID: "site-b", Name: "site b"},
						SourceRouter: collector.NamedReference{ID: "router-a-1", Name: "router a.1"},
						DestRouter:   collector.NamedReference{ID: "router-b-2", Name: "router b.2"},
						Protocol:     "tcp",
						RoutingKey:   "soup",
						FlowStore:    flowStor,
					},
				)...,
			),
			Flows: wrapRecords(
				vanflow.TransportBiflowRecord{
					BaseRecord: vanflow.NewBase("flow:1"),
					Trace:      ptrTo("router a.1|router b.1|router b.2"),
					Octets:     ptrTo(uint64(33)),
					SourceHost: ptrTo("source.local"),
				},
			),
			ExpectOK:    true,
			ExpectCount: 1,
			ExpectResults: func(t *testing.T, results []api.ConnectionRecord) {
				r := results[0]
				assert.DeepEqual(t, r, api.ConnectionRecord{
					Identity:          "flow:1",
					RoutingKey:        "soup",
					SourceHost:        "source.local",
					SourceProcessId:   "p1",
					DestProcessId:     "p2",
					SourceProcessName: "p:1",
					DestProcessName:   "p:2",
					SourceSiteName:    "site a",
					DestSiteName:      "site b",
					SourceSiteId:      "site-a",
					DestSiteId:        "site-b",
					Protocol:          "tcp",

					Octets:       33,
					TraceRouters: []string{"router a.1", "router b.1", "router b.2"},
					TraceSites:   []string{"site a", "site b"},
				})
			},
		}, {
			Records: wrapRecords(
				collector.ConnectionRecord{ID: "flow:1", FlowStore: flowStor},
				collector.ConnectionRecord{ID: "flow:2", FlowStore: flowStor},
				collector.ConnectionRecord{ID: "flow:3", FlowStore: flowStor},
				collector.ConnectionRecord{ID: "flow:4", FlowStore: flowStor},
				collector.ConnectionRecord{ID: "flow:5", FlowStore: flowStor},
			),
			Parameters: map[string][]string{"timeRangeStart": {fmt.Sprint(t0.UnixMicro())}},
			Flows: wrapRecords(
				// terminated flows
				vanflow.TransportBiflowRecord{BaseRecord: vanflow.NewBase("flow:1", timeline[0], timeline[1])},
				vanflow.TransportBiflowRecord{BaseRecord: vanflow.NewBase("flow:2", timeline[0], timeline[2])},
				// active flows
				vanflow.TransportBiflowRecord{BaseRecord: vanflow.NewBase("flow:3", timeline[3])},
				vanflow.TransportBiflowRecord{BaseRecord: vanflow.NewBase("flow:4", timeline[4])},
				vanflow.TransportBiflowRecord{BaseRecord: vanflow.NewBase("flow:5", timeline[0])},
				// no connection record
				vanflow.TransportBiflowRecord{BaseRecord: vanflow.NewBase("flow:6", timeline[4])},
			),
			ExpectOK:             true,
			ExpectCount:          5,
			ExpectTimeRangeCount: 5,
		}, {
			Records: wrapRecords(
				collector.ConnectionRecord{ID: "flow:1", FlowStore: flowStor},
				collector.ConnectionRecord{ID: "flow:2", FlowStore: flowStor},
				collector.ConnectionRecord{ID: "flow:3", FlowStore: flowStor},
				collector.ConnectionRecord{ID: "flow:4", FlowStore: flowStor},
				collector.ConnectionRecord{ID: "flow:5", FlowStore: flowStor},
			),
			Flows: wrapRecords(
				// terminated flows
				vanflow.TransportBiflowRecord{BaseRecord: vanflow.NewBase("flow:1", timeline[0], timeline[1])},
				vanflow.TransportBiflowRecord{BaseRecord: vanflow.NewBase("flow:2", timeline[0], timeline[2])},
				// active flows
				vanflow.TransportBiflowRecord{BaseRecord: vanflow.NewBase("flow:3", timeline[3])},
				vanflow.TransportBiflowRecord{BaseRecord: vanflow.NewBase("flow:4", timeline[4])},
				vanflow.TransportBiflowRecord{BaseRecord: vanflow.NewBase("flow:5", timeline[0])},
				// no connection record
				vanflow.TransportBiflowRecord{BaseRecord: vanflow.NewBase("flow:6", timeline[4])},
			),
			Parameters: map[string][]string{
				"state":          {"active"},
				"timeRangeStart": {fmt.Sprint(t0.UnixMicro())},
				"limit":          {"0"},
			},
			ExpectOK:             true,
			ExpectCount:          0,
			ExpectTimeRangeCount: 3,
		},
	}

	for _, tc := range testcases {
		t.Run("", func(t *testing.T) {
			stor.Replace(tc.Records)
			flowStor.Replace(tc.Flows)
			graph.(reset).Reset()
			for _, r := range tc.Records {
				graph.(reset).Reindex(r.Record)
			}
			resp, err := c.ConnectionsWithResponse(context.TODO(), WithParameters(tc.Parameters))
			assert.Check(t, err)
			if tc.ExpectOK {
				assert.Equal(t, resp.StatusCode(), 200)
				assert.Equal(t, resp.JSON200.Count, int64(tc.ExpectCount))
				assert.Equal(t, len(resp.JSON200.Results), tc.ExpectCount)
				if tc.ExpectTimeRangeCount != 0 {
					assert.Equal(t, resp.JSON200.TimeRangeCount, int64(tc.ExpectTimeRangeCount))
				}
				if tc.ExpectResults != nil {
					tc.ExpectResults(t, resp.JSON200.Results)
				}
			}
		})
	}
}

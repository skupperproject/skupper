package server

import (
	"context"
	"log/slog"
	"testing"

	"github.com/skupperproject/skupper/cmd/network-observer/internal/api"
	"github.com/skupperproject/skupper/cmd/network-observer/internal/collector"
	"github.com/skupperproject/skupper/pkg/vanflow"
	"github.com/skupperproject/skupper/pkg/vanflow/store"
	"gotest.tools/v3/assert"
)

func TestConnectors(t *testing.T) {
	tlog := slog.Default()
	stor := store.NewSyncMapStore(store.SyncMapStoreConfig{Indexers: collector.RecordIndexers()})
	graph := collector.NewGraph(stor)
	srv, c := requireTestClient(t, New(tlog, stor, graph))
	defer srv.Close()
	testcases := []collectionTestCase[api.ConnectorRecord]{
		{ExpectOK: true},
		{
			ExpectOK: true,
			Records: wrapRecords( // totally empty
				vanflow.ConnectorRecord{
					BaseRecord: vanflow.NewBase("c1"),
				},
			),
			ExpectCount: 1,
			ExpectResults: func(t *testing.T, results []api.ConnectorRecord) {
				assert.Equal(t, len(results), 1)
				result := results[0]
				assert.DeepEqual(t, result, api.ConnectorRecord{
					Identity: "c1", Name: "unknown", Parent: "unknown",
					SiteName: "unknown", SiteId: "unknown",
					Protocol: "unknown", Address: "unknown", DestHost: "unknown",
					ProcessId: "", DestPort: "unknown",
				})
			},
		}, {
			Records: wrapRecords(
				vanflow.SiteRecord{BaseRecord: vanflow.NewBase("s1"), Name: ptrTo("one")},
				vanflow.RouterRecord{BaseRecord: vanflow.NewBase("r1"), Parent: ptrTo("s1")},
				vanflow.ConnectorRecord{
					BaseRecord: vanflow.NewBase("c1"), Name: ptrTo("conn-one"),
					Parent:    ptrTo("r1"),
					ProcessID: ptrTo("p1"),
					Protocol:  ptrTo("tp"),
					Address:   ptrTo("trombone"),
					DestHost:  ptrTo("10.0.four.two"), DestPort: ptrTo("amqp"),
				},
				vanflow.ProcessRecord{BaseRecord: vanflow.NewBase("p1"), Name: ptrTo("proc-one")},
				vanflow.ListenerRecord{
					BaseRecord: vanflow.NewBase("l1"),
					Protocol:   ptrTo("tp"),
					Address:    ptrTo("trombone"),
				},
			),
			ExpectOK:    true,
			ExpectCount: 1,
			ExpectResults: func(t *testing.T, results []api.ConnectorRecord) {
				assert.Equal(t, len(results), 1)
				result := results[0]
				assert.DeepEqual(t, result, api.ConnectorRecord{
					Identity: "c1", Name: "conn-one", Parent: "r1",
					SiteName: "one", SiteId: "s1",
					Protocol: "tp", Address: "trombone", DestHost: "10.0.four.two",
					ProcessId: "p1", Target: ptrTo("proc-one"), DestPort: "amqp",
				})
			},
		}, {
			Records: wrapRecords(
				vanflow.SiteRecord{BaseRecord: vanflow.NewBase("s1"), Name: ptrTo("one")},
				vanflow.RouterRecord{BaseRecord: vanflow.NewBase("r1"), Parent: ptrTo("s1")},
				vanflow.ConnectorRecord{
					BaseRecord: vanflow.NewBase("c1"), Name: ptrTo("conn-one"),
					Parent:    ptrTo("r1"),
					ProcessID: ptrTo("does-not-exist"),
					Protocol:  ptrTo("tp"),
					Address:   ptrTo("trombone"),
					DestHost:  ptrTo("10.0.four.two"), DestPort: ptrTo("amqp"),
				},
				vanflow.ProcessRecord{
					BaseRecord: vanflow.NewBase("proc-site"),
					Name:       ptrTo("my site service"),
					Parent:     ptrTo("s1"),
					SourceHost: ptrTo("10.0.four.two")},
			),
			ExpectOK:    true,
			ExpectCount: 1,
			ExpectResults: func(t *testing.T, results []api.ConnectorRecord) {
				assert.Equal(t, len(results), 1)
				result := results[0]
				assert.Equal(t, result.ProcessId, "proc-site")
				assert.DeepEqual(t, result.Target, ptrTo("my site service"))
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
			resp, err := c.ConnectorsWithResponse(context.TODO(), withParameters(tc.Parameters))
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
			} else {
				assert.Check(t, resp.JSON400 != nil)
			}
		})
	}
}

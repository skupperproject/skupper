package server

import (
	"context"
	"log/slog"
	"testing"

	"github.com/skupperproject/skupper/cmd/network-console-collector/internal/api"
	"github.com/skupperproject/skupper/cmd/network-console-collector/internal/collector"
	"github.com/skupperproject/skupper/pkg/vanflow"
	"github.com/skupperproject/skupper/pkg/vanflow/store"
	"gotest.tools/assert"
)

func TestProcesses(t *testing.T) {
	tlog := slog.Default()
	stor := store.NewSyncMapStore(store.SyncMapStoreConfig{Indexers: collector.RecordIndexers()})
	graph := collector.NewGraph(stor)
	srv, c := requireTestClient(t, New(tlog, stor, graph))
	defer srv.Close()

	testcases := []struct {
		Records              []store.Entry
		ExpectOK             bool
		ExpectCount          int
		ExpectTimeRangeCount int
		ExpectResults        func(t *testing.T, results []api.ProcessRecord)
	}{
		{ExpectOK: true},
		{
			Records: wrapRecords(
				vanflow.ProcessRecord{BaseRecord: vanflow.NewBase("1")},
				vanflow.ProcessRecord{BaseRecord: vanflow.NewBase("2")},
				vanflow.ProcessRecord{BaseRecord: vanflow.NewBase("3")},
			),
			ExpectOK:    true,
			ExpectCount: 3,
		}, {
			Records: wrapRecords(
				vanflow.ProcessRecord{BaseRecord: vanflow.NewBase("1")},
			),
			ExpectOK:    true,
			ExpectCount: 1,
			ExpectResults: func(t *testing.T, results []api.ProcessRecord) {
				r := results[0]
				assert.DeepEqual(t, r, api.ProcessRecord{
					Identity:       "1",
					Parent:         "unknown",
					ParentName:     "unknown",
					GroupName:      "unknown",
					GroupIdentity:  "unknown",
					ProcessBinding: api.Unbound,
					Name:           "unknown",
					ProcessRole:    api.External,
					SourceHost:     "unknown",
				})
			},
		},
		{
			Records: wrapRecords(
				vanflow.ProcessRecord{
					Parent:     ptrTo("site-1"),
					Name:       ptrTo("processone"),
					BaseRecord: vanflow.NewBase("1"),
					Group:      ptrTo("group-one"),
					SourceHost: ptrTo("10.99.99.2"),
					Mode:       ptrTo("internal"),
				},
				collector.ProcessGroupRecord{ID: "group-1-id", Name: "group-one"},
				collector.AddressRecord{ID: "pizza-addr-id", Name: "pizza", Protocol: "tcp"},
				vanflow.ConnectorRecord{
					BaseRecord: vanflow.NewBase("c1"),
					Address:    ptrTo("pizza"),
					Protocol:   ptrTo("tcp"),
					ProcessID:  ptrTo("1"),
				},
				collector.AddressRecord{ID: "icecream-addr-id", Name: "icecream", Protocol: "tcp"},
				vanflow.RouterRecord{BaseRecord: vanflow.NewBase("router-1"), Parent: ptrTo("site-1")},
				vanflow.SiteRecord{BaseRecord: vanflow.NewBase("site-1"), Name: ptrTo("site one")},
				vanflow.ConnectorRecord{
					Parent:     ptrTo("router-1"),
					BaseRecord: vanflow.NewBase("c2"),
					Address:    ptrTo("icecream"),
					Protocol:   ptrTo("tcp"),
					DestHost:   ptrTo("10.99.99.2"),
				},
			),
			ExpectOK:    true,
			ExpectCount: 1,
			ExpectResults: func(t *testing.T, results []api.ProcessRecord) {
				r := results[0]
				assert.DeepEqual(t, r, api.ProcessRecord{
					Identity:       "1",
					Parent:         "site-1",
					ParentName:     "site one",
					Addresses:      ptrTo([]string{"pizza@pizza-addr-id@tcp", "icecream@icecream-addr-id@tcp"}),
					GroupName:      "group-one",
					GroupIdentity:  "group-1-id",
					ProcessBinding: api.Bound,
					Name:           "processone",
					ProcessRole:    api.Internal,
					SourceHost:     "10.99.99.2",
				})
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
			resp, err := c.ProcessesWithResponse(context.TODO())
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

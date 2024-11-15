package server

import (
	"context"
	"log/slog"
	"testing"

	"github.com/skupperproject/skupper/cmd/network-console-collector/internal/api"
	"github.com/skupperproject/skupper/cmd/network-console-collector/internal/collector"
	"github.com/skupperproject/skupper/pkg/vanflow/store"
	"gotest.tools/assert"
)

func TestProcessPairsByAddress(t *testing.T) {
	tlog := slog.Default()
	stor := store.NewSyncMapStore(store.SyncMapStoreConfig{Indexers: collector.RecordIndexers()})
	graph := collector.NewGraph(stor)
	srv, c := requireTestClient(t, New(tlog, stor, graph))
	defer srv.Close()

	testcases := []struct {
		Records              []store.Entry
		ID                   string
		Parameters           map[string][]string
		ExpectOK             bool
		ExpectCount          int
		ExpectTimeRangeCount int
		ExpectResults        func(t *testing.T, results []api.FlowAggregateRecord)
	}{
		{ExpectOK: false, ID: "dne"},
		{
			ExpectOK: true,
			ID:       "address1",
			Records:  wrapRecords(collector.AddressRecord{ID: "address1"}),
		},
	}

	for _, tc := range testcases {
		t.Run("", func(t *testing.T) {
			stor.Replace(tc.Records)
			graph.(reset).Reset()
			for _, r := range tc.Records {
				graph.(reset).Reindex(r.Record)
			}
			resp, err := c.ProcessPairsByAddressWithResponse(context.TODO(), tc.ID, withParameters(tc.Parameters))
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
				assert.Check(t, resp.JSON404 != nil)
			}
		})
	}
}

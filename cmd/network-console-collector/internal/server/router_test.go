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

func TestRouters(t *testing.T) {
	tlog := slog.Default()
	stor := store.NewSyncMapStore(store.SyncMapStoreConfig{Indexers: collector.RecordIndexers()})
	graph := collector.NewGraph(stor)
	srv, c := requireTestClient(t, New(tlog, stor, graph))
	defer srv.Close()

	van := []vanflow.Record{
		vanflow.SiteRecord{BaseRecord: vanflow.NewBase("site-a"), Name: ptrTo("site a")},
		vanflow.RouterRecord{BaseRecord: vanflow.NewBase("router-a-1"), Name: ptrTo("router a.1"), Parent: ptrTo("site-a"),
			Mode: ptrTo("inter-router"),
		},

		vanflow.SiteRecord{BaseRecord: vanflow.NewBase("site-b"), Name: ptrTo("site b")},
		vanflow.RouterRecord{BaseRecord: vanflow.NewBase("router-b-1"), Name: ptrTo("router b.1"), Parent: ptrTo("site-b")},
		vanflow.RouterRecord{BaseRecord: vanflow.NewBase("router-b-2"), Name: ptrTo("router b.2"), Parent: ptrTo("site-b")},
	}

	testcases := []struct {
		Records              []store.Entry
		Parameters           map[string][]string
		ExpectOK             bool
		ExpectCount          int
		ExpectTimeRangeCount int
		ExpectResults        func(t *testing.T, results []api.RouterRecord)
	}{
		{ExpectOK: true},
		{
			ExpectOK:    true,
			Records:     wrapRecords(van...),
			ExpectCount: 3,
			ExpectResults: func(t *testing.T, results []api.RouterRecord) {
				for _, result := range results {
					switch result.Identity {
					case "router-a-1":
						assert.DeepEqual(
							t, result, api.RouterRecord{
								Identity:     "router-a-1",
								Name:         "router a.1",
								HostName:     "unknown",
								ImageName:    "unknown",
								ImageVersion: "unknown",
								Mode:         "inter-router",
								Parent:       "site-a",
							},
						)

					}

				}
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
			resp, err := c.RoutersWithResponse(context.TODO(), WithParameters(tc.Parameters))
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

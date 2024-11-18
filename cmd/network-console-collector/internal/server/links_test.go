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

func TestRouterlinks(t *testing.T) {
	tlog := slog.Default()
	stor := store.NewSyncMapStore(store.SyncMapStoreConfig{Indexers: collector.RecordIndexers()})
	graph := collector.NewGraph(stor)
	srv, c := requireTestClient(t, New(tlog, stor, graph))
	defer srv.Close()

	van := []vanflow.Record{
		vanflow.SiteRecord{BaseRecord: vanflow.NewBase("site-a"), Name: ptrTo("site a")},
		vanflow.RouterRecord{BaseRecord: vanflow.NewBase("router-a-1"), Name: ptrTo("router a.1"), Parent: ptrTo("site-a")},
		vanflow.RouterAccessRecord{BaseRecord: vanflow.NewBase("routeraccess-a-1"), Name: ptrTo("routeraccess a.1"), Parent: ptrTo("router-a-1")},

		vanflow.SiteRecord{BaseRecord: vanflow.NewBase("site-b"), Name: ptrTo("site b")},
		vanflow.RouterRecord{BaseRecord: vanflow.NewBase("router-b-1"), Name: ptrTo("router b.1"), Parent: ptrTo("site-b")},
		vanflow.RouterAccessRecord{BaseRecord: vanflow.NewBase("routeraccess-b-1"), Name: ptrTo("routeraccess b.1"), Parent: ptrTo("router-b-1")},
		vanflow.RouterRecord{BaseRecord: vanflow.NewBase("router-b-2"), Name: ptrTo("router b.2"), Parent: ptrTo("site-b")},
		vanflow.RouterAccessRecord{BaseRecord: vanflow.NewBase("routeraccess-b-2"), Name: ptrTo("routeraccess b.2"), Parent: ptrTo("router-b-2")},
	}

	testcases := []collectionTestCase[api.RouterLinkRecord]{
		{ExpectOK: true},
		{
			ExpectOK: true,
			Records: wrapRecords(
				append(
					van,
					vanflow.LinkRecord{
						BaseRecord: vanflow.NewBase("link1"),
						Parent:     ptrTo("router-b-1"),
						Name:       ptrTo("linkb1"),
						Status:     ptrTo("down"),
					},
					vanflow.LinkRecord{
						BaseRecord: vanflow.NewBase("link2"),
						Parent:     ptrTo("router-b-2"),
						Name:       ptrTo("linkb2"),
						Status:     ptrTo("up"),
						Peer:       ptrTo("routeraccess-a-1"),
						LinkCost:   ptrTo(uint64(3)),
					},
				)...,
			),
			ExpectCount: 2,
			ExpectResults: func(t *testing.T, results []api.RouterLinkRecord) {
				link1, link2 := results[0], results[1]
				if link1.Identity != "link1" {
					link1, link2 = link2, link1
				}
				assert.Equal(t, string(link1.Status), "down")
				assert.Equal(t, link2.SourceSiteName, "site b")
				assert.DeepEqual(t, link2, api.RouterLinkRecord{
					Identity:              "link2",
					Cost:                  ptrTo(uint64(3)),
					Status:                api.Up,
					Name:                  "linkb2",
					Role:                  "unknown",
					RouterId:              "router-b-2",
					RouterName:            "router b.2",
					SourceSiteId:          "site-b",
					SourceSiteName:        "site b",
					RouterAccessId:        ptrTo("routeraccess-a-1"),
					DestinationSiteId:     ptrTo("site-a"),
					DestinationSiteName:   ptrTo("site a"),
					DestinationRouterId:   ptrTo("router-a-1"),
					DestinationRouterName: ptrTo("router a.1"),
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
			resp, err := c.RouterlinksWithResponse(context.TODO(), withParameters(tc.Parameters))
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

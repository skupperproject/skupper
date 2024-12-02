package server

import (
	"context"
	"log/slog"
	"strings"
	"testing"

	"github.com/skupperproject/skupper/cmd/network-observer/internal/api"
	"github.com/skupperproject/skupper/cmd/network-observer/internal/collector"
	"github.com/skupperproject/skupper/pkg/vanflow"
	"github.com/skupperproject/skupper/pkg/vanflow/store"
	"gotest.tools/v3/assert"
)

func TestSites(t *testing.T) {
	tlog := slog.Default()
	stor := store.NewSyncMapStore(store.SyncMapStoreConfig{})
	graph := collector.NewGraph(stor)
	srv, c := requireTestClient(t, New(tlog, stor, graph))
	defer srv.Close()

	testcases := []collectionTestCase[api.SiteRecord]{
		{ExpectOK: true},
		{
			Records: wrapRecords(
				vanflow.SiteRecord{BaseRecord: vanflow.NewBase("site-1")},
				vanflow.SiteRecord{BaseRecord: vanflow.NewBase("site-2")},
				vanflow.SiteRecord{BaseRecord: vanflow.NewBase("site-3")},
			),
			ExpectOK:    true,
			ExpectCount: 3,
		},
		{
			Records: wrapRecords(
				vanflow.SiteRecord{BaseRecord: vanflow.NewBase("site-1")},
				vanflow.SiteRecord{BaseRecord: vanflow.NewBase("site-2")},
				vanflow.SiteRecord{BaseRecord: vanflow.NewBase("site-3")},
			),
			ExpectOK:             true,
			ExpectCount:          1,
			ExpectTimeRangeCount: 3,
			Parameters:           map[string][]string{"limit": {"1"}},
		},
		{
			Records: wrapRecords(
				vanflow.SiteRecord{BaseRecord: vanflow.NewBase("site-1"), Namespace: ptrTo("")},
				vanflow.SiteRecord{BaseRecord: vanflow.NewBase("site-2"), Namespace: ptrTo("testns")},
				vanflow.SiteRecord{BaseRecord: vanflow.NewBase("site-3")},
				vanflow.RouterRecord{BaseRecord: vanflow.NewBase("router-2a"), Parent: ptrTo("site-2")},
				vanflow.RouterRecord{BaseRecord: vanflow.NewBase("router-2b"), Parent: ptrTo("site-2")},
				vanflow.RouterRecord{BaseRecord: vanflow.NewBase("router-2c"), Parent: ptrTo("site-2")},
				vanflow.RouterRecord{BaseRecord: vanflow.NewBase("router-1a"), Parent: ptrTo("site-1")},
			),
			ExpectOK:             true,
			ExpectCount:          1,
			ExpectTimeRangeCount: 1,
			ExpectResults: func(t *testing.T, results []api.SiteRecord) {
				assert.Equal(t, results[0].Identity, "site-2")
				assert.Equal(t, results[0].RouterCount, 3)
			},
			Parameters: map[string][]string{"nameSpace": {"testns"}},
		},
		{
			Parameters:  map[string][]string{"fizz": {"baz", "buz"}},
			ExpectError: "invalid filter",
		},
	}

	for _, tc := range testcases {
		t.Run("", func(t *testing.T) {
			stor.Replace(tc.Records)
			graph.(reset).Reset()
			resp, err := c.SitesWithResponse(context.TODO(), withParameters(tc.Parameters))
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
				assert.Check(t, strings.Contains(resp.JSON400.Message, tc.ExpectError), "expected string %q in message %q", tc.ExpectError, resp.JSON400.Message)
			}
		})

	}
}

func TestSiteByID(t *testing.T) {
	tlog := slog.Default()
	stor := store.NewSyncMapStore(store.SyncMapStoreConfig{})
	graph := collector.NewGraph(stor)
	srv, c := requireTestClient(t, New(tlog, stor, graph))
	defer srv.Close()

	testcases := []struct {
		ID           string
		Records      []store.Entry
		ExpectOK     bool
		ExpectResult func(t *testing.T, results api.SiteRecord)
	}{
		{
			ID:       "testing",
			ExpectOK: false,
		}, {
			ID: "site-1",
			Records: wrapRecords(
				vanflow.SiteRecord{BaseRecord: vanflow.NewBase("site-1")},
				vanflow.SiteRecord{BaseRecord: vanflow.NewBase("site-2")},
				vanflow.SiteRecord{BaseRecord: vanflow.NewBase("site-3")},
			),
			ExpectOK: true,
			ExpectResult: func(t *testing.T, results api.SiteRecord) {
				assert.DeepEqual(t, results, api.SiteRecord{
					Identity:    "site-1",
					Name:        "unknown",
					Platform:    "unknown",
					SiteVersion: "unknown",
				})
			},
		},
	}

	for _, tc := range testcases {
		t.Run("", func(t *testing.T) {
			stor.Replace(tc.Records)
			graph.(reset).Reset()
			resp, err := c.SiteByIdWithResponse(context.TODO(), tc.ID)
			assert.Check(t, err)
			if tc.ExpectOK {
				assert.Equal(t, resp.StatusCode(), 200)
				if tc.ExpectResult != nil {
					tc.ExpectResult(t, resp.JSON200.Results)
				}
			} else {
				assert.Check(t, resp.JSON404 != nil)
				assert.Equal(t, resp.JSON404.Code, "ErrNotFound")
			}
		})

	}
}

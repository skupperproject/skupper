package server

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/skupperproject/skupper/cmd/network-console-collector/internal/api"
	"github.com/skupperproject/skupper/cmd/network-console-collector/internal/collector"
	"github.com/skupperproject/skupper/pkg/vanflow"
	"github.com/skupperproject/skupper/pkg/vanflow/store"
	"gotest.tools/assert"
)

func TestSites(t *testing.T) {
	tlog := slog.Default()
	stor := store.NewSyncMapStore(store.SyncMapStoreConfig{})
	graph := collector.NewGraph(stor)
	srv, c := requireTestClient(t, New(tlog, stor, graph))
	defer srv.Close()

	testcases := []struct {
		Records              []store.Entry
		Parameters           map[string][]string
		ExpectOK             bool
		ExpectCount          int
		ExpectTimeRangeCount int
		ExpectResults        func(t *testing.T, results []api.SiteRecord)
		ExpectError          string
	}{
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
			),
			ExpectOK:             true,
			ExpectCount:          1,
			ExpectTimeRangeCount: 1,
			ExpectResults: func(t *testing.T, results []api.SiteRecord) {
				assert.Equal(t, results[0].Identity, "site-2")
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
			resp, err := c.SitesWithResponse(context.TODO(), func(ctx context.Context, r *http.Request) error {
				values := r.URL.Query()
				for k, vs := range tc.Parameters {
					for _, v := range vs {
						values.Add(k, v)
					}
				}
				r.URL.RawQuery = values.Encode()
				return nil
			})
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

func requireTestClient(t *testing.T, impl api.ServerInterface) (*httptest.Server, api.ClientWithResponsesInterface) {
	t.Helper()
	htsrv := httptest.NewTLSServer(api.Handler(impl))
	client, err := api.NewClientWithResponses(htsrv.URL, api.WithHTTPClient(htsrv.Client()))
	if err != nil {
		t.Fatalf("unexpected error setting up test http client: %s", err)
	}
	return htsrv, client
}

func ptrTo[T any](c T) *T {
	return &c
}

func wrapRecords(records ...vanflow.Record) []store.Entry {
	entries := make([]store.Entry, len(records))
	for i := range records {
		entries[i].Record = records[i]
	}
	return entries
}

type reset interface {
	Reset()
}

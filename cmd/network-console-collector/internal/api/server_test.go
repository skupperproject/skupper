package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"gotest.tools/assert"
)

type stubSiteServer struct {
	ServerInterface
	Records []SiteRecord
}

func (s stubSiteServer) Sites(w http.ResponseWriter, r *http.Request) {
	resp := &SiteListResponse{}
	resp.SetCount(int64(len(s.Records)))
	resp.SetTimeRangeCount(int64(len(s.Records)))
	resp.SetResults(s.Records)
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		panic(err)
	}

}

func TestExampleTest(t *testing.T) {
	// purely contrived test of a dummy implementation of the server
	expected := []SiteRecord{
		{
			Identity: "a",
			Name:     "site-a",
		},
	}
	srv, c := requireTestClient(t, stubSiteServer{
		Records: expected,
	})
	defer srv.Close()

	resp, err := c.SitesWithResponse(context.TODO())
	assert.Check(t, err)
	assert.Equal(t, resp.StatusCode(), 200)
	assert.DeepEqual(t, resp.JSON200.Results, expected)
}

func requireTestClient(t *testing.T, impl ServerInterface) (*httptest.Server, ClientWithResponsesInterface) {
	t.Helper()
	htsrv := httptest.NewTLSServer(Handler(impl))
	client, err := NewClientWithResponses(htsrv.URL, WithHTTPClient(htsrv.Client()))
	if err != nil {
		t.Fatalf("unexpected error setting up test http client: %s", err)
	}
	return htsrv, client
}

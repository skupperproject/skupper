package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/skupperproject/skupper/cmd/network-observer/internal/api"
	"github.com/skupperproject/skupper/pkg/vanflow"
	"github.com/skupperproject/skupper/pkg/vanflow/store"
)

func requireTestClient(t *testing.T, impl api.ServerInterface) (*httptest.Server, api.ClientWithResponsesInterface) {
	t.Helper()
	htsrv := httptest.NewTLSServer(api.Handler(impl))
	client, err := api.NewClientWithResponses(htsrv.URL, api.WithHTTPClient(htsrv.Client()))
	if err != nil {
		t.Fatalf("unexpected error setting up test http client: %s", err)
	}
	return htsrv, client
}

type collectionTestCase[T api.Record] struct {
	Records              []store.Entry
	Flows                []store.Entry
	Parameters           map[string][]string
	ExpectOK             bool
	ExpectCount          int
	ExpectTimeRangeCount int
	ExpectError          string
	ExpectResults        func(t *testing.T, results []T)
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
	Reindex(vanflow.Record)
}

func withParameters(params map[string][]string) func(context.Context, *http.Request) error {
	return func(ctx context.Context, r *http.Request) error {
		values := r.URL.Query()
		for k, vs := range params {
			for _, v := range vs {
				values.Add(k, v)
			}
		}
		r.URL.RawQuery = values.Encode()
		return nil
	}
}

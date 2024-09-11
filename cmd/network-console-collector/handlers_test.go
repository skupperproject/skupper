package main

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandleProxyPrometheusAPI(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Origin") != "" {
			w.Header().Add("Access-Control-Allow-Origin", "*")
		}
		switch r.URL.Path {
		case "/api/v1/query":
			w.WriteHeader(200)
			w.Write([]byte("query response"))
		case "/api/v1/query_range":
			w.WriteHeader(200)
			w.Write([]byte("query_range response"))
		default:
			w.WriteHeader(404)
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer upstream.Close()

	promAPI, err := parsePrometheusAPI(upstream.URL)
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewTLSServer(handleProxyPrometheusAPI("/test", promAPI))
	defer srv.Close()

	client := srv.Client()

	testCases := []struct {
		Path            string
		RequestHeaders  map[string]string
		ResponseHeaders map[string]string
		Status          int
		Content         []byte
	}{
		{
			Path:   "/query",
			Status: 404,
		}, {
			Path:   "/test/query/extra",
			Status: 404,
		}, {
			Path:    "/test/query",
			Status:  200,
			Content: []byte("query response"),
		}, {
			Path: "/test/query",
			RequestHeaders: map[string]string{
				"Origin": "foo",
			},
			ResponseHeaders: map[string]string{
				"Access-Control-Allow-Origin": "*",
			},
			Status:  200,
			Content: []byte("query response"),
		}, {
			Path:    "/test/query/",
			Status:  200,
			Content: []byte("query response"),
		}, {
			Path:    "/test/rangequery",
			Content: []byte("query_range response"),
			Status:  200,
		}, {
			Path:   "/test/rangequery/",
			Status: 200,
		}, {
			Path:   "/test/rangequery/1",
			Status: 404,
		}, {
			Path:    "/test/query_range",
			Content: []byte("query_range response"),
			Status:  200,
		}, {
			Path:   "/test/query_range/",
			Status: 200,
		}, {
			Path:   "/test/query_range/1",
			Status: 404,
		},
	}
	for _, tc := range testCases {
		req, _ := http.NewRequest(http.MethodGet, srv.URL+tc.Path, nil)
		for k, v := range tc.RequestHeaders {
			req.Header.Set(k, v)
		}
		r, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		if r.StatusCode != tc.Status {
			t.Errorf("expected status %d but got %d", tc.Status, r.StatusCode)
		}
		if len(tc.Content) > 0 {
			defer r.Body.Close()
			content, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.EqualFold(content, tc.Content) {
				t.Errorf("expected response body %q but got %q", string(tc.Content), string(content))
			}
		}
		for k, expected := range tc.ResponseHeaders {
			if actual := r.Header.Get(k); !strings.EqualFold(expected, actual) {
				t.Errorf("expected header %q: %q %q", k, expected, actual)
			}
		}
	}
}

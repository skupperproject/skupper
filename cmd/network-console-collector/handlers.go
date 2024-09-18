package main

import (
	"io/fs"
	"net/http"
	"net/http/httputil"
	"net/url"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func handleMetrics(reg *prometheus.Registry) http.Handler {
	return promhttp.HandlerFor(reg, promhttp.HandlerOpts{Registry: reg})
}

func handleSwagger(prefix string, content fs.FS) http.Handler {
	return http.StripPrefix(prefix, http.FileServer(http.FS(content)))
}
func handleConsoleAssets(consoleDir string) http.Handler {
	return http.FileServer(http.Dir(consoleDir))
}

func handleNoContent() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
}

func handleProxyPrometheusAPI(prefix string, target *url.URL) http.Handler {
	proxy := httputil.NewSingleHostReverseProxy(target)
	return http.StripPrefix(prefix,
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/query/":
				r.URL.Path = "/query"
				fallthrough
			case "/query":
				proxy.ServeHTTP(w, r)
			case "/rangequery":
				fallthrough
			case "/rangequery/":
				fallthrough
			case "/query_range/":
				r.URL.Path = "/query_range"
				fallthrough
			case "/query_range":
				proxy.ServeHTTP(w, r)
			default:
				http.NotFound(w, r)
			}
		}),
	)
}

package main

import (
	"encoding/base64"
	"encoding/json"
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

func handleGetUser() http.Handler {
	type UserResponse struct {
		Username string `json:"username"`
		AuthMode string `json:"authType"`
	}
	handleEmpty := handleNoContent()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var response UserResponse
		if cookie, err := r.Cookie("_oauth_proxy"); err == nil && cookie != nil {
			if cookieDecoded, _ := base64.StdEncoding.DecodeString(cookie.Value); cookieDecoded != nil {
				response.Username = string(cookieDecoded)
				response.AuthMode = "openshift"
				json.NewEncoder(w).Encode(response)
				return
			}
		}
		handleEmpty.ServeHTTP(w, r)
	})
}

func handleUserLogout() http.Handler {
	handleEmpty := handleNoContent()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// when an oauth proxy cookie is present, delete it
		if cookie, err := r.Cookie("_oauth_proxy"); err == nil && cookie != nil {
			cookie := http.Cookie{
				Name:   "_oauth_proxy", // openshift cookie name
				Path:   "/",
				MaxAge: -1,
				Domain: r.Host,
			}
			http.SetCookie(w, &cookie)
			return
		}
		handleEmpty.ServeHTTP(w, r)
	})
}

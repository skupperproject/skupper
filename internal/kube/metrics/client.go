package metrics

import (
	"context"
	"net/url"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/client-go/tools/metrics"
)

// MustRegisterClientGoMetrics registers a set of metrics exposed from the
// k8s.io/client-go/tools/metrics package with the prometheus registry.
func MustRegisterClientGoMetrics(registry *prometheus.Registry) {
	httpMetrics := &clientGoHttpMetrics{
		latency: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: "skupper",
			Subsystem: "kubernetes_client",
			Name:      "http_request_duration_seconds",
			Help:      "Latency of kubernetes client requests in seconds by endpoint.",
		}, []string{"method", "endpoint"}),
		results: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "skupper",
			Subsystem: "kubernetes_client",
			Name:      "http_requests_total",
			Help:      "Total number of kubernetes client requests by status code.",
		}, []string{"status_code"}),
		retries: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "skupper",
			Subsystem: "kubernetes_client",
			Name:      "http_retries_total",
			Help:      "Total number of kubernetes client requests retried by status code.",
		}, []string{"status_code"}),
	}
	rateLimiterMetrics := &clientGoRateLimiterMetrics{
		latency: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: "skupper",
			Subsystem: "kubernetes_client",
			Name:      "rate_limiter_duration_seconds",
			Help:      "Latency of kubernetes client side rate limiting in seconds by endpoint.",
		}, []string{"method", "endpoint"}),
	}

	registry.MustRegister(httpMetrics.latency, httpMetrics.results, httpMetrics.retries, rateLimiterMetrics.latency)
	metrics.Register(metrics.RegisterOpts{
		RequestLatency: httpMetrics,
		RequestResult:  httpMetrics,
		RequestRetry:   httpMetrics,

		RateLimiterLatency: rateLimiterMetrics,
	})
}

type clientGoHttpMetrics struct {
	latency *prometheus.HistogramVec
	results *prometheus.CounterVec
	retries *prometheus.CounterVec
}

func (m *clientGoHttpMetrics) Observe(ctx context.Context, verb string, url url.URL, latency time.Duration) {
	m.latency.WithLabelValues(verb, url.EscapedPath()).Observe(latency.Seconds())
}

func (m *clientGoHttpMetrics) Increment(ctx context.Context, code string, method string, host string) {
	m.results.WithLabelValues(code).Inc()
}
func (m *clientGoHttpMetrics) IncrementRetry(ctx context.Context, code string, _ string, _ string) {
	m.retries.WithLabelValues(code).Inc()
}

type clientGoRateLimiterMetrics struct {
	latency *prometheus.HistogramVec
}

func (m *clientGoRateLimiterMetrics) Observe(ctx context.Context, verb string, url url.URL, latency time.Duration) {
	m.latency.WithLabelValues(verb, url.EscapedPath()).Observe(latency.Seconds())
}

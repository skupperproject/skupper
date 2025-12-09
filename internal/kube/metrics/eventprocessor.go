package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/skupperproject/skupper/internal/kube/watchers"
)

func MustRegisterEventProcessorMetrics(registry *prometheus.Registry) watchers.MetricsProvider {
	provider := eventProcessorMetrics{
		adds: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "skupper",
			Subsystem: "workqueue",
			Name:      "adds_total",
			Help:      "Total number of events queued.",
		}, []string{"kind"}),
		depth: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: "skupper",
			Subsystem: "workqueue",
			Name:      "depth",
			Help:      "Current depth of event queue.",
		}, []string{"kind"}),
		delayDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: "skupper",
			Subsystem: "workqueue",
			Name:      "delay_seconds",
			Help:      "How long in seconds an event is queued before it is handled.",
			Buckets:   prometheus.ExponentialBuckets(1e-9, 10, 12),
		}, []string{"kind"}),
		workDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: "skupper",
			Subsystem: "workqueue",
			Name:      "duration_seconds",
			Help:      "How long in seconds handling a watch event takes.",
			Buckets:   prometheus.ExponentialBuckets(1e-9, 10, 12),
		}, []string{"kind"}),
		retries: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "skupper",
			Subsystem: "workqueue",
			Name:      "retries_total",
			Help:      "Total number of retries queued.",
		}, []string{"kind"}),
	}
	registry.MustRegister(provider.adds, provider.depth, provider.delayDuration, provider.workDuration, provider.retries)
	return provider
}

type eventProcessorMetrics struct {
	adds          *prometheus.CounterVec
	depth         *prometheus.GaugeVec
	delayDuration *prometheus.HistogramVec
	workDuration  *prometheus.HistogramVec
	retries       *prometheus.CounterVec
}

func (p eventProcessorMetrics) NewAddedMetric(kind string) watchers.CounterMetric {
	return p.adds.WithLabelValues(kind)
}
func (p eventProcessorMetrics) NewDepthMetric(kind string) watchers.GaugeMetric {
	return p.depth.WithLabelValues(kind)
}
func (p eventProcessorMetrics) NewDelayedMetric(kind string) watchers.ObservableMetric {
	return p.delayDuration.WithLabelValues(kind)
}
func (p eventProcessorMetrics) NewWorkDurationMetric(kind string) watchers.ObservableMetric {
	return p.workDuration.WithLabelValues(kind)
}
func (p eventProcessorMetrics) NewRetriesMetric(kind string) watchers.CounterMetric {
	return p.retries.WithLabelValues(kind)
}

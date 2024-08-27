package collector

import "github.com/prometheus/client_golang/prometheus"

type metrics struct {
	flowOpenedCounter        *prometheus.CounterVec
	flowClosedCounter        *prometheus.CounterVec
	flowBytesSentCounter     *prometheus.CounterVec
	flowBytesReceivedCounter *prometheus.CounterVec
	requestsCounter          *prometheus.CounterVec

	internal metricsInternal
}

type metricsInternal struct {
	flowLatency        *prometheus.HistogramVec
	legancyLatency     *prometheus.HistogramVec
	flowProcessingTime *prometheus.HistogramVec
	reconcileTime      *prometheus.HistogramVec
	queueUtilization   *prometheus.GaugeVec
	pendingFlows       *prometheus.GaugeVec
}

func register(reg *prometheus.Registry) metrics {
	m := metrics{
		flowOpenedCounter: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "skupper",
			Name:      "connections_opened_total",
			Help:      "Number of connections opened through the application network",
		}, flowMetricLables),
		flowClosedCounter: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "skupper",
			Name:      "connections_closed_total",
			Help:      "Number of connections opened through the application network that have been closed",
		}, flowMetricLables),
		flowBytesSentCounter: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "skupper",
			Name:      "sent_bytes_total",
			Help:      "Bytes sent through the application network from client to service",
		}, flowMetricLables),
		flowBytesReceivedCounter: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "skupper",
			Name:      "received_bytes_total",
			Help:      "Bytes sent through the application network back from service to client",
		}, flowMetricLables),
		requestsCounter: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "skupper",
			Name:      "requests_total",
			Help:      "Counter incremented for each request handled through the skupper network",
		}, appFlowMetricLables),
		internal: metricsInternal{
			flowLatency: prometheus.NewHistogramVec(prometheus.HistogramOpts{
				Namespace: "skupper",
				Subsystem: "internal",
				Name:      "latency_seconds",
				Help:      "Latency observed measured as seconds difference between TTFB between listener and connector sides",
				Buckets:   histBucketsFast,
			}, flowMetricLables),
			legancyLatency: prometheus.NewHistogramVec(prometheus.HistogramOpts{
				Namespace: "legacy",
				Name:      "flow_latency_microseconds",
				Help:      "Time to first byte observed from the listener (client) side",
				Buckets:   []float64{10, 100, 1000, 2000, 5000, 10000, 100000, 1000000, 10000000},
			}, []string{"sourceSite", "destSite", "address", "protocol", "sourceProcess", "destProcess"}),
			reconcileTime: prometheus.NewHistogramVec(prometheus.HistogramOpts{
				Namespace: "skupper",
				Subsystem: "internal",
				Name:      "collector_job_seconds",
				Help:      "Time spent in periodic reconcile jobs in the collector",
				Buckets:   histBucketsFast,
			}, []string{"eventsource", "type"}),
			flowProcessingTime: prometheus.NewHistogramVec(prometheus.HistogramOpts{
				Namespace: "skupper",
				Subsystem: "internal",
				Name:      "flow_processing_seconds",
				Help:      "Time spent handling individual vanflow record updates",
				Buckets:   histBucketsFast,
			}, []string{"type"}),
			queueUtilization: prometheus.NewGaugeVec(prometheus.GaugeOpts{
				Namespace: "skupper",
				Subsystem: "internal",
				Name:      "queue_utilization_percentage",
				Help:      "Percentage of vanflow event processing queue full",
			}, []string{"type"}),
			pendingFlows: prometheus.NewGaugeVec(prometheus.GaugeOpts{
				Namespace: "skupper",
				Subsystem: "internal",
				Name:      "pending_flows",
			}, []string{"type", "reason", "eventsource"}),
		},
	}

	reg.MustRegister(
		m.flowOpenedCounter,
		m.flowClosedCounter,
		m.flowBytesSentCounter,
		m.flowBytesReceivedCounter,
		m.requestsCounter,
		m.internal.legancyLatency,
		m.internal.flowLatency,
		m.internal.reconcileTime,
		m.internal.queueUtilization,
		m.internal.flowProcessingTime,
		m.internal.pendingFlows,
	)
	return m
}

var (
	histBucketsFast  = []float64{0.001, 0.002, .005, .01, .025, .05, .1, .25, .5, 1, 2.5}
	flowMetricLables = []string{
		"source_site_id",
		"dest_site_id",
		"source_site_name",
		"dest_site_name",
		"routing_key",
		"protocol",
		"source_process",
		"dest_process",
	}
	appFlowMetricLables = append(flowMetricLables, "method", "code")
)

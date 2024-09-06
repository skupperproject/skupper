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
		}, flowMetricLabels),
		flowClosedCounter: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "skupper",
			Name:      "connections_closed_total",
			Help:      "Number of connections opened through the application network that have been closed",
		}, flowMetricLabels),
		flowBytesSentCounter: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "skupper",
			Name:      "sent_bytes_total",
			Help:      "Bytes sent through the application network from client to service",
		}, flowMetricLabels),
		flowBytesReceivedCounter: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "skupper",
			Name:      "received_bytes_total",
			Help:      "Bytes sent through the application network back from service to client",
		}, flowMetricLabels),
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
			}, flowMetricLabels),
			legancyLatency: prometheus.NewHistogramVec(prometheus.HistogramOpts{
				Namespace: "legacy",
				Name:      "flow_latency_microseconds",
				Help:      "Time to first byte observed from the listener (client) side",
				//                 1ms,  2 ms, 5ms,  10ms,  100ms,  1s,      10s
				Buckets: []float64{1000, 2000, 5000, 10000, 100000, 1000000, 10000000},
			}, append(flowMetricLabels, "direction")),
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
	flowMetricLabels = []string{
		"source_site_id",
		"dest_site_id",
		"source_site_name",
		"dest_site_name",
		"source_component_id",
		"dest_component_id",
		"source_component_name",
		"dest_component_name",
		"routing_key",
		"protocol",
		"source_process_name",
		"dest_process_name",
	}
	appFlowMetricLables = append(flowMetricLabels, "method", "code")
)

type labelSet struct {
	SourceSiteID        string
	DestSiteID          string
	SourceSiteName      string
	DestSiteName        string
	SourceComponentID   string
	DestComponentID     string
	SourceComponentName string
	DestComponentName   string
	RoutingKey          string
	Protocol            string
	SourceProcess       string
	DestProcess         string
}

func (ls labelSet) asLabels() prometheus.Labels {
	return map[string]string{
		"source_site_id":        ls.SourceSiteID,
		"source_site_name":      ls.SourceSiteName,
		"dest_site_id":          ls.DestSiteID,
		"dest_site_name":        ls.DestSiteName,
		"source_component_id":   ls.SourceComponentID,
		"source_component_name": ls.SourceComponentName,
		"dest_component_id":     ls.DestComponentID,
		"dest_component_name":   ls.DestComponentName,
		"routing_key":           ls.RoutingKey,
		"protocol":              ls.Protocol,
		"source_process_name":   ls.SourceProcess,
		"dest_process_name":     ls.DestProcess,
	}
}

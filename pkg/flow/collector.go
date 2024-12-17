package flow

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/pkg/messaging"
	"github.com/skupperproject/skupper/pkg/version"
	"k8s.io/client-go/kubernetes"
)

type senderDirect struct {
	sender    *sender
	outgoing  chan interface{}
	heartbeat bool
}

type ApiRequest struct {
	RecordType  int
	HandlerName string
	Request     *http.Request
}

type ApiResponse struct {
	Body   *string
	Status int
}

type eventSource struct {
	EventSourceRecord
	receivers []*receiver
	send      *senderDirect
}

type collectorMetrics struct {
	info            *prometheus.GaugeVec
	collectorOctets prometheus.Counter
	flows           *prometheus.CounterVec
	octets          *prometheus.CounterVec
	httpReqsMethod  *prometheus.CounterVec
	httpReqsResult  *prometheus.CounterVec
	activeFlows     *prometheus.GaugeVec
	lastAccessed    *prometheus.GaugeVec
	flowLatency     *prometheus.HistogramVec
	activeReconcile *prometheus.GaugeVec
	apiQueryLatency *prometheus.HistogramVec
	activeLinks     *prometheus.GaugeVec
	activeRouters   prometheus.Gauge
	activeSites     prometheus.Gauge
}

func (fc *FlowCollector) NewMetrics(reg prometheus.Registerer) *collectorMetrics {
	m := &collectorMetrics{
		info: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "skupper_info",
				Help: "Skupper deployment information",
			},
			[]string{"version"}),
		collectorOctets: prometheus.NewCounter(
			prometheus.CounterOpts{
				Name: "collector_octets_total",
				Help: "The total number of record octets received by collector",
			}),
		flows: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "flows_total",
				Help: "Total Flows",
			},
			[]string{"sourceSite", "destSite", "address", "protocol", "direction", "sourceProcess", "destProcess", "sourceHost", "destHost"}),
		octets: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "octets_total",
				Help: "Total Octets",
			},
			[]string{"sourceSite", "destSite", "address", "protocol", "direction", "sourceProcess", "destProcess", "sourceHost", "destHost"}),
		httpReqsMethod: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "http_requests_method_total",
				Help: "How many HTTP requests processed, partitioned by method",
			},
			[]string{"sourceSite", "destSite", "address", "protocol", "direction", "sourceProcess", "destProcess", "method", "sourceHost", "destHost"}),
		httpReqsResult: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "http_requests_result_total",
				Help: "How many HTTP requests processed, partitioned by result code",
			},
			[]string{"sourceSite", "destSite", "address", "protocol", "direction", "sourceProcess", "destProcess", "code", "sourceHost", "destHost"}),
		activeFlows: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "active_flows",
				Help: "Number of flows that are currently active, partitioned by source and destination",
			},
			[]string{"sourceSite", "destSite", "address", "protocol", "direction", "sourceProcess", "destProcess", "sourceHost", "destHost"}),
		lastAccessed: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "address_last_time_seconds",
				Help: "The last time the address was served",
			},
			[]string{"sourceSite", "destSite", "address", "protocol", "direction", "sourceProcess", "destProcess", "sourceHost", "destHost"}),
		flowLatency: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name: "flow_latency_microseconds",
				Help: "The measure latency for the direction of flow",
				//                 1ms,  2 ms, 5ms,  10ms,  100ms,  1s,      10s
				Buckets: []float64{1000, 2000, 5000, 10000, 100000, 1000000, 10000000},
			},
			[]string{"sourceSite", "destSite", "address", "protocol", "direction", "sourceProcess", "destProcess", "sourceHost", "destHost"}),
		activeReconcile: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "active_reconciles",
				Help: "Number of active reconcile tasks, partitioned by type",
			},
			[]string{"reconcileTask"}),
		apiQueryLatency: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name: "api_query_latency_microseconds",
				Help: "The measure latency for the direction of query to api",
				//                 10us,100us, 1ms,  2 ms, 5ms,  10ms,  100ms,  1s,      10s
				Buckets: []float64{10, 100, 1000, 2000, 5000, 10000, 100000, 1000000, 10000000},
			},
			[]string{"recordType", "handler"}),
		activeLinks: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "active_links",
				Help: "Number of active links by site and direction",
			}, []string{"sourceSite", "direction"},
		),
		activeRouters: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "active_routers",
				Help: "Number of routers",
			},
		),
		activeSites: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "active_sites",
				Help: "Number of sites attached to the network",
			},
		),
	}
	reg.MustRegister(m.info)
	reg.MustRegister(m.collectorOctets)
	reg.MustRegister(m.flows)
	reg.MustRegister(m.octets)
	reg.MustRegister(m.httpReqsMethod)
	reg.MustRegister(m.httpReqsResult)
	reg.MustRegister(m.activeFlows)
	reg.MustRegister(m.lastAccessed)
	reg.MustRegister(m.flowLatency)
	reg.MustRegister(m.activeReconcile)
	reg.MustRegister(m.apiQueryLatency)
	reg.MustRegister(m.activeLinks)
	reg.MustRegister(m.activeRouters)
	reg.MustRegister(m.activeSites)
	return m

}

type FlowToPairRecord struct {
	forwardId string
	created   uint64
}

type CollectorMode int

const (
	RecordStatus CollectorMode = iota
	RecordMetrics
)

type FlowCollectorSpec struct {
	Mode                CollectorMode
	Namespace           string
	Origin              string
	PromReg             prometheus.Registerer
	ConnectionFactory   messaging.ConnectionFactory
	FlowRecordTtl       time.Duration
	NetworkStatusClient kubernetes.Interface
}

type FlowCollector struct {
	kubeclient              kubernetes.Interface
	mode                    CollectorMode
	origin                  string
	namespace               string
	startTime               uint64
	Collector               CollectorRecord
	connectionFactory       messaging.ConnectionFactory
	recordTtl               time.Duration
	prometheusReg           prometheus.Registerer
	metrics                 *collectorMetrics
	beaconsIncoming         chan []interface{}
	heartbeatsIncoming      chan []interface{}
	recordsIncoming         chan []interface{}
	Request                 chan ApiRequest
	Response                chan ApiResponse
	eventSources            map[string]*eventSource
	beaconReceiver          *receiver
	pendingFlush            map[string]*senderDirect
	Beacons                 map[string]*BeaconRecord
	Sites                   map[string]*SiteRecord
	Hosts                   map[string]*HostRecord
	Routers                 map[string]*RouterRecord
	Links                   map[string]*LinkRecord
	Listeners               map[string]*ListenerRecord
	Connectors              map[string]*ConnectorRecord
	recentConnectors        map[string]*ConnectorRecord
	Flows                   map[string]*FlowRecord
	FlowPairs               map[string]*FlowPairRecord
	FlowAggregates          map[string]*FlowAggregateRecord
	Processes               map[string]*ProcessRecord
	ProcessGroups           map[string]*ProcessGroupRecord
	VanAddresses            map[string]*VanAddressRecord
	flowsToProcessReconcile map[string]string
	flowsToPairReconcile    map[string]*FlowToPairRecord
	connectorsToReconcile   map[string]string
	processesToReconcile    map[string]*ProcessRecord
	aggregatesToReconcile   map[string]*FlowPairRecord

	begin           time.Time
	networkStatusUp bool
}

func getTtl(ttl time.Duration) time.Duration {
	if ttl == 0 {
		return types.DefaultFlowTimeoutDuration
	}
	if ttl < time.Minute {
		return time.Minute
	}
	return ttl
}

func NewFlowCollector(spec FlowCollectorSpec) *FlowCollector {
	fc := &FlowCollector{
		kubeclient:              spec.NetworkStatusClient,
		mode:                    spec.Mode,
		namespace:               spec.Namespace,
		origin:                  spec.Origin,
		startTime:               uint64(time.Now().UnixNano()) / uint64(time.Microsecond),
		connectionFactory:       spec.ConnectionFactory,
		recordTtl:               getTtl(spec.FlowRecordTtl),
		prometheusReg:           spec.PromReg,
		beaconsIncoming:         make(chan []interface{}, 10),
		heartbeatsIncoming:      make(chan []interface{}, 10),
		recordsIncoming:         make(chan []interface{}, 10),
		Request:                 make(chan ApiRequest),
		Response:                make(chan ApiResponse),
		eventSources:            make(map[string]*eventSource),
		pendingFlush:            make(map[string]*senderDirect),
		Beacons:                 make(map[string]*BeaconRecord),
		Sites:                   make(map[string]*SiteRecord),
		Hosts:                   make(map[string]*HostRecord),
		Routers:                 make(map[string]*RouterRecord),
		Links:                   make(map[string]*LinkRecord),
		Listeners:               make(map[string]*ListenerRecord),
		Connectors:              make(map[string]*ConnectorRecord),
		recentConnectors:        make(map[string]*ConnectorRecord),
		Flows:                   make(map[string]*FlowRecord),
		FlowPairs:               make(map[string]*FlowPairRecord),
		FlowAggregates:          make(map[string]*FlowAggregateRecord),
		VanAddresses:            make(map[string]*VanAddressRecord),
		Processes:               make(map[string]*ProcessRecord),
		ProcessGroups:           make(map[string]*ProcessGroupRecord),
		flowsToProcessReconcile: make(map[string]string),
		flowsToPairReconcile:    make(map[string]*FlowToPairRecord),
		connectorsToReconcile:   make(map[string]string),
		processesToReconcile:    make(map[string]*ProcessRecord),
		aggregatesToReconcile:   make(map[string]*FlowPairRecord),
	}
	fc.Collector = CollectorRecord{
		Base: Base{
			RecType:   recordNames[Collector],
			Identity:  uuid.New().String(),
			Parent:    spec.Origin,
			StartTime: uint64(time.Now().UnixNano()) / uint64(time.Microsecond),
		},
	}
	return fc
}

func (fc *FlowCollector) serveRecords(request ApiRequest) ApiResponse {
	request.HandlerName = mux.CurrentRoute(request.Request).GetName()
	response := ApiResponse{
		Body:   nil,
		Status: http.StatusOK,
	}
	result, err := fc.retrieve(request)
	if err == nil {
		response.Body = result
	} else {
		response.Status = http.StatusInternalServerError
	}
	return response
}

func getRealSizeOf(v interface{}) (int, error) {
	b := new(bytes.Buffer)
	if err := gob.NewEncoder(b).Encode(v); err != nil {
		return 0, err
	}
	return b.Len(), nil
}

func (c *FlowCollector) beaconUpdate(beacon BeaconRecord) {
	if source, ok := c.eventSources[beacon.Identity]; !ok {
		var receivers []*receiver
		log.Printf("COLLECTOR: Detected event source %s of type %s \n", beacon.Identity, beacon.SourceType)
		receivers = append(receivers, newReceiver(c.connectionFactory, beacon.Address, c.recordsIncoming))
		if beacon.SourceType == recordNames[Router] {
			switch c.mode {
			case RecordMetrics:
				receivers = append(receivers, newReceiver(c.connectionFactory, beacon.Address+".flows", c.recordsIncoming))
			case RecordStatus:
				receivers = append(receivers, newReceiver(c.connectionFactory, beacon.Address+".logs", c.recordsIncoming))
			}
		} else if beacon.SourceType == recordNames[Controller] {
			receivers = append(receivers, newReceiver(c.connectionFactory, beacon.Address+".heartbeats", c.heartbeatsIncoming))
		}
		outgoing := make(chan interface{})
		s := newSender(c.connectionFactory, beacon.Direct, false, outgoing)
		if c.connectionFactory != nil {
			s.start()
		}
		now := uint64(time.Now().UnixNano()) / uint64(time.Microsecond)
		c.eventSources[beacon.Identity] = &eventSource{
			EventSourceRecord: EventSourceRecord{
				Base: Base{
					RecType:   recordNames[EventSource],
					Identity:  beacon.Identity,
					StartTime: now,
					EndTime:   0,
				},
				Beacon:    &beacon,
				LastHeard: now,
				Beacons:   1,
			},
			receivers: receivers,
			send: &senderDirect{
				sender:    s,
				outgoing:  outgoing,
				heartbeat: false,
			},
		}
		if c.connectionFactory != nil {
			for _, receiver := range receivers {
				receiver.start()
			}
		}
		c.pendingFlush[beacon.Direct] = c.eventSources[beacon.Identity].send
	} else {
		source.LastHeard = uint64(time.Now().UnixNano()) / uint64(time.Microsecond)
		source.Beacons++
	}
}

func (c *FlowCollector) recordUpdates(stopCh <-chan struct{}) {
	tickerFlush := time.NewTicker(250 * time.Millisecond)
	defer tickerFlush.Stop()
	tickerReconcile := time.NewTicker(2 * time.Second)
	defer tickerReconcile.Stop()
	tickerAge := time.NewTicker(5 * time.Second)
	defer tickerAge.Stop()

	for {
		select {
		case beaconUpdates := <-c.beaconsIncoming:
			for _, beaconUpdate := range beaconUpdates {
				beacon, ok := beaconUpdate.(BeaconRecord)
				if !ok {
					log.Println("COLLECTOR: Unable to convert interface to beacon")
				} else {
					c.beaconUpdate(beacon)
				}
			}
		case heartbeatUpdates := <-c.heartbeatsIncoming:
			for _, heartbeatUpdate := range heartbeatUpdates {
				heartbeat, ok := heartbeatUpdate.(HeartbeatRecord)
				if !ok {
					log.Println("COLLECTOR: Unable to convert interface to heartbeat")
				} else {
					err := c.updateRecord(heartbeat)
					if err != nil {
						log.Println("COLLECTOR: heartbeat record error", err.Error())
					}
				}
			}
		case recordUpdates := <-c.recordsIncoming:
			for _, update := range recordUpdates {
				size, _ := getRealSizeOf(update)
				if c.mode == RecordMetrics {
					c.metrics.collectorOctets.Add(float64(size))
				}
				err := c.updateRecord(update)
				if err != nil {
					log.Println("COLLECTOR: Update record error", err.Error())
				}
			}
		case request := <-c.Request:
			response := c.serveRecords(request)
			c.Response <- response
		case <-tickerFlush.C:
			for address, sender := range c.pendingFlush {
				if sender.heartbeat {
					log.Println("COLLECTOR: Sending flush to ", address)
					sender.outgoing <- &FlushRecord{Address: address}
					delete(c.pendingFlush, address)
				}
			}
		case <-tickerReconcile.C:
			if c.mode == RecordMetrics {
				c.reconcileFlowRecords()
			}
			c.reconcileConnectorRecords()
		case <-tickerAge.C:
			c.ageAndPurgeRecords()
		case <-stopCh:
			return
		}
	}
}

func (c *FlowCollector) Start(stopCh <-chan struct{}) {
	c.begin = time.Now()
	go c.run(stopCh)
}

// PrimeSiteBeacons "sends" the collector phony beacon records for a controller
// and a router event source when their addresses are known in order to avoid
// the startup time waiting for beacon records.
func (c *FlowCollector) PrimeSiteBeacons(controllerID, routerID string) {
	var incoming []interface{}
	if controllerID != "" {
		incoming = append(incoming, BeaconRecord{
			Version:    1,
			SourceType: "CONTROLLER",
			Address:    fmt.Sprintf("mc/sfe.%s", controllerID),
			Direct:     fmt.Sprintf("sfe.%s", controllerID),
			Identity:   controllerID,
		})
	}
	if routerID != "" {
		incoming = append(incoming, BeaconRecord{
			Version:    1,
			SourceType: "ROUTER",
			Address:    fmt.Sprintf("mc/sfe.%s", routerID),
			Direct:     fmt.Sprintf("sfe.%s", routerID),
			Identity:   routerID,
		})
	}
	if len(incoming) == 0 {
		return
	}
	c.beaconsIncoming <- incoming
}

func (c *FlowCollector) run(stopCh <-chan struct{}) {
	if c.mode == RecordMetrics {
		c.metrics = c.NewMetrics(c.prometheusReg)
		c.metrics.info.With(prometheus.Labels{"version": version.Version}).Set(1)
	}
	c.beaconReceiver = newReceiver(c.connectionFactory, BeaconAddress, c.beaconsIncoming)
	c.beaconReceiver.start()

	done := make(chan struct{})
	go func() {
		defer close(done)
		c.recordUpdates(stopCh)
	}()
	<-done
	log.Println("COLLECTOR: Finished running. Shutting down")
	for _, eventsource := range c.eventSources {
		for _, receiver := range eventsource.receivers {
			receiver.stop()
		}
		eventsource.send.sender.stop()
	}
	c.beaconReceiver.stop()
}

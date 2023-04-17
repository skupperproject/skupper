package flow

import (
	"bytes"
	"encoding/gob"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/pkg/messaging"
	"github.com/skupperproject/skupper/pkg/version"
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
	nonFlowReceiver *receiver
	flowReceiver    *receiver
	send            *senderDirect
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
			[]string{"sourceSite", "destSite", "address", "protocol", "direction", "sourceProcess", "destProcess"}),
		octets: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "octets_total",
				Help: "Total Octets",
			},
			[]string{"sourceSite", "destSite", "address", "protocol", "direction", "sourceProcess", "destProcess"}),
		httpReqsMethod: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "http_requests_method_total",
				Help: "How many HTTP requests processed, partitioned by method",
			},
			[]string{"sourceSite", "destSite", "address", "protocol", "direction", "sourceProcess", "destProcess", "method"}),
		httpReqsResult: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "http_requests_result_total",
				Help: "How many HTTP requests processed, partitioned by result code",
			},
			[]string{"sourceSite", "destSite", "address", "protocol", "direction", "sourceProcess", "destProcess", "code"}),
		activeFlows: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "active_flows",
				Help: "Number of flows that are currently active, partititioned by source and destination",
			},
			[]string{"sourceSite", "destSite", "address", "protocol", "direction", "sourceProcess", "destProcess"}),
		lastAccessed: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "address_last_time_seconds",
				Help: "The last time the address was served",
			},
			[]string{"sourceSite", "destSite", "address", "protocol", "direction", "sourceProcess", "destProcess"}),
		flowLatency: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name: "flow_latency_microseconds",
				Help: "The measure latency for the direction of flow",
				//                 1ms,  2 ms, 5ms,  10ms,  100ms,  1s,      10s
				Buckets: []float64{1000, 2000, 5000, 10000, 100000, 1000000, 10000000},
			},
			[]string{"sourceSite", "destSite", "address", "protocol", "direction", "sourceProcess", "destProcess"}),
		activeReconcile: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "active_reconciles",
				Help: "Number of active reconcile tasks, partitione by type",
			},
			[]string{"reconcileTask"}),
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
	return m

}

type FlowCollector struct {
	origin                  string
	Collector               CollectorRecord
	connectionFactory       messaging.ConnectionFactory
	recordTtl               time.Duration
	prometheusReg           prometheus.Registerer
	metrics                 *collectorMetrics
	beaconsIncoming         chan []interface{}
	recordsIncoming         chan []interface{}
	Request                 chan ApiRequest
	Response                chan ApiResponse
	eventSources            map[string]*eventSource
	receivers               map[string]*receiver
	senders                 map[string]*senderDirect
	pendingFlush            map[string]*senderDirect
	Beacons                 map[string]*BeaconRecord
	Sites                   map[string]*SiteRecord
	Hosts                   map[string]*HostRecord
	Routers                 map[string]*RouterRecord
	Links                   map[string]*LinkRecord
	Listeners               map[string]*ListenerRecord
	Connectors              map[string]*ConnectorRecord
	Flows                   map[string]*FlowRecord
	FlowPairs               map[string]*FlowPairRecord
	FlowAggregates          map[string]*FlowAggregateRecord
	Processes               map[string]*ProcessRecord
	ProcessGroups           map[string]*ProcessGroupRecord
	VanAddresses            map[string]*VanAddressRecord
	routersToSiteReconcile  map[string]string
	flowsToProcessReconcile map[string]string
	flowsToPairReconcile    map[string]string
	connectorsToReconcile   map[string]string
	processesToReconcile    map[string]*ProcessRecord
	aggregatesToReconcile   map[string]*FlowPairRecord
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

func NewFlowCollector(origin string, reg prometheus.Registerer, connectionFactory messaging.ConnectionFactory, recordTtl time.Duration) *FlowCollector {
	fc := &FlowCollector{
		origin:                  origin,
		connectionFactory:       connectionFactory,
		recordTtl:               getTtl(recordTtl),
		prometheusReg:           reg,
		beaconsIncoming:         make(chan []interface{}),
		recordsIncoming:         make(chan []interface{}),
		Request:                 make(chan ApiRequest),
		Response:                make(chan ApiResponse),
		eventSources:            make(map[string]*eventSource),
		receivers:               make(map[string]*receiver),
		senders:                 make(map[string]*senderDirect),
		pendingFlush:            make(map[string]*senderDirect),
		Beacons:                 make(map[string]*BeaconRecord),
		Sites:                   make(map[string]*SiteRecord),
		Hosts:                   make(map[string]*HostRecord),
		Routers:                 make(map[string]*RouterRecord),
		Links:                   make(map[string]*LinkRecord),
		Listeners:               make(map[string]*ListenerRecord),
		Connectors:              make(map[string]*ConnectorRecord),
		Flows:                   make(map[string]*FlowRecord),
		FlowPairs:               make(map[string]*FlowPairRecord),
		FlowAggregates:          make(map[string]*FlowAggregateRecord),
		VanAddresses:            make(map[string]*VanAddressRecord),
		Processes:               make(map[string]*ProcessRecord),
		ProcessGroups:           make(map[string]*ProcessGroupRecord),
		routersToSiteReconcile:  make(map[string]string),
		flowsToProcessReconcile: make(map[string]string),
		flowsToPairReconcile:    make(map[string]string),
		connectorsToReconcile:   make(map[string]string),
		processesToReconcile:    make(map[string]*ProcessRecord),
		aggregatesToReconcile:   make(map[string]*FlowPairRecord),
	}
	fc.Collector = CollectorRecord{
		Base: Base{
			RecType:   recordNames[Collector],
			Identity:  uuid.New().String(),
			Parent:    origin,
			StartTime: uint64(time.Now().UnixNano()) / uint64(time.Microsecond),
		},
	}
	return fc
}

func listToJSON(list []interface{}) *string {
	data, err := json.MarshalIndent(list, "", " ")
	if err == nil {
		sd := string(data)
		return &sd
	}
	return nil
}

func itemToJSON(item interface{}) *string {
	data, err := json.Marshal(item)
	if err == nil {
		sd := string(data)
		return &sd
	}
	return nil
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

func (c *FlowCollector) updates(stopCh <-chan struct{}) {
	tickerAge := time.NewTicker(10 * time.Second)
	defer tickerAge.Stop()
	tickerReconcile := time.NewTicker(5 * time.Second)
	defer tickerReconcile.Stop()

	for {
		select {
		case beaconUpdates := <-c.beaconsIncoming:
			for _, beaconUpdate := range beaconUpdates {
				beacon, ok := beaconUpdate.(BeaconRecord)
				if !ok {
					log.Println("Unable to convert interface to beacon")
				} else {
					if source, ok := c.eventSources[beacon.Identity]; !ok {
						log.Printf("Detected event source %s of type %s \n", beacon.Identity, beacon.SourceType)
						nonFlowReceiver := newReceiver(c.connectionFactory, beacon.Address, c.recordsIncoming)
						nonFlowReceiver.start()
						var flowReceiver *receiver = nil
						if beacon.SourceType == recordNames[Router] {
							flowReceiver = newReceiver(c.connectionFactory, beacon.Address+".flows", c.recordsIncoming)
							flowReceiver.start()
						}
						outgoing := make(chan interface{})
						s := newSender(c.connectionFactory, beacon.Direct, outgoing)
						s.start()
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
							nonFlowReceiver: nonFlowReceiver,
							flowReceiver:    flowReceiver,
							send: &senderDirect{
								sender:    s,
								outgoing:  outgoing,
								heartbeat: false,
							},
						}
						c.pendingFlush[beacon.Direct] = c.eventSources[beacon.Identity].send
					} else {
						source.LastHeard = uint64(time.Now().UnixNano()) / uint64(time.Microsecond)
						source.Beacons++
					}
				}
			}
		case recordUpdates := <-c.recordsIncoming:
			for _, update := range recordUpdates {
				size, _ := getRealSizeOf(update)
				c.metrics.collectorOctets.Add(float64(size))
				err := c.updateRecord(update)
				if err != nil {
					log.Println("Update record error", err.Error())
				}
			}
		case request := <-c.Request:
			response := c.serveRecords(request)
			c.Response <- response
		case <-tickerReconcile.C:
			c.reconcileRecords()
		case <-tickerAge.C:
			for address, sender := range c.pendingFlush {
				if sender.heartbeat {
					log.Println("Sending flush to: ", address)
					sender.outgoing <- &FlushRecord{Address: address}
					delete(c.pendingFlush, address)
				}
			}
		case <-stopCh:
			return
		}
	}
}

func (c *FlowCollector) Start(stopCh <-chan struct{}) {
	go c.run(stopCh)
}

func (c *FlowCollector) run(stopCh <-chan struct{}) {
	c.metrics = c.NewMetrics(c.prometheusReg)
	c.metrics.info.With(prometheus.Labels{"version": version.Version}).Set(1)
	r := newReceiver(c.connectionFactory, BeaconAddress, c.beaconsIncoming)
	c.receivers[BeaconAddress] = r
	r.start()
	c.updates(stopCh)
	<-stopCh
	for _, eventsource := range c.eventSources {
		log.Println("Stopping receiver and sender for: ", eventsource.Identity)
		eventsource.nonFlowReceiver.stop()
		if eventsource.flowReceiver != nil {
			eventsource.flowReceiver.stop()
		}
		eventsource.send.sender.stop()
	}
}

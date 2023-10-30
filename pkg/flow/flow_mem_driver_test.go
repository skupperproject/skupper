package flow

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus"
	"gotest.tools/assert"
)

func TestRecordGraphWithMetrics(t *testing.T) {
	name := "skupper-site"
	namespace := "skupper-public"
	provider := "aws"
	address := "tcp-go-echo"
	address2 := "redis-cart"
	address3 := "mongo"
	address4 := "cartservice"
	protocol := "tcp"
	counterFlow1 := "flow:0"
	counterFlow2 := "flow:2"
	counterFlow4 := "flow:4"
	flowTrace := "router:0|router:1"
	processName1 := "checkout-1234"
	processName2 := "payment-1234"
	groupName1 := "online-store"
	listenerProcess := "process:0"
	connectorProcess1 := "process:1"
	connectorProcess2 := "process:2"
	processConnector := "connector:0"

	sites := []SiteRecord{
		{
			Base: Base{
				RecType:   recordNames[Site],
				Identity:  "site:0",
				StartTime: uint64(time.Now().UnixNano()) / uint64(time.Microsecond),
			},
			Name:      &name,
			NameSpace: &namespace,
			Provider:  &provider,
		},
	}
	routers := []RouterRecord{
		{
			Base: Base{
				RecType:   recordNames[Router],
				Identity:  "router:0",
				Parent:    "site:0",
				StartTime: uint64(time.Now().UnixNano()) / uint64(time.Microsecond),
			},
			Name: &name,
		},
		{
			Base: Base{
				RecType:   recordNames[Router],
				Identity:  "router:1",
				Parent:    "site:0",
				StartTime: uint64(time.Now().UnixNano()) / uint64(time.Microsecond),
			},
			Name: &name,
		},
	}
	hosts := []HostRecord{
		{
			Base: Base{
				RecType:   recordNames[Host],
				Identity:  "host:0",
				Parent:    "site:0",
				StartTime: uint64(time.Now().UnixNano()) / uint64(time.Microsecond),
			},
		},
		{
			Base: Base{
				RecType:   recordNames[Host],
				Identity:  "host:1",
				Parent:    "site:0",
				StartTime: uint64(time.Now().UnixNano()) / uint64(time.Microsecond),
			},
		},
		{
			Base: Base{
				RecType:   recordNames[Host],
				Identity:  "host:2",
				Parent:    "site:0",
				StartTime: uint64(time.Now().UnixNano()) / uint64(time.Microsecond),
			},
		},
	}
	links := []LinkRecord{
		{
			Base: Base{
				RecType:   recordNames[Link],
				Identity:  "link:0",
				Parent:    "router:0",
				StartTime: uint64(time.Now().UnixNano()) / uint64(time.Microsecond),
			},
		},
		{
			Base: Base{
				RecType:   recordNames[Link],
				Identity:  "link:1",
				Parent:    "router:1",
				StartTime: uint64(time.Now().UnixNano()) / uint64(time.Microsecond),
			},
		},
	}
	processes := []ProcessRecord{
		{
			Base: Base{
				RecType:   recordNames[Process],
				Identity:  "process:0",
				Parent:    "site:0",
				StartTime: uint64(time.Now().UnixNano()) / uint64(time.Microsecond),
			},
			Name:      &processName1,
			GroupName: &groupName1,
		},
		{
			Base: Base{
				RecType:   recordNames[Process],
				Identity:  "process:1",
				Parent:    "site:0",
				StartTime: uint64(time.Now().UnixNano()) / uint64(time.Microsecond),
			},
			Name:      &processName2,
			GroupName: &groupName1,
			connector: &processConnector,
		},
		{
			Base: Base{
				RecType:   recordNames[Process],
				Identity:  "process:2",
				Parent:    "site:0",
				StartTime: uint64(time.Now().UnixNano()) / uint64(time.Microsecond),
			},
			Name:      &processName1,
			GroupName: &groupName1,
		},
	}
	listeners := []ListenerRecord{
		{
			Base: Base{
				RecType:   recordNames[Listener],
				Identity:  "listener:0",
				Parent:    "router:0",
				StartTime: uint64(time.Now().UnixNano()) / uint64(time.Microsecond),
			},
			Address:  &address,
			Protocol: &protocol,
		},
		{
			Base: Base{
				RecType:   recordNames[Listener],
				Identity:  "listener:1",
				Parent:    "router:1",
				StartTime: uint64(time.Now().UnixNano()) / uint64(time.Microsecond),
			},
			Protocol: &protocol,
		},
		{
			Base: Base{
				RecType:   recordNames[Listener],
				Identity:  "listener:2",
				Parent:    "router:1",
				StartTime: uint64(time.Now().UnixNano()) / uint64(time.Microsecond),
			},
			Protocol: &protocol,
			Address:  &address2,
		},
	}
	connectors := []ConnectorRecord{
		{
			Base: Base{
				RecType:   recordNames[Connector],
				Identity:  "connector:0",
				Parent:    "router:0",
				StartTime: uint64(time.Now().UnixNano()) / uint64(time.Microsecond),
			},
			Address:   &address3,
			Protocol:  &protocol,
			ProcessId: &connectorProcess1,
		},
		{
			Base: Base{
				RecType:   recordNames[Connector],
				Identity:  "connector:1",
				Parent:    "router:1",
				StartTime: uint64(time.Now().UnixNano()) / uint64(time.Microsecond),
			},
			Protocol: &protocol,
		},
		{
			Base: Base{
				RecType:   recordNames[Connector],
				Identity:  "connector:1",
				Parent:    "router:1",
				StartTime: uint64(time.Now().UnixNano()) / uint64(time.Microsecond),
			},
			Protocol:  &protocol,
			Address:   &address4,
			ProcessId: &connectorProcess2,
		},
	}
	flows := []FlowRecord{
		{
			Base: Base{
				RecType:   recordNames[Flow],
				Identity:  "flow:0",
				Parent:    "listener:0",
				StartTime: uint64(time.Now().UnixNano()) / uint64(time.Microsecond),
			},
			ProcessName: &processName1,
			Process:     &listenerProcess,
		},
		{
			Base: Base{
				RecType:   recordNames[Flow],
				Identity:  "flow:1",
				Parent:    "connector:0",
				StartTime: uint64(time.Now().UnixNano()) / uint64(time.Microsecond),
			},
			CounterFlow: &counterFlow1,
			Trace:       &flowTrace,
			ProcessName: &processName2,
			Process:     &connectorProcess1,
		},
		{
			Base: Base{
				RecType:   recordNames[Flow],
				Identity:  "flow:2",
				Parent:    "listener:0",
				StartTime: uint64(time.Now().UnixNano()) / uint64(time.Microsecond),
			},
			ProcessName: &processName1,
			Process:     &listenerProcess,
		},
		{
			Base: Base{
				RecType:   recordNames[Flow],
				Identity:  "flow:3",
				Parent:    "connector:0",
				StartTime: uint64(time.Now().UnixNano()) / uint64(time.Microsecond),
			},
			CounterFlow: &counterFlow2,
			ProcessName: &processName2,
			Process:     &connectorProcess1,
		},
		{
			Base: Base{
				RecType:   recordNames[Flow],
				Identity:  "flow:2",
				Parent:    "listener:0",
				StartTime: uint64(time.Now().UnixNano()) / uint64(time.Microsecond),
				EndTime:   uint64(time.Now().UnixNano()) / uint64(time.Microsecond),
			},
			ProcessName: &processName1,
		},
		{
			Base: Base{
				RecType:   recordNames[Flow],
				Identity:  "flow:4",
				Parent:    "flow:0",
				StartTime: uint64(time.Now().UnixNano()) / uint64(time.Microsecond),
			},
			ProcessName: &processName1,
			Process:     &listenerProcess,
		},
		{
			Base: Base{
				RecType:   recordNames[Flow],
				Identity:  "flow:5",
				Parent:    "flow:1",
				StartTime: uint64(time.Now().UnixNano()) / uint64(time.Microsecond),
			},
			CounterFlow: &counterFlow4,
			ProcessName: &processName2,
			Trace:       &flowTrace,
			Process:     &connectorProcess1,
		},
	}

	reg := prometheus.NewRegistry()
	u, _ := time.ParseDuration("5m")
	fc := NewFlowCollector(FlowCollectorSpec{
		Mode:              RecordMetrics,
		Origin:            "origin",
		PromReg:           reg,
		ConnectionFactory: nil,
		FlowRecordTtl:     u,
	})
	fc.metrics = fc.NewMetrics(fc.prometheusReg)
	for _, s := range sites {
		err := fc.updateRecord(s)
		assert.Assert(t, err)
		err = fc.updateRecord(s)
		assert.Assert(t, err)
	}
	for _, r := range routers {
		err := fc.updateRecord(r)
		assert.Assert(t, err)
		err = fc.updateRecord(r)
		assert.Assert(t, err)
	}
	for _, h := range hosts {
		err := fc.updateRecord(h)
		assert.Assert(t, err)
		err = fc.updateRecord(h)
		assert.Assert(t, err)
	}
	for _, l := range links {
		err := fc.updateRecord(l)
		assert.Assert(t, err)
		err = fc.updateRecord(l)
		assert.Assert(t, err)
	}
	for _, p := range processes {
		err := fc.updateRecord(p)
		assert.Assert(t, err)
		err = fc.updateRecord(p)
		assert.Assert(t, err)
	}
	for _, l := range listeners {
		err := fc.updateRecord(l)
		assert.Assert(t, err)
		err = fc.updateRecord(l)
		assert.Assert(t, err)
	}
	for _, c := range connectors {
		err := fc.updateRecord(c)
		assert.Assert(t, err)
		err = fc.updateRecord(c)
		assert.Assert(t, err)
	}
	for _, f := range flows {
		err := fc.updateRecord(f)
		assert.Assert(t, err)
		err = fc.updateRecord(f)
		assert.Assert(t, err)
	}

	for _, s := range sites {
		id := fc.getRecordSiteId(s)
		assert.Equal(t, id, s.Identity)
	}
	for _, r := range routers {
		id := fc.getRecordSiteId(r)
		assert.Equal(t, id, r.Parent)
	}
	for _, h := range hosts {
		id := fc.getRecordSiteId(h)
		assert.Equal(t, id, h.Parent)
	}
	for _, l := range links {
		id := fc.getRecordSiteId(l)
		assert.Equal(t, id, "site:0")
	}
	for _, p := range processes {
		id := fc.getRecordSiteId(p)
		assert.Equal(t, id, p.Parent)
	}
	for _, l := range listeners {
		id := fc.getRecordSiteId(l)
		assert.Equal(t, id, "site:0")
	}
	for _, l := range connectors {
		id := fc.getRecordSiteId(l)
		assert.Equal(t, id, "site:0")
	}
	for _, f := range flows {
		if f.Parent != "" {
			id := fc.getRecordSiteId(f)
			assert.Equal(t, id, "site:0")
		}
		if f.Trace != nil {
			trace := fc.annotateFlowTrace(&f)
			assert.Equal(t, *trace, "skupper-site@skupper-site")
		}
	}

	fc.reconcileConnectorRecords()
	fc.reconcileFlowRecords()

	type test struct {
		recordType   int
		method       string
		url          string
		params       map[string]string
		vars         map[string]string
		name         string
		responseSize int
	}

	testTable := []test{
		{
			recordType:   Site,
			method:       "Get",
			url:          "/",
			params:       map[string]string{"sortBy": "identity.asc"},
			vars:         map[string]string{},
			name:         "list",
			responseSize: len(sites),
		},
		{
			recordType:   Site,
			method:       "Get",
			url:          "/",
			params:       map[string]string{"sortBy": "identity.asc"},
			vars:         map[string]string{"id": "site:0"},
			name:         "item",
			responseSize: 1,
		},
		{
			recordType:   Site,
			method:       "Get",
			url:          "/",
			params:       map[string]string{"sortBy": "identity.asc"},
			vars:         map[string]string{"id": "site:0"},
			name:         "routers",
			responseSize: len(routers),
		},
		{
			recordType:   Site,
			method:       "Get",
			url:          "/",
			params:       map[string]string{"sortBy": "identity.asc"},
			vars:         map[string]string{"id": "site:0"},
			name:         "links",
			responseSize: len(links),
		},
		{
			recordType:   Site,
			method:       "Get",
			url:          "/",
			params:       map[string]string{"sortBy": "identity.asc"},
			vars:         map[string]string{"id": "site:0"},
			name:         "hosts",
			responseSize: len(hosts),
		},
		{
			recordType:   Site,
			method:       "Get",
			url:          "/",
			params:       map[string]string{"sortBy": "identity.asc"},
			vars:         map[string]string{"id": "site:0"},
			name:         "processes",
			responseSize: len(processes),
		},
		{
			recordType:   Host,
			method:       "Get",
			url:          "/",
			params:       map[string]string{"sortBy": "identity.asc"},
			vars:         map[string]string{},
			name:         "list",
			responseSize: len(hosts),
		},
		{
			recordType:   Host,
			method:       "Get",
			url:          "/",
			params:       map[string]string{"sortBy": "identity.asc"},
			vars:         map[string]string{"id": "host:0"},
			name:         "item",
			responseSize: 1,
		},
		{
			recordType:   Router,
			method:       "Get",
			url:          "/",
			params:       map[string]string{"sortBy": "identity.asc"},
			vars:         map[string]string{},
			name:         "list",
			responseSize: len(routers),
		},
		{
			recordType:   Router,
			method:       "Get",
			url:          "/",
			params:       map[string]string{"sortBy": "identity.asc"},
			vars:         map[string]string{"id": "router:0"},
			name:         "item",
			responseSize: 1,
		},
		{
			recordType:   Router,
			method:       "Get",
			url:          "/",
			params:       map[string]string{"sortBy": "identity.asc"},
			vars:         map[string]string{"id": "router:0"},
			name:         "flows",
			responseSize: 6,
		},
		{
			recordType:   Router,
			method:       "Get",
			url:          "/",
			params:       map[string]string{"sortBy": "identity.asc"},
			vars:         map[string]string{"id": "router:0"},
			name:         "links",
			responseSize: 1,
		},
		{
			recordType:   Router,
			method:       "Get",
			url:          "/",
			params:       map[string]string{"sortBy": "identity.asc"},
			vars:         map[string]string{"id": "router:0"},
			name:         "listeners",
			responseSize: 1,
		},
		{
			recordType:   Router,
			method:       "Get",
			url:          "/",
			params:       map[string]string{"sortBy": "identity.asc"},
			vars:         map[string]string{"id": "router:0"},
			name:         "connectors",
			responseSize: 1,
		},
		{
			recordType:   Link,
			method:       "Get",
			url:          "/",
			params:       map[string]string{"sortBy": "identity.asc"},
			vars:         map[string]string{},
			name:         "list",
			responseSize: len(links),
		},
		{
			recordType:   Link,
			method:       "Get",
			url:          "/",
			params:       map[string]string{"sortBy": "identity.asc"},
			vars:         map[string]string{"id": "link:0"},
			name:         "item",
			responseSize: 1,
		},
		{
			recordType:   Listener,
			method:       "Get",
			url:          "/",
			params:       map[string]string{"sortBy": "identity.asc"},
			vars:         map[string]string{},
			name:         "list",
			responseSize: len(listeners),
		},
		{
			recordType:   Listener,
			method:       "Get",
			url:          "/",
			params:       map[string]string{"sortBy": "identity.asc"},
			vars:         map[string]string{"id": "listener:0"},
			name:         "item",
			responseSize: 1,
		},
		{
			recordType:   Listener,
			method:       "Get",
			url:          "/",
			params:       map[string]string{"sortBy": "identity.asc"},
			vars:         map[string]string{"id": "listener:0"},
			name:         "flows",
			responseSize: 3,
		},
		{
			recordType:   Connector,
			method:       "Get",
			url:          "/",
			params:       map[string]string{"sortBy": "identity.asc"},
			vars:         map[string]string{},
			name:         "list",
			responseSize: 2,
		},
		{
			recordType:   Connector,
			method:       "Get",
			url:          "/",
			params:       map[string]string{"sortBy": "identity.asc"},
			vars:         map[string]string{"id": "connector:0"},
			name:         "item",
			responseSize: 1,
		},
		{
			recordType:   Connector,
			method:       "Get",
			url:          "/",
			params:       map[string]string{"sortBy": "identity.asc"},
			vars:         map[string]string{"id": "connector:0"},
			name:         "flows",
			responseSize: 3,
		},
		{
			recordType:   Connector,
			method:       "Get",
			url:          "/",
			params:       map[string]string{"sortBy": "identity.asc"},
			vars:         map[string]string{"id": "connector:0"},
			name:         "process",
			responseSize: 1,
		},
		{
			recordType:   Address,
			method:       "Get",
			url:          "/",
			params:       map[string]string{"sortBy": "identity.asc"},
			vars:         map[string]string{},
			name:         "list",
			responseSize: len(fc.VanAddresses),
		},
		{
			recordType:   Connector,
			method:       "Get",
			url:          "/",
			params:       map[string]string{"sortBy": "identity.asc"},
			vars:         map[string]string{"id": "connector:0"},
			name:         "item",
			responseSize: 1,
		},
		{
			recordType:   Flow,
			method:       "Get",
			url:          "/",
			params:       map[string]string{"sortBy": "identity.asc"},
			vars:         map[string]string{},
			name:         "list",
			responseSize: len(fc.Flows),
		},
		{
			recordType:   Flow,
			method:       "Get",
			url:          "/",
			params:       map[string]string{"sortBy": "identity.asc"},
			vars:         map[string]string{"id": "flow:1"},
			name:         "item",
			responseSize: 1,
		},
		{
			recordType:   Flow,
			method:       "Get",
			url:          "/",
			params:       map[string]string{"sortBy": "identity.asc"},
			vars:         map[string]string{"id": "flow:1"},
			name:         "process",
			responseSize: 1,
		},
		{
			recordType:   FlowPair,
			method:       "Get",
			url:          "/",
			params:       map[string]string{"sortBy": "identity.asc"},
			vars:         map[string]string{},
			name:         "list",
			responseSize: len(fc.FlowPairs),
		},
		{
			recordType:   FlowPair,
			method:       "Get",
			url:          "/",
			params:       map[string]string{"sortBy": "identity.asc"},
			vars:         map[string]string{"id": "fp-flow:0"},
			name:         "item",
			responseSize: 1,
		},
		{
			recordType:   Process,
			method:       "Get",
			url:          "/",
			params:       map[string]string{"sortBy": "identity.asc"},
			vars:         map[string]string{},
			name:         "list",
			responseSize: len(processes),
		},
		{
			recordType:   Process,
			method:       "Get",
			url:          "/",
			params:       map[string]string{"sortBy": "identity.asc"},
			vars:         map[string]string{"id": "process:1"},
			name:         "item",
			responseSize: 1,
		},
		{
			recordType:   Process,
			method:       "Get",
			url:          "/",
			params:       map[string]string{"sortBy": "identity.asc"},
			vars:         map[string]string{"id": "process:0"},
			name:         "flows",
			responseSize: 3,
		},
		{
			recordType:   Process,
			method:       "Get",
			url:          "/",
			params:       map[string]string{"sortBy": "identity.asc"},
			vars:         map[string]string{"id": "process:1"},
			name:         "addresses",
			responseSize: 1,
		},
		{
			recordType:   Process,
			method:       "Get",
			url:          "/",
			params:       map[string]string{"sortBy": "identity.asc"},
			vars:         map[string]string{"id": "process:1"},
			name:         "connector",
			responseSize: 1,
		},
		{
			recordType:   ProcessGroup,
			method:       "Get",
			url:          "/",
			params:       map[string]string{"sortBy": "identity.asc"},
			vars:         map[string]string{},
			name:         "list",
			responseSize: 1,
		},
		{
			recordType:   SitePair,
			method:       "Get",
			url:          "/",
			params:       map[string]string{"sortBy": "identity.asc"},
			vars:         map[string]string{},
			name:         "list",
			responseSize: 1,
		},
		{
			recordType:   SitePair,
			method:       "Get",
			url:          "/",
			params:       map[string]string{"sortBy": "identity.asc"},
			vars:         map[string]string{"id": "site:0-to-site:0"},
			name:         "item",
			responseSize: 1,
		},
		{
			recordType:   ProcessGroupPair,
			method:       "Get",
			url:          "/",
			params:       map[string]string{"sortBy": "identity.asc"},
			vars:         map[string]string{},
			name:         "list",
			responseSize: 1,
		},
		{
			recordType:   ProcessPair,
			method:       "Get",
			url:          "/",
			params:       map[string]string{"sortBy": "identity.asc"},
			vars:         map[string]string{},
			name:         "list",
			responseSize: 1,
		},
		{
			recordType:   ProcessPair,
			method:       "Get",
			url:          "/",
			params:       map[string]string{"sortBy": "identity.asc"},
			vars:         map[string]string{"id": "process:0-to-process:1"},
			name:         "item",
			responseSize: 1,
		},
		{
			recordType:   Collector,
			method:       "Get",
			url:          "/",
			params:       map[string]string{"sortBy": "identity.asc"},
			vars:         map[string]string{},
			name:         "list",
			responseSize: 1,
		},
	}

	var payload Payload
	for _, test := range testTable {
		req, _ := http.NewRequest(test.method, test.url, nil)
		q := req.URL.Query()
		for k, v := range test.params {
			q.Add(k, v)
		}
		req = mux.SetURLVars(req, test.vars)
		req.URL.RawQuery = q.Encode()
		resp, err := fc.retrieve(ApiRequest{RecordType: test.recordType, HandlerName: test.name, Request: req})
		assert.Assert(t, err)
		err = json.Unmarshal([]byte(*resp), &payload)
		assert.Assert(t, err)
		assert.Equal(t, test.responseSize, payload.Count)
	}

	endTime := uint64(time.Now().UnixNano()) / uint64(time.Microsecond)
	for _, c := range connectors {
		c.EndTime = endTime
		err := fc.updateRecord(c)
		assert.Assert(t, err)
	}
	for _, l := range listeners {
		l.EndTime = endTime
		err := fc.updateRecord(l)
		assert.Assert(t, err)
	}
	for _, p := range processes {
		p.EndTime = endTime
		err := fc.updateRecord(p)
		assert.Assert(t, err)
	}
	for _, l := range links {
		l.EndTime = endTime
		err := fc.updateRecord(l)
		assert.Assert(t, err)
	}
	for _, h := range hosts {
		h.EndTime = endTime
		err := fc.updateRecord(h)
		assert.Assert(t, err)
	}
	for _, r := range routers {
		r.EndTime = endTime
		err := fc.updateRecord(r)
		assert.Assert(t, err)
	}
	for _, s := range sites {
		s.EndTime = endTime
		err := fc.updateRecord(s)
		assert.Assert(t, err)
	}
}

func TestRecordGraphNetworkStatus(t *testing.T) {
	name := "skupper-site"
	namespace := "skupper-public"
	provider := "aws"
	address := "tcp-go-echo"
	address2 := "redis-cart"
	address3 := "mongo"
	address4 := "cartservice"
	protocol := "tcp"
	processName1 := "checkout-1234"
	processName2 := "payment-1234"
	groupName1 := "online-store"
	connectorProcess1 := "process:1"
	connectorProcess2 := "process:2"
	processConnector := "connector:0"

	sites := []SiteRecord{
		{
			Base: Base{
				RecType:   recordNames[Site],
				Identity:  "site:0",
				StartTime: uint64(time.Now().UnixNano()) / uint64(time.Microsecond),
			},
			Name:      &name,
			NameSpace: &namespace,
			Provider:  &provider,
		},
	}
	routers := []RouterRecord{
		{
			Base: Base{
				RecType:   recordNames[Router],
				Identity:  "router:0",
				Parent:    "site:0",
				StartTime: uint64(time.Now().UnixNano()) / uint64(time.Microsecond),
			},
			Name: &name,
		},
		{
			Base: Base{
				RecType:   recordNames[Router],
				Identity:  "router:1",
				Parent:    "site:0",
				StartTime: uint64(time.Now().UnixNano()) / uint64(time.Microsecond),
			},
			Name: &name,
		},
	}
	hosts := []HostRecord{
		{
			Base: Base{
				RecType:   recordNames[Host],
				Identity:  "host:0",
				Parent:    "site:0",
				StartTime: uint64(time.Now().UnixNano()) / uint64(time.Microsecond),
			},
		},
		{
			Base: Base{
				RecType:   recordNames[Host],
				Identity:  "host:1",
				Parent:    "site:0",
				StartTime: uint64(time.Now().UnixNano()) / uint64(time.Microsecond),
			},
		},
		{
			Base: Base{
				RecType:   recordNames[Host],
				Identity:  "host:2",
				Parent:    "site:0",
				StartTime: uint64(time.Now().UnixNano()) / uint64(time.Microsecond),
			},
		},
	}
	links := []LinkRecord{
		{
			Base: Base{
				RecType:   recordNames[Link],
				Identity:  "link:0",
				Parent:    "router:0",
				StartTime: uint64(time.Now().UnixNano()) / uint64(time.Microsecond),
			},
		},
		{
			Base: Base{
				RecType:   recordNames[Link],
				Identity:  "link:1",
				Parent:    "router:1",
				StartTime: uint64(time.Now().UnixNano()) / uint64(time.Microsecond),
			},
		},
	}
	processes := []ProcessRecord{
		{
			Base: Base{
				RecType:   recordNames[Process],
				Identity:  "process:0",
				Parent:    "site:0",
				StartTime: uint64(time.Now().UnixNano()) / uint64(time.Microsecond),
			},
			Name:      &processName1,
			GroupName: &groupName1,
		},
		{
			Base: Base{
				RecType:   recordNames[Process],
				Identity:  "process:1",
				Parent:    "site:0",
				StartTime: uint64(time.Now().UnixNano()) / uint64(time.Microsecond),
			},
			Name:      &processName2,
			GroupName: &groupName1,
			connector: &processConnector,
		},
		{
			Base: Base{
				RecType:   recordNames[Process],
				Identity:  "process:2",
				Parent:    "site:0",
				StartTime: uint64(time.Now().UnixNano()) / uint64(time.Microsecond),
			},
			Name:      &processName1,
			GroupName: &groupName1,
		},
	}
	listeners := []ListenerRecord{
		{
			Base: Base{
				RecType:   recordNames[Listener],
				Identity:  "listener:0",
				Parent:    "router:0",
				StartTime: uint64(time.Now().UnixNano()) / uint64(time.Microsecond),
			},
			Address:  &address,
			Protocol: &protocol,
		},
		{
			Base: Base{
				RecType:   recordNames[Listener],
				Identity:  "listener:1",
				Parent:    "router:1",
				StartTime: uint64(time.Now().UnixNano()) / uint64(time.Microsecond),
			},
			Protocol: &protocol,
		},
		{
			Base: Base{
				RecType:   recordNames[Listener],
				Identity:  "listener:2",
				Parent:    "router:1",
				StartTime: uint64(time.Now().UnixNano()) / uint64(time.Microsecond),
			},
			Protocol: &protocol,
			Address:  &address2,
		},
	}
	connectors := []ConnectorRecord{
		{
			Base: Base{
				RecType:   recordNames[Connector],
				Identity:  "connector:0",
				Parent:    "router:0",
				StartTime: uint64(time.Now().UnixNano()) / uint64(time.Microsecond),
			},
			Address:   &address3,
			Protocol:  &protocol,
			ProcessId: &connectorProcess1,
		},
		{
			Base: Base{
				RecType:   recordNames[Connector],
				Identity:  "connector:1",
				Parent:    "router:1",
				StartTime: uint64(time.Now().UnixNano()) / uint64(time.Microsecond),
			},
			Protocol: &protocol,
		},
		{
			Base: Base{
				RecType:   recordNames[Connector],
				Identity:  "connector:1",
				Parent:    "router:1",
				StartTime: uint64(time.Now().UnixNano()) / uint64(time.Microsecond),
			},
			Protocol:  &protocol,
			Address:   &address4,
			ProcessId: &connectorProcess2,
		},
	}

	u, _ := time.ParseDuration("5m")
	fc := NewFlowCollector(FlowCollectorSpec{
		Mode:              RecordStatus,
		Origin:            "origin",
		PromReg:           nil,
		ConnectionFactory: nil,
		FlowRecordTtl:     u,
	})
	for _, s := range sites {
		err := fc.updateRecord(s)
		assert.Assert(t, err)
		err = fc.updateRecord(s)
		assert.Assert(t, err)
	}
	for _, r := range routers {
		err := fc.updateRecord(r)
		assert.Assert(t, err)
		err = fc.updateRecord(r)
		assert.Assert(t, err)
	}
	for _, h := range hosts {
		err := fc.updateRecord(h)
		assert.Assert(t, err)
		err = fc.updateRecord(h)
		assert.Assert(t, err)
	}
	for _, l := range links {
		err := fc.updateRecord(l)
		assert.Assert(t, err)
		err = fc.updateRecord(l)
		assert.Assert(t, err)
	}
	for _, p := range processes {
		err := fc.updateRecord(p)
		assert.Assert(t, err)
		err = fc.updateRecord(p)
		assert.Assert(t, err)
	}
	for _, l := range listeners {
		err := fc.updateRecord(l)
		assert.Assert(t, err)
		err = fc.updateRecord(l)
		assert.Assert(t, err)
	}
	for _, c := range connectors {
		err := fc.updateRecord(c)
		assert.Assert(t, err)
		err = fc.updateRecord(c)
		assert.Assert(t, err)
	}

	for _, s := range sites {
		id := fc.getRecordSiteId(s)
		assert.Equal(t, id, s.Identity)
	}
	for _, r := range routers {
		id := fc.getRecordSiteId(r)
		assert.Equal(t, id, r.Parent)
	}
	for _, h := range hosts {
		id := fc.getRecordSiteId(h)
		assert.Equal(t, id, h.Parent)
	}
	for _, l := range links {
		id := fc.getRecordSiteId(l)
		assert.Equal(t, id, "site:0")
	}
	for _, p := range processes {
		id := fc.getRecordSiteId(p)
		assert.Equal(t, id, p.Parent)
	}
	for _, l := range listeners {
		id := fc.getRecordSiteId(l)
		assert.Equal(t, id, "site:0")
	}
	for _, l := range connectors {
		id := fc.getRecordSiteId(l)
		assert.Equal(t, id, "site:0")
	}

	fc.reconcileConnectorRecords()

	err := fc.updateNetworkStatus()
	assert.Assert(t, err)
}

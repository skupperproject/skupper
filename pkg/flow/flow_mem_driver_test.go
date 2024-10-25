package flow

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/skupperproject/skupper/api/types"
	"gotest.tools/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestRecordGraphWithMetrics(t *testing.T) {
	name := "skupper-site1"
	namespace := "skupper-public1"
	provider := "aws"
	name2 := "skupper-site2"
	namespace2 := "skupper-public2"
	provider2 := "gke"
	address := "tcp-go-echo"
	address2 := "redis-cart"
	address3 := "mongo"
	address4 := "cartservice"
	addressExt := "svc-acme-foo"
	protocol := "tcp"
	counterFlow2 := "flow:2"
	counterFlow4 := "flow:4"
	counterFlow10 := "flow:10"
	processName1 := "checkout-1234"
	processName2 := "payment-1234"
	groupName1 := "online-store"
	connectorProcess1 := "process:1"
	connectorProcess2 := "process:2"
	processConnector := "connector:0"
	sourceHost1 := "11.22.33.44"
	destHost1 := "10.20.30.40"
	destHost2 := "host.cloud.com"
	destHostExt := "172.4.4.8"
	connectorHost := "0.0.0.0"
	linkNames := []string{
		"site1.1",
		"site2.0",
	}
	routerNames := []string{
		"0/site1.0",
		"0/site1.1",
		"0/site2.0",
	}

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
		{
			Base: Base{
				RecType:   recordNames[Site],
				Identity:  "site:1",
				StartTime: uint64(time.Now().UnixNano()) / uint64(time.Microsecond),
			},
			Name:      &name2,
			NameSpace: &namespace2,
			Provider:  &provider2,
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
			Name: &routerNames[0],
		},
		{
			Base: Base{
				RecType:   recordNames[Router],
				Identity:  "router:1",
				Parent:    "site:0",
				StartTime: uint64(time.Now().UnixNano()) / uint64(time.Microsecond),
			},
			Name: &routerNames[1],
		},
		{
			Base: Base{
				RecType:   recordNames[Router],
				Identity:  "router:2",
				Parent:    "site:1",
				StartTime: uint64(time.Now().UnixNano()) / uint64(time.Microsecond),
			},
			Name: &routerNames[2],
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
				Parent:    "router:1",
				StartTime: uint64(time.Now().UnixNano()) / uint64(time.Microsecond),
			},
			Name:      &linkNames[0],
			Direction: &Incoming,
		},
		{
			Base: Base{
				RecType:   recordNames[Link],
				Identity:  "link:1",
				Parent:    "router:2",
				StartTime: uint64(time.Now().UnixNano()) / uint64(time.Microsecond),
			},
			Name:      &linkNames[1],
			Direction: &Outgoing,
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
			Name:       &processName1,
			GroupName:  &groupName1,
			SourceHost: &sourceHost1,
		},
		{
			Base: Base{
				RecType:   recordNames[Process],
				Identity:  "process:1",
				Parent:    "site:0",
				StartTime: uint64(time.Now().UnixNano()) / uint64(time.Microsecond),
			},
			Name:       &processName2,
			GroupName:  &groupName1,
			connector:  &processConnector,
			SourceHost: &destHost1,
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
			HostName:  &destHost2,
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
			Address:   &address,
			AddressId: &address,
			Protocol:  &protocol,
		},
		{
			Base: Base{
				RecType:   recordNames[Listener],
				Identity:  "listener:1",
				Parent:    "router:1",
				StartTime: uint64(time.Now().UnixNano()) / uint64(time.Microsecond),
			},
			Protocol:  &protocol,
			Address:   &address,
			AddressId: &address,
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
		{
			Base: Base{
				RecType:   recordNames[Listener],
				Identity:  "listener:3",
				Parent:    "router:1",
				StartTime: uint64(time.Now().UnixNano()) / uint64(time.Microsecond),
			},
			Protocol: &protocol,
			Address:  &addressExt,
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
			DestHost:  &destHost1,
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
			DestHost:  &destHost2,
		},
		{
			Base: Base{
				RecType:   recordNames[Connector],
				Identity:  "connector:2",
				Parent:    "router:0",
				StartTime: uint64(time.Now().UnixNano()) / uint64(time.Microsecond),
			},
			Protocol:  &protocol,
			Address:   &address,
			ProcessId: &connectorProcess1,
			DestHost:  &destHost1,
		},
		{
			Base: Base{
				RecType:   recordNames[Connector],
				Identity:  "connector:3",
				Parent:    "router:0",
				StartTime: uint64(time.Now().Add(-3*time.Minute).UnixNano()) / uint64(time.Microsecond),
			},
			Protocol: &protocol,
			Address:  &addressExt,
			DestHost: &destHostExt,
		},
	}
	// Two very screwy flow pairs
	//	flow:2/flow:3 a regular tcp flow pair from process:0 -> process:1
	//  flow:4/flow:5 a L7 flow pair with parents flow:0/flow:1. Also from process:0 -> process:1 with protocol tcp
	//  un-paired flow:0/flow:1 act as parents for the L7 flow pair. Mostly nonsense, as they reference l4 listeners and connectors
	flows := []FlowRecord{
		{
			Base: Base{
				RecType:   recordNames[Flow],
				Identity:  "flow:0",
				Parent:    "listener:0",
				StartTime: uint64(time.Now().UnixNano()) / uint64(time.Microsecond),
			},
			SourceHost: &sourceHost1,
		},
		{
			Base: Base{
				RecType:   recordNames[Flow],
				Identity:  "flow:1",
				Parent:    "connector:0",
				StartTime: uint64(time.Now().UnixNano()) / uint64(time.Microsecond),
			},
			SourceHost: &connectorHost,
		},
		{
			Base: Base{
				RecType:   recordNames[Flow],
				Identity:  "flow:2",
				Parent:    "listener:0",
				StartTime: uint64(time.Now().UnixNano()) / uint64(time.Microsecond),
			},
			SourceHost: &sourceHost1,
		},
		{
			Base: Base{
				RecType:   recordNames[Flow],
				Identity:  "flow:3",
				Parent:    "connector:0",
				StartTime: uint64(time.Now().UnixNano()) / uint64(time.Microsecond),
			},
			SourceHost:  &connectorHost,
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
			SourceHost:  &sourceHost1,
			ProcessName: &processName1,
		},
		{
			Base: Base{
				RecType:   recordNames[Flow],
				Identity:  "flow:4",
				Parent:    "flow:0",
				StartTime: uint64(time.Now().UnixNano()) / uint64(time.Microsecond),
			},
		},
		{
			Base: Base{
				RecType:   recordNames[Flow],
				Identity:  "flow:5",
				Parent:    "flow:1",
				StartTime: uint64(time.Now().UnixNano()) / uint64(time.Microsecond),
			},
			CounterFlow: &counterFlow4,
		},
	}

	// An additional tcp flow pair (10,11):
	// Exchange between process:0 -> an unknown external process. Should
	// introduce a site-server process
	flows = append(flows, []FlowRecord{
		{
			Base: Base{
				RecType:   recordNames[Flow],
				Identity:  "flow:10",
				Parent:    "listener:3",
				StartTime: uint64(time.Now().UnixNano()) / uint64(time.Microsecond),
			},
			SourceHost: &sourceHost1,
		},
		{
			Base: Base{
				RecType:   recordNames[Flow],
				Identity:  "flow:11",
				Parent:    "connector:3",
				StartTime: uint64(time.Now().UnixNano()) / uint64(time.Microsecond),
			},
			SourceHost:  &connectorHost,
			CounterFlow: &counterFlow10,
		},
	}...)

	reg := prometheus.NewRegistry()
	u, _ := time.ParseDuration("5m")
	fc := NewFlowCollector(FlowCollectorSpec{
		Mode:              RecordMetrics,
		Origin:            "origin",
		PromReg:           reg,
		ConnectionFactory: nil,
		FlowRecordTtl:     u,
	})
	fc.startTime = uint64(time.Now().Add(time.Minute * -5).UnixMicro())
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
	for _, f := range flows {
		err := fc.updateRecord(f)
		assert.Assert(t, err)
		err = fc.updateRecord(f)
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
		expected := "site:0"
		if l.Parent == "router:2" {
			expected = "site:1"
		}
		assert.Equal(t, id, expected)
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
			assert.Equal(t, *trace, "site1.0@skupper-site1")
		}
	}

	fc.reconcileConnectorRecords()
	fc.reconcileFlowRecords()

	var addressId string
	for _, address := range fc.VanAddresses {
		if address.Name == "tcp-go-echo" {
			addressId = address.Identity
			break
		}
	}

	var processGroupId string
	for x, group := range fc.ProcessGroups {
		if group.Name != nil && !strings.HasPrefix(*group.Name, "site-servers") {
			processGroupId = x
			break
		}
	}

	fc.reconcileConnectorRecords()
	fc.reconcileFlowRecords()

	type test struct {
		desc         string
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
			desc:         "Get Site list",
			recordType:   Site,
			method:       "Get",
			url:          "/",
			params:       map[string]string{"sortBy": "identity.asc"},
			vars:         map[string]string{},
			name:         "list",
			responseSize: len(sites),
		},
		{
			desc:         "Get Site item",
			recordType:   Site,
			method:       "Get",
			url:          "/",
			params:       map[string]string{"sortBy": "identity.asc"},
			vars:         map[string]string{"id": "site:0"},
			name:         "item",
			responseSize: 1,
		},
		{
			desc:         "Get Site routers",
			recordType:   Site,
			method:       "Get",
			url:          "/",
			params:       map[string]string{"sortBy": "identity.asc"},
			vars:         map[string]string{"id": "site:0"},
			name:         "routers",
			responseSize: 2,
		},
		{
			desc:         "Get Site links",
			recordType:   Site,
			method:       "Get",
			url:          "/",
			params:       map[string]string{"sortBy": "identity.asc"},
			vars:         map[string]string{"id": "site:0"},
			name:         "links",
			responseSize: 1,
		},
		{
			desc:         "Get Site hosts",
			recordType:   Site,
			method:       "Get",
			url:          "/",
			params:       map[string]string{"sortBy": "identity.asc"},
			vars:         map[string]string{"id": "site:0"},
			name:         "hosts",
			responseSize: len(hosts),
		},
		{
			desc:         "Get Site processes",
			recordType:   Site,
			method:       "Get",
			url:          "/",
			params:       map[string]string{"sortBy": "identity.asc"},
			vars:         map[string]string{"id": "site:0"},
			name:         "processes",
			responseSize: len(processes) + 1,
		},
		{
			desc:         "Get Host list",
			recordType:   Host,
			method:       "Get",
			url:          "/",
			params:       map[string]string{"sortBy": "identity.asc"},
			vars:         map[string]string{},
			name:         "list",
			responseSize: len(hosts),
		},
		{
			desc:         "Get Host item",
			recordType:   Host,
			method:       "Get",
			url:          "/",
			params:       map[string]string{"sortBy": "identity.asc"},
			vars:         map[string]string{"id": "host:0"},
			name:         "item",
			responseSize: 1,
		},
		{
			desc:         "Get Router list",
			recordType:   Router,
			method:       "Get",
			url:          "/",
			params:       map[string]string{"sortBy": "identity.asc"},
			vars:         map[string]string{},
			name:         "list",
			responseSize: len(routers),
		},
		{
			desc:         "Get Router item",
			recordType:   Router,
			method:       "Get",
			url:          "/",
			params:       map[string]string{"sortBy": "identity.asc"},
			vars:         map[string]string{"id": "router:0"},
			name:         "item",
			responseSize: 1,
		},
		{
			desc:         "Get Router flows",
			recordType:   Router,
			method:       "Get",
			url:          "/",
			params:       map[string]string{"sortBy": "identity.asc"},
			vars:         map[string]string{"id": "router:0"},
			name:         "flows",
			responseSize: 7,
		},
		{
			desc:         "Get Router links",
			recordType:   Router,
			method:       "Get",
			url:          "/",
			params:       map[string]string{"sortBy": "identity.asc"},
			vars:         map[string]string{"id": "router:1"},
			name:         "links",
			responseSize: 1,
		},
		{
			desc:         "Get Router listeners",
			recordType:   Router,
			method:       "Get",
			url:          "/",
			params:       map[string]string{"sortBy": "identity.asc"},
			vars:         map[string]string{"id": "router:0"},
			name:         "listeners",
			responseSize: 1,
		},
		{
			desc:         "Get Router connectors",
			recordType:   Router,
			method:       "Get",
			url:          "/",
			params:       map[string]string{"sortBy": "identity.asc"},
			vars:         map[string]string{"id": "router:0"},
			name:         "connectors",
			responseSize: 3,
		},
		{
			desc:         "Get Link list",
			recordType:   Link,
			method:       "Get",
			url:          "/",
			params:       map[string]string{"sortBy": "identity.asc"},
			vars:         map[string]string{},
			name:         "list",
			responseSize: len(links),
		},
		{
			desc:         "Get Link item",
			recordType:   Link,
			method:       "Get",
			url:          "/",
			params:       map[string]string{"sortBy": "identity.asc"},
			vars:         map[string]string{"id": "link:0"},
			name:         "item",
			responseSize: 1,
		},
		{
			desc:         "Get Listener list",
			recordType:   Listener,
			method:       "Get",
			url:          "/",
			params:       map[string]string{"sortBy": "identity.asc"},
			vars:         map[string]string{},
			name:         "list",
			responseSize: len(listeners),
		},
		{
			desc:         "Get Listener item",
			recordType:   Listener,
			method:       "Get",
			url:          "/",
			params:       map[string]string{"sortBy": "identity.asc"},
			vars:         map[string]string{"id": "listener:0"},
			name:         "item",
			responseSize: 1,
		},
		{
			desc:         "Get Listener flows",
			recordType:   Listener,
			method:       "Get",
			url:          "/",
			params:       map[string]string{"sortBy": "identity.asc"},
			vars:         map[string]string{"id": "listener:0"},
			name:         "flows",
			responseSize: 3,
		},
		{
			desc:         "Get Connector list",
			recordType:   Connector,
			method:       "Get",
			url:          "/",
			params:       map[string]string{"sortBy": "identity.asc"},
			vars:         map[string]string{},
			name:         "list",
			responseSize: 4,
		},
		{
			desc:         "Get Connector item",
			recordType:   Connector,
			method:       "Get",
			url:          "/",
			params:       map[string]string{"sortBy": "identity.asc"},
			vars:         map[string]string{"id": "connector:0"},
			name:         "item",
			responseSize: 1,
		},
		{
			desc:         "Get Connector flows",
			recordType:   Connector,
			method:       "Get",
			url:          "/",
			params:       map[string]string{"sortBy": "identity.asc"},
			vars:         map[string]string{"id": "connector:0"},
			name:         "flows",
			responseSize: 3,
		},
		{
			desc:         "Get Connector process",
			recordType:   Connector,
			method:       "Get",
			url:          "/",
			params:       map[string]string{"sortBy": "identity.asc"},
			vars:         map[string]string{"id": "connector:0"},
			name:         "process",
			responseSize: 1,
		},
		{
			desc:         "Get Address list",
			recordType:   Address,
			method:       "Get",
			url:          "/",
			params:       map[string]string{"sortBy": "identity.asc"},
			vars:         map[string]string{},
			name:         "list",
			responseSize: len(fc.VanAddresses),
		},
		{
			desc:         "Get Address item",
			recordType:   Address,
			method:       "Get",
			url:          "/",
			params:       map[string]string{"sortBy": "identity.asc"},
			vars:         map[string]string{"id": addressId},
			name:         "item",
			responseSize: 1,
		},
		{
			desc:         "Get Address flows",
			recordType:   Address,
			method:       "Get",
			url:          "/",
			params:       map[string]string{"sortBy": "identity.asc"},
			vars:         map[string]string{"id": addressId},
			name:         "flows",
			responseSize: 3,
		},
		{
			desc:         "Get Address flowpairs",
			recordType:   Address,
			method:       "Get",
			url:          "/",
			params:       map[string]string{"sortBy": "identity.asc"},
			vars:         map[string]string{"id": addressId},
			name:         "flowpairs",
			responseSize: 2,
		},
		{
			desc:         "Get Address listeners",
			recordType:   Address,
			method:       "Get",
			url:          "/",
			params:       map[string]string{"sortBy": "identity.asc"},
			vars:         map[string]string{"id": addressId},
			name:         "listeners",
			responseSize: 2,
		},
		{
			desc:         "Get Address connectors",
			recordType:   Address,
			method:       "Get",
			url:          "/",
			params:       map[string]string{"sortBy": "identity.asc"},
			vars:         map[string]string{"id": addressId},
			name:         "connectors",
			responseSize: 1,
		},
		{
			desc:         "Get Flow list",
			recordType:   Flow,
			method:       "Get",
			url:          "/",
			params:       map[string]string{"sortBy": "identity.asc"},
			vars:         map[string]string{},
			name:         "list",
			responseSize: len(fc.Flows),
		},
		{
			desc:         "Get Flow item",
			recordType:   Flow,
			method:       "Get",
			url:          "/",
			params:       map[string]string{"sortBy": "identity.asc"},
			vars:         map[string]string{"id": "flow:1"},
			name:         "item",
			responseSize: 1,
		},
		{
			desc:         "Get Flow process",
			recordType:   Flow,
			method:       "Get",
			url:          "/",
			params:       map[string]string{"sortBy": "identity.asc"},
			vars:         map[string]string{"id": "flow:1"},
			name:         "process",
			responseSize: 1,
		},
		{
			desc:         "Get Flowpair list",
			recordType:   FlowPair,
			method:       "Get",
			url:          "/",
			params:       map[string]string{"sortBy": "identity.asc"},
			vars:         map[string]string{},
			name:         "list",
			responseSize: len(fc.FlowPairs),
		},
		{
			desc:         "Get Flowpair item",
			recordType:   FlowPair,
			method:       "Get",
			url:          "/",
			params:       map[string]string{"sortBy": "identity.asc"},
			vars:         map[string]string{"id": "fp-flow:2"},
			name:         "item",
			responseSize: 1,
		},
		{
			desc:         "Get Process list",
			recordType:   Process,
			method:       "Get",
			url:          "/",
			params:       map[string]string{"sortBy": "identity.asc"},
			vars:         map[string]string{},
			name:         "list",
			responseSize: len(processes) + 1,
		},
		{
			desc:         "Get Process site-servers",
			recordType:   Process,
			method:       "Get",
			url:          "/",
			params:       map[string]string{"sortBy": "identity.asc", "name": "site-servers", "processBinding": "bound"},
			vars:         map[string]string{},
			name:         "list",
			responseSize: 1,
		},
		{
			desc:         "Get Process item",
			recordType:   Process,
			method:       "Get",
			url:          "/",
			params:       map[string]string{"sortBy": "identity.asc"},
			vars:         map[string]string{"id": "process:1"},
			name:         "item",
			responseSize: 1,
		},
		{
			desc:         "Get Process flows",
			recordType:   Process,
			method:       "Get",
			url:          "/",
			params:       map[string]string{"sortBy": "identity.asc"},
			vars:         map[string]string{"id": "process:0"},
			name:         "flows",
			responseSize: 4,
		},
		{
			desc:         "Get Process addresses",
			recordType:   Process,
			method:       "Get",
			url:          "/",
			params:       map[string]string{"sortBy": "identity.asc"},
			vars:         map[string]string{"id": "process:1"},
			name:         "addresses",
			responseSize: 2,
		},
		{
			desc:         "Get Process connector",
			recordType:   Process,
			method:       "Get",
			url:          "/",
			params:       map[string]string{"sortBy": "identity.asc"},
			vars:         map[string]string{"id": "process:1"},
			name:         "connector",
			responseSize: 1,
		},
		{
			desc:         "Get ProcessGroup list",
			recordType:   ProcessGroup,
			method:       "Get",
			url:          "/",
			params:       map[string]string{"sortBy": "identity.asc"},
			vars:         map[string]string{},
			name:         "list",
			responseSize: 2,
		},
		{
			desc:         "Get ProcessGroup item",
			recordType:   ProcessGroup,
			method:       "Get",
			url:          "/",
			params:       map[string]string{"sortBy": "identity.asc"},
			vars:         map[string]string{"id": processGroupId},
			name:         "item",
			responseSize: 1,
		},
		{
			desc:         "Get ProcessGroup processes",
			recordType:   ProcessGroup,
			method:       "Get",
			url:          "/",
			params:       map[string]string{"sortBy": "identity.asc"},
			vars:         map[string]string{"id": processGroupId},
			name:         "processes",
			responseSize: 3,
		},
		{
			desc:         "Get SitePair list",
			recordType:   SitePair,
			method:       "Get",
			url:          "/",
			params:       map[string]string{"sortBy": "identity.asc"},
			vars:         map[string]string{},
			name:         "list",
			responseSize: 1,
		},
		{
			desc:         "Get SitePair item",
			recordType:   SitePair,
			method:       "Get",
			url:          "/",
			params:       map[string]string{"sortBy": "identity.asc"},
			vars:         map[string]string{"id": "site:0-to-site:0"},
			name:         "item",
			responseSize: 1,
		},
		{
			desc:         "Get ProcessGroupPair list",
			recordType:   ProcessGroupPair,
			method:       "Get",
			url:          "/",
			params:       map[string]string{"sortBy": "identity.asc"},
			vars:         map[string]string{},
			name:         "list",
			responseSize: 2,
		},
		{
			desc:         "Get ProcessPair list",
			recordType:   ProcessPair,
			method:       "Get",
			url:          "/",
			params:       map[string]string{"sortBy": "identity.asc"},
			vars:         map[string]string{},
			name:         "list",
			responseSize: 2,
		},
		{
			desc:         "Get ProcessPair item",
			recordType:   ProcessPair,
			method:       "Get",
			url:          "/",
			params:       map[string]string{"sortBy": "identity.asc"},
			vars:         map[string]string{"id": "process:0-to-process:1"},
			name:         "item",
			responseSize: 1,
		},
		{
			desc:         "Get Collector list",
			recordType:   Collector,
			method:       "Get",
			url:          "/",
			params:       map[string]string{"sortBy": "identity.asc"},
			vars:         map[string]string{},
			name:         "list",
			responseSize: 1,
		},
		{
			desc:         "Get Collector item",
			recordType:   Collector,
			method:       "Get",
			url:          "/",
			params:       map[string]string{"sortBy": "identity.asc"},
			vars:         map[string]string{},
			name:         "item",
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
		assert.Equal(t, test.responseSize, payload.Count, test.desc)
	}

	for _, flow := range fc.Flows {
		assert.DeepEqual(t, flow.Protocol, &protocol)
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
	client := fake.NewSimpleClientset()
	configMapClient := client.CoreV1().ConfigMaps("default")
	_, err := configMapClient.Create(
		context.Background(),
		&v1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: types.NetworkStatusConfigMapName}},
		metav1.CreateOptions{},
	)
	if err != nil {
		t.Fatalf("kube cliet setup failed: %v", err)
	}
	fc := NewFlowCollector(FlowCollectorSpec{
		Mode:                RecordStatus,
		Namespace:           "default",
		Origin:              "origin",
		PromReg:             nil,
		ConnectionFactory:   nil,
		FlowRecordTtl:       u,
		NetworkStatusClient: client,
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

	fc.updateNetworkStatus()

	cm, err := configMapClient.Get(context.Background(), types.NetworkStatusConfigMapName, metav1.GetOptions{})
	if err != nil {
		t.Fatal("expected configmap", err)
	}
	networkStatus, ok := cm.Data["NetworkStatus"]
	assert.Check(t, ok)
	var status NetworkStatus
	assert.Check(t, json.Unmarshal([]byte(networkStatus), &status))
	assert.Equal(t, len(status.Sites), len(sites))
	assert.Equal(t, len(status.Addresses), 4)

}

func TestGraph(t *testing.T) {
	newSite := func(id string) *SiteRecord {
		return &SiteRecord{
			Base: Base{
				RecType:  recordNames[Site],
				Identity: id,
			},
		}
	}
	newRouter := func(id string, parent string, name string) *RouterRecord {
		rn := "0/" + name
		return &RouterRecord{Base: Base{
			RecType:  recordNames[Router],
			Identity: id,
			Parent:   parent,
		},
			Name: &rn,
		}
	}

	type largs struct {
		ID             string
		RouterID       string
		PeerRouterName string
		Direction      string
		Role           string
	}
	newLink := func(a largs) *LinkRecord {
		record := &LinkRecord{
			Base: Base{
				RecType:  recordNames[Router],
				Identity: a.ID,
				Parent:   a.RouterID,
			},
			Name:      &a.PeerRouterName,
			Direction: &a.Direction,
		}
		if a.Role != "" {
			record.Mode = &a.Role
		}
		return record
	}
	tests := []struct {
		Name                string
		Sites               []*SiteRecord
		Routers             []*RouterRecord
		Links               []*LinkRecord
		ExpectedRouterNodes map[string]*node
		ExpectedSiteNodes   map[string]*node
	}{
		{
			Name: "empty",
		}, {
			Name: "missing routers",
			Sites: []*SiteRecord{
				newSite("site:0"), newSite("site:1"),
			},
			Links: []*LinkRecord{
				newLink(largs{ID: "norouter:0", RouterID: "router:0", PeerRouterName: "r1", Direction: Incoming}),
				newLink(largs{ID: "norouter:1", RouterID: "router:1", PeerRouterName: "r0", Direction: Outgoing}),
				newLink(largs{ID: "norouter:2", RouterID: "router:2", PeerRouterName: "r0", Direction: Outgoing, Role: "edge"}),
			},
			ExpectedSiteNodes: map[string]*node{
				"site:0": {ID: "site:0"},
				"site:1": {ID: "site:1"},
			},
		}, {
			Name: "missing site",
			Sites: []*SiteRecord{
				newSite("site:0"),
			},
			Routers: []*RouterRecord{
				newRouter("router0", "site:0", "r0"),
				newRouter("router1", "site:1", "r1"),
				newRouter("router2", "site:2", "r2"),
			},
			Links: []*LinkRecord{
				newLink(largs{ID: "link0", RouterID: "router0", PeerRouterName: "r1", Direction: Incoming}),
				newLink(largs{ID: "link1", RouterID: "router1", PeerRouterName: "r0", Direction: Outgoing}),
				newLink(largs{ID: "link2", RouterID: "router2", PeerRouterName: "r0", Direction: Outgoing, Role: "edge"}),
			},
			ExpectedRouterNodes: map[string]*node{
				"router0": {ID: "router0"},
				"router1": {ID: "router1"},
				"router2": {ID: "router2"},
			},
			ExpectedSiteNodes: map[string]*node{
				"site:0": {ID: "site:0"},
			},
		}, {
			Name: "single link pair",
			Sites: []*SiteRecord{
				newSite("site:0"), newSite("site:1"),
			},
			Routers: []*RouterRecord{
				newRouter("router0", "site:0", "r0"),
				newRouter("router1", "site:1", "r1"),
			},
			Links: []*LinkRecord{
				newLink(largs{ID: "link0", RouterID: "router0", PeerRouterName: "r1", Direction: Incoming}),
				newLink(largs{ID: "link1", RouterID: "router1", PeerRouterName: "r0", Direction: Outgoing}),
			},
			ExpectedRouterNodes: map[string]*node{
				"router0": {ID: "router0", Backward: []string{"router1"}},
				"router1": {ID: "router1", Forward: []string{"router0"}},
			},
			ExpectedSiteNodes: map[string]*node{
				"site:0": {ID: "site:0", Backward: []string{"site:1"}},
				"site:1": {ID: "site:1", Forward: []string{"site:0"}},
			},
		}, {
			Name: "redundant links",
			Sites: []*SiteRecord{
				newSite("site:0"), newSite("site:1"),
			},
			Routers: []*RouterRecord{
				newRouter("router0", "site:0", "r0"),
				newRouter("router1", "site:1", "r1"),
			},
			Links: []*LinkRecord{
				newLink(largs{ID: "link0", RouterID: "router0", PeerRouterName: "r1", Direction: Incoming}),
				newLink(largs{ID: "link1", RouterID: "router1", PeerRouterName: "r0", Direction: Outgoing}),
				newLink(largs{ID: "link2", RouterID: "router0", PeerRouterName: "r1", Direction: Incoming}),
				newLink(largs{ID: "link3", RouterID: "router1", PeerRouterName: "r0", Direction: Outgoing}),
				newLink(largs{ID: "link80", RouterID: "router0", PeerRouterName: "r1", Direction: Incoming}),
				newLink(largs{ID: "link90", RouterID: "router0", PeerRouterName: "r1", Direction: Incoming}),
			},
			ExpectedRouterNodes: map[string]*node{
				"router0": {ID: "router0", Backward: []string{"router1"}},
				"router1": {ID: "router1", Forward: []string{"router0"}},
			},
			ExpectedSiteNodes: map[string]*node{
				"site:0": {ID: "site:0", Backward: []string{"site:1"}},
				"site:1": {ID: "site:1", Forward: []string{"site:0"}},
			},
		}, {
			Name: "complex",
			Sites: []*SiteRecord{
				newSite("site:0"), newSite("site:1"), newSite("site:2"),
			},
			Routers: []*RouterRecord{
				newRouter("router0", "site:0", "r0"),
				newRouter("router1", "site:1", "r1"),
				newRouter("router2", "site:2", "r2"),
			},
			Links: []*LinkRecord{
				newLink(largs{ID: "link0", RouterID: "router1", PeerRouterName: "r0", Direction: Outgoing}), // half link to router1 -> router0
				newLink(largs{ID: "link1", RouterID: "router1", PeerRouterName: "r2", Direction: Outgoing}), // |
				newLink(largs{ID: "link2", RouterID: "router2", PeerRouterName: "r1", Direction: Incoming}), // | link pair router1,router2
				newLink(largs{ID: "link3", RouterID: "router2", PeerRouterName: "r0", Direction: Incoming}), // half link to router2 -> router0
				newLink(largs{ID: "bogus1", RouterID: "router2", PeerRouterName: "rdne", Direction: Incoming, Role: "edge"}),
				newLink(largs{ID: "bogus2", RouterID: "routerdne", PeerRouterName: "r0", Direction: Incoming}),
			},
			ExpectedRouterNodes: map[string]*node{
				"router0": {ID: "router0"},
				"router1": {ID: "router1", Forward: []string{"router2"}},
				"router2": {ID: "router2", Backward: []string{"router1"}},
			},
			ExpectedSiteNodes: map[string]*node{
				"site:0": {ID: "site:0"},
				"site:1": {ID: "site:1", Forward: []string{"site:2"}},
				"site:2": {ID: "site:2", Backward: []string{"site:1"}},
			},
		}, {
			Name: "edge links counted on connector side only",
			Sites: []*SiteRecord{
				newSite("site:0"), newSite("site:1"), newSite("site:2"),
			},
			Routers: []*RouterRecord{
				newRouter("router0", "site:0", "r0"),
				newRouter("router1", "site:1", "r1"),
				newRouter("router2", "site:2", "r2"),
			},
			Links: []*LinkRecord{
				newLink(largs{ID: "link1", RouterID: "router1", PeerRouterName: "r0", Direction: Outgoing, Role: "inter-router"}),
				newLink(largs{ID: "link2", RouterID: "router2", PeerRouterName: "r0", Direction: Outgoing, Role: "edge"}),
				newLink(largs{ID: "link-invalid", RouterID: "router0", PeerRouterName: "r2", Direction: Incoming, Role: "edge"}),
			},
			ExpectedRouterNodes: map[string]*node{
				"router0": {ID: "router0"},
				"router1": {ID: "router1"},
				"router2": {ID: "router2", Forward: []string{"router0"}},
			},
			ExpectedSiteNodes: map[string]*node{
				"site:0": {ID: "site:0"},
				"site:1": {ID: "site:1"},
				"site:2": {ID: "site:2", Forward: []string{"site:0"}},
			},
		}, {
			Name: "exclude intra-site links",
			Sites: []*SiteRecord{
				newSite("site:0"),
			},
			Routers: []*RouterRecord{
				newRouter("router0", "site:0", "r0"),
				newRouter("router1", "site:0", "r1"),
				newRouter("router2", "site:0", "r2"),
			},
			Links: []*LinkRecord{
				newLink(largs{ID: "link1", RouterID: "router0", PeerRouterName: "r1", Direction: Incoming, Role: "inter-router"}),
				newLink(largs{ID: "link2", RouterID: "router1", PeerRouterName: "r0", Direction: Outgoing, Role: "inter-router"}),
				newLink(largs{ID: "link3", RouterID: "router2", PeerRouterName: "r0", Direction: Outgoing, Role: "edge"}),
			},
			ExpectedRouterNodes: map[string]*node{
				"router0": {ID: "router0", Backward: []string{"router1"}},
				"router1": {ID: "router1", Forward: []string{"router0"}},
				"router2": {ID: "router2", Forward: []string{"router0"}},
			},
			ExpectedSiteNodes: map[string]*node{
				"site:0": {ID: "site:0"},
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			fc := NewFlowCollector(FlowCollectorSpec{})
			for _, site := range tc.Sites {
				fc.Sites[site.Identity] = site
			}
			for _, router := range tc.Routers {
				fc.Routers[router.Identity] = router
			}
			for _, link := range tc.Links {
				fc.Links[link.Identity] = link
			}
			if tc.ExpectedRouterNodes == nil {
				tc.ExpectedRouterNodes = make(map[string]*node)
			}
			if tc.ExpectedSiteNodes == nil {
				tc.ExpectedSiteNodes = make(map[string]*node)
			}
			routers, sites := fc.graph()
			assert.DeepEqual(t, routers, tc.ExpectedRouterNodes)
			assert.DeepEqual(t, sites, tc.ExpectedSiteNodes)
		})
	}
}

func TestLinkOutgoingDelete(t *testing.T) {
	name1 := "east-skupper-router-1111-router1"
	name2 := "west-skupper-router-1111-router0"

	links := []LinkRecord{
		{
			Base: Base{
				RecType:   recordNames[Link],
				Identity:  "link:0",
				Parent:    "router0:0",
				StartTime: uint64(time.Now().UnixNano()) / uint64(time.Microsecond),
			},
			Name:      &name1,
			Direction: &Incoming,
		},
		{
			Base: Base{
				RecType:   recordNames[Link],
				Identity:  "link:1",
				Parent:    "router1:0",
				StartTime: uint64(time.Now().UnixNano()) / uint64(time.Microsecond),
			},
			Name:      &name2,
			Direction: &Outgoing,
		},
	}

	routers := []RouterRecord{
		{
			Base: Base{
				RecType:   recordNames[Router],
				Identity:  "router0:0",
				Parent:    "site:0",
				StartTime: uint64(time.Now().UnixNano()) / uint64(time.Microsecond),
			},
			Name: &name2,
		},
		{
			Base: Base{
				RecType:   recordNames[Router],
				Identity:  "router1:0",
				Parent:    "site:0",
				StartTime: uint64(time.Now().UnixNano()) / uint64(time.Microsecond),
			},
			Name: &name1,
		},
	}

	u, _ := time.ParseDuration("5m")
	client := fake.NewSimpleClientset()
	configMapClient := client.CoreV1().ConfigMaps("default")
	_, err := configMapClient.Create(
		context.Background(),
		&v1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: types.NetworkStatusConfigMapName}},
		metav1.CreateOptions{},
	)
	if err != nil {
		t.Fatalf("kube client setup failed: %v", err)
	}
	fc := NewFlowCollector(FlowCollectorSpec{
		Mode:                RecordStatus,
		Namespace:           "default",
		Origin:              "origin",
		PromReg:             nil,
		ConnectionFactory:   nil,
		FlowRecordTtl:       u,
		NetworkStatusClient: client,
	})

	for _, r := range routers {
		err := fc.updateRecord(r)
		assert.Assert(t, err)
	}

	for _, l := range links {
		err := fc.updateRecord(l)
		assert.Assert(t, err)
	}

	assert.Equal(t, len(fc.Links), 2)
	assert.Equal(t, len(fc.Routers), 2)

	endTime := uint64(time.Now().UnixNano()) / uint64(time.Microsecond)

	links[0].EndTime = endTime
	err = fc.updateRecord(links[0])
	assert.Assert(t, err)

	// expect code to remove both incoming and corresponding outgoing links
	assert.Equal(t, len(fc.Links), 0)

}

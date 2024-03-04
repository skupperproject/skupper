package flow

import (
	"context"
	"encoding/json"
	"net/http"
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
	destHost1 := "10.20.30.40"
	destHost2 := "host.cloud.com"
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
			SourceHost:  &destHost1,
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
	for x := range fc.ProcessGroups {
		processGroupId = x
		break
	}

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
			responseSize: len(processes),
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
			responseSize: 6,
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
			responseSize: 2,
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
			responseSize: 3,
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
			responseSize: 3,
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
			vars:         map[string]string{"id": "fp-flow:0"},
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
			responseSize: len(processes),
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
			responseSize: 3,
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
			responseSize: 1,
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
			responseSize: 1,
		},
		{
			desc:         "Get ProcessPair list",
			recordType:   ProcessPair,
			method:       "Get",
			url:          "/",
			params:       map[string]string{"sortBy": "identity.asc"},
			vars:         map[string]string{},
			name:         "list",
			responseSize: 1,
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

	fc.reconcileConnectorRecords()
	fc.reconcileFlowRecords()

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

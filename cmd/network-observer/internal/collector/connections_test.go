package collector

import (
	"context"
	"fmt"
	"log/slog"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/skupperproject/skupper/pkg/vanflow"
	"github.com/skupperproject/skupper/pkg/vanflow/store"
	"gotest.tools/v3/assert"
)

func TestConnectionManager(t *testing.T) {
	tCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	tlog := slog.Default()
	vanStor := store.NewSyncMapStore(store.SyncMapStoreConfig{Indexers: RecordIndexers()})
	graf := NewGraph(vanStor).(*graph)
	// TODO(ck)  newConnectionmanager starts goroutines that can "steal" work
	// from manually invoked manager methods (i.e. runReconcile). Write
	// idempotent assertions.
	manager := newConnectionmanager(tCtx, tlog, store.SourceRef{}, vanStor, graf, register(prometheus.NewRegistry()), time.Minute)
	defer manager.Stop()
	flowStor := manager.flows

	vanStor.Replace(wrapRecords(van...))
	graf.Reset()

	flowStor.Add(vanflow.AppBiflowRecord{
		BaseRecord: vanflow.NewBase("appflow-01", time.Now(), time.Now()),
		Parent:     ptrTo("tflow-01"),
		Protocol:   ptrTo("HTTP/1.1"),
		Method:     ptrTo("GET"),
		Result:     ptrTo("200"),
	}, store.SourceRef{})
	// App flow missing Connection
	appResult := manager.runAppReconcile()
	assert.Equal(t, appResult.PendingTransportCount, 1)

	flowStor.Add(vanflow.TransportBiflowRecord{
		BaseRecord: vanflow.NewBase("tflow-01", time.Now()),
		Parent:     ptrTo("listener-backend"),
		SourceHost: ptrTo("10.111.0.111"),
	}, store.SourceRef{})
	// Transport Flow Missing Connector reference
	result := manager.runReconcile()
	assert.Equal(t, result.PendingConnectorCount, 1)
	// App flow pending Connection reconcile
	appResult = manager.runAppReconcile()
	assert.Equal(t, appResult.PendingTransportReconcileCount, 1)

	// Complete transport flow with connector
	flowStor.Patch(vanflow.TransportBiflowRecord{
		BaseRecord:  vanflow.NewBase("tflow-01", time.Now()),
		ConnectorID: ptrTo("connector-backend-1-6"),
	}, store.SourceRef{})

	result = manager.runReconcile()
	pending := result.PendingConnectorCount + result.PendingSourceCount + result.PendingDestCount
	assert.Equal(t, pending, 0)

	appResult = manager.runAppReconcile()
	pending = appResult.PendingTransportCount + appResult.PendingTransportReconcileCount + appResult.PendingProtocolCount + appResult.PendingUnknownCount
	assert.Equal(t, pending, 0)

	entry, ok := vanStor.Get("tflow-01")
	if !ok {
		t.Fatal("missing ConnectionRecord for tflow-01")
	}
	transportRecord := entry.Record.(ConnectionRecord)

	assert.Equal(t, transportRecord.Protocol, "tcp")
	assert.Equal(t, transportRecord.Source.Name, "client-west-01")
	assert.Equal(t, transportRecord.Dest.Name, "server-east-06")

	entry, ok = vanStor.Get("appflow-01")
	if !ok {
		t.Fatal("missing RequestRecord for appflow-01")
	}
	requestRecord := entry.Record.(RequestRecord)

	assert.Equal(t, requestRecord.Protocol, "http1")
	assert.Equal(t, requestRecord.Source.Name, "client-west-01")
	assert.Equal(t, requestRecord.Dest.Name, "server-east-06")
}

func benchmarkRunReconcile(b *testing.B, connections int) {
	tCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	tlog := slog.Default()
	vanStor := store.NewSyncMapStore(store.SyncMapStoreConfig{Indexers: RecordIndexers()})
	graf := NewGraph(vanStor).(*graph)
	manager := newConnectionmanager(tCtx, tlog, store.SourceRef{}, vanStor, graf, register(prometheus.NewRegistry()), time.Minute)
	defer manager.Stop()
	flowStor := manager.flows

	vanStor.Replace(wrapRecords(van...))
	graf.Reset()

	for i := 0; i < connections; i++ {
		flowStor.Add(vanflow.TransportBiflowRecord{
			BaseRecord:  vanflow.NewBase(fmt.Sprintf("tflow-%d", i), time.Now()),
			Parent:      ptrTo("listener-backend"),
			ConnectorID: ptrTo("connector-backend-1-6"),
			SourceHost:  ptrTo("10.111.0.111"),
		}, store.SourceRef{})
	}
	r := manager.runReconcile()
	r.Reconciled, r.Dirty = nil, 0
	assert.DeepEqual(b, r, reconcileResult{})
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		manager.runReconcile()
	}
}

func BenchmarkReconcile10(b *testing.B) {
	benchmarkRunReconcile(b, 10)
}
func BenchmarkReconcile3k(b *testing.B) {
	benchmarkRunReconcile(b, 3_000)
}
func BenchmarkReconcile10k(b *testing.B) {
	benchmarkRunReconcile(b, 10_000)
}
func benchmarkRunAppReconcile(b *testing.B, connections int, requests int) {
	tCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	tlog := slog.Default()
	vanStor := store.NewSyncMapStore(store.SyncMapStoreConfig{Indexers: RecordIndexers()})
	graf := NewGraph(vanStor).(*graph)
	manager := newConnectionmanager(tCtx, tlog, store.SourceRef{}, vanStor, graf, register(prometheus.NewRegistry()), time.Minute)
	defer manager.Stop()
	flowStor := manager.flows

	vanStor.Replace(wrapRecords(van...))
	graf.Reset()

	for i := 0; i < connections; i++ {
		flowStor.Add(vanflow.TransportBiflowRecord{
			BaseRecord:  vanflow.NewBase(fmt.Sprintf("tflow-%d", i), time.Now()),
			Parent:      ptrTo("listener-backend"),
			ConnectorID: ptrTo("connector-backend-1-6"),
			SourceHost:  ptrTo("10.111.0.111"),
		}, store.SourceRef{})
	}
	for i := 0; i < requests; i++ {
		id := fmt.Sprintf("appflow-%d", i)
		connID := fmt.Sprintf("tflow-%d", i%connections)
		flowStor.Add(vanflow.AppBiflowRecord{
			BaseRecord: vanflow.NewBase(id, time.Now(), time.Now()),
			Parent:     &connID,
			Protocol:   ptrTo("HTTP/1.1"),
			Method:     ptrTo("GET"),
			Result:     ptrTo("200"),
		}, store.SourceRef{})
	}
	r := manager.runReconcile()
	r.Reconciled, r.Dirty = nil, 0
	assert.DeepEqual(b, r, reconcileResult{})
	a := manager.runAppReconcile()
	a.Reconciled, a.Dirty = nil, 0
	assert.DeepEqual(b, a, appReconcileResult{})
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		manager.runAppReconcile()
	}
}

func BenchmarkAppReconcile10(b *testing.B) {
	benchmarkRunAppReconcile(b, 10, 10)
}
func BenchmarkAppReconcile3k(b *testing.B) {
	benchmarkRunAppReconcile(b, 100, 3_000)
}
func BenchmarkAppReconcile10k(b *testing.B) {
	benchmarkRunAppReconcile(b, 100, 10_000)
}

var van = []vanflow.Record{
	// site east and west
	vanflow.SiteRecord{BaseRecord: vanflow.NewBase("site-east"), Name: ptrTo("east")},
	vanflow.SiteRecord{BaseRecord: vanflow.NewBase("site-west"), Name: ptrTo("west")},

	// one router for west, two (HA) for east
	vanflow.RouterRecord{BaseRecord: vanflow.NewBase("router-0-west"), Parent: ptrTo("site-west"), Name: ptrTo("router-west")},
	vanflow.RouterRecord{BaseRecord: vanflow.NewBase("router-0-east"), Parent: ptrTo("site-east"), Name: ptrTo("router-0-east")},
	vanflow.RouterRecord{BaseRecord: vanflow.NewBase("router-1-east"), Parent: ptrTo("site-east"), Name: ptrTo("router-1-east")},

	// One backend listener on west
	vanflow.ListenerRecord{
		BaseRecord: vanflow.NewBase("listener-backend"),
		Parent:     ptrTo("router-0-west"),
		Address:    ptrTo("backend"),
		Protocol:   ptrTo("tcp"),
	},

	// backend connectors in east x2 routers x2 hosts
	vanflow.ConnectorRecord{
		BaseRecord: vanflow.NewBase("connector-backend-0-6"),
		Parent:     ptrTo("router-0-east"),
		Address:    ptrTo("backend"),
		Protocol:   ptrTo("tcp"),
		DestHost:   ptrTo("10.0.0.6"),
		DestPort:   ptrTo("8080"),
	},
	vanflow.ConnectorRecord{
		BaseRecord: vanflow.NewBase("connector-backend-1-6"),
		Parent:     ptrTo("router-1-east"),
		Address:    ptrTo("backend"),
		Protocol:   ptrTo("tcp"),
		DestHost:   ptrTo("10.0.0.6"),
		DestPort:   ptrTo("8080"),
	},
	vanflow.ConnectorRecord{
		BaseRecord: vanflow.NewBase("connector-backend-0-8"),
		Parent:     ptrTo("router-0-east"),
		Address:    ptrTo("backend"),
		Protocol:   ptrTo("tcp"),
		DestHost:   ptrTo("10.0.0.8"),
		DestPort:   ptrTo("8080"),
	},
	vanflow.ConnectorRecord{
		BaseRecord: vanflow.NewBase("connector-backend-1-8"),
		Parent:     ptrTo("router-1-east"),
		Address:    ptrTo("backend"),
		Protocol:   ptrTo("tcp"),
		DestHost:   ptrTo("10.0.0.8"),
		DestPort:   ptrTo("8080"),
	},

	// one client process at site west
	vanflow.ProcessRecord{
		BaseRecord: vanflow.NewBase("client-p01"),
		Parent:     ptrTo("site-west"),
		Name:       ptrTo("client-west-01"),
		SourceHost: ptrTo("10.111.0.111"),
		Group:      ptrTo("clients"),
	},
	// two service processes at site east
	vanflow.ProcessRecord{
		BaseRecord: vanflow.NewBase("backend-06"),
		Name:       ptrTo("server-east-06"),
		Parent:     ptrTo("site-east"),
		SourceHost: ptrTo("10.0.0.6"),
		Group:      ptrTo("servers"),
	},
	vanflow.ProcessRecord{
		BaseRecord: vanflow.NewBase("backend-08"),
		Parent:     ptrTo("site-east"),
		Name:       ptrTo("server-east-08"),
		SourceHost: ptrTo("10.0.0.8"),
		Group:      ptrTo("servers"),
	},

	// process groups
	ProcessGroupRecord{ID: "pg-01", Name: "clients"},
	ProcessGroupRecord{ID: "pg-02", Name: "servers"},
}

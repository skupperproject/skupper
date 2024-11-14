package collector

import (
	"testing"

	"github.com/skupperproject/skupper/pkg/vanflow"
	"github.com/skupperproject/skupper/pkg/vanflow/store"
	"gotest.tools/assert"
)

func TestGraphRelations(t *testing.T) {
	stor := store.NewSyncMapStore(store.SyncMapStoreConfig{Indexers: RecordIndexers()})

	stor.Replace(wrapRecords(
		vanflow.SiteRecord{BaseRecord: vanflow.NewBase("s1")},
		vanflow.RouterRecord{BaseRecord: vanflow.NewBase("r1"), Parent: ptrTo("s1")},
		vanflow.SiteRecord{BaseRecord: vanflow.NewBase("s2")},
		vanflow.RouterRecord{BaseRecord: vanflow.NewBase("r2"), Parent: ptrTo("s2")},
		vanflow.ProcessRecord{BaseRecord: vanflow.NewBase("p1"), Parent: ptrTo("s1"), SourceHost: ptrTo("x.x.x.1")},
		vanflow.ProcessRecord{BaseRecord: vanflow.NewBase("px"), Parent: ptrTo("sx"), SourceHost: ptrTo("x.x.x.1")},
		vanflow.ConnectorRecord{BaseRecord: vanflow.NewBase("c1"), Parent: ptrTo("r1"), DestHost: ptrTo("x.x.x.1"), Address: ptrTo("chips"), Protocol: ptrTo("snacks")},
		vanflow.ProcessRecord{BaseRecord: vanflow.NewBase("p2"), Parent: ptrTo("s2"), SourceHost: ptrTo("x.x.x.2")},
		vanflow.ConnectorRecord{BaseRecord: vanflow.NewBase("c2-a"), Parent: ptrTo("r2"), DestHost: ptrTo("x.x.x.2"), Address: ptrTo("pizza"), Protocol: ptrTo("snacks")},
		vanflow.ConnectorRecord{BaseRecord: vanflow.NewBase("c2-b"), Parent: ptrTo("r2"), ProcessID: ptrTo("p2"), Address: ptrTo("pizza"), Protocol: ptrTo("snacks")},
		vanflow.ConnectorRecord{BaseRecord: vanflow.NewBase("c2-c"), Parent: ptrTo("r2"), ProcessID: ptrTo("doesnotexist"), DestHost: ptrTo("x.x.x.2"), Address: ptrTo("pizza"), Protocol: ptrTo("snacks")},
		vanflow.ListenerRecord{BaseRecord: vanflow.NewBase("l2"), Parent: ptrTo("r2"), Address: ptrTo("chips"), Protocol: ptrTo("snacks")},
		AddressRecord{Name: "chips", ID: "adr1", Protocol: "snacks"},
		AddressRecord{Name: "pizza", ID: "adr2", Protocol: "snacks"},
	))
	g := NewGraph(stor).(*graph)
	g.Reset()
	for _, r := range stor.List() {
		g.Reindex(r.Record)
	}
	assert.Equal(t, g.Connector("c1").Target().ID(), "p1")
	assert.Equal(t, g.Connector("c2-a").Target().ID(), "p2")
	assert.Equal(t, g.Connector("c2-b").Target().ID(), "p2")
	assert.Equal(t, g.Connector("c2-c").Target().ID(), "p2")
	listenerConnectors := g.Listener("l2").Address().RoutingKey().Connectors()
	if len(listenerConnectors) != 1 {
		t.Errorf("expected examtly one connector for listener l2: %v", listenerConnectors)
	}
	assert.Equal(t, listenerConnectors[0].ID(), "c1")
}

func wrapRecords(records ...vanflow.Record) []store.Entry {
	entries := make([]store.Entry, len(records))
	for i := range records {
		entries[i].Record = records[i]
	}
	return entries
}

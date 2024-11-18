package collector

import (
	"testing"

	"github.com/skupperproject/skupper/pkg/vanflow"
	"github.com/skupperproject/skupper/pkg/vanflow/store"
	"gotest.tools/assert"
)

func TestGraphRelations(t *testing.T) {
	testCases := []struct {
		Name    string
		Records []vanflow.Record
		Expect  func(t *testing.T, graf Graph)
	}{
		{
			Name: "Connector::processID::Process",
			Records: []vanflow.Record{
				vanflow.ConnectorRecord{BaseRecord: vanflow.NewBase("c1"), ProcessID: ptrTo("p1")},
				vanflow.ProcessRecord{BaseRecord: vanflow.NewBase("p1")},
			},
			Expect: func(t *testing.T, graf Graph) {
				assert.Equal(t, graf.Connector("c1").Target().ID(), "p1")

			},
		}, {
			Name: "Connector::SiteHost::Process",
			Records: []vanflow.Record{
				vanflow.SiteRecord{BaseRecord: vanflow.NewBase("s1")},
				vanflow.RouterRecord{BaseRecord: vanflow.NewBase("r1"), Parent: ptrTo("s1")},
				vanflow.ConnectorRecord{
					BaseRecord: vanflow.NewBase("c1"),
					DestHost:   ptrTo("10.x.x.1"),
					Parent:     ptrTo("r1"), Protocol: ptrTo("test")},
				vanflow.ProcessRecord{BaseRecord: vanflow.NewBase("p1"),
					Parent: ptrTo("s1"), SourceHost: ptrTo("10.x.x.1")},
			},
			Expect: func(t *testing.T, graf Graph) {
				assert.Equal(t, graf.Connector("c1").Target().ID(), "p1")
				assert.Equal(t, graf.SiteHost(SiteHostID("s1", "10.x.x.1")).Process().ID(), "p1")
				assert.Equal(t, len(graf.SiteHost(SiteHostID("s1", "10.x.x.1")).Connectors()), 1)
			},
		}, {
			Name: "Connector::SiteHost::Process Ignores ProcessID",
			Records: []vanflow.Record{
				vanflow.SiteRecord{BaseRecord: vanflow.NewBase("s1")},
				vanflow.RouterRecord{BaseRecord: vanflow.NewBase("r1"), Parent: ptrTo("s1")},
				vanflow.ConnectorRecord{
					BaseRecord: vanflow.NewBase("c1"),
					ProcessID:  ptrTo("IGNORE_ME"),
					DestHost:   ptrTo("10.x.x.1"),
					Parent:     ptrTo("r1"), Protocol: ptrTo("test")},
				vanflow.ProcessRecord{BaseRecord: vanflow.NewBase("p1"),
					Parent: ptrTo("s1"), SourceHost: ptrTo("10.x.x.1")},
			},
			Expect: func(t *testing.T, graf Graph) {
				assert.Equal(t, graf.Connector("c1").Target().ID(), "p1")
				assert.Equal(t, graf.SiteHost(SiteHostID("s1", "10.x.x.1")).Process().ID(), "p1")
				assert.Equal(t, len(graf.SiteHost(SiteHostID("s1", "10.x.x.1")).Connectors()), 1)
			},
		}, {
			Name: "Listener::SiteHost::Process",
			Records: []vanflow.Record{
				vanflow.SiteRecord{BaseRecord: vanflow.NewBase("s1")},
				vanflow.RouterRecord{BaseRecord: vanflow.NewBase("r1"), Parent: ptrTo("s1")},
				vanflow.ListenerRecord{
					BaseRecord: vanflow.NewBase("l1"),
					Parent:     ptrTo("r1"), Protocol: ptrTo("test")},
				vanflow.ProcessRecord{BaseRecord: vanflow.NewBase("p1"),
					Parent: ptrTo("s1"), SourceHost: ptrTo("10.x.x.1")},
				vanflow.ProcessRecord{BaseRecord: vanflow.NewBase("p2"),
					Parent: ptrTo("OTHER_SITE"), SourceHost: ptrTo("10.x.x.1")},
			},
			Expect: func(t *testing.T, graf Graph) {
				siteID := graf.Listener("l1").Parent().Parent().ID()
				assert.Equal(t, siteID, "s1")
				proc, ok := graf.SiteHost(SiteHostID(siteID, "10.x.x.1")).Process().GetRecord()
				assert.Assert(t, ok)
				assert.Equal(t, proc.ID, "p1")
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			stor := store.NewSyncMapStore(store.SyncMapStoreConfig{Indexers: RecordIndexers()})
			stor.Replace(wrapRecords(tc.Records...))
			graf := NewGraph(stor).(*graph)
			graf.Reset()
			for _, r := range stor.List() {
				graf.Reindex(r.Record)
			}
			tc.Expect(t, graf)
		})
	}
}

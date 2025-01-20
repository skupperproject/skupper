package server

import (
	"net/http"

	"github.com/skupperproject/skupper/cmd/network-observer/internal/api"
	"github.com/skupperproject/skupper/cmd/network-observer/internal/collector"
	"github.com/skupperproject/skupper/cmd/network-observer/internal/server/views"
	"github.com/skupperproject/skupper/pkg/vanflow"
	"github.com/skupperproject/skupper/pkg/vanflow/store"
)

var _ api.ServerInterface = (*server)(nil)

// (GET /api/v2alpha1/connections)
func (s *server) Connections(w http.ResponseWriter, r *http.Request) {
	results := views.NewConnectionsSliceProvider(s.records)(listByType[collector.ConnectionRecord](s.records))
	if err := handleCollection(w, r, &api.ConnectionListResponse{}, results); err != nil {
		s.logWriteError(r, err)
	}
}

// (GET /api/v2alpha1/services/{id}/connections)
func (s *server) ConnectionsByService(w http.ResponseWriter, r *http.Request, id string) {
	getExemplar := fetchAndMap(s.records, func(a collector.AddressRecord) store.Entry {
		return store.Entry{Record: collector.ConnectionRecord{RoutingKey: a.Name, Protocol: a.Protocol}}
	}, id)
	if err := handleSubCollection(w, r, &api.ConnectionListResponse{}, getExemplar, func(exemplar store.Entry) []api.ConnectionRecord {
		return views.NewConnectionsSliceProvider(s.records)(index(s.records, collector.IndexFlowByAddress, exemplar))
	}); err != nil {
		s.logWriteError(r, err)
	}
}

func (s *server) Applicationflows(w http.ResponseWriter, r *http.Request) {
	results := views.NewRequestSliceProvider(s.records)(listByType[collector.RequestRecord](s.records))
	if err := handleCollection(w, r, &api.ApplicationFlowResponse{}, results); err != nil {
		s.logWriteError(r, err)
	}
}

// (GET /api/v2alpha1/services)
func (s *server) Services(w http.ResponseWriter, r *http.Request) {
	results := views.NewServiceSliceProvider(s.records, s.graph)(listByType[collector.AddressRecord](s.records))
	if err := handleCollection(w, r, &api.ServiceListResponse{}, results); err != nil {
		s.logWriteError(r, err)
	}
}

// (GET /api/v2alpha1/services/{id})
func (s *server) ServiceByID(w http.ResponseWriter, r *http.Request, id string) {
	getRecord := fetchAndMap(s.records, views.NewServiceProvider(s.records, s.graph), id)
	if err := handleSingle(w, r, &api.ServiceResponse{}, getRecord); err != nil {
		s.logWriteError(r, err)
	}
}

// (GET /api/v2alpha1/services/{id}/processes)
func (s *server) ProcessesByService(w http.ResponseWriter, r *http.Request, id string) {
	//todo(ck) find a way to more directly index this
	anode := s.graph.Address(id)
	if !anode.IsKnown() {
		resp := api.ErrorNotFound{
			Code: "ErrNotFound",
		}
		if err := encodeResponse(w, http.StatusNotFound, resp); err != nil {
			s.logWriteError(r, err)
		}
		return
	}
	cnodes := anode.RoutingKey().Connectors()
	entries := make([]store.Entry, 0, len(cnodes))
	for _, cnode := range cnodes {
		if entry, ok := cnode.Target().Get(); ok {
			entries = append(entries, entry)
		}
	}
	results := views.NewProcessSliceProvider(s.records, s.graph)(entries)
	if err := handleCollection(w, r, &api.ProcessListResponse{}, results); err != nil {
		s.logWriteError(r, err)
	}

}

// (GET /api/v2alpha1/services/{id}/processes)
func (s *server) ProcessPairsByService(w http.ResponseWriter, r *http.Request, id string) {
	//todo(ck) find a way to more directly index this
	addr, ok := s.graph.Address(id).GetRecord()
	if !ok {
		resp := api.ErrorNotFound{
			Code: "ErrNotFound",
		}
		if err := encodeResponse(w, http.StatusNotFound, resp); err != nil {
			s.logWriteError(r, err)
		}
		return
	}
	exemplar := store.Entry{
		Record: vanflow.ConnectorRecord{
			Protocol: &addr.Protocol,
			Address:  &addr.Name,
		},
	}
	connectors := index(s.records, collector.IndexByAddress, exemplar)
	targetProcessIDs := make(map[string]struct{})
	for _, c := range connectors {
		cn := s.graph.Connector(c.Record.Identity())
		pn := cn.Target()
		if pn.IsKnown() {
			targetProcessIDs[pn.ID()] = struct{}{}
		}
	}
	allProcPairs := views.NewProcessPairSliceProvider(s.graph)(listByType[collector.ProcPairRecord](s.records))
	results := make([]api.FlowAggregateRecord, 0)
	for _, proc := range allProcPairs {
		if _, ok := targetProcessIDs[proc.DestinationId]; ok {
			results = append(results, proc)
		}
	}
	if err := handleCollection(w, r, &api.FlowAggregateListResponse{}, results); err != nil {
		s.logWriteError(r, err)
	}
}

// (GET /api/v2alpha1/connectors)
func (s *server) Connectors(w http.ResponseWriter, r *http.Request) {
	results := views.NewConnectorSliceProvider(s.graph)(listByType[vanflow.ConnectorRecord](s.records))
	if err := handleCollection(w, r, &api.ConnectorListResponse{}, results); err != nil {
		s.logWriteError(r, err)
	}
}

// (GET /api/v2alpha1/connectors/{id})
func (s *server) ConnectorByID(w http.ResponseWriter, r *http.Request, id string) {
	getRecord := fetchAndMap(s.records, views.NewConnectorProvider(s.graph), id)
	if err := handleSingle(w, r, &api.ConnectorResponse{}, getRecord); err != nil {
		s.logWriteError(r, err)
	}
}

// Hosts deprecated
// (GET /api/v2alpha1/hosts)
func (s *server) Hosts(w http.ResponseWriter, r *http.Request) {
	if err := handleCollection(w, r, &api.SiteListResponse{}, []api.SiteRecord{}); err != nil {
		s.logWriteError(r, err)
	}
}

// HostsByID deprecated
// (GET /api/v2alpha1/hosts/{id})
func (s *server) HostsByID(w http.ResponseWriter, r *http.Request, id string) {
	if err := handleSingle(w, r, &api.SiteResponse{}, func() (r api.SiteRecord, found bool) {
		return r, false
	}); err != nil {
		s.logWriteError(r, err)
	}
}

// (GET /api/v2alpha1/links)
func (s *server) Links(w http.ResponseWriter, r *http.Request) {
	results := views.NewLinkSliceProvider(s.graph)(listByType[vanflow.LinkRecord](s.records))
	if err := handleCollection(w, r, &api.LinkListResponse{}, results); err != nil {
		s.logWriteError(r, err)
	}
}

// (GET /api/v2alpha1/links/{id})
func (s *server) LinkByID(w http.ResponseWriter, r *http.Request, id string) {
	getRecord := fetchAndConditionalMap(s.records, views.NewLinkProvider(s.graph), id)
	if err := handleSingle(w, r, &api.LinkResponse{}, getRecord); err != nil {
		s.logWriteError(r, err)
	}
}

// (GET /api/v2alpha1/listeners)
func (s *server) Listeners(w http.ResponseWriter, r *http.Request) {
	results := views.NewListenerSliceProvider(s.graph)(listByType[vanflow.ListenerRecord](s.records))
	if err := handleCollection(w, r, &api.ListenerListResponse{}, results); err != nil {
		s.logWriteError(r, err)
	}
}

// (GET /api/v2alpha1/listeners/{id})
func (s *server) ListenerByID(w http.ResponseWriter, r *http.Request, id string) {
	getRecord := fetchAndMap(s.records, views.NewListenerProvider(s.graph), id)
	if err := handleSingle(w, r, &api.ListenerResponse{}, getRecord); err != nil {
		s.logWriteError(r, err)
	}
}

// (GET /api/v2alpha1/processes)
func (s *server) Processes(w http.ResponseWriter, r *http.Request) {
	results := views.NewProcessSliceProvider(s.records, s.graph)(listByType[vanflow.ProcessRecord](s.records))
	if err := handleCollection(w, r, &api.ProcessListResponse{}, results); err != nil {
		s.logWriteError(r, err)
	}
}

// (GET /api/v2alpha1/processes/{id})
func (s *server) ProcessById(w http.ResponseWriter, r *http.Request, id string) {
	getRecord := fetchAndConditionalMap(s.records, views.NewProcessProvider(s.records, s.graph), id)
	if err := handleSingle(w, r, &api.ProcessResponse{}, getRecord); err != nil {
		s.logWriteError(r, err)
	}
}

// (GET /api/v2alpha1/componentpairs)
func (s *server) Componentpairs(w http.ResponseWriter, r *http.Request) {
	results := views.NewComponentPairSliceProvider()(listByType[collector.ProcGroupPairRecord](s.records))
	if err := handleCollection(w, r, &api.FlowAggregateListResponse{}, results); err != nil {
		s.logWriteError(r, err)
	}
}

// (GET /api/v2alpha1/componentpairs/{id})
func (s *server) ComponentpairByID(w http.ResponseWriter, r *http.Request, id string) {
	getRecord := fetchAndMap(s.records, views.NewComponentPairProvider(), id)
	if err := handleSingle(w, r, &api.FlowAggregateResponse{}, getRecord); err != nil {
		s.logWriteError(r, err)
	}
}

// (GET /api/v2alpha1/components)
func (s *server) Components(w http.ResponseWriter, r *http.Request) {
	results := views.NewComponentSliceProvider(s.records)(listByType[collector.ProcessGroupRecord](s.records))
	if err := handleCollection(w, r, &api.ComponentListResponse{}, results); err != nil {
		s.logWriteError(r, err)
	}
}

// (GET /api/v2alpha1/components/{id})
func (s *server) ComponentByID(w http.ResponseWriter, r *http.Request, id string) {
	getRecord := fetchAndMap(s.records, views.NewComponentProvider(s.records), id)
	if err := handleSingle(w, r, &api.ComponentResponse{}, getRecord); err != nil {
		s.logWriteError(r, err)
	}
}

// (GET /api/v2alpha1/processpairs)
func (s *server) Processpairs(w http.ResponseWriter, r *http.Request) {
	results := views.NewProcessPairSliceProvider(s.graph)(listByType[collector.ProcPairRecord](s.records))
	if err := handleCollection(w, r, &api.FlowAggregateListResponse{}, results); err != nil {
		s.logWriteError(r, err)
	}
}

// (GET /api/v2alpha1/processpairs/{id})
func (s *server) ProcesspairByID(w http.ResponseWriter, r *http.Request, id string) {
	getRecord := fetchAndMap(s.records, views.NewProcessPairProvider(s.graph), id)
	if err := handleSingle(w, r, &api.FlowAggregateResponse{}, getRecord); err != nil {
		s.logWriteError(r, err)
	}
}

// (GET /api/v2alpha1/routeraccess)
func (s *server) Routeraccess(w http.ResponseWriter, r *http.Request) {
	results := views.RouterAccessList(listByType[vanflow.RouterAccessRecord](s.records))
	if err := handleCollection(w, r, &api.RouterAccessListResponse{}, results); err != nil {
		s.logWriteError(r, err)
	}
}

// (GET /api/v2alpha1/routeraccess/{id})
func (s *server) RouteraccessByID(w http.ResponseWriter, r *http.Request, id string) {
	getRecord := fetchAndMap(s.records, views.RouterAccess, id)
	if err := handleSingle(w, r, &api.RouterAccessResponse{}, getRecord); err != nil {
		s.logWriteError(r, err)
	}
}

// (GET /api/v2alpha1/routerlinks)
func (s *server) Routerlinks(w http.ResponseWriter, r *http.Request) {
	results := views.NewRotuerLinkSliceProvider(s.graph)(listByType[vanflow.LinkRecord](s.records))
	if err := handleCollection(w, r, &api.RouterLinkListResponse{}, results); err != nil {
		s.logWriteError(r, err)
	}
}

// (GET /api/v2alpha1/routerlinks/{id})
func (s *server) RouterlinkByID(w http.ResponseWriter, r *http.Request, id string) {
	getRecord := fetchAndConditionalMap(s.records, views.NewRouterLinkProvider(s.graph), id)
	if err := handleSingle(w, r, &api.RouterLinkResponse{}, getRecord); err != nil {
		s.logWriteError(r, err)
	}
}

// (GET /api/v2alpha1/routers)
func (s *server) Routers(w http.ResponseWriter, r *http.Request) {
	results := views.Routers(listByType[vanflow.RouterRecord](s.records))
	if err := handleCollection(w, r, &api.RouterListResponse{}, results); err != nil {
		s.logWriteError(r, err)
	}
}

// (GET /api/v2alpha1/routers/{id})
func (s *server) RouterByID(w http.ResponseWriter, r *http.Request, id string) {
	getRecord := fetchAndMap(s.records, views.Router, id)
	if err := handleSingle(w, r, &api.RouterResponse{}, getRecord); err != nil {
		s.logWriteError(r, err)
	}
}

// (GET /api/v2alpha1/routers/{id}/links)
func (s *server) LinksByRouter(w http.ResponseWriter, r *http.Request, id string) {
	exemplar := store.Entry{Record: vanflow.LinkRecord{Parent: &id}}
	results := views.NewLinkSliceProvider(s.graph)(index(s.records, collector.IndexByTypeParent, exemplar))
	if err := handleCollection(w, r, &api.LinkListResponse{}, results); err != nil {
		s.logWriteError(r, err)
	}
}

// (GET /api/v2alpha1/sitepairs)
func (s *server) Sitepairs(w http.ResponseWriter, r *http.Request) {
	results := views.NewSitePairSliceProvider(s.graph)(listByType[collector.SitePairRecord](s.records))
	if err := handleCollection(w, r, &api.FlowAggregateListResponse{}, results); err != nil {
		s.logWriteError(r, err)
	}
}

// (GET /api/v2alpha1/sitepairs/{id})
func (s *server) SitepairByID(w http.ResponseWriter, r *http.Request, id string) {
}

// (GET /api/v2alpha1/sites)
func (s *server) Sites(w http.ResponseWriter, r *http.Request) {
	results := views.NewSiteSliceProvider(s.graph)(listByType[vanflow.SiteRecord](s.records))
	if err := handleCollection(w, r, &api.SiteListResponse{}, results); err != nil {
		s.logWriteError(r, err)
	}
}

// (GET /api/v2alpha1/sites/{id})
func (s *server) SiteById(w http.ResponseWriter, r *http.Request, id string) {
	getRecord := fetchAndMap(s.records, views.NewSiteProvider(s.graph), id)
	if err := handleSingle(w, r, &api.SiteResponse{}, getRecord); err != nil {
		s.logWriteError(r, err)
	}
}

// (GET /api/v2alpha1/sites/{id}/hosts)
func (s *server) HostsBySite(w http.ResponseWriter, r *http.Request, id string) {
	//TODO(ck) implement
	if err := handleCollection(w, r, &api.SiteListResponse{}, []api.SiteRecord{}); err != nil {
		s.logWriteError(r, err)
	}
}

// (GET /api/v2alpha1/sites/{id}/links)
func (s *server) LinksBySite(w http.ResponseWriter, r *http.Request, id string) {
	node := s.graph.Site(id)
	linkNodes := node.Links()
	linkEntries := make([]store.Entry, 0, len(linkNodes))
	for _, ln := range linkNodes {
		if le, ok := ln.Get(); ok {
			linkEntries = append(linkEntries, le)
		}
	}
	results := views.NewLinkSliceProvider(s.graph)(linkEntries)
	if err := handleCollection(w, r, &api.LinkListResponse{}, results); err != nil {
		s.logWriteError(r, err)
	}
}

// (GET /api/v2alpha1/sites/{id}/processes)
func (s *server) ProcessesBySite(w http.ResponseWriter, r *http.Request, id string) {
	exemplar := store.Entry{Record: vanflow.ProcessRecord{Parent: &id}}
	results := views.NewProcessSliceProvider(s.records, s.graph)(index(s.records, collector.IndexByTypeParent, exemplar))
	if err := handleCollection(w, r, &api.ProcessListResponse{}, results); err != nil {
		s.logWriteError(r, err)
	}
}

// (GET /api/v2alpha1/sites/{id}/routers)
func (s *server) RoutersBySite(w http.ResponseWriter, r *http.Request, id string) {
	exemplar := store.Entry{Record: vanflow.RouterRecord{Parent: &id}}
	results := views.Routers(index(s.records, collector.IndexByTypeParent, exemplar))
	if err := handleCollection(w, r, &api.RouterListResponse{}, results); err != nil {
		s.logWriteError(r, err)
	}
}

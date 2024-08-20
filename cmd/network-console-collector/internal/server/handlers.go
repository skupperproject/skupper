package server

import (
	"net/http"

	"github.com/skupperproject/skupper/cmd/network-console-collector/internal/api"
	"github.com/skupperproject/skupper/cmd/network-console-collector/internal/collector"
	"github.com/skupperproject/skupper/cmd/network-console-collector/internal/server/views"
	"github.com/skupperproject/skupper/pkg/vanflow"
	"github.com/skupperproject/skupper/pkg/vanflow/store"
)

var _ api.ServerInterface = (*server)(nil)

// (GET /api/v1alpha1/addresses/)
func (s *server) Addresses(w http.ResponseWriter, r *http.Request) {
	results := views.NewAddressSliceProvider(s.graph)(listByType[collector.AddressRecord](s.records))
	if err := handleCollection(w, r, &api.AddressListResponse{}, results); err != nil {
		s.logWriteError(r, err)
	}
}

// (GET /api/v1alpha1/addresses/{id}/)
func (s *server) AddressByID(w http.ResponseWriter, r *http.Request, id string) {
	getRecord := fetchAndMap(s.records, views.NewAddressProvider(s.graph), id)
	if err := handleSingle(w, r, &api.AddressResponse{}, getRecord); err != nil {
		s.logWriteError(r, err)
	}
}

// (GET /api/v1alpha1/addresses/{id}/connectors/)
func (s *server) ConnectorsByAddress(w http.ResponseWriter, r *http.Request, id string) {
}

// (GET /api/v1alpha1/addresses/{id}/listeners/)
func (s *server) ListenersByAddress(w http.ResponseWriter, r *http.Request, id string) {
}

// (GET /api/v1alpha1/addresses/{id}/processes/)
func (s *server) ProcessesByAddress(w http.ResponseWriter, r *http.Request, id string) {
	//TODO(ck) implement
	if err := handleCollection(w, r, &api.ProcessListResponse{}, []api.ProcessRecord{}); err != nil {
		s.logWriteError(r, err)
	}
}

// (GET /api/v1alpha1/addresses/{id}/processpairs/)
func (s *server) ProcessPairsByAddress(w http.ResponseWriter, r *http.Request, id string) {
	//TODO(ck) implement
	if err := handleCollection(w, r, &api.FlowAggregateListResponse{}, []api.FlowAggregateRecord{}); err != nil {
		s.logWriteError(r, err)
	}
}

// (GET /api/v1alpha1/connectors/)
func (s *server) Connectors(w http.ResponseWriter, r *http.Request) {
	results := views.NewConnectorSliceProvider(s.graph)(listByType[vanflow.ConnectorRecord](s.records))
	if err := handleCollection(w, r, &api.ConnectorListResponse{}, results); err != nil {
		s.logWriteError(r, err)
	}
}

// (GET /api/v1alpha1/connectors/{id}/)
func (s *server) ConnectorByID(w http.ResponseWriter, r *http.Request, id string) {
	getRecord := fetchAndMap(s.records, views.NewConnectorProvider(s.graph), id)
	if err := handleSingle(w, r, &api.ConnectorResponse{}, getRecord); err != nil {
		s.logWriteError(r, err)
	}
}

// Hosts deprecated
// (GET /api/v1alpha1/hosts/)
func (s *server) Hosts(w http.ResponseWriter, r *http.Request) {
	if err := handleCollection(w, r, &api.SiteListResponse{}, []api.SiteRecord{}); err != nil {
		s.logWriteError(r, err)
	}
}

// HostsByID deprecated
// (GET /api/v1alpha1/hosts/{id}/)
func (s *server) HostsByID(w http.ResponseWriter, r *http.Request, id string) {
	if err := handleSingle(w, r, &api.SiteResponse{}, func() (r api.SiteRecord, found bool) {
		return r, false
	}); err != nil {
		s.logWriteError(r, err)
	}
}

// (GET /api/v1alpha1/links/)
func (s *server) Links(w http.ResponseWriter, r *http.Request) {
	results := views.NewLinkSliceProvider(s.graph)(listByType[vanflow.LinkRecord](s.records))
	if err := handleCollection(w, r, &api.LinkListResponse{}, results); err != nil {
		s.logWriteError(r, err)
	}
}

// (GET /api/v1alpha1/links/{id}/)
func (s *server) LinkByID(w http.ResponseWriter, r *http.Request, id string) {
	getRecord := fetchAndConditionalMap(s.records, views.NewLinkProvider(s.graph), id)
	if err := handleSingle(w, r, &api.LinkResponse{}, getRecord); err != nil {
		s.logWriteError(r, err)
	}
}

// (GET /api/v1alpha1/listeners/)
func (s *server) Listeners(w http.ResponseWriter, r *http.Request) {
	results := views.NewListenerSliceProvider(s.graph)(listByType[vanflow.ListenerRecord](s.records))
	if err := handleCollection(w, r, &api.ListenerListResponse{}, results); err != nil {
		s.logWriteError(r, err)
	}
}

// (GET /api/v1alpha1/listeners/{id}/)
func (s *server) ListenerByID(w http.ResponseWriter, r *http.Request, id string) {
	getRecord := fetchAndMap(s.records, views.NewListenerProvider(s.graph), id)
	if err := handleSingle(w, r, &api.ListenerResponse{}, getRecord); err != nil {
		s.logWriteError(r, err)
	}
}

// (GET /api/v1alpha1/listeners/{id}/flows)
func (s *server) FlowsByListener(w http.ResponseWriter, r *http.Request, id string) {
}

// (GET /api/v1alpha1/processes/)
func (s *server) Processes(w http.ResponseWriter, r *http.Request) {
	results := views.NewProcessSliceProvider(s.records, s.graph)(listByType[vanflow.ProcessRecord](s.records))
	if err := handleCollection(w, r, &api.ProcessListResponse{}, results); err != nil {
		s.logWriteError(r, err)
	}
}

// (GET /api/v1alpha1/processes/{id}/)
func (s *server) ProcessById(w http.ResponseWriter, r *http.Request, id string) {
	getRecord := fetchAndMap(s.records, views.NewProcessProvider(s.records, s.graph), id)
	if err := handleSingle(w, r, &api.ProcessResponse{}, getRecord); err != nil {
		s.logWriteError(r, err)
	}
}

// (GET /api/v1alpha1/processes/{id}/addresses/)
func (s *server) AddressesByProcess(w http.ResponseWriter, r *http.Request, id string) {
	//TODO(ck) implement
	if err := handleCollection(w, r, &api.AddressListResponse{}, []api.AddressRecord{}); err != nil {
		s.logWriteError(r, err)
	}
}

// (GET /api/v1alpha1/processes/{id}/connector/)
func (s *server) ConnectorByProcess(w http.ResponseWriter, r *http.Request, id string) {
}

// (GET /api/v1alpha1/processgrouppairs/)
func (s *server) Processgrouppairs(w http.ResponseWriter, r *http.Request) {
	//TODO(ck) implement
	if err := handleCollection(w, r, &api.FlowAggregateListResponse{}, []api.FlowAggregateRecord{}); err != nil {
		s.logWriteError(r, err)
	}
}

// (GET /api/v1alpha1/processgrouppairs/{id}/)
func (s *server) ProcessgrouppairByID(w http.ResponseWriter, r *http.Request, id string) {
}

// (GET /api/v1alpha1/processgroups/)
func (s *server) Processgroups(w http.ResponseWriter, r *http.Request) {
	results := views.NewProcessGroupSliceProvider(s.records)(listByType[collector.ProcessGroupRecord](s.records))
	if err := handleCollection(w, r, &api.ProcessGroupListResponse{}, results); err != nil {
		s.logWriteError(r, err)
	}
}

// (GET /api/v1alpha1/processgroups/{id}/)
func (s *server) ProcessgroupByID(w http.ResponseWriter, r *http.Request, id string) {
	getRecord := fetchAndMap(s.records, views.NewProcessGroupProvider(s.records), id)
	if err := handleSingle(w, r, &api.ProcessGroupResponse{}, getRecord); err != nil {
		s.logWriteError(r, err)
	}
}

// (GET /api/v1alpha1/processgroups/{id}/processes/)
func (s *server) ProcessesByProcessGroup(w http.ResponseWriter, r *http.Request, id string) {
}

// (GET /api/v1alpha1/processpairs/)
func (s *server) Processpairs(w http.ResponseWriter, r *http.Request) {
	//TODO(ck) implement
	if err := handleCollection(w, r, &api.FlowAggregateListResponse{}, []api.FlowAggregateRecord{}); err != nil {
		s.logWriteError(r, err)
	}
}

// (GET /api/v1alpha1/processpairs/{id}/)
func (s *server) ProcesspairByID(w http.ResponseWriter, r *http.Request, id string) {
}

// (GET /api/v1alpha1/routeraccess/)
func (s *server) Routeraccess(w http.ResponseWriter, r *http.Request) {
	results := views.RouterAccessList(listByType[vanflow.RouterAccessRecord](s.records))
	if err := handleCollection(w, r, &api.RouterAccessListResponse{}, results); err != nil {
		s.logWriteError(r, err)
	}
}

// (GET /api/v1alpha1/routeraccess/{id}/)
func (s *server) RouteraccessByID(w http.ResponseWriter, r *http.Request, id string) {
	getRecord := fetchAndMap(s.records, views.RouterAccess, id)
	if err := handleSingle(w, r, &api.RouterAccessResponse{}, getRecord); err != nil {
		s.logWriteError(r, err)
	}
}

// (GET /api/v1alpha1/routerlinks/)
func (s *server) Routerlinks(w http.ResponseWriter, r *http.Request) {
	results := views.NewRotuerLinkSliceProvider(s.graph)(listByType[vanflow.LinkRecord](s.records))
	if err := handleCollection(w, r, &api.RouterLinkListResponse{}, results); err != nil {
		s.logWriteError(r, err)
	}
}

// (GET /api/v1alpha1/routerlinks/{id}/)
func (s *server) RouterlinkByID(w http.ResponseWriter, r *http.Request, id string) {
	getRecord := fetchAndConditionalMap(s.records, views.NewRouterLinkProvider(s.graph), id)
	if err := handleSingle(w, r, &api.RouterLinkResponse{}, getRecord); err != nil {
		s.logWriteError(r, err)
	}
}

// (GET /api/v1alpha1/routers/)
func (s *server) Routers(w http.ResponseWriter, r *http.Request) {
	results := views.Routers(listByType[vanflow.RouterRecord](s.records))
	if err := handleCollection(w, r, &api.RouterListResponse{}, results); err != nil {
		s.logWriteError(r, err)
	}
}

// (GET /api/v1alpha1/routers/{id}/)
func (s *server) RouterByID(w http.ResponseWriter, r *http.Request, id string) {
	getRecord := fetchAndMap(s.records, views.Router, id)
	if err := handleSingle(w, r, &api.RouterResponse{}, getRecord); err != nil {
		s.logWriteError(r, err)
	}
}

// (GET /api/v1alpha1/routers/{id}/connectors/)
func (s *server) ConnectorsByRouter(w http.ResponseWriter, r *http.Request, id string) {
}

// (GET /api/v1alpha1/routers/{id}/flows/)
func (s *server) FlowsByRouter(w http.ResponseWriter, r *http.Request, id string) {
}

// (GET /api/v1alpha1/routers/{id}/links/)
func (s *server) LinksByRouter(w http.ResponseWriter, r *http.Request, id string) {
	//TODO(ck) implement
	if err := handleCollection(w, r, &api.LinkListResponse{}, []api.LinkRecord{}); err != nil {
		s.logWriteError(r, err)
	}
}

// (GET /api/v1alpha1/routers/{id}/listeners/)
func (s *server) ListenersByRouter(w http.ResponseWriter, r *http.Request, id string) {
}

// (GET /api/v1alpha1/sitepairs/)
func (s *server) Sitepairs(w http.ResponseWriter, r *http.Request) {
	//TODO(ck) implement
	if err := handleCollection(w, r, &api.FlowAggregateListResponse{}, []api.FlowAggregateRecord{}); err != nil {
		s.logWriteError(r, err)
	}
}

// (GET /api/v1alpha1/sitepairs/{id}/)
func (s *server) SitepairByID(w http.ResponseWriter, r *http.Request, id string) {
}

// (GET /api/v1alpha1/sites/)
func (s *server) Sites(w http.ResponseWriter, r *http.Request) {
	results := views.Sites(listByType[vanflow.SiteRecord](s.records))
	if err := handleCollection(w, r, &api.SiteListResponse{}, results); err != nil {
		s.logWriteError(r, err)
	}
}

// (GET /api/v1alpha1/sites/{id}/)
func (s *server) SiteById(w http.ResponseWriter, r *http.Request, id string) {
	getRecord := fetchAndMap(s.records, views.Site, id)
	if err := handleSingle(w, r, &api.SiteResponse{}, getRecord); err != nil {
		s.logWriteError(r, err)
	}
}

// (GET /api/v1alpha1/sites/{id}/flows/)
func (s *server) FlowsBySite(w http.ResponseWriter, r *http.Request, id string) {
}

// (GET /api/v1alpha1/sites/{id}/hosts/)
func (s *server) HostsBySite(w http.ResponseWriter, r *http.Request, id string) {
	//TODO(ck) implement
	if err := handleCollection(w, r, &api.SiteListResponse{}, []api.SiteRecord{}); err != nil {
		s.logWriteError(r, err)
	}
}

// (GET /api/v1alpha1/sites/{id}/links/)
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

// (GET /api/v1alpha1/sites/{id}/processes/)
func (s *server) ProcessesBySite(w http.ResponseWriter, r *http.Request, id string) {
	exemplar := store.Entry{Record: vanflow.ProcessRecord{Parent: &id}}
	results := views.NewProcessSliceProvider(s.records, s.graph)(index(s.records, collector.IndexByTypeParent, exemplar))
	if err := handleCollection(w, r, &api.ProcessListResponse{}, results); err != nil {
		s.logWriteError(r, err)
	}
}

// (GET /api/v1alpha1/sites/{id}/routers/)
func (s *server) RoutersBySite(w http.ResponseWriter, r *http.Request, id string) {
	exemplar := store.Entry{Record: vanflow.RouterRecord{Parent: &id}}
	results := views.Routers(index(s.records, collector.IndexByTypeParent, exemplar))
	if err := handleCollection(w, r, &api.RouterListResponse{}, results); err != nil {
		s.logWriteError(r, err)
	}
}

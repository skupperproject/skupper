package server

import (
	"net/http"

	"github.com/skupperproject/skupper/cmd/network-console-collector/internal/api"
	"github.com/skupperproject/skupper/cmd/network-console-collector/internal/server/views"
	"github.com/skupperproject/skupper/pkg/vanflow"
)

var _ api.ServerInterface = (*server)(nil)

// (GET /api/v1alpha1/addresses/)
func (s *server) Addresses(w http.ResponseWriter, r *http.Request) {}

// (GET /api/v1alpha1/addresses/{id}/)
func (s *server) AddressByID(w http.ResponseWriter, r *http.Request, id string) {}

// (GET /api/v1alpha1/addresses/{id}/connectors/)
func (s *server) ConnectorsByAddress(w http.ResponseWriter, r *http.Request, id string) {}

// (GET /api/v1alpha1/addresses/{id}/listeners/)
func (s *server) ListenersByAddress(w http.ResponseWriter, r *http.Request, id string) {}

// (GET /api/v1alpha1/addresses/{id}/processes/)
func (s *server) ProcessesByAddress(w http.ResponseWriter, r *http.Request, id string) {}

// (GET /api/v1alpha1/addresses/{id}/processpairs/)
func (s *server) ProcessPairsByAddress(w http.ResponseWriter, r *http.Request, id string) {}

// (GET /api/v1alpha1/connectors/)
func (s *server) Connectors(w http.ResponseWriter, r *http.Request) {}

// (GET /api/v1alpha1/connectors/{id}/)
func (s *server) ConnectorByID(w http.ResponseWriter, r *http.Request, id string) {}

// (GET /api/v1alpha1/hosts/)
func (s *server) Hosts(w http.ResponseWriter, r *http.Request) {}

// (GET /api/v1alpha1/hosts/{id}/)
func (s *server) HostsByID(w http.ResponseWriter, r *http.Request, id string) {}

// (GET /api/v1alpha1/links/)
func (s *server) Links(w http.ResponseWriter, r *http.Request) {}

// (GET /api/v1alpha1/links/{id}/)
func (s *server) LinkByID(w http.ResponseWriter, r *http.Request, id string) {}

// (GET /api/v1alpha1/listeners/)
func (s *server) Listeners(w http.ResponseWriter, r *http.Request) {}

// (GET /api/v1alpha1/listeners/{id}/)
func (s *server) ListenerByID(w http.ResponseWriter, r *http.Request, id string) {}

// (GET /api/v1alpha1/listeners/{id}/flows)
func (s *server) FlowsByListener(w http.ResponseWriter, r *http.Request, id string) {}

// (GET /api/v1alpha1/processes/)
func (s *server) Processes(w http.ResponseWriter, r *http.Request) {}

// (GET /api/v1alpha1/processes/{id}/)
func (s *server) ProcessById(w http.ResponseWriter, r *http.Request, id string) {}

// (GET /api/v1alpha1/processes/{id}/addresses/)
func (s *server) AddressesByProcess(w http.ResponseWriter, r *http.Request, id string) {}

// (GET /api/v1alpha1/processes/{id}/connector/)
func (s *server) ConnectorByProcess(w http.ResponseWriter, r *http.Request, id string) {}

// (GET /api/v1alpha1/processgrouppairs/)
func (s *server) Processgrouppairs(w http.ResponseWriter, r *http.Request) {}

// (GET /api/v1alpha1/processgrouppairs/{id}/)
func (s *server) ProcessgrouppairByID(w http.ResponseWriter, r *http.Request, id string) {}

// (GET /api/v1alpha1/processgroups/)
func (s *server) Processgroups(w http.ResponseWriter, r *http.Request) {}

// (GET /api/v1alpha1/processgroups/{id}/)
func (s *server) ProcessgroupByID(w http.ResponseWriter, r *http.Request, id string) {}

// (GET /api/v1alpha1/processgroups/{id}/processes/)
func (s *server) ProcessesByProcessGroup(w http.ResponseWriter, r *http.Request, id string) {}

// (GET /api/v1alpha1/processpairs/)
func (s *server) Processpairs(w http.ResponseWriter, r *http.Request) {}

// (GET /api/v1alpha1/processpairs/{id}/)
func (s *server) ProcesspairByID(w http.ResponseWriter, r *http.Request, id string) {}

// (GET /api/v1alpha1/routeraccess/)
func (s *server) Routeraccess(w http.ResponseWriter, r *http.Request) {}

// (GET /api/v1alpha1/routeraccess/{id}/)
func (s *server) RouteraccessByID(w http.ResponseWriter, r *http.Request, id string) {}

// (GET /api/v1alpha1/routerlinks/)
func (s *server) Routerlinks(w http.ResponseWriter, r *http.Request) {}

// (GET /api/v1alpha1/routerlinks/{id}/)
func (s *server) RouterlinkByID(w http.ResponseWriter, r *http.Request, id string) {}

// (GET /api/v1alpha1/routers/)
func (s *server) Routers(w http.ResponseWriter, r *http.Request) {}

// (GET /api/v1alpha1/routers/{id}/)
func (s *server) RouterByID(w http.ResponseWriter, r *http.Request, id string) {}

// (GET /api/v1alpha1/routers/{id}/connectors/)
func (s *server) ConnectorsByRouter(w http.ResponseWriter, r *http.Request, id string) {}

// (GET /api/v1alpha1/routers/{id}/flows/)
func (s *server) FlowsByRouter(w http.ResponseWriter, r *http.Request, id string) {}

// (GET /api/v1alpha1/routers/{id}/links/)
func (s *server) LinksByRouter(w http.ResponseWriter, r *http.Request, id string) {}

// (GET /api/v1alpha1/routers/{id}/listeners/)
func (s *server) ListenersByRouter(w http.ResponseWriter, r *http.Request, id string) {}

// (GET /api/v1alpha1/sitepairs/)
func (s *server) Sitepairs(w http.ResponseWriter, r *http.Request) {}

// (GET /api/v1alpha1/sitepairs/{id}/)
func (s *server) SitepairByID(w http.ResponseWriter, r *http.Request, id string) {}

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
func (s *server) FlowsBySite(w http.ResponseWriter, r *http.Request, id string) {}

// (GET /api/v1alpha1/sites/{id}/hosts/)
func (s *server) HostsBySite(w http.ResponseWriter, r *http.Request, id string) {}

// (GET /api/v1alpha1/sites/{id}/links/)
func (s *server) LinksBySite(w http.ResponseWriter, r *http.Request, id string) {}

// (GET /api/v1alpha1/sites/{id}/processes/)
func (s *server) ProcessesBySite(w http.ResponseWriter, r *http.Request, id string) {}

// (GET /api/v1alpha1/sites/{id}/routers/)
func (s *server) RoutersBySite(w http.ResponseWriter, r *http.Request, id string) {}

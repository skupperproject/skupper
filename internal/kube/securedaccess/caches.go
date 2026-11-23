package securedaccess

import "github.com/skupperproject/skupper/internal/kube/watchers"

// ResourceCaches are controller-owned inventories used by snapshots.
// Optional provider caches remain nil when unavailable at registration; registered
// caches must pass the controller's startup sync barrier before scopes execute.
type ResourceCaches struct {
	Services                         *watchers.ServiceWatcher
	Routes                           *watchers.RouteWatcher
	Ingresses                        *watchers.IngressWatcher
	HTTPProxies, TLSRoutes, Gateways *watchers.DynamicWatcher
	Accesses                         *watchers.SecuredAccessWatcher
	Certificates                     *watchers.CertificateWatcher
}

func (m *SecuredAccessManager) BindCaches(caches *ResourceCaches) { m.watcher = caches }

// Return the existing stable handlers, never a new identity per invalidation.
func (m *SecuredAccessManager) CertificateHandler() watchers.ResourceChangeHandler {
	return m.certificates
}
func (m *SecuredAccessManager) GatewayHandler() watchers.ResourceChangeHandler { return m.gateway }

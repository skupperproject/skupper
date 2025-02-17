package securedaccess

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/informers/internalinterfaces"

	routev1interfaces "github.com/openshift/client-go/route/informers/externalversions/internalinterfaces"

	internalclient "github.com/skupperproject/skupper/internal/kube/client"
	skupperv2alpha1 "github.com/skupperproject/skupper/pkg/apis/skupper/v2alpha1"
)

type SecuredAccessResourceWatcher struct {
	accessMgr            *SecuredAccessManager
	serviceWatcher       *internalclient.ServiceWatcher
	routeWatcher         *internalclient.RouteWatcher
	ingressWatcher       *internalclient.IngressWatcher
	httpProxyWatcher     *internalclient.DynamicWatcher
	tlsRouteWatcher      *internalclient.DynamicWatcher
	securedAccessWatcher *internalclient.SecuredAccessWatcher
}

func NewSecuredAccessResourceWatcher(accessMgr *SecuredAccessManager) *SecuredAccessResourceWatcher {
	return &SecuredAccessResourceWatcher{
		accessMgr: accessMgr,
	}
}

func (m *SecuredAccessResourceWatcher) WatchResources(controller *internalclient.Controller, namespace string) {
	m.serviceWatcher = controller.WatchServices(coreSecuredAccess(), namespace, internalclient.FilterByNamespace(m.isControlledResource, m.accessMgr.CheckService))
	m.ingressWatcher = controller.WatchIngresses(coreSecuredAccess(), namespace, internalclient.FilterByNamespace(m.isControlledResource, m.accessMgr.CheckIngress))
	m.routeWatcher = controller.WatchRoutes(routeSecuredAccess(), namespace, internalclient.FilterByNamespace(m.isControlledResource, m.accessMgr.CheckRoute))
	m.httpProxyWatcher = controller.WatchContourHttpProxies(dynamicSecuredAccess(), namespace, internalclient.FilterByNamespace(m.isControlledResource, m.accessMgr.CheckHttpProxy))
	m.tlsRouteWatcher = controller.WatchTlsRoutes(dynamicSecuredAccess(), namespace, internalclient.FilterByNamespace(m.isControlledResource, m.accessMgr.CheckTlsRoute))
}

func (m *SecuredAccessResourceWatcher) WatchGateway(controller *internalclient.Controller, namespace string) {
	controller.WatchGateways(dynamicByName("skupper"), namespace, internalclient.FilterByNamespace(m.isControlledResource, m.accessMgr.CheckGateway))
}

func (m *SecuredAccessResourceWatcher) WatchSecuredAccesses(controller *internalclient.Controller, namespace string, handler internalclient.SecuredAccessHandler) {
	f := func(key string, sa *skupperv2alpha1.SecuredAccess) error {
		if sa == nil {
			return m.accessMgr.SecuredAccessDeleted(key)
		}
		if handler != nil {
			handler(key, sa)
		}
		return m.accessMgr.SecuredAccessChanged(key, sa)
	}
	m.securedAccessWatcher = controller.WatchSecuredAccesses(namespace, internalclient.FilterByNamespace(m.isControlledResource, f))
}

func (m *SecuredAccessResourceWatcher) Recover() {
	for _, service := range m.serviceWatcher.List() {
		if !m.isControlledResource(service.Namespace) {
			continue
		}
		m.accessMgr.RecoverService(service)
	}
	if m.routeWatcher != nil {
		for _, route := range m.routeWatcher.List() {
			if !m.isControlledResource(route.Namespace) {
				continue
			}
			m.accessMgr.RecoverRoute(route)
		}
	}
	if m.ingressWatcher != nil {
		for _, ingress := range m.ingressWatcher.List() {
			if !m.isControlledResource(ingress.Namespace) {
				continue
			}
			m.accessMgr.RecoverIngress(ingress)
		}
	}
	if m.httpProxyWatcher != nil {
		for _, httpProxy := range m.httpProxyWatcher.List() {
			if !m.isControlledResource(httpProxy.GetNamespace()) {
				continue
			}
			m.accessMgr.RecoverHttpProxy(httpProxy)
		}
	}
	if m.tlsRouteWatcher != nil {
		for _, route := range m.tlsRouteWatcher.List() {
			if !m.isControlledResource(route.GetNamespace()) {
				continue
			}
			m.accessMgr.RecoverTlsRoute(route)
		}
	}
	//once all resources are recovered, can process definitions
	for _, sa := range m.securedAccessWatcher.List() {
		if !m.isControlledResource(sa.Namespace) {
			continue
		}
		m.accessMgr.SecuredAccessChanged(sa.Namespace+"/"+sa.Name, sa)
	}
}

func (m *SecuredAccessResourceWatcher) isControlledResource(namespace string) bool {
	if m.accessMgr.context != nil {
		return m.accessMgr.context.IsControlled(namespace)
	}
	return true
}

func coreSecuredAccess() internalinterfaces.TweakListOptionsFunc {
	return func(options *metav1.ListOptions) {
		options.LabelSelector = "internal.skupper.io/secured-access"
	}
}

func routeSecuredAccess() routev1interfaces.TweakListOptionsFunc {
	return func(options *metav1.ListOptions) {
		options.LabelSelector = "internal.skupper.io/secured-access"
	}
}

func dynamicSecuredAccess() dynamicinformer.TweakListOptionsFunc {
	return func(options *metav1.ListOptions) {
		options.LabelSelector = "internal.skupper.io/secured-access"
	}
}

func dynamicByName(name string) dynamicinformer.TweakListOptionsFunc {
	return func(options *metav1.ListOptions) {
		options.FieldSelector = "metadata.name=" + name
	}
}

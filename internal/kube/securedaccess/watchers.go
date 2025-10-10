package securedaccess

import (
	"errors"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/informers/internalinterfaces"

	routev1interfaces "github.com/openshift/client-go/route/informers/externalversions/internalinterfaces"

	"github.com/skupperproject/skupper/internal/kube/watchers"
	skupperv2alpha1 "github.com/skupperproject/skupper/pkg/apis/skupper/v2alpha1"
)

type SecuredAccessResourceWatcher struct {
	accessMgr            *SecuredAccessManager
	serviceWatcher       *watchers.ServiceWatcher
	routeWatcher         *watchers.RouteWatcher
	ingressWatcher       *watchers.IngressWatcher
	httpProxyWatcher     *watchers.DynamicWatcher
	tlsRouteWatcher      *watchers.DynamicWatcher
	securedAccessWatcher *watchers.SecuredAccessWatcher
}

func NewSecuredAccessResourceWatcher(accessMgr *SecuredAccessManager) *SecuredAccessResourceWatcher {
	return &SecuredAccessResourceWatcher{
		accessMgr: accessMgr,
	}
}

func (m *SecuredAccessResourceWatcher) WatchResources(processor *watchers.EventProcessor, namespace string) {
	m.serviceWatcher = processor.WatchServices(coreSecuredAccess(), namespace, watchers.FilterByNamespace(m.isControlledResource, m.accessMgr.CheckService))
	m.ingressWatcher = processor.WatchIngresses(coreSecuredAccess(), namespace, watchers.FilterByNamespace(m.isControlledResource, m.accessMgr.CheckIngress))
	m.routeWatcher = processor.WatchRoutes(routeSecuredAccess(), namespace, watchers.FilterByNamespace(m.isControlledResource, m.accessMgr.CheckRoute))
	m.httpProxyWatcher = processor.WatchContourHttpProxies(dynamicSecuredAccess(), namespace, watchers.FilterByNamespace(m.isControlledResource, m.accessMgr.CheckHttpProxy))
	m.tlsRouteWatcher = processor.WatchTlsRoutes(dynamicSecuredAccess(), namespace, watchers.FilterByNamespace(m.isControlledResource, m.accessMgr.CheckTlsRoute))
}

func (m *SecuredAccessResourceWatcher) WatchGateway(processor *watchers.EventProcessor, namespace string) {
	processor.WatchGateways(dynamicByName("skupper"), namespace, watchers.FilterByNamespace(m.isControlledResource, m.accessMgr.CheckGateway))
}

func (m *SecuredAccessResourceWatcher) WatchSecuredAccesses(processor *watchers.EventProcessor, namespace string, handler watchers.SecuredAccessHandler) {
	var wrappedHandler = m.handleSecuredAccess
	if handler != nil {
		wrappedHandler = func(key string, sa *skupperv2alpha1.SecuredAccess) error {
			return errors.Join(handler(key, sa), m.handleSecuredAccess(key, sa))
		}
	}
	m.securedAccessWatcher = processor.WatchSecuredAccesses(namespace, watchers.FilterByNamespace(m.isControlledResource, wrappedHandler))
}

func (m *SecuredAccessResourceWatcher) handleSecuredAccess(key string, sa *skupperv2alpha1.SecuredAccess) error {
	if sa == nil {
		return m.accessMgr.SecuredAccessDeleted(key)
	}
	return m.accessMgr.SecuredAccessChanged(key, sa)
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

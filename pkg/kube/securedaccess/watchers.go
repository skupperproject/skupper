package securedaccess

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/informers/internalinterfaces"

	routev1interfaces "github.com/openshift/client-go/route/informers/externalversions/internalinterfaces"

	skupperv2alpha1 "github.com/skupperproject/skupper/pkg/apis/skupper/v2alpha1"
	"github.com/skupperproject/skupper/pkg/kube"
)

type SecuredAccessResourceWatcher struct {
	accessMgr            *SecuredAccessManager
	serviceWatcher       *kube.ServiceWatcher
	routeWatcher         *kube.RouteWatcher
	ingressWatcher       *kube.IngressWatcher
	httpProxyWatcher     *kube.DynamicWatcher
	tlsRouteWatcher      *kube.DynamicWatcher
	securedAccessWatcher *kube.SecuredAccessWatcher
}

func NewSecuredAccessResourceWatcher(accessMgr *SecuredAccessManager) *SecuredAccessResourceWatcher {
	return &SecuredAccessResourceWatcher{
		accessMgr: accessMgr,
	}
}

func (m *SecuredAccessResourceWatcher) WatchResources(controller *kube.Controller, namespace string) {
	m.serviceWatcher = controller.WatchServices(coreSecuredAccess(), namespace, m.accessMgr.CheckService)
	m.ingressWatcher = controller.WatchIngresses(coreSecuredAccess(), namespace, m.accessMgr.CheckIngress)
	m.routeWatcher = controller.WatchRoutes(routeSecuredAccess(), namespace, m.accessMgr.CheckRoute)
	m.httpProxyWatcher = controller.WatchContourHttpProxies(dynamicSecuredAccess(), namespace, m.accessMgr.CheckHttpProxy)
	m.tlsRouteWatcher = controller.WatchTlsRoutes(dynamicSecuredAccess(), namespace, m.accessMgr.CheckTlsRoute)
}

func (m *SecuredAccessResourceWatcher) WatchGateway(controller *kube.Controller, namespace string) {
	controller.WatchGateways(dynamicByName("skupper"), namespace, m.accessMgr.CheckGateway)
}

func (m *SecuredAccessResourceWatcher) WatchSecuredAccesses(controller *kube.Controller, namespace string, handler kube.SecuredAccessHandler) {
	f := func(key string, sa *skupperv2alpha1.SecuredAccess) error {
		if sa == nil {
			return m.accessMgr.SecuredAccessDeleted(key)
		}
		if handler != nil {
			handler(key, sa)
		}
		return m.accessMgr.SecuredAccessChanged(key, sa)
	}
	m.securedAccessWatcher = controller.WatchSecuredAccesses(namespace, f)
}

func (m *SecuredAccessResourceWatcher) Recover() {
	for _, service := range m.serviceWatcher.List() {
		m.accessMgr.RecoverService(service)
	}
	if m.routeWatcher != nil {
		for _, route := range m.routeWatcher.List() {
			m.accessMgr.RecoverRoute(route)
		}
	}
	if m.ingressWatcher != nil {
		for _, ingress := range m.ingressWatcher.List() {
			m.accessMgr.RecoverIngress(ingress)
		}
	}
	if m.httpProxyWatcher != nil {
		for _, httpProxy := range m.httpProxyWatcher.List() {
			m.accessMgr.RecoverHttpProxy(httpProxy)
		}
	}
	if m.tlsRouteWatcher != nil {
		for _, route := range m.tlsRouteWatcher.List() {
			m.accessMgr.RecoverTlsRoute(route)
		}
	}
	//once all resources are recovered, can process definitions
	for _, sa := range m.securedAccessWatcher.List() {
		m.accessMgr.SecuredAccessChanged(sa.Namespace+"/"+sa.Name, sa)
	}
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

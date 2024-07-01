package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"strings"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/informers/internalinterfaces"
	"k8s.io/client-go/tools/cache"

	routev1 "github.com/openshift/api/route/v1"
	routev1interfaces "github.com/openshift/client-go/route/informers/externalversions/internalinterfaces"

	skupperv1alpha1 "github.com/skupperproject/skupper/pkg/apis/skupper/v1alpha1"
	"github.com/skupperproject/skupper/pkg/event"
	"github.com/skupperproject/skupper/pkg/flow"
	"github.com/skupperproject/skupper/pkg/kube"
	"github.com/skupperproject/skupper/pkg/kube/certificates"
	"github.com/skupperproject/skupper/pkg/kube/claims"
	"github.com/skupperproject/skupper/pkg/kube/securedaccess"
	"github.com/skupperproject/skupper/pkg/kube/site"
	"github.com/skupperproject/skupper/pkg/kube/tokens"
)

type Controller struct {
	controller           *kube.Controller
	stopCh               <-chan struct{}
	siteWatcher          *kube.SiteWatcher
	listenerWatcher      *kube.ListenerWatcher
	connectorWatcher     *kube.ConnectorWatcher
	linkAccessWatcher    *kube.RouterAccessWatcher
	grantWatcher         *kube.AccessGrantWatcher
	sites                map[string]*site.Site
	grants               claims.GrantManager
	accessMgr            *securedaccess.SecuredAccessManager
	accessRecovery       AccessRecovery
	certMgr              *certificates.CertificateManagerImpl
	attachableConnectors map[string]*skupperv1alpha1.AttachedConnector
}

type AccessRecovery struct {
	serviceWatcher   *kube.ServiceWatcher
	routeWatcher     *kube.RouteWatcher
	ingressWatcher   *kube.IngressWatcher
	httpProxyWatcher *kube.DynamicWatcher
}

func (m *AccessRecovery) recoverAll(accessMgr *securedaccess.SecuredAccessManager) {
	for _, service := range m.serviceWatcher.List() {
		accessMgr.RecoverService(service)
	}
	if m.routeWatcher != nil {
		for _, route := range m.routeWatcher.List() {
			accessMgr.RecoverRoute(route)
		}
	}
	if m.ingressWatcher != nil {
		for _, ingress := range m.ingressWatcher.List() {
			accessMgr.RecoverIngress(ingress)
		}
	}
	if m.httpProxyWatcher != nil {
		for _, httpProxy := range m.httpProxyWatcher.List() {
			accessMgr.RecoverHttpProxy(httpProxy)
		}
	}
}

func skupperRouterService() internalinterfaces.TweakListOptionsFunc {
	return func(options *metav1.ListOptions) {
		options.FieldSelector = "metadata.name=skupper-router"
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

func skupperNetworkStatus() internalinterfaces.TweakListOptionsFunc {
	return func(options *metav1.ListOptions) {
		options.FieldSelector = "metadata.name=skupper-network-status"
	}
}

func dynamicWatcherOptions(selector string) dynamicinformer.TweakListOptionsFunc {
	return func(options *metav1.ListOptions) {
		options.LabelSelector = selector
	}
}

func dynamicSecuredAccess() dynamicinformer.TweakListOptionsFunc {
	return dynamicWatcherOptions("internal.skupper.io/secured-access")
}

func NewController(cli kube.Clients, watchNamespace string, currentNamespace string) (*Controller, error) {
	controller := &Controller{
		controller:           kube.NewController("Controller", cli),
		sites:                map[string]*site.Site{},
		attachableConnectors: map[string]*skupperv1alpha1.AttachedConnector{},
	}

	controller.siteWatcher = controller.controller.WatchSites(watchNamespace, controller.checkSite)
	controller.listenerWatcher = controller.controller.WatchListeners(watchNamespace, controller.checkListener)
	controller.connectorWatcher = controller.controller.WatchConnectors(watchNamespace, controller.checkConnector)
	controller.linkAccessWatcher = controller.controller.WatchRouterAccesses(watchNamespace, controller.checkRouterAccess)
	controller.controller.WatchAttachedConnectors(watchNamespace, controller.checkAttachedConnector)
	controller.controller.WatchAttachedConnectorAnchors(watchNamespace, controller.checkAttachedConnectorAnchor)
	controller.controller.WatchLinks(watchNamespace, controller.checkLink)
	controller.controller.WatchConfigMaps(skupperNetworkStatus(), watchNamespace, controller.networkStatusUpdate)
	controller.controller.WatchAccessTokens(watchNamespace, controller.checkAccessToken)
	controller.controller.WatchSecuredAccesses(watchNamespace, controller.checkSecuredAccess)

	controller.certMgr = certificates.NewCertificateManager(controller.controller)
	controller.certMgr.Watch(watchNamespace)
	controller.accessMgr = securedaccess.NewSecuredAccessManager(controller.controller, controller.certMgr, securedaccess.GetAccessTypeFromEnv())

	controller.accessRecovery.serviceWatcher = controller.controller.WatchServices(coreSecuredAccess(), watchNamespace, controller.checkSecuredAccessService)
	controller.accessRecovery.ingressWatcher = controller.controller.WatchIngresses(coreSecuredAccess(), watchNamespace, controller.checkSecuredAccessIngress)
	controller.accessRecovery.routeWatcher = controller.controller.WatchRoutes(routeSecuredAccess(), watchNamespace, controller.checkSecuredAccessRoute)
	controller.accessRecovery.httpProxyWatcher = controller.controller.WatchContourHttpProxies(dynamicSecuredAccess(), watchNamespace, controller.checkSecuredAccessHttpProxy)

	controller.grants = claims.NewGrantManager(controller.controller, claims.GrantConfigFromEnv(), controller.generateLinkConfig)
	controller.grantWatcher = controller.controller.WatchAccessGrants(watchNamespace, controller.grants.GrantChanged)
	controller.grants.Watch(controller.controller, currentNamespace)

	return controller, nil
}

func (c *Controller) Run(stopCh <-chan struct{}) error {
	log.Println("Starting informers")
	event.StartDefaultEventStore(stopCh)
	c.controller.StartWatchers(stopCh)
	c.stopCh = stopCh

	log.Println("Waiting for informer caches to sync")
	if ok := c.controller.WaitForCacheSync(stopCh); !ok {
		return fmt.Errorf("Failed to wait for caches to sync")
	}
	//TODO: need to recover active sites first
	//recover existing sites & bindings
	for _, site := range c.siteWatcher.List() {
		log.Printf("Recovering site %s/%s", site.ObjectMeta.Namespace, site.ObjectMeta.Name)
		err := c.getSite(site.ObjectMeta.Namespace).Recover(site)
		if err != nil {
			log.Printf("Error recovering site for %s/%s: %s", site.ObjectMeta.Namespace, site.ObjectMeta.Name, err)
		}
	}
	for _, connector := range c.connectorWatcher.List() {
		site := c.getSite(connector.ObjectMeta.Namespace)
		log.Printf("checking connector %s in %s", connector.ObjectMeta.Name, connector.ObjectMeta.Namespace)
		site.CheckConnector(connector.ObjectMeta.Name, connector)
	}
	for _, listener := range c.listenerWatcher.List() {
		site := c.getSite(listener.ObjectMeta.Namespace)
		log.Printf("checking listener %s in %s", listener.ObjectMeta.Name, listener.ObjectMeta.Namespace)
		site.CheckListener(listener.ObjectMeta.Name, listener)
	}
	for _, la := range c.linkAccessWatcher.List() {
		site := c.getSite(la.ObjectMeta.Namespace)
		site.CheckRouterAccess(la.ObjectMeta.Name, la)
	}
	for _, grant := range c.grantWatcher.List() {
		c.grants.GrantChanged(fmt.Sprintf("%s/%s", grant.Namespace, grant.Name), grant)
	}
	c.certMgr.Recover()
	c.accessRecovery.recoverAll(c.accessMgr)
	c.grants.Start()

	log.Println("Starting event loop")
	c.controller.Start(stopCh)
	<-stopCh
	log.Println("Shutting down")
	return nil
}

func (c *Controller) getSite(namespace string) *site.Site {
	if existing, ok := c.sites[namespace]; ok {
		return existing
	}
	site := site.NewSite(namespace, c.controller, c.certMgr, c.accessMgr)
	c.sites[namespace] = site
	return site
}

func (c *Controller) checkSite(key string, site *skupperv1alpha1.Site) error {
	log.Printf("Checking site %s", key)
	if site != nil {
		err := c.getSite(site.ObjectMeta.Namespace).Reconcile(site)
		if err != nil {
			log.Printf("Error initialising site for %s: %s", key, err)
		}
	} else {
		namespace, _, err := cache.SplitMetaNamespaceKey(key)
		if err != nil {
			return err
		}
		c.getSite(namespace).Deleted()
		delete(c.sites, namespace)
	}
	return nil
}

func (c *Controller) checkConnector(key string, connector *skupperv1alpha1.Connector) error {
	log.Printf("checkConnector(%s)", key)
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return err
	}
	return c.getSite(namespace).CheckConnector(name, connector)
}

func (c *Controller) checkListener(key string, listener *skupperv1alpha1.Listener) error {
	log.Printf("checkListener(%s)", key)
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return err
	}
	return c.getSite(namespace).CheckListener(name, listener)
}

func (c *Controller) checkLink(key string, linkconfig *skupperv1alpha1.Link) error {
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return err
	}
	return c.getSite(namespace).CheckLink(name, linkconfig)
}

func (c *Controller) checkSecuredAccessService(key string, svc *corev1.Service) error {
	return c.accessMgr.CheckService(key, svc)
}

func (c *Controller) checkSecuredAccessIngress(key string, ingress *networkingv1.Ingress) error {
	return c.accessMgr.CheckIngress(key, ingress)
}

func (c *Controller) checkSecuredAccessRoute(key string, ingress *routev1.Route) error {
	return c.accessMgr.CheckRoute(key, ingress)
}

func (c *Controller) checkSecuredAccessHttpProxy(key string, o *unstructured.Unstructured) error {
	return c.accessMgr.CheckHttpProxy(key, o)
}

func (c *Controller) checkAccessToken(key string, token *skupperv1alpha1.AccessToken) error {
	if token == nil || token.Status.Redeemed {
		return nil
	}
	site := c.getSite(token.Namespace).GetSite()
	if site == nil {
		return nil
	}
	return claims.RedeemAccessToken(token, site, c.controller)
}

func (c *Controller) generateLinkConfig(namespace string, name string, subject string, writer io.Writer) error {
	site := c.getSite(namespace).GetSite()
	if site == nil {
		return fmt.Errorf("Site not yet defined for %s", namespace)
	}
	generator, err := tokens.NewTokenGeneratorForSite(site, c.controller)
	if err != nil {
		return err
	}
	token := generator.NewCertToken(name, subject)
	return token.Write(writer)
}

func (c *Controller) checkSecuredAccess(key string, se *skupperv1alpha1.SecuredAccess) error {
	if se == nil {
		return c.accessMgr.SecuredAccessDeleted(key)
	}
	c.getSite(se.ObjectMeta.Namespace).CheckSecuredAccess(se)
	c.grants.SecuredAccessChanged(key, se)
	return c.accessMgr.SecuredAccessChanged(key, se)
}

func (c *Controller) checkRouterAccess(key string, ra *skupperv1alpha1.RouterAccess) error {
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return err
	}
	return c.getSite(namespace).CheckRouterAccess(name, ra)
}

func (c *Controller) checkAttachedConnectorAnchor(key string, anchor *skupperv1alpha1.AttachedConnectorAnchor) error {
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return err
	}
	return c.getSite(namespace).CheckAttachedConnectorAnchor(namespace, name, anchor)
}

func (c *Controller) checkAttachedConnector(key string, connector *skupperv1alpha1.AttachedConnector) error {
	if connector == nil {
		if previous, ok := c.attachableConnectors[key]; ok {
			delete(c.attachableConnectors, key)
			return c.getSite(previous.Spec.SiteNamespace).AttachedConnectorDeleted(previous.Namespace, previous.Name)
		} else {
			return nil
		}
	} else {
		return c.getSite(connector.Spec.SiteNamespace).AttachedConnectorUpdated(connector)
	}
}

func (c *Controller) networkStatusUpdate(key string, cm *corev1.ConfigMap) error {
	if cm == nil {
		return nil
	}
	encoded := cm.Data["NetworkStatus"]
	if encoded == "" {
		log.Printf("No network status found in %s", key)
		return nil
	}
	status := &flow.NetworkStatus{}
	err := json.Unmarshal([]byte(encoded), status)
	if err != nil {
		log.Printf("Error unmarshalling network status from %s: %s", key, err)
		return nil
	}
	log.Printf("Updating network status for %s", cm.ObjectMeta.Namespace)
	return c.getSite(cm.ObjectMeta.Namespace).NetworkStatusUpdated(extractSiteRecords(status))
}

func extractSiteRecords(status *flow.NetworkStatus) []skupperv1alpha1.SiteRecord {
	if status == nil {
		return nil
	}
	var records []skupperv1alpha1.SiteRecord
	routers := map[string]string{} // router name -> site name
	for _, site := range status.Sites {
		for _, router := range site.RouterStatus {
			if router.Router.Name != nil && site.Site.Name != nil {
				name := *router.Router.Name
				parts := strings.Split(name, "/")
				if len(parts) == 2 {
					name = parts[1]
				}
				routers[name] = *site.Site.Name
			}
		}
	}
	for _, site := range status.Sites {
		record := skupperv1alpha1.SiteRecord{
			Id: site.Site.Identity,
		}
		if site.Site.Name != nil {
			record.Name = *site.Site.Name
		}
		if site.Site.Platform != nil {
			record.Platform = *site.Site.Platform
		}
		if site.Site.NameSpace != nil {
			record.Platform = *site.Site.NameSpace
		}
		if site.Site.Version != nil {
			record.Version = *site.Site.Version
		}
		services := map[string]*skupperv1alpha1.ServiceRecord{}
		for _, router := range site.RouterStatus {
			for _, link := range router.Links {
				if link.Name != nil {
					if siteName, ok := routers[*link.Name]; ok {
						record.Links = append(record.Links, siteName)
					}
				}
			}
			for _, connector := range router.Connectors {
				if connector.Address != nil && connector.DestHost != nil {
					address := *connector.Address
					service, ok := services[address]
					if !ok {
						service = &skupperv1alpha1.ServiceRecord{
							RoutingKey: address,
						}
						services[address] = service
					}
					service.Connectors = append(service.Connectors, *connector.DestHost)
				}
			}
			for _, listener := range router.Listeners {
				if listener.Address != nil && listener.Name != nil {
					address := *listener.Address
					service, ok := services[address]
					if !ok {
						service = &skupperv1alpha1.ServiceRecord{
							RoutingKey: address,
						}
						services[address] = service
					}
					service.Listeners = append(service.Listeners, *listener.Name)
				}
			}
		}
		for _, service := range services {
			record.Services = append(record.Services, *service)
		}
		records = append(records, record)
	}
	return records
}

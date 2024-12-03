package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"log/slog"
	"os"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers/internalinterfaces"
	"k8s.io/client-go/tools/cache"

	internalclient "github.com/skupperproject/skupper/internal/kube/client"
	skupperv2alpha1 "github.com/skupperproject/skupper/pkg/apis/skupper/v2alpha1"
	"github.com/skupperproject/skupper/pkg/kube"
	"github.com/skupperproject/skupper/pkg/kube/certificates"
	"github.com/skupperproject/skupper/pkg/kube/grants"
	"github.com/skupperproject/skupper/pkg/kube/securedaccess"
	"github.com/skupperproject/skupper/pkg/kube/site"
	"github.com/skupperproject/skupper/pkg/network"
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
	startGrantServer     func()
	accessMgr            *securedaccess.SecuredAccessManager
	accessRecovery       *securedaccess.SecuredAccessResourceWatcher
	certMgr              *certificates.CertificateManagerImpl
	attachableConnectors map[string]*skupperv2alpha1.AttachedConnector
}

func skupperRouterService() internalinterfaces.TweakListOptionsFunc {
	return func(options *metav1.ListOptions) {
		options.FieldSelector = "metadata.name=skupper-router"
	}
}

func skupperNetworkStatus() internalinterfaces.TweakListOptionsFunc {
	return func(options *metav1.ListOptions) {
		options.FieldSelector = "metadata.name=skupper-network-status"
	}
}

func NewController(cli internalclient.Clients, grantConfig *grants.GrantConfig, securedAccessConfig *securedaccess.Config, watchNamespace string, currentNamespace string) (*Controller, error) {
	controller := &Controller{
		controller:           kube.NewController("Controller", cli),
		sites:                map[string]*site.Site{},
		attachableConnectors: map[string]*skupperv2alpha1.AttachedConnector{},
	}

	podname := os.Getenv("HOSTNAME")
	owner, err := controller.controller.GetDeploymentForPod(podname, currentNamespace)
	controllerContext := securedaccess.ControllerContext{
		Namespace: currentNamespace,
	}
	if err != nil {
		log.Printf("Could not deduce owning deployment, some resources may need to be manually deleted when controller is deleted: %s", err)
	} else {
		controllerContext.Name = owner.Name
		controllerContext.UID = string(owner.UID)
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
	controller.controller.WatchPods("skupper.io/component=router,skupper.io/type=site", watchNamespace, controller.routerPodEvent)

	controller.certMgr = certificates.NewCertificateManager(controller.controller)
	controller.certMgr.Watch(watchNamespace)

	controller.accessMgr = securedaccess.NewSecuredAccessManager(controller.controller, controller.certMgr, securedAccessConfig, controllerContext)
	controller.accessRecovery = securedaccess.NewSecuredAccessResourceWatcher(controller.accessMgr)
	controller.accessRecovery.WatchResources(controller.controller, watchNamespace)
	controller.accessRecovery.WatchSecuredAccesses(controller.controller, watchNamespace, controller.checkSecuredAccess)
	controller.accessRecovery.WatchGateway(controller.controller, currentNamespace)

	controller.startGrantServer = grants.Initialise(controller.controller, currentNamespace, watchNamespace, grantConfig, controller.generateLinkConfig)

	controller.controller.WatchConfigMaps(skupperLogConfig(), currentNamespace, controller.logConfigUpdate)

	return controller, nil
}

func (c *Controller) Run(stopCh <-chan struct{}) error {
	log.Println("Starting informers")
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
		err := c.getSite(site.ObjectMeta.Namespace).StartRecovery(site)
		if err != nil {
			log.Printf("Error recovering site for %s/%s: %s", site.ObjectMeta.Namespace, site.ObjectMeta.Name, err)
		}
	}
	for _, connector := range c.connectorWatcher.List() {
		site := c.getSite(connector.ObjectMeta.Namespace)
		log.Printf("Recovering connector %s in %s", connector.ObjectMeta.Name, connector.ObjectMeta.Namespace)
		site.CheckConnector(connector.ObjectMeta.Name, connector)
	}
	for _, listener := range c.listenerWatcher.List() {
		site := c.getSite(listener.ObjectMeta.Namespace)
		log.Printf("Recovering listener %s in %s", listener.ObjectMeta.Name, listener.ObjectMeta.Namespace)
		site.CheckListener(listener.ObjectMeta.Name, listener)
	}
	for _, la := range c.linkAccessWatcher.List() {
		site := c.getSite(la.ObjectMeta.Namespace)
		site.CheckRouterAccess(la.ObjectMeta.Name, la)
	}
	for _, site := range c.siteWatcher.List() {
		err := c.getSite(site.ObjectMeta.Namespace).Reconcile(site)
		if err != nil {
			log.Printf("Error recovering site for %s/%s: %s", site.ObjectMeta.Namespace, site.ObjectMeta.Name, err)
		}
		log.Printf("Recovered site %s/%s", site.ObjectMeta.Namespace, site.ObjectMeta.Name)
	}
	c.certMgr.Recover()
	c.accessRecovery.Recover()
	if c.startGrantServer != nil {
		c.startGrantServer()
	}

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

func (c *Controller) checkSite(key string, site *skupperv2alpha1.Site) error {
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

func (c *Controller) checkConnector(key string, connector *skupperv2alpha1.Connector) error {
	log.Printf("checkConnector(%s)", key)
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return err
	}
	return c.getSite(namespace).CheckConnector(name, connector)
}

func (c *Controller) checkListener(key string, listener *skupperv2alpha1.Listener) error {
	log.Printf("checkListener(%s)", key)
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return err
	}
	return c.getSite(namespace).CheckListener(name, listener)
}

func (c *Controller) checkLink(key string, linkconfig *skupperv2alpha1.Link) error {
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return err
	}
	return c.getSite(namespace).CheckLink(name, linkconfig)
}

func (c *Controller) checkAccessToken(key string, token *skupperv2alpha1.AccessToken) error {
	if token == nil || token.IsRedeemed() {
		return nil
	}
	site := c.getSite(token.Namespace).GetSite()
	if site == nil {
		return nil
	}
	return grants.RedeemAccessToken(token, site, c.controller)
}

func (c *Controller) routerPodEvent(key string, pod *corev1.Pod) error {
	namespace, _, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return err
	}
	return c.getSite(namespace).RouterPodEvent(key, pod)
}

func (c *Controller) generateLinkConfig(namespace string, name string, subject string, writer io.Writer) error {
	site := c.getSite(namespace).GetSite()
	if site == nil {
		return fmt.Errorf("Site not yet defined for %s", namespace)
	}
	generator, err := grants.NewTokenGenerator(site, c.controller)
	if err != nil {
		return err
	}
	token := generator.NewCertToken(name, subject)
	return token.Write(writer)
}

func (c *Controller) checkSecuredAccess(key string, se *skupperv2alpha1.SecuredAccess) error {
	c.getSite(se.ObjectMeta.Namespace).CheckSecuredAccess(se)
	return nil
}

func (c *Controller) checkRouterAccess(key string, ra *skupperv2alpha1.RouterAccess) error {
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return err
	}
	return c.getSite(namespace).CheckRouterAccess(name, ra)
}

func (c *Controller) checkAttachedConnectorAnchor(key string, anchor *skupperv2alpha1.AttachedConnectorAnchor) error {
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return err
	}
	return c.getSite(namespace).CheckAttachedConnectorAnchor(namespace, name, anchor)
}

func (c *Controller) checkAttachedConnector(key string, connector *skupperv2alpha1.AttachedConnector) error {
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
	var status network.NetworkStatusInfo
	err := json.Unmarshal([]byte(encoded), &status)
	if err != nil {
		log.Printf("Error unmarshalling network status from %s: %s", key, err)
		return nil
	}
	log.Printf("Updating network status for %s", cm.ObjectMeta.Namespace)
	return c.getSite(cm.ObjectMeta.Namespace).NetworkStatusUpdated(extractSiteRecords(status))
}

func extractSiteRecords(status network.NetworkStatusInfo) []skupperv2alpha1.SiteRecord {
	var records []skupperv2alpha1.SiteRecord
	routerAPs := map[string]string{} // router access point ID -> site ID
	siteNames := map[string]string{} // site ID -> site name
	for _, site := range status.SiteStatus {
		siteNames[site.Site.Identity] = site.Site.Name
		for _, router := range site.RouterStatus {
			for _, ap := range router.AccessPoints {
				routerAPs[ap.Identity] = site.Site.Identity
			}
		}
	}
	for _, site := range status.SiteStatus {
		record := skupperv2alpha1.SiteRecord{
			Id:        site.Site.Identity,
			Name:      site.Site.Name,
			Platform:  site.Site.Platform,
			Namespace: site.Site.Namespace,
			Version:   site.Site.Version,
		}
		services := map[string]*skupperv2alpha1.ServiceRecord{}
		for _, router := range site.RouterStatus {
			for _, link := range router.Links {
				if link.Name == "" || link.Peer == "" {
					continue
				}

				if site, ok := routerAPs[link.Peer]; ok {
					record.Links = append(record.Links, skupperv2alpha1.LinkRecord{
						Name:           link.Name,
						RemoteSiteId:   site,
						RemoteSiteName: siteNames[site],
						Operational:    strings.EqualFold(link.Status, "up"),
					})
				}
			}
			for _, connector := range router.Connectors {
				if connector.Address != "" && connector.DestHost != "" {
					address := connector.Address
					service, ok := services[address]
					if !ok {
						service = &skupperv2alpha1.ServiceRecord{
							RoutingKey: address,
						}
						services[address] = service
					}
					service.Connectors = append(service.Connectors, connector.DestHost)
				}
			}
			for _, listener := range router.Listeners {
				if listener.Address != "" && listener.Name != "" {
					address := listener.Address
					service, ok := services[address]
					if !ok {
						service = &skupperv2alpha1.ServiceRecord{
							RoutingKey: address,
						}
						services[address] = service
					}
					service.Listeners = append(service.Listeners, listener.Name)
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

func skupperLogConfig() internalinterfaces.TweakListOptionsFunc {
	return func(options *metav1.ListOptions) {
		options.FieldSelector = "metadata.name=skupper-log-config"
	}
}

func convertLogLevel(logLevel string) slog.Level {
	switch strings.ToLower(logLevel) {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	}
	return slog.LevelInfo
}

func (c *Controller) logConfigUpdate(key string, cm *corev1.ConfigMap) error {
	const controllerLogLevelKey = "CONTROLLER_LOG_LEVEL"
	var slogLevel slog.Level
	if cm == nil {
		// if configmap is deleted, then set log level to info
		slogLevel = slog.LevelInfo
	} else {
		logLevel := cm.Data[controllerLogLevelKey]
		slogLevel = convertLogLevel(logLevel)
	}

	if slogLevel != controllerLogLevel.Level() {
		controllerLogLevel.Set(slogLevel)
	}
	slog.Info("Updating log level", slog.String("logLevel", slogLevel.String()))
	return nil
}

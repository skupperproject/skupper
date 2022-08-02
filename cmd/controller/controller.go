package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/informers/internalinterfaces"
	"k8s.io/client-go/tools/cache"

	"github.com/skupperproject/skupper/client"
	skupperv1alpha1 "github.com/skupperproject/skupper/pkg/apis/skupper/v1alpha1"
	"github.com/skupperproject/skupper/pkg/event"
	"github.com/skupperproject/skupper/pkg/flow"
	"github.com/skupperproject/skupper/pkg/kube"
	"github.com/skupperproject/skupper/pkg/kube/site"
	"github.com/skupperproject/skupper/pkg/version"
)

type Controller struct {
	vanClient            *client.VanClient
	controller           *kube.Controller
	stopCh               <-chan struct{}
	siteWatcher          *kube.SiteWatcher
	listenerWatcher      *kube.ListenerWatcher
	connectorWatcher     *kube.ConnectorWatcher
	linkConfigWatcher    *kube.LinkConfigWatcher
	serviceWatcher       *kube.ServiceWatcher
	networkStatusWatcher *kube.ConfigMapWatcher
	dynamicWatchers      map[string]*kube.DynamicWatcher
	sites                map[string]*site.Site
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

func dynamicWatcherOptions(selector string) dynamicinformer.TweakListOptionsFunc {
	return func(options *metav1.ListOptions) {
		options.LabelSelector = selector
	}
}

func NewController(cli *client.VanClient) (*Controller, error) {
	var watchNamespace string

	// Startup message
	if os.Getenv("WATCH_NAMESPACE") != "" {
		watchNamespace = os.Getenv("WATCH_NAMESPACE")
		log.Println("Skupper controller watching current namespace ", watchNamespace)
	} else {
		watchNamespace = metav1.NamespaceAll
		log.Println("Skupper controller watching all namespaces")
	}
	log.Printf("Version: %s", version.Version)

	controller := &Controller{
		vanClient:       cli,
		controller:      kube.NewController("Controller", cli),
		sites:           map[string]*site.Site{},
		dynamicWatchers: map[string]*kube.DynamicWatcher{},
	}
	controller.siteWatcher = controller.controller.WatchSites(watchNamespace, controller.checkSite)
	controller.listenerWatcher = controller.controller.WatchListeners(watchNamespace, controller.checkListener)
	controller.connectorWatcher = controller.controller.WatchConnectors(watchNamespace, controller.checkConnector)
	controller.linkConfigWatcher = controller.controller.WatchLinkConfigs(watchNamespace, controller.checkLinkConfig)
	controller.serviceWatcher = controller.controller.WatchServices(skupperRouterService(), watchNamespace, controller.checkRouterService)
	controller.networkStatusWatcher = controller.controller.WatchConfigMaps(skupperNetworkStatus(), watchNamespace, controller.networkStatusUpdate)
	for _, resource := range kube.GetSupportedIngressResources(controller.controller.GetDiscoveryClient()) {
		watcher := controller.controller.WatchDynamic(resource, dynamicWatcherOptions("internal.skupper.io/controlled"), watchNamespace, controller.checkIngressResource)
		controller.dynamicWatchers[resource.Resource] = watcher
	}

	return controller, nil
}

func (c *Controller) Run(stopCh <-chan struct{}) error {
	log.Println("Starting informers")
	event.StartDefaultEventStore(stopCh)
	c.siteWatcher.Start(stopCh)
	c.listenerWatcher.Start(stopCh)
	c.connectorWatcher.Start(stopCh)
	c.linkConfigWatcher.Start(stopCh)
	c.serviceWatcher.Start(stopCh)
	c.networkStatusWatcher.Start(stopCh)
	c.stopCh = stopCh

	log.Println("Waiting for informer caches to sync")
	if ok := cache.WaitForCacheSync(stopCh, c.siteWatcher.HasSynced(), c.listenerWatcher.HasSynced(), c.connectorWatcher.HasSynced(), c.linkConfigWatcher.HasSynced(), c.serviceWatcher.HasSynced(), c.networkStatusWatcher.HasSynced()); !ok {
		return fmt.Errorf("Failed to wait for caches to sync")
	}
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
	site := site.NewSite(namespace, c.controller)
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

func (c *Controller) checkLinkConfig(key string, linkconfig *skupperv1alpha1.LinkConfig) error {
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return err
	}
	return c.getSite(namespace).CheckLinkConfig(name, linkconfig)
}

func (c *Controller) checkRouterService(key string, svc *corev1.Service) error {
	if svc == nil || svc.Spec.Type != corev1.ServiceTypeLoadBalancer {
		return nil
	}
	return c.getSite(svc.ObjectMeta.Namespace).CheckLoadBalancer(svc)
}

func (c *Controller) checkIngressResource(key string, o *unstructured.Unstructured) error {
	if o == nil {
		return nil
	}
	return c.getSite(o.GetNamespace()).ResolveHosts(o)
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
		record := skupperv1alpha1.SiteRecord {
			Id:   site.Site.Identity,
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

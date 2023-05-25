package main

import (
	"fmt"
	"log"
	"os"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/informers/internalinterfaces"
	"k8s.io/client-go/tools/cache"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/client"
	"github.com/skupperproject/skupper/pkg/event"
	"github.com/skupperproject/skupper/pkg/kube"
	"github.com/skupperproject/skupper/pkg/kube/site"
	"github.com/skupperproject/skupper/pkg/version"
)

type Controller struct {
	vanClient         *client.VanClient
	controller        *kube.Controller
	stopCh            <-chan struct{}
	siteWatcher       *kube.ConfigMapWatcher
	listenerWatcher   *kube.ConfigMapWatcher
	connectorWatcher  *kube.ConfigMapWatcher
	linkConfigWatcher *kube.SecretWatcher
	serviceWatcher    *kube.ServiceWatcher
	dynamicWatchers   map[string]*kube.DynamicWatcher
	sites             map[string]*site.Site
}

func siteWatcherOptions() internalinterfaces.TweakListOptionsFunc {
	return func(options *metav1.ListOptions) {
		options.FieldSelector = "metadata.name=skupper-site"
		options.LabelSelector = "!" + types.SiteControllerIgnore
	}
}

func skupperRouterService() internalinterfaces.TweakListOptionsFunc {
	return func(options *metav1.ListOptions) {
		options.FieldSelector = "metadata.name=skupper-router"
	}
}

func skupperTypeWatcherOptions(skupperType string) internalinterfaces.TweakListOptionsFunc {
	return func(options *metav1.ListOptions) {
		options.LabelSelector = "skupper.io/type=" + skupperType
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
	controller.siteWatcher = controller.controller.WatchConfigMaps(siteWatcherOptions(), watchNamespace, controller.checkSite)
	controller.listenerWatcher = controller.controller.WatchConfigMaps(skupperTypeWatcherOptions("listener"), watchNamespace, controller.checkListener)
	controller.connectorWatcher = controller.controller.WatchConfigMaps(skupperTypeWatcherOptions("connector"), watchNamespace, controller.checkConnector)
	controller.linkConfigWatcher = controller.controller.WatchSecrets(skupperTypeWatcherOptions("connection-token"), watchNamespace, controller.checkLinkConfig)
	controller.serviceWatcher = controller.controller.WatchServices(skupperRouterService(), watchNamespace, controller.checkRouterService)
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
	c.stopCh = stopCh

	log.Println("Waiting for informer caches to sync")
	if ok := cache.WaitForCacheSync(stopCh, c.siteWatcher.HasSynced(), c.listenerWatcher.HasSynced(), c.connectorWatcher.HasSynced(), c.linkConfigWatcher.HasSynced(), c.serviceWatcher.HasSynced()); !ok {
		return fmt.Errorf("Failed to wait for caches to sync")
	}
	//recover existing sites & bindings
	for _, cm := range c.siteWatcher.List() {
		err := c.getSite(cm.ObjectMeta.Namespace).Recover(cm)
		if err != nil {
			log.Printf("Error initialising site for %s: %s", cm.ObjectMeta.Namespace, err)
		}
	}
	for _, cm := range c.connectorWatcher.List() {
		site := c.getSite(cm.ObjectMeta.Namespace)
		log.Printf("checking connector %s in %s", cm.ObjectMeta.Name, cm.ObjectMeta.Namespace)
		site.UpdateConnector(cm.ObjectMeta.Name, cm)
	}
	for _, cm := range c.listenerWatcher.List() {
		site := c.getSite(cm.ObjectMeta.Namespace)
		log.Printf("checking listener %s in %s", cm.ObjectMeta.Name, cm.ObjectMeta.Namespace)
		site.UpdateListener(cm.ObjectMeta.Name, cm)
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

func (c *Controller) checkSite(key string, configmap *corev1.ConfigMap) error {
	if configmap != nil {
		err := c.getSite(configmap.ObjectMeta.Namespace).Reconcile(configmap)
		if err != nil {
			log.Printf("Error initialising site for %s: %s", configmap.ObjectMeta.Namespace, err)
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

func (c *Controller) checkConnector(key string, configmap *corev1.ConfigMap) error {
	log.Printf("checkConnector(%s)", key)
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return err
	}
	return c.getSite(namespace).CheckConnector(name, configmap)
}

func (c *Controller) checkListener(key string, configmap *corev1.ConfigMap) error {
	log.Printf("checkListener(%s)", key)
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return err
	}
	return c.getSite(namespace).CheckListener(name, configmap)
}

func (c *Controller) checkLinkConfig(key string, secret *corev1.Secret) error {
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return err
	}
	return c.getSite(namespace).CheckLinkConfig(name, secret)
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

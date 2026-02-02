// Package watchers provides a means of watching changes in different
// Kubernetes resources.
package watchers

import (
	"log/slog"
	"time"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	corev1informer "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/informers/internalinterfaces"
	networkingv1informer "k8s.io/client-go/informers/networking/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"

	routev1 "github.com/openshift/api/route/v1"
	openshiftroute "github.com/openshift/client-go/route/clientset/versioned"
	routev1client "github.com/openshift/client-go/route/clientset/versioned/typed/route/v1"
	routev1interfaces "github.com/openshift/client-go/route/informers/externalversions/internalinterfaces"
	routev1informer "github.com/openshift/client-go/route/informers/externalversions/route/v1"

	internalclient "github.com/skupperproject/skupper/internal/kube/client"
	"github.com/skupperproject/skupper/internal/kube/resource"
	"github.com/skupperproject/skupper/pkg/apis/skupper/v2alpha1"
	skupperclient "github.com/skupperproject/skupper/pkg/generated/client/clientset/versioned"
	skupperv2alpha1interfaces "github.com/skupperproject/skupper/pkg/generated/client/informers/externalversions/internalinterfaces"
	skupperv2alpha1informer "github.com/skupperproject/skupper/pkg/generated/client/informers/externalversions/skupper/v2alpha1"
)

// ResourceChange is the form in which events are added to the
// EventProcessor's work queue. Each ResourceChange event has a key
// that identifies the resource along with an implementation of the
// ResourceChangeHandler interface that will be used when processing
// the event.
type ResourceChange struct {
	Handler ResourceChangeHandler
	Key     string
}

// The ResourceChangeHandler interface allows the event processing
// loop to handle events from different resource types in a general
// way.
type ResourceChangeHandler interface {
	// The Handle method is used to process the event.
	Handle(event ResourceChange) error
	// The Describe method is used to log information about the
	// event when an error is returned by the Handle method.
	Describe(event ResourceChange) string
	// Kind of Resource
	Kind() string
}

// The Watcher interface allows the EventProcessor to interact with
// different informers on startup.
type Watcher interface {
	HasSynced() func() bool
	Start(stopCh <-chan struct{})
}

// A EventProcessor provides a way to handle events from multiple
// different informers on the same go routine. It does this using a
// single work queue into which the events are added as instances of the
// ResourceChange struct.
type EventProcessor struct {
	errorKey        string
	client          kubernetes.Interface
	routeClient     openshiftroute.Interface
	dynamicClient   dynamic.Interface
	discoveryClient discovery.DiscoveryInterface
	skupperClient   skupperclient.Interface
	metrics         *metricsQueue
	queue           workqueue.RateLimitingInterface
	resync          time.Duration
	resyncShort     time.Duration
	watchers        []Watcher
	logger          *slog.Logger
}

type EventProcessorCustomizer func(e *EventProcessor)

// Creates a properly initialised EventProcessor instance.
func NewEventProcessor(name string, clients internalclient.Clients, options ...EventProcessorCustomizer) *EventProcessor {
	e := &EventProcessor{
		errorKey:        name + "Error",
		client:          clients.GetKubeClient(),
		routeClient:     clients.GetRouteInterface(),
		discoveryClient: clients.GetDiscoveryClient(),
		dynamicClient:   clients.GetDynamicClient(),
		skupperClient:   clients.GetSkupperClient(),
		queue:           workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), name),
		resync:          time.Minute * 5,
		resyncShort:     time.Second * 30,
		metrics: &metricsQueue{
			provider: noopMetricsProvider{},
			metrics:  make(map[string]metricsSet),
			pending:  make(map[ResourceChange]time.Time),
		},
		logger: slog.New(slog.Default().Handler()).With(slog.String("component", "kube.watchers.eventProcessor")),
	}
	for _, opt := range options {
		opt(e)
	}
	return e
}

func WithMetricsProvider(provider MetricsProvider) EventProcessorCustomizer {
	return func(e *EventProcessor) {
		if provider == nil {
			return
		}
		e.metrics.provider = provider
	}
}

func (c *EventProcessor) SetResync(resync time.Duration) {
	c.resync = resync
}

func (c *EventProcessor) SetResyncShort(resync time.Duration) {
	c.resyncShort = resync
}

func (c *EventProcessor) GetKubeClient() kubernetes.Interface {
	return c.client
}

func (c *EventProcessor) GetDynamicClient() dynamic.Interface {
	return c.dynamicClient
}

func (c *EventProcessor) GetDiscoveryClient() discovery.DiscoveryInterface {
	return c.discoveryClient
}

func (c *EventProcessor) HasContourHttpProxy() bool {
	return resource.IsResourceAvailable(c.discoveryClient, resource.ContourHttpProxyResource())
}

func (c *EventProcessor) HasGateway() bool {
	return resource.IsResourceAvailable(c.discoveryClient, resource.GatewayResource())
}

func (c *EventProcessor) HasTlsRoute() bool {
	return resource.IsResourceAvailable(c.discoveryClient, resource.TlsRouteResource())
}

func (c *EventProcessor) GetRouteInterface() openshiftroute.Interface {
	return c.routeClient
}

func (c *EventProcessor) GetRouteClient() routev1client.RouteV1Interface {
	if c.routeClient == nil {
		return nil
	}
	return c.routeClient.RouteV1()
}

func (c *EventProcessor) GetSkupperClient() skupperclient.Interface {
	return c.skupperClient
}

// Starts the event processing loop in a new go routine.
func (c *EventProcessor) Start(stopCh <-chan struct{}) {
	go wait.Until(c.run, time.Second, stopCh)
}

func (c *EventProcessor) GetWatchers() []Watcher {
	return c.watchers
}

func (c *EventProcessor) run() {
	for c.process() {
	}
}

// This is a convenience function for tests that use the EventProcessor,
// which may wish to process events more granularly.
func (c *EventProcessor) TestProcess() bool {
	return c.process()
}

// This is a convenience function for tests that use the EventProcessor.
func (c *EventProcessor) TestProcessAll() {
	for c.queue.Len() > 0 {
		c.process()
	}
}

// The process method is the heart of the event processing loop.
func (c *EventProcessor) process() bool {
	obj, shutdown := c.queue.Get()

	if shutdown {
		return false
	}

	var (
		hasError bool
		isRetry  bool
	)
	defer c.queue.Done(obj)
	if evt, ok := obj.(ResourceChange); ok {
		resolve := c.metrics.get(evt)
		defer func() {
			resolve(evt, isRetry)
		}()
		err := evt.Handler.Handle(evt)
		if err != nil {
			hasError = true
			c.logger.Error("Error while handling event",
				slog.String("errorKey", c.errorKey),
				slog.String("eventDescription", evt.Handler.Describe(evt)),
				slog.Any("error", err))
		}
	} else {
		c.logger.Error("Invalid object on event queue", slog.String("errorKey", c.errorKey), slog.Any("object", obj))
	}

	if hasError && c.queue.NumRequeues(obj) < 5 {
		isRetry = true
		c.queue.AddRateLimited(obj)
		return true
	}
	c.queue.Forget(obj)

	return true
}

// Stops event processing.
func (c *EventProcessor) Stop() {
	c.queue.ShutDown()
}

// Creates an event handler that will take handle events from an
// informer by constructing an appropriate ResourceChange instance and
// adding it to the EventProcessor's work queue.
func (c *EventProcessor) newEventHandler(handler ResourceChangeHandler) *cache.ResourceEventHandlerFuncs {
	evt := ResourceChange{
		Handler: handler,
	}
	return &cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			key, err := cache.MetaNamespaceKeyFunc(obj)
			if err != nil {
				utilruntime.HandleError(err)
			} else {
				evt.Key = key
				c.metrics.add(evt)
				c.queue.Add(evt)
			}
		},
		UpdateFunc: func(old, new interface{}) {
			key, err := cache.MetaNamespaceKeyFunc(new)
			if err != nil {
				utilruntime.HandleError(err)
			} else {
				evt.Key = key
				c.metrics.add(evt)
				c.queue.Add(evt)
			}
		},
		DeleteFunc: func(obj interface{}) {
			key, err := cache.MetaNamespaceKeyFunc(obj)
			if err != nil {
				utilruntime.HandleError(err)
			} else {
				evt.Key = key
				c.metrics.add(evt)
				c.queue.Add(evt)
			}
		},
	}
}

func (c *EventProcessor) addWatcher(watcher Watcher) {
	c.watchers = append(c.watchers, watcher)
}

// Starts all the configured informers.
func (c *EventProcessor) StartWatchers(stopCh <-chan struct{}) {
	for _, watcher := range c.watchers {
		watcher.Start(stopCh)
	}
}

// Wait for all the configured informers to sync.
func (c *EventProcessor) WaitForCacheSync(stopCh <-chan struct{}) bool {
	return cache.WaitForCacheSync(stopCh, c.haveWatchersSynced()...)
}

func (c *EventProcessor) haveWatchersSynced() []cache.InformerSynced {
	var combined []cache.InformerSynced
	for _, watcher := range c.watchers {
		combined = append(combined, watcher.HasSynced())
	}
	return combined
}

func addEventProcessorWatcher[T runtime.Object](c *EventProcessor, handler Handler[T], gv schema.GroupVersion, informer cache.SharedIndexInformer) *ResourceWatcher[T] {
	watcher := NewResourceWatcher(handler, gv, informer)
	informer.AddEventHandler(c.newEventHandler(watcher))
	c.addWatcher(watcher)
	return watcher
}

// Watches for ConfigMap related events matching options and invokes the handler function accordingly.
func (c *EventProcessor) WatchConfigMaps(options internalinterfaces.TweakListOptionsFunc, namespace string, handler ConfigMapHandler) *ConfigMapWatcher {
	informer := corev1informer.NewFilteredConfigMapInformer(
		c.client,
		namespace,
		c.resync,
		cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc},
		options)
	return addEventProcessorWatcher(c, handler, corev1.SchemeGroupVersion, informer)
}

func (c *EventProcessor) WatchSecrets(options internalinterfaces.TweakListOptionsFunc, namespace string, handler SecretHandler) *SecretWatcher {
	informer := corev1informer.NewFilteredSecretInformer(
		c.client,
		namespace,
		c.resync,
		cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc},
		options)
	return addEventProcessorWatcher(c, handler, corev1.SchemeGroupVersion, informer)
}

func (c *EventProcessor) WatchAllSecrets(namespace string, handler SecretHandler) *SecretWatcher {
	informer := corev1informer.NewSecretInformer(
		c.client,
		namespace,
		c.resync,
		cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc},
	)
	return addEventProcessorWatcher(c, handler, corev1.SchemeGroupVersion, informer)
}

func (c *EventProcessor) WatchServices(options internalinterfaces.TweakListOptionsFunc, namespace string, handler ServiceHandler) *ServiceWatcher {
	informer := corev1informer.NewFilteredServiceInformer(
		c.client,
		namespace,
		c.resync,
		cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc},
		options,
	)
	return addEventProcessorWatcher(c, handler, corev1.SchemeGroupVersion, informer)
}

func (c *EventProcessor) WatchPods(selector string, namespace string, handler PodHandler) *PodWatcher {
	options := func(options *metav1.ListOptions) {
		options.LabelSelector = selector
	}
	informer := corev1informer.NewFilteredPodInformer(
		c.client,
		namespace,
		c.resync,
		cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc},
		options,
	)
	return addEventProcessorWatcher(c, handler, corev1.SchemeGroupVersion, informer)
}

func (c *EventProcessor) WatchContourHttpProxies(options dynamicinformer.TweakListOptionsFunc, namespace string, handler DynamicHandler) *DynamicWatcher {
	if !c.HasContourHttpProxy() {
		c.logger.Error("Cannot watch HttpProxies; resource not installed")
		return nil
	}
	return c.WatchDynamic(resource.ContourHttpProxyResource(), options, namespace, handler)
}

func (c *EventProcessor) WatchGateways(options dynamicinformer.TweakListOptionsFunc, namespace string, handler DynamicHandler) *DynamicWatcher {
	if !c.HasGateway() {
		c.logger.Error("Cannot watch Gateways; resource not installed")
		return nil
	}
	return c.WatchDynamic(resource.GatewayResource(), options, namespace, handler)
}

func (c *EventProcessor) WatchTlsRoutes(options dynamicinformer.TweakListOptionsFunc, namespace string, handler DynamicHandler) *DynamicWatcher {
	if !c.HasTlsRoute() {
		c.logger.Error("Cannot watch TLSRoutes; resource not installed")
		return nil
	}
	return c.WatchDynamic(resource.TlsRouteResource(), options, namespace, handler)
}

func (c *EventProcessor) WatchDynamic(resource schema.GroupVersionResource, options dynamicinformer.TweakListOptionsFunc, namespace string, handler DynamicHandler) *DynamicWatcher {
	informer := dynamicinformer.NewFilteredDynamicInformer(
		c.dynamicClient,
		resource,
		namespace,
		c.resync,
		cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc},
		options,
	).Informer()
	return addEventProcessorWatcher(c, handler, corev1.SchemeGroupVersion, informer)
}

func (c *EventProcessor) WatchNamespaces(options internalinterfaces.TweakListOptionsFunc, handler NamespaceHandler) *NamespaceWatcher {
	informer := corev1informer.NewFilteredNamespaceInformer(
		c.client,
		c.resync,
		cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc},
		options)
	return addEventProcessorWatcher(c, handler, corev1.SchemeGroupVersion, informer)
}

func (c *EventProcessor) WatchNodes(handler NodeHandler) *NodeWatcher {
	informer := corev1informer.NewNodeInformer(
		c.client,
		c.resync,
		cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
	return addEventProcessorWatcher(c, handler, corev1.SchemeGroupVersion, informer)
}

func (c *EventProcessor) WatchIngresses(options internalinterfaces.TweakListOptionsFunc, namespace string, handler IngressHandler) *IngressWatcher {
	informer := networkingv1informer.NewFilteredIngressInformer(
		c.client,
		namespace,
		c.resync,
		cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc},
		options)
	return addEventProcessorWatcher(c, handler, networkingv1.SchemeGroupVersion, informer)
}

func (c *EventProcessor) WatchRoutes(options routev1interfaces.TweakListOptionsFunc, namespace string, handler RouteHandler) *RouteWatcher {
	if c.routeClient == nil {
		return nil
	}
	informer := routev1informer.NewFilteredRouteInformer(
		c.routeClient,
		namespace,
		c.resync,
		cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc},
		options)
	return addEventProcessorWatcher(c, handler, routev1.SchemeGroupVersion, informer)
}

func (c *EventProcessor) WatchSites(namespace string, handler SiteHandler) *SiteWatcher {
	informer := skupperv2alpha1informer.NewSiteInformer(
		c.skupperClient,
		namespace,
		c.resyncShort,
		cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
	return addEventProcessorWatcher(c, handler, v2alpha1.SchemeGroupVersion, informer)
}

func (c *EventProcessor) WatchListeners(namespace string, handler ListenerHandler) *ListenerWatcher {
	informer := skupperv2alpha1informer.NewListenerInformer(
		c.skupperClient,
		namespace,
		c.resyncShort,
		cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
	return addEventProcessorWatcher(c, handler, v2alpha1.SchemeGroupVersion, informer)
}

func (c *EventProcessor) WatchConnectors(namespace string, handler ConnectorHandler) *ConnectorWatcher {
	informer := skupperv2alpha1informer.NewConnectorInformer(
		c.skupperClient,
		namespace,
		c.resyncShort,
		cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
	return addEventProcessorWatcher(c, handler, v2alpha1.SchemeGroupVersion, informer)
}

func (c *EventProcessor) WatchLinks(namespace string, handler LinkHandler) *LinkWatcher {
	informer := skupperv2alpha1informer.NewLinkInformer(
		c.skupperClient,
		namespace,
		c.resyncShort,
		cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
	return addEventProcessorWatcher(c, handler, v2alpha1.SchemeGroupVersion, informer)
}

func (c *EventProcessor) WatchAccessTokens(namespace string, handler AccessTokenHandler) *AccessTokenWatcher {
	informer := skupperv2alpha1informer.NewAccessTokenInformer(
		c.skupperClient,
		namespace,
		c.resyncShort,
		cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
	return addEventProcessorWatcher(c, handler, v2alpha1.SchemeGroupVersion, informer)
}

func (c *EventProcessor) WatchAccessGrants(namespace string, handler AccessGrantHandler) *AccessGrantWatcher {
	informer := skupperv2alpha1informer.NewAccessGrantInformer(
		c.skupperClient,
		namespace,
		c.resyncShort,
		cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
	return addEventProcessorWatcher(c, handler, v2alpha1.SchemeGroupVersion, informer)
}

func (c *EventProcessor) WatchSecuredAccesses(namespace string, handler SecuredAccessHandler) *SecuredAccessWatcher {
	informer := skupperv2alpha1informer.NewSecuredAccessInformer(
		c.skupperClient,
		namespace,
		c.resyncShort,
		cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
	return addEventProcessorWatcher(c, handler, v2alpha1.SchemeGroupVersion, informer)
}

func (c *EventProcessor) WatchSecuredAccessesWithOptions(options skupperv2alpha1interfaces.TweakListOptionsFunc, namespace string, handler SecuredAccessHandler) *SecuredAccessWatcher {
	informer := skupperv2alpha1informer.NewFilteredSecuredAccessInformer(
		c.skupperClient,
		namespace,
		c.resyncShort,
		cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc},
		options)
	return addEventProcessorWatcher(c, handler, v2alpha1.SchemeGroupVersion, informer)
}

func (c *EventProcessor) WatchCertificates(namespace string, handler CertificateHandler) *CertificateWatcher {
	informer := skupperv2alpha1informer.NewCertificateInformer(
		c.skupperClient,
		namespace,
		c.resyncShort,
		cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
	return addEventProcessorWatcher(c, handler, v2alpha1.SchemeGroupVersion, informer)
}

func (c *EventProcessor) WatchRouterAccesses(namespace string, handler RouterAccessHandler) *RouterAccessWatcher {
	informer := skupperv2alpha1informer.NewRouterAccessInformer(
		c.skupperClient,
		namespace,
		c.resyncShort,
		cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
	return addEventProcessorWatcher(c, handler, v2alpha1.SchemeGroupVersion, informer)
}

func (c *EventProcessor) WatchAttachedConnectorBindings(namespace string, handler AttachedConnectorBindingHandler) *AttachedConnectorBindingWatcher {
	informer := skupperv2alpha1informer.NewAttachedConnectorBindingInformer(
		c.skupperClient,
		namespace,
		c.resyncShort,
		cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
	return addEventProcessorWatcher(c, handler, v2alpha1.SchemeGroupVersion, informer)
}

func (c *EventProcessor) WatchAttachedConnectors(namespace string, handler AttachedConnectorHandler) *AttachedConnectorWatcher {
	informer := skupperv2alpha1informer.NewAttachedConnectorInformer(
		c.skupperClient,
		namespace,
		c.resyncShort,
		cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
	return addEventProcessorWatcher(c, handler, v2alpha1.SchemeGroupVersion, informer)
}

func FilterByNamespace[V any](match func(string) bool, handler func(string, V) error) func(string, V) error {
	if match == nil {
		return handler
	}
	return func(key string, value V) error {
		namespace, _, _ := cache.SplitMetaNamespaceKey(key)
		if match(namespace) {
			return handler(key, value)
		}
		return nil
	}
}

func ByName(name string) internalinterfaces.TweakListOptionsFunc {
	return func(options *metav1.ListOptions) {
		options.FieldSelector = "metadata.name=" + name
	}
}

func SkupperResourceByName(name string) skupperv2alpha1interfaces.TweakListOptionsFunc {
	return func(options *metav1.ListOptions) {
		options.FieldSelector = "metadata.name=" + name
	}
}

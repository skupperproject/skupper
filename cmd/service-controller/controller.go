package main

import (
	"crypto/tls"
	jsonencoding "encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apimachinerytypes "k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	appsv1informer "k8s.io/client-go/informers/apps/v1"
	corev1informer "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/informers/internalinterfaces"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"

	amqp "github.com/interconnectedcloud/go-amqp"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/client"
	"github.com/skupperproject/skupper/pkg/event"
	"github.com/skupperproject/skupper/pkg/kube"
	kubeqdr "github.com/skupperproject/skupper/pkg/kube/qdr"
	"github.com/skupperproject/skupper/pkg/qdr"
	"github.com/skupperproject/skupper/pkg/service"
	"github.com/skupperproject/skupper/pkg/service_sync"
)

type Controller struct {
	origin            string
	vanClient         *client.VanClient
	policy            *client.ClusterPolicyValidator
	bridgeDefInformer cache.SharedIndexInformer
	svcDefInformer    cache.SharedIndexInformer
	svcInformer       cache.SharedIndexInformer
	headlessInformer  cache.SharedIndexInformer

	// control loop state:
	events   workqueue.RateLimitingInterface
	bindings map[string]*service.ServiceBindings
	ports    *FreePorts

	// service_sync state:
	disableServiceSync bool
	tlsConfig          *tls.Config
	amqpClient         *amqp.Client
	amqpSession        *amqp.Session
	byOrigin           map[string]map[string]types.ServiceInterface
	localServices      map[string]types.ServiceInterface
	byName             map[string]types.ServiceInterface
	heardFrom          map[string]time.Time

	definitionMonitor *DefinitionMonitor
	consoleServer     *ConsoleServer
	siteQueryServer   *SiteQueryServer
	claimVerifier     *ClaimVerifier
	tokenHandler      *SecretController
	claimHandler      *SecretController
	serviceSync       *service_sync.ServiceSync
	policyHandler     *PolicyController
}

const (
	ServiceControllerEvent       string = "ServiceControllerEvent"
	ServiceControllerError       string = "ServiceControllerError"
	ServiceControllerCreateEvent string = "ServiceControllerCreateEvent"
	ServiceControllerUpdateEvent string = "ServiceControllerUpdateEvent"
	ServiceControllerDeleteEvent string = "ServiceControllerDeleteEvent"
)

func hasProxyAnnotation(service corev1.Service) bool {
	if _, ok := service.ObjectMeta.Annotations[types.ProxyQualifier]; ok {
		return true
	} else {
		return false
	}
}

func getProxyName(name string) string {
	return name + "-proxy"
}

func getServiceName(name string) string {
	return strings.TrimSuffix(name, "-proxy")
}

func hasSkupperAnnotation(service corev1.Service, annotation string) bool {
	_, ok := service.ObjectMeta.Annotations[annotation]
	return ok
}

func hasOriginalSelector(service corev1.Service) bool {
	return hasSkupperAnnotation(service, types.OriginalSelectorQualifier)
}

func hasOriginalTargetPort(service corev1.Service) bool {
	return hasSkupperAnnotation(service, types.OriginalTargetPortQualifier)
}

func hasOriginalAssigned(service corev1.Service) bool {
	return hasSkupperAnnotation(service, types.OriginalAssignedQualifier)
}

func NewController(cli *client.VanClient, origin string, tlsConfig *tls.Config, disableServiceSync bool) (*Controller, error) {

	// create informers
	svcInformer := corev1informer.NewServiceInformer(
		cli.KubeClient,
		cli.Namespace,
		time.Second*30,
		cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
	svcDefInformer := corev1informer.NewFilteredConfigMapInformer(
		cli.KubeClient,
		cli.Namespace,
		time.Second*30,
		cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc},
		internalinterfaces.TweakListOptionsFunc(func(options *metav1.ListOptions) {
			options.FieldSelector = "metadata.name=skupper-services"
		}))
	bridgeDefInformer := corev1informer.NewFilteredConfigMapInformer(
		cli.KubeClient,
		cli.Namespace,
		time.Second*30,
		cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc},
		internalinterfaces.TweakListOptionsFunc(func(options *metav1.ListOptions) {
			options.FieldSelector = "metadata.name=skupper-internal"
		}))
	headlessInformer := appsv1informer.NewFilteredStatefulSetInformer(
		cli.KubeClient,
		cli.Namespace,
		time.Second*30,
		cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc},
		internalinterfaces.TweakListOptionsFunc(func(options *metav1.ListOptions) {
			options.LabelSelector = "internal.skupper.io/type=proxy"
		}))

	events := workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "skupper-service-controller")

	controller := &Controller{
		vanClient:          cli,
		policy:             client.NewClusterPolicyValidator(cli),
		origin:             origin,
		tlsConfig:          tlsConfig,
		bridgeDefInformer:  bridgeDefInformer,
		svcDefInformer:     svcDefInformer,
		svcInformer:        svcInformer,
		headlessInformer:   headlessInformer,
		events:             events,
		ports:              newFreePorts(),
		disableServiceSync: disableServiceSync,
	}
	AddStaticPolicyWatcher(controller.policy)

	// Organize service definitions
	controller.byOrigin = make(map[string]map[string]types.ServiceInterface)
	controller.localServices = make(map[string]types.ServiceInterface)
	controller.byName = make(map[string]types.ServiceInterface)
	controller.heardFrom = make(map[string]time.Time)

	log.Println("Setting up event handlers")
	svcDefInformer.AddEventHandler(controller.newEventHandler("servicedefs", AnnotatedKey, ConfigMapResourceVersionTest))
	bridgeDefInformer.AddEventHandler(controller.newEventHandler("bridges", AnnotatedKey, ConfigMapResourceVersionTest))
	svcInformer.AddEventHandler(controller.newEventHandler("actual-services", AnnotatedKey, ServiceResourceVersionTest))
	headlessInformer.AddEventHandler(controller.newEventHandler("statefulset", AnnotatedKey, StatefulSetResourceVersionTest))
	controller.consoleServer = newConsoleServer(cli, tlsConfig)
	controller.siteQueryServer = newSiteQueryServer(cli, tlsConfig)

	controller.definitionMonitor = newDefinitionMonitor(controller.origin, controller.vanClient, controller.svcDefInformer, controller.svcInformer)
	if enableClaimVerifier() {
		controller.claimVerifier = newClaimVerifier(controller.vanClient)
	}
	controller.tokenHandler = newTokenHandler(controller.vanClient, origin)
	controller.claimHandler = newClaimHandler(controller.vanClient, origin)
	handler := func(changed []types.ServiceInterface, deleted []string, origin string) error {
		return kube.UpdateSkupperServices(changed, deleted, origin, cli.ConfigMapManager(cli.Namespace))
	}
	controller.serviceSync = service_sync.NewServiceSync(origin, client.Version, qdr.NewConnectionFactory("amqps://"+types.QualifiedServiceName(types.LocalTransportServiceName, cli.Namespace)+":5671", tlsConfig), handler)

	controller.policyHandler = NewPolicyController(controller.vanClient)
	return controller, nil
}

type ResourceVersionTest func(a interface{}, b interface{}) bool

func ConfigMapResourceVersionTest(a interface{}, b interface{}) bool {
	aa := a.(*corev1.ConfigMap)
	bb := b.(*corev1.ConfigMap)
	return aa.ResourceVersion == bb.ResourceVersion
}

func PodResourceVersionTest(a interface{}, b interface{}) bool {
	aa := a.(*corev1.Pod)
	bb := b.(*corev1.Pod)
	return aa.ResourceVersion == bb.ResourceVersion
}

func ServiceResourceVersionTest(a interface{}, b interface{}) bool {
	aa := a.(*corev1.Service)
	bb := b.(*corev1.Service)
	return aa.ResourceVersion == bb.ResourceVersion
}

func StatefulSetResourceVersionTest(a interface{}, b interface{}) bool {
	aa := a.(*appsv1.StatefulSet)
	bb := b.(*appsv1.StatefulSet)
	return aa.ResourceVersion == bb.ResourceVersion
}

type CacheKeyStrategy func(category string, object interface{}) (string, error)

func AnnotatedKey(category string, obj interface{}) (string, error) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		return "", err
	}
	return category + "@" + key, nil
}

func SimpleKey(category string, obj interface{}) (string, error) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		return "", err
	}
	return key, nil
}

func FixedKey(category string, obj interface{}) (string, error) {
	return category, nil
}

func splitKey(key string) (string, string) {
	parts := strings.Split(key, "@")
	return parts[0], parts[1]
}

func (c *Controller) newEventHandler(category string, keyStrategy CacheKeyStrategy, test ResourceVersionTest) *cache.ResourceEventHandlerFuncs {
	return newEventHandlerFor(c.events, category, keyStrategy, test)
}

func newEventHandlerFor(events workqueue.RateLimitingInterface, category string, keyStrategy CacheKeyStrategy, test ResourceVersionTest) *cache.ResourceEventHandlerFuncs {
	return &cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			key, err := keyStrategy(category, obj)
			if err != nil {
				utilruntime.HandleError(err)
			} else {
				events.Add(key)
			}
		},
		UpdateFunc: func(old, new interface{}) {
			if !test(old, new) {
				key, err := keyStrategy(category, new)
				if err != nil {
					utilruntime.HandleError(err)
				} else {
					events.Add(key)
				}
			}
		},
		DeleteFunc: func(obj interface{}) {
			key, err := keyStrategy(category, obj)
			if err != nil {
				utilruntime.HandleError(err)
			} else {
				events.Add(key)
			}
		},
	}
}

func (c *Controller) Run(stopCh <-chan struct{}) error {
	// fire up the informers
	go c.svcDefInformer.Run(stopCh)
	go c.bridgeDefInformer.Run(stopCh)
	go c.svcInformer.Run(stopCh)
	go c.headlessInformer.Run(stopCh)

	defer utilruntime.HandleCrash()
	defer c.events.ShutDown()

	log.Println("Starting the Skupper controller")

	log.Println("Waiting for informer caches to sync")
	if ok := cache.WaitForCacheSync(stopCh, c.svcDefInformer.HasSynced, c.bridgeDefInformer.HasSynced, c.svcInformer.HasSynced, c.headlessInformer.HasSynced); !ok {
		return fmt.Errorf("Failed to wait for caches to sync")
	}

	log.Println("Starting workers")
	if !c.disableServiceSync {
		c.serviceSync.Start(stopCh)
	}
	go wait.Until(c.runServiceCtrl, time.Second, stopCh)
	c.definitionMonitor.start(stopCh)
	c.siteQueryServer.start(stopCh)
	c.consoleServer.start(stopCh)
	if c.claimVerifier != nil {
		c.claimVerifier.start(stopCh)
	}
	c.tokenHandler.start(stopCh)
	c.claimHandler.start(stopCh)
	c.policyHandler.start(stopCh)

	log.Println("Started workers")
	<-stopCh
	log.Println("Shutting down workers")
	c.definitionMonitor.stop()
	c.tokenHandler.stop()
	c.claimHandler.stop()
	c.policyHandler.stop()

	return nil
}

func (c *Controller) DeleteService(svc *corev1.Service, options *metav1.DeleteOptions) error {
	if options == nil {
		options = &metav1.DeleteOptions{}
	}
	event.Recordf(ServiceControllerDeleteEvent, "Deleting service %s", svc.ObjectMeta.Name)
	return c.vanClient.ServiceManager(c.vanClient.Namespace).DeleteService(svc, options)
}

func (c *Controller) UpdateService(svc *corev1.Service) (*corev1.Service, error) {
	return c.vanClient.ServiceManager(c.vanClient.Namespace).UpdateService(svc)
}

func (c *Controller) CreateService(svc *corev1.Service) (*corev1.Service, error) {
	setOwnerReferences(&svc.ObjectMeta)
	return c.vanClient.ServiceManager(c.vanClient.Namespace).CreateService(svc)
}

func (c *Controller) IsOwned(service *corev1.Service) bool {
	return isOwned(service)
}

func (c *Controller) GetService(name string, options *metav1.GetOptions) (*corev1.Service, bool, error) {
	obj, exists, err := c.svcInformer.GetStore().GetByKey(c.namespaced(name))
	if err != nil {
		return nil, false, err
	}
	if !exists {
		return nil, false, nil
	}
	actual := obj.(*corev1.Service)
	return actual, true, nil
}

func (c *Controller) ListServices(options *metav1.ListOptions) ([]corev1.Service, error) {
	if options == nil {
		options = &metav1.ListOptions{}
	}
	list, err := c.vanClient.ServiceManager(c.vanClient.Namespace).ListServices(options)
	if err != nil {
		return nil, err
	}
	return list, nil
}

func (c *Controller) updateActualServices() {
	for k, v := range c.bindings {
		event.Recordf(ServiceControllerEvent, "Checking service for: %s", k)
		err := v.RealiseIngress()
		if err != nil {
			event.Recordf(ServiceControllerError, "Error updating services: %s", err)
		}
	}
	services := c.svcInformer.GetStore().List()
	for _, v := range services {
		svc := v.(*corev1.Service)
		_, deleteSvc := c.getBindingsForService(svc)
		if deleteSvc {
			event.Recordf(ServiceControllerDeleteEvent, "No service binding found for %s", svc.ObjectMeta.Name)
			c.DeleteService(svc, nil)
			c.handleRemovingTlsSupport(types.SkupperServiceCertPrefix + svc.ObjectMeta.Name)
		}
	}
}

func (c *Controller) getBindingsForService(svc *corev1.Service) (*service.ServiceBindings, bool) {
	owned := isOwned(svc)
	if owned && svc.Spec.ClusterIP == "None" {
		if svcName := svc.ObjectMeta.Annotations[types.ServiceQualifier]; svcName != "" {
			bindings := c.bindings[svcName]
			if bindings == nil || !bindings.IsHeadless() {
				c.DeleteService(svc, nil)
			}
			return nil, false
		}
	}
	if bindings := c.bindings[svc.ObjectMeta.Name]; bindings != nil {
		return bindings, false
	}
	return nil, owned
}

// TODO: move to pkg
func equalOwnerRefs(a, b []metav1.OwnerReference) bool {
	if len(a) != len(b) {
		return false
	}
	for i, v := range a {
		if v != b[i] {
			return false
		}
	}
	return true
}

func setOwnerReferences(o *metav1.ObjectMeta) {
	owner := getOwnerReference()
	if owner != nil {
		o.OwnerReferences = []metav1.OwnerReference{*owner}
	}
}

func getOwnerReference() *metav1.OwnerReference {
	ownerName := os.Getenv("OWNER_NAME")
	ownerUid := os.Getenv("OWNER_UID")
	if ownerName == "" || ownerUid == "" {
		return nil
	} else {
		return &metav1.OwnerReference{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
			Name:       ownerName,
			UID:        apimachinerytypes.UID(ownerUid),
		}
	}
}

func isOwned(service *corev1.Service) bool {
	owner := getOwnerReference()
	if owner == nil {
		return false
	}

	ownerRefs := []metav1.OwnerReference{
		*owner,
	}

	if controlled, ok := service.ObjectMeta.Annotations[types.ControlledQualifier]; ok {
		if controlled == "true" {
			return equalOwnerRefs(service.ObjectMeta.OwnerReferences, ownerRefs)
		} else {
			return false
		}
	} else {
		return false
	}
}

func (c *Controller) namespaced(name string) string {
	return c.vanClient.Namespace + "/" + name
}

func parseServiceDefinitions(cm *corev1.ConfigMap) map[string]types.ServiceInterface {
	definitions := make(map[string]types.ServiceInterface)
	if len(cm.Data) > 0 {
		for _, v := range cm.Data {
			si := types.ServiceInterface{}
			err := jsonencoding.Unmarshal([]byte(v), &si)
			if err == nil {
				definitions[si.Address] = si
			}
		}
	}
	return definitions
}

func (c *Controller) runServiceCtrl() {
	for c.processNextEvent() {
	}
}

func (c *Controller) getInitialBridgeConfig() (*qdr.BridgeConfig, error) {
	name := c.namespaced(types.TransportConfigMapName)
	obj, exists, err := c.bridgeDefInformer.GetStore().GetByKey(name)
	if err != nil {
		return nil, fmt.Errorf("Error reading skupper-internal from cache: %s", err)
	} else if exists {
		cm, ok := obj.(*corev1.ConfigMap)
		if !ok {
			return nil, fmt.Errorf("Expected ConfigMap for %s but got %#v", name, obj)
		}
		return qdr.GetBridgeConfigFromConfigMap(cm)
	} else {
		return nil, nil
	}
}

func (c *Controller) updateBridgeConfig(name string) error {
	obj, exists, err := c.bridgeDefInformer.GetStore().GetByKey(name)
	if err != nil {
		return fmt.Errorf("Error reading skupper-internal from cache: %s", err)
	} else if !exists {
		return fmt.Errorf("skupper-internal does not exist")
	} else {
		cm, ok := obj.(*corev1.ConfigMap)
		if !ok {
			return fmt.Errorf("Expected ConfigMap for %s but got %#v", name, obj)
		}
		desiredBridges := service.RequiredBridges(c.bindings, c.origin)
		update, err := desiredBridges.UpdateConfigMap(cm)
		if err != nil {
			return fmt.Errorf("Error updating %s: %s", cm.ObjectMeta.Name, err)
		}
		if update {
			event.Recordf(ServiceControllerUpdateEvent, "Updating %s", cm.ObjectMeta.Name)
			_, err = c.vanClient.ConfigMapManager(c.vanClient.Namespace).UpdateConfigMap(cm)
			if err != nil {
				return fmt.Errorf("Failed to update %s: %v", name, err.Error())
			}
		}
	}
	return nil
}

func (c *Controller) initialiseServiceBindingsMap() (map[string][]int, error) {
	c.bindings = map[string]*service.ServiceBindings{}
	// on first initiliasing the service bindings map, need to get any
	// port allocations from bridge config
	bridges, err := c.getInitialBridgeConfig()
	if err != nil {
		return nil, err
	}
	allocations := c.ports.getPortAllocations(bridges)
	// TODO: should deduce the ports in use by the router by
	// reading config rather than hardcoding them here
	c.ports.inuse(int(types.AmqpDefaultPort))
	c.ports.inuse(int(types.AmqpsDefaultPort))
	c.ports.inuse(int(types.EdgeListenerPort))
	c.ports.inuse(int(types.InterRouterListenerPort))
	c.ports.inuse(int(types.ConsoleDefaultServicePort))
	c.ports.inuse(9090) // currently hardcoded in config
	return allocations, nil

}

func (c *Controller) deleteServiceBindings(k string, v *service.ServiceBindings) {
	if v != nil {
		v.Stop()
	}
	delete(c.bindings, k)
}

func (c *Controller) updateServiceSync(defs *corev1.ConfigMap) {
	definitions := parseServiceDefinitions(defs)
	c.serviceSync.LocalDefinitionsUpdated(definitions)
}

func (c *Controller) deleteHeadlessProxy(statefulset *appsv1.StatefulSet) error {
	return c.vanClient.StatefulSetManager(c.vanClient.Namespace).DeleteStatefulSet(statefulset, &metav1.DeleteOptions{})
}

func (c *Controller) ensureHeadlessProxyFor(bindings *service.ServiceBindings, statefulset *appsv1.StatefulSet) error {
	serviceInterface := bindings.AsServiceInterface()
	config, err := qdr.GetRouterConfigForHeadlessProxy(serviceInterface, c.origin, client.Version, c.vanClient.Namespace)
	if err != nil {
		return err
	}

	_, err = kube.CheckProxyStatefulSet(client.GetRouterImageDetails(), serviceInterface, statefulset, config, c.vanClient.Namespace, c.vanClient)
	return err
}

func (c *Controller) createHeadlessProxyFor(bindings *service.ServiceBindings) error {
	serviceInterface := bindings.AsServiceInterface()
	config, err := qdr.GetRouterConfigForHeadlessProxy(serviceInterface, c.origin, client.Version, c.vanClient.Namespace)
	if err != nil {
		return err
	}

	_, err = kube.NewProxyStatefulSet(client.GetRouterImageDetails(), serviceInterface, config, c.vanClient.Namespace, c.vanClient)
	return err
}

func (c *Controller) updateHeadlessProxies() {
	for _, v := range c.bindings {
		if v.IsHeadless() {
			c.ensureHeadlessProxyFor(v, nil)
		}
	}
	proxies := c.headlessInformer.GetStore().List()
	for _, v := range proxies {
		proxy := v.(*appsv1.StatefulSet)
		if c.getBindingsForHeadlessProxy(proxy) == nil {
			c.deleteHeadlessProxy(proxy)
		}
	}
}

func (c *Controller) getBindingsForHeadlessProxy(statefulset *appsv1.StatefulSet) *service.ServiceBindings {
	svcName := statefulset.ObjectMeta.Annotations[types.ServiceQualifier]
	bindings, ok := c.bindings[svcName]
	if !ok || bindings == nil || !bindings.IsHeadless() {
		return nil
	}
	return bindings
}

func (c *Controller) processNextEvent() bool {

	obj, shutdown := c.events.Get()

	if shutdown {
		return false
	}

	err := func(obj interface{}) error {
		defer c.events.Done(obj)

		var ok bool
		var key string
		if key, ok = obj.(string); !ok {
			// invalid item
			c.events.Forget(obj)
			return fmt.Errorf("expected string in events but got %#v", obj)
		} else {
			category, name := splitKey(key)
			switch category {
			case "servicedefs":
				event.Record(ServiceControllerEvent, "Service definitions have changed")
				// get the configmap, parse the json, check against the current servicebindings map
				obj, exists, err := c.svcDefInformer.GetStore().GetByKey(name)
				if err != nil {
					return fmt.Errorf("Error reading skupper-services from cache: %s", err)
				} else if exists {
					var portAllocations map[string][]int
					if c.bindings == nil {
						portAllocations, err = c.initialiseServiceBindingsMap()
						if err != nil {
							return err
						}
					}
					cm, ok := obj.(*corev1.ConfigMap)
					if !ok {
						return fmt.Errorf("Expected ConfigMap for %s but got %#v", name, obj)
					}
					c.updateServiceSync(cm)
					if cm.Data != nil && len(cm.Data) > 0 {
						for k, v := range cm.Data {
							si := types.ServiceInterface{}
							err := jsonencoding.Unmarshal([]byte(v), &si)
							if err == nil {
								c.updateServiceBindings(si, portAllocations)
								c.handleEnableTlsSupport(si.Address, si.TlsCredentials)
							} else {
								event.Recordf(ServiceControllerError, "Could not parse service definition for %s: %s", k, err)
							}
						}
						for k, v := range c.bindings {
							_, ok := cm.Data[k]
							if !ok {
								c.deleteServiceBindings(k, v)
							}
						}
					} else if len(c.bindings) > 0 {
						for k, v := range c.bindings {
							c.deleteServiceBindings(k, v)
						}
					}
				}
				c.updateBridgeConfig(c.namespaced(types.TransportConfigMapName))
				c.updateActualServices()
				c.updateHeadlessProxies()
			case "bridges":
				if c.bindings == nil {
					// not yet initialised
					return nil
				}
				err := c.updateBridgeConfig(name)
				if err != nil {
					return err
				}
			case "actual-services":
				if c.bindings == nil {
					// not yet initialised
					return nil
				}
				event.Recordf(ServiceControllerEvent, "service event for %s", name)
				// name is fully qualified name of the actual service
				obj, exists, err := c.svcInformer.GetStore().GetByKey(name)
				if err != nil {
					return fmt.Errorf("Error reading service %s from cache: %s", name, err)
				} else if exists {
					svc, ok := obj.(*corev1.Service)
					if !ok {
						return fmt.Errorf("Expected Service for %s but got %#v", name, obj)
					}
					bindings, deleteSvc := c.getBindingsForService(svc)
					if bindings != nil {
						// check that service matches binding def, else update it
						err = bindings.RealiseIngress()
						if err != nil {
							return err
						}
					} else if deleteSvc {
						err = c.DeleteService(svc, nil)
						if err != nil {
							return err
						}
					}
				} else {
					_, unqualified, err := cache.SplitMetaNamespaceKey(name)
					if err != nil {
						return fmt.Errorf("Could not determine name of deleted service from key %s: %w", name, err)
					}
					bindings := c.bindings[unqualified]
					if bindings != nil {
						err = bindings.RealiseIngress()
						if err != nil {
							return err
						}
					}
				}
			case "targetpods":
				event.Recordf(ServiceControllerEvent, "Got targetpods event %s", name)
				// name is the address of the skupper service
				c.updateBridgeConfig(c.namespaced(types.TransportConfigMapName))
				c.updateActualServices()
			case "statefulset":
				event.Recordf(ServiceControllerEvent, "Got statefulset proxy event %s", name)
				obj, exists, err := c.headlessInformer.GetStore().GetByKey(name)
				if err != nil {
					return fmt.Errorf("Error reading statefulset %s from cache: %s", name, err)
				} else if exists {
					statefulset, ok := obj.(*appsv1.StatefulSet)
					if !ok {
						return fmt.Errorf("Expected StatefulSet for %s but got %#v", name, obj)
					}
					// a headless proxy was created or updated, does it match the desired binding?
					bindings := c.getBindingsForHeadlessProxy(statefulset)
					if bindings == nil {
						err = c.deleteHeadlessProxy(statefulset)
						if err != nil {
							return err
						}
					} else {
						err = c.ensureHeadlessProxyFor(bindings, statefulset)
						if err != nil {
							return err
						}
					}
				} else {
					// a headless proxy was deleted, does it need to be recreated?
					_, unqualified, err := cache.SplitMetaNamespaceKey(name)
					if err != nil {
						return fmt.Errorf("Could not determine name of deleted statefulset from key %s: %w", name, err)
					}
					for _, v := range c.bindings {
						if v.IsHeadless() && v.HeadlessName() == unqualified {
							err = c.createHeadlessProxyFor(v)
							if err != nil {
								return err
							}
						}
					}

				}
			default:
				c.events.Forget(obj)
				return fmt.Errorf("unexpected event key %s (%s, %s)", key, category, name)
			}
			c.events.Forget(obj)
		}
		return nil
	}(obj)

	if err != nil {
		if c.events.NumRequeues(obj) < 5 {
			event.Recordf(ServiceControllerError, "Requeuing %v after error: %v", obj, err)
			c.events.AddRateLimited(obj)
		} else {
			event.Recordf(ServiceControllerError, "Giving up on %v after error: %v", obj, err)
		}
		utilruntime.HandleError(err)
		return true
	}

	return true
}

func (c *Controller) handleEnableTlsSupport(address string, tlsCredentials string) error {

	if len(tlsCredentials) > 0 {
		_, _, err := c.vanClient.SecretManager(c.vanClient.Namespace).GetSecret(tlsCredentials, &metav1.GetOptions{})

		// If the requested certificate is one generated by skupper it can be generated in other sites as well
		if err != nil && strings.HasPrefix(tlsCredentials, types.SkupperServiceCertPrefix) {

			configmap, _, err := c.vanClient.ConfigMapManager(c.vanClient.Namespace).GetConfigMap(types.TransportConfigMapName, &metav1.GetOptions{})

			if err != nil {
				return err
			}

			serviceCredential := types.Credential{
				CA:          types.ServiceCaSecret,
				Name:        tlsCredentials,
				Subject:     address,
				Hosts:       []string{address},
				ConnectJson: false,
				Post:        false,
			}

			ownerReference := metav1.OwnerReference{
				APIVersion: "v1",
				Kind:       "ConfigMap",
				Name:       configmap.Name,
				UID:        configmap.UID,
			}
			serviceSecret, err := kube.NewSecret(serviceCredential, &ownerReference, c.vanClient.Namespace, c.vanClient.KubeClient)

			if err != nil {
				return err
			}

			err = kubeqdr.AddSslProfile(serviceSecret.Name, c.vanClient.ConfigMapManager(c.vanClient.Namespace))
			if err != nil {
				return err
			}

			return nil
		}
	}

	return nil
}

func (c *Controller) handleRemovingTlsSupport(tlsCredentials string) error {

	if len(tlsCredentials) > 0 {
		err := kubeqdr.RemoveSslProfile(tlsCredentials, c.vanClient.ConfigMapManager(c.vanClient.Namespace))
		if err != nil {
			return err
		}

		tlsCredentialsSecret, _, err := c.vanClient.SecretManager(c.vanClient.Namespace).GetSecret(tlsCredentials, &metav1.GetOptions{})

		if err == nil {

			err = c.vanClient.SecretManager(c.vanClient.Namespace).DeleteSecret(tlsCredentialsSecret, &metav1.DeleteOptions{})

			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (c *Controller) NewTargetResolver(address string, selector string, skipTargetStatus bool) (service.TargetResolver, error) {
	resolver := kube.NewPodTargetResolver(c.vanClient.KubeClient, c.vanClient.Namespace, address, selector, skipTargetStatus)
	resolver.AddEventHandler(c.newEventHandler("targetpods@"+address, FixedKey, PodResourceVersionTest))
	err := resolver.Start()
	return resolver, err
}

func (c *Controller) NewServiceIngress(def *types.ServiceInterface) service.ServiceIngress {
	if def.Headless != nil {
		return kube.NewHeadlessServiceIngress(c, def.Origin)
	}
	return kube.NewServiceIngressAlways(c)
}

func (c *Controller) realiseServiceBindings(required types.ServiceInterface, ports []int) error {
	bindings := service.NewServiceBindings(required, ports, c)
	return bindings.RealiseIngress()
}

func (c *Controller) updateServiceBindings(required types.ServiceInterface, portAllocations map[string][]int) error {
	res := c.policy.ValidateImportService(required.Address)
	bindings := c.bindings[required.Address]
	if bindings == nil {
		if !res.Allowed() {
			event.Recordf(ServiceControllerError, "Policy validation error: service %s cannot be created", required.Address)
			return nil
		}
		var ports []int
		// headless services use distinct proxy pods, so don't need to allocate a port
		if required.Headless != nil {
			ports = required.Ports
		} else {
			if portAllocations != nil {
				// existing bridge configuration is used on initialising map to recover
				// any previous port allocations
				ports = portAllocations[required.Address]
			}
			if len(ports) == 0 {
				for i := 0; i < len(required.Ports); i++ {
					port, err := c.ports.nextFreePort()
					if err != nil {
						return err
					}
					ports = append(ports, port)
				}
			}
		}

		c.bindings[required.Address] = service.NewServiceBindings(required, ports, c)
	} else {
		if !res.Allowed() {
			event.Recordf(ServiceControllerError, "Policy validation error: service %s has been removed", required.Address)
			delete(c.bindings, required.Address)
			return nil
		}
		// check it is configured correctly
		bindings.Update(required, c)
	}
	return nil
}

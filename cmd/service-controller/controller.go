package main

import (
	"crypto/tls"
	jsonencoding "encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/skupperproject/skupper/pkg/utils"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apimachinerytypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
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
	"github.com/skupperproject/skupper/pkg/kube"
	"github.com/skupperproject/skupper/pkg/qdr"
)

type Controller struct {
	origin            string
	vanClient         *client.VanClient
	bridgeDefInformer cache.SharedIndexInformer
	svcDefInformer    cache.SharedIndexInformer
	svcInformer       cache.SharedIndexInformer
	headlessInformer  cache.SharedIndexInformer

	//control loop state:
	events   workqueue.RateLimitingInterface
	bindings map[string]*ServiceBindings
	ports    *FreePorts

	//service_sync state:
	tlsConfig       *tls.Config
	amqpClient      *amqp.Client
	amqpSession     *amqp.Session
	byOrigin        map[string]map[string]types.ServiceInterface
	localServices   map[string]types.ServiceInterface
	byName          map[string]types.ServiceInterface
	desiredServices map[string]types.ServiceInterface
	heardFrom       map[string]time.Time

	definitionMonitor *DefinitionMonitor
	consoleServer     *ConsoleServer
	siteQueryServer   *SiteQueryServer
	configSync        *ConfigSync
}

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

func hasRouterSelector(service corev1.Service) bool {
	_, ok := service.Spec.Selector["skupper.io/component"]
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

func NewController(cli *client.VanClient, origin string, tlsConfig *tls.Config) (*Controller, error) {

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
		vanClient:         cli,
		origin:            origin,
		tlsConfig:         tlsConfig,
		bridgeDefInformer: bridgeDefInformer,
		svcDefInformer:    svcDefInformer,
		svcInformer:       svcInformer,
		headlessInformer:  headlessInformer,
		events:            events,
		ports:             newFreePorts(),
	}

	// Organize service definitions
	controller.byOrigin = make(map[string]map[string]types.ServiceInterface)
	controller.localServices = make(map[string]types.ServiceInterface)
	controller.byName = make(map[string]types.ServiceInterface)
	controller.desiredServices = make(map[string]types.ServiceInterface)
	controller.heardFrom = make(map[string]time.Time)

	log.Println("Setting up event handlers")
	svcDefInformer.AddEventHandler(controller.newEventHandler("servicedefs", AnnotatedKey, ConfigMapResourceVersionTest))
	bridgeDefInformer.AddEventHandler(controller.newEventHandler("bridges", AnnotatedKey, ConfigMapResourceVersionTest))
	svcInformer.AddEventHandler(controller.newEventHandler("actual-services", AnnotatedKey, ServiceResourceVersionTest))
	headlessInformer.AddEventHandler(controller.newEventHandler("statefulset", AnnotatedKey, StatefulSetResourceVersionTest))
	controller.consoleServer = newConsoleServer(cli, tlsConfig)
	controller.siteQueryServer = newSiteQueryServer(cli, tlsConfig)

	controller.definitionMonitor = newDefinitionMonitor(controller.origin, controller.vanClient, controller.svcDefInformer, controller.svcInformer)
	controller.configSync = newConfigSync(controller.bridgeDefInformer, tlsConfig)
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
	go wait.Until(c.runServiceSync, time.Second, stopCh)
	go wait.Until(c.runServiceCtrl, time.Second, stopCh)
	c.definitionMonitor.start(stopCh)
	c.siteQueryServer.start(stopCh)
	c.consoleServer.start(stopCh)
	c.configSync.start(stopCh)

	log.Println("Started workers")
	<-stopCh
	log.Println("Shutting down workers")
	c.configSync.stop()
	c.definitionMonitor.stop()

	return nil
}

func (c *Controller) createServiceFor(desired *ServiceBindings) error {
	log.Println("Creating new service for ", desired.address)
	_, err := kube.NewServiceForAddress(desired.address, desired.publicPort, desired.ingressPort, getOwnerReference(), c.vanClient.Namespace, c.vanClient.KubeClient)
	if err != nil {
		log.Printf("Error while creating service %s: %s", desired.address, err)
	}
	return err
}

func (c *Controller) createHeadlessServiceFor(desired *ServiceBindings) error {
	log.Println("Creating new headless service for ", desired.address)
	_, err := kube.NewHeadlessServiceForAddress(desired.address, desired.publicPort, desired.ingressPort, getOwnerReference(), c.vanClient.Namespace, c.vanClient.KubeClient)
	if err != nil {
		log.Printf("Error while creating headless service %s: %s", desired.address, err)
	}
	return err
}

func equivalentSelectors(a map[string]string, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if v2, ok := b[k]; !ok || v != v2 {
			return false
		}
	}
	for k, v := range b {
		if v2, ok := a[k]; !ok || v != v2 {
			return false
		}
	}
	return true
}

func (c *Controller) checkServiceFor(desired *ServiceBindings, actual *corev1.Service) error {
	log.Printf("Checking service changes for %s", actual.ObjectMeta.Name)
	update := false
	if len(actual.Spec.Ports) > 0 {
		if actual.Spec.Ports[0].Port != int32(desired.publicPort) {
			update = true
			actual.Spec.Ports[0].Port = int32(desired.publicPort)
		}
		if actual.Spec.Ports[0].TargetPort.IntValue() != desired.ingressPort {
			update = true
			originalAssignedPort, _ := strconv.Atoi(actual.Annotations[types.OriginalAssignedQualifier])
			actualTargetPort := actual.Spec.Ports[0].TargetPort.IntValue()
			if actual.ObjectMeta.Annotations == nil {
				actual.ObjectMeta.Annotations = map[string]string{}
			}
			// If target port has been modified by user
			if actualTargetPort != originalAssignedPort {
				actual.ObjectMeta.Annotations[types.OriginalTargetPortQualifier] = strconv.Itoa(actualTargetPort)
			}
			actual.ObjectMeta.Annotations[types.OriginalAssignedQualifier] = strconv.Itoa(desired.ingressPort)
			actual.Spec.Ports[0].TargetPort = intstr.FromInt(desired.ingressPort)
		}
	}
	if desired.headless == nil && !equivalentSelectors(actual.Spec.Selector, kube.GetLabelsForRouter()) {
		update = true
		if actual.ObjectMeta.Annotations == nil {
			actual.ObjectMeta.Annotations = map[string]string{}
		}
		actual.ObjectMeta.Annotations[types.OriginalSelectorQualifier] = utils.StringifySelector(actual.Spec.Selector)
		actual.Spec.Selector = kube.GetLabelsForRouter()
	}
	if update {
		_, err := c.vanClient.KubeClient.CoreV1().Services(c.vanClient.Namespace).Update(actual)
		return err
	}
	return nil
}

func (c *Controller) ensureServiceFor(desired *ServiceBindings) error {
	log.Println("Checking service for: ", desired.address)
	obj, exists, err := c.svcInformer.GetStore().GetByKey(c.namespaced(desired.address))
	if err != nil {
		return fmt.Errorf("Error checking service %s", err)
	} else if !exists {
		if desired.headless == nil {
			return c.createServiceFor(desired)
		} else if desired.origin == "" {
			// i.e. originating namespace
			log.Printf("Headless service does not exist for for %s", desired.address)
			return nil
		} else {
			return c.createHeadlessServiceFor(desired)
		}
	} else {
		svc := obj.(*corev1.Service)
		return c.checkServiceFor(desired, svc)
	}
}

func (c *Controller) deleteService(svc *corev1.Service) error {
	log.Println("Deleting service ", svc.ObjectMeta.Name)
	return c.vanClient.KubeClient.CoreV1().Services(c.vanClient.Namespace).Delete(svc.ObjectMeta.Name, &metav1.DeleteOptions{})
}

func (c *Controller) updateActualServices() {
	for _, v := range c.bindings {
		c.ensureServiceFor(v)
	}
	services := c.svcInformer.GetStore().List()
	for _, v := range services {
		svc := v.(*corev1.Service)
		if c.bindings[svc.ObjectMeta.Name] == nil && isOwned(svc) {
			log.Println("No service binding found for ", svc.ObjectMeta.Name)
			c.deleteService(svc)
		}
	}
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

func (c *Controller) parseServiceDefinitions(cm *corev1.ConfigMap) map[string]types.ServiceInterface {
	definitions := make(map[string]types.ServiceInterface)
	if len(cm.Data) > 0 {
		for _, v := range cm.Data {
			si := types.ServiceInterface{}
			err := jsonencoding.Unmarshal([]byte(v), &si)
			if err == nil {
				definitions[si.Address] = si
			}
		}
		c.desiredServices = definitions
	}
	return definitions
}

func (c *Controller) runServiceCtrl() {
	for c.processNextEvent() {
	}
}

func (c *Controller) getInitialBridgeConfig() (*qdr.BridgeConfig, error) {
	name := c.namespaced("skupper-internal")
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
		return fmt.Errorf("skupper-internal does not exist: %v", err.Error())
	} else {
		cm, ok := obj.(*corev1.ConfigMap)
		if !ok {
			return fmt.Errorf("Expected ConfigMap for %s but got %#v", name, obj)
		}
		desiredBridges := requiredBridges(c.bindings, c.origin)
		update, err := desiredBridges.UpdateConfigMap(cm)
		if err != nil {
			return fmt.Errorf("Error updating %s: %s", cm.ObjectMeta.Name, err)
		}
		if update {
			log.Printf("Updating %s", cm.ObjectMeta.Name)
			_, err = c.vanClient.KubeClient.CoreV1().ConfigMaps(c.vanClient.Namespace).Update(cm)
			if err != nil {
				return fmt.Errorf("Failed to update %s: %v", name, err.Error())
			}
		}
	}
	return nil
}

func (c *Controller) initialiseServiceBindingsMap() (map[string]int, error) {
	c.bindings = map[string]*ServiceBindings{}
	//on first initiliasing the service bindings map, need to get any
	//port allocations from bridge config
	bridges, err := c.getInitialBridgeConfig()
	if err != nil {
		return nil, err
	}
	allocations := c.ports.getPortAllocations(bridges)
	//TODO: should deduce the ports in use by the router by
	//reading config rather than hardcoding them here
	c.ports.inuse(int(types.AmqpDefaultPort))
	c.ports.inuse(int(types.AmqpsDefaultPort))
	c.ports.inuse(int(types.EdgeListenerPort))
	c.ports.inuse(int(types.InterRouterListenerPort))
	c.ports.inuse(int(types.ConsoleDefaultServicePort))
	c.ports.inuse(9090) //currently hardcoded in config
	return allocations, nil

}

func (c *Controller) deleteServiceBindings(k string, v *ServiceBindings) {
	if v != nil {
		v.stop()
	}
	delete(c.bindings, k)
}

func (c *Controller) updateServiceSync(defs *corev1.ConfigMap) {
	c.serviceSyncDefinitionsUpdated(c.parseServiceDefinitions(defs))
}

func (c *Controller) deleteHeadlessProxy(statefulset *appsv1.StatefulSet) error {
	return c.vanClient.KubeClient.AppsV1().StatefulSets(c.vanClient.Namespace).Delete(statefulset.ObjectMeta.Name, &metav1.DeleteOptions{})
}

func (c *Controller) ensureHeadlessProxyFor(bindings *ServiceBindings, statefulset *appsv1.StatefulSet) error {
	serviceInterface := asServiceInterface(bindings)
	config, err := qdr.GetRouterConfigForHeadlessProxy(serviceInterface, c.origin, client.Version, c.vanClient.Namespace)
	if err != nil {
		return err
	}

	_, err = kube.CheckProxyStatefulSet(client.GetRouterImageDetails(), serviceInterface, statefulset, config, c.vanClient.Namespace, c.vanClient.KubeClient)
	return err
}

func (c *Controller) createHeadlessProxyFor(bindings *ServiceBindings) error {
	serviceInterface := asServiceInterface(bindings)
	config, err := qdr.GetRouterConfigForHeadlessProxy(serviceInterface, c.origin, client.Version, c.vanClient.Namespace)
	if err != nil {
		return err
	}

	_, err = kube.NewProxyStatefulSet(client.GetRouterImageDetails(), serviceInterface, config, c.vanClient.Namespace, c.vanClient.KubeClient)
	return err
}

func (c *Controller) updateHeadlessProxies() {
	for _, v := range c.bindings {
		if v.headless != nil {
			c.ensureHeadlessProxyFor(v, nil)
		}
	}
	proxies := c.headlessInformer.GetStore().List()
	for _, v := range proxies {
		proxy := v.(*appsv1.StatefulSet)
		def, ok := c.bindings[proxy.Spec.ServiceName]
		if !ok || def == nil || def.headless == nil {
			c.deleteHeadlessProxy(proxy)
		}
	}
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
				log.Printf("Service definitions have changed")
				//get the configmap, parse the json, check against the current servicebindings map
				obj, exists, err := c.svcDefInformer.GetStore().GetByKey(name)
				if err != nil {
					return fmt.Errorf("Error reading skupper-services from cache: %s", err)
				} else if exists {
					var portAllocations map[string]int
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
							} else {
								log.Printf("Could not parse service definition for %s: %s", k, err)
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
				c.updateBridgeConfig(c.namespaced("skupper-internal"))
				c.updateActualServices()
				c.updateHeadlessProxies()
			case "bridges":
				if c.bindings == nil {
					//not yet initialised
					return nil
				}
				err := c.updateBridgeConfig(name)
				if err != nil {
					return err
				}
			case "actual-services":
				if c.bindings == nil {
					//not yet initialised
					return nil
				}
				log.Printf("service event for %s", name)
				//name is fully qualified name of the actual service
				obj, exists, err := c.svcInformer.GetStore().GetByKey(name)
				if err != nil {
					return fmt.Errorf("Error reading service %s from cache: %s", name, err)
				} else if exists {
					svc, ok := obj.(*corev1.Service)
					if !ok {
						return fmt.Errorf("Expected Service for %s but got %#v", name, obj)
					}
					bindings := c.bindings[svc.ObjectMeta.Name]
					if bindings == nil {
						if isOwned(svc) {
							err = c.deleteService(svc)
							if err != nil {
								return err
							}
						}
					} else {
						//check that service matches binding def, else update it
						err = c.checkServiceFor(bindings, svc)
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
						if bindings.headless == nil {
							err = c.createServiceFor(bindings)
						} else if bindings.origin != "" {
							err = c.createHeadlessServiceFor(bindings)
						}
						if err != nil {
							return err
						}
					}
				}
			case "targetpods":
				log.Printf("Got targetpods event %s", name)
				//name is the address of the skupper service
				c.updateBridgeConfig(c.namespaced("skupper-internal"))
			case "statefulset":
				log.Printf("Got statefulset proxy event %s", name)
				obj, exists, err := c.headlessInformer.GetStore().GetByKey(name)
				if err != nil {
					return fmt.Errorf("Error reading statefulset %s from cache: %s", name, err)
				} else if exists {
					statefulset, ok := obj.(*appsv1.StatefulSet)
					if !ok {
						return fmt.Errorf("Expected StatefulSet for %s but got %#v", name, obj)
					}
					// a headless proxy was created or updated, does it match the desired binding?
					bindings, ok := c.bindings[statefulset.Spec.ServiceName]
					if !ok || bindings == nil || bindings.headless == nil {
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
						if v.headless != nil && v.headless.Name == unqualified {
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
			log.Printf("[controller] Requeuing %v after error: %v", obj, err)
			c.events.AddRateLimited(obj)
		} else {
			log.Printf("[controller] Giving up on %v after error: %v", obj, err)
		}
		utilruntime.HandleError(err)
		return true
	}

	return true
}

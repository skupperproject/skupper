package main

import (
	"context"
	"crypto/tls"
	jsonencoding "encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/skupperproject/skupper/pkg/utils"

	"github.com/skupperproject/skupper/pkg/images"
	"github.com/skupperproject/skupper/pkg/version"
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
	"github.com/skupperproject/skupper/pkg/flow"
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
	externalBridges   cache.SharedIndexInformer

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
	flowController    *flow.FlowController
	ipLookup          *IpLookup
	policyHandler     *PolicyController
	nodeWatcher       *NodeWatcher
	tlsManager        *kubeqdr.TlsManager
	eventHandler      event.EventHandlerInterface
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
	externalBridges := appsv1informer.NewFilteredDeploymentInformer(
		cli.KubeClient,
		cli.Namespace,
		time.Second*30,
		cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc},
		internalinterfaces.TweakListOptionsFunc(func(options *metav1.ListOptions) {
			options.LabelSelector = "skupper.io/external-bridge"
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
		externalBridges:    externalBridges,
		events:             events,
		ports:              newFreePorts(),
		disableServiceSync: disableServiceSync,
	}
	AddStaticPolicyWatcher(controller.policy)

	siteCreationTime := uint64(time.Now().UnixNano()) / uint64(time.Microsecond)
	configmap, err := kube.GetConfigMap(types.SiteConfigMapName, cli.Namespace, cli.KubeClient)
	if err == nil {
		siteCreationTime = uint64(configmap.ObjectMeta.CreationTimestamp.UnixNano()) / uint64(time.Microsecond)
	}
	siteConfig, _ := controller.vanClient.SiteConfigInspect(context.TODO(), configmap)

	var ttl time.Duration
	var enableSkupperEvents = true
	if siteConfig != nil {
		ttl = siteConfig.Spec.SiteTtl
		enableSkupperEvents = siteConfig.Spec.EnableSkupperEvents
	}

	if enableSkupperEvents {
		controller.eventHandler = kube.NewSkupperEventRecorder(cli.Namespace, cli.KubeClient)
	} else {
		controller.eventHandler = event.NewDefaultEventLogger()
	}

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
	externalBridges.AddEventHandler(controller.newEventHandler("external-bridges", AnnotatedKey, DeploymentResourceVersionTest))
	controller.consoleServer = newConsoleServer(cli, tlsConfig, controller.eventHandler)
	controller.siteQueryServer = newSiteQueryServer(cli, tlsConfig)

	controller.definitionMonitor = newDefinitionMonitor(controller.origin, controller.vanClient, controller.svcDefInformer, controller.svcInformer)
	if enableClaimVerifier() {
		controller.claimVerifier = newClaimVerifier(controller.vanClient)
	}
	controller.tokenHandler = newTokenHandler(controller.vanClient, origin, controller.eventHandler)
	controller.claimHandler = newClaimHandler(controller.vanClient, origin)
	handler := func(changed []types.ServiceInterface, deleted []string, origin string) error {
		return kube.UpdateSkupperServices(changed, deleted, origin, cli.Namespace, cli.KubeClient)
	}

	controller.serviceSync = service_sync.NewServiceSync(origin, ttl, version.Version, qdr.NewConnectionFactory("amqps://"+types.QualifiedServiceName(types.LocalTransportServiceName, cli.Namespace)+":5671", tlsConfig), handler, controller.eventHandler)

	controller.flowController = flow.NewFlowController(origin, siteCreationTime, qdr.NewConnectionFactory("amqps://"+types.QualifiedServiceName(types.LocalTransportServiceName, cli.Namespace)+":5671", tlsConfig))
	ipHandler := func(deleted bool, name string, process *flow.ProcessRecord) error {
		return flow.UpdateProcess(controller.flowController, deleted, name, process)
	}
	controller.ipLookup = NewIpLookup(controller.vanClient, ipHandler)
	controller.policyHandler = NewPolicyController(controller.vanClient, controller.eventHandler)
	nwHandler := func(deleted bool, name string, host *flow.HostRecord) error {
		return flow.UpdateHost(controller.flowController, deleted, name, host)
	}
	controller.nodeWatcher = NewNodeWatcher(controller.vanClient, nwHandler)
	controller.tlsManager = &kubeqdr.TlsManager{KubeClient: controller.vanClient.KubeClient, Namespace: controller.vanClient.Namespace}
	return controller, nil
}

type ResourceVersionTest func(a interface{}, b interface{}) bool

func ConfigMapResourceVersionTest(a interface{}, b interface{}) bool {
	aa := a.(*corev1.ConfigMap)
	bb := b.(*corev1.ConfigMap)
	return aa.ResourceVersion == bb.ResourceVersion
}

func NodeResourceVersionTest(a interface{}, b interface{}) bool {
	aa := a.(*corev1.Node)
	bb := b.(*corev1.Node)
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
	go c.externalBridges.Run(stopCh)

	defer utilruntime.HandleCrash()
	defer c.events.ShutDown()

	log.Println("Starting the Skupper controller")

	log.Println("Waiting for informer caches to sync")
	if ok := cache.WaitForCacheSync(stopCh, c.svcDefInformer.HasSynced, c.bridgeDefInformer.HasSynced, c.svcInformer.HasSynced, c.headlessInformer.HasSynced, c.externalBridges.HasSynced); !ok {
		return fmt.Errorf("Failed to wait for caches to sync")
	}

	log.Println("Starting workers")
	if !c.disableServiceSync {
		c.serviceSync.Start(stopCh)
	}
	c.flowController.Start(stopCh)
	go wait.Until(c.runServiceCtrl, time.Second, stopCh)
	c.definitionMonitor.start(stopCh)
	c.siteQueryServer.start(stopCh)
	c.ipLookup.start(stopCh)
	if c.nodeWatcher != nil {
		c.nodeWatcher.start(stopCh)
	}
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

func (c *Controller) DeleteService(svc *corev1.Service) error {
	message := fmt.Sprintf("Deleting service %s", svc.ObjectMeta.Name)
	c.eventHandler.RecordNormalEvent(ServiceControllerDeleteEvent, message)

	return c.vanClient.KubeClient.CoreV1().Services(c.vanClient.Namespace).Delete(context.TODO(), svc.ObjectMeta.Name, metav1.DeleteOptions{})
}

func (c *Controller) UpdateService(svc *corev1.Service) error {
	_, err := c.vanClient.KubeClient.CoreV1().Services(c.vanClient.Namespace).Update(context.TODO(), svc, metav1.UpdateOptions{})
	return err
}

func (c *Controller) CreateService(svc *corev1.Service) error {
	setOwnerReferences(&svc.ObjectMeta)
	_, err := c.vanClient.KubeClient.CoreV1().Services(c.vanClient.Namespace).Create(context.TODO(), svc, metav1.CreateOptions{})
	reason := ServiceControllerCreateEvent

	if err == nil {
		message := fmt.Sprintf("Creating service %s", svc.ObjectMeta.Name)
		c.eventHandler.RecordNormalEvent(reason, message)
	} else {
		message := fmt.Sprintf("Error trying to create service %s: %s", svc.ObjectMeta.Name, err)
		c.eventHandler.RecordWarningEvent(reason, message)
	}

	return err
}

func (c *Controller) IsOwned(service *corev1.Service) bool {
	return isOwned(service)
}

func (c *Controller) GetService(name string) (*corev1.Service, bool, error) {
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

func (c *Controller) updateExternalBridges() {
	for key, binding := range c.bindings {
		if binding.RequiresExternalBridge() {
			err := binding.RealiseExternalBridge()
			if err != nil {
				event.Recordf(ServiceControllerError, "Error realising external bridge for %s: %s", key, err)
			}
		}
	}
	stale := []string{}
	for _, obj := range c.externalBridges.GetStore().List() {
		if bridge, ok := obj.(*appsv1.Deployment); ok {
			if address, ok := bridge.ObjectMeta.Labels["skupper.io/external-bridge"]; ok {
				if binding, ok := c.bindings[address]; ok && binding.RequiresExternalBridge() {
					continue
				}
			}
			// couldn't find a matching binding, assume stale
			stale = append(stale, bridge.ObjectMeta.Name)
		}
	}
	//delete all stale deployments:
	for _, name := range stale {
		err := kube.DeleteDeployment(name, c.vanClient.Namespace, c.vanClient.KubeClient)
		if err != nil {
			event.Recordf(ServiceControllerError, "Error deleting stale external bridge %s: %s", name, err)
		}
	}
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
			c.DeleteService(svc)
		}
	}
}

func (c *Controller) getBindingsForService(svc *corev1.Service) (*service.ServiceBindings, bool) {
	owned := isOwned(svc)
	if owned && svc.Spec.ClusterIP == "None" {
		if svcName := svc.ObjectMeta.Annotations[types.ServiceQualifier]; svcName != "" {
			bindings := c.bindings[svcName]
			if bindings == nil || !bindings.IsHeadless() {
				c.DeleteService(svc)
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

func getOwnerRefs() []metav1.OwnerReference {
	owner := getOwnerReference()
	if owner != nil {
		return []metav1.OwnerReference{*owner}
	}
	return nil
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
		//check credentials before creating bridges
		errWithProfiles := kubeqdr.CheckBindingSecrets(c.bindings, c.vanClient.Namespace, c.vanClient.KubeClient)
		if errWithProfiles != nil {
			return fmt.Errorf("error checking SSL profiles before adding the bindings: %s", errWithProfiles)
		}
		desiredBridges := service.RequiredBridges(c.bindings, c.origin)
		update, err := desiredBridges.UpdateConfigMap(cm)
		if err != nil {
			return fmt.Errorf("Error updating %s: %s", cm.ObjectMeta.Name, err)
		}
		if update {
			event.Recordf(ServiceControllerUpdateEvent, "Updating %s", cm.ObjectMeta.Name)
			_, err = c.vanClient.KubeClient.CoreV1().ConfigMaps(c.vanClient.Namespace).Update(context.TODO(), cm, metav1.UpdateOptions{})
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
	c.ports.inuse(9191) // currently hardcoded in config
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
	return c.vanClient.KubeClient.AppsV1().StatefulSets(c.vanClient.Namespace).Delete(context.TODO(), statefulset.ObjectMeta.Name, metav1.DeleteOptions{})
}

func (c *Controller) ensureHeadlessProxyFor(bindings *service.ServiceBindings, statefulset *appsv1.StatefulSet) error {
	serviceInterface := bindings.AsServiceInterface()
	config, err := qdr.GetRouterConfigForHeadlessProxy(serviceInterface, c.origin, version.Version, c.vanClient.Namespace)
	if err != nil {
		return err
	}

	_, err = kube.CheckProxyStatefulSet(images.GetRouterImageDetails(), serviceInterface, statefulset, config, c.vanClient.Namespace, c.vanClient.KubeClient)
	if err != nil {
		event.Recordf(ServiceControllerError, "Error creating new proxy stateful set: %s", err)
	}
	return err
}

func (c *Controller) createHeadlessProxyFor(bindings *service.ServiceBindings) error {
	serviceInterface := bindings.AsServiceInterface()
	config, err := qdr.GetRouterConfigForHeadlessProxy(serviceInterface, c.origin, version.Version, c.vanClient.Namespace)
	if err != nil {
		return err
	}

	_, err = kube.NewProxyStatefulSet(images.GetRouterImageDetails(), serviceInterface, config, c.vanClient.Namespace, c.vanClient.KubeClient)
	if err != nil {
		event.Recordf(ServiceControllerError, "Error creating new proxy stateful set: %s", err)
	}
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

								tlsSupport := kubeqdr.TlsServiceSupport{Address: si.Address, Credentials: si.TlsCredentials}
								err = c.tlsManager.EnableTlsSupport(tlsSupport)
								if err != nil {
									event.Recordf(ServiceControllerError, "Could not parse service definition for %s: %s", k, err)
								}
							} else {
								event.Recordf(ServiceControllerError, "Could not parse service definition for %s: %s", k, err)
							}
						}
						for k, v := range c.bindings {
							_, ok := cm.Data[k]
							if !ok {
								c.deleteServiceBindings(k, v)
								serviceList, err := c.vanClient.ServiceInterfaceList(context.TODO())
								tlsSupport := kubeqdr.TlsServiceSupport{Address: v.Address, Credentials: v.TlsCredentials}
								err = c.tlsManager.DisableTlsSupport(tlsSupport, serviceList)
								if err != nil {
									event.Recordf(ServiceControllerError, "Disabling TLS support for Skupper credentials has failed: %s", err)
								}
							}
						}
					} else if len(c.bindings) > 0 {
						for k, v := range c.bindings {
							c.deleteServiceBindings(k, v)
							serviceList, err := c.vanClient.ServiceInterfaceList(context.TODO())
							tlsSupport := kubeqdr.TlsServiceSupport{
								Address:     v.Address,
								Credentials: v.TlsCredentials,
							}
							err = c.tlsManager.DisableTlsSupport(tlsSupport, serviceList)
							if err != nil {
								event.Recordf(ServiceControllerError, "Disabling TLS support for Skupper credentials has failed: %s", err)
							}
						}
					}
				}
				c.updateBridgeConfig(c.namespaced(types.TransportConfigMapName))
				c.updateActualServices()
				c.updateHeadlessProxies()
				c.updateExternalBridges()
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
						err = c.DeleteService(svc)
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
			case "external-bridges":
				//Nothing to do
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

func (c *Controller) NewTargetResolver(address string, selector string, skipTargetStatus bool, namespace string) (service.TargetResolver, error) {
	resolver := kube.NewPodTargetResolver(c.vanClient.KubeClient, utils.GetOrDefault(namespace, c.vanClient.GetNamespace()), address, selector, skipTargetStatus)
	resolver.AddEventHandler(c.newEventHandler("targetpods@"+address, FixedKey, PodResourceVersionTest))
	err := resolver.Start()
	return resolver, err
}

func (c *Controller) NewServiceIngress(def *types.ServiceInterface) service.ServiceIngress {
	if def.RequiresExternalBridge() {
		return kube.NewServiceIngressExternalBridge(c, def.Address)
	}
	if def.Headless != nil {
		return kube.NewHeadlessServiceIngress(c, def.Origin)
	}
	if def.ExposeIngress == types.ServiceIngressModeNever {
		return kube.NewServiceIngressNever(c, isOwned)
	} else {
		return kube.NewServiceIngressAlways(c)
	}
}

func (c *Controller) NewExternalBridge(def *types.ServiceInterface) service.ExternalBridge {
	return kube.NewExternalBridge(kube.NewDeployments(c.vanClient.KubeClient, c.vanClient.Namespace, c.externalBridges, getOwnerRefs()), def)
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
		if required.RequiresIngressPortAllocations() {
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
		} else {
			ports = required.Ports
		}

		c.bindings[required.Address] = service.NewServiceBindings(required, ports, c)
	} else {
		if !res.Allowed() {
			event.Recordf(ServiceControllerError, "Policy validation error: service %s has been removed", required.Address)
			delete(c.bindings, required.Address)
			return nil
		}
		ports := bindings.GetIngressPorts()
		if len(ports) < len(required.Ports) {
			for i := 0; i < len(required.Ports); i++ {
				port, err := c.ports.nextFreePort()
				if err != nil {
					return err
				}
				ports = append(ports, port)
			}
			bindings.SetIngressPorts(ports)
		} else if len(ports) > len(required.Ports) {
			// in case updated service exposes less ports than before
			for i := len(required.Ports); i < len(ports); i++ {
				c.ports.release(ports[i])
			}
			ports = ports[:len(required.Ports)]
			bindings.SetIngressPorts(ports)
		}

		// check it is configured correctly
		bindings.Update(required, c)
	}
	return nil
}

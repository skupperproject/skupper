package main

import (
	"crypto/tls"
	jsonencoding "encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apimachinerytypes "k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	corev1informer "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/informers/internalinterfaces"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"

	amqp "github.com/interconnectedcloud/go-amqp"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/client"
	"github.com/skupperproject/skupper/pkg/kube"
)

type Controller struct {
	origin            string
	vanClient         *client.VanClient
	tlsConfig         *tls.Config
	bridgeDefInformer cache.SharedIndexInformer
	svcDefInformer    cache.SharedIndexInformer
	svcInformer       cache.SharedIndexInformer
	events            workqueue.RateLimitingInterface
	bindings          map[string]*ServiceBindings
	ports             *FreePorts
	amqpClient        *amqp.Client
	amqpSession       *amqp.Session
	byOrigin          map[string]map[string]types.ServiceInterface
	Local             []types.ServiceInterface
	byName            map[string]types.ServiceInterface
	desiredServices   map[string]types.ServiceInterface
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

func hasOriginalSelector(service corev1.Service) bool {
	if _, ok := service.ObjectMeta.Annotations[types.OriginalSelectorQualifier]; ok {
		return true
	} else {
		return false
	}
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

	events := workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "skupper-service-controller")

	controller := &Controller{
		vanClient:         cli,
		origin:            origin,
		tlsConfig:         tlsConfig,
		bridgeDefInformer: bridgeDefInformer,
		svcDefInformer:    svcDefInformer,
		svcInformer:       svcInformer,
		events:            events,
		ports:             newFreePorts(),
	}

	// Organize service definitions
	controller.byOrigin = make(map[string]map[string]types.ServiceInterface)
	controller.byName = make(map[string]types.ServiceInterface)
	controller.desiredServices = make(map[string]types.ServiceInterface)

	log.Println("Setting up event handlers")
	svcDefInformer.AddEventHandler(controller.newEventHandler("servicedefs", AnnotatedKey, ConfigMapResourceVersionTest))
	bridgeDefInformer.AddEventHandler(controller.newEventHandler("bridges", AnnotatedKey, ConfigMapResourceVersionTest))
	svcInformer.AddEventHandler(controller.newEventHandler("actual-services", AnnotatedKey, ServiceResourceVersionTest))

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

type CacheKeyStrategy func(category string, object interface{}) (string, error)

func AnnotatedKey(category string, obj interface{}) (string, error) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		return "", err
	}
	return category + "@" + key, nil
}

func FixedKey(category string, obj interface{}) (string, error) {
	return category, nil
}

func splitKey(key string) (string, string) {
	parts := strings.Split(key, "@")
	return parts[0], parts[1]
}

func (c *Controller) newEventHandler(category string, keyStrategy CacheKeyStrategy, test ResourceVersionTest) *cache.ResourceEventHandlerFuncs {
	return &cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			key, err := keyStrategy(category, obj)
			if err != nil {
				utilruntime.HandleError(err)
			} else {
				c.events.Add(key)
			}
		},
		UpdateFunc: func(old, new interface{}) {
			if !test(old, new) {
				key, err := keyStrategy(category, new)
				if err != nil {
					utilruntime.HandleError(err)
				} else {
					c.events.Add(key)
				}
			}
		},
		DeleteFunc: func(obj interface{}) {
			key, err := keyStrategy(category, obj)
			if err != nil {
				utilruntime.HandleError(err)
			} else {
				c.events.Add(key)
			}
		},
	}
}

func (c *Controller) Run(stopCh <-chan struct{}) error {
	// fire up the informers
	go c.svcDefInformer.Run(stopCh)
	go c.bridgeDefInformer.Run(stopCh)
	go c.svcInformer.Run(stopCh)

	defer utilruntime.HandleCrash()
	defer c.events.ShutDown()

	log.Println("Starting the Skupper controller")

	log.Println("Waiting for informer caches to sync")
	if ok := cache.WaitForCacheSync(stopCh, c.svcDefInformer.HasSynced, c.bridgeDefInformer.HasSynced, c.svcInformer.HasSynced); !ok {
		return fmt.Errorf("Failed to wait for caches to sync")
	}

	log.Println("Starting workers")
	go wait.Until(c.runServiceSync, time.Second, stopCh)
	go wait.Until(c.runServiceCtrl, time.Second, stopCh)

	log.Println("Started workers")
	<-stopCh
	log.Println("Shutting down workers")

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

func (c *Controller) checkServiceFor(desired *ServiceBindings, actual *corev1.Service) error {
	//selector, port, targetPort
	// TODO: check services changes
	log.Printf("We need to check service changes for %s", actual.ObjectMeta.Name)
	return nil
}

func (c *Controller) ensureServiceFor(desired *ServiceBindings) error {
	log.Println("Checking service for: ", desired.address)
	obj, exists, err := c.svcInformer.GetStore().GetByKey(c.namespaced(desired.address))
	if err != nil {
		return fmt.Errorf("Error checking service %s", err)
	} else if !exists {
		return c.createServiceFor(desired)
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

const (
	BRIDGE_CONFIG = "bridges.json"
)

func (c *Controller) getRequiredBridgeConfig() (string, error) {
	bridges := requiredBridges(c.bindings, c.origin)
	config, err := writeBridgeConfiguration(bridges)
	if err != nil {
		return "", fmt.Errorf("Error writing json for bridge config %s", err)
	} else {
		return string(config), nil
	}
}

func (c *Controller) getRequiredBridgeConfigAsMap() (map[string]string, error) {
	val, err := c.getRequiredBridgeConfig()
	if err != nil {
		return nil, err
	} else {
		return map[string]string{
			BRIDGE_CONFIG: val,
		}, nil
	}
}

func (c *Controller) getInitialBridgeConfig() (*BridgeConfiguration, error) {
	name := c.namespaced("skupper-internal")
	obj, exists, err := c.bridgeDefInformer.GetStore().GetByKey(name)
	if err != nil {
		return nil, fmt.Errorf("Error reading skupper-internal from cache: %s", err)
	} else if exists {
		cm, ok := obj.(*corev1.ConfigMap)
		if !ok {
			return nil, fmt.Errorf("Expected ConfigMap for %s but got %#v", name, obj)
		}
		if cm.Data == nil || cm.Data[BRIDGE_CONFIG] == "" {
			return nil, nil
		} else {
			log.Printf("Reading initial bridge configuration: %s", cm.Data[BRIDGE_CONFIG])
			currentBridges, err := readBridgeConfiguration([]byte(cm.Data[BRIDGE_CONFIG]))
			if err != nil {
				return nil, fmt.Errorf("Error reading bridge config from %s: %v", name, err.Error())
			}
			return currentBridges, nil
		}
	} else {
		return nil, nil
	}
}

func (c *Controller) updateBridgeConfig(name string) error {
	obj, exists, err := c.bridgeDefInformer.GetStore().GetByKey(name)
	if err != nil {
		return fmt.Errorf("Error reading skupper-internal from cache: %s", err)
	} else if exists {
		cm, ok := obj.(*corev1.ConfigMap)
		if !ok {
			return fmt.Errorf("Expected ConfigMap for %s but got %#v", name, obj)
		}
		var update bool
		if cm.Data == nil {
			cm.Data, err = c.getRequiredBridgeConfigAsMap()
			if err != nil {
				return fmt.Errorf("Error building required bridge config: %v", err.Error())
			}
			update = true
		} else if cm.Data[BRIDGE_CONFIG] == "" {
			cm.Data[BRIDGE_CONFIG], err = c.getRequiredBridgeConfig()
			if err != nil {
				return fmt.Errorf("Error building required bridge config: %v", err.Error())
			}
			update = true
		} else {
			desiredBridges := requiredBridges(c.bindings, c.origin)
			currentBridges, err := readBridgeConfiguration([]byte(cm.Data[BRIDGE_CONFIG]))
			if err != nil {
				return fmt.Errorf("Error reading bridge config from %s: %v", name, err.Error())
			}
			if updateBridgeConfiguration(desiredBridges, currentBridges) {
				update = true
				config, err := writeBridgeConfiguration(desiredBridges)
				if err != nil {
					return fmt.Errorf("Error writing json for bridge config %s", err)
				}
				cm.Data[BRIDGE_CONFIG] = string(config)
			}
		}
		if update {
			log.Printf("Updating %s", cm.ObjectMeta.Name)
			_, err = c.vanClient.KubeClient.CoreV1().ConfigMaps(c.vanClient.Namespace).Update(cm)
			if err != nil {
				return fmt.Errorf("Failed to update %s: %v", name, err.Error())
			}
		}
	} else {
		data, err := c.getRequiredBridgeConfigAsMap()
		if err != nil {
			return fmt.Errorf("Error building required bridge config: %v", err.Error())
		}
		_, err = kube.NewConfigMap("skupper-internal" /*TODO define constant*/, &data, getOwnerReference(), c.vanClient.Namespace, c.vanClient.KubeClient)
		if err != nil {
			return err
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

func (c *Controller) updateServiceSync(defs *corev1.ConfigMap) {
	c.serviceSyncDefinitionsUpdated(c.parseServiceDefinitions(defs))
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
								if v != nil {
									v.stop()
								}
								delete(c.bindings, k)
							}
						}
					} else if len(c.bindings) > 0 {
						for k, v := range c.bindings {
							if v != nil {
								v.stop()
							}
							delete(c.bindings, k)
						}
					}
				}
				c.updateBridgeConfig(c.namespaced("skupper-internal"))
				c.updateActualServices()
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
					bindings := c.bindings[name]
					if bindings != nil {
						err = c.createServiceFor(bindings)
						if err != nil {
							return err
						}
					}
				}
			case "targetpods":
				log.Printf("Got targetpods event %s", name)
				//name is the address of the skupper service
				c.updateBridgeConfig(c.namespaced("skupper-internal"))
			default:
				c.events.Forget(obj)
				return fmt.Errorf("unexpected event key %s (%s, %s)", key, category, name)
			}
			c.events.Forget(obj)
		}
		return nil
	}(obj)

	if err != nil {
		utilruntime.HandleError(err)
		return true
	}

	return true
}

package main

import (
	"crypto/tls"
    "fmt"
	jsonencoding "encoding/json"    
    "log"
    "os"
    "strings"
    "time"

   	appsv1 "k8s.io/api/apps/v1"
    corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"    
    appsv1informer "k8s.io/client-go/informers/apps/v1"
    corev1informer "k8s.io/client-go/informers/core/v1"
    "k8s.io/client-go/informers/internalinterfaces"    
    utilruntime "k8s.io/apimachinery/pkg/util/runtime"
    apimachinerytypes "k8s.io/apimachinery/pkg/types"
    "k8s.io/apimachinery/pkg/util/wait"
 	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"

    amqp "github.com/Azure/go-amqp"
    
	"github.com/ajssmith/skupper/api/types"
	"github.com/ajssmith/skupper/client"
	"github.com/ajssmith/skupper/pkg/kube"
)

type Controller struct {
    origin          string
    vanClient       *client.VanClient
    tlsConfig       *tls.Config
    depInformer     cache.SharedIndexInformer
    cmInformer      cache.SharedIndexInformer
    svcInformer     cache.SharedIndexInformer
    cmWorkqueue     workqueue.RateLimitingInterface
    depWorkqueue    workqueue.RateLimitingInterface
    svcWorkqueue    workqueue.RateLimitingInterface    
    amqpClient      *amqp.Client
    amqpSession     *amqp.Session
    byOrigin        map[string]map[string]types.ServiceInterface    
    Local           []types.ServiceInterface
    byName          map[string]types.ServiceInterface
    desiredServices map[string]types.ServiceInterface
    actualServices  map[string]corev1.Service
    proxies         map[string]appsv1.Deployment
    ssProxies       map[string]*appsv1.StatefulSet
}

func hasProxyAnnotation(service corev1.Service) bool {
    if _, ok := service.ObjectMeta.Annotations[types.ProxyQualifier]; ok {
        return true
    } else {
        return false
    }
}

func getProxyName (name string) string {
    return name + "-proxy"
}

func getServiceName (name string) string {
    return strings.TrimSuffix(name, "-proxy")
}

func hasOriginalSelector(service corev1.Service) bool {
    if _, ok := service.ObjectMeta.Annotations[types.OriginalSelectorQualifier]; ok {
        return true
    } else {
        return false
    }
}

func equivalentProxyConfig(desired types.ServiceInterface, deployment appsv1.Deployment) bool {
    envVar := kube.FindEnvVar(deployment.Spec.Template.Spec.Containers[0].Env, "SKUPPER_PROXY_CONFIG")
    encodedDesired, _ := jsonencoding.Marshal(desired)
    return string(encodedDesired) == envVar.Value
}

func (c *Controller) printAllKeys() {
    depKeys := []string{}
    proxyKeys := []string{}
    svcKeys := []string{}

    for key, _ := range c.proxies {
        proxyKeys = append(proxyKeys, key)
    }
    for key, _ := range c.desiredServices {
        depKeys = append(depKeys, key)
    }
    for key, _ := range c.actualServices {
        svcKeys = append(svcKeys, key)
    }

    log.Println("Desired services: ", depKeys)
    log.Println("Proxies: ", proxyKeys)
    log.Println("Actual Services: ", svcKeys)

}

func NewController(cli *client.VanClient,origin string, tlsConfig *tls.Config) (*Controller, error) {

    // create informers
    depInformer:= appsv1informer.NewFilteredDeploymentInformer(
        cli.KubeClient,
        cli.Namespace,
        time.Second*30,
        cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc},
        internalinterfaces.TweakListOptionsFunc(func(options *metav1.ListOptions) {
            options.LabelSelector = types.TypeProxyQualifier
        }))
    svcInformer := corev1informer.NewServiceInformer(
        cli.KubeClient,
        cli.Namespace,
        time.Second*30,
        cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
    cmInformer := corev1informer.NewFilteredConfigMapInformer(
        cli.KubeClient,
        cli.Namespace,
        time.Second*30,
        cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc},
        internalinterfaces.TweakListOptionsFunc(func(options *metav1.ListOptions) {
            options.FieldSelector = "metadata.name=skupper-services"
        }))
    
    // create a workqueue per informer
    cmWorkqueue := workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "skupper-controller-cm")
    depWorkqueue := workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "skupper-controller-dep")
    svcWorkqueue := workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "skupper-controller-svc")
       
    controller := &Controller{
        vanClient:    cli,
        origin:       origin,
        tlsConfig:    tlsConfig,
        depInformer:  depInformer,
        cmInformer:   cmInformer,
        svcInformer:  svcInformer,
        cmWorkqueue:  cmWorkqueue,
        depWorkqueue: depWorkqueue,
        svcWorkqueue: svcWorkqueue,
    }
    
    // Organize service definitions
    controller.byOrigin = make(map[string]map[string]types.ServiceInterface)
    controller.byName = make(map[string]types.ServiceInterface)
    controller.desiredServices = make(map[string]types.ServiceInterface)
    controller.actualServices = make(map[string]corev1.Service)
    controller.proxies = make(map[string]appsv1.Deployment)
    controller.ssProxies = make(map[string]*appsv1.StatefulSet)
    
    log.Println("Setting up event handlers")
    cmInformer.AddEventHandler(&cache.ResourceEventHandlerFuncs{
        AddFunc: controller.enqueueConfigMap,
        UpdateFunc: func(old, new interface{}) {
            newCm := new.(*corev1.ConfigMap)
            oldCm := old.(*corev1.ConfigMap)
            if newCm.ResourceVersion == oldCm.ResourceVersion {
                return
            }
            controller.enqueueConfigMap(new)
        },
    })

    depInformer.AddEventHandler(&cache.ResourceEventHandlerFuncs{
        AddFunc: controller.enqueueDeployment,
        UpdateFunc: func(old, new interface{}) {
            newDep := new.(*appsv1.Deployment)
            oldDep := old.(*appsv1.Deployment)
            if newDep.ResourceVersion == oldDep.ResourceVersion {
                return
            }
            controller.enqueueDeployment(new)
        },
        DeleteFunc: controller.enqueueDeployment,
    })

    svcInformer.AddEventHandler(&cache.ResourceEventHandlerFuncs{
        AddFunc: controller.enqueueService,
        UpdateFunc: func(old, new interface{}) {
            newSvc := new.(*corev1.Service)
            oldSvc := old.(*corev1.Service)
            if newSvc.ResourceVersion == oldSvc.ResourceVersion {
                return
            }
            controller.enqueueService(new)
        },
        DeleteFunc: controller.enqueueService,        
    })

    return controller, nil
}

func (c *Controller) Run (stopCh <-chan struct{}) error {
    defer utilruntime.HandleCrash()
    defer c.cmWorkqueue.ShutDown()
    defer c.depWorkqueue.ShutDown()
    defer c.svcWorkqueue.ShutDown()

    log.Println("Starting the Skupper controller")
    
    log.Println("Waiting for informer caches to sync")
    if ok := cache.WaitForCacheSync(stopCh, c.cmInformer.HasSynced, c.depInformer.HasSynced, c.svcInformer.HasSynced); !ok {
        return fmt.Errorf("Failed to wait for caches to sync")
    }

    log.Println("Starting workers")
    go wait.Until(c.runServiceSync, time.Second, stopCh)
    go wait.Until(c.runConfigMapWorker, time.Second, stopCh)
    go wait.Until(c.runDeploymentWorker, time.Second, stopCh)
    go wait.Until(c.runServiceWorker, time.Second, stopCh)

    log.Println("Started workers")
    <-stopCh
    log.Println("Shutting down workers")
    
    return nil
}

func (c *Controller) ensureProxyDeployment(name string){
    proxyName := getProxyName(name)
    proxy, proxyDefined := c.proxies[proxyName]
    serviceInterface := c.desiredServices[name]
    
    if serviceInterface.Headless != nil {
        log.Println("TODO: Proxy is for a stateful set")
    } else {
        if !proxyDefined {
            log.Printf("Need to create proxy for %s (%s)\n", serviceInterface.Address, proxyName)
            proxyDep, err := kube.NewProxyDeployment(serviceInterface, c.vanClient.Namespace, c.vanClient.KubeClient)
            if err == nil {
                c.proxies[proxyName] = *proxyDep
            }
        } else {
            if !equivalentProxyConfig(serviceInterface, proxy) {
                log.Println("TODO: Need to update proxy config for ", proxy.Name)
             } else {
                log.Println("TODO: Nothing to do here for proxy config", proxy.Name)
             }
        }
    }
}

func (c *Controller) ensureServiceFor(name string) {
    log.Println("Checking service for: ", name)
    var ok bool
    desired, ok := c.desiredServices[name]
    if !ok {
        log.Println("Unable to retrieve desired service")
        return
    }
    if desired.Headless != nil {
        // TODO: setup headless
        log.Println("We have a headless service to set up")
    } else {
        if _, ok := c.actualServices[name]; !ok {
            log.Println("Creating new service for proxy", name)
            kube.NewServiceForProxy(desired, c.vanClient.Namespace, c.vanClient.KubeClient)
        } else {
            // TODO: check services changes
            log.Println("We need to check service changes")
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

func isOwned(service corev1.Service) bool {
    ownerName := os.Getenv("OWNER_NAME")
    ownerUid := os.Getenv("OWNER_UID")
    if ownerName == "" || ownerUid == "" {
        return false
    }

    ownerRefs := []metav1.OwnerReference {
        {
            APIVersion: "apps/v1",
            Kind:       "Deployment",
            Name:       ownerName,
            UID:        apimachinerytypes.UID(ownerUid),
        },
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

func (c *Controller) reconcile() error {
    log.Println("Reconciling...")
    
    // reconcile proxy deployments with desired services:
    for name, _ := range c.desiredServices {
        c.ensureProxyDeployment(name)
    }
    for proxyname, _ := range c.proxies {
        if _, ok := c.desiredServices[getServiceName(proxyname)]; !ok {
            log.Println("Undeploying proxy: ", proxyname)
            kube.DeleteDeployment(proxyname, c.vanClient.Namespace, c.vanClient.KubeClient)
        }
    }

    // reconcile actual services with desired services:
    for name, _ := range c.desiredServices {
        c.ensureServiceFor(name)
    }
    
    for name, svc := range c.actualServices {
        if _, ok := c.desiredServices[name]; !ok {
            if isOwned(svc) {
                log.Println("Deleting service: ", name)
                kube.DeleteService(name, c.vanClient.Namespace, c.vanClient.KubeClient)
            }
        }
    }
    
    return nil
}

func (c *Controller) runConfigMapWorker() {
    for c.processNextConfigMapWorkItem() {
    }
}

func (c *Controller) processNextConfigMapWorkItem() bool {
    obj, shutdown := c.cmWorkqueue.Get()

    if shutdown {
        return false
    }

    err := func(obj interface{}) error {
        defer c.cmWorkqueue.Done(obj)

        var key string
        var ok bool
        
        if key, ok = obj.(string); !ok {
            // invalid item
            c.cmWorkqueue.Forget(obj)
            utilruntime.HandleError(fmt.Errorf("expected string in cm workqueue but got %#v",obj))
            return nil
        }
        namespace, name, err := cache.SplitMetaNamespaceKey(key)
        if err != nil {
            utilruntime.HandleError(fmt.Errorf("invalid resource key: %s",key))
            return nil
        }

        // TODO: is this ok or get from informer store?
        // also, be able to use common pkg file kube.GetConfigMap
        cm, err := c.vanClient.KubeClient.CoreV1().ConfigMaps(namespace).Get(name, metav1.GetOptions{})
        if err == nil {
            definitions := make(map[string]types.ServiceInterface)
            if len(cm.Data) > 0 {
                for _, v := range cm.Data {
                    si := types.ServiceInterface {}
                    err = jsonencoding.Unmarshal([]byte(v), &si)
                    if err == nil {
                        definitions[si.Address] = si
                    }
                }
                c.desiredServices = definitions
                keys := []string{}
                for key, _ := range c.desiredServices {
                    keys = append(keys, key)
                }
                log.Println("Desired service configuration updated: ", keys)
                c.reconcile()
            } else {
                c.desiredServices = definitions            
                log.Println("No skupper services defined.")
                c.reconcile()
            }
            c.serviceSyncDefinitionsUpdated(definitions)
        }
        c.cmWorkqueue.Forget(obj)
        return nil
    }(obj)

    if err != nil {
        utilruntime.HandleError(err)
        return true
    }

    return true
}

// enqueueConfigMap takes a ConfigMap resource and converts it into a namespace/name
// string which is then put onto the work queue. This method should *not* be
// passed resources of any type other than ConfigMap.
func (c *Controller) enqueueConfigMap(obj interface{}) {
	var key string
	var err error
	if key, err = cache.MetaNamespaceKeyFunc(obj); err != nil {
		utilruntime.HandleError(err)
		return
	}
	c.cmWorkqueue.Add(key)
}

func (c *Controller) runDeploymentWorker() {
    for c.processNextDeploymentWorkItem() {
    }
}

func (c *Controller) processNextDeploymentWorkItem() bool {

    obj, shutdown := c.depWorkqueue.Get()

    if shutdown {
        return false
    }

    err := func(obj interface{}) error {
        defer c.depWorkqueue.Done(obj)

        var ok bool
        if _, ok = obj.(string); !ok {
            // invalid item
            c.depWorkqueue.Forget(obj)
            utilruntime.HandleError(fmt.Errorf("expected string in dep workqueue but got %#v",obj))
        } else {
            // TODO: get list from informer??
            deps, err := c.vanClient.KubeClient.AppsV1().Deployments(c.vanClient.Namespace).List(metav1.ListOptions{LabelSelector: types.TypeProxyQualifier})
            if err != nil {
                return err
            } else {
                proxies := make(map[string]appsv1.Deployment)
                for _, dep := range deps.Items {
                    proxies[dep.ObjectMeta.Name] = dep
                }
                c.proxies = proxies
                keys := []string{}
                for key, _ := range c.proxies {
                    keys = append(keys, key)
                }
                log.Println("proxy deployments updated: ", keys)
                c.reconcile()
            }
            c.depWorkqueue.Forget(obj)
        }        
        return nil
    }(obj)

    if err != nil {
        utilruntime.HandleError(err)
        return true
    }

    return true
}

// enqueueDeployment takes a Deployment resource and converts it into a namespace/name
// string which is then put onto the work queue. This method should *not* be
// passed resources of any type other than Deployment.
func (c *Controller) enqueueDeployment(obj interface{}) {
	var key string
	var err error
	if key, err = cache.MetaNamespaceKeyFunc(obj); err != nil {
		utilruntime.HandleError(err)
		return
	}
	c.depWorkqueue.Add(key)
}

func (c *Controller) runServiceWorker() {
    for c.processNextServiceWorkItem() {
    }
}

func (c *Controller) processNextServiceWorkItem() bool {

    obj, shutdown := c.svcWorkqueue.Get()

    if shutdown {
        return false
    }

    err := func(obj interface{}) error {
        defer c.svcWorkqueue.Done(obj)

        var ok bool        
        if _, ok = obj.(string); !ok {
            // invalid item
            c.svcWorkqueue.Forget(obj)
            utilruntime.HandleError(fmt.Errorf("expected string in dep workqueue but got %#v",obj))
        } else {
            // TODO: get list from informer??
            svcs, err := c.vanClient.KubeClient.CoreV1().Services(c.vanClient.Namespace).List(metav1.ListOptions{})
            if err != nil {
                return err
            } else {
                actualServices := make(map[string]corev1.Service)
                for _, svc := range svcs.Items {
                    actualServices[svc.ObjectMeta.Name] = svc
                }
                c.actualServices = actualServices
                keys := []string{}
                for key, _ := range c.actualServices {
                    keys = append(keys, key)
                }
                log.Println("services updated: ", keys)
                c.reconcile()
           }
           c.svcWorkqueue.Forget(obj)
        }
        return nil
    }(obj)

    if err != nil {
        utilruntime.HandleError(err)
        return true
    }

    return true
}

// enqueueService takes a Service resource and converts it into a namespace/name
// string which is then put onto the work queue. This method should *not* be
// passed resources of any type other than Service.
func (c *Controller) enqueueService(obj interface{}) {
	var key string
	var err error
	if key, err = cache.MetaNamespaceKeyFunc(obj); err != nil {
		utilruntime.HandleError(err)
		return
	}
    fmt.Println("Enqueue service")
	c.svcWorkqueue.Add(key)
}

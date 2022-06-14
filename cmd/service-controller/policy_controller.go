package main

import (
	"context"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/client"
	"github.com/skupperproject/skupper/pkg/apis/skupper/v1alpha1"
	"github.com/skupperproject/skupper/pkg/generated/client/clientset/versioned"
	v1alpha12 "github.com/skupperproject/skupper/pkg/generated/client/informers/externalversions/skupper/v1alpha1"
	"github.com/skupperproject/skupper/pkg/kube"
	"github.com/skupperproject/skupper/pkg/qdr"
	"github.com/skupperproject/skupper/pkg/utils"
	"k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"

	"github.com/skupperproject/skupper/pkg/event"
)

var (
	staticPolicyWatchers []*client.ClusterPolicyValidator
)

// AddStaticPolicyWatcher all watchers must be defined before
// the PolicyController is started
func AddStaticPolicyWatcher(pv *client.ClusterPolicyValidator) {
	staticPolicyWatchers = append(staticPolicyWatchers, pv)
}

type PolicyController struct {
	name      string
	cli       *client.VanClient
	validator *client.ClusterPolicyValidator
	informer  cache.SharedIndexInformer
	queue     workqueue.RateLimitingInterface
	activeMap map[string]time.Time
}

func (c *PolicyController) loadActiveMap() {
	c.activeMap = map[string]time.Time{}
	policies, _ := c.validator.LoadNamespacePolicies()
	now := time.Now()
	for _, p := range policies {
		c.activeMap[p.Name] = now
	}
}

func (c *PolicyController) enqueue(obj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err == nil {
		c.queue.Add(key)
	} else {
		event.Recordf(c.name, "Error retrieving key: %s", err)
	}
}

func (c *PolicyController) OnAdd(obj interface{}) {
	c.enqueue(obj)
}

func (c *PolicyController) OnUpdate(a, b interface{}) {
	aa := a.(*v1alpha1.SkupperClusterPolicy)
	bb := b.(*v1alpha1.SkupperClusterPolicy)
	if aa.ResourceVersion != bb.ResourceVersion {
		c.enqueue(b)
	}
}

func (c *PolicyController) OnDelete(obj interface{}) {
	c.enqueue(obj)
}

func (c *PolicyController) start(stopCh <-chan struct{}) error {
	go func() {
		period := time.NewTicker(time.Second)
		var crdCh chan struct{}
		enabled := false
		disabledReported := false
		informerRunning := false
		startInformer := func() bool {
			crdCh = make(chan struct{})
			c.createInformer()
			go c.informer.Run(crdCh)
			if ok := cache.WaitForCacheSync(crdCh, c.informer.HasSynced); !ok {
				event.Recordf(c.name, "Error waiting for cache to sync")
				close(crdCh)
				return false
			}
			go wait.Until(c.run, time.Second, crdCh)
			return true
		}
		for {
			if !enabled && c.validator.Enabled() {
				log.Println("Skupper policy is enabled")
				if !c.validator.HasPermission() {
					log.Printf("-> No permission to read SkupperClusterPolicies")
				} else {
					if informerRunning = startInformer(); !informerRunning {
						continue
					}
				}
				c.loadActiveMap()
				c.validateStateChanged()
				enabled = true
			} else if !c.validator.Enabled() && !disabledReported {
				disabledReported = true
				log.Printf("Skupper policy is disabled")
			}

			select {
			case <-period.C:
				if enabled && !c.validator.Enabled() {
					if informerRunning {
						close(crdCh)
					}
					log.Println("Skupper policy has been disabled")
					// reverts what has been denied by policies
					c.validateStateChanged()
					enabled = false
				} else if enabled && !informerRunning && c.validator.HasPermission() {
					// permission has been granted, running informer
					informerRunning = startInformer()
					log.Println("Permission to read SkupperClusterPolicies has been granted")
				} else if enabled && informerRunning && !c.validator.HasPermission() {
					// permission revoked, stopping informer
					close(crdCh)
					informerRunning = false
					c.validateStateChanged()
					log.Println("Permission to read SkupperClusterPolicies has been revoked")
				}
			case <-stopCh:
				if enabled {
					close(crdCh)
				}
				return
			}
		}
	}()
	return nil
}

func (c *PolicyController) stop() {
	c.queue.ShutDown()
}

func (c *PolicyController) run() {
	for c.process() {
	}
}

func (c *PolicyController) process() bool {
	if !c.validator.Enabled() {
		return true
	}

	obj, shutdown := c.queue.Get()

	if shutdown {
		return false
	}

	defer c.queue.Done(obj)
	if key, ok := obj.(string); ok {
		_, active := c.activeMap[key]
		appliesToNs := c.validator.AppliesToNS(key)
		if appliesToNs || active {
			if !appliesToNs {
				delete(c.activeMap, key)
			} else {
				c.activeMap[key] = time.Now()
			}
			event.Recordf(c.name, "Skupper policy has changed: %s", key)
			c.validateStateChanged()
		}
	} else {
		event.Recordf(c.name, "Expected key to be string, was %#v", key)
	}
	c.queue.Forget(obj)

	return true
}

func (c *PolicyController) validateIncomingLinkStateChanged() {
	res := c.validator.ValidateIncomingLink()
	if res.Error() != nil {
		event.Recordf(c.name, "[validateIncomingLinkStateChanged] error validating policy: %v", res.Error())
		return
	}
	source := "validateIncomingLinkStateChanged"
	allowed := res.Allowed()
	listeners := map[string]func(options types.SiteConfigSpec) qdr.Listener{
		"interior-listener": client.InteriorListener,
		"edge-listener":     client.EdgeListener,
	}

	// Retrieving listener info
	configmap, err := kube.GetConfigMap(types.TransportConfigMapName, c.cli.GetNamespace(), c.cli.KubeClient)
	if err != nil {
		event.Recordf(c.name, "[%s] Unable to read %s ConfigMap: %v", source, types.TransportConfigMapName, err)
		return
	}
	current, err := qdr.GetRouterConfigFromConfigMap(configmap)

	// If mode is edge, no need to proceed
	if current.IsEdge() {
		return
	}

	// Changed to allowed
	if allowed {
		event.Recordf(c.name, "[%s] allowing links", source)
	} else {
		event.Recordf(c.name, "[%s] blocking links", source)
	}

	siteConfig, err := c.cli.SiteConfigInspect(context.Background(), nil)
	if err != nil {
		event.Recordf(c.name, "[%s] error retrieving site config: %v", source, err)
		return
	}

	for listenerName, listenerFn := range listeners {
		// Retrieving listener info
		_, listenerFound := current.Listeners[listenerName]

		// If nothing changed, just return
		if listenerFound == allowed {
			return
		}

		// Changed to allowed
		if allowed {
			current.AddListener(listenerFn(siteConfig.Spec))
		} else {
			delete(current.Listeners, listenerName)
		}
	}

	// Update router config
	updated, err := current.UpdateConfigMap(configmap)
	if err != nil {
		event.Recordf(c.name, "[%s] error updating listeners: %v", source, err)
		return
	}

	if updated {
		_, err = c.cli.KubeClient.CoreV1().ConfigMaps(c.cli.GetNamespace()).Update(configmap)
		if err != nil {
			event.Recordf(c.name, "[%s] error updating %s ConfigMap: %v", source, configmap.Name, err)
			return
		}
		// TODO Once config sync handles listeners this won't be needed
		if err = c.cli.RouterRestart(context.Background(), c.cli.Namespace); err != nil {
			event.Recordf(c.name, "[%s] error restarting router: %v", source, err)
			return
		}
	}
}

func (c *PolicyController) validateOutgoingLinkStateChanged() {
	// Iterate through all links
	links, err := c.cli.ConnectorList(context.Background())
	if err != nil {
		event.Recordf(c.name, "[validateOutgoingLinkStateChanged] error reading existing links: %v", err)
		return
	}
	for _, link := range links {
		// Retrieving state of respective link (enabled/disabled)
		secret, err := c.cli.KubeClient.CoreV1().Secrets(c.cli.GetNamespace()).Get(link.Name, v1.GetOptions{})
		if err != nil {
			event.Recordf(c.name, "[validateOutgoingLinkStateChanged] error reading secret %s: %v", link.Name, err)
			return
		}
		disabledValue, ok := secret.ObjectMeta.Labels[types.SkupperDisabledQualifier]
		disabled := false
		if ok {
			disabled, _ = strconv.ParseBool(disabledValue)
		}
		linkUrl := strings.Split(link.Url, ":")
		hostname := linkUrl[0]

		// Validating if hostname is allowed
		res := c.validator.ValidateOutgoingLink(hostname)
		if res.Error() != nil {
			event.Recordf(c.name, "[validateOutgoingLinkStateChanged] error validating if outgoing link to %s is allowed: %v", hostname, res.Error())
			return
		}

		// Not changed, continue to next link
		if res.Allowed() != disabled {
			continue
		}

		// Rule has changed for the related hostname
		if res.Allowed() {
			event.Recordf(c.name, "[validateOutgoingLinkStateChanged] enabling link %s", link.Name)
			delete(secret.Labels, types.SkupperDisabledQualifier)
		} else {
			event.Recordf(c.name, "[validateOutgoingLinkStateChanged] disabling link %s", link.Name)
			secret.Labels[types.SkupperDisabledQualifier] = "true"
		}

		// Update secret
		_, err = c.cli.KubeClient.CoreV1().Secrets(c.cli.GetNamespace()).Update(secret)
		if err != nil {
			event.Recordf(c.name, "[validateOutgoingLinkStateChanged] error updating secret %s: %v", link.Name, res.Error())
			return
		}
	}
}

func (c *PolicyController) validateExposeStateChanged() {
	policies, err := c.validator.LoadNamespacePolicies()
	if err != nil {
		event.Recordf(c.name, "[validateExposeStateChanged] error retrieving policies: %v", err)
		return
	}

	for _, policy := range policies {
		// If there is a policy allowing all resources, no need to continue
		if utils.StringSliceContains(policy.Spec.AllowedExposedResources, "*") {
			return
		}
	}

	serviceList, err := c.cli.ServiceInterfaceList(context.Background())
	if err != nil {
		event.Recordf(c.name, "[validateExposeStateChanged] error retrieving service list: %v", err)
		return
	}

	// iterate through service list and inspect if respective targets are allowed
	for _, service := range serviceList {
		if service.Targets == nil || len(service.Targets) == 0 {
			continue
		}
		for _, target := range service.Targets {
			targetType := c.inferTargetType(target)
			res := c.validator.ValidateExpose(targetType, target.Name)
			if res.Error() != nil {
				event.Recordf(c.name, "[validateExposeStateChanged] error validating if target can still be exposed: %v", err)
				return
			}
			if !res.Allowed() {
				// resource is no longer allowed, unbinding
				event.Recordf(c.name, "[validateExposeStateChanged] exposed resource is no longer authorized - unbinding service %s: %v", service.Address, err)
				err = c.cli.ServiceInterfaceUnbind(context.Background(), "deployment", target.Name, service.Address, false)
				if err != nil {
					event.Recordf(c.name, "[validateExposeStateChanged] error unbinding service %s: %v", service.Address, err)
					return
				}
			}
		}
	}
}

func (c *PolicyController) validateServiceStateChanged() {
	serviceList, err := c.cli.ServiceInterfaceList(context.Background())
	if err != nil {
		event.Recordf(c.name, "[validateServiceStateChanged] error retrieving service list: %v", err)
		return
	}

	for _, service := range serviceList {
		res := c.validator.ValidateImportService(service.Address)
		if res.Error() != nil {
			event.Recordf(c.name, "[validateServiceStateChanged] error validating service policy: %v", res.Error())
			return
		}
		if !res.Allowed() {
			err = c.cli.ServiceInterfaceRemove(context.Background(), service.Address)
			if err != nil {
				event.Recordf(c.name, "[validateServiceStateChanged] error removing service definition %s: %v", service.Address, err)
				return
			}
		} else {
			// Validating if allowed service exists
			_, err := kube.GetService(service.Address, c.cli.Namespace, c.cli.KubeClient)
			// If service is now allowed, but does not exist, remove its definition to let service sync recreate it
			if len(service.Origin) > 0 && err != nil && errors.IsNotFound(err) {
				event.Recordf(c.name, "[validateServiceStateChanged] service is now allowed %s", service.Address)
				c.cli.ServiceInterfaceRemove(context.Background(), service.Address)
			}
		}
	}
}

func (c *PolicyController) inferTargetType(target types.ServiceInterfaceTarget) string {
	if target.Service != "" {
		return "service"
	}
	if target.Selector == "" {
		return ""
	}
	getBySelector := func(targetTypes ...string) string {
		for _, targetType := range targetTypes {
			retTarget, err := kube.GetServiceInterfaceTarget(targetType, target.Name, true, c.cli.Namespace, c.cli.KubeClient)
			if err == nil {
				if retTarget.Selector == target.Selector {
					return targetType
				}
			}
		}
		return ""
	}

	return getBySelector("deployment", "statefulset")
}

func NewPolicyController(cli *client.VanClient) *PolicyController {
	controller := &PolicyController{
		name:      "PolicyController",
		cli:       cli,
		validator: client.NewClusterPolicyValidator(cli),
		queue:     workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "PolicyHandler"),
		activeMap: map[string]time.Time{},
	}
	return controller
}

func (c *PolicyController) createInformer() {
	skupperCli, err := versioned.NewForConfig(c.cli.RestConfig)
	if err != nil {
		return
	}
	c.informer = v1alpha12.NewSkupperClusterPolicyInformer(
		skupperCli,
		time.Second*30,
		cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc},
	)
	c.informer.AddEventHandler(c)
}

func (c *PolicyController) validateStateChanged() {
	// Loading namespace policies
	c.LoadStaticPolicyList()

	// Validate incomingLink stage changed
	c.validateIncomingLinkStateChanged()

	// Validate outgoingLink state changed
	c.validateOutgoingLinkStateChanged()

	// Validate expose state changed
	c.validateExposeStateChanged()

	// Validate service state changed
	c.validateServiceStateChanged()
}

func (c *PolicyController) LoadStaticPolicyList() {
	pv := client.NewClusterPolicyValidator(c.cli)
	policies, err := pv.LoadNamespacePolicies()
	if err != nil {
		event.Recordf(c.name, "[LoadStaticPolicyList] error retrieving policies: %v", err)
		return
	}
	c.validator.SetStaticPolicyList(policies)
	// Notify policies used by controller
	for _, pv := range staticPolicyWatchers {
		pv.SetStaticPolicyList(policies)
	}
}

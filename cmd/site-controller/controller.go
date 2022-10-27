package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/skupperproject/skupper/pkg/version"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	corev1informer "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/informers/internalinterfaces"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/client"
)

type SiteController struct {
	vanClient            *client.VanClient
	siteInformer         cache.SharedIndexInformer
	tokenRequestInformer cache.SharedIndexInformer
	workqueue            workqueue.RateLimitingInterface
}

func NewSiteController(cli *client.VanClient) (*SiteController, error) {
	var watchNamespace string

	// Startup message
	if os.Getenv("WATCH_NAMESPACE") != "" {
		watchNamespace = os.Getenv("WATCH_NAMESPACE")
		log.Println("Skupper site controller watching current namespace ", watchNamespace)
	} else {
		watchNamespace = metav1.NamespaceAll
		log.Println("Skupper site controller watching all namespaces")
	}
	log.Printf("Version: %s", version.Version)

	siteInformer := corev1informer.NewFilteredConfigMapInformer(
		cli.KubeClient,
		watchNamespace,
		time.Second*30,
		cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc},
		internalinterfaces.TweakListOptionsFunc(func(options *metav1.ListOptions) {
			options.FieldSelector = "metadata.name=skupper-site"
			options.LabelSelector = "!" + types.SiteControllerIgnore
		}))
	tokenRequestInformer := corev1informer.NewFilteredSecretInformer(
		cli.KubeClient,
		watchNamespace,
		time.Second*30,
		cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc},
		internalinterfaces.TweakListOptionsFunc(func(options *metav1.ListOptions) {
			options.LabelSelector = types.TypeTokenRequestQualifier
		}))
	workqueue := workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "skupper-site-controller")

	controller := &SiteController{
		vanClient:            cli,
		siteInformer:         siteInformer,
		tokenRequestInformer: tokenRequestInformer,
		workqueue:            workqueue,
	}

	siteInformer.AddEventHandler(controller.getHandlerFuncs(SiteConfig, configmapResourceVersionTest))
	tokenRequestInformer.AddEventHandler(controller.getHandlerFuncs(TokenRequest, secretResourceVersionTest))
	return controller, nil
}

type resourceVersionTest func(a interface{}, b interface{}) bool

func configmapResourceVersionTest(a interface{}, b interface{}) bool {
	aa := a.(*corev1.ConfigMap)
	bb := b.(*corev1.ConfigMap)
	return aa.ResourceVersion == bb.ResourceVersion
}

func secretResourceVersionTest(a interface{}, b interface{}) bool {
	aa := a.(*corev1.Secret)
	bb := b.(*corev1.Secret)
	return aa.ResourceVersion == bb.ResourceVersion
}

func (c *SiteController) getHandlerFuncs(category triggerType, test resourceVersionTest) *cache.ResourceEventHandlerFuncs {
	return &cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			c.enqueueTrigger(obj, category)
		},
		UpdateFunc: func(old, new interface{}) {
			if !test(old, new) {
				c.enqueueTrigger(new, category)
			}
		},
		DeleteFunc: func(obj interface{}) {
			c.enqueueTrigger(obj, category)
		},
	}
}

func (c *SiteController) Run(stopCh <-chan struct{}) error {
	defer utilruntime.HandleCrash()
	defer c.workqueue.ShutDown()

	log.Println("Starting the Skupper site controller informers")
	go c.siteInformer.Run(stopCh)
	go c.tokenRequestInformer.Run(stopCh)

	log.Println("Waiting for informer caches to sync")
	if ok := cache.WaitForCacheSync(stopCh, c.siteInformer.HasSynced); !ok {
		return fmt.Errorf("Failed to wait for caches to sync")
	}
	log.Printf("Checking if sites need updates (%s)", version.Version)
	c.updateChecks()
	log.Println("Starting workers")
	go wait.Until(c.run, time.Second, stopCh)
	log.Println("Started workers")

	<-stopCh
	log.Println("Shutting down workers")
	return nil
}

type triggerType int

const (
	SiteConfig triggerType = iota
	Token
	TokenRequest
)

type trigger struct {
	key      string
	category triggerType
}

func (c *SiteController) run() {
	for c.processNextTrigger() {
	}
}

func (c *SiteController) processNextTrigger() bool {
	obj, shutdown := c.workqueue.Get()

	if shutdown {
		return false
	}

	defer c.workqueue.Done(obj)
	var t trigger
	var ok bool
	if t, ok = obj.(trigger); !ok {
		// invalid item
		c.workqueue.Forget(obj)
		utilruntime.HandleError(fmt.Errorf("Invalid item on work queue %#v", obj))
		return true
	}

	err := c.dispatchTrigger(t)
	c.workqueue.Forget(obj)
	if err != nil {
		utilruntime.HandleError(err)
	}

	return true
}

func (c *SiteController) dispatchTrigger(trigger trigger) error {
	switch trigger.category {
	case SiteConfig:
		return c.checkSite(trigger.key)
	case TokenRequest:
		return c.checkTokenRequest(trigger.key)
	default:
		return fmt.Errorf("invalid trigger %d", trigger.category)
	}

}

func (c *SiteController) enqueueTrigger(obj interface{}, category triggerType) {
	var key string
	var err error
	if key, err = cache.MetaNamespaceKeyFunc(obj); err != nil {
		utilruntime.HandleError(err)
		return
	}
	c.workqueue.Add(trigger{
		key:      key,
		category: category,
	})
}

func (c *SiteController) checkAllForSite() {
	// Now need to check whether there are any token requests already in place
	log.Println("Checking token requests...")
	c.checkAllTokenRequests()
	log.Println("Done.")
}

func (c *SiteController) checkSite(key string) error {
	// get site namespace
	siteNamespace, _, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		log.Println("Error checking skupper-site namespace: ", err)
		return err
	}
	// get skupper-site configmap
	obj, exists, err := c.siteInformer.GetStore().GetByKey(key)
	if err != nil {
		log.Println("Error checking skupper-site config map: ", err)
		return err
	} else if exists {
		configmap := obj.(*corev1.ConfigMap)
		_, err := c.vanClient.RouterInspectNamespace(context.Background(), configmap.ObjectMeta.Namespace)
		if err == nil {
			log.Println("Skupper site exists", key)
			updatedLogging, err := c.vanClient.RouterUpdateLogging(context.Background(), configmap, false)
			if err != nil {
				log.Println("Error checking router logging configuration:", err)
			}
			updatedDebugMode, err := c.vanClient.RouterUpdateDebugMode(context.Background(), configmap)
			if err != nil {
				log.Println("Error updating router debug mode:", err)
			}
			if updatedLogging {
				if updatedDebugMode {
					log.Println("Updated router logging and debug mode for", key)
				} else {
					err = c.vanClient.RouterRestart(context.Background(), configmap.ObjectMeta.Namespace)
					if err != nil {
						log.Println("Error restarting router:", err)
					} else {
						log.Println("Updated router logging for", key)
					}
				}
			} else if updatedDebugMode {
				log.Println("Updated debug mode for", key)
			}
			updatedAnnotations, err := c.vanClient.RouterUpdateAnnotations(context.Background(), configmap)
			if err != nil {
				log.Println("Error checking annotations:", err)
			} else if updatedAnnotations {
				log.Println("Updated annotations for", key)
			}

			c.checkAllForSite()
		} else if errors.IsNotFound(err) {
			log.Println("Initialising skupper site ...")
			siteConfig, _ := c.vanClient.SiteConfigInspect(context.Background(), configmap)
			siteConfig.Spec.SkupperNamespace = siteNamespace
			ctx, cancel := context.WithTimeout(context.Background(), types.DefaultTimeoutDuration)
			defer cancel()
			err = c.vanClient.RouterCreate(ctx, *siteConfig)
			if err != nil {
				log.Println("Error initialising skupper: ", err)
				return err
			} else {
				log.Println("Skupper site initialised")
				c.checkAllForSite()
			}
		} else {
			log.Println("Error inspecting VAN router: ", err)
			return err
		}
	}
	return nil
}

func getTokenCost(token *corev1.Secret) (int32, bool) {
	if token.ObjectMeta.Annotations == nil {
		return 0, false
	}
	if costString, ok := token.ObjectMeta.Annotations[types.TokenCost]; ok {
		cost, err := strconv.Atoi(costString)
		if err != nil {
			log.Printf("Ignoring invalid cost annotation %q", costString)
			return 0, false
		}
		return int32(cost), true
	}
	return 0, false
}

func (c *SiteController) connect(token *corev1.Secret, namespace string) error {
	log.Printf("Connecting site in %s using token %s", namespace, token.ObjectMeta.Name)
	var options types.ConnectorCreateOptions
	options.Name = token.ObjectMeta.Name
	options.SkupperNamespace = namespace
	if cost, ok := getTokenCost(token); ok {
		options.Cost = cost
	}
	return c.vanClient.ConnectorCreate(context.Background(), token, options)
}

func (c *SiteController) disconnect(name string, namespace string) error {
	log.Printf("Disconnecting connector %s from site in %s", name, namespace)
	var options types.ConnectorRemoveOptions
	options.Name = name
	options.SkupperNamespace = namespace
	// Secret has already been deleted so force update to current active secrets
	options.ForceCurrent = true
	return c.vanClient.ConnectorRemove(context.Background(), options)
}

func (c *SiteController) generate(token *corev1.Secret) error {
	log.Printf("Generating token for request %s...", token.ObjectMeta.Name)
	generated, _, err := c.vanClient.ConnectorTokenCreate(context.Background(), token.ObjectMeta.Name, token.ObjectMeta.Namespace)
	if err == nil {
		token.Data = generated.Data
		if token.ObjectMeta.Annotations == nil {
			token.ObjectMeta.Annotations = make(map[string]string)
		}
		for key, value := range generated.ObjectMeta.Annotations {
			token.ObjectMeta.Annotations[key] = value
		}
		token.ObjectMeta.Labels[types.SkupperTypeQualifier] = types.TypeToken
		siteId := c.getSiteIdForNamespace(token.ObjectMeta.Namespace)
		if siteId != "" {
			token.ObjectMeta.Annotations[types.TokenGeneratedBy] = siteId
		}
		_, err = c.vanClient.KubeClient.CoreV1().Secrets(token.ObjectMeta.Namespace).Update(token)
		return err
	} else {
		log.Printf("Failed to generate token for request %s: %s", token.ObjectMeta.Name, err)
		return err
	}
}

func (c *SiteController) checkAllTokenRequests() {
	// can we rely on the cache here?
	tokens := c.tokenRequestInformer.GetStore().List()
	for _, t := range tokens {
		// service from workqueue
		c.enqueueTrigger(t, TokenRequest)
	}
}

func (c *SiteController) checkTokenRequest(key string) error {
	log.Printf("Handling token request for %s", key)
	obj, exists, err := c.tokenRequestInformer.GetStore().GetByKey(key)
	if err != nil {
		log.Println("Error checking connection-token-request secret: ", err)
		return err
	} else if exists {
		token := obj.(*corev1.Secret)
		if !c.isTokenRequestValidInSite(token) {
			log.Println("Cannot handle token request, as site not yet initialised")
			return nil
		}
		return c.generate(token)
	}
	return nil
}

func (c *SiteController) getSiteIdForNamespace(namespace string) string {
	cm, err := c.vanClient.KubeClient.CoreV1().ConfigMaps(namespace).Get(types.SiteConfigMapName, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			log.Printf("Could not obtain siteid for namespace %q, assuming not yet initialised", namespace)
		} else {
			log.Printf("Error checking siteid for namespace %q: %s", namespace, err)
		}
		return ""
	}
	return string(cm.ObjectMeta.UID)
}

func (c *SiteController) isTokenValidInSite(token *corev1.Secret) bool {
	siteId := c.getSiteIdForNamespace(token.ObjectMeta.Namespace)
	if author, ok := token.ObjectMeta.Annotations[types.TokenGeneratedBy]; ok && author == siteId {
		// token was generated by this site so should not be applied
		return false
	} else {
		return true
	}
}

func (c *SiteController) isTokenRequestValidInSite(token *corev1.Secret) bool {
	siteId := c.getSiteIdForNamespace(token.ObjectMeta.Namespace)
	if siteId == "" {
		return false
	}
	return true
}

func (c *SiteController) updateChecks() {
	sites := c.siteInformer.GetStore().List()
	for _, s := range sites {
		if site, ok := s.(*corev1.ConfigMap); ok {
			updated, err := c.vanClient.RouterUpdateVersionInNamespace(context.Background(), false, site.ObjectMeta.Namespace)
			if err != nil {
				log.Printf("Version update check failed for namespace %q: %s", site.ObjectMeta.Namespace, err)
			} else if updated {
				log.Printf("Updated version for namespace %q", site.ObjectMeta.Namespace)
			} else {
				log.Printf("Version update not required for namespace %q", site.ObjectMeta.Namespace)
			}
		} else {
			log.Printf("Unexpected item in site informer store: %v", s)
		}
	}
}

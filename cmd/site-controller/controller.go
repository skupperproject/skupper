package main

import (
	"context"
	"fmt"
	"log"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1informer "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/informers/internalinterfaces"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
 	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/client"
)

type SiteController struct {
	vanClient     *client.VanClient
	siteInformer  cache.SharedIndexInformer
	tokenInformer cache.SharedIndexInformer
	tokenRequestInformer cache.SharedIndexInformer
	workqueue     workqueue.RateLimitingInterface
	siteId        string
}

func NewSiteController(cli *client.VanClient) (*SiteController, error) {
	siteInformer:= corev1informer.NewFilteredConfigMapInformer(
		cli.KubeClient,
		cli.Namespace,
		time.Second*30,
		cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc},
		internalinterfaces.TweakListOptionsFunc(func(options *metav1.ListOptions) {
			options.FieldSelector = "metadata.name=skupper-site"
			options.LabelSelector = "!internal.skupper.io/site-controller-ignore"
		}))
	tokenInformer:= corev1informer.NewFilteredSecretInformer(
		cli.KubeClient,
		cli.Namespace,
		time.Second*30,
		cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc},
		internalinterfaces.TweakListOptionsFunc(func(options *metav1.ListOptions) {
			options.LabelSelector = types.TypeTokenQualifier
		}))
	tokenRequestInformer:= corev1informer.NewFilteredSecretInformer(
		cli.KubeClient,
		cli.Namespace,
		time.Second*30,
		cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc},
		internalinterfaces.TweakListOptionsFunc(func(options *metav1.ListOptions) {
			options.LabelSelector = types.TypeTokenRequestQualifier
		}))
	workqueue := workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "skupper-site-controller")

	controller := &SiteController{
		vanClient:    cli,
		siteInformer:  siteInformer,
		tokenInformer:  tokenInformer,
		tokenRequestInformer:  tokenRequestInformer,
		workqueue:  workqueue,
	}

	siteInformer.AddEventHandler(controller.getHandlerFuncs(SiteConfig, configmapResourceVersionTest))
	tokenInformer.AddEventHandler(controller.getHandlerFuncs(Token, secretResourceVersionTest))
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
	}
}

func (c *SiteController) Run (stopCh <-chan struct{}) error {
	defer utilruntime.HandleCrash()
	defer c.workqueue.ShutDown()

	log.Println("Starting the Skupper site controller informers")
	go c.siteInformer.Run(stopCh)
	go c.tokenInformer.Run(stopCh)
	go c.tokenRequestInformer.Run(stopCh)

	log.Println("Waiting for informer caches to sync")
	if ok := cache.WaitForCacheSync(stopCh, c.siteInformer.HasSynced, c.tokenInformer.HasSynced); !ok {
		return fmt.Errorf("Failed to wait for caches to sync")
	}

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
	case Token:
		return c.checkToken(trigger.key)
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
               key: key,
               category: category,
       })
}

func (c *SiteController) enqueueConfigMap(obj interface{}) {
	c.enqueueTrigger(obj, SiteConfig)
}

func (c *SiteController) enqueueSecret(obj interface{}) {
	c.enqueueTrigger(obj, Token)
}

func (c *SiteController) setSiteId(skupperSite *corev1.ConfigMap) {
	c.siteId = string(skupperSite.ObjectMeta.UID)
	// Now need to check whether there are any token requests already in place
	log.Println("Checking tokens...")
	c.checkAllTokens()
	log.Println("Checking token requests...")
	c.checkAllTokenRequests()
	log.Println("Done.")
}

func (c *SiteController) checkSite(key string) error {
	//get skupper-site configmap
	obj, exists, err := c.siteInformer.GetStore().GetByKey(key)
	if err != nil {
		log.Println("Error checking skupper-site config map: ", err)
		return err
	} else if exists {
		configmap := obj.(*corev1.ConfigMap)
		vanRouterInspectResponse, err := c.vanClient.VanRouterInspect(context.Background())
		if err == nil {
			log.Println("Skupper site exists ", key)
			wantEdgeMode := configmap.Data["edge"] == "true"
			haveEdgeMode := vanRouterInspectResponse.Status.Mode == string(types.TransportModeEdge)
			// TODO: enable richer comparison/checking (possibly with GetVanRouterSpecFromOpts?)
			if  wantEdgeMode != haveEdgeMode {
				//TODO: enable van router update
			}
			c.setSiteId(configmap)
		} else if errors.IsNotFound(err) {
			log.Println("Initialising skupper site ...")
			siteConfig, _ := c.vanClient.VanSiteConfigInspect(context.Background(), configmap)
			err = c.vanClient.VanRouterCreate(context.Background(), *siteConfig)
			if err != nil {
				log.Println("Error initialising skupper: ", err)
				return err
			} else {
				log.Println("Skupper site initialised")
				c.setSiteId(configmap)
			}
		} else {
			log.Println("Error inspecting VAN router: ", err)
			return err
		}
	}
	return nil
}

func (c *SiteController) connect(token *corev1.Secret) error {
	var options types.VanConnectorCreateOptions
	options.Name = token.ObjectMeta.Name
	//TODO: infer cost from token metadata?
	return c.vanClient.VanConnectorCreate(context.Background(), token, options)
}

func (c *SiteController) generate(token *corev1.Secret) error {
	log.Printf("Generating token for request %s...", token.ObjectMeta.Name)
	log.Println()
	generated, _, err := c.vanClient.VanConnectorTokenCreate(context.Background(), token.ObjectMeta.Name)
	if err == nil {
		token.Data = generated.Data
		if token.ObjectMeta.Annotations == nil {
			token.ObjectMeta.Annotations = make(map[string]string)
		}
		for key, value := range generated.ObjectMeta.Annotations {
			token.ObjectMeta.Annotations[key] = value
		}
		token.ObjectMeta.Labels[types.SkupperTypeQualifier] = types.TypeToken
		token.ObjectMeta.Annotations[types.TokenGeneratedBy] = c.siteId
		_, err = c.vanClient.KubeClient.CoreV1().Secrets(c.vanClient.Namespace).Update(token)
		return err
	} else {
		log.Printf("Failed to generate token for request %s: %s", token.ObjectMeta.Name, err)
		log.Println()
		return err
	}
}

func (c *SiteController) checkAllTokens() {
	//can we rely on the cache here?
	tokens := c.tokenInformer.GetStore().List()
	for _, t := range tokens {
		token := t.(*corev1.Secret)
		if !c.isOwnToken(token) {
			err := c.connect(token)
			if err != nil {
				log.Println("Error checking connection-token secret: ", err)
			}
		}
	}
}

func (c *SiteController) checkAllTokenRequests() {
	//can we rely on the cache here?
	tokens := c.tokenInformer.GetStore().List()
	for _, t := range tokens {
		token := t.(*corev1.Secret)
		err := c.generate(token)
		if err != nil {
			log.Println("Error checking connection-token secret: ", err)
		}
	}
}

func (c *SiteController) checkToken(key string) error {
	if c.siteId != "" {
		obj, exists, err := c.tokenInformer.GetStore().GetByKey(key)
		if err != nil {
			log.Println("Error checking connection-token secret: ", err)
			return err
		} else if exists {
			token := obj.(*corev1.Secret)
			if !c.isOwnToken(token) {
				return c.connect(token)
			} else {
				return nil
			}
		}
	} else {
		log.Println("Cannot handle token, as site not yet initialised")
	}
	return nil
}

func (c *SiteController) checkTokenRequest(key string) error {
	if c.siteId != "" {
		log.Printf("Handling token request for %s", key)
		log.Println()
		obj, exists, err := c.tokenRequestInformer.GetStore().GetByKey(key)
		if err != nil {
			log.Println("Error checking connection-token-request secret: ", err)
			return err
		} else if exists {
			token := obj.(*corev1.Secret)
			return c.generate(token)
		}
	} else {
		log.Println("Cannot handle token request, as site not yet initialised")
	}
	return nil
}

func (c *SiteController) isOwnToken(token *corev1.Secret) bool {
	if author, ok := token.ObjectMeta.Annotations[types.TokenGeneratedBy]; ok {
		return author == c.siteId
	} else {
		return false
	}
}

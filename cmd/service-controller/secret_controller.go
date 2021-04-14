package main

import (
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	corev1informer "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/informers/internalinterfaces"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"

	"github.com/skupperproject/skupper/pkg/event"
)

type SecretHandler interface {
	Handle(name string, secret *corev1.Secret) error
}

type SecretController struct {
	name     string
	handler  SecretHandler
	informer cache.SharedIndexInformer
	queue    workqueue.RateLimitingInterface
}

func (c *SecretController) enqueue(obj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err == nil {
		c.queue.Add(key)
	} else {
		event.Recordf(c.name, "Error retrieving key: %s", err)
	}
}

func (c *SecretController) OnAdd(obj interface{}) {
	c.enqueue(obj)
}

func (c *SecretController) OnUpdate(a, b interface{}) {
	aa := a.(*corev1.Secret)
	bb := b.(*corev1.Secret)
	if aa.ResourceVersion != bb.ResourceVersion {
		c.enqueue(b)
	}
}

func (c *SecretController) OnDelete(obj interface{}) {
	c.enqueue(obj)
}

func (c *SecretController) start(stopCh <-chan struct{}) error {
	go c.informer.Run(stopCh)
	if ok := cache.WaitForCacheSync(stopCh, c.informer.HasSynced); !ok {
		return fmt.Errorf("Failed to wait for caches to sync")
	}
	go wait.Until(c.run, time.Second, stopCh)
	return nil
}

func (c *SecretController) stop() {
	c.queue.ShutDown()
}

func (c *SecretController) run() {
	for c.process() {
	}
}

func (c *SecretController) process() bool {
	obj, shutdown := c.queue.Get()

	if shutdown {
		return false
	}

	defer c.queue.Done(obj)
	retry := false
	if key, ok := obj.(string); ok {
		entity, exists, err := c.informer.GetStore().GetByKey(key)
		if err != nil {
			event.Recordf(c.name, "Error retrieving secret %q: %s", key, err)
		}
		if exists {
			if secret, ok := entity.(*corev1.Secret); ok {
				err := c.handler.Handle(key, secret)
				if err != nil {
					retry = true
					event.Recordf(c.name, "Error handling %q: %s", key, err)
				}
			} else {
				event.Recordf(c.name, "Expected secret, got %#v", entity)
			}
		} else {
			err := c.handler.Handle(key, nil)
			if err != nil {
				retry = true
				event.Recordf(c.name, "Error handling %q: %s", key, err)
			}
		}
	} else {
		event.Recordf(c.name, "Expected key to be string, was %#v", key)
	}
	c.queue.Forget(obj)

	if retry && c.queue.NumRequeues(obj) < 5 {
		c.queue.AddRateLimited(obj)
	}
	return true
}

func NewSecretController(name string, selector string, client kubernetes.Interface, namespace string, handler SecretHandler) *SecretController {
	informer := corev1informer.NewFilteredSecretInformer(
		client,
		namespace,
		time.Second*30,
		cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc},
		internalinterfaces.TweakListOptionsFunc(func(options *metav1.ListOptions) {
			options.LabelSelector = selector
		}))
	queue := workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), name)

	controller := &SecretController{
		name:     name,
		handler:  handler,
		informer: informer,
		queue:    queue,
	}

	informer.AddEventHandler(controller)
	return controller
}

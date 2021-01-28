package main

import (
	"fmt"
	"log"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	corev1informer "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"

	"github.com/skupperproject/skupper/client"
)

type IpLookup struct {
	informer cache.SharedIndexInformer
	events   workqueue.RateLimitingInterface
	lookup   map[string]string
	reverse  map[string]string
	lock     sync.RWMutex
}

func NewIpLookup(cli *client.VanClient) *IpLookup {
	informer := corev1informer.NewPodInformer(
		cli.KubeClient,
		cli.Namespace,
		time.Second*30,
		cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})

	events := workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "skupper-ip-lookup")

	iplookup := &IpLookup{
		informer: informer,
		events:   events,
		lookup:   map[string]string{},
		reverse:  map[string]string{},
	}

	informer.AddEventHandler(newEventHandlerFor(iplookup.events, "", SimpleKey, PodResourceVersionTest))

	return iplookup
}

func (i *IpLookup) getPodName(ip string) string {
	i.lock.RLock()
	defer i.lock.RUnlock()
	return i.lookup[ip]
}

//support data.NameMapping interface
func (i *IpLookup) Lookup(ip string) string {
	name := i.getPodName(ip)
	log.Printf("LOOKUP: %s -> %s", ip, name)
	if name == "" {
		return ip
	} else {
		return name
	}
}

func (i *IpLookup) translateKeys(ips map[string]interface{}) map[string]interface{} {
	out := map[string]interface{}{}
	i.lock.RLock()
	defer i.lock.RUnlock()
	for key, value := range ips {
		if changed, ok := i.lookup[key]; ok {
			out[changed] = value
		} else {
			out[key] = value
		}
	}
	return out
}

func (i *IpLookup) updateLookup(name string, key string, ip string) {
	log.Printf("%s mapped to %s", ip, name)
	i.lock.Lock()
	defer i.lock.Unlock()
	i.lookup[ip] = name
	i.reverse[key] = ip
}

func (i *IpLookup) deleteLookup(key string) string {
	i.lock.Lock()
	defer i.lock.Unlock()
	ip, ok := i.reverse[key]
	if ok {
		delete(i.lookup, ip)
		delete(i.reverse, key)
		return ip
	}
	return ""
}

func (i *IpLookup) start(stopCh <-chan struct{}) error {
	go i.informer.Run(stopCh)
	if ok := cache.WaitForCacheSync(stopCh, i.informer.HasSynced); !ok {
		return fmt.Errorf("Failed to wait for caches to sync")
	}
	go wait.Until(i.runIpLookup, time.Second, stopCh)

	return nil
}

func (i *IpLookup) stop() {
	i.events.ShutDown()
}

func (i *IpLookup) runIpLookup() {
	for i.processNextEvent() {
	}
}

func (i *IpLookup) processNextEvent() bool {

	obj, shutdown := i.events.Get()

	if shutdown {
		return false
	}

	err := func(obj interface{}) error {
		defer i.events.Done(obj)

		var ok bool
		var key string
		if key, ok = obj.(string); !ok {
			// invalid item
			i.events.Forget(obj)
			return fmt.Errorf("expected string in events but got %#v", obj)
		} else {
			obj, exists, err := i.informer.GetStore().GetByKey(key)
			if err != nil {
				return fmt.Errorf("Error reading pod from cache: %s", err)
			} else if exists {
				pod, ok := obj.(*corev1.Pod)
				if !ok {
					return fmt.Errorf("Expected Pod for %s but got %#v", key, obj)
				}
				i.updateLookup(pod.ObjectMeta.Name, key, pod.Status.PodIP)
			} else {
				ip := i.deleteLookup(key)
				if ip != "" {
					log.Printf("mapping for %s deleted", ip)
				}
			}
		}
		i.events.Forget(obj)
		return nil
	}(obj)

	if err != nil {
		utilruntime.HandleError(err)
		return true
	}

	return true
}

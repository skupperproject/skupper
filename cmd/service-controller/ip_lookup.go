package main

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	corev1informer "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"

	"github.com/skupperproject/skupper/client"
	"github.com/skupperproject/skupper/pkg/event"
	"github.com/skupperproject/skupper/pkg/flow"
)

type ProcessUpdateHandler func(deleted bool, name string, process *flow.ProcessRecord) error

type IpLookup struct {
	informer cache.SharedIndexInformer
	events   workqueue.RateLimitingInterface
	lookup   map[string]string
	reverse  map[string]string
	lock     sync.RWMutex
	handler  ProcessUpdateHandler
}

func NewIpLookup(cli *client.VanClient, handler ProcessUpdateHandler) *IpLookup {
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
		handler:  handler,
	}

	informer.AddEventHandler(newEventHandlerFor(iplookup.events, "", SimpleKey, PodResourceVersionTest))

	return iplookup
}

func (i *IpLookup) getPodName(ip string) string {
	i.lock.RLock()
	defer i.lock.RUnlock()
	return i.lookup[ip]
}

// support data.NameMapping interface
func (i *IpLookup) Lookup(ip string) string {
	name := i.getPodName(ip)
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

const (
	IpMappingEvent string = "IpMappingEvent"
)

func (i *IpLookup) updateLookup(name string, key string, ip string) {
	event.Recordf(IpMappingEvent, "%s mapped to %s", ip, name)
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
				if i.handler != nil {
					process := &flow.ProcessRecord{}
					process.Identity = string(pod.ObjectMeta.UID)
					process.Parent = os.Getenv("SKUPPER_SITE_ID")
					process.StartTime = uint64(pod.ObjectMeta.CreationTimestamp.UnixNano())
					process.Name = &pod.ObjectMeta.Name
					if pod.Status.PodIP != "" {
						process.SourceHost = &pod.Status.PodIP
					}
					process.Image = &pod.Spec.Containers[0].Image
					process.ImageName = process.Image
					process.HostName = &pod.Spec.NodeName
					if labelName, ok := pod.ObjectMeta.Labels["app.kubernetes.io/part-of"]; ok {
						process.GroupName = &labelName
					} else if labelComponent, ok := pod.ObjectMeta.Labels["app.kubernetes.io/name"]; ok {
						process.GroupName = &labelComponent
					} else if partOf, ok := pod.ObjectMeta.Labels["app.kubernetes.io/component"]; ok {
						process.GroupName = &partOf
					} else {
						// generate process group from image name
						parts := strings.Split(*process.ImageName, "/")
						part := parts[len(parts)-1]
						pg := strings.Split(part, ":")
						process.GroupName = &pg[0]
					}
					i.handler(false, key, process)
				}
			} else {
				ip := i.deleteLookup(key)
				if i.handler != nil {
					i.handler(true, key, nil)
				}
				if ip != "" {
					event.Recordf(IpMappingEvent, "mapping for %s deleted", ip)
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

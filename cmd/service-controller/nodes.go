package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	corev1informer "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"

	"github.com/skupperproject/skupper/client"
	"github.com/skupperproject/skupper/pkg/flow"
)

type NodeUpdateHandler func(deleted bool, name string, host *flow.HostRecord) error

type NodeWatcher struct {
	informer cache.SharedIndexInformer
	events   workqueue.RateLimitingInterface
	//	lookup   map[string]string
	//	reverse  map[string]string
	lock         sync.RWMutex
	handler      NodeUpdateHandler
	cliNamespace string
}

func NewNodeWatcher(cli *client.VanClient, handler NodeUpdateHandler) *NodeWatcher {
	_, err := cli.KubeClient.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil
	}

	informer := corev1informer.NewNodeInformer(
		cli.KubeClient,
		time.Second*30,
		cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})

	events := workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "skupper-node-watcher")

	nodeWatcher := &NodeWatcher{
		informer:     informer,
		events:       events,
		handler:      handler,
		cliNamespace: cli.GetNamespace(),
	}

	informer.AddEventHandler(newEventHandlerFor(nodeWatcher.events, "", SimpleKey, NodeResourceVersionTest))

	return nodeWatcher
}

const (
	NodeMappingEvent string = "NodeMappingEvent"
)

func (nw *NodeWatcher) start(stopCh <-chan struct{}) error {
	go nw.informer.Run(stopCh)
	if ok := cache.WaitForCacheSync(stopCh, nw.informer.HasSynced); !ok {
		return fmt.Errorf("Failed to wait for caches to sync")
	}
	go wait.Until(nw.runNodeWatcher, time.Second, stopCh)

	return nil
}

func (nw *NodeWatcher) stop() {
	nw.events.ShutDown()
}

func (nw *NodeWatcher) runNodeWatcher() {
	for nw.processNextEvent() {
	}
}

func (nw *NodeWatcher) processNextEvent() bool {

	obj, shutdown := nw.events.Get()

	if shutdown {
		return false
	}

	err := func(obj interface{}) error {
		defer nw.events.Done(obj)

		var ok bool
		var key string
		if key, ok = obj.(string); !ok {
			// invalid item
			nw.events.Forget(obj)
			return fmt.Errorf("expected string in events but got %#v", obj)
		} else {
			obj, exists, err := nw.informer.GetStore().GetByKey(key)
			if err != nil {
				return fmt.Errorf("Error reading pod from cache: %s", err)
			} else if exists {
				node, ok := obj.(*corev1.Node)
				if !ok {
					return fmt.Errorf("Expected Node for %s but got %#v", key, obj)
				}
				host := &flow.HostRecord{}
				// Note: skupper running in multiple ns in same cluster the hosts are the same
				host.Identity = string(node.ObjectMeta.UID) + "-" + nw.cliNamespace
				host.Parent = os.Getenv("SKUPPER_SITE_ID")
				host.StartTime = uint64(node.ObjectMeta.CreationTimestamp.UnixNano())
				host.Name = &node.ObjectMeta.Name
				host.Arch = &node.Status.NodeInfo.Architecture

				provider := strings.Split(node.Spec.ProviderID, "://")
				if len(provider) > 0 {
					host.Provider = &provider[0]
				}
				if region, ok := node.ObjectMeta.Labels["topology.kubernetes.io/region"]; ok {
					host.Location = &region
					host.Region = &region
				}
				if zone, ok := node.ObjectMeta.Labels["topology.kubernetes.io/zone"]; ok {
					host.Zone = &zone
				}
				host.ContainerRuntime = &node.Status.NodeInfo.ContainerRuntimeVersion
				host.KernelVersion = &node.Status.NodeInfo.KernelVersion
				host.KubeProxyVersion = &node.Status.NodeInfo.KubeProxyVersion
				host.KubeletVersion = &node.Status.NodeInfo.KubeletVersion
				if nw.handler != nil {
					nw.handler(false, *host.Name, host)
				}
			} else {
				if nw.handler != nil {
					nw.handler(true, key, nil)
				}
				if key != "" {
					fmt.Printf("Mapping for node %s deleted\n", key)
				}
			}
		}
		nw.events.Forget(obj)
		return nil
	}(obj)

	if err != nil {
		utilruntime.HandleError(err)
		return true
	}

	return true
}

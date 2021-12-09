package main

import (
	"fmt"
	"log"
	"math"
	"time"

	corev1 "k8s.io/api/core/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"

	"github.com/skupperproject/skupper/pkg/qdr"
)

// Syncs the live router config with the configmap (currently only
// bridge configuration needs to be synced in this way)
type ConfigSync struct {
	informer  cache.SharedIndexInformer
	events    workqueue.RateLimitingInterface
	agentPool *qdr.AgentPool
}

func enqueue(events workqueue.RateLimitingInterface, obj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err == nil {
		events.Add(key)
	} else {
		log.Printf("Error getting key: %s", err)
	}

}

func newEventHandler(events workqueue.RateLimitingInterface) *cache.ResourceEventHandlerFuncs {
	return &cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			enqueue(events, obj)
		},
		UpdateFunc: func(old, new interface{}) {
			enqueue(events, new)
		},
		DeleteFunc: func(obj interface{}) {
			enqueue(events, obj)
		},
	}
}

func newConfigSync(configInformer cache.SharedIndexInformer) *ConfigSync {
	configSync := &ConfigSync{
		informer:  configInformer,
		agentPool: qdr.NewAgentPool("amqp://localhost:5672", nil),
	}
	configSync.events = workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "skupper-config-sync")
	configSync.informer.AddEventHandler(newEventHandler(configSync.events))
	return configSync
}

func (c *ConfigSync) start(stopCh <-chan struct{}) error {
	go wait.Until(c.runConfigSync, time.Second, stopCh)

	return nil
}

func (c *ConfigSync) stop() {
	c.events.ShutDown()
}

func (c *ConfigSync) runConfigSync() {
	for c.processNextEvent() {
	}
}

const (
	ConfigSyncEvent string = "ConfigSyncEvent"
	ConfigSyncError string = "ConfigSyncError"
)

func (c *ConfigSync) processNextEvent() bool {
	log.Println("getting sync event")
	obj, shutdown := c.events.Get()
	log.Println("sync triggered")

	if shutdown {
		return false
	}

	err := func(obj interface{}) error {
		defer c.events.Done(obj)

		var ok bool
		var key string
		if key, ok = obj.(string); !ok {
			// invalid item
			log.Printf("expected string in events but got %#v", obj)
			c.events.Forget(obj)
			return fmt.Errorf("expected string in events but got %#v", obj)
		} else {
			obj, exists, err := c.informer.GetStore().GetByKey(key)
			if err != nil {
				return fmt.Errorf("Error reading pod from cache: %s", err)
			}
			if exists {
				configmap, ok := obj.(*corev1.ConfigMap)
				if !ok {
					return fmt.Errorf("Expected ConfigMap for %s but got %#v", key, obj)
				}
				bridges, err := qdr.GetBridgeConfigFromConfigMap(configmap)
				if err != nil {
					return fmt.Errorf("Error parsing bridge configuration from %s: %s", key, err)
				}
				err = c.syncConfig(bridges)
				if err != nil {
					log.Printf("sync failed: %s", err)
					return err
				}
			}
		}
		log.Println("sync succeeded")
		c.events.Forget(obj)
		return nil
	}(obj)

	if err != nil {
		if c.events.NumRequeues(obj) < 5 {
			log.Printf("Requeuing %v after error: %v", obj, err)
			c.events.AddRateLimited(obj)
		} else {
			log.Printf("Delayed requeue %v after error: %v", obj, err)
			c.events.AddAfter(obj, time.Duration(math.Min(float64(c.events.NumRequeues(obj)/5), 10))*time.Minute)
		}
		utilruntime.HandleError(err)
	}

	return true
}

func syncConfig(agent *qdr.Agent, desired *qdr.BridgeConfig) (bool, error) {
	actual, err := agent.GetLocalBridgeConfig()
	if err != nil {
		return false, fmt.Errorf("Error retrieving bridges: %s", err)
	}
	differences := actual.Difference(desired)
	if differences.Empty() {
		return true, nil
	} else {
		differences.Print()
		if err = agent.UpdateLocalBridgeConfig(differences); err != nil {
			return false, fmt.Errorf("Error syncing bridges: %s", err)
		}
		return false, nil
	}
}

func (c *ConfigSync) syncConfig(desired *qdr.BridgeConfig) error {
	agent, err := c.agentPool.Get()
	if err != nil {
		return fmt.Errorf("Could not get management agent : %s", err)
	}
	var synced bool
	for i := 0; i < 3 && err == nil && !synced; i++ {
		synced, err = syncConfig(agent, desired)
	}
	c.agentPool.Put(agent)
	if err != nil {
		return fmt.Errorf("Error while syncing bridge config : %s", err)
	}
	if !synced {
		return fmt.Errorf("Failed to sync bridge config")
	}
	return nil
}

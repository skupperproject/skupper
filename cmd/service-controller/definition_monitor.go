package main

import (
	jsonencoding "encoding/json"
	"fmt"
	"log"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	appsv1informer "k8s.io/client-go/informers/apps/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/client"
	"github.com/skupperproject/skupper/pkg/kube"
)

// DefinitionMonitor updates skupper service definitions based on
// changes to other entities (currently statefulsets exposed via
// headless services)
type DefinitionMonitor struct {
	origin              string
	vanClient           *client.VanClient
	statefulSetInformer cache.SharedIndexInformer
	svcDefInformer      cache.SharedIndexInformer
	events              workqueue.RateLimitingInterface
	headless            map[string]types.ServiceInterface
}

func newDefinitionMonitor(origin string, client *client.VanClient, svcDefInformer cache.SharedIndexInformer) *DefinitionMonitor {
	monitor := &DefinitionMonitor{
		origin:         origin,
		vanClient:      client,
		svcDefInformer: svcDefInformer,
		headless:       make(map[string]types.ServiceInterface),
	}
	monitor.statefulSetInformer = appsv1informer.NewStatefulSetInformer(
		client.KubeClient,
		client.Namespace,
		time.Second*30,
		cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
	monitor.events = workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "skupper-service-monitor")

	monitor.statefulSetInformer.AddEventHandler(newEventHandlerFor(monitor.events, "statefulsets", AnnotatedKey, StatefulSetResourceVersionTest))
	monitor.svcDefInformer.AddEventHandler(newEventHandlerFor(monitor.events, "servicedefs", AnnotatedKey, ConfigMapResourceVersionTest))

	return monitor
}

func (m *DefinitionMonitor) start(stopCh <-chan struct{}) error {
	go m.statefulSetInformer.Run(stopCh)
	if ok := cache.WaitForCacheSync(stopCh, m.statefulSetInformer.HasSynced); !ok {
		return fmt.Errorf("Failed to wait for caches to sync")
	}
	go wait.Until(m.runDefinitionMonitor, time.Second, stopCh)

	return nil
}

func (m *DefinitionMonitor) stop() {
	m.events.ShutDown()
}

func (m *DefinitionMonitor) runDefinitionMonitor() {
	for m.processNextEvent() {
	}
}

func (m *DefinitionMonitor) processNextEvent() bool {

	obj, shutdown := m.events.Get()

	if shutdown {
		return false
	}

	err := func(obj interface{}) error {
		defer m.events.Done(obj)

		var ok bool
		var key string
		if key, ok = obj.(string); !ok {
			// invalid item
			m.events.Forget(obj)
			return fmt.Errorf("expected string in events but got %#v", obj)
		} else {
			category, name := splitKey(key)
			switch category {
			case "servicedefs":
				//get the configmap, parse the json, check against the current servicebindings map
				obj, exists, err := m.svcDefInformer.GetStore().GetByKey(name)
				if err != nil {
					return fmt.Errorf("Error reading skupper-services from cache: %s", err)
				} else if exists {
					cm, ok := obj.(*corev1.ConfigMap)
					if !ok {
						return fmt.Errorf("Expected ConfigMap for %s but got %#v", name, obj)
					}
					if cm.Data != nil && len(cm.Data) > 0 {
						for k, v := range cm.Data {
							svc := types.ServiceInterface{}
							err := jsonencoding.Unmarshal([]byte(v), &svc)
							if err == nil {
								if svc.Headless != nil && svc.Origin == "" {
									m.headless[svc.Headless.Name] = svc
								}
							} else {
								log.Printf("Could not parse service definition for %s: %s", k, err)
							}
						}
						for k, v := range m.headless {
							_, ok := cm.Data[v.Address]
							if !ok {
								delete(m.headless, k)
							}
						}
					} else {
						m.headless = make(map[string]types.ServiceInterface)
					}
				}
			case "statefulsets":
				log.Printf("statefulset event for %s", name)
				obj, exists, err := m.statefulSetInformer.GetStore().GetByKey(name)
				if err != nil {
					return fmt.Errorf("Error reading statefulset %s from cache: %s", name, err)
				} else if exists {
					statefulset, ok := obj.(*appsv1.StatefulSet)
					if !ok {
						return fmt.Errorf("Expected StatefulSet for %s but got %#v", name, obj)
					}
					svc, ok := m.headless[statefulset.ObjectMeta.Name]
					if ok {
						if svc.Headless.Size != int(*statefulset.Spec.Replicas) {
							svc.Headless.Size = int(*statefulset.Spec.Replicas)
							changed := []types.ServiceInterface{
								svc,
							}
							deleted := []string{}
							kube.UpdateSkupperServices(changed, deleted, m.origin, m.vanClient.Namespace, m.vanClient.KubeClient)
						}
					}
				} else {
					_, unqualified, err := cache.SplitMetaNamespaceKey(name)
					if err != nil {
						return fmt.Errorf("Could not determine name of deleted statefulset from key %s: %w", name, err)
					}
					svc, ok := m.headless[unqualified]
					if ok {
						changed := []types.ServiceInterface{}
						deleted := []string{
							svc.Address,
						}
						kube.UpdateSkupperServices(changed, deleted, m.origin, m.vanClient.Namespace, m.vanClient.KubeClient)
					}
				}
			default:
				m.events.Forget(obj)
				return fmt.Errorf("unexpected event key %s (%s, %s)", key, category, name)
			}
			m.events.Forget(obj)
		}
		return nil
	}(obj)

	if err != nil {
		utilruntime.HandleError(err)
		return true
	}

	return true
}

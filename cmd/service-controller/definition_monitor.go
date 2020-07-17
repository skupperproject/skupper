package main

import (
	jsonencoding "encoding/json"
	"fmt"
	"log"
	"strconv"
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
	"github.com/skupperproject/skupper/pkg/utils"
)

// DefinitionMonitor updates skupper service definitions based on
// changes to other entities (currently statefulsets exposed via
// headless services)
type DefinitionMonitor struct {
	origin               string
	vanClient            *client.VanClient
	statefulSetInformer  cache.SharedIndexInformer
	deploymentInformer   cache.SharedIndexInformer
	svcDefInformer       cache.SharedIndexInformer
	svcInformer          cache.SharedIndexInformer
	events               workqueue.RateLimitingInterface
	headless             map[string]types.ServiceInterface
	annotated            map[string]types.ServiceInterface
	annotatedDeployments map[string]string
	annotatedServices    map[string]string
}

func newDefinitionMonitor(origin string, client *client.VanClient, svcDefInformer cache.SharedIndexInformer, svcInformer cache.SharedIndexInformer) *DefinitionMonitor {
	monitor := &DefinitionMonitor{
		origin:               origin,
		vanClient:            client,
		svcDefInformer:       svcDefInformer,
		svcInformer:          svcInformer,
		headless:             make(map[string]types.ServiceInterface),
		annotated:            make(map[string]types.ServiceInterface),
		annotatedDeployments: make(map[string]string),
		annotatedServices:    make(map[string]string),
	}
	monitor.statefulSetInformer = appsv1informer.NewStatefulSetInformer(
		client.KubeClient,
		client.Namespace,
		time.Second*30,
		cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
	monitor.deploymentInformer = appsv1informer.NewDeploymentInformer(
		client.KubeClient,
		client.Namespace,
		time.Second*30,
		cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
	monitor.events = workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "skupper-service-monitor")

	monitor.statefulSetInformer.AddEventHandler(newEventHandlerFor(monitor.events, "statefulsets", AnnotatedKey, StatefulSetResourceVersionTest))
	monitor.deploymentInformer.AddEventHandler(newEventHandlerFor(monitor.events, "deployments", AnnotatedKey, DeploymentResourceVersionTest))
	monitor.svcDefInformer.AddEventHandler(newEventHandlerFor(monitor.events, "servicedefs", AnnotatedKey, ConfigMapResourceVersionTest))
	monitor.svcInformer.AddEventHandler(newEventHandlerFor(monitor.events, "services", AnnotatedKey, ServiceResourceVersionTest))

	return monitor
}

func DeploymentResourceVersionTest(a interface{}, b interface{}) bool {
	aa := a.(*appsv1.Deployment)
	bb := b.(*appsv1.Deployment)
	return aa.ResourceVersion == bb.ResourceVersion
}

func (m *DefinitionMonitor) start(stopCh <-chan struct{}) error {
	go m.statefulSetInformer.Run(stopCh)
	go m.deploymentInformer.Run(stopCh)
	if ok := cache.WaitForCacheSync(stopCh, m.statefulSetInformer.HasSynced, m.deploymentInformer.HasSynced); !ok {
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

func deducePort(deployment *appsv1.Deployment) int {
	if port, ok := deployment.ObjectMeta.Annotations[types.PortQualifier]; ok {
		if iport, err := strconv.Atoi(port); err == nil {
			return iport
		} else {
			return 0
		}
	} else {
		return int(kube.GetContainerPort(deployment))
	}
}

func deducePortFromService(service *corev1.Service) int {
	if len(service.Spec.Ports) > 0 {
		return int(service.Spec.Ports[0].Port)
	}
	return 0
}

func deduceTargetPortFromService(service *corev1.Service) int {
	if len(service.Spec.Ports) > 0 {
		return service.Spec.Ports[0].TargetPort.IntValue()
	}
	return 0
}

func updateAnnotatedServiceDefinition(actual *types.ServiceInterface, desired *types.ServiceInterface) bool {
	if actual.Origin != "annotation" {
		return false
	}
	if actual.Protocol != desired.Protocol || actual.Port != desired.Port {
		return true
	}
	if len(actual.Targets) != len(desired.Targets) {
		return true
	}
	if len(desired.Targets) > 0 {
		if actual.Targets[0].Name != actual.Targets[0].Name || actual.Targets[0].Selector != actual.Targets[0].Selector {
			return true
		}
	}
	return false
}

func (m *DefinitionMonitor) getServiceDefinitionFromAnnotatedDeployment(deployment *appsv1.Deployment) (types.ServiceInterface, bool) {
	var svc types.ServiceInterface
	if protocol, ok := deployment.ObjectMeta.Annotations[types.ProxyQualifier]; ok {
		if port := deducePort(deployment); port != 0 {
			svc.Port = int(port)
		} else if protocol == "http" {
			svc.Port = 80
		} else {
			log.Printf("Ignoring annotated deployment %s; cannot deduce port", deployment.ObjectMeta.Name)
			return svc, false
		}
		svc.Protocol = protocol
		if address, ok := deployment.ObjectMeta.Annotations[types.AddressQualifier]; ok {
			svc.Address = address
		} else {
			svc.Address = deployment.ObjectMeta.Name
		}
		svc.Targets = []types.ServiceInterfaceTarget{
			types.ServiceInterfaceTarget{
				Name:     deployment.ObjectMeta.Name,
				Selector: utils.StringifySelector(deployment.Spec.Selector.MatchLabels),
			},
		}
		svc.Origin = "annotation"
		return svc, true
	} else {
		return svc, false
	}
}

func (m *DefinitionMonitor) getServiceDefinitionFromAnnotatedService(service *corev1.Service) (types.ServiceInterface, bool) {
	var svc types.ServiceInterface
	if protocol, ok := service.ObjectMeta.Annotations[types.ProxyQualifier]; ok {
		if port := deducePortFromService(service); port != 0 {
			svc.Port = int(port)
		} else if protocol == "http" {
			svc.Port = 80
		} else {
			log.Printf("Ignoring annotated service %s; cannot deduce port", service.ObjectMeta.Name)
			return svc, false
		}
		svc.Protocol = protocol
		if address, ok := service.ObjectMeta.Annotations[types.AddressQualifier]; ok {
			svc.Address = address
		} else {
			svc.Address = service.ObjectMeta.Name
		}
		target := types.ServiceInterfaceTarget{
			Name:     service.ObjectMeta.Name,
			Selector: utils.StringifySelector(service.Spec.Selector),
		}
		if targetPort := deduceTargetPortFromService(service); targetPort != 0 {
			target.TargetPort = targetPort
		}
		svc.Targets = []types.ServiceInterfaceTarget{
			target,
		}
		svc.Origin = "annotation"
		return svc, true
	} else {
		return svc, false
	}
}

func (m *DefinitionMonitor) deleteServiceDefinitionForAddress(address string) error {
	svc, ok := m.annotated[address]
	if ok {
		// delete the svc definition
		changed := []types.ServiceInterface{}
		deleted := []string{
			svc.Address,
		}
		return kube.UpdateSkupperServices(changed, deleted, "annotation", m.vanClient.Namespace, m.vanClient.KubeClient)
	}
	return nil
}

func (m *DefinitionMonitor) deleteServiceDefinitionForAnnotatedDeployment(name string) error {
	return m.deleteServiceDefinitionForAnnotatedObject(name, "deployment", m.annotatedDeployments)
}

func (m *DefinitionMonitor) deleteServiceDefinitionForAnnotatedService(name string) error {
	return m.deleteServiceDefinitionForAnnotatedObject(name, "service", m.annotatedServices)
}

func (m *DefinitionMonitor) deleteServiceDefinitionForAnnotatedObject(name string, objectType string, index map[string]string) error {
	address, ok := index[name]
	if ok {
		log.Printf("[DefMon] Deleting service definition for annotated %s %s", objectType, name)
		err := m.deleteServiceDefinitionForAddress(address)
		if err != nil {
			return err
		}
		delete(index, name)
	}
	return nil
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
								} else if svc.Origin == "annotation" {
									m.annotated[svc.Address] = svc
								}
							} else {
								log.Printf("[DefMon] Could not parse service definition for %s: %s", k, err)
							}
						}
						for k, v := range m.headless {
							_, ok := cm.Data[v.Address]
							if !ok {
								delete(m.headless, k)
							}
						}
						for k, v := range m.annotated {
							_, ok := cm.Data[v.Address]
							if !ok {
								delete(m.annotated, k)
							}
						}
					} else {
						m.headless = make(map[string]types.ServiceInterface)
					}
				}
			case "statefulsets":
				log.Printf("[DefMon] statefulset event for %s", name)
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
			case "deployments":
				log.Printf("[DefMon] deployment event for %s", name)
				obj, exists, err := m.deploymentInformer.GetStore().GetByKey(name)
				if err != nil {
					return fmt.Errorf("Error reading deployment %s from cache: %s", name, err)
				} else if exists {
					deployment, ok := obj.(*appsv1.Deployment)
					if !ok {
						return fmt.Errorf("Expected Deployment for %s but got %#v", name, obj)
					}

					desired, ok := m.getServiceDefinitionFromAnnotatedDeployment(deployment)
					if ok {
						log.Printf("[DefMon] Checking annotated deployment %s", name)
						actual, ok := m.annotated[desired.Address]
						if !ok || updateAnnotatedServiceDefinition(&actual, &desired) {
							log.Printf("[DefMon] Updating service definition for annotated deployment %s to %#v", name, desired)
							changed := []types.ServiceInterface{
								desired,
							}
							deleted := []string{}
							err = kube.UpdateSkupperServices(changed, deleted, "annotation", m.vanClient.Namespace, m.vanClient.KubeClient)
							if err != nil {
								return fmt.Errorf("failed to update service definition for annotated deployment %s: %s", name, err)
							}
						}
						address, ok := m.annotatedDeployments[name]
						if ok {
							if address != desired.Address {
								log.Printf("[DefMon] Address changed for annotated deployment %s. Was %s, now %s", name, address, desired.Address)
								if err := m.deleteServiceDefinitionForAddress(address); err != nil {
									return fmt.Errorf("Failed to delete stale service definition for %s", address)
								}
								m.annotatedDeployments[name] = desired.Address
							}
						} else {
							m.annotatedDeployments[name] = desired.Address
						}

					} else {
						err := m.deleteServiceDefinitionForAnnotatedDeployment(name)
						if err != nil {
							return fmt.Errorf("Failed to delete service definition on deployment %s which is no longer annotated: %s", name, err)
						}
					}
				} else {
					err := m.deleteServiceDefinitionForAnnotatedDeployment(name)
					if err != nil {
						return fmt.Errorf("Failed to delete service definition on removal of previously annotated deployment %s: %s", name, err)
					}
				}
			case "services":
				log.Printf("[DefMon] service event for %s", name)
				obj, exists, err := m.svcInformer.GetStore().GetByKey(name)
				if err != nil {
					return fmt.Errorf("Error reading service %s from cache: %s", name, err)
				} else if exists {
					service, ok := obj.(*corev1.Service)
					if !ok {
						return fmt.Errorf("Expected Service for %s but got %#v", name, obj)
					}

					desired, ok := m.getServiceDefinitionFromAnnotatedService(service)
					if ok {
						log.Printf("[DefMon] Checking annotated service %s", name)
						actual, ok := m.annotated[desired.Address]
						if !ok || updateAnnotatedServiceDefinition(&actual, &desired) {
							log.Printf("[DefMon] Updating service definition for annotated service %s to %#v", name, desired)
							changed := []types.ServiceInterface{
								desired,
							}
							deleted := []string{}
							err = kube.UpdateSkupperServices(changed, deleted, "annotation", m.vanClient.Namespace, m.vanClient.KubeClient)
							if err != nil {
								return fmt.Errorf("failed to update service definition for annotated service %s: %s", name, err)
							}
						}
						address, ok := m.annotatedServices[name]
						if ok {
							if address != desired.Address {
								log.Printf("[DefMon] Address changed for annotated service %s. Was %s, now %s", name, address, desired.Address)
								if err := m.deleteServiceDefinitionForAddress(address); err != nil {
									return fmt.Errorf("Failed to delete stale service definition for %s", address)
								}
								m.annotatedServices[name] = desired.Address
							}
						} else {
							m.annotatedServices[name] = desired.Address
						}

					} else {
						err := m.deleteServiceDefinitionForAnnotatedService(name)
						if err != nil {
							return fmt.Errorf("Failed to delete service definition on service %s which is no longer annotated: %s", name, err)
						}
					}
				} else {
					err := m.deleteServiceDefinitionForAnnotatedService(name)
					if err != nil {
						return fmt.Errorf("Failed to delete service definition on removal of previously annotated service %s: %s", name, err)
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

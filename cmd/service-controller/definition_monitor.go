package main

import (
	jsonencoding "encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	appsv1informer "k8s.io/client-go/informers/apps/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/client"
	"github.com/skupperproject/skupper/pkg/event"
	"github.com/skupperproject/skupper/pkg/kube"
	"github.com/skupperproject/skupper/pkg/utils"
)

// DefinitionMonitor updates skupper service definitions based on
// changes to other entities (currently statefulsets exposed via
// headless services)
type DefinitionMonitor struct {
	origin                string
	vanClient             *client.VanClient
	policy                *client.ClusterPolicyValidator
	statefulSetInformer   cache.SharedIndexInformer
	daemonSetInformer     cache.SharedIndexInformer
	deploymentInformer    cache.SharedIndexInformer
	svcDefInformer        cache.SharedIndexInformer
	svcInformer           cache.SharedIndexInformer
	events                workqueue.RateLimitingInterface
	headless              map[string]types.ServiceInterface
	annotated             map[string]types.ServiceInterface
	annotatedDeployments  map[string]string
	annotatedStatefulSets map[string]string
	annotatedDaemonSets   map[string]string
	annotatedServices     map[string]string
}

const (
	DefinitionMonitorIgnored       string = "DefinitionMonitorIgnored"
	DefinitionMonitorEvent         string = "DefinitionMonitorEvent"
	DefinitionMonitorError         string = "DefinitionMonitorEvent"
	DefinitionMonitorDeletionEvent string = "DefinitionMonitorDeletionEvent"
	DefinitionMonitorUpdateEvent   string = "DefinitionMonitorUpdateEvent"
)

func newDefinitionMonitor(origin string, cli *client.VanClient, svcDefInformer cache.SharedIndexInformer, svcInformer cache.SharedIndexInformer) *DefinitionMonitor {
	monitor := &DefinitionMonitor{
		origin:                origin,
		vanClient:             cli,
		policy:                client.NewClusterPolicyValidator(cli),
		svcDefInformer:        svcDefInformer,
		svcInformer:           svcInformer,
		headless:              make(map[string]types.ServiceInterface),
		annotated:             make(map[string]types.ServiceInterface),
		annotatedDeployments:  make(map[string]string),
		annotatedStatefulSets: make(map[string]string),
		annotatedDaemonSets:   make(map[string]string),
		annotatedServices:     make(map[string]string),
	}
	AddStaticPolicyWatcher(monitor.policy)
	monitor.statefulSetInformer = appsv1informer.NewStatefulSetInformer(
		cli.KubeClient,
		cli.Namespace,
		time.Second*30,
		cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
	monitor.daemonSetInformer = appsv1informer.NewDaemonSetInformer(
		cli.KubeClient,
		cli.Namespace,
		time.Second*30,
		cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
	monitor.deploymentInformer = appsv1informer.NewDeploymentInformer(
		cli.KubeClient,
		cli.Namespace,
		time.Second*30,
		cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
	monitor.events = workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "skupper-service-monitor")

	monitor.statefulSetInformer.AddEventHandler(newEventHandlerFor(monitor.events, "statefulsets", AnnotatedKey, StatefulSetResourceVersionTest))
	monitor.daemonSetInformer.AddEventHandler(newEventHandlerFor(monitor.events, "daemonsets", AnnotatedKey, DaemonSetResourceVersionTest))
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

func DaemonSetResourceVersionTest(a interface{}, b interface{}) bool {
	aa := a.(*appsv1.DaemonSet)
	bb := b.(*appsv1.DaemonSet)
	return aa.ResourceVersion == bb.ResourceVersion
}

func (m *DefinitionMonitor) start(stopCh <-chan struct{}) error {
	go m.statefulSetInformer.Run(stopCh)
	go m.daemonSetInformer.Run(stopCh)
	go m.deploymentInformer.Run(stopCh)
	if ok := cache.WaitForCacheSync(stopCh, m.statefulSetInformer.HasSynced, m.daemonSetInformer.HasSynced, m.deploymentInformer.HasSynced); !ok {
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

func deducePort(deployment *appsv1.Deployment) map[int]int {
	if port, ok := deployment.ObjectMeta.Annotations[types.PortQualifier]; ok {
		return kube.PortLabelStrToMap(port)
	} else {
		return kube.GetContainerPort(deployment)
	}
}

func deducePortFromStatefulSet(statefulSet *appsv1.StatefulSet) map[int]int {
	if port, ok := statefulSet.ObjectMeta.Annotations[types.PortQualifier]; ok {
		return kube.PortLabelStrToMap(port)
	} else {
		return kube.GetContainerPortForStatefulSet(statefulSet)
	}
}

func deducePortFromDaemonSet(daemonSet *appsv1.DaemonSet) map[int]int {
	if port, ok := daemonSet.ObjectMeta.Annotations[types.PortQualifier]; ok {
		return kube.PortLabelStrToMap(port)
	} else {
		return kube.GetContainerPortForDaemonSet(daemonSet)
	}
}

func deducePortFromService(service *corev1.Service) map[int]int {
	if len(service.Spec.Ports) > 0 {
		return kube.GetServicePortMap(service)
	}
	return map[int]int{}
}

func updateAnnotatedServiceDefinition(actual *types.ServiceInterface, desired *types.ServiceInterface) bool {
	if actual.Origin != "annotation" {
		return false
	}
	if actual.Protocol != desired.Protocol || !reflect.DeepEqual(actual.Ports, desired.Ports) {
		return true
	}
	if len(actual.Targets) != len(desired.Targets) {
		return true
	}
	if len(desired.Targets) > 0 {
		nameChanged := actual.Targets[0].Name != desired.Targets[0].Name
		selectorChanged := actual.Targets[0].Selector != desired.Targets[0].Selector
		targetPortChanged := !reflect.DeepEqual(actual.Targets[0].TargetPorts, desired.Targets[0].TargetPorts)
		if nameChanged || selectorChanged || targetPortChanged {
			return true
		}
	}
	return false
}

func (m *DefinitionMonitor) getServiceDefinitionFromAnnotatedDeployment(deployment *appsv1.Deployment) (types.ServiceInterface, bool) {
	var svc types.ServiceInterface
	if protocol, ok := deployment.ObjectMeta.Annotations[types.ProxyQualifier]; ok {
		port := map[int]int{}
		if port = deducePort(deployment); len(port) > 0 {
			svc.Ports = []int{}
			for p, _ := range port {
				svc.Ports = append(svc.Ports, p)
			}
		} else if protocol == "http" {
			svc.Ports = []int{80}
		} else {
			event.Recordf(DefinitionMonitorIgnored, "Ignoring annotated deployment %s; cannot deduce port", deployment.ObjectMeta.Name)
			return svc, false
		}
		svc.Protocol = protocol
		if address, ok := deployment.ObjectMeta.Annotations[types.AddressQualifier]; ok {
			svc.Address = address
		} else {
			svc.Address = deployment.ObjectMeta.Name
		}

		selector := ""
		if deployment.Spec.Selector != nil {
			selector = utils.StringifySelector(deployment.Spec.Selector.MatchLabels)
		}
		svc.Targets = []types.ServiceInterfaceTarget{
			types.ServiceInterfaceTarget{
				Name:     deployment.ObjectMeta.Name,
				Selector: selector,
			},
		}
		if len(port) > 0 {
			svc.Targets[0].TargetPorts = port
		}
		if labels, ok := deployment.ObjectMeta.Annotations[types.ServiceLabels]; ok {
			svc.Labels = utils.LabelToMap(labels)
		}
		svc.Origin = "annotation"

		if policyRes := m.policy.ValidateExpose("deployment", deployment.Name); !policyRes.Allowed() {
			event.Recordf(DefinitionMonitorIgnored, "Policy validation error: deployment/%s cannot be exposed", deployment.ObjectMeta.Name)
			return types.ServiceInterface{}, false
		}
		if policyRes := m.policy.ValidateImportService(svc.Address); !policyRes.Allowed() {
			event.Recordf(DefinitionMonitorIgnored, "Policy validation error: service %s cannot be created", svc.Address)
			return types.ServiceInterface{}, false
		}

		return svc, true
	} else {
		return svc, false
	}
}

func (m *DefinitionMonitor) getServiceDefinitionFromAnnotatedStatefulSet(statefulset *appsv1.StatefulSet) (types.ServiceInterface, bool) {
	var svc types.ServiceInterface
	if protocol, ok := statefulset.ObjectMeta.Annotations[types.ProxyQualifier]; ok {
		port := map[int]int{}
		if port = deducePortFromStatefulSet(statefulset); len(port) > 0 {
			svc.Ports = []int{}
			for p, _ := range port {
				svc.Ports = append(svc.Ports, p)
			}
		} else if protocol == "http" {
			svc.Ports = []int{80}
		} else {
			event.Recordf(DefinitionMonitorIgnored, "Ignoring annotated statefulset %s; cannot deduce port", statefulset.ObjectMeta.Name)
			return svc, false
		}
		svc.Protocol = protocol
		if address, ok := statefulset.ObjectMeta.Annotations[types.AddressQualifier]; ok {
			svc.Address = address
		} else {
			svc.Address = statefulset.ObjectMeta.Name
		}

		selector := ""
		if statefulset.Spec.Selector != nil {
			selector = utils.StringifySelector(statefulset.Spec.Selector.MatchLabels)
		}
		svc.Targets = []types.ServiceInterfaceTarget{
			types.ServiceInterfaceTarget{
				Name:     statefulset.ObjectMeta.Name,
				Selector: selector,
			},
		}
		if len(port) > 0 {
			svc.Targets[0].TargetPorts = port
		}
		svc.Origin = "annotation"

		if policyRes := m.policy.ValidateExpose("statefulset", statefulset.Name); !policyRes.Allowed() {
			event.Recordf(DefinitionMonitorIgnored, "Policy validation error: statefulset/%s cannot be exposed", statefulset.ObjectMeta.Name)
			return types.ServiceInterface{}, false
		}
		if policyRes := m.policy.ValidateImportService(svc.Address); !policyRes.Allowed() {
			event.Recordf(DefinitionMonitorIgnored, "Policy validation error: service %s cannot be created", svc.Address)
			return types.ServiceInterface{}, false
		}

		return svc, true
	} else {
		return svc, false
	}
}

func (m *DefinitionMonitor) getServiceDefinitionFromAnnotatedDaemonSet(daemonset *appsv1.DaemonSet) (types.ServiceInterface, bool) {
	var svc types.ServiceInterface
	if protocol, ok := daemonset.ObjectMeta.Annotations[types.ProxyQualifier]; ok {
		port := map[int]int{}
		if port = deducePortFromDaemonSet(daemonset); len(port) > 0 {
			svc.Ports = []int{}
			for p, _ := range port {
				svc.Ports = append(svc.Ports, p)
			}
		} else if protocol == "http" {
			svc.Ports = []int{80}
		} else {
			event.Recordf(DefinitionMonitorIgnored, "Ignoring annotated daemonset %s; cannot deduce port", daemonset.ObjectMeta.Name)
			return svc, false
		}
		svc.Protocol = protocol
		if address, ok := daemonset.ObjectMeta.Annotations[types.AddressQualifier]; ok {
			svc.Address = address
		} else {
			svc.Address = daemonset.ObjectMeta.Name
		}

		selector := ""
		if daemonset.Spec.Selector != nil {
			selector = utils.StringifySelector(daemonset.Spec.Selector.MatchLabels)
		}
		svc.Targets = []types.ServiceInterfaceTarget{
			types.ServiceInterfaceTarget{
				Name:     daemonset.ObjectMeta.Name,
				Selector: selector,
			},
		}
		if len(port) > 0 {
			svc.Targets[0].TargetPorts = port
		}
		if labels, ok := daemonset.ObjectMeta.Annotations[types.ServiceLabels]; ok {
			svc.Labels = utils.LabelToMap(labels)
		}
		svc.Origin = "annotation"

		if policyRes := m.policy.ValidateExpose("daemonset", daemonset.Name); !policyRes.Allowed() {
			event.Recordf(DefinitionMonitorIgnored, "Policy validation error: daemonset/%s cannot be exposed", daemonset.ObjectMeta.Name)
			return types.ServiceInterface{}, false
		}
		if policyRes := m.policy.ValidateImportService(svc.Address); !policyRes.Allowed() {
			event.Recordf(DefinitionMonitorIgnored, "Policy validation error: service %s cannot be created", svc.Address)
			return types.ServiceInterface{}, false
		}

		return svc, true
	} else {
		return svc, false
	}
}

func (m *DefinitionMonitor) getServiceDefinitionFromAnnotatedService(service *corev1.Service) (types.ServiceInterface, bool) {
	var svc types.ServiceInterface
	if protocol, ok := service.ObjectMeta.Annotations[types.ProxyQualifier]; ok {
		svc.Ports = []int{}
		if port := deducePortFromService(service); len(port) > 0 {
			for p, _ := range port {
				svc.Ports = append(svc.Ports, p)
			}
		}
		svc.Protocol = protocol
		if address, ok := service.ObjectMeta.Annotations[types.AddressQualifier]; ok {
			svc.Address = address
		} else {
			svc.Address = service.ObjectMeta.Name
		}
		if target, ok := service.ObjectMeta.Annotations[types.TargetServiceQualifier]; ok {
			port, err := kube.GetPortsForServiceTarget(target, m.vanClient.Namespace, m.vanClient.KubeClient)
			if err != nil {
				event.Recordf(DefinitionMonitorError, "Could not deduce port for target service %s on annotated service %s: %s", target, service.ObjectMeta.Name, err)
			}
			if len(svc.Ports) == 0 {
				if len(port) > 0 {
					svc.Ports = []int{}
					for p, _ := range port {
						svc.Ports = append(svc.Ports, p)
					}
				} else if protocol == "http" {
					svc.Ports = []int{80}
				} else {
					event.Recordf(DefinitionMonitorIgnored, "Ignoring annotated service %s; cannot deduce port", service.ObjectMeta.Name)
					return svc, false
				}
			}
			svcTgt := types.ServiceInterfaceTarget{
				Name:    target,
				Service: target,
			}
			svcPorts := map[int]int{}
			for _, p := range svc.Ports {
				svcPorts[p] = p
			}
			if len(port) == 1 && len(svc.Ports) == 1 && !reflect.DeepEqual(port, svcPorts) {
				svcTgt.TargetPorts = map[int]int{}
				for sPort, _ := range port {
					svcTgt.TargetPorts[svc.Ports[0]] = sPort
				}
			} else if len(svc.Ports) >= 1 && reflect.DeepEqual(port, svcPorts) {
				svcTgt.TargetPorts = port
			} else if len(port) > 0 {
				// if target service has multiple ports but ports do not match
				event.Recordf(DefinitionMonitorIgnored, "Ignoring annotated service %s; incompatible number of ports", service.ObjectMeta.Name)
				return svc, false
			}
			svc.Targets = []types.ServiceInterfaceTarget{
				svcTgt,
			}
		} else if service.Spec.Selector != nil {
			if len(svc.Ports) == 0 {
				if protocol == "http" {
					svc.Ports = []int{80}
				} else {
					event.Recordf(DefinitionMonitorIgnored, "Ignoring annotated service %s; cannot deduce port", service.ObjectMeta.Name)
					return svc, false
				}
			}

			// By default we use what is defined in the service itself as the
			// selector, but if the service is already exposed (and pointing to
			// the router) and the original selector annotation is available,
			// use it instead so that the target will be the correct endpoint.
			svcSelector := getApplicationSelector(service)
			if hasRouterSelector(*service) && hasOriginalSelector(*service) {
				svcSelector = service.Annotations[types.OriginalSelectorQualifier]
			}
			if svcSelector == "" {
				event.Recordf(DefinitionMonitorIgnored, "Ignoring annotated service %s; cannot deduce selector", service.ObjectMeta.Name)
				return svc, false
			}
			target := types.ServiceInterfaceTarget{
				Name:     service.ObjectMeta.Name,
				Selector: svcSelector,
			}
			if !hasOriginalTargetPort(*service) {
				// If getting target port from new annotated service, deduce target port from service
				if targetPort := deducePortFromService(service); len(targetPort) > 0 {
					target.TargetPorts = targetPort
				}
			} else {
				// If getting target port from previously annotated service, deduce target port from existing annotation
				// as in this case the target port might have been already modified
				originalTargetPort, _ := service.Annotations[types.OriginalTargetPortQualifier]
				target.TargetPorts = kube.PortLabelStrToMap(originalTargetPort)
			}
			svc.Targets = []types.ServiceInterfaceTarget{
				target,
			}
		} else {
			event.Recordf(DefinitionMonitorIgnored, "Ignoring annotated service %s; no selector defined", service.ObjectMeta.Name)
			return svc, false
		}
		if labels, ok := service.ObjectMeta.Annotations[types.ServiceLabels]; ok {
			svc.Labels = utils.LabelToMap(labels)
		}
		svc.Origin = "annotation"

		if policyRes := m.policy.ValidateExpose("service", service.Name); !policyRes.Allowed() {
			event.Recordf(DefinitionMonitorIgnored, "Policy validation error: service/%s cannot be exposed", service.ObjectMeta.Name)
			return types.ServiceInterface{}, false
		}
		if policyRes := m.policy.ValidateImportService(svc.Address); !policyRes.Allowed() {
			event.Recordf(DefinitionMonitorIgnored, "Policy validation error: service %s cannot be created", svc.Address)
			return types.ServiceInterface{}, false
		}

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

func (m *DefinitionMonitor) deleteServiceDefinitionForAnnotatedStatefulSet(name string) error {
	return m.deleteServiceDefinitionForAnnotatedObject(name, "statefulset", m.annotatedStatefulSets)
}

func (m *DefinitionMonitor) deleteServiceDefinitionForAnnotatedDaemonSet(name string) error {
	return m.deleteServiceDefinitionForAnnotatedObject(name, "daemonset", m.annotatedDaemonSets)
}

func (m *DefinitionMonitor) deleteServiceDefinitionForAnnotatedService(name string) error {
	return m.deleteServiceDefinitionForAnnotatedObject(name, "service", m.annotatedServices)
}

func (m *DefinitionMonitor) deleteServiceDefinitionForAnnotatedObject(name string, objectType string, index map[string]string) error {
	address, ok := index[name]
	if ok {
		event.Recordf(DefinitionMonitorDeletionEvent, "Deleting service definition for annotated %s %s", objectType, name)
		err := m.deleteServiceDefinitionForAddress(address)
		if err != nil {
			return err
		}
		delete(index, name)
	}
	return nil
}

func (m *DefinitionMonitor) restoreServiceDefinitions(service *corev1.Service) error {
	updated := false
	if hasOriginalSelector(*service) {
		updated = true
		originalSelector := service.ObjectMeta.Annotations[types.OriginalSelectorQualifier]
		delete(service.ObjectMeta.Annotations, types.OriginalSelectorQualifier)
		service.Spec.Selector = utils.LabelToMap(originalSelector)
	}
	if hasOriginalTargetPort(*service) {
		updated = true
		originalTargetPort, _ := strconv.Atoi(service.ObjectMeta.Annotations[types.OriginalTargetPortQualifier])
		delete(service.ObjectMeta.Annotations, types.OriginalTargetPortQualifier)
		service.Spec.Ports[0].TargetPort = intstr.FromInt(originalTargetPort)
	}
	if hasOriginalAssigned(*service) {
		updated = true
		delete(service.ObjectMeta.Annotations, types.OriginalAssignedQualifier)
	}
	if updated {
		_, err := m.vanClient.KubeClient.CoreV1().Services(m.vanClient.Namespace).Update(service)
		return err
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
				event.Recordf(DefinitionMonitorEvent, "Service definitions have changed")
				// get the configmap, parse the json, check against the current servicebindings map
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
								event.Recordf(DefinitionMonitorError, "Could not parse service definition for %s: %s", k, err)
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
						m.annotated = make(map[string]types.ServiceInterface)
					}
				}
			case "statefulsets":
				event.Recordf(DefinitionMonitorEvent, "statefulset event for %s", name)
				obj, exists, err := m.statefulSetInformer.GetStore().GetByKey(name)
				if err != nil {
					return fmt.Errorf("Error reading statefulset %s from cache: %s", name, err)
				} else if exists {
					statefulset, ok := obj.(*appsv1.StatefulSet)
					if !ok {
						return fmt.Errorf("Expected StatefulSet for %s but got %#v", name, obj)
					}
					// is this statefulset one that has been exposed with the headless option?
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
					} else {
						// does it have a skupper annotation?
						if desired, ok := m.getServiceDefinitionFromAnnotatedStatefulSet(statefulset); ok {
							event.Recordf(DefinitionMonitorEvent, "Checking annotated statefulSet %s", name)
							actual, ok := m.annotated[desired.Address]
							if !ok || updateAnnotatedServiceDefinition(&actual, &desired) {
								event.Recordf(DefinitionMonitorUpdateEvent, "Updating service definition for annotated statefulSet %s to %#v", name, desired)
								changed := []types.ServiceInterface{
									desired,
								}
								deleted := []string{}
								err = kube.UpdateSkupperServices(changed, deleted, "annotation", m.vanClient.Namespace, m.vanClient.KubeClient)
								if err != nil {
									return fmt.Errorf("failed to update service definition for annotated statefulSet %s: %s", name, err)
								}
							}
							if address, ok := m.annotatedStatefulSets[name]; ok {
								if address != desired.Address {
									event.Recordf(DefinitionMonitorUpdateEvent, "Address changed for annotated statefulSet %s. Was %s, now %s", name, address, desired.Address)
									if err := m.deleteServiceDefinitionForAddress(address); err != nil {
										return fmt.Errorf("Failed to delete stale service definition for %s", address)
									}
									m.annotatedStatefulSets[name] = desired.Address
								}
							} else {
								m.annotatedStatefulSets[name] = desired.Address
							}

						} else {
							err := m.deleteServiceDefinitionForAnnotatedStatefulSet(name)
							if err != nil {
								return fmt.Errorf("Failed to delete service definition on statefulset %s which is no longer annotated: %s", name, err)
							}
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
					} else {
						err := m.deleteServiceDefinitionForAnnotatedStatefulSet(name)
						if err != nil {
							return fmt.Errorf("Failed to delete service definition on statefulset %s which is no longer annotated: %s", name, err)
						}
					}
				}
			case "deployments":
				event.Recordf(DefinitionMonitorEvent, "deployment event for %s", name)
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
						event.Recordf(DefinitionMonitorEvent, "Checking annotated deployment %s", name)
						actual, ok := m.annotated[desired.Address]
						if !ok || updateAnnotatedServiceDefinition(&actual, &desired) {
							event.Recordf(DefinitionMonitorUpdateEvent, "Updating service definition for annotated deployment %s to %#v", name, desired)
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
								event.Recordf(DefinitionMonitorUpdateEvent, "Address changed for annotated deployment %s. Was %s, now %s", name, address, desired.Address)
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
			case "daemonsets":
				event.Recordf(DefinitionMonitorEvent, "daemonset event for %s", name)
				obj, exists, err := m.daemonSetInformer.GetStore().GetByKey(name)
				if err != nil {
					return fmt.Errorf("Error reading daemonset %s from cache: %s", name, err)
				} else if exists {
					daemonSet, ok := obj.(*appsv1.DaemonSet)
					if !ok {
						return fmt.Errorf("Expected DaemonSet for %s but got %#v", name, obj)
					}

					desired, ok := m.getServiceDefinitionFromAnnotatedDaemonSet(daemonSet)
					if ok {
						event.Recordf(DefinitionMonitorEvent, "Checking annotated daemonset %s", name)
						actual, ok := m.annotated[desired.Address]
						if !ok || updateAnnotatedServiceDefinition(&actual, &desired) {
							event.Recordf(DefinitionMonitorUpdateEvent, "Updating service definition for annotated daemonset %s to %#v", name, desired)
							changed := []types.ServiceInterface{
								desired,
							}
							deleted := []string{}
							err = kube.UpdateSkupperServices(changed, deleted, "annotation", m.vanClient.Namespace, m.vanClient.KubeClient)
							if err != nil {
								return fmt.Errorf("failed to update service definition for annotated daemonset %s: %s", name, err)
							}
						}
						address, ok := m.annotatedDaemonSets[name]
						if ok {
							if address != desired.Address {
								event.Recordf(DefinitionMonitorUpdateEvent, "Address changed for annotated daemonset %s. Was %s, now %s", name, address, desired.Address)
								if err := m.deleteServiceDefinitionForAddress(address); err != nil {
									return fmt.Errorf("Failed to delete stale service definition for %s", address)
								}
								m.annotatedDaemonSets[name] = desired.Address
							}
						} else {
							m.annotatedDaemonSets[name] = desired.Address
						}

					} else {
						err := m.deleteServiceDefinitionForAnnotatedDaemonSet(name)
						if err != nil {
							return fmt.Errorf("Failed to delete service definition on daemonset %s which is no longer annotated: %s", name, err)
						}
					}
				} else {
					err := m.deleteServiceDefinitionForAnnotatedDaemonSet(name)
					if err != nil {
						return fmt.Errorf("Failed to delete service definition on removal of previously annotated daemonset %s: %s", name, err)
					}
				}
			case "services":
				event.Recordf(DefinitionMonitorEvent, "service event for %s", name)
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
						event.Recordf(DefinitionMonitorEvent, "Checking annotated service %s", name)
						actual, ok := m.annotated[desired.Address]
						if !ok || updateAnnotatedServiceDefinition(&actual, &desired) {
							event.Recordf(DefinitionMonitorUpdateEvent, "Updating service definition for annotated service %s to %#v", name, desired)
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
								event.Recordf(DefinitionMonitorUpdateEvent, "Address changed for annotated service %s. Was %s, now %s", name, address, desired.Address)
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
						err = m.restoreServiceDefinitions(service)
						if err != nil {
							return fmt.Errorf("Failed to restore service definitions on service %s: %s", name, err)
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
		if m.events.NumRequeues(obj) < 5 {
			event.Recordf(DefinitionMonitorError, "Requeuing %v after error: %v", obj, err)
			m.events.AddRateLimited(obj)
		} else {
			event.Recordf(DefinitionMonitorError, "Giving up on %v after error: %v", obj, err)
		}
		utilruntime.HandleError(err)
		return true
	}

	return true
}

package main

import (
	jsonencoding "encoding/json"
	"fmt"
	appv1 "github.com/openshift/api/apps/v1"
	v1 "github.com/openshift/client-go/apps/informers/externalversions/apps/v1"
	"reflect"
	"strconv"
	"strings"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	origin                   string
	vanClient                *client.VanClient
	policy                   *client.ClusterPolicyValidator
	statefulSetInformer      cache.SharedIndexInformer
	daemonSetInformer        cache.SharedIndexInformer
	deploymentInformer       cache.SharedIndexInformer
	deploymentConfigInformer cache.SharedIndexInformer
	svcDefInformer           cache.SharedIndexInformer
	svcInformer              cache.SharedIndexInformer
	events                   workqueue.RateLimitingInterface
	headless                 map[string]types.ServiceInterface
	annotated                map[string]types.ServiceInterface
	annotatedObjects         map[objectKey]string
}

type objectKey struct {
	name       string
	objectType string
}

const (
	DefinitionMonitorIgnored       string = "DefinitionMonitorIgnored"
	DefinitionMonitorEvent         string = "DefinitionMonitorEvent"
	DefinitionMonitorError         string = "DefinitionMonitorEvent"
	DefinitionMonitorDeletionEvent string = "DefinitionMonitorDeletionEvent"
	DefinitionMonitorUpdateEvent   string = "DefinitionMonitorUpdateEvent"
	ServiceObjectType              string = "service"
	DaemonSetObjectType            string = "daemonset"
	DeploymentObjectType           string = "deployment"
	DeploymentConfigObjectType     string = "deploymentconfig"
	StatefulSetObjectType          string = "statefulset"
)

func newDefinitionMonitor(origin string, cli *client.VanClient, svcDefInformer cache.SharedIndexInformer, svcInformer cache.SharedIndexInformer) *DefinitionMonitor {
	monitor := &DefinitionMonitor{
		origin:           origin,
		vanClient:        cli,
		policy:           client.NewClusterPolicyValidator(cli),
		svcDefInformer:   svcDefInformer,
		svcInformer:      svcInformer,
		headless:         make(map[string]types.ServiceInterface),
		annotated:        make(map[string]types.ServiceInterface),
		annotatedObjects: make(map[objectKey]string),
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

	if cli.OCAppsClient != nil {
		monitor.deploymentConfigInformer = v1.NewDeploymentConfigInformer(cli.OCAppsClient, cli.Namespace, time.Second*30, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
		monitor.deploymentConfigInformer.AddEventHandler(newEventHandlerFor(monitor.events, "deploymentconfigs", AnnotatedKey, DeploymentConfigResourceVersionTest))
	}

	return monitor
}

func DeploymentResourceVersionTest(a interface{}, b interface{}) bool {
	aa := a.(*appsv1.Deployment)
	bb := b.(*appsv1.Deployment)
	return aa.ResourceVersion == bb.ResourceVersion
}

func DeploymentConfigResourceVersionTest(a interface{}, b interface{}) bool {
	aa := a.(*appv1.DeploymentConfig)
	bb := b.(*appv1.DeploymentConfig)
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

	if m.deploymentConfigInformer != nil {
		go m.deploymentConfigInformer.Run(stopCh)
	}

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

func deducePortFromDeploymentConfig(deploymentConfig *appv1.DeploymentConfig) map[int]int {
	if port, ok := deploymentConfig.ObjectMeta.Annotations[types.PortQualifier]; ok {
		return kube.PortLabelStrToMap(port)
	} else {
		return kube.GetContainerPortForDeploymentConfig(deploymentConfig)
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
	if actual.Headless != desired.Headless {
		return true
	}
	targets := map[string]types.ServiceInterfaceTarget{}
	updated := false
	for _, target := range actual.Targets {
		targets[target.Name] = target
	}
	for _, target := range desired.Targets {
		existing, ok := targets[target.Name]
		if !ok || !reflect.DeepEqual(target, existing) {
			targets[target.Name] = target
			updated = true
		}
	}
	if !reflect.DeepEqual(actual.Labels, desired.Labels) || !reflect.DeepEqual(actual.Annotations, desired.Annotations) {
		updated = true
	}
	if !updated {
		return false
	}
	desired.Targets = []types.ServiceInterfaceTarget{}
	for _, target := range targets {
		desired.Targets = append(desired.Targets, target)
	}
	return true
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

		if tlsCert, ok := deployment.ObjectMeta.Annotations[types.TlsCertQualifier]; ok {
			if protocol == "http" {
				event.Recordf(DefinitionMonitorIgnored, "Ignoring annotated deployment %s; cannot enable TLS with http protocol", deployment.ObjectMeta.Name)
				return svc, false
			}

			svc.TlsCredentials = tlsCert
		}

		if tlsTrust, ok := deployment.ObjectMeta.Annotations[types.TlsTrustQualifier]; ok {
			if protocol == "http" {
				event.Recordf(DefinitionMonitorIgnored, "Ignoring annotated deployment %s; cannot enable TLS with http protocol", deployment.ObjectMeta.Name)
				return svc, false
			}

			svc.TlsCertAuthority = tlsTrust
		}

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
		if annotations, ok := deployment.ObjectMeta.Annotations[types.ServiceAnnotations]; ok {
			svc.Annotations = utils.LabelToMap(annotations)
		}
		if ingressMode, ok := deployment.ObjectMeta.Annotations[types.IngressModeQualifier]; ok {
			err := svc.SetIngressMode(ingressMode)
			if err != nil {
				event.Recordf(DefinitionMonitorIgnored, "Ignoring invalid annotation %s: %s", types.IngressModeQualifier, err)
			}
		}
		svc.Origin = "annotation"

		if policyRes := m.policy.ValidateExpose(DeploymentObjectType, deployment.Name); !policyRes.Allowed() {
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

func (m *DefinitionMonitor) getServiceDefinitionFromAnnotatedDeploymentConfig(deploymentConfig *appv1.DeploymentConfig) (types.ServiceInterface, bool) {
	var svc types.ServiceInterface
	if protocol, ok := deploymentConfig.ObjectMeta.Annotations[types.ProxyQualifier]; ok {
		port := map[int]int{}
		if port = deducePortFromDeploymentConfig(deploymentConfig); len(port) > 0 {
			svc.Ports = []int{}
			for p, _ := range port {
				svc.Ports = append(svc.Ports, p)
			}
		} else if protocol == "http" {
			svc.Ports = []int{80}
		} else {
			event.Recordf(DefinitionMonitorIgnored, "Ignoring annotated deploymentconfig %s; cannot deduce port", deploymentConfig.ObjectMeta.Name)
			return svc, false
		}
		svc.Protocol = protocol
		if address, ok := deploymentConfig.ObjectMeta.Annotations[types.AddressQualifier]; ok {
			svc.Address = address
		} else {
			svc.Address = deploymentConfig.ObjectMeta.Name
		}

		selector := ""
		if deploymentConfig.Spec.Selector != nil {
			selector = utils.StringifySelector(deploymentConfig.Spec.Selector)
		}
		svc.Targets = []types.ServiceInterfaceTarget{
			{
				Name:     deploymentConfig.ObjectMeta.Name,
				Selector: selector,
			},
		}
		if len(port) > 0 {
			svc.Targets[0].TargetPorts = port
		}
		if labels, ok := deploymentConfig.ObjectMeta.Annotations[types.ServiceLabels]; ok {
			svc.Labels = utils.LabelToMap(labels)
		}
		svc.Origin = "annotation"

		if policyRes := m.policy.ValidateExpose(DeploymentConfigObjectType, deploymentConfig.Name); !policyRes.Allowed() {
			event.Recordf(DefinitionMonitorIgnored, "Policy validation error: deploymentconfig/%s cannot be exposed", deploymentConfig.ObjectMeta.Name)
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
		if headless, ok := statefulset.ObjectMeta.Annotations[types.HeadlessQualifier]; ok && strings.EqualFold(headless, "true") {
			svc.Headless = &types.Headless{
				Name: statefulset.ObjectMeta.Name,
				Size: int(*statefulset.Spec.Replicas),
			}
			if cpu, ok := statefulset.ObjectMeta.Annotations[types.CpuRequestAnnotation]; ok {
				quantity, err := resource.ParseQuantity(cpu)
				if err != nil {
					event.Recordf(DefinitionMonitorError, "Invalid cpu annotation on statefulset %s: %s", statefulset.ObjectMeta.Name, err)
				} else {
					svc.Headless.CpuRequest = &quantity
				}
			}
			if memory, ok := statefulset.ObjectMeta.Annotations[types.MemoryRequestAnnotation]; ok {
				quantity, err := resource.ParseQuantity(memory)
				if err != nil {
					event.Recordf(DefinitionMonitorError, "Invalid memory annotation on statefulset %s: %s", statefulset.ObjectMeta.Name, err)
				} else {
					svc.Headless.MemoryRequest = &quantity
				}
			}
			if affinity, ok := statefulset.ObjectMeta.Annotations[types.AffinityAnnotation]; ok {
				svc.Headless.Affinity = utils.LabelToMap(affinity)
			}
			if antiAffinity, ok := statefulset.ObjectMeta.Annotations[types.AntiAffinityAnnotation]; ok {
				svc.Headless.AntiAffinity = utils.LabelToMap(antiAffinity)
			}
			if nodeSelector, ok := statefulset.ObjectMeta.Annotations[types.NodeSelectorAnnotation]; ok {
				svc.Headless.NodeSelector = utils.LabelToMap(nodeSelector)
			}
		}
		if address, ok := statefulset.ObjectMeta.Annotations[types.AddressQualifier]; ok {
			svc.Address = address
		} else if svc.Headless != nil {
			svc.Address = statefulset.Spec.ServiceName
		} else {
			svc.Address = statefulset.ObjectMeta.Name
		}
		if svc.Headless == nil {
			if ingressMode, ok := statefulset.ObjectMeta.Annotations[types.IngressModeQualifier]; ok {
				err := svc.SetIngressMode(ingressMode)
				if err != nil {
					event.Recordf(DefinitionMonitorIgnored, "Ignoring invalid annotation %s: %s", types.IngressModeQualifier, err)
				}
			}
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
		if labels, ok := statefulset.ObjectMeta.Annotations[types.ServiceLabels]; ok {
			svc.Labels = utils.LabelToMap(labels)
		}
		if annotations, ok := statefulset.ObjectMeta.Annotations[types.ServiceAnnotations]; ok {
			svc.Annotations = utils.LabelToMap(annotations)
		}
		svc.Origin = "annotation"

		if policyRes := m.policy.ValidateExpose(StatefulSetObjectType, statefulset.Name); !policyRes.Allowed() {
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
		if annotations, ok := daemonset.ObjectMeta.Annotations[types.ServiceAnnotations]; ok {
			svc.Annotations = utils.LabelToMap(annotations)
		}
		svc.Origin = "annotation"

		if policyRes := m.policy.ValidateExpose(DaemonSetObjectType, daemonset.Name); !policyRes.Allowed() {
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

		if tlsCert, ok := service.ObjectMeta.Annotations[types.TlsCertQualifier]; ok {
			if protocol == "http" {
				event.Recordf(DefinitionMonitorIgnored, "Ignoring annotated deployment %s; cannot enable TLS with http protocol", service.ObjectMeta.Name)
				return svc, false
			}

			svc.TlsCredentials = tlsCert
		}

		if tlsTrust, ok := service.ObjectMeta.Annotations[types.TlsTrustQualifier]; ok {
			if protocol == "http" {
				event.Recordf(DefinitionMonitorIgnored, "Ignoring annotated deployment %s; cannot enable TLS with http protocol", service.ObjectMeta.Name)
				return svc, false
			}

			svc.TlsCertAuthority = tlsTrust
		}

		if address, ok := service.ObjectMeta.Annotations[types.AddressQualifier]; ok {
			svc.Address = address
		} else {
			svc.Address = service.ObjectMeta.Name
		}
		if _, ok := service.ObjectMeta.Annotations[types.IngressOnlyQualifier]; ok {
			//no targets should be defined
		} else if target, ok := service.ObjectMeta.Annotations[types.TargetServiceQualifier]; ok {
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
				Name:    service.ObjectMeta.Name,
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
			svcSelector := kube.GetApplicationSelector(service)
			if kube.HasRouterSelector(*service) && hasOriginalSelector(*service) {
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
		if annotations, ok := service.ObjectMeta.Annotations[types.ServiceAnnotations]; ok {
			svc.Annotations = utils.LabelToMap(annotations)
		}
		if ingressMode, ok := service.ObjectMeta.Annotations[types.IngressModeQualifier]; ok {
			err := svc.SetIngressMode(ingressMode)
			if err != nil {
				event.Recordf(DefinitionMonitorIgnored, "Ignoring invalid annotation %s: %s", types.IngressModeQualifier, err)
			}
		}

		svc.Origin = "annotation"

		if policyRes := m.policy.ValidateExpose(ServiceObjectType, service.Name); !policyRes.Allowed() {
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

func (m *DefinitionMonitor) deleteServiceDefinitionForAddress(address string, targetName string) error {
	svc, ok := m.annotated[address]
	if ok {
		if targetName != "" {
			// remove target
			var targets []types.ServiceInterfaceTarget
			for _, target := range svc.Targets {
				if target.Name != targetName {
					targets = append(targets, target)
				}
			}
			svc.Targets = targets
			if len(svc.Targets) > 0 {
				changed := []types.ServiceInterface{
					svc,
				}
				deleted := []string{}
				return kube.UpdateSkupperServices(changed, deleted, "annotation", m.vanClient.Namespace, m.vanClient.KubeClient)
			}
		}

		// delete the svc definition
		changed := []types.ServiceInterface{}
		deleted := []string{
			svc.Address,
		}
		return kube.UpdateSkupperServices(changed, deleted, "annotation", m.vanClient.Namespace, m.vanClient.KubeClient)
	}
	return nil
}

func (m *DefinitionMonitor) deleteServiceDefinitionForAnnotatedObject(name string, objectType string, index map[objectKey]string) error {
	address, ok := index[objectKey{name, objectType}]
	if ok {
		event.Recordf(DefinitionMonitorDeletionEvent, "Deleting service definition for annotated %s %s", objectType, name)
		_, unqualified, err := cache.SplitMetaNamespaceKey(name)
		if err != nil {
			return err
		}
		err = m.deleteServiceDefinitionForAddress(address, unqualified)
		if err != nil {
			return err
		}
		delete(index, objectKey{name, objectType})
	}
	return nil
}

func (m *DefinitionMonitor) restoreServiceDefinitions(name string) error {
	service, err := m.vanClient.KubeClient.CoreV1().Services(m.vanClient.Namespace).Get(name, v1.GetOptions{})
	if err != nil {
		return fmt.Errorf("error retrieving service: %w", err)
	}
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
								if svc.Headless != nil && svc.IsOfLocalOrigin() {
									m.headless[svc.Headless.Name] = svc
								}
								if svc.Origin == "annotation" {
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
							m.annotated[desired.Address] = desired
							if desired.Headless != nil {
								m.headless[desired.Address] = desired
							} else if _, ok := m.headless[desired.Address]; ok {
								delete(m.headless, desired.Address)
							}
						}
						if address, ok := m.annotatedObjects[objectKey{name, StatefulSetObjectType}]; ok {
							if address != desired.Address {
								event.Recordf(DefinitionMonitorUpdateEvent, "Address changed for annotated statefulSet %s. Was %s, now %s", name, address, desired.Address)
								if err := m.deleteServiceDefinitionForAnnotatedObject(name, StatefulSetObjectType, m.annotatedObjects); err != nil {
									return fmt.Errorf("Failed to delete stale service definition for %s", address)
								}
								m.annotatedObjects[objectKey{name, StatefulSetObjectType}] = desired.Address
							}
						} else {
							m.annotatedObjects[objectKey{name, StatefulSetObjectType}] = desired.Address
						}

					} else {
						// is this statefulset one that has been exposed with the headless option?
						if _, ok := m.annotatedObjects[objectKey{name, StatefulSetObjectType}]; ok {
							err := m.deleteServiceDefinitionForAnnotatedObject(name, StatefulSetObjectType, m.annotatedObjects)
							if err != nil {
								return fmt.Errorf("Failed to delete service definition on statefulset %s which is no longer annotated: %s", name, err)
							}
						} else if svc, ok := m.headless[statefulset.ObjectMeta.Name]; ok && svc.Headless.Size != int(*statefulset.Spec.Replicas) {
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
					} else {
						err := m.deleteServiceDefinitionForAnnotatedObject(name, StatefulSetObjectType, m.annotatedObjects)
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
							m.annotated[desired.Address] = desired
						}
						address, ok := m.annotatedObjects[objectKey{name, DeploymentObjectType}]
						if ok {
							if address != desired.Address {
								event.Recordf(DefinitionMonitorUpdateEvent, "Address changed for annotated deployment %s. Was %s, now %s", name, address, desired.Address)
								if err := m.deleteServiceDefinitionForAnnotatedObject(name, DeploymentObjectType, m.annotatedObjects); err != nil {
									return fmt.Errorf("Failed to delete stale service definition for %s", address)
								}
								m.annotatedObjects[objectKey{name, DeploymentObjectType}] = desired.Address
							}
						} else {
							m.annotatedObjects[objectKey{name, DeploymentObjectType}] = desired.Address
						}

					} else {
						err := m.deleteServiceDefinitionForAnnotatedObject(name, DeploymentObjectType, m.annotatedObjects)
						if err != nil {
							return fmt.Errorf("Failed to delete service definition on deployment %s which is no longer annotated: %s", name, err)
						}
					}
				} else {
					err := m.deleteServiceDefinitionForAnnotatedObject(name, DeploymentObjectType, m.annotatedObjects)
					if err != nil {
						return fmt.Errorf("Failed to delete service definition on removal of previously annotated deployment %s: %s", name, err)
					}
				}
			case "deploymentconfigs":
				event.Recordf(DefinitionMonitorEvent, "deploymentconfig event for %s", name)
				obj, exists, err := m.deploymentConfigInformer.GetStore().GetByKey(name)
				if err != nil {
					return fmt.Errorf("Error reading deploymentconfig %s from cache: %s", name, err)
				} else if exists {
					deploymentConfig, ok := obj.(*appv1.DeploymentConfig)
					if !ok {
						return fmt.Errorf("Expected DeploymentConfig for %s but got %#v", name, obj)
					}

					desired, ok := m.getServiceDefinitionFromAnnotatedDeploymentConfig(deploymentConfig)
					if ok {
						event.Recordf(DefinitionMonitorEvent, "Checking annotated deploymentconfig %s", name)
						actual, ok := m.annotated[desired.Address]
						if !ok || updateAnnotatedServiceDefinition(&actual, &desired) {
							event.Recordf(DefinitionMonitorUpdateEvent, "Updating service definition for annotated deploymentconfig %s to %#v", name, desired)
							changed := []types.ServiceInterface{
								desired,
							}
							deleted := []string{}
							err = kube.UpdateSkupperServices(changed, deleted, "annotation", m.vanClient.Namespace, m.vanClient.KubeClient)
							if err != nil {
								return fmt.Errorf("failed to update service definition for annotated deploymentconfig %s: %s", name, err)
							}
							m.annotated[desired.Address] = desired
						}
						address, ok := m.annotatedObjects[objectKey{name, DeploymentConfigObjectType}]
						if ok {
							if address != desired.Address {
								event.Recordf(DefinitionMonitorUpdateEvent, "Address changed for annotated deploymentconfig %s. Was %s, now %s", name, address, desired.Address)
								if err := m.deleteServiceDefinitionForAnnotatedObject(name, DeploymentConfigObjectType, m.annotatedObjects); err != nil {
									return fmt.Errorf("Failed to delete stale service definition for %s", address)
								}
								m.annotatedObjects[objectKey{name, DeploymentConfigObjectType}] = desired.Address
							}
						} else {
							m.annotatedObjects[objectKey{name, DeploymentConfigObjectType}] = desired.Address
						}

					} else {
						err := m.deleteServiceDefinitionForAnnotatedObject(name, DeploymentConfigObjectType, m.annotatedObjects)
						if err != nil {
							return fmt.Errorf("Failed to delete service definition on deploymentconfig %s which is no longer annotated: %s", name, err)
						}
					}
				} else {
					err := m.deleteServiceDefinitionForAnnotatedObject(name, DeploymentConfigObjectType, m.annotatedObjects)
					if err != nil {
						return fmt.Errorf("Failed to delete service definition on removal of previously annotated deploymentconfig %s: %s", name, err)
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
							m.annotated[desired.Address] = desired
						}
						address, ok := m.annotatedObjects[objectKey{name, DaemonSetObjectType}]
						if ok {
							if address != desired.Address {
								event.Recordf(DefinitionMonitorUpdateEvent, "Address changed for annotated daemonset %s. Was %s, now %s", name, address, desired.Address)
								if err := m.deleteServiceDefinitionForAnnotatedObject(name, DaemonSetObjectType, m.annotatedObjects); err != nil {
									return fmt.Errorf("Failed to delete stale service definition for %s", address)
								}
								m.annotatedObjects[objectKey{name, DaemonSetObjectType}] = desired.Address
							}
						} else {
							m.annotatedObjects[objectKey{name, DaemonSetObjectType}] = desired.Address
						}

					} else {
						err := m.deleteServiceDefinitionForAnnotatedObject(name, DaemonSetObjectType, m.annotatedObjects)
						if err != nil {
							return fmt.Errorf("Failed to delete service definition on daemonset %s which is no longer annotated: %s", name, err)
						}
					}
				} else {
					err := m.deleteServiceDefinitionForAnnotatedObject(name, DaemonSetObjectType, m.annotatedObjects)
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
							m.annotated[desired.Address] = desired
						}
						address, ok := m.annotatedObjects[objectKey{name, ServiceObjectType}]
						if ok {
							if address != desired.Address {
								event.Recordf(DefinitionMonitorUpdateEvent, "Address changed for annotated service %s. Was %s, now %s", name, address, desired.Address)
								if err := m.deleteServiceDefinitionForAnnotatedObject(name, ServiceObjectType, m.annotatedObjects); err != nil {
									return fmt.Errorf("Failed to delete stale service definition for %s", address)
								}
								m.annotatedObjects[objectKey{name, ServiceObjectType}] = desired.Address
							}
						} else {
							m.annotatedObjects[objectKey{name, ServiceObjectType}] = desired.Address
						}

					} else {
						err := m.deleteServiceDefinitionForAnnotatedObject(name, ServiceObjectType, m.annotatedObjects)
						if err != nil {
							return fmt.Errorf("Failed to delete service definition on service %s which is no longer annotated: %s", name, err)
						}
						err = m.restoreServiceDefinitions(service.Name)
						if err != nil {
							return fmt.Errorf("Failed to restore service definitions on service %s: %s", name, err)
						}
					}
				} else {
					err := m.deleteServiceDefinitionForAnnotatedObject(name, ServiceObjectType, m.annotatedObjects)
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

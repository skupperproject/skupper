package kube

import (
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/skupperproject/skupper/api/types"
)

func GetLabelsForRouter() map[string]string {
	return map[string]string{
		"application":          types.TransportDeploymentName,
		"skupper.io/component": "router",
	}
}

func GetLoadBalancerHostOrIP(service *corev1.Service) string {
	for _, i := range service.Status.LoadBalancer.Ingress {
		if i.IP != "" {
			return i.IP
		} else if i.Hostname != "" {
			return i.Hostname
		}
	}
	return ""
}

func GetServicePortMap(service *corev1.Service) map[int]int {
	actualPorts := map[int]int{}
	if len(service.Spec.Ports) > 0 {
		for _, port := range service.Spec.Ports {
			targetPort := port.TargetPort.IntValue()
			if targetPort == 0 {
				targetPort = int(port.Port)
			}
			actualPorts[int(port.Port)] = targetPort
		}
	}
	return actualPorts
}

func IndexServicePorts(service *corev1.Service) map[int]int {
	actualPorts := map[int]int{}
	if len(service.Spec.Ports) > 0 {
		for _, port := range service.Spec.Ports {
			targetPort := port.TargetPort.IntValue()
			if targetPort == 0 {
				targetPort = int(port.Port)
			}
			actualPorts[int(port.Port)] = targetPort
		}
	}
	return actualPorts
}

func GetOriginalAssignedPorts(service *corev1.Service) map[int]int {
	originalAssignedPort := service.Annotations[types.OriginalAssignedQualifier]
	return PortLabelStrToMap(originalAssignedPort)
}

func GetOriginalTargetPorts(service *corev1.Service) map[int]int {
	originalTargetPort := service.Annotations[types.OriginalTargetPortQualifier]
	return PortLabelStrToMap(originalTargetPort)
}

func UpdatePorts(spec *corev1.ServiceSpec, desiredPortMappings map[int]int, protocol corev1.Protocol) bool {
	var ports []corev1.ServicePort
	update := false
	for _, port := range spec.Ports {
		if targetPort, ok := desiredPortMappings[int(port.Port)]; ok {
			if targetPort != int(port.TargetPort.IntVal) {
				port.TargetPort = intstr.IntOrString{IntVal: int32(targetPort)}
				update = true
			}
			ports = append(ports, port)
			delete(desiredPortMappings, int(port.Port))
		} else {
			update = true // port should be deleted
		}
	}
	for port, targetPort := range desiredPortMappings {
		update = true
		ports = append(ports, corev1.ServicePort{
			Name:       fmt.Sprintf("port%d", port),
			Port:       int32(port),
			TargetPort: intstr.IntOrString{IntVal: int32(targetPort)},
			Protocol:   protocol,
		})
	}
	if update {
		spec.Ports = ports
		return true
	}
	return false
}

func PortsAsString(ports []corev1.ServicePort) string {
	formatted := []string{}
	for _, port := range ports {
		formatted = append(formatted, fmt.Sprintf("%d:%d", port.Port, port.TargetPort.IntVal))
	}
	return strings.Join(formatted, ",")
}

func EquivalentSelectors(a map[string]string, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if v2, ok := b[k]; !ok || v != v2 {
			return false
		}
	}
	for k, v := range b {
		if v2, ok := a[k]; !ok || v != v2 {
			return false
		}
	}
	return true
}

func GetSelectorAsMap(selector string) (map[string]string, error) {
	ls, err := metav1.ParseToLabelSelector(selector)
	if err != nil {
		return nil, err
	}
	m, err := metav1.LabelSelectorAsMap(ls)
	if err != nil {
		return nil, err
	}
	return m, nil
}

func UpdateSelectorFromMap(spec *corev1.ServiceSpec, desired map[string]string) bool {
	if !EquivalentSelectors(spec.Selector, desired) {
		spec.Selector = desired
		return true
	}
	return false
}

func UpdateSelector(spec *corev1.ServiceSpec, selector string) (bool, error) {
	desired, err := GetSelectorAsMap(selector)
	if err != nil {
		return false, err
	}
	return UpdateSelectorFromMap(spec, desired), nil
}

func protocol(protocol string) corev1.Protocol {
	if strings.EqualFold(protocol, "udp") {
		return corev1.ProtocolUDP
	}
	return corev1.ProtocolTCP
}

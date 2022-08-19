package kube

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/pkg/utils"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
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

func DeleteService(name string, cli types.Services) error {
	svc, _, err := cli.GetService(name)
	if err == nil {
		err = cli.DeleteService(svc.ObjectMeta.Name)
	}
	return err
}

func GetService(name string, cli types.Services) (*corev1.Service, error) {
	current, _, err := cli.GetService(name)
	return current, err
}

func NewHeadlessServiceForAddress(address string, ports []int, targetPorts []int, labels map[string]string, owner *metav1.OwnerReference, namespace string, cli types.VanClientInterface) (*corev1.Service, error) {
	selector := map[string]string{
		"internal.skupper.io/service": address,
	}
	service := makeServiceObjectForAddress(address, ports, targetPorts, labels, selector, owner)
	service.Spec.ClusterIP = "None"

	return createServiceFromObject(service, cli.ServiceManager(namespace))
}

func NewHeadlessService(name string, address string, ports []int, targetPorts []int, labels map[string]string, owner *metav1.OwnerReference, namespace string, cli types.VanClientInterface) (*corev1.Service, error) {
	selector := map[string]string{
		"internal.skupper.io/service": address,
	}
	service := makeServiceObjectForAddress(name, ports, targetPorts, labels, selector, owner)
	service.Spec.ClusterIP = "None"
	service.Annotations[types.ServiceQualifier] = address

	return createServiceFromObject(service, cli.ServiceManager(namespace))
}

func makeServiceObjectForAddress(address string, ports []int, targetPorts []int, labels, selector map[string]string, owner *metav1.OwnerReference) *corev1.Service {
	// TODO: make common service creation and deal with annotation, label differences
	servicePorts := []corev1.ServicePort{}
	for i := 0; i < len(ports); i++ {
		sPort := ports[i]
		tPort := targetPorts[i]
		servicePorts = append(servicePorts, corev1.ServicePort{
			Name:       fmt.Sprintf("%s%d", address, sPort),
			Port:       int32(sPort),
			TargetPort: intstr.FromInt(tPort),
		})
	}
	service := &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Service",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: address,
			Annotations: map[string]string{
				"internal.skupper.io/controlled": "true",
			},
			Labels: labels,
		},
		Spec: corev1.ServiceSpec{
			Selector: selector,
			Ports:    servicePorts,
		},
	}
	if owner != nil {
		service.ObjectMeta.OwnerReferences = []metav1.OwnerReference{*owner}

	}
	return service
}

func createServiceFromObject(service *corev1.Service, cli types.Services) (*corev1.Service, error) {
	created, err := cli.CreateService(service)
	if err != nil {
		return nil, err
	} else {
		return created, nil
	}
}

func CreateService(service *corev1.Service, cli types.Services) (*corev1.Service, error) {
	return createServiceFromObject(service, cli)
}

func GetLoadBalancerHostOrIp(service *corev1.Service) string {
	for _, i := range service.Status.LoadBalancer.Ingress {
		if i.IP != "" {
			return i.IP
		} else if i.Hostname != "" {
			return i.Hostname
		}
	}
	return ""
}

func GetPortsForServiceTarget(targetName string, defaultNamespace string, cliFn func(namespace string) types.Services) (map[int]int, error) {
	ports := map[int]int{}
	parts := strings.Split(targetName, ".")
	var name, namespace string
	if len(parts) > 1 {
		name = parts[0]
		namespace = parts[1]
	} else {
		name = targetName
		namespace = defaultNamespace
	}
	targetSvc, err := GetService(name, cliFn(namespace))
	if err == nil {
		if len(targetSvc.Spec.Ports) > 0 {
			for _, p := range targetSvc.Spec.Ports {
				ports[int(p.Port)] = int(p.Port)
			}
		}
		return ports, nil
	} else if errors.IsNotFound(err) {
		// don't consider the service not yet existing as an error, just can't deduce port
		return ports, nil
	} else {
		return ports, err
	}
}

func CopyService(src string, dest string, annotations map[string]string, cli types.Services) (*corev1.Service, error) {
	original, _, err := cli.GetService(src)
	if err != nil {
		return nil, err
	}
	service := &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Service",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:            dest,
			Annotations:     original.ObjectMeta.Annotations,
			OwnerReferences: original.ObjectMeta.OwnerReferences,
		},
		Spec: corev1.ServiceSpec{
			Selector: original.Spec.Selector,
			Type:     original.Spec.Type,
		},
	}
	for key, _ := range service.ObjectMeta.Annotations {
		if alternative, ok := annotations[key]; ok {
			service.ObjectMeta.Annotations[key] = alternative
		}
	}
	for _, port := range original.Spec.Ports {
		service.Spec.Ports = append(service.Spec.Ports, corev1.ServicePort{
			Name:       port.Name,
			Protocol:   port.Protocol,
			Port:       port.Port,
			TargetPort: port.TargetPort,
		})
	}

	copied, err := cli.CreateService(service)
	if err != nil {
		return nil, err
	}
	return copied, nil
}

func WaitServiceExists(name string, cli types.Services, timeout, interval time.Duration) (*corev1.Service, error) {
	var svc *corev1.Service
	var err error

	ctx, cancel := context.WithTimeout(context.TODO(), timeout)
	defer cancel()
	err = utils.RetryWithContext(ctx, interval, func() (bool, error) {
		svc, _, err = cli.GetService(name)
		if err != nil {
			return false, nil
		}
		return true, nil
	})

	return svc, err
}

func GetServicePort(service *corev1.Service, port int) *corev1.ServicePort {
	for _, p := range service.Spec.Ports {
		if p.Port == int32(port) {
			return &p
		}
	}
	return nil
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

func UpdatePorts(spec *corev1.ServiceSpec, desiredPortMappings map[int]int) bool {
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

func IsOriginalServiceModified(name string, cli types.Services) bool {
	svc, err := GetService(name, cli)
	if err != nil {
		return false
	}
	_, origSelector := svc.Annotations[types.OriginalSelectorQualifier]
	_, origTargetPorts := svc.Annotations[types.OriginalTargetPortQualifier]
	return origSelector || origTargetPorts
}

func RemoveServiceAnnotations(name string, cli types.Services, annotations []string) (*corev1.Service, error) {
	svc, err := GetService(name, cli)
	if err != nil {
		return nil, err
	}
	for _, annotation := range annotations {
		delete(svc.ObjectMeta.Annotations, annotation)
	}
	return cli.UpdateService(svc)
}

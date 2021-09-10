package kube

import (
	"context"
	"fmt"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/pkg/utils"
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

func DeleteService(name string, namespace string, kubeclient kubernetes.Interface) error {
	_, err := kubeclient.CoreV1().Services(namespace).Get(name, metav1.GetOptions{})
	if err == nil {
		err = kubeclient.CoreV1().Services(namespace).Delete(name, &metav1.DeleteOptions{})
	}
	return err
}

func GetService(name string, namespace string, kubeclient kubernetes.Interface) (*corev1.Service, error) {
	current, err := kubeclient.CoreV1().Services(namespace).Get(name, metav1.GetOptions{})
	return current, err
}

func NewServiceForAddress(address string, ports []int, targetPorts []int, labels map[string]string, owner *metav1.OwnerReference, namespace string, kubeclient kubernetes.Interface) (*corev1.Service, error) {
	selector := GetLabelsForRouter()
	service := makeServiceObjectForAddress(address, ports, targetPorts, labels, selector, owner)
	return createServiceFromObject(service, namespace, kubeclient)
}

func NewHeadlessServiceForAddress(address string, ports []int, targetPorts []int, labels map[string]string, owner *metav1.OwnerReference, namespace string, kubeclient kubernetes.Interface) (*corev1.Service, error) {
	selector := map[string]string{
		"internal.skupper.io/service": address,
	}
	service := makeServiceObjectForAddress(address, ports, targetPorts, labels, selector, owner)
	service.Spec.ClusterIP = "None"
	return createServiceFromObject(service, namespace, kubeclient)
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

func createServiceFromObject(service *corev1.Service, namespace string, kubeclient kubernetes.Interface) (*corev1.Service, error) {
	created, err := kubeclient.CoreV1().Services(namespace).Create(service)
	if err != nil {
		return nil, err
	} else {
		return created, nil
	}
}

func CreateService(service *corev1.Service, namespace string, kubeclient kubernetes.Interface) (*corev1.Service, error) {
	return createServiceFromObject(service, namespace, kubeclient)
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

func GetPortsForServiceTarget(targetName string, defaultNamespace string, kubeclient kubernetes.Interface) (map[int]int, error) {
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
	targetSvc, err := GetService(name, namespace, kubeclient)
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

func CopyService(src string, dest string, annotations map[string]string, namespace string, kubeclient kubernetes.Interface) (*corev1.Service, error) {
	original, err := kubeclient.CoreV1().Services(namespace).Get(src, metav1.GetOptions{})
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

	copied, err := kubeclient.CoreV1().Services(namespace).Create(service)
	if err != nil {
		return nil, err
	}
	return copied, nil
}

func WaitServiceExists(name string, namespace string, cli kubernetes.Interface, timeout, interval time.Duration) (*corev1.Service, error) {
	var svc *corev1.Service
	var err error

	ctx, cancel := context.WithTimeout(context.TODO(), timeout)
	defer cancel()
	err = utils.RetryWithContext(ctx, interval, func() (bool, error) {
		svc, err = cli.CoreV1().Services(namespace).Get(name, metav1.GetOptions{})
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

func GetOriginalAssignedPorts(service *corev1.Service) map[int]int {
	originalAssignedPort := service.Annotations[types.OriginalAssignedQualifier]
	return PortLabelStrToMap(originalAssignedPort)
}

func GetOriginalTargetPorts(service *corev1.Service) map[int]int {
	originalTargetPort := service.Annotations[types.OriginalTargetPortQualifier]
	return PortLabelStrToMap(originalTargetPort)
}

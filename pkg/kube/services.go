package kube

import (
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"

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

func NewServiceForAddress(address string, port int, targetPort int, owner *metav1.OwnerReference, namespace string, kubeclient kubernetes.Interface) (*corev1.Service, error) {
	labels := GetLabelsForRouter()
	service := makeServiceObjectForAddress(address, port, targetPort, labels, owner)
	return createServiceFromObject(service, namespace, kubeclient)
}

func NewHeadlessServiceForAddress(address string, port int, targetPort int, owner *metav1.OwnerReference, namespace string, kubeclient kubernetes.Interface) (*corev1.Service, error) {
	labels := map[string]string{
		"internal.skupper.io/service": "myservice2",
	}
	service := makeServiceObjectForAddress(address, port, targetPort, labels, owner)
	service.Spec.ClusterIP = "None"
	return createServiceFromObject(service, namespace, kubeclient)
}

func makeServiceObjectForAddress(address string, port int, targetPort int, labels map[string]string, owner *metav1.OwnerReference) *corev1.Service {
	// TODO: make common service creation and deal with annotation, label differences
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
		},
		Spec: corev1.ServiceSpec{
			Selector: labels,
			Ports: []corev1.ServicePort{
				corev1.ServicePort{
					Name:       address,
					Port:       int32(port),
					TargetPort: intstr.FromInt(targetPort),
				},
			},
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
		return nil, fmt.Errorf("Failed to create service: %w", err)
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

func GetPortForServiceTarget(targetName string, defaultNamespace string, kubeclient kubernetes.Interface) (int, error) {
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
			return int(targetSvc.Spec.Ports[0].Port), nil
		} else {
			return 0, nil
		}
	} else if errors.IsNotFound(err) {
		//don't consider the service not yet existing as an error, just can't deduce port
		return 0, nil
	} else {
		return 0, err
	}
}

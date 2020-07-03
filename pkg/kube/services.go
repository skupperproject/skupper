package kube

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"

	"github.com/skupperproject/skupper/api/types"
)

func getLabelsForRouter() map[string]string {
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
	labels := getLabelsForRouter()
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

func NewService(svc types.Service, labels map[string]string, owner *metav1.OwnerReference, namespace string, kubeclient kubernetes.Interface) (*corev1.Service, error) {
	services := kubeclient.CoreV1().Services(namespace)
	existing, err := services.Get(svc.Name, metav1.GetOptions{})
	if err == nil {
		//TODO: already exists
		return existing, nil
	} else if errors.IsNotFound(err) {
		service := &corev1.Service{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "v1",
				Kind:       "Service",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:        svc.Name,
				Annotations: svc.Annotations,
			},
			Spec: corev1.ServiceSpec{
				Selector: labels,
				Ports:    svc.Ports,
			},
		}
		if svc.Type == "LoadBalancer" {
			service.Spec.Type = corev1.ServiceTypeLoadBalancer
		}
		if owner != nil {
			service.ObjectMeta.OwnerReferences = []metav1.OwnerReference{
				*owner,
			}
		}
		return createServiceFromObject(service, namespace, kubeclient)
	} else {
		service := &corev1.Service{}
		return service, fmt.Errorf("Failed to check service: %w", err)
	}
}

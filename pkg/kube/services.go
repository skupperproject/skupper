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

func getLabels(component string, name string) map[string]string {
	//TODO: cleanup handling of labels
	if component == "controller" {
		return map[string]string{
			"internal.skupper.io/service": name,
		}
	} else {
		application := "skupper"
		if component == "router" {
			//the automeshing function of the router image expects the application
			//to be used as a unique label for identifying routers to connect to
			application = types.TransportDeploymentName
		}
		return map[string]string{
			"application":          application,
			"skupper.io/component": component,
		}
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

func NewServiceForProxy(desiredService types.ServiceInterface, namespace string, kubeclient kubernetes.Interface) (*corev1.Service, error) {
	// TODO: Max for target port, 1024
	// TODO: make common service creation and deal with annotation, label differences
	deployments := kubeclient.AppsV1().Deployments(namespace)
	transportDep, err := deployments.Get(types.TransportDeploymentName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	ownerRef := GetDeploymentOwnerReference(transportDep)

	current, err := kubeclient.CoreV1().Services(namespace).Get(desiredService.Address, metav1.GetOptions{})
	if err == nil {
		// It shouldn't already exist??
		return current, nil
	} else {
		labels := getLabels("controller", desiredService.Address)
		service := &corev1.Service{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "v1",
				Kind:       "Service",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:            desiredService.Address,
				OwnerReferences: []metav1.OwnerReference{ownerRef},
				Annotations: map[string]string{
					"internal.skupper.io/controlled": "true",
				},
			},
			Spec: corev1.ServiceSpec{
				Selector: labels,
				Ports: []corev1.ServicePort{
					corev1.ServicePort{
						Name:       desiredService.Address,
						Port:       int32(desiredService.Port),
						TargetPort: intstr.FromInt(desiredService.Port),
					},
				},
			},
		}

		created, err := kubeclient.CoreV1().Services(namespace).Create(service)
		if err != nil {
			return nil, err
		} else {
			return created, nil
		}
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

		created, err := services.Create(service)

		if err != nil {
			return nil, fmt.Errorf("Failed to create service: %w", err)
		} else {
			return created, nil
		}
	} else {
		service := &corev1.Service{}
		return service, fmt.Errorf("Failed to check service: %w", err)
	}
}

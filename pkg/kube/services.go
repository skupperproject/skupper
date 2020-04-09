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

func DeleteService(name string, namespace string, kubeclient *kubernetes.Clientset) error {
	_, err := kubeclient.CoreV1().Services(namespace).Get(name, metav1.GetOptions{})
	if err == nil {
		err = kubeclient.CoreV1().Services(namespace).Delete(name, &metav1.DeleteOptions{})
	}
	return err
}

func GetService(name string, namespace string, kubeclient *kubernetes.Clientset) (*corev1.Service, error) {
	current, err := kubeclient.CoreV1().Services(namespace).Get(name, metav1.GetOptions{})
	return current, err
}

func NewServiceForProxy(desiredService types.ServiceInterface, namespace string, kubeclient *kubernetes.Clientset) (*corev1.Service, error) {
	// TODO: Max for target port, 1024
	// TODO: make common service creation and deal with annotation, label differences
	deployments := kubeclient.AppsV1().Deployments(namespace)
	transportDep, err := deployments.Get(types.TransportDeploymentName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	ownerRef := GetOwnerReference(transportDep)

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

func NewServiceWithOwner(svc types.Service, owner metav1.OwnerReference, namespace string, kubeclient *kubernetes.Clientset) (*corev1.Service, error) {
	current, err := kubeclient.CoreV1().Services(namespace).Get(svc.Name, metav1.GetOptions{})
	if err == nil {
		return current, nil
	} else if errors.IsNotFound(err) {
		labels := getLabels("router", "")
		service := &corev1.Service{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "v1",
				Kind:       "Service",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:            svc.Name,
				OwnerReferences: []metav1.OwnerReference{owner},
				Annotations:     svc.Annotations,
			},
			Spec: corev1.ServiceSpec{
				Selector: labels,
				Ports:    svc.Ports,
			},
		}
		created, err := kubeclient.CoreV1().Services(namespace).Create(service)
		if err != nil {
			return nil, fmt.Errorf("Failed to create service: %w", err)
		} else {
			return created, nil
		}
	}
	return nil, fmt.Errorf("Failed while checking service: %w", err)
}

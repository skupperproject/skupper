package kube

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/ajssmith/skupper/api/types"
)

func getLabels(component string) map[string]string {
	//TODO: cleanup handling of labels
	application := "skupper"
	if component == "router" {
		//the automeshing function of the router image expects the application
		//to be used as a unique label for identifying routers to connect to
		application = "skupper-router"
	}
	return map[string]string{
		"application":          application,
		"skupper.io/component": component,
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

func GetService(name string, namespace string, kubeclient *kubernetes.Clientset) (*corev1.Service, error) {
	current, err := kubeclient.CoreV1().Services(namespace).Get(name, metav1.GetOptions{})
	return current, err
}

func NewServiceWithOwner(svc types.Service, owner metav1.OwnerReference, namespace string, kubeclient *kubernetes.Clientset) (*corev1.Service, error) {
	current, err := kubeclient.CoreV1().Services(namespace).Get(svc.Name, metav1.GetOptions{})
	if err == nil {
		fmt.Println("Service", svc.Name, "already exists")
		return current, nil
	} else if errors.IsNotFound(err) {
		labels := getLabels("router")
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
			fmt.Println("Failed to create service: ", err.Error())
			return nil, err
		} else {
			return created, nil
		}
	} else {
		fmt.Println("Failed while checking service: ", err.Error())
		return nil, err
	}
}

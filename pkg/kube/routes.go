package kube

import (
	"fmt"

	routev1 "github.com/openshift/api/route/v1"
	routev1client "github.com/openshift/client-go/route/clientset/versioned/typed/route/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func GetRoute(name string, namespace string, rc *routev1client.RouteV1Client) (*routev1.Route, error) {
	current, err := rc.Routes(namespace).Get(name, metav1.GetOptions{})
	return current, err
}

func CreateRoute(route *routev1.Route, namespace string, rc *routev1client.RouteV1Client) (*routev1.Route, error) {
	current, err := rc.Routes(namespace).Get(route.Name, metav1.GetOptions{})
	if err == nil {
		return current, fmt.Errorf("Route %s already exists", route.Name)
	} else if errors.IsNotFound(err) {
		created, err := rc.Routes(namespace).Create(route)
		if err != nil {
			return nil, err
		} else {
			return created, nil
		}
	} else {
		return nil, fmt.Errorf("Failed while checking route: %w", err)
	}
}

func UpdateTargetServiceForRoute(routeName string, serviceName string, namespace string, rc *routev1client.RouteV1Client) error {
	current, err := rc.Routes(namespace).Get(routeName, metav1.GetOptions{})
	if err != nil {
		return err
	}
	current.Spec.To.Name = serviceName
	_, err = rc.Routes(namespace).Update(current)
	if err != nil {
		return err
	}
	return nil
}

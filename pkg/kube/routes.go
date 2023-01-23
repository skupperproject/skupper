package kube

import (
	"context"
	"fmt"

	routev1 "github.com/openshift/api/route/v1"
	routev1client "github.com/openshift/client-go/route/clientset/versioned/typed/route/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func GetRoute(name string, namespace string, rc *routev1client.RouteV1Client) (*routev1.Route, error) {
	current, err := rc.Routes(namespace).Get(context.TODO(), name, metav1.GetOptions{})
	return current, err
}

func CreateRoute(route *routev1.Route, namespace string, rc *routev1client.RouteV1Client) (*routev1.Route, error) {
	current, err := rc.Routes(namespace).Get(context.TODO(), route.Name, metav1.GetOptions{})
	if err == nil {
		return current, errors.NewAlreadyExists(schema.GroupResource{Group: "openshift.io", Resource: "routes"}, route.Name)
	} else if errors.IsNotFound(err) {
		created, err := rc.Routes(namespace).Create(context.TODO(), route, metav1.CreateOptions{})
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
	current, err := rc.Routes(namespace).Get(context.TODO(), routeName, metav1.GetOptions{})
	if err != nil {
		return err
	}
	current.Spec.To.Name = serviceName
	_, err = rc.Routes(namespace).Update(context.TODO(), current, metav1.UpdateOptions{})
	if err != nil {
		return err
	}
	return nil
}

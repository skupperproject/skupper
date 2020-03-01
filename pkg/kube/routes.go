package kube

import (
	"fmt"

	routev1 "github.com/openshift/api/route/v1"
	routev1client "github.com/openshift/client-go/route/clientset/versioned/typed/route/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/ajssmith/skupper/api/types"
)

func GetRoute(name string, namespace string, rc *routev1client.RouteV1Client) (*routev1.Route, error) {
	current, err := rc.Routes(namespace).Get(name, metav1.GetOptions{})
	return current, err
}

func NewRouteWithOwner(rte types.Route, owner metav1.OwnerReference, namespace string, rc *routev1client.RouteV1Client) (*routev1.Route, error) {
	insecurePolicy := routev1.InsecureEdgeTerminationPolicyNone
	if rte.Termination != routev1.TLSTerminationPassthrough {
		insecurePolicy = routev1.InsecureEdgeTerminationPolicyRedirect
	}
	current, err := rc.Routes(namespace).Get(rte.Name, metav1.GetOptions{})
	if err == nil {
		fmt.Println("Route", rte.Name, "already exists")
		return current, nil
	} else if errors.IsNotFound(err) {
		route := &routev1.Route{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "v1",
				Kind:       "Route",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:            rte.Name,
				OwnerReferences: []metav1.OwnerReference{owner},
			},
			Spec: routev1.RouteSpec{
				Path: "",
				Port: &routev1.RoutePort{
					TargetPort: intstr.FromString(rte.TargetPort),
				},
				To: routev1.RouteTargetReference{
					Kind: "Service",
					Name: rte.TargetService,
				},
				TLS: &routev1.TLSConfig{
					Termination:                   rte.Termination,
					InsecureEdgeTerminationPolicy: insecurePolicy,
				},
			},
		}

		created, err := rc.Routes(namespace).Create(route)
		if err != nil {
			fmt.Println("Failed to create route: ", err.Error())
			return nil, err
		} else {
			return created, nil
		}
	} else {
		fmt.Println("Failed while checking route: ", err.Error())
		return nil, err
	}
}

package securedaccess

import (
	"context"
	"fmt"
	"log"
	"reflect"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	routev1 "github.com/openshift/api/route/v1"

	skupperv2alpha1 "github.com/skupperproject/skupper/pkg/apis/skupper/v2alpha1"
)

type RouteAccessType struct {
	manager *SecuredAccessManager
}

func newRouteAccess(m *SecuredAccessManager) AccessType {
	return &RouteAccessType{
		manager: m,
	}
}

func (o *RouteAccessType) RealiseAndResolve(access *skupperv2alpha1.SecuredAccess, svc *corev1.Service) ([]skupperv2alpha1.Endpoint, error) {
	var endpoints []skupperv2alpha1.Endpoint
	for _, port := range access.Spec.Ports {
		desired := desiredRouteForPort(access, port)
		err, route := o.ensureRoute(access.Namespace, desired)
		if err != nil {
			return nil, err
		}
		for _, ingress := range route.Status.Ingress {
			endpoints = append(endpoints, skupperv2alpha1.Endpoint{
				Name: port.Name,
				Host: ingress.Host,
				Port: "443",
			})
		}
	}
	return endpoints, nil
}

func (o *RouteAccessType) ensureRoute(namespace string, route *routev1.Route) (error, *routev1.Route) {
	key := fmt.Sprintf("%s/%s", namespace, route.Name)
	if existing, ok := o.manager.routes[key]; ok {
		if equivalentRoute(existing, route) {
			return nil, existing
		}
		copy := *existing
		copy.Spec = route.Spec
		updated, err := o.manager.clients.GetRouteClient().Routes(namespace).Update(context.Background(), &copy, metav1.UpdateOptions{})
		if err != nil {
			log.Printf("Error on update for route %s/%s: %s", namespace, route.Name, err)
			return err, nil
		}
		log.Printf("Route %s/%s updated successfully", namespace, route.Name)
		o.manager.routes[key] = updated
		return nil, updated
	}
	created, err := o.manager.clients.GetRouteClient().Routes(namespace).Create(context.Background(), route, metav1.CreateOptions{})
	if err != nil {
		log.Printf("Error on create for route %s/%s: %s", namespace, route.Name, err)
		return err, nil
	}
	log.Printf("Route %s/%s created successfully", namespace, route.Name)
	o.manager.routes[key] = created
	return nil, created
}

func desiredRouteForPort(sa *skupperv2alpha1.SecuredAccess, port skupperv2alpha1.SecuredAccessPort) *routev1.Route {
	name := fmt.Sprintf("%s-%s", sa.Name, port.Name)
	host := sa.Spec.Options["domain"]
	if host != "" {
		host = fmt.Sprintf("%s.%s.%s", name, sa.Namespace, host)
	}
	route := &routev1.Route{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Route",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				"internal.skupper.io/secured-access": "true",
			},
			Annotations: map[string]string{
				"internal.skupper.io/controlled": "true",
			},
			OwnerReferences: ownerReferences(sa),
		},
		Spec: routev1.RouteSpec{
			Path: "",
			Host: host,
			Port: &routev1.RoutePort{
				TargetPort: intstr.FromString(port.Name),
			},
			To: routev1.RouteTargetReference{
				Kind: "Service",
				Name: sa.Name,
			},
			TLS: &routev1.TLSConfig{
				Termination:                   routev1.TLSTerminationPassthrough,
				InsecureEdgeTerminationPolicy: routev1.InsecureEdgeTerminationPolicyNone,
			},
		},
	}
	return route
}

func equivalentRoute(actual *routev1.Route, desired *routev1.Route) bool {
	return (desired.Spec.Host == "" || desired.Spec.Host == actual.Spec.Host) &&
		reflect.DeepEqual(desired.Spec.Port, actual.Spec.Port) &&
		reflect.DeepEqual(desired.Spec.To, actual.Spec.To) &&
		reflect.DeepEqual(desired.Spec.TLS, actual.Spec.TLS)
}

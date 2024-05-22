package securedaccess

import (
	"fmt"
	"reflect"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	routev1 "github.com/openshift/api/route/v1"

	skupperv1alpha1 "github.com/skupperproject/skupper/pkg/apis/skupper/v1alpha1"
)

type RouteAccessType struct {
	manager *SecuredAccessManager
}

func newRouteAccess(m *SecuredAccessManager) AccessType {
	return &RouteAccessType{
		manager: m,
	}
}

func (o *RouteAccessType) Realise(access *skupperv1alpha1.SecuredAccess) bool {
	var errors []string
	for _, port := range access.Spec.Ports {
		route := desiredRouteForPort(access, port)
		if err, _ := o.manager.ensureRoute(access.Namespace, route); err != nil {
			errors = append(errors, err.Error())
		}
	}
	if len(errors) == 0 {
		return false
	}
	message := strings.Join(errors, ",")
	if message == access.Status.Status {
		return false
	}
	access.Status.Status = message
	return true
}

func (o *RouteAccessType) Resolve(access *skupperv1alpha1.SecuredAccess) bool {
	var urls []skupperv1alpha1.SecuredAccessUrl
	for _, port := range access.Spec.Ports {
		key := routeKey(access, port)
		if route, ok := o.manager.routes[key]; ok && route.Spec.Host != "" {
			urls = append(urls, skupperv1alpha1.SecuredAccessUrl{
				Name: port.Name,
				Url:  route.Spec.Host + ":443",
			})
		}
	}
	if urls == nil || reflect.DeepEqual(urls, access.Status.Urls) {
		return false
	}
	access.Status.Urls = urls
	return true
}

func routeKey(sa *skupperv1alpha1.SecuredAccess, port skupperv1alpha1.SecuredAccessPort) string {
	return fmt.Sprintf("%s/%s-%s", sa.Namespace, sa.Name, port.Name)
}

func desiredRouteForPort(sa *skupperv1alpha1.SecuredAccess, port skupperv1alpha1.SecuredAccessPort) *routev1.Route {
	name := fmt.Sprintf("%s-%s", sa.Name, port.Name)
	host := sa.Spec.Options["domain"]
	if host != "" {
		host = fmt.Sprintf("%s-%s.%s", sa.Namespace, host)
	}
	route := &routev1.Route{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Route",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
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

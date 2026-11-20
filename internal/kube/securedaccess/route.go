package securedaccess

import (
	"fmt"
	routev1 "github.com/openshift/api/route/v1"
	skupperv2alpha1 "github.com/skupperproject/skupper/pkg/apis/skupper/v2alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"reflect"
)

var routeWeight100 int32 = 100

func desiredRouteForPort(sa *skupperv2alpha1.SecuredAccess, port skupperv2alpha1.SecuredAccessPort) *routev1.Route {
	name := fmt.Sprintf("%s-%s", sa.Name, port.Name)
	host := sa.Spec.Settings["domain"]
	if host != "" {
		host = fmt.Sprintf("%s.%s.%s", name, sa.Namespace, host)
	}
	return &routev1.Route{
		TypeMeta:   metav1.TypeMeta{APIVersion: "route.openshift.io/v1", Kind: "Route"},
		ObjectMeta: exposureMetadata(sa, name),
		Spec: routev1.RouteSpec{Host: host, Port: &routev1.RoutePort{TargetPort: intstr.FromString(port.Name)},
			To:  routev1.RouteTargetReference{Kind: "Service", Name: sa.Name, Weight: &routeWeight100},
			TLS: &routev1.TLSConfig{Termination: routev1.TLSTerminationPassthrough, InsecureEdgeTerminationPolicy: routev1.InsecureEdgeTerminationPolicyNone}},
	}
}

func equivalentRoute(actual, desired *routev1.Route) bool {
	return (desired.Spec.Host == "" || desired.Spec.Host == actual.Spec.Host) && reflect.DeepEqual(desired.Spec.Port, actual.Spec.Port) && reflect.DeepEqual(desired.Spec.To, actual.Spec.To) && reflect.DeepEqual(desired.Spec.TLS, actual.Spec.TLS)
}

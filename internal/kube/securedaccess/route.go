package securedaccess

import (
	"context"
	"fmt"
	"log/slog"
	"reflect"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	routev1 "github.com/openshift/api/route/v1"

	skupperv2alpha1 "github.com/skupperproject/skupper/pkg/apis/skupper/v2alpha1"
)

var routeWeight100 int32 = 100

type RouteAccessType struct {
	manager *SecuredAccessManager
	logger  *slog.Logger
}

func newRouteAccess(m *SecuredAccessManager) AccessType {
	return &RouteAccessType{
		manager: m,
		logger:  slog.New(slog.Default().Handler()).With(slog.String("component", "kube.securedaccess.routeAccessType")),
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
		changed := false
		copy := *existing
		if !equivalentRoute(existing, route) {
			copy.Spec = route.Spec
			changed = true
		}
		if o.manager.context != nil {
			if copy.ObjectMeta.Labels == nil {
				copy.ObjectMeta.Labels = map[string]string{}
			}
			if copy.ObjectMeta.Annotations == nil {
				copy.ObjectMeta.Annotations = map[string]string{}
			}
			if o.manager.context.SetLabels(namespace, copy.Name, "Route", copy.ObjectMeta.Labels) {
				changed = true
			}
			if o.manager.context.SetAnnotations(namespace, copy.Name, "Route", copy.ObjectMeta.Annotations) {
				changed = true
			}
		}
		if !changed {
			return nil, existing
		}
		updated, err := o.manager.clients.GetRouteClient().Routes(namespace).Update(context.Background(), &copy, metav1.UpdateOptions{})
		if err != nil {
			o.logger.Error("Error on update for route",
				slog.String("namespace", namespace),
				slog.String("name", route.Name),
				slog.Any("error", err))
			return err, nil
		}
		o.manager.routes[key] = updated
		return nil, updated
	}
	if o.manager.context != nil {
		o.manager.context.SetLabels(namespace, route.Name, "Route", route.ObjectMeta.Labels)
		o.manager.context.SetAnnotations(namespace, route.Name, "Route", route.ObjectMeta.Annotations)
	}
	created, err := o.manager.clients.GetRouteClient().Routes(namespace).Create(context.Background(), route, metav1.CreateOptions{})
	if err != nil {
		o.logger.Error("Error on create for route",
			slog.String("namespace", namespace),
			slog.String("name", route.Name),
			slog.Any("error", err))
		return err, nil
	}
	o.logger.Info("Route created successfully", slog.String("namespace", namespace), slog.String("name", route.Name))
	o.manager.routes[key] = created
	return nil, created
}

func desiredRouteForPort(sa *skupperv2alpha1.SecuredAccess, port skupperv2alpha1.SecuredAccessPort) *routev1.Route {
	name := fmt.Sprintf("%s-%s", sa.Name, port.Name)
	host := sa.Spec.Settings["domain"]
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
				Kind:   "Service",
				Name:   sa.Name,
				Weight: &routeWeight100,
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

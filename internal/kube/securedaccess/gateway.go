package securedaccess

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"log/slog"
	"strconv"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/skupperproject/skupper/internal/kube/resource"
	skupperv2alpha1 "github.com/skupperproject/skupper/pkg/apis/skupper/v2alpha1"
)

//go:embed gateway.yaml
var gatewayTemplate string

type GatewayParameters struct {
	Name  string
	Class string
	Port  int
}

//go:embed tls-route.yaml
var tlsRouteTemplate string

type TlsRouteParameters struct {
	Name             string
	GatewayName      string
	GatewayNamespace string
	OwnerUID         string
	Hostname         string
	ServiceName      string
	ServiceNamespace string
	ServicePort      int
	Labels           map[string]string
	Annotations      map[string]string
}

type GatewayAccessType struct {
	manager          *SecuredAccessManager
	class            string
	domain           string
	port             int
	gatewayNamespace string
	controllerName   string
	controllerUID    string
	unreconciled     map[string]*skupperv2alpha1.SecuredAccess
	logger           *slog.Logger
}

func newGatewayAccess(manager *SecuredAccessManager, class string, domain string, port int, context ControllerContext) (AccessType, func() error, error) {
	at := &GatewayAccessType{
		manager:      manager,
		class:        class,
		domain:       domain,
		port:         port,
		unreconciled: map[string]*skupperv2alpha1.SecuredAccess{},
		logger:       slog.New(slog.Default().Handler()).With(slog.String("component", "kube.securedaccess.gatewayAccessType")),
	}
	if context != nil {
		at.gatewayNamespace = context.Namespace()
		at.controllerName = context.Name()
		at.controllerUID = context.UID()
	}
	if err := at.init(); err != nil {
		return nil, nil, err
	}
	return at, at.init, nil
}

func (o *GatewayAccessType) init() error {
	// create gateway
	template := resource.Template{
		Name:     "gateway",
		Template: gatewayTemplate,
		Parameters: GatewayParameters{
			Name:  "skupper",
			Class: o.class,
			Port:  o.port,
		},
		Resource: schema.GroupVersionResource{
			Group:    "gateway.networking.k8s.io",
			Version:  "v1",
			Resource: "gateways",
		},
	}
	gateway, err := template.Apply(o.manager.clients.GetDynamicClient(), context.Background(), o.gatewayNamespace)
	if err != nil {
		return err
	}
	if o.domain == "" {
		if domain := getBaseDomain(gateway); domain != "" {
			o.domain = domain //TODO: or keep configured v deduced domain separate?
			o.processUnreconciled()
		} else {
			o.logger.Error("Could not determine base domain for gateway", slog.String("namespace", gateway.GetNamespace()), slog.String("name", gateway.GetName()))
		}
	}
	return nil
}

func (o *GatewayAccessType) processUnreconciled() {
	for _, access := range o.unreconciled {
		o.manager.reconcile(access)
	}
	o.unreconciled = map[string]*skupperv2alpha1.SecuredAccess{}
}

func (o *GatewayAccessType) RealiseAndResolve(access *skupperv2alpha1.SecuredAccess, svc *corev1.Service) ([]skupperv2alpha1.Endpoint, error) {
	if o.domain == "" {
		o.unreconciled[string(access.UID)] = access
		return nil, errors.New("Gateway base domain not yet resolved")
	}
	var endpoints []skupperv2alpha1.Endpoint
	for _, port := range access.Spec.Ports {
		name := fmt.Sprintf("%s-%s", access.Name, port.Name)
		hostname := fmt.Sprintf("%s.%s.%s", name, access.Namespace, o.domain)
		var labels map[string]string
		var annotations map[string]string
		if o.manager.context != nil {
			labels = map[string]string{}
			annotations = map[string]string{}
			o.manager.context.SetLabels(access.Namespace, name, "TlsRoute", labels)
			o.manager.context.SetAnnotations(access.Namespace, name, "TlsRoute", annotations)
		}
		template := resource.Template{
			Name:     "tlsroute",
			Template: tlsRouteTemplate,
			Parameters: TlsRouteParameters{
				Name:             name,
				GatewayName:      "skupper",
				GatewayNamespace: o.gatewayNamespace,
				OwnerUID:         string(access.ObjectMeta.UID),
				Hostname:         hostname,
				ServiceName:      access.Name,
				ServiceNamespace: access.Namespace,
				ServicePort:      port.Port,
				Labels:           labels,
				Annotations:      annotations,
			},
			Resource: schema.GroupVersionResource{
				Group:    "gateway.networking.k8s.io",
				Version:  "v1alpha2",
				Resource: "tlsroutes",
			},
		}

		if _, err := template.Apply(o.manager.clients.GetDynamicClient(), context.Background(), access.Namespace); err != nil {
			return nil, err
		}
		endpoints = append(endpoints, skupperv2alpha1.Endpoint{
			Name: port.Name,
			Host: hostname,
			Port: strconv.Itoa(o.port),
		})
	}
	return endpoints, nil
}

func getBaseDomain(obj *unstructured.Unstructured) string {
	addresses, _, _ := unstructured.NestedSlice(obj.UnstructuredContent(), "status", "addresses")
	for _, a := range addresses {
		if address, ok := a.(map[string]interface{}); ok {
			value, _, _ := unstructured.NestedString(address, "value")
			if addressType, _, _ := unstructured.NestedString(address, "type"); addressType == "Hostname" && value != "" {
				return value
			}
		}
	}
	for _, a := range addresses {
		if address, ok := a.(map[string]interface{}); ok {
			value, _, _ := unstructured.NestedString(address, "value")
			if addressType, _, _ := unstructured.NestedString(address, "type"); addressType == "IPAddress" && value != "" {
				return value + ".nip.io"
			}
		}
	}
	return ""
}

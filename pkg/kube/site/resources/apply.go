package resources

import (
	"bytes"
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/json"
	"fmt"
	"text/template"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer/yaml"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/discovery/cached/memory"
	"k8s.io/client-go/restmapper"

	skuppertypes "github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/pkg/images"
	"github.com/skupperproject/skupper/pkg/kube"
	"github.com/skupperproject/skupper/pkg/kube/resolver"
)

var decoder = yaml.NewDecodingSerializer(unstructured.UnstructuredJSONScheme)

//go:embed skupper-router-deployment.yaml
var routerDeploymentTemplate string

//go:embed skupper-router-service.yaml
var routerServiceTemplate string

//go:embed skupper-router-local-service.yaml
var routerLocalServiceTemplate string

//go:embed route.yaml
var routeTemplate string

//go:embed status-configmap.yaml
var statusTemplate string

type NamedTemplate struct {
	name   string
	value  string
	params interface{}
}

func (t NamedTemplate) getYaml() ([]byte, error) {
	tmpl, err := template.New(t.name).Parse(t.value)
	if err != nil {
		return nil, err
	}
	var buffer bytes.Buffer
	err = tmpl.Execute(&buffer, t.params)
	if err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

func resourceTemplates(clients kube.Clients, name string, siteId string, namespace string, config *skuppertypes.SiteConfigSpec) []NamedTemplate {
	options := getCoreParams(name, siteId, namespace, config)
	templates := []NamedTemplate{
		{
			name:   "deployment",
			value:  routerDeploymentTemplate,
			params: options,
		},
		{
			name:   "localService",
			value:  routerLocalServiceTemplate,
			params: options,
		},
	}
	/* TODO: remove this permanently
	if !options.IsEdge {
		templates = append(templates, NamedTemplate{
			name:   "service",
			value:  routerServiceTemplate,
			params: options,
		})
	}

	// ingress related resources, depending on configuration and availability
	if config.IsIngressRoute() && kube.IsResourceAvailable(clients.GetDiscoveryClient(), routeGVR) {
		for _, params := range routeParams(siteId) {
			templates = append(templates, NamedTemplate{
				name:   params.RouteName,
				value:  routeTemplate,
				params: params,
			})
		}
	}
	if config.IsIngressNginxIngress() && kube.IsResourceAvailable(clients.GetDiscoveryClient(), ingressGVR) {
		//TODO:
	}
	if config.IsIngressKubernetes() && kube.IsResourceAvailable(clients.GetDiscoveryClient(), ingressGVR) {
		//TODO:
	}
	if config.IsIngressContourHttpProxy() && kube.IsResourceAvailable(clients.GetDiscoveryClient(), httpProxyGVR) {
		//TODO:
	}
	*/
	return templates
}

type CoreParams struct {
	SiteId          string
	SiteName        string
	Replicas        int
	ServiceAccount  string
	IsEdge          bool
	ServiceType     string
	ConfigDigest    string
	RouterImage     skuppertypes.ImageDetails
	ConfigSyncImage skuppertypes.ImageDetails
}

type RouteParams struct {
	SiteId    string
	RouteName string
	PortName  string
}

func isEdge(config *skuppertypes.SiteConfigSpec) bool {
	return config != nil && config.RouterMode == string(skuppertypes.TransportModeEdge)
}

func serviceType(config *skuppertypes.SiteConfigSpec) string {
	if config != nil {
		if config.IsIngressLoadBalancer() {
			return "LoadBalancer"
		}
		if config.IsIngressNodePort() {
			return "NodePort"
		}
	}
	return ""
}

func configDigest(config *skuppertypes.SiteConfigSpec) string {
	if config != nil {
		h := sha256.New()
		h.Write([]byte(config.RouterMode))
		for _, r := range config.Router.Logging {
			h.Write([]byte(r.Module + r.Level))
		}
		h.Write([]byte(config.Router.DataConnectionCount))
		return fmt.Sprintf("%x", h.Sum(nil))
	}
	return ""
}

func getCoreParams(name string, siteId string, namespace string, config *skuppertypes.SiteConfigSpec) CoreParams {
	replicas := config.Routers
	if replicas == 0 {
		replicas = 1
	}
	return CoreParams{
		SiteId:          siteId,
		SiteName:        name,
		Replicas:        replicas,
		ServiceAccount:  "skupper-router", //TODO, take from config, e.g. config.ServiceAccount,
		IsEdge:          isEdge(config),
		ServiceType:     serviceType(config),
		ConfigDigest:    configDigest(config),
		RouterImage:     images.GetRouterImageDetails(),
		ConfigSyncImage: images.GetConfigSyncImageDetails(),
	}
}

var routeGVR = schema.GroupVersionResource{
	Group:    "route.openshift.io",
	Version:  "v1",
	Resource: "routes",
}

var httpProxyGVR = schema.GroupVersionResource{
	Group:    "projectcontour.io",
	Version:  "v1",
	Resource: "httpproxies",
}

var ingressGVR = schema.GroupVersionResource{
	Group:    "networking.k8s.io",
	Version:  "v1",
	Resource: "ingresses",
}

//TODO: do we need to support v1beta1 for ingress separately?

func routeParams(siteId string) []RouteParams {
	return []RouteParams{
		{
			SiteId:    siteId,
			RouteName: skuppertypes.EdgeRouteName,
			PortName:  "edge",
		},
		{
			SiteId:    siteId,
			RouteName: skuppertypes.InterRouterRouteName,
			PortName:  "inter-router",
		},
		{
			SiteId:    siteId,
			RouteName: skuppertypes.ClaimRedemptionRouteName,
			PortName:  "claims",
		},
	}
}

func Apply(clients kube.Clients, ctx context.Context, namespace string, name string, siteId string, config *skuppertypes.SiteConfigSpec) error {
	for _, t := range resourceTemplates(clients, name, siteId, namespace, config) {
		raw, err := t.getYaml()
		if err != nil {
			return err
		}
		err = apply(clients, ctx, namespace, raw)
		if err != nil {
			return err
		}
	}
	return nil
}

func apply(clients kube.Clients, ctx context.Context, namespace string, raw []byte) error {
	obj := &unstructured.Unstructured{}
	_, gvk, err := decoder.Decode(raw, nil, obj)
	if err != nil {
		return err
	}
	data, err := json.Marshal(obj)
	if err != nil {
		return err
	}

	mapper := restmapper.NewDeferredDiscoveryRESTMapper(memory.NewMemCacheClient(clients.GetDiscoveryClient()))
	mapping, err := mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	if err != nil {
		return err
	}
	resource := clients.GetDynamicClient().Resource(mapping.Resource).Namespace(namespace)

	_, err = resource.Patch(ctx, obj.GetName(), types.ApplyPatchType, data, metav1.PatchOptions{
		FieldManager: "skupper-controller",
	})
	return err
}

type StatusParams struct {
	SiteName  string
	SiteId    string
	Addresses string
	Errors    string
}

func ApplyStatus(clients kube.Clients, ctx context.Context, namespace string, siteName string, siteId string, addresses resolver.HostPorts, errors []string) error {
	params := StatusParams{
		SiteName: siteName,
		SiteId:   siteId,
	}
	b, err := json.Marshal(addresses)
	if err != nil {
		return err
	}
	params.Addresses = string(b)
	b, err = json.Marshal(errors)
	if err != nil {
		return err
	}
	params.Errors = string(b)
	template := NamedTemplate{
		name:   "status",
		value:  statusTemplate,
		params: params,
	}
	raw, err := template.getYaml()
	if err != nil {
		return err
	}
	return apply(clients, ctx, namespace, raw)
}

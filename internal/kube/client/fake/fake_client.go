package fake

import (
	"context"
	"fmt"

	routev1 "github.com/openshift/api/route/v1"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	discoveryfake "k8s.io/client-go/discovery/fake"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	routefake "github.com/openshift/client-go/route/clientset/versioned/fake"
	"github.com/skupperproject/skupper/internal/kube/client"
	"github.com/skupperproject/skupper/internal/kube/resource"
	skupperclientfake "github.com/skupperproject/skupper/pkg/generated/client/clientset/versioned/fake"
	fakeskupperv2alpha1 "github.com/skupperproject/skupper/pkg/generated/client/clientset/versioned/typed/skupper/v2alpha1/fake"
	testing "k8s.io/client-go/testing"
)

func NewFakeClient(namespace string, k8sObjects []runtime.Object, skupperObjects []runtime.Object, fakeSkupperError string) (*client.KubeClient, error) {
	c := &client.KubeClient{}

	var routes []runtime.Object
	var standard []runtime.Object
	var dynamic []runtime.Object
	for _, obj := range k8sObjects {
		if route, ok := obj.(*routev1.Route); ok {
			routes = append(routes, route)
		} else if u, ok := obj.(*unstructured.Unstructured); ok {
			dynamic = append(dynamic, u)
		} else {
			standard = append(standard, obj)
		}
	}

	c.Namespace = namespace
	c.Kube = k8sfake.NewClientset(standard...)
	c.Skupper = skupperclientfake.NewSimpleClientset(skupperObjects...)
	// Note: brute force error return for any client access, we could make it more granular if needed
	if fakeSkupperError != "" {
		c.Skupper.SkupperV2alpha1().(*fakeskupperv2alpha1.FakeSkupperV2alpha1).PrependReactor("*", "*", func(action testing.Action) (handled bool, ret runtime.Object, err error) {
			return true, nil, fmt.Errorf("%s", fakeSkupperError)
		})
	}
	scheme := runtime.NewScheme()
	appsv1.AddToScheme(scheme)
	c.Dynamic = dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme, map[schema.GroupVersionResource]string{
		resource.ContourHttpProxyResource(): "HTTPProxyList",
		resource.GatewayResource():          "GatewayList",
		resource.TlsRouteResource():         "TLSRouteList",
		resource.DeploymentResource():       "DeploymentList",
	}, dynamic...)
	// prepopulated objects not working for some reason with dynamic client, so create them manually here for now:
	for _, d := range dynamic {
		obj := d.(*unstructured.Unstructured)
		if gvr, ok := gvrFromGvk(obj.GroupVersionKind()); ok {
			c.Dynamic.Resource(gvr).Namespace(obj.GetNamespace()).Create(context.Background(), obj, metav1.CreateOptions{})
		}
	}
	c.Discovery = c.Skupper.Discovery()
	if fakeDiscoveryClient, ok := c.Discovery.(*discoveryfake.FakeDiscovery); ok {
		fakeDiscoveryClient.Resources = append(fakeDiscoveryClient.Resources, fakedApiResources()...)
	}
	c.Route = routefake.NewSimpleClientset(routes...)

	return c, nil
}
func gvrFromGvk(gvk schema.GroupVersionKind) (schema.GroupVersionResource, bool) {
	switch gvk.Group {
	case "projectcountour.io":
		if gvk.Kind == "HTTPProxy" {
			return resource.ContourHttpProxyResource(), true
		}
	case "gateway.networking.k8s.io":
		if gvk.Kind == "TLSRoute" {
			return resource.TlsRouteResource(), true
		}
		if gvk.Kind == "Gateway" {
			return resource.GatewayResource(), true
		}
	}
	return schema.GroupVersionResource{}, false
}
func fakedApiResources() []*metav1.APIResourceList {
	return []*metav1.APIResourceList{
		{
			GroupVersion: "projectcontour.io/v1",
			APIResources: []metav1.APIResource{
				{
					Name:         "httpproxies",
					SingularName: "httpproxy",
					Namespaced:   true,
					Group:        "projectcontour.io",
					Version:      "v1",
					Kind:         "HTTPProxy",
				},
			},
		},
		{
			GroupVersion: "gateway.networking.k8s.io/v1alpha2",
			APIResources: []metav1.APIResource{
				{
					Name:         "tlsroutes",
					SingularName: "tlsroute",
					Namespaced:   true,
					Group:        "gateway.networking.k8s.io",
					Version:      "v1alpha2",
					Kind:         "TLSRoute",
				},
			},
		},
		{
			GroupVersion: "gateway.networking.k8s.io/v1",
			APIResources: []metav1.APIResource{
				{
					Name:         "gateways",
					SingularName: "gateway",
					Namespaced:   true,
					Group:        "gateway.networking.k8s.io",
					Version:      "v1",
					Kind:         "Gateway",
				},
			},
		},
	}
}

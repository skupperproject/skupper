package securedaccess

import (
	"context"
	"errors"
	"reflect"
	"testing"

	routev1 "github.com/openshift/api/route/v1"
	fakeroute "github.com/openshift/client-go/route/clientset/versioned/typed/route/v1/fake"
	internalclient "github.com/skupperproject/skupper/internal/kube/client"
	fakeclient "github.com/skupperproject/skupper/internal/kube/client/fake"
	"github.com/skupperproject/skupper/internal/kube/watchers"
	skupperv2alpha1 "github.com/skupperproject/skupper/pkg/apis/skupper/v2alpha1"
	fakev2alpha1 "github.com/skupperproject/skupper/pkg/generated/client/clientset/versioned/typed/skupper/v2alpha1/fake"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/intstr"
	fakedynamic "k8s.io/client-go/dynamic/fake"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

type ExpectedHttpProxy struct {
	HttpProxy
	namespace string
}

func TestSecuredAccessRecovery(t *testing.T) {
	testTable := []struct {
		name                 string
		config               Config
		k8sObjects           []runtime.Object
		skupperObjects       []runtime.Object
		errors               []ClientError
		expectedServices     []*corev1.Service
		expectedRoutes       []*routev1.Route
		expectedIngresses    []*networkingv1.Ingress
		expectedProxies      []ExpectedHttpProxy
		expectedCertificates []MockCertificate
		expectedStatuses     []*skupperv2alpha1.SecuredAccess
	}{
		{
			name: "simple loadbalancer recovery",
			config: Config{
				EnabledAccessTypes: []string{"loadbalancer"},
			},
			k8sObjects: []runtime.Object{
				addLoadbalancerIP(service("mysvc", "test", selector(), corev1.ServiceTypeLoadBalancer, servicePorts()), "10.1.1.10"),
			},
			skupperObjects: []runtime.Object{
				securedAccess("mysvc", "test", selector(), securedAccessPorts()),
			},
			expectedServices: []*corev1.Service{
				service("mysvc", "test", selector(), corev1.ServiceTypeLoadBalancer, servicePorts()),
			},
			expectedStatuses: []*skupperv2alpha1.SecuredAccess{
				status("mysvc", "test", "OK", endpoint("a", "8080", "10.1.1.10"), endpoint("b", "9090", "10.1.1.10")),
			},
		},
		{
			name: "create loadbalancer on recovery",
			config: Config{
				EnabledAccessTypes: []string{"loadbalancer"},
			},
			skupperObjects: []runtime.Object{
				securedAccess("mysvc", "test", selector(), securedAccessPorts()),
			},
			expectedServices: []*corev1.Service{
				service("mysvc", "test", selector(), corev1.ServiceTypeLoadBalancer, servicePorts()),
			},
		},
		{
			name: "update service ports on recovery",
			config: Config{
				EnabledAccessTypes: []string{"loadbalancer"},
			},
			k8sObjects: []runtime.Object{
				addLoadbalancerIP(service("mysvc", "test", selector(), corev1.ServiceTypeLoadBalancer, servicePortsFromMap(map[string]int{"foo": 1234})), "10.1.1.10"),
			},
			skupperObjects: []runtime.Object{
				securedAccess("mysvc", "test", selector(), securedAccessPorts()),
			},
			expectedServices: []*corev1.Service{
				service("mysvc", "test", selector(), corev1.ServiceTypeLoadBalancer, servicePorts()),
			},
			expectedStatuses: []*skupperv2alpha1.SecuredAccess{
				status("mysvc", "test", "OK", endpoint("a", "8080", "10.1.1.10"), endpoint("b", "9090", "10.1.1.10")),
			},
		},
		{
			name: "update service port with same name on recovery",
			config: Config{
				EnabledAccessTypes: []string{"loadbalancer"},
			},
			k8sObjects: []runtime.Object{
				addLoadbalancerIP(service("mysvc", "test", selector(), corev1.ServiceTypeLoadBalancer, servicePortsFromMap(map[string]int{"a": 1234})), "10.1.1.10"),
			},
			skupperObjects: []runtime.Object{
				securedAccess("mysvc", "test", selector(), securedAccessPorts()),
			},
			expectedServices: []*corev1.Service{
				service("mysvc", "test", selector(), corev1.ServiceTypeLoadBalancer, servicePorts()),
			},
			expectedStatuses: []*skupperv2alpha1.SecuredAccess{
				status("mysvc", "test", "OK", endpoint("a", "8080", "10.1.1.10"), endpoint("b", "9090", "10.1.1.10")),
			},
		},
		{
			name: "update selector on recovery",
			config: Config{
				EnabledAccessTypes: []string{"loadbalancer"},
			},
			k8sObjects: []runtime.Object{
				addLoadbalancerIP(service("mysvc", "test", map[string]string{"x": "y"}, corev1.ServiceTypeLoadBalancer, servicePorts()), "10.1.1.10"),
			},
			skupperObjects: []runtime.Object{
				securedAccess("mysvc", "test", selector(), securedAccessPorts()),
			},
			expectedServices: []*corev1.Service{
				service("mysvc", "test", selector(), corev1.ServiceTypeLoadBalancer, servicePorts()),
			},
			expectedStatuses: []*skupperv2alpha1.SecuredAccess{
				status("mysvc", "test", "OK", endpoint("a", "8080", "10.1.1.10"), endpoint("b", "9090", "10.1.1.10")),
			},
		},
		{
			name: "update service type on recovery",
			config: Config{
				EnabledAccessTypes: []string{"loadbalancer"},
			},
			k8sObjects: []runtime.Object{
				addLoadbalancerIP(service("mysvc", "test", selector(), "", servicePorts()), "10.1.1.10"),
			},
			skupperObjects: []runtime.Object{
				securedAccess("mysvc", "test", selector(), securedAccessPorts()),
			},
			expectedServices: []*corev1.Service{
				service("mysvc", "test", selector(), corev1.ServiceTypeLoadBalancer, servicePorts()),
			},
			expectedStatuses: []*skupperv2alpha1.SecuredAccess{
				status("mysvc", "test", "OK", endpoint("a", "8080", "10.1.1.10"), endpoint("b", "9090", "10.1.1.10")),
			},
		},
		{
			name: "create routes on recovery",
			config: Config{
				EnabledAccessTypes: []string{"route"},
			},
			k8sObjects: []runtime.Object{
				service("mysvc", "test", selector(), "", servicePorts()),
			},
			skupperObjects: []runtime.Object{
				securedAccess("mysvc", "test", selector(), securedAccessPorts()),
			},
			expectedServices: []*corev1.Service{
				service("mysvc", "test", selector(), "", servicePorts()),
			},
			expectedRoutes: []*routev1.Route{
				route("mysvc-a", "test", "mysvc", "a", ""),
				route("mysvc-b", "test", "mysvc", "b", ""),
			},
			expectedStatuses: []*skupperv2alpha1.SecuredAccess{
				status("mysvc", "test", "Pending"),
			},
		},
		{
			name: "update routes on recovery",
			config: Config{
				EnabledAccessTypes: []string{"route"},
			},
			k8sObjects: []runtime.Object{
				service("mysvc", "test", selector(), "", servicePorts()),
				route("mysvc-a", "test", "hoo", "foo", ""),
				route("mysvc-b", "test", "hah", "bar", ""),
			},
			skupperObjects: []runtime.Object{
				securedAccess("mysvc", "test", selector(), securedAccessPorts()),
			},
			expectedServices: []*corev1.Service{
				service("mysvc", "test", selector(), "", servicePorts()),
			},
			expectedRoutes: []*routev1.Route{
				route("mysvc-a", "test", "mysvc", "a", ""),
				route("mysvc-b", "test", "mysvc", "b", ""),
			},
			expectedStatuses: []*skupperv2alpha1.SecuredAccess{
				status("mysvc", "test", "Pending"),
			},
		},
		{
			name: "create ingress on recovery",
			config: Config{
				EnabledAccessTypes: []string{"ingress-nginx"},
			},
			k8sObjects: []runtime.Object{
				service("mysvc", "test", selector(), "", servicePorts()),
			},
			skupperObjects: []runtime.Object{
				securedAccess("mysvc", "test", selector(), securedAccessPorts()),
			},
			expectedServices: []*corev1.Service{
				service("mysvc", "test", selector(), "", servicePorts()),
			},
			expectedIngresses: []*networkingv1.Ingress{
				ingress("mysvc", "test", ingressRule("a.test", "mysvc", 8080), ingressRule("b.test", "mysvc", 9090)),
			},
			expectedStatuses: []*skupperv2alpha1.SecuredAccess{
				status("mysvc", "test", "Pending"),
			},
		},
		{
			name: "update ingress on recovery",
			config: Config{
				EnabledAccessTypes: []string{"ingress-nginx"},
			},
			k8sObjects: []runtime.Object{
				service("mysvc", "test", selector(), "", servicePorts()),
				ingress("mysvc", "test", ingressRule("a.test", "foo", 1111), ingressRule("b.test", "bar", 2222)),
			},
			skupperObjects: []runtime.Object{
				securedAccess("mysvc", "test", selector(), securedAccessPorts()),
			},
			expectedServices: []*corev1.Service{
				service("mysvc", "test", selector(), "", servicePorts()),
			},
			expectedIngresses: []*networkingv1.Ingress{
				ingress("mysvc", "test", ingressRule("a.test", "mysvc", 8080), ingressRule("b.test", "mysvc", 9090)),
			},
			expectedStatuses: []*skupperv2alpha1.SecuredAccess{
				status("mysvc", "test", "Pending"),
			},
		},
		{
			name: "delete redundant ingress",
			config: Config{
				EnabledAccessTypes: []string{"ingress-nginx"},
			},
			k8sObjects: []runtime.Object{
				service("mysvc", "test", selector(), "", servicePorts()),
				ingress("mysvc", "test", ingressRule("a.test", "foo", 1111), ingressRule("b.test", "bar", 2222)),
				ingress("foo", "test", ingressRule("a.test", "foo", 1111), ingressRule("b.test", "bar", 2222)),
			},
			skupperObjects: []runtime.Object{
				securedAccess("mysvc", "test", selector(), securedAccessPorts()),
			},
			expectedServices: []*corev1.Service{
				service("mysvc", "test", selector(), "", servicePorts()),
			},
			expectedIngresses: []*networkingv1.Ingress{
				ingress("mysvc", "test", ingressRule("a.test", "mysvc", 8080), ingressRule("b.test", "mysvc", 9090)),
			},
			expectedStatuses: []*skupperv2alpha1.SecuredAccess{
				status("mysvc", "test", "Pending"),
			},
		},
		{
			name: "replace routes with ingress on recovery",
			config: Config{
				EnabledAccessTypes: []string{"route", "ingress-nginx"},
			},
			k8sObjects: []runtime.Object{
				service("mysvc", "test", selector(), "", servicePorts()),
				route("mysvc-a", "test", "mysvc", "a", ""),
				route("mysvc-b", "test", "mysvc", "b", ""),
			},
			skupperObjects: []runtime.Object{
				securedAccessWithType("mysvc", "test", "ingress-nginx", selector(), securedAccessPorts()),
			},
			expectedServices: []*corev1.Service{
				service("mysvc", "test", selector(), "", servicePorts()),
			},
			expectedIngresses: []*networkingv1.Ingress{
				ingress("mysvc", "test", ingressRule("a.test", "mysvc", 8080), ingressRule("b.test", "mysvc", 9090)),
			},
			expectedStatuses: []*skupperv2alpha1.SecuredAccess{
				status("mysvc", "test", "Pending"),
			},
		},
		{
			name: "replace ingress with routes on recovery",
			config: Config{
				EnabledAccessTypes: []string{"route", "ingress-nginx"},
			},
			k8sObjects: []runtime.Object{
				service("mysvc", "test", selector(), "", servicePorts()),
				ingress("mysvc", "test", ingressRule("a.test", "mysvc", 8080), ingressRule("b.test", "mysvc", 9090)),
			},
			skupperObjects: []runtime.Object{
				securedAccessWithType("mysvc", "test", "route", selector(), securedAccessPorts()),
			},
			expectedServices: []*corev1.Service{
				service("mysvc", "test", selector(), "", servicePorts()),
			},
			expectedRoutes: []*routev1.Route{
				route("mysvc-a", "test", "mysvc", "a", ""),
				route("mysvc-b", "test", "mysvc", "b", ""),
			},
			expectedStatuses: []*skupperv2alpha1.SecuredAccess{
				status("mysvc", "test", "Pending"),
			},
		},
		{
			name: "http proxy recovery",
			config: Config{
				EnabledAccessTypes: []string{"contour-http-proxy"},
			},
			k8sObjects: []runtime.Object{
				service("mysvc", "test", selector(), "", servicePorts()),
				httpProxy("mysvc-a", "test", "mysvc-a.test", "mysvc", 8080),
				httpProxy("mysvc-b", "test", "mysvc-b.test", "mysvc", 9090),
			},
			skupperObjects: []runtime.Object{
				securedAccess("mysvc", "test", selector(), securedAccessPorts()),
			},
			expectedServices: []*corev1.Service{
				service("mysvc", "test", selector(), "", servicePorts()),
			},
			expectedProxies: []ExpectedHttpProxy{
				expectedHttpProxy("mysvc-a", "test", "mysvc-a.test", "mysvc", 8080),
				expectedHttpProxy("mysvc-b", "test", "mysvc-b.test", "mysvc", 9090),
			},
			expectedStatuses: []*skupperv2alpha1.SecuredAccess{
				status("mysvc", "test", "OK", endpoint("a", "443", "mysvc-a.test"), endpoint("b", "443", "mysvc-b.test")),
			},
		},
		{
			name: "create http proxies on recovery",
			config: Config{
				EnabledAccessTypes: []string{"contour-http-proxy"},
			},
			k8sObjects: []runtime.Object{
				service("mysvc", "test", selector(), "", servicePorts()),
			},
			skupperObjects: []runtime.Object{
				securedAccess("mysvc", "test", selector(), securedAccessPorts()),
			},
			expectedServices: []*corev1.Service{
				service("mysvc", "test", selector(), "", servicePorts()),
			},
			expectedProxies: []ExpectedHttpProxy{
				expectedHttpProxy("mysvc-a", "test", "mysvc-a.test", "mysvc", 8080),
				expectedHttpProxy("mysvc-b", "test", "mysvc-b.test", "mysvc", 9090),
			},
			expectedStatuses: []*skupperv2alpha1.SecuredAccess{
				status("mysvc", "test", "OK", endpoint("a", "443", "mysvc-a.test"), endpoint("b", "443", "mysvc-b.test")),
			},
		},
		{
			name: "update http proxies on recovery",
			config: Config{
				EnabledAccessTypes: []string{"contour-http-proxy"},
			},
			k8sObjects: []runtime.Object{
				service("mysvc", "test", selector(), "", servicePorts()),
				httpProxy("mysvc-a", "test", "foo", "mysvc", 1234),
				httpProxy("mysvc-b", "test", "foo", "mysvc", 5678),
			},
			skupperObjects: []runtime.Object{
				securedAccess("mysvc", "test", selector(), securedAccessPorts()),
			},
			expectedServices: []*corev1.Service{
				service("mysvc", "test", selector(), "", servicePorts()),
			},
			expectedProxies: []ExpectedHttpProxy{
				expectedHttpProxy("mysvc-a", "test", "mysvc-a.test", "mysvc", 8080),
				expectedHttpProxy("mysvc-b", "test", "mysvc-b.test", "mysvc", 9090),
			},
			expectedStatuses: []*skupperv2alpha1.SecuredAccess{
				status("mysvc", "test", "OK", endpoint("a", "443", "mysvc-a.test"), endpoint("b", "443", "mysvc-b.test")),
			},
		},
		{
			name: "update http proxies for port change",
			config: Config{
				EnabledAccessTypes: []string{"contour-http-proxy"},
			},
			k8sObjects: []runtime.Object{
				service("mysvc", "test", selector(), "", servicePorts()),
				httpProxy("mysvc-a", "test", "mysvc-a.test", "mysvc", 8080),
				httpProxy("mysvc-c", "test", "mysvc-c.test", "mysvc", 9090),
			},
			skupperObjects: []runtime.Object{
				securedAccess("mysvc", "test", selector(), securedAccessPorts()),
			},
			expectedServices: []*corev1.Service{
				service("mysvc", "test", selector(), "", servicePorts()),
			},
			expectedProxies: []ExpectedHttpProxy{
				expectedHttpProxy("mysvc-a", "test", "mysvc-a.test", "mysvc", 8080),
				expectedHttpProxy("mysvc-b", "test", "mysvc-b.test", "mysvc", 9090),
			},
			expectedStatuses: []*skupperv2alpha1.SecuredAccess{
				status("mysvc", "test", "OK", endpoint("a", "443", "mysvc-a.test"), endpoint("b", "443", "mysvc-b.test")),
			},
		},
		{
			name: "delete redundant http proxy",
			config: Config{
				EnabledAccessTypes: []string{"contour-http-proxy"},
			},
			k8sObjects: []runtime.Object{
				service("mysvc", "test", selector(), "", servicePorts()),
				httpProxy("mysvc-a", "test", "mysvc-a.test", "mysvc", 8080),
				httpProxy("mysvc-b", "test", "mysvc-b.test", "mysvc", 9090),
				httpProxy("mysvc-c", "test", "mysvc-c.test", "mysvc", 7070),
			},
			skupperObjects: []runtime.Object{
				securedAccess("mysvc", "test", selector(), securedAccessPorts()),
			},
			expectedServices: []*corev1.Service{
				service("mysvc", "test", selector(), "", servicePorts()),
			},
			expectedProxies: []ExpectedHttpProxy{
				expectedHttpProxy("mysvc-a", "test", "mysvc-a.test", "mysvc", 8080),
				expectedHttpProxy("mysvc-b", "test", "mysvc-b.test", "mysvc", 9090),
			},
			expectedStatuses: []*skupperv2alpha1.SecuredAccess{
				status("mysvc", "test", "OK", endpoint("a", "443", "mysvc-a.test"), endpoint("b", "443", "mysvc-b.test")),
			},
		},
		{
			name: "replace http proxies with routes on recovery",
			config: Config{
				EnabledAccessTypes: []string{"route", "contour-http-proxy"},
			},
			k8sObjects: []runtime.Object{
				service("mysvc", "test", selector(), "", servicePorts()),
				httpProxy("mysvc-a", "test", "mysvc", "a", 8080),
				httpProxy("mysvc-b", "test", "mysvc", "b", 9090),
			},
			skupperObjects: []runtime.Object{
				securedAccessWithType("mysvc", "test", "route", selector(), securedAccessPorts()),
			},
			expectedServices: []*corev1.Service{
				service("mysvc", "test", selector(), "", servicePorts()),
			},
			expectedRoutes: []*routev1.Route{
				route("mysvc-a", "test", "mysvc", "a", ""),
				route("mysvc-b", "test", "mysvc", "b", ""),
			},
			expectedStatuses: []*skupperv2alpha1.SecuredAccess{
				status("mysvc", "test", "Pending"),
			},
		},
		{
			name: "remove redundant service",
			config: Config{
				EnabledAccessTypes: []string{"route"},
			},
			k8sObjects: []runtime.Object{
				service("foo", "test", selector(), "", servicePorts()),
			},
			skupperObjects: []runtime.Object{
				securedAccessWithType("mysvc", "test", "route", selector(), securedAccessPorts()),
			},
			expectedServices: []*corev1.Service{
				service("mysvc", "test", selector(), "", servicePorts()),
			},
			expectedRoutes: []*routev1.Route{
				route("mysvc-a", "test", "mysvc", "a", ""),
				route("mysvc-b", "test", "mysvc", "b", ""),
			},
			expectedStatuses: []*skupperv2alpha1.SecuredAccess{
				status("mysvc", "test", "Pending"),
			},
		},
		{
			name: "recover tlsroute",
			config: Config{
				EnabledAccessTypes: []string{"gateway"},
				GatewayClass:       "contour",
			},
			k8sObjects: []runtime.Object{
				service("mysvc", "test", selector(), "", servicePorts()),
				tlsroute("mysvc-a", "test"),
				tlsroute("mysvc-b", "test"),
			},
			skupperObjects: []runtime.Object{
				securedAccess("mysvc", "test", selector(), securedAccessPorts()),
			},
			expectedServices: []*corev1.Service{
				service("mysvc", "test", selector(), "", servicePorts()),
			},
		},
		{
			name: "error creating routes",
			config: Config{
				EnabledAccessTypes: []string{"route"},
			},
			skupperObjects: []runtime.Object{
				securedAccess("mysvc", "test", selector(), securedAccessPorts()),
			},
			errors: []ClientError{
				RouteClientError("create", "routes", "create is blocked"),
			},
			expectedServices: []*corev1.Service{
				service("mysvc", "test", selector(), "", servicePorts()),
			},
			expectedStatuses: []*skupperv2alpha1.SecuredAccess{
				status("mysvc", "test", "create is blocked"),
			},
		},
		{
			name: "error updating routes",
			config: Config{
				EnabledAccessTypes: []string{"route"},
			},
			k8sObjects: []runtime.Object{
				route("mysvc-a", "test", "mysvc", "x", ""),
				route("mysvc-b", "test", "mysvc", "y", ""),
			},
			skupperObjects: []runtime.Object{
				securedAccess("mysvc", "test", selector(), securedAccessPorts()),
			},
			errors: []ClientError{
				RouteClientError("update", "routes", "update is blocked"),
			},
			expectedServices: []*corev1.Service{
				service("mysvc", "test", selector(), "", servicePorts()),
			},
			expectedRoutes: []*routev1.Route{
				route("mysvc-a", "test", "mysvc", "x", ""),
				route("mysvc-b", "test", "mysvc", "y", ""),
			},
			expectedStatuses: []*skupperv2alpha1.SecuredAccess{
				status("mysvc", "test", "update is blocked"),
			},
		},
		{
			name: "error creating service",
			config: Config{
				EnabledAccessTypes: []string{"local"},
			},
			skupperObjects: []runtime.Object{
				securedAccess("mysvc", "test", selector(), securedAccessPorts()),
			},
			errors: []ClientError{
				CoreClientError("create", "services", "service create is blocked"),
			},
			expectedStatuses: []*skupperv2alpha1.SecuredAccess{
				status("mysvc", "test", "service create is blocked"),
			},
		},
		{
			name: "create ingress on recovery",
			config: Config{
				EnabledAccessTypes: []string{"ingress-nginx"},
			},
			k8sObjects: []runtime.Object{
				service("mysvc", "test", selector(), "", servicePorts()),
			},
			skupperObjects: []runtime.Object{
				securedAccess("mysvc", "test", selector(), securedAccessPorts()),
			},
			errors: []ClientError{
				CoreClientError("create", "ingresses", "ingress create is blocked"),
			},
			expectedServices: []*corev1.Service{
				service("mysvc", "test", selector(), "", servicePorts()),
			},
			expectedStatuses: []*skupperv2alpha1.SecuredAccess{
				status("mysvc", "test", "ingress create is blocked"),
			},
		},
		{
			name: "error updating ingress",
			config: Config{
				EnabledAccessTypes: []string{"ingress-nginx"},
			},
			k8sObjects: []runtime.Object{
				service("mysvc", "test", selector(), "", servicePorts()),
				ingress("mysvc", "test", ingressRule("a.test", "foo", 1111), ingressRule("b.test", "bar", 2222)),
			},
			skupperObjects: []runtime.Object{
				securedAccess("mysvc", "test", selector(), securedAccessPorts()),
			},
			errors: []ClientError{
				CoreClientError("update", "ingresses", "ingress update is blocked"),
			},
			expectedServices: []*corev1.Service{
				service("mysvc", "test", selector(), "", servicePorts()),
			},
			expectedIngresses: []*networkingv1.Ingress{
				ingress("mysvc", "test", ingressRule("a.test", "foo", 1111), ingressRule("b.test", "bar", 2222)),
			},
			expectedStatuses: []*skupperv2alpha1.SecuredAccess{
				status("mysvc", "test", "ingress update is blocked"),
			},
		},
		{
			name: "bad structure for http proxies",
			config: Config{
				EnabledAccessTypes: []string{"contour-http-proxy"},
			},
			k8sObjects: []runtime.Object{
				service("mysvc", "test", selector(), "", servicePorts()),
				httpProxyWithContent("mysvc-a", "test", map[string]interface{}{"spec": false}),
				httpProxyWithContent("mysvc-b", "test", map[string]interface{}{"spec": map[string]interface{}{"virtualhost": map[string]interface{}{"tls": false}}}),
			},
			skupperObjects: []runtime.Object{
				securedAccessWithType("mysvc", "test", "contour-http-proxy", selector(), securedAccessPorts()),
			},
			expectedServices: []*corev1.Service{
				service("mysvc", "test", selector(), "", servicePorts()),
			},
			expectedProxies: []ExpectedHttpProxy{
				expectedHttpProxy("mysvc-a", "test", "", "", 0),
				expectedHttpProxy("mysvc-b", "test", "", "", 0),
			},
			expectedStatuses: []*skupperv2alpha1.SecuredAccess{
				status("mysvc", "test", "Unexpected structure for HTTPProxy"),
			},
		},
		{
			name: "error updating http proxies",
			config: Config{
				EnabledAccessTypes: []string{"contour-http-proxy"},
			},
			k8sObjects: []runtime.Object{
				service("mysvc", "test", selector(), "", servicePorts()),
				httpProxy("mysvc-a", "test", "mysvc-a.test", "mysvc", 1234),
				httpProxy("mysvc-b", "test", "mysvc-b.test", "mysvc", 4321),
			},
			skupperObjects: []runtime.Object{
				securedAccessWithType("mysvc", "test", "contour-http-proxy", selector(), securedAccessPorts()),
			},
			errors: []ClientError{
				DynamicClientError("update", "httpproxies", "http proxy update is blocked"),
			},
			expectedServices: []*corev1.Service{
				service("mysvc", "test", selector(), "", servicePorts()),
			},
			expectedProxies: []ExpectedHttpProxy{
				expectedHttpProxy("mysvc-a", "test", "mysvc-a.test", "mysvc", 1234),
				expectedHttpProxy("mysvc-b", "test", "mysvc-b.test", "mysvc", 4321),
			},
			expectedStatuses: []*skupperv2alpha1.SecuredAccess{
				status("mysvc", "test", "http proxy update is blocked"),
			},
		},
		{
			name: "error creating http proxies",
			config: Config{
				EnabledAccessTypes: []string{"contour-http-proxy"},
			},
			skupperObjects: []runtime.Object{
				securedAccessWithType("mysvc", "test", "contour-http-proxy", selector(), securedAccessPorts()),
			},
			errors: []ClientError{
				DynamicClientError("create", "httpproxies", "http proxy create is blocked"),
			},
			expectedServices: []*corev1.Service{
				service("mysvc", "test", selector(), "", servicePorts()),
			},
			expectedStatuses: []*skupperv2alpha1.SecuredAccess{
				status("mysvc", "test", "http proxy create is blocked"),
			},
		},
		{
			name: "status update error",
			config: Config{
				EnabledAccessTypes: []string{"loadbalancer"},
			},
			k8sObjects: []runtime.Object{
				addLoadbalancerIP(service("mysvc", "test", selector(), corev1.ServiceTypeLoadBalancer, servicePorts()), "10.1.1.10"),
			},
			skupperObjects: []runtime.Object{
				securedAccess("mysvc", "test", selector(), securedAccessPorts()),
			},
			errors: []ClientError{
				SkupperClientError("update", "*", "status update is blocked"),
			},
			expectedServices: []*corev1.Service{
				service("mysvc", "test", selector(), corev1.ServiceTypeLoadBalancer, servicePorts()),
			},
			expectedStatuses: []*skupperv2alpha1.SecuredAccess{
				status("mysvc", "test", ""),
			},
		},
	}
	for _, tt := range testTable {
		t.Run(tt.name, func(t *testing.T) {
			client, err := fakeclient.NewFakeClient("test", tt.k8sObjects, tt.skupperObjects, "")
			if err != nil {
				assert.Assert(t, err)
			}
			for _, e := range tt.errors {
				e.Prepend(client)
			}
			certs := newMockCertificateManager()
			m := NewSecuredAccessManager(client, certs, &tt.config, &FakeControllerContext{namespace: "test"})
			w := NewSecuredAccessResourceWatcher(m)
			controller := watchers.NewEventProcessor("Controller", client)
			w.WatchResources(controller, metav1.NamespaceAll)
			w.WatchSecuredAccesses(controller, metav1.NamespaceAll, func(string, *skupperv2alpha1.SecuredAccess) error { return nil })
			stopCh := make(chan struct{})
			controller.StartWatchers(stopCh)
			controller.WaitForCacheSync(stopCh)
			w.Recover()

			controller.TestProcessAll()

			for _, desired := range tt.expectedServices {
				actual, err := client.GetKubeClient().CoreV1().Services(desired.Namespace).Get(context.Background(), desired.Name, metav1.GetOptions{})
				assert.Assert(t, err)
				assert.DeepEqual(t, desired.Spec.Selector, actual.Spec.Selector)
				for _, port := range desired.Spec.Ports {
					assert.Assert(t, cmp.Contains(actual.Spec.Ports, port))
				}
				assert.Equal(t, len(desired.Spec.Ports), len(actual.Spec.Ports))
				assert.DeepEqual(t, desired.Spec.Type, actual.Spec.Type)
				if !reflect.DeepEqual(desired.Status, actual.Status) {
					actual.Status = desired.Status
					actual, err = client.GetKubeClient().CoreV1().Services(desired.Namespace).UpdateStatus(context.Background(), actual, metav1.UpdateOptions{})
					assert.Assert(t, err)
				}
			}
			services, err := client.GetKubeClient().CoreV1().Services(metav1.NamespaceAll).List(context.Background(), metav1.ListOptions{})
			assert.Assert(t, err)
			assert.Equal(t, len(tt.expectedServices), len(services.Items), "wrong number of services")
			for _, desired := range tt.expectedRoutes {
				actual, err := client.GetRouteClient().Routes(desired.Namespace).Get(context.Background(), desired.Name, metav1.GetOptions{})
				assert.Assert(t, err)
				assert.DeepEqual(t, desired.Spec, actual.Spec)
			}
			routes, err := client.GetRouteClient().Routes(metav1.NamespaceAll).List(context.Background(), metav1.ListOptions{})
			assert.Assert(t, err)
			assert.Equal(t, len(tt.expectedRoutes), len(routes.Items), "wrong number of routes")
			for _, desired := range tt.expectedIngresses {
				actual, err := client.GetKubeClient().NetworkingV1().Ingresses(desired.Namespace).Get(context.Background(), desired.Name, metav1.GetOptions{})
				assert.Assert(t, err)
				for _, rule := range desired.Spec.Rules {
					assert.Assert(t, cmp.Contains(actual.Spec.Rules, rule))
				}
				assert.Equal(t, len(desired.Spec.Rules), len(actual.Spec.Rules))
			}
			ingresses, err := client.GetKubeClient().NetworkingV1().Ingresses(metav1.NamespaceAll).List(context.Background(), metav1.ListOptions{})
			assert.Assert(t, err)
			assert.Equal(t, len(tt.expectedIngresses), len(ingresses.Items), "wrong number of ingresses")
			for _, desired := range tt.expectedProxies {
				obj, err := client.GetDynamicClient().Resource(httpProxyResource).Namespace(desired.namespace).Get(context.TODO(), desired.Name, metav1.GetOptions{})
				assert.Assert(t, err)
				actual := ExpectedHttpProxy{}
				actual.Name = desired.Name
				actual.namespace = desired.namespace
				actual.readFromContourProxy(obj)
				assert.Equal(t, desired, actual)
			}
			proxies, err := client.GetDynamicClient().Resource(httpProxyResource).Namespace(metav1.NamespaceAll).List(context.TODO(), metav1.ListOptions{})
			assert.Assert(t, err)
			assert.Equal(t, len(tt.expectedProxies), len(proxies.Items), "wrong number of proxies")
			certs.checkCertificates(t, tt.expectedCertificates)
			for _, desired := range tt.expectedStatuses {
				actual, err := client.GetSkupperClient().SkupperV2alpha1().SecuredAccesses(desired.Namespace).Get(context.Background(), desired.Name, metav1.GetOptions{})
				assert.Assert(t, err)
				assert.Equal(t, desired.Status.Message, actual.Status.Message)
				assert.Equal(t, len(desired.Status.Endpoints), len(actual.Status.Endpoints))
				for _, endpoint := range desired.Status.Endpoints {
					assert.Assert(t, cmp.Contains(actual.Status.Endpoints, endpoint))
				}
			}
		})
	}
}

func securedAccessWithType(name string, namespace string, accessType string, selector map[string]string, ports []skupperv2alpha1.SecuredAccessPort) *skupperv2alpha1.SecuredAccess {
	sa := securedAccess(name, namespace, selector, ports)
	sa.Spec.AccessType = accessType
	return sa
}

func securedAccess(name string, namespace string, selector map[string]string, ports []skupperv2alpha1.SecuredAccessPort) *skupperv2alpha1.SecuredAccess {
	return &skupperv2alpha1.SecuredAccess{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "skupper.io/v2alpha1",
			Kind:       "SecuredAccess",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: skupperv2alpha1.SecuredAccessSpec{
			Selector: selector,
			Ports:    ports,
			Issuer:   "skupper-site-ca",
		},
	}
}

func securedAccessPorts() []skupperv2alpha1.SecuredAccessPort {
	return []skupperv2alpha1.SecuredAccessPort{
		{
			Name:       "a",
			Port:       8080,
			TargetPort: 8081,
			Protocol:   "TCP",
		},
		{
			Name:       "b",
			Port:       9090,
			TargetPort: 9191,
			Protocol:   "TCP",
		},
	}
}

func service(name string, namespace string, selector map[string]string, serviceType corev1.ServiceType, ports []corev1.ServicePort) *corev1.Service {
	return &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Service",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				"internal.skupper.io/secured-access": "true",
			},
			Annotations: map[string]string{
				"internal.skupper.io/controlled": "true",
			},
		},
		Spec: corev1.ServiceSpec{
			Selector: selector,
			Type:     serviceType, //ACCESS_TYPE_ROUTE,
			Ports:    ports,
		},
	}
}

func servicePorts() []corev1.ServicePort {
	return []corev1.ServicePort{
		{
			Name:       "a",
			Port:       8080,
			TargetPort: intstr.IntOrString{IntVal: int32(8081)},
			Protocol:   corev1.Protocol("TCP"),
		},
		{
			Name:       "b",
			Port:       9090,
			TargetPort: intstr.IntOrString{IntVal: int32(9191)},
			Protocol:   corev1.Protocol("TCP"),
		},
	}
}

func servicePortsFromMap(ports map[string]int) []corev1.ServicePort {
	var result []corev1.ServicePort
	for key, value := range ports {
		result = append(result,
			corev1.ServicePort{
				Name:       key,
				Port:       int32(value),
				TargetPort: intstr.IntOrString{IntVal: int32(value)},
				Protocol:   corev1.Protocol("TCP"),
			})
	}
	return result
}

func selector() map[string]string {
	return map[string]string{
		"app": "foo",
	}
}

func addLoadbalancerIP(svc *corev1.Service, ip string) *corev1.Service {
	svc.Status.LoadBalancer.Ingress = []corev1.LoadBalancerIngress{
		{
			IP: ip,
		},
	}
	return svc
}

func addLoadbalancerHostname(svc *corev1.Service, hostname string) *corev1.Service {
	svc.Status.LoadBalancer.Ingress = []corev1.LoadBalancerIngress{
		{
			Hostname: hostname,
		},
	}
	return svc
}

func status(name string, namespace string, message string, endpoints ...skupperv2alpha1.Endpoint) *skupperv2alpha1.SecuredAccess {
	sa := securedAccess(name, namespace, nil, nil)
	sa.Status.Message = message
	for _, endpoint := range endpoints {
		sa.Status.Endpoints = append(sa.Status.Endpoints, endpoint)
	}
	return sa
}

func statusOnly(message string, endpoints ...skupperv2alpha1.Endpoint) skupperv2alpha1.SecuredAccessStatus {
	return status("", "", message, endpoints...).Status
}

func endpoint(name string, port string, host string) skupperv2alpha1.Endpoint {
	return skupperv2alpha1.Endpoint{
		Name: name,
		Port: port,
		Host: host,
	}
}

func route(name string, namespace string, svcName string, portName string, host string) *routev1.Route {
	hundred := int32(100)
	return &routev1.Route{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Route",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				"internal.skupper.io/secured-access": "true",
			},
			Annotations: map[string]string{
				"internal.skupper.io/controlled": "true",
			},
		},
		Spec: routev1.RouteSpec{
			Host: host,
			Port: &routev1.RoutePort{
				TargetPort: intstr.FromString(portName),
			},
			To: routev1.RouteTargetReference{
				Kind:   "Service",
				Name:   svcName,
				Weight: &hundred,
			},
			TLS: &routev1.TLSConfig{
				Termination:                   routev1.TLSTerminationPassthrough,
				InsecureEdgeTerminationPolicy: routev1.InsecureEdgeTerminationPolicyNone,
			},
		},
	}
}

func ingress(name string, namespace string, rules ...networkingv1.IngressRule) *networkingv1.Ingress {
	ingress := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				"internal.skupper.io/secured-access": "true",
			},
			Annotations: map[string]string{
				"internal.skupper.io/controlled": "true",
			},
		},
	}
	for _, rule := range rules {
		ingress.Spec.Rules = append(ingress.Spec.Rules, rule)
	}
	return ingress
}

func ingressRule(host string, svc string, port int) networkingv1.IngressRule {
	pathTypePrefix := networkingv1.PathTypePrefix
	return networkingv1.IngressRule{
		Host: host,
		IngressRuleValue: networkingv1.IngressRuleValue{
			HTTP: &networkingv1.HTTPIngressRuleValue{
				Paths: []networkingv1.HTTPIngressPath{
					{
						Path:     "/",
						PathType: &pathTypePrefix,
						Backend: networkingv1.IngressBackend{
							Service: &networkingv1.IngressServiceBackend{
								Name: svc,
								Port: networkingv1.ServiceBackendPort{
									Number: int32(port),
								},
							},
						},
					},
				},
			},
		},
	}
}

func addIngressHostname(ingress *networkingv1.Ingress, hostname string) *networkingv1.Ingress {
	ingress.Status.LoadBalancer.Ingress = []networkingv1.IngressLoadBalancerIngress{
		{
			Hostname: hostname,
		},
	}
	return ingress
}

func addIngressIP(ingress *networkingv1.Ingress, ip string) *networkingv1.Ingress {
	ingress.Status.LoadBalancer.Ingress = []networkingv1.IngressLoadBalancerIngress{
		{
			IP: ip,
		},
	}
	return ingress
}

func httpProxy(name string, namespace string, host string, svc string, port int) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{}
	obj.SetUnstructuredContent(map[string]interface{}{})
	obj.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "projectcountour.io",
		Version: "v1",
		Kind:    "HTTPProxy",
	})
	obj.SetName(name)
	obj.SetNamespace(namespace)
	obj.SetLabels(map[string]string{
		"internal.skupper.io/secured-access": "true",
	})
	obj.SetAnnotations(map[string]string{
		"internal.skupper.io/controlled": "true",
	})
	proxy := &HttpProxy{
		Host:        host,
		ServiceName: svc,
		ServicePort: port,
	}
	proxy.writeToContourProxy(obj)
	return obj
}

func httpProxyWithContent(name string, namespace string, content map[string]interface{}) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{}
	obj.SetUnstructuredContent(content)
	obj.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "projectcountour.io",
		Version: "v1",
		Kind:    "HTTPProxy",
	})
	obj.SetName(name)
	obj.SetNamespace(namespace)
	obj.SetLabels(map[string]string{
		"internal.skupper.io/secured-access": "true",
	})
	obj.SetAnnotations(map[string]string{
		"internal.skupper.io/controlled": "true",
	})
	return obj
}

func expectedHttpProxy(name string, namespace string, host string, svc string, port int) ExpectedHttpProxy {
	return ExpectedHttpProxy{
		HttpProxy: HttpProxy{
			Name:        name,
			Host:        host,
			ServiceName: svc,
			ServicePort: port,
		},
		namespace: namespace,
	}
}

type GatewayUpdate struct {
	gateway  *unstructured.Unstructured
	ip       string
	hostname string
}

func TestGateway(t *testing.T) {
	testTable := []struct {
		name           string
		config         Config
		k8sObjects     []runtime.Object
		namespace      string
		ssaRecorder    *ServerSideApplyRecorder
		expectedStatus skupperv2alpha1.SecuredAccessStatus
		gatewayUpdates []GatewayUpdate
	}{
		{
			name:           "gateway not enabled",
			config:         Config{},
			namespace:      "test",
			expectedStatus: statusOnly("unsupported access type"),
		},
		{
			name: "gateway enabled but failed",
			config: Config{
				EnabledAccessTypes: []string{"gateway"},
				GatewayClass:       "contour",
			},
			namespace:      "test",
			expectedStatus: statusOnly("unsupported access type"),
		},
		{
			name: "gateway enabled",
			config: Config{
				EnabledAccessTypes: []string{"gateway"},
				GatewayClass:       "contour",
			},
			namespace:      "test",
			ssaRecorder:    newServerSideApplyRecorder(),
			expectedStatus: statusOnly("Gateway base domain not yet resolved"),
		},
		{
			name:      "check gateway when not enabled",
			config:    Config{},
			namespace: "test",
			gatewayUpdates: []GatewayUpdate{
				{
					gateway: gateway("skupper", "test"),
				},
			},
			expectedStatus: statusOnly("unsupported access type"),
		},
		{
			name: "gateway enabled and preexists",
			config: Config{
				EnabledAccessTypes: []string{"gateway"},
				GatewayClass:       "contour",
			},
			k8sObjects: []runtime.Object{
				gateway("skupper", "test"),
			},
			namespace:      "test",
			ssaRecorder:    newServerSideApplyRecorder(),
			expectedStatus: statusOnly("Gateway base domain not yet resolved"),
		},
		{
			name: "gateway preexists but not enabled",
			k8sObjects: []runtime.Object{
				gateway("skupper", "test"),
			},
			namespace:      "test",
			ssaRecorder:    newServerSideApplyRecorder(),
			expectedStatus: statusOnly("unsupported access type"),
		},
		{
			name: "gateway status has ip",
			config: Config{
				EnabledAccessTypes: []string{"gateway"},
				GatewayClass:       "contour",
				GatewayPort:        7777,
			},
			k8sObjects: []runtime.Object{
				gateway("skupper", "test"),
			},
			namespace:      "test",
			ssaRecorder:    newServerSideApplyRecorder().setGatewayIP("test/skupper", "10.20.20.10"),
			expectedStatus: statusOnly("OK", endpoint("a", "7777", "mysvc-a.test.10.20.20.10.nip.io"), endpoint("b", "7777", "mysvc-b.test.10.20.20.10.nip.io")),
		},
		{
			name: "apply blocked for tlsroute",
			config: Config{
				EnabledAccessTypes: []string{"gateway"},
				GatewayClass:       "contour",
				GatewayPort:        7777,
			},
			k8sObjects: []runtime.Object{
				gateway("skupper", "test"),
			},
			namespace:      "test",
			ssaRecorder:    newServerSideApplyRecorder().setGatewayIP("test/skupper", "10.20.20.10").setError("test/mysvc-a", "apply blocked"),
			expectedStatus: statusOnly("apply blocked"),
		},
		{
			name: "handle unreconciled instances once gateway is resolved",
			config: Config{
				EnabledAccessTypes: []string{"gateway"},
				GatewayClass:       "contour",
				GatewayPort:        5555,
			},
			k8sObjects: []runtime.Object{
				gateway("skupper", "test"),
			},
			namespace:   "test",
			ssaRecorder: newServerSideApplyRecorder(),
			gatewayUpdates: []GatewayUpdate{
				{
					gateway:  gateway("skupper", "test"),
					hostname: "mygateway.net",
				},
			},
			expectedStatus: statusOnly("OK", endpoint("a", "5555", "mysvc-a.test.mygateway.net"), endpoint("b", "5555", "mysvc-b.test.mygateway.net")),
		},
	}
	for _, tt := range testTable {
		t.Run(tt.name, func(t *testing.T) {
			skupperObjects := []runtime.Object{
				securedAccess("mysvc", "test", selector(), securedAccessPorts()),
			}
			client, err := fakeclient.NewFakeClient("test", tt.k8sObjects, skupperObjects, "")
			if err != nil {
				assert.Assert(t, err)
			}
			if tt.ssaRecorder != nil {
				assert.Assert(t, tt.ssaRecorder.enable(client.GetDynamicClient()))
			}
			certs := newMockCertificateManager()
			m := NewSecuredAccessManager(client, certs, &tt.config, &FakeControllerContext{namespace: tt.namespace})
			w := NewSecuredAccessResourceWatcher(m)
			controller := watchers.NewEventProcessor("Controller", client)
			w.WatchResources(controller, metav1.NamespaceAll)
			w.WatchSecuredAccesses(controller, metav1.NamespaceAll, func(string, *skupperv2alpha1.SecuredAccess) error { return nil })
			w.WatchGateway(controller, tt.namespace)
			stopCh := make(chan struct{})
			controller.StartWatchers(stopCh)
			controller.WaitForCacheSync(stopCh)
			w.Recover()
			for _, update := range tt.gatewayUpdates {
				if update.hostname != "" {
					tt.ssaRecorder.setGatewayHostname("test/skupper", update.hostname)
				}
				if update.ip != "" {
					tt.ssaRecorder.setGatewayIP("test/skupper", update.ip)
				}
				if update.gateway != nil {
					m.CheckGateway(update.gateway.GetNamespace()+"/"+update.gateway.GetName(), update.gateway)
				}
			}
			actual, err := client.GetSkupperClient().SkupperV2alpha1().SecuredAccesses("test").Get(context.Background(), "mysvc", metav1.GetOptions{})
			assert.Assert(t, err)
			assert.Equal(t, tt.expectedStatus.Message, actual.Status.Message)
			assert.Equal(t, len(tt.expectedStatus.Endpoints), len(actual.Status.Endpoints))
			for _, endpoint := range tt.expectedStatus.Endpoints {
				assert.Assert(t, cmp.Contains(actual.Status.Endpoints, endpoint))
			}
		})
	}
}

type FakeClient interface {
	PrependReactor(verb, resource string, reaction k8stesting.ReactionFunc)
}

type ClientError struct {
	scope    func(internalclient.Clients) FakeClient
	verb     string
	resource string
	err      string
}

func (e *ClientError) Prepend(clients internalclient.Clients) {
	e.scope(clients).PrependReactor(e.verb, e.resource, func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, nil, errors.New(e.err)
	})
}

func clientError(verb string, resource string, err string, scope func(internalclient.Clients) FakeClient) ClientError {
	return ClientError{
		scope:    scope,
		verb:     verb,
		resource: resource,
		err:      err,
	}
}

func SkupperClientError(verb string, resource string, err string) ClientError {
	return clientError(verb, resource, err, func(clients internalclient.Clients) FakeClient {
		f, _ := clients.GetSkupperClient().SkupperV2alpha1().(*fakev2alpha1.FakeSkupperV2alpha1)
		return f
	})
}

func RouteClientError(verb string, resource string, err string) ClientError {
	return clientError(verb, resource, err, func(clients internalclient.Clients) FakeClient {
		f, _ := clients.GetRouteClient().(*fakeroute.FakeRouteV1)
		return f
	})
}

func CoreClientError(verb string, resource string, err string) ClientError {
	return clientError(verb, resource, err, func(clients internalclient.Clients) FakeClient {
		f, _ := clients.GetKubeClient().(*k8sfake.Clientset)
		return f
	})
}

func DynamicClientError(verb string, resource string, err string) ClientError {
	return clientError(verb, resource, err, func(clients internalclient.Clients) FakeClient {
		f, _ := clients.GetDynamicClient().(*fakedynamic.FakeDynamicClient)
		return f
	})
}

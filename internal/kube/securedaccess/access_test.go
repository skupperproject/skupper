package securedaccess

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"testing"

	routev1 "github.com/openshift/api/route/v1"
	internalclient "github.com/skupperproject/skupper/internal/kube/client"
	fakeclient "github.com/skupperproject/skupper/internal/kube/client/fake"
	"github.com/skupperproject/skupper/internal/kube/watchers"
	skupperv2alpha1 "github.com/skupperproject/skupper/pkg/apis/skupper/v2alpha1"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/dynamic"
	fakedynamic "k8s.io/client-go/dynamic/fake"
	k8stesting "k8s.io/client-go/testing"
)

func TestSecuredAccessGeneral(t *testing.T) {
	pathTypePrefix := networkingv1.PathTypePrefix
	testTable := []struct {
		name                 string
		config               Config
		definition           *skupperv2alpha1.SecuredAccess
		k8sObjects           []runtime.Object
		ssaRecorder          *ServerSideApplyRecorder
		skupperObjects       []runtime.Object
		expectedServices     []*corev1.Service
		expectedRoutes       []*routev1.Route
		expectedIngresses    []*networkingv1.Ingress
		expectedProxies      []HttpProxy
		expectedSSA          map[string]*unstructured.Unstructured
		expectedCertificates []MockCertificate
		expectedStatus       string
		expectedEndpoints    []skupperv2alpha1.Endpoint
		expectedError        string
		defaultDomain        string
		nodePorts            map[string]int32
		labels               map[string]string
		annotations          map[string]string
	}{
		{
			name: "loadbalancer",
			config: Config{
				EnabledAccessTypes: []string{
					ACCESS_TYPE_LOADBALANCER,
					ACCESS_TYPE_ROUTE,
					ACCESS_TYPE_LOCAL,
				},
			},
			labels: map[string]string{
				"foo": "bar",
			},
			annotations: map[string]string{
				"abc": "123",
			},
			definition: &skupperv2alpha1.SecuredAccess{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "skupper.io/v2alpha1",
					Kind:       "SecuredAccess",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "mysvc",
					Namespace: "test",
				},
				Spec: skupperv2alpha1.SecuredAccessSpec{
					AccessType: ACCESS_TYPE_LOADBALANCER,
					Selector: map[string]string{
						"app": "foo",
					},
					Ports: []skupperv2alpha1.SecuredAccessPort{
						{
							Name:       "port1",
							Port:       8080,
							TargetPort: 8081,
							Protocol:   "TCP",
						},
					},
					Certificate: "my-cert",
					Issuer:      "skupper-site-ca",
				},
			},
			expectedServices: []*corev1.Service{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "mysvc",
						Namespace: "test",
						Labels: map[string]string{
							"foo": "bar",
						},
						Annotations: map[string]string{
							"abc": "123",
						},
					},
					Spec: corev1.ServiceSpec{
						Selector: map[string]string{
							"app": "foo",
						},
						Type: corev1.ServiceTypeLoadBalancer,
						Ports: []corev1.ServicePort{
							{
								Name:       "port1",
								Port:       8080,
								TargetPort: intstr.IntOrString{IntVal: int32(8081)},
								Protocol:   corev1.Protocol("TCP"),
							},
						},
					},
					Status: corev1.ServiceStatus{
						LoadBalancer: corev1.LoadBalancerStatus{
							Ingress: []corev1.LoadBalancerIngress{
								{
									Hostname: "my-ingress.my-cluster.org",
								},
							},
						},
					},
				},
			},
			expectedCertificates: []MockCertificate{
				{
					namespace: "test",
					name:      "my-cert",
					ca:        "skupper-site-ca",
					subject:   "mysvc",
					hosts:     []string{"mysvc", "mysvc.test", "my-ingress.my-cluster.org"},
					client:    false,
					server:    true,
					refs:      nil,
				},
			},
			expectedStatus: "OK",
			expectedEndpoints: []skupperv2alpha1.Endpoint{
				{
					Name: "port1",
					Port: "8080",
					Host: "my-ingress.my-cluster.org",
				},
			},
		},
		{
			name: "loadbalancer with IP",
			config: Config{
				EnabledAccessTypes: []string{
					ACCESS_TYPE_LOADBALANCER,
					ACCESS_TYPE_ROUTE,
					ACCESS_TYPE_LOCAL,
				},
			},
			definition: &skupperv2alpha1.SecuredAccess{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "skupper.io/v2alpha1",
					Kind:       "SecuredAccess",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "mysvc",
					Namespace: "test",
				},
				Spec: skupperv2alpha1.SecuredAccessSpec{
					AccessType: ACCESS_TYPE_LOADBALANCER,
					Selector: map[string]string{
						"app": "foo",
					},
					Ports: []skupperv2alpha1.SecuredAccessPort{
						{
							Name:       "port1",
							Port:       8080,
							TargetPort: 8081,
							Protocol:   "TCP",
						},
					},
					Issuer: "skupper-site-ca",
				},
			},
			expectedServices: []*corev1.Service{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "mysvc",
						Namespace: "test",
					},
					Spec: corev1.ServiceSpec{
						Selector: map[string]string{
							"app": "foo",
						},
						Type: corev1.ServiceTypeLoadBalancer,
						Ports: []corev1.ServicePort{
							{
								Name:       "port1",
								Port:       8080,
								TargetPort: intstr.IntOrString{IntVal: int32(8081)},
								Protocol:   corev1.Protocol("TCP"),
							},
						},
					},
					Status: corev1.ServiceStatus{
						LoadBalancer: corev1.LoadBalancerStatus{
							Ingress: []corev1.LoadBalancerIngress{
								{
									IP: "100.10.10.100",
								},
							},
						},
					},
				},
			},
			expectedCertificates: []MockCertificate{
				{
					namespace: "test",
					name:      "mysvc",
					ca:        "skupper-site-ca",
					subject:   "mysvc",
					hosts:     []string{"mysvc", "mysvc.test", "100.10.10.100"},
					client:    false,
					server:    true,
					refs:      nil,
				},
			},
			expectedStatus: "OK",
			expectedEndpoints: []skupperv2alpha1.Endpoint{
				{
					Name: "port1",
					Port: "8080",
					Host: "100.10.10.100",
				},
			},
		},
		{
			name: "unresolved loadbalancer",
			config: Config{
				EnabledAccessTypes: []string{
					ACCESS_TYPE_LOADBALANCER,
					ACCESS_TYPE_ROUTE,
					ACCESS_TYPE_LOCAL,
				},
			},
			definition: &skupperv2alpha1.SecuredAccess{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "skupper.io/v2alpha1",
					Kind:       "SecuredAccess",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "mysvc",
					Namespace: "test",
				},
				Spec: skupperv2alpha1.SecuredAccessSpec{
					AccessType: ACCESS_TYPE_LOADBALANCER,
					Selector: map[string]string{
						"app": "foo",
					},
					Ports: []skupperv2alpha1.SecuredAccessPort{
						{
							Name:       "port1",
							Port:       8080,
							TargetPort: 8081,
							Protocol:   "TCP",
						},
					},
					Certificate: "my-cert",
					Issuer:      "skupper-site-ca",
				},
			},
			expectedServices: []*corev1.Service{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "mysvc",
						Namespace: "test",
					},
					Spec: corev1.ServiceSpec{
						Selector: map[string]string{
							"app": "foo",
						},
						Type: corev1.ServiceTypeLoadBalancer,
						Ports: []corev1.ServicePort{
							{
								Name:       "port1",
								Port:       8080,
								TargetPort: intstr.IntOrString{IntVal: int32(8081)},
								Protocol:   corev1.Protocol("TCP"),
							},
						},
					},
					Status: corev1.ServiceStatus{
						LoadBalancer: corev1.LoadBalancerStatus{
							Ingress: []corev1.LoadBalancerIngress{
								{},
							},
						},
					},
				},
			},
			expectedCertificates: []MockCertificate{
				{
					namespace: "test",
					name:      "my-cert",
					ca:        "skupper-site-ca",
					subject:   "mysvc",
					hosts:     []string{"mysvc", "mysvc.test"},
					client:    false,
					server:    true,
					refs:      nil,
				},
			},
			expectedStatus: "Pending",
		},
		{
			name: "route",
			labels: map[string]string{
				"foo": "bar",
			},
			annotations: map[string]string{
				"abc": "123",
			},
			config: Config{
				EnabledAccessTypes: []string{
					ACCESS_TYPE_LOADBALANCER,
					ACCESS_TYPE_ROUTE,
					ACCESS_TYPE_LOCAL,
				},
			},
			definition: &skupperv2alpha1.SecuredAccess{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "skupper.io/v2alpha1",
					Kind:       "SecuredAccess",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "mysvc",
					Namespace: "test",
				},
				Spec: skupperv2alpha1.SecuredAccessSpec{
					AccessType: ACCESS_TYPE_ROUTE,
					Selector: map[string]string{
						"app": "foo",
					},
					Ports: []skupperv2alpha1.SecuredAccessPort{
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
					},
					Certificate: "my-cert",
					Issuer:      "skupper-site-ca",
				},
			},
			expectedServices: []*corev1.Service{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "mysvc",
						Namespace: "test",
					},
					Spec: corev1.ServiceSpec{
						Selector: map[string]string{
							"app": "foo",
						},
						Ports: []corev1.ServicePort{
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
						},
					},
				},
			},
			expectedRoutes: []*routev1.Route{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "mysvc-a",
						Namespace: "test",
						Labels: map[string]string{
							"foo": "bar",
						},
						Annotations: map[string]string{
							"abc": "123",
						},
					},
					Spec: routev1.RouteSpec{
						Host: "",
						Port: &routev1.RoutePort{
							TargetPort: intstr.FromString("a"),
						},
						To: routev1.RouteTargetReference{
							Kind: "Service",
							Name: "mysvc",
						},
						TLS: &routev1.TLSConfig{
							Termination:                   routev1.TLSTerminationPassthrough,
							InsecureEdgeTerminationPolicy: routev1.InsecureEdgeTerminationPolicyNone,
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "mysvc-b",
						Namespace: "test",
						Labels: map[string]string{
							"foo": "bar",
						},
						Annotations: map[string]string{
							"abc": "123",
						},
					},
					Spec: routev1.RouteSpec{
						Host: "",
						Port: &routev1.RoutePort{
							TargetPort: intstr.FromString("b"),
						},
						To: routev1.RouteTargetReference{
							Kind: "Service",
							Name: "mysvc",
						},
						TLS: &routev1.TLSConfig{
							Termination:                   routev1.TLSTerminationPassthrough,
							InsecureEdgeTerminationPolicy: routev1.InsecureEdgeTerminationPolicyNone,
						},
					},
				},
			},
			expectedCertificates: []MockCertificate{
				{
					namespace: "test",
					name:      "my-cert",
					ca:        "skupper-site-ca",
					subject:   "mysvc",
					hosts:     []string{"mysvc", "mysvc.test", "mysvc-a.test.mycluster.org", "mysvc-b.test.mycluster.org"},
					client:    false,
					server:    true,
					refs:      nil,
				},
			},
			expectedStatus: "OK",
			expectedEndpoints: []skupperv2alpha1.Endpoint{
				{
					Name: "a",
					Port: "443",
					Host: "mysvc-a.test.mycluster.org",
				},
				{
					Name: "b",
					Port: "443",
					Host: "mysvc-b.test.mycluster.org",
				},
			},
			defaultDomain: "mycluster.org",
		},
		{
			name: "route with user supplied domain",
			config: Config{
				EnabledAccessTypes: []string{
					ACCESS_TYPE_LOADBALANCER,
					ACCESS_TYPE_ROUTE,
					ACCESS_TYPE_LOCAL,
				},
			},
			definition: &skupperv2alpha1.SecuredAccess{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "skupper.io/v2alpha1",
					Kind:       "SecuredAccess",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "mysvc",
					Namespace: "test",
				},
				Spec: skupperv2alpha1.SecuredAccessSpec{
					AccessType: ACCESS_TYPE_ROUTE,
					Settings: map[string]string{
						"domain": "users.domain.org",
					},
					Selector: map[string]string{
						"app": "foo",
					},
					Ports: []skupperv2alpha1.SecuredAccessPort{
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
					},
					Certificate: "my-cert",
					Issuer:      "skupper-site-ca",
				},
			},
			expectedServices: []*corev1.Service{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "mysvc",
						Namespace: "test",
					},
					Spec: corev1.ServiceSpec{
						Selector: map[string]string{
							"app": "foo",
						},
						Ports: []corev1.ServicePort{
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
						},
					},
				},
			},
			expectedRoutes: []*routev1.Route{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "mysvc-a",
						Namespace: "test",
					},
					Spec: routev1.RouteSpec{
						Host: "mysvc-a.test.users.domain.org",
						Port: &routev1.RoutePort{
							TargetPort: intstr.FromString("a"),
						},
						To: routev1.RouteTargetReference{
							Kind: "Service",
							Name: "mysvc",
						},
						TLS: &routev1.TLSConfig{
							Termination:                   routev1.TLSTerminationPassthrough,
							InsecureEdgeTerminationPolicy: routev1.InsecureEdgeTerminationPolicyNone,
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "mysvc-b",
						Namespace: "test",
					},
					Spec: routev1.RouteSpec{
						Host: "mysvc-b.test.users.domain.org",
						Port: &routev1.RoutePort{
							TargetPort: intstr.FromString("b"),
						},
						To: routev1.RouteTargetReference{
							Kind: "Service",
							Name: "mysvc",
						},
						TLS: &routev1.TLSConfig{
							Termination:                   routev1.TLSTerminationPassthrough,
							InsecureEdgeTerminationPolicy: routev1.InsecureEdgeTerminationPolicyNone,
						},
					},
				},
			},
			expectedCertificates: []MockCertificate{
				{
					namespace: "test",
					name:      "my-cert",
					ca:        "skupper-site-ca",
					subject:   "mysvc",
					hosts:     []string{"mysvc-a.test.users.domain.org", "mysvc-b.test.users.domain.org", "mysvc", "mysvc.test"},
					client:    false,
					server:    true,
					refs:      nil,
				},
			},
			expectedStatus: "OK",
			expectedEndpoints: []skupperv2alpha1.Endpoint{
				{
					Name: "a",
					Port: "443",
					Host: "mysvc-a.test.users.domain.org",
				},
				{
					Name: "b",
					Port: "443",
					Host: "mysvc-b.test.users.domain.org",
				},
			},
		},
		{
			name: "ingress-nginx",
			config: Config{
				EnabledAccessTypes: []string{
					ACCESS_TYPE_LOADBALANCER,
					ACCESS_TYPE_INGRESS_NGINX,
					ACCESS_TYPE_ROUTE,
					ACCESS_TYPE_LOCAL,
				},
			},
			labels: map[string]string{
				"foo": "bar",
			},
			annotations: map[string]string{
				"abc": "123",
			},
			definition: &skupperv2alpha1.SecuredAccess{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "skupper.io/v2alpha1",
					Kind:       "SecuredAccess",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "mysvc",
					Namespace: "test",
				},
				Spec: skupperv2alpha1.SecuredAccessSpec{
					AccessType: ACCESS_TYPE_INGRESS_NGINX,
					Selector: map[string]string{
						"app": "foo",
					},
					Ports: []skupperv2alpha1.SecuredAccessPort{
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
					},
					Certificate: "my-cert",
					Issuer:      "skupper-site-ca",
				},
			},
			expectedServices: []*corev1.Service{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "mysvc",
						Namespace: "test",
					},
					Spec: corev1.ServiceSpec{
						Selector: map[string]string{
							"app": "foo",
						},
						Ports: []corev1.ServicePort{
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
						},
					},
				},
			},
			expectedIngresses: []*networkingv1.Ingress{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "mysvc",
						Namespace: "test",
						Labels: map[string]string{
							"foo": "bar",
						},
						Annotations: map[string]string{
							"abc": "123",
						},
					},
					Spec: networkingv1.IngressSpec{
						Rules: []networkingv1.IngressRule{
							{
								Host: "a.test",
								IngressRuleValue: networkingv1.IngressRuleValue{
									HTTP: &networkingv1.HTTPIngressRuleValue{
										Paths: []networkingv1.HTTPIngressPath{
											{
												Path:     "/",
												PathType: &pathTypePrefix,
												Backend: networkingv1.IngressBackend{
													Service: &networkingv1.IngressServiceBackend{
														Name: "mysvc",
														Port: networkingv1.ServiceBackendPort{
															Number: int32(8080),
														},
													},
												},
											},
										},
									},
								},
							},
							{
								Host: "b.test",
								IngressRuleValue: networkingv1.IngressRuleValue{
									HTTP: &networkingv1.HTTPIngressRuleValue{
										Paths: []networkingv1.HTTPIngressPath{
											{
												Path:     "/",
												PathType: &pathTypePrefix,
												Backend: networkingv1.IngressBackend{
													Service: &networkingv1.IngressServiceBackend{
														Name: "mysvc",
														Port: networkingv1.ServiceBackendPort{
															Number: int32(9090),
														},
													},
												},
											},
										},
									},
								},
							},
						},
					},
					Status: networkingv1.IngressStatus{
						LoadBalancer: networkingv1.IngressLoadBalancerStatus{
							Ingress: []networkingv1.IngressLoadBalancerIngress{
								{
									Hostname: "my-ingress-gateway.net",
								},
							},
						},
					},
				},
			},
			expectedCertificates: []MockCertificate{
				{
					namespace: "test",
					name:      "my-cert",
					ca:        "skupper-site-ca",
					subject:   "mysvc",
					hosts:     []string{"mysvc", "mysvc.test", "a.test.my-ingress-gateway.net", "b.test.my-ingress-gateway.net"},
					client:    false,
					server:    true,
					refs:      nil,
				},
			},
			expectedStatus: "OK",
			expectedEndpoints: []skupperv2alpha1.Endpoint{
				{
					Name: "a",
					Port: "443",
					Host: "a.test.my-ingress-gateway.net",
				},
				{
					Name: "b",
					Port: "443",
					Host: "b.test.my-ingress-gateway.net",
				},
			},
		},
		{
			name: "ingress-nginx with IP",
			config: Config{
				EnabledAccessTypes: []string{
					ACCESS_TYPE_LOADBALANCER,
					ACCESS_TYPE_INGRESS_NGINX,
					ACCESS_TYPE_ROUTE,
					ACCESS_TYPE_LOCAL,
				},
			},
			definition: &skupperv2alpha1.SecuredAccess{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "skupper.io/v2alpha1",
					Kind:       "SecuredAccess",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "mysvc",
					Namespace: "test",
				},
				Spec: skupperv2alpha1.SecuredAccessSpec{
					AccessType: ACCESS_TYPE_INGRESS_NGINX,
					Selector: map[string]string{
						"app": "foo",
					},
					Ports: []skupperv2alpha1.SecuredAccessPort{
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
					},
					Certificate: "my-cert",
					Issuer:      "skupper-site-ca",
				},
			},
			expectedServices: []*corev1.Service{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "mysvc",
						Namespace: "test",
					},
					Spec: corev1.ServiceSpec{
						Selector: map[string]string{
							"app": "foo",
						},
						Ports: []corev1.ServicePort{
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
						},
					},
				},
			},
			expectedIngresses: []*networkingv1.Ingress{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "mysvc",
						Namespace: "test",
					},
					Spec: networkingv1.IngressSpec{
						Rules: []networkingv1.IngressRule{
							{
								Host: "a.test",
								IngressRuleValue: networkingv1.IngressRuleValue{
									HTTP: &networkingv1.HTTPIngressRuleValue{
										Paths: []networkingv1.HTTPIngressPath{
											{
												Path:     "/",
												PathType: &pathTypePrefix,
												Backend: networkingv1.IngressBackend{
													Service: &networkingv1.IngressServiceBackend{
														Name: "mysvc",
														Port: networkingv1.ServiceBackendPort{
															Number: int32(8080),
														},
													},
												},
											},
										},
									},
								},
							},
							{
								Host: "b.test",
								IngressRuleValue: networkingv1.IngressRuleValue{
									HTTP: &networkingv1.HTTPIngressRuleValue{
										Paths: []networkingv1.HTTPIngressPath{
											{
												Path:     "/",
												PathType: &pathTypePrefix,
												Backend: networkingv1.IngressBackend{
													Service: &networkingv1.IngressServiceBackend{
														Name: "mysvc",
														Port: networkingv1.ServiceBackendPort{
															Number: int32(9090),
														},
													},
												},
											},
										},
									},
								},
							},
						},
					},
					Status: networkingv1.IngressStatus{
						LoadBalancer: networkingv1.IngressLoadBalancerStatus{
							Ingress: []networkingv1.IngressLoadBalancerIngress{
								{
									IP: "100.5.10.5",
								},
							},
						},
					},
				},
			},
			expectedCertificates: []MockCertificate{
				{
					namespace: "test",
					name:      "my-cert",
					ca:        "skupper-site-ca",
					subject:   "mysvc",
					hosts:     []string{"mysvc", "mysvc.test", "a.test.100.5.10.5.nip.io", "b.test.100.5.10.5.nip.io"},
					client:    false,
					server:    true,
					refs:      nil,
				},
			},
			expectedStatus: "OK",
			expectedEndpoints: []skupperv2alpha1.Endpoint{
				{
					Name: "a",
					Port: "443",
					Host: "a.test.100.5.10.5.nip.io",
				},
				{
					Name: "b",
					Port: "443",
					Host: "b.test.100.5.10.5.nip.io",
				},
			},
		},
		{
			name: "unresolved ingress-nginx",
			config: Config{
				EnabledAccessTypes: []string{
					ACCESS_TYPE_LOADBALANCER,
					ACCESS_TYPE_INGRESS_NGINX,
					ACCESS_TYPE_ROUTE,
					ACCESS_TYPE_LOCAL,
				},
			},
			definition: &skupperv2alpha1.SecuredAccess{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "skupper.io/v2alpha1",
					Kind:       "SecuredAccess",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "mysvc",
					Namespace: "test",
				},
				Spec: skupperv2alpha1.SecuredAccessSpec{
					AccessType: ACCESS_TYPE_INGRESS_NGINX,
					Selector: map[string]string{
						"app": "foo",
					},
					Ports: []skupperv2alpha1.SecuredAccessPort{
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
					},
					Certificate: "my-cert",
					Issuer:      "skupper-site-ca",
				},
			},
			expectedServices: []*corev1.Service{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "mysvc",
						Namespace: "test",
					},
					Spec: corev1.ServiceSpec{
						Selector: map[string]string{
							"app": "foo",
						},
						Ports: []corev1.ServicePort{
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
						},
					},
				},
			},
			expectedIngresses: []*networkingv1.Ingress{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "mysvc",
						Namespace: "test",
					},
					Spec: networkingv1.IngressSpec{
						Rules: []networkingv1.IngressRule{
							{
								Host: "a.test",
								IngressRuleValue: networkingv1.IngressRuleValue{
									HTTP: &networkingv1.HTTPIngressRuleValue{
										Paths: []networkingv1.HTTPIngressPath{
											{
												Path:     "/",
												PathType: &pathTypePrefix,
												Backend: networkingv1.IngressBackend{
													Service: &networkingv1.IngressServiceBackend{
														Name: "mysvc",
														Port: networkingv1.ServiceBackendPort{
															Number: int32(8080),
														},
													},
												},
											},
										},
									},
								},
							},
							{
								Host: "b.test",
								IngressRuleValue: networkingv1.IngressRuleValue{
									HTTP: &networkingv1.HTTPIngressRuleValue{
										Paths: []networkingv1.HTTPIngressPath{
											{
												Path:     "/",
												PathType: &pathTypePrefix,
												Backend: networkingv1.IngressBackend{
													Service: &networkingv1.IngressServiceBackend{
														Name: "mysvc",
														Port: networkingv1.ServiceBackendPort{
															Number: int32(9090),
														},
													},
												},
											},
										},
									},
								},
							},
						},
					},
					Status: networkingv1.IngressStatus{
						LoadBalancer: networkingv1.IngressLoadBalancerStatus{
							Ingress: []networkingv1.IngressLoadBalancerIngress{
								{},
							},
						},
					},
				},
			},
			expectedCertificates: []MockCertificate{
				{
					namespace: "test",
					name:      "my-cert",
					ca:        "skupper-site-ca",
					subject:   "mysvc",
					hosts:     []string{"mysvc", "mysvc.test"},
					client:    false,
					server:    true,
					refs:      nil,
				},
			},
			expectedStatus: "Pending",
		},
		{
			name: "nodeport",
			config: Config{
				ClusterHost: "mycluster.com",
				EnabledAccessTypes: []string{
					ACCESS_TYPE_LOADBALANCER,
					ACCESS_TYPE_NODEPORT,
					ACCESS_TYPE_ROUTE,
					ACCESS_TYPE_LOCAL,
				},
			},
			definition: &skupperv2alpha1.SecuredAccess{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "skupper.io/v2alpha1",
					Kind:       "SecuredAccess",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "mysvc",
					Namespace: "test",
				},
				Spec: skupperv2alpha1.SecuredAccessSpec{
					AccessType: ACCESS_TYPE_NODEPORT,
					Selector: map[string]string{
						"app": "foo",
					},
					Ports: []skupperv2alpha1.SecuredAccessPort{
						{
							Name:       "port1",
							Port:       8080,
							TargetPort: 8081,
							Protocol:   "TCP",
						},
					},
					Certificate: "my-cert",
					Issuer:      "skupper-site-ca",
				},
			},
			expectedServices: []*corev1.Service{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "mysvc",
						Namespace: "test",
					},
					Spec: corev1.ServiceSpec{
						Selector: map[string]string{
							"app": "foo",
						},
						Type: corev1.ServiceTypeNodePort,
						Ports: []corev1.ServicePort{
							{
								Name:       "port1",
								Port:       8080,
								TargetPort: intstr.IntOrString{IntVal: int32(8081)},
								Protocol:   corev1.Protocol("TCP"),
							},
						},
					},
				},
			},
			expectedCertificates: []MockCertificate{
				{
					namespace: "test",
					name:      "my-cert",
					ca:        "skupper-site-ca",
					subject:   "mysvc",
					hosts:     []string{"mycluster.com", "mysvc", "mysvc.test"},
					client:    false,
					server:    true,
					refs:      nil,
				},
			},
			expectedStatus: "OK",
			expectedEndpoints: []skupperv2alpha1.Endpoint{
				{
					Name: "port1",
					Host: "mycluster.com",
					Port: "33333",
				},
			},
			nodePorts: map[string]int32{
				"port1": 33333,
			},
		},
		{
			name: "local",
			config: Config{
				ClusterHost: "mycluster.com",
				EnabledAccessTypes: []string{
					ACCESS_TYPE_LOADBALANCER,
					ACCESS_TYPE_NODEPORT,
					ACCESS_TYPE_ROUTE,
					ACCESS_TYPE_LOCAL,
				},
			},
			definition: &skupperv2alpha1.SecuredAccess{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "skupper.io/v2alpha1",
					Kind:       "SecuredAccess",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "mysvc",
					Namespace: "test",
				},
				Spec: skupperv2alpha1.SecuredAccessSpec{
					AccessType: ACCESS_TYPE_LOCAL,
					Selector: map[string]string{
						"app": "foo",
					},
					Ports: []skupperv2alpha1.SecuredAccessPort{
						{
							Name:       "port1",
							Port:       8080,
							TargetPort: 8081,
							Protocol:   "TCP",
						},
					},
					Certificate: "my-cert",
					Issuer:      "skupper-site-ca",
				},
			},
			expectedServices: []*corev1.Service{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "mysvc",
						Namespace: "test",
					},
					Spec: corev1.ServiceSpec{
						Selector: map[string]string{
							"app": "foo",
						},
						Ports: []corev1.ServicePort{
							{
								Name:       "port1",
								Port:       8080,
								TargetPort: intstr.IntOrString{IntVal: int32(8081)},
								Protocol:   corev1.Protocol("TCP"),
							},
						},
					},
				},
			},
			expectedEndpoints: []skupperv2alpha1.Endpoint{
				{
					Name: "port1",
					Port: "8080",
					Host: "mysvc.test",
				},
			},
			expectedCertificates: []MockCertificate{
				{
					namespace: "test",
					name:      "my-cert",
					ca:        "skupper-site-ca",
					subject:   "mysvc",
					hosts:     []string{"mysvc", "mysvc.test"},
					client:    false,
					server:    true,
					refs:      nil,
				},
			},
			expectedStatus: "OK",
		},
		{
			name: "unsupported",
			config: Config{
				EnabledAccessTypes: []string{
					ACCESS_TYPE_LOADBALANCER,
					ACCESS_TYPE_ROUTE,
					ACCESS_TYPE_LOCAL,
				},
			},
			definition: &skupperv2alpha1.SecuredAccess{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "skupper.io/v2alpha1",
					Kind:       "SecuredAccess",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "mysvc",
					Namespace: "test",
				},
				Spec: skupperv2alpha1.SecuredAccessSpec{
					AccessType: "surpriseme!",
					Selector: map[string]string{
						"app": "foo",
					},
					Ports: []skupperv2alpha1.SecuredAccessPort{
						{
							Name:       "port1",
							Port:       8080,
							TargetPort: 8081,
							Protocol:   "TCP",
						},
					},
					Certificate: "my-cert",
					Issuer:      "skupper-site-ca",
				},
			},
			expectedStatus: "unsupported access type",
		},
		{
			name: "loadbalancer by default",
			config: Config{
				EnabledAccessTypes: []string{
					ACCESS_TYPE_LOADBALANCER,
					ACCESS_TYPE_ROUTE,
					ACCESS_TYPE_LOCAL,
				},
				DefaultAccessType: ACCESS_TYPE_LOADBALANCER,
			},
			definition: &skupperv2alpha1.SecuredAccess{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "skupper.io/v2alpha1",
					Kind:       "SecuredAccess",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "mysvc",
					Namespace: "test",
				},
				Spec: skupperv2alpha1.SecuredAccessSpec{
					Selector: map[string]string{
						"app": "foo",
					},
					Ports: []skupperv2alpha1.SecuredAccessPort{
						{
							Name:       "port1",
							Port:       8080,
							TargetPort: 8081,
							Protocol:   "TCP",
						},
					},
					Certificate: "my-cert",
					Issuer:      "skupper-site-ca",
				},
			},
			expectedServices: []*corev1.Service{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "mysvc",
						Namespace: "test",
					},
					Spec: corev1.ServiceSpec{
						Selector: map[string]string{
							"app": "foo",
						},
						Type: corev1.ServiceTypeLoadBalancer,
						Ports: []corev1.ServicePort{
							{
								Name:       "port1",
								Port:       8080,
								TargetPort: intstr.IntOrString{IntVal: int32(8081)},
								Protocol:   corev1.Protocol("TCP"),
							},
						},
					},
					Status: corev1.ServiceStatus{
						LoadBalancer: corev1.LoadBalancerStatus{
							Ingress: []corev1.LoadBalancerIngress{
								{
									Hostname: "my-ingress.my-cluster.org",
								},
							},
						},
					},
				},
			},
			expectedCertificates: []MockCertificate{
				{
					namespace: "test",
					name:      "my-cert",
					ca:        "skupper-site-ca",
					subject:   "mysvc",
					hosts:     []string{"mysvc", "mysvc.test", "my-ingress.my-cluster.org"},
					client:    false,
					server:    true,
					refs:      nil,
				},
			},
			expectedStatus: "OK",
			expectedEndpoints: []skupperv2alpha1.Endpoint{
				{
					Name: "port1",
					Port: "8080",
					Host: "my-ingress.my-cluster.org",
				},
			},
		},
		{
			name: "contour http-proxy",
			config: Config{
				EnabledAccessTypes: []string{
					ACCESS_TYPE_LOADBALANCER,
					ACCESS_TYPE_CONTOUR_HTTP_PROXY,
					ACCESS_TYPE_LOCAL,
				},
				HttpProxyDomain: "gateway.acme.com",
			},
			labels: map[string]string{
				"foo": "bar",
			},
			annotations: map[string]string{
				"abc": "123",
			},
			definition: &skupperv2alpha1.SecuredAccess{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "skupper.io/v2alpha1",
					Kind:       "SecuredAccess",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "mysvc",
					Namespace: "test",
				},
				Spec: skupperv2alpha1.SecuredAccessSpec{
					AccessType: ACCESS_TYPE_CONTOUR_HTTP_PROXY,
					Selector: map[string]string{
						"app": "foo",
					},
					Ports: []skupperv2alpha1.SecuredAccessPort{
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
					},
					Certificate: "my-cert",
					Issuer:      "skupper-site-ca",
				},
			},
			expectedServices: []*corev1.Service{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "mysvc",
						Namespace: "test",
					},
					Spec: corev1.ServiceSpec{
						Selector: map[string]string{
							"app": "foo",
						},
						Ports: []corev1.ServicePort{
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
						},
					},
				},
			},
			expectedProxies: []HttpProxy{
				{
					Name:        "mysvc-a",
					Host:        "mysvc-a.test.gateway.acme.com",
					ServiceName: "mysvc",
					ServicePort: 8080,
				},
				{
					Name:        "mysvc-b",
					Host:        "mysvc-b.test.gateway.acme.com",
					ServiceName: "mysvc",
					ServicePort: 9090,
				},
			},
			expectedCertificates: []MockCertificate{
				{
					namespace: "test",
					name:      "my-cert",
					ca:        "skupper-site-ca",
					subject:   "mysvc",
					hosts:     []string{"mysvc", "mysvc.test", "mysvc-a.test.gateway.acme.com", "mysvc-b.test.gateway.acme.com"},
					client:    false,
					server:    true,
					refs:      nil,
				},
			},
			expectedStatus: "OK",
			expectedEndpoints: []skupperv2alpha1.Endpoint{
				{
					Name: "a",
					Port: "443",
					Host: "mysvc-a.test.gateway.acme.com",
				},
				{
					Name: "b",
					Port: "443",
					Host: "mysvc-b.test.gateway.acme.com",
				},
			},
		},
		{
			name: "gateway",
			config: Config{
				EnabledAccessTypes: []string{
					ACCESS_TYPE_GATEWAY,
				},
				GatewayClass:  "xyz",
				GatewayDomain: "mygateway.net",
				GatewayPort:   8443,
			},
			labels: map[string]string{
				"foo": "bar",
			},
			annotations: map[string]string{
				"abc": "123",
			},
			ssaRecorder: newServerSideApplyRecorder(),
			expectedSSA: map[string]*unstructured.Unstructured{
				"test/skupper": gateway("skupper", "test"),
				"test/mysvc-a": tlsroute("mysvc-a", "test"),
				"test/mysvc-b": tlsroute("mysvc-b", "test"),
			},
			definition: &skupperv2alpha1.SecuredAccess{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "skupper.io/v2alpha1",
					Kind:       "SecuredAccess",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "mysvc",
					Namespace: "test",
				},
				Spec: skupperv2alpha1.SecuredAccessSpec{
					AccessType: ACCESS_TYPE_GATEWAY,
					Selector: map[string]string{
						"app": "foo",
					},
					Ports: []skupperv2alpha1.SecuredAccessPort{
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
					},
					Certificate: "my-cert",
					Issuer:      "skupper-site-ca",
				},
			},
			expectedServices: []*corev1.Service{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "mysvc",
						Namespace: "test",
					},
					Spec: corev1.ServiceSpec{
						Selector: map[string]string{
							"app": "foo",
						},
						Ports: []corev1.ServicePort{
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
						},
					},
				},
			},
			expectedCertificates: []MockCertificate{
				{
					namespace: "test",
					name:      "my-cert",
					ca:        "skupper-site-ca",
					subject:   "mysvc",
					hosts:     []string{"mysvc", "mysvc.test", "mysvc-a.test.mygateway.net", "mysvc-b.test.mygateway.net"},
					client:    false,
					server:    true,
					refs:      nil,
				},
			},
			expectedStatus: "OK",
			expectedEndpoints: []skupperv2alpha1.Endpoint{
				{
					Name: "a",
					Port: "8443",
					Host: "mysvc-a.test.mygateway.net",
				},
				{
					Name: "b",
					Port: "8443",
					Host: "mysvc-b.test.mygateway.net",
				},
			},
		},
		{
			name: "gateway with auto resolved hostname",
			config: Config{
				EnabledAccessTypes: []string{
					ACCESS_TYPE_GATEWAY,
				},
				GatewayClass: "xyz",
				GatewayPort:  7443,
			},
			ssaRecorder: newServerSideApplyRecorder().setGatewayHostname("test/skupper", "autogateway.foo"),
			expectedSSA: map[string]*unstructured.Unstructured{
				"test/skupper": gateway("skupper", "test"),
				"test/mysvc-a": tlsroute("mysvc-a", "test"),
				"test/mysvc-b": tlsroute("mysvc-b", "test"),
			},
			definition: &skupperv2alpha1.SecuredAccess{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "skupper.io/v2alpha1",
					Kind:       "SecuredAccess",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "mysvc",
					Namespace: "test",
				},
				Spec: skupperv2alpha1.SecuredAccessSpec{
					AccessType: ACCESS_TYPE_GATEWAY,
					Selector: map[string]string{
						"app": "foo",
					},
					Ports: []skupperv2alpha1.SecuredAccessPort{
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
					},
					Certificate: "my-cert",
					Issuer:      "skupper-site-ca",
				},
			},
			expectedServices: []*corev1.Service{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "mysvc",
						Namespace: "test",
					},
					Spec: corev1.ServiceSpec{
						Selector: map[string]string{
							"app": "foo",
						},
						Ports: []corev1.ServicePort{
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
						},
					},
				},
			},
			expectedCertificates: []MockCertificate{
				{
					namespace: "test",
					name:      "my-cert",
					ca:        "skupper-site-ca",
					subject:   "mysvc",
					hosts:     []string{"mysvc", "mysvc.test", "mysvc-a.test.autogateway.foo", "mysvc-b.test.autogateway.foo"},
					client:    false,
					server:    true,
					refs:      nil,
				},
			},
			expectedStatus: "OK",
			expectedEndpoints: []skupperv2alpha1.Endpoint{
				{
					Name: "a",
					Port: "7443",
					Host: "mysvc-a.test.autogateway.foo",
				},
				{
					Name: "b",
					Port: "7443",
					Host: "mysvc-b.test.autogateway.foo",
				},
			},
		},
		{
			name: "gateway with auto resolved ip",
			config: Config{
				EnabledAccessTypes: []string{
					ACCESS_TYPE_GATEWAY,
				},
				GatewayClass: "xyz",
				GatewayPort:  7443,
			},
			ssaRecorder: newServerSideApplyRecorder().setGatewayIP("test/skupper", "10.1.1.10"),
			expectedSSA: map[string]*unstructured.Unstructured{
				"test/skupper": gateway("skupper", "test"),
				"test/mysvc-a": tlsroute("mysvc-a", "test"),
				"test/mysvc-b": tlsroute("mysvc-b", "test"),
			},
			definition: &skupperv2alpha1.SecuredAccess{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "skupper.io/v2alpha1",
					Kind:       "SecuredAccess",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "mysvc",
					Namespace: "test",
				},
				Spec: skupperv2alpha1.SecuredAccessSpec{
					AccessType: ACCESS_TYPE_GATEWAY,
					Selector: map[string]string{
						"app": "foo",
					},
					Ports: []skupperv2alpha1.SecuredAccessPort{
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
					},
					Certificate: "my-cert",
					Issuer:      "skupper-site-ca",
				},
			},
			expectedServices: []*corev1.Service{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "mysvc",
						Namespace: "test",
					},
					Spec: corev1.ServiceSpec{
						Selector: map[string]string{
							"app": "foo",
						},
						Ports: []corev1.ServicePort{
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
						},
					},
				},
			},
			expectedCertificates: []MockCertificate{
				{
					namespace: "test",
					name:      "my-cert",
					ca:        "skupper-site-ca",
					subject:   "mysvc",
					hosts:     []string{"mysvc", "mysvc.test", "mysvc-a.test.10.1.1.10.nip.io", "mysvc-b.test.10.1.1.10.nip.io"},
					client:    false,
					server:    true,
					refs:      nil,
				},
			},
			expectedStatus: "OK",
			expectedEndpoints: []skupperv2alpha1.Endpoint{
				{
					Name: "a",
					Port: "7443",
					Host: "mysvc-a.test.10.1.1.10.nip.io",
				},
				{
					Name: "b",
					Port: "7443",
					Host: "mysvc-b.test.10.1.1.10.nip.io",
				},
			},
		},
		{
			name: "unresolved gateway",
			config: Config{
				EnabledAccessTypes: []string{
					ACCESS_TYPE_GATEWAY,
				},
				GatewayClass: "xyz",
				GatewayPort:  7443,
			},
			ssaRecorder: newServerSideApplyRecorder(),
			expectedSSA: map[string]*unstructured.Unstructured{
				"test/skupper": gateway("skupper", "test"),
			},
			definition: &skupperv2alpha1.SecuredAccess{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "skupper.io/v2alpha1",
					Kind:       "SecuredAccess",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "mysvc",
					Namespace: "test",
				},
				Spec: skupperv2alpha1.SecuredAccessSpec{
					AccessType: ACCESS_TYPE_GATEWAY,
					Selector: map[string]string{
						"app": "foo",
					},
					Ports: []skupperv2alpha1.SecuredAccessPort{
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
					},
					Certificate: "my-cert",
					Issuer:      "skupper-site-ca",
				},
			},
			expectedServices: []*corev1.Service{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "mysvc",
						Namespace: "test",
					},
					Spec: corev1.ServiceSpec{
						Selector: map[string]string{
							"app": "foo",
						},
						Ports: []corev1.ServicePort{
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
						},
					},
				},
			},
			expectedCertificates: []MockCertificate{
				{
					namespace: "test",
					name:      "my-cert",
					ca:        "skupper-site-ca",
					subject:   "mysvc",
					hosts:     []string{"mysvc", "mysvc.test"},
					client:    false,
					server:    true,
					refs:      nil,
				},
			},
			expectedStatus: "Gateway base domain not yet resolved",
		},
	}
	for _, tt := range testTable {
		t.Run(tt.name, func(t *testing.T) {
			client, err := fakeclient.NewFakeClient(tt.definition.Namespace, tt.k8sObjects, tt.skupperObjects, "")
			if err != nil {
				assert.Assert(t, err)
			}
			if tt.ssaRecorder != nil {
				assert.Assert(t, tt.ssaRecorder.enable(client.GetDynamicClient()))
			}
			certs := newMockCertificateManager()
			m := NewSecuredAccessManager(client, certs, &tt.config, &FakeControllerContext{namespace: "test", labels: tt.labels, annotations: tt.annotations})

			err = m.Ensure(tt.definition.Namespace, tt.definition.Name, tt.definition.Spec, nil, nil)
			if tt.expectedError != "" {
				assert.ErrorContains(t, err, tt.expectedError)
			} else if err != nil {
				t.Error(err)
			} else {
				sa, err := m.clients.GetSkupperClient().SkupperV2alpha1().SecuredAccesses(tt.definition.Namespace).Get(context.Background(), tt.definition.Name, metav1.GetOptions{})
				assert.Assert(t, err)
				err = m.SecuredAccessChanged(fmt.Sprintf("%s/%s", sa.Namespace, sa.Name), sa)
				for _, desired := range tt.expectedServices {
					actual, err := m.clients.GetKubeClient().CoreV1().Services(desired.Namespace).Get(context.Background(), desired.Name, metav1.GetOptions{})
					assert.Assert(t, err)
					assert.DeepEqual(t, desired.Spec.Selector, actual.Spec.Selector)
					assert.Equal(t, len(desired.Spec.Ports), len(actual.Spec.Ports))
					for _, port := range desired.Spec.Ports {
						assert.Assert(t, cmp.Contains(actual.Spec.Ports, port))
					}
					assert.DeepEqual(t, desired.Spec.Type, actual.Spec.Type)
					if !reflect.DeepEqual(desired.Status, actual.Status) {
						actual.Status = desired.Status
						actual, err = m.clients.GetKubeClient().CoreV1().Services(desired.Namespace).UpdateStatus(context.Background(), actual, metav1.UpdateOptions{})
						assert.Assert(t, err)
					}
					if tt.nodePorts != nil {
						var updatedPorts []corev1.ServicePort
						for _, port := range actual.Spec.Ports {
							if nodePort, ok := tt.nodePorts[port.Name]; ok {
								port.NodePort = nodePort
							}
							updatedPorts = append(updatedPorts, port)
						}
						if !reflect.DeepEqual(updatedPorts, actual.Spec.Ports) {
							actual.Spec.Ports = updatedPorts
							actual, err = m.clients.GetKubeClient().CoreV1().Services(desired.Namespace).Update(context.Background(), actual, metav1.UpdateOptions{})
							assert.Assert(t, err)
						}
					}
					for k, v := range desired.ObjectMeta.Labels {
						assert.Assert(t, actual.ObjectMeta.Labels != nil)
						assert.Equal(t, actual.ObjectMeta.Labels[k], v)
					}
					for k, v := range desired.ObjectMeta.Annotations {
						assert.Assert(t, actual.ObjectMeta.Annotations != nil)
						assert.Equal(t, actual.ObjectMeta.Annotations[k], v)
					}
					err = m.CheckService(serviceKey(actual), actual)
					assert.Assert(t, err)
				}
				for _, desired := range tt.expectedRoutes {
					actual, err := m.clients.GetRouteClient().Routes(desired.Namespace).Get(context.Background(), desired.Name, metav1.GetOptions{})
					assert.Assert(t, err)
					assert.DeepEqual(t, desired.Spec, actual.Spec)
					for k, v := range desired.ObjectMeta.Labels {
						assert.Assert(t, actual.ObjectMeta.Labels != nil)
						assert.Equal(t, actual.ObjectMeta.Labels[k], v)
					}
					for k, v := range desired.ObjectMeta.Annotations {
						assert.Assert(t, actual.ObjectMeta.Annotations != nil)
						assert.Equal(t, actual.ObjectMeta.Annotations[k], v)
					}
					host := actual.Spec.Host
					if host == "" {
						host = fmt.Sprintf("%s.%s.%s", actual.Name, actual.Namespace, tt.defaultDomain)
					}
					actual.Status.Ingress = append(actual.Status.Ingress, routev1.RouteIngress{Host: host})
					actual, err = m.clients.GetRouteClient().Routes(desired.Namespace).UpdateStatus(context.Background(), actual, metav1.UpdateOptions{})
					assert.Assert(t, err)
					err = m.CheckRoute(actual.Namespace+"/"+actual.Name, actual)
					assert.Assert(t, err)
				}
				for _, desired := range tt.expectedIngresses {
					actual, err := m.clients.GetKubeClient().NetworkingV1().Ingresses(desired.Namespace).Get(context.Background(), desired.Name, metav1.GetOptions{})
					assert.Assert(t, err)
					assert.Equal(t, len(desired.Spec.Rules), len(actual.Spec.Rules))
					for _, rule := range desired.Spec.Rules {
						assert.Assert(t, cmp.Contains(actual.Spec.Rules, rule))
					}
					for k, v := range desired.ObjectMeta.Labels {
						assert.Assert(t, actual.ObjectMeta.Labels != nil)
						assert.Equal(t, actual.ObjectMeta.Labels[k], v)
					}
					for k, v := range desired.ObjectMeta.Annotations {
						assert.Assert(t, actual.ObjectMeta.Annotations != nil)
						assert.Equal(t, actual.ObjectMeta.Annotations[k], v)
					}
					if !reflect.DeepEqual(desired.Status, actual.Status) {
						actual.Status = desired.Status
						actual, err = m.clients.GetKubeClient().NetworkingV1().Ingresses(desired.Namespace).UpdateStatus(context.Background(), actual, metav1.UpdateOptions{})
						assert.Assert(t, err)
					}
					err = m.CheckIngress(actual.Namespace+"/"+actual.Name, actual)
					assert.Assert(t, err)
				}
				for _, desired := range tt.expectedProxies {
					obj, err := m.clients.GetDynamicClient().Resource(httpProxyResource).Namespace(tt.definition.Namespace).Get(context.TODO(), desired.Name, metav1.GetOptions{})
					assert.Assert(t, err)
					actual := HttpProxy{
						Name: desired.Name,
					}
					err = actual.readFromContourProxy(obj)
					assert.Assert(t, err)
					assert.Equal(t, desired, actual)
					err = m.CheckHttpProxy(obj.GetNamespace()+"/"+obj.GetName(), obj)
					assert.Assert(t, err)
				}
				if len(tt.expectedSSA) > 0 {
					assert.Equal(t, len(tt.expectedSSA), len(tt.ssaRecorder.objects))
					for key, desired := range tt.expectedSSA {
						actual, ok := tt.ssaRecorder.objects[key]
						assert.Assert(t, ok, "No ssa object found for "+key)
						assert.Equal(t, desired.GetName(), actual.GetName())
						assert.Equal(t, desired.GetNamespace(), actual.GetNamespace())
						assert.Equal(t, desired.GroupVersionKind(), actual.GroupVersionKind())
						if actual.GroupVersionKind().Kind == "TLSRoute" {
							m.CheckTlsRoute(actual.GetNamespace()+"/"+actual.GetName(), actual)
						}
					}
				}
				certs.checkCertificates(t, tt.expectedCertificates)
				//retrieve securedaccess again and verify status
				sa, err = m.clients.GetSkupperClient().SkupperV2alpha1().SecuredAccesses(tt.definition.Namespace).Get(context.Background(), tt.definition.Name, metav1.GetOptions{})
				assert.Assert(t, err)
				assert.Equal(t, tt.expectedStatus, sa.Status.Message)
				for _, endpoint := range tt.expectedEndpoints {
					assert.Assert(t, cmp.Contains(sa.Status.Endpoints, endpoint))
				}
				assert.Equal(t, len(tt.expectedEndpoints), len(sa.Status.Endpoints), "wrong number of endpoints")
			}
		})
	}
}

func TestSecuredAccessManagerEnsure(t *testing.T) {
	type args struct {
		namespace   string
		name        string
		spec        skupperv2alpha1.SecuredAccessSpec
		annotations map[string]string
		refs        []metav1.OwnerReference
	}

	testTable := []struct {
		name           string
		args           []args
		k8sObjects     []runtime.Object
		skupperObjects []runtime.Object
		errors         []ClientError
		skupperError   string
		expectedErrors []string
	}{
		{
			name: "no existing secureAccess",
			args: []args{
				{
					namespace: "test",
					name:      "skupper",
					spec: skupperv2alpha1.SecuredAccessSpec{
						AccessType: ACCESS_TYPE_LOADBALANCER,
						Ports: []skupperv2alpha1.SecuredAccessPort{
							{
								Name:       "port1",
								Port:       8080,
								TargetPort: 8081,
								Protocol:   "TCP",
							},
						},
						Certificate: "skupper",
						Issuer:      "skupper-site-ca",
					},
					annotations: map[string]string{
						"internal.skupper.io/controlled":   "true",
						"internal.skupper.io/routeraccess": "name",
					},
					refs: []metav1.OwnerReference{
						{
							Kind:       "SecuredAccess",
							APIVersion: "skupper.io/v2alpha1",
							Name:       "ownerRef",
							UID:        "8a96ffdf-403b-4e4a-83a8-97d3d459adb6",
						},
					},
				},
			},
		},
		{
			name: "changed spec",
			args: []args{
				{
					namespace: "test",
					name:      "skupper",
					spec: skupperv2alpha1.SecuredAccessSpec{
						AccessType: ACCESS_TYPE_LOADBALANCER,
						Ports: []skupperv2alpha1.SecuredAccessPort{
							{
								Name:       "port1",
								Port:       8080,
								TargetPort: 8081,
								Protocol:   "TCP",
							},
						},
						Certificate: "skupper",
						Issuer:      "skupper-site-ca",
					},
					annotations: map[string]string{
						"internal.skupper.io/controlled":   "true",
						"internal.skupper.io/routeraccess": "name",
					},
					refs: []metav1.OwnerReference{
						{
							Kind:       "SecuredAccess",
							APIVersion: "skupper.io/v2alpha1",
							Name:       "ownerRef",
							UID:        "8a96ffdf-403b-4e4a-83a8-97d3d459adb6",
						},
					},
				},
				{
					namespace: "test",
					name:      "skupper",
					spec: skupperv2alpha1.SecuredAccessSpec{
						AccessType: ACCESS_TYPE_LOADBALANCER,
						Ports: []skupperv2alpha1.SecuredAccessPort{
							{
								Name:       "port1",
								Port:       9090,
								TargetPort: 9091,
								Protocol:   "TCP",
							},
						},
						Certificate: "skupper",
						Issuer:      "skupper-site-ca",
					},
					annotations: map[string]string{
						"internal.skupper.io/controlled":   "true",
						"internal.skupper.io/routeraccess": "name",
					},
					refs: []metav1.OwnerReference{
						{
							Kind:       "SecuredAccess",
							APIVersion: "skupper.io/v2alpha1",
							Name:       "ownerRef",
							UID:        "8a96ffdf-403b-4e4a-83a8-97d3d459adb6",
						},
					},
				},
			},
		},
		{
			name: "changed owners",
			args: []args{
				{
					namespace: "test",
					name:      "skupper",
					spec: skupperv2alpha1.SecuredAccessSpec{
						AccessType: ACCESS_TYPE_LOADBALANCER,
						Ports: []skupperv2alpha1.SecuredAccessPort{
							{
								Name:       "port1",
								Port:       8080,
								TargetPort: 8081,
								Protocol:   "TCP",
							},
						},
						Certificate: "skupper",
						Issuer:      "skupper-site-ca",
					},
					annotations: map[string]string{
						"internal.skupper.io/controlled":   "true",
						"internal.skupper.io/routeraccess": "name",
					},
					refs: []metav1.OwnerReference{
						{
							Kind:       "Site",
							APIVersion: "skupper.io/v2alpha1",
							Name:       "ownerRef",
							UID:        "ffffffdf-403b-4e4a-83a8-97d3d459adb6",
						},
						{
							Kind:       "Site",
							APIVersion: "skupper.io/v2alpha1",
							Name:       "ownerRef",
							UID:        "eeeeeeee-403b-4e4a-83a8-97d3d459adb6",
						},
					},
				},
				{
					namespace: "test",
					name:      "skupper",
					spec: skupperv2alpha1.SecuredAccessSpec{
						AccessType: ACCESS_TYPE_LOADBALANCER,
						Ports: []skupperv2alpha1.SecuredAccessPort{
							{
								Name:       "port1",
								Port:       8080,
								TargetPort: 8081,
								Protocol:   "TCP",
							},
						},
						Certificate: "skupper",
						Issuer:      "skupper-site-ca",
					},
					annotations: map[string]string{
						"internal.skupper.io/controlled":   "true",
						"internal.skupper.io/routeraccess": "name",
					},
					refs: []metav1.OwnerReference{
						{
							Kind:       "SecuredAccess",
							APIVersion: "skupper.io/v2alpha1",
							Name:       "ownerRef",
							UID:        "000000df-403b-4e4a-83a8-97d3d459adb6",
						},
					},
				},
			},
		},
		{
			name: "changed annotations",
			args: []args{
				{
					namespace: "test",
					name:      "skupper",
					spec: skupperv2alpha1.SecuredAccessSpec{
						AccessType: ACCESS_TYPE_LOADBALANCER,
						Ports: []skupperv2alpha1.SecuredAccessPort{
							{
								Name:       "port1",
								Port:       8080,
								TargetPort: 8081,
								Protocol:   "TCP",
							},
						},
						Certificate: "skupper",
						Issuer:      "skupper-site-ca",
					},
					annotations: map[string]string{
						"internal.skupper.io/controlled":   "true",
						"internal.skupper.io/routeraccess": "name",
					},
					refs: []metav1.OwnerReference{
						{
							Kind:       "SecuredAccess",
							APIVersion: "skupper.io/v2alpha1",
							Name:       "ownerRef",
							UID:        "8a96ffdf-403b-4e4a-83a8-97d3d459adb6",
						},
					},
				},
				{
					namespace: "test",
					name:      "skupper",
					spec: skupperv2alpha1.SecuredAccessSpec{
						AccessType: ACCESS_TYPE_LOADBALANCER,
						Ports: []skupperv2alpha1.SecuredAccessPort{
							{
								Name:       "port1",
								Port:       8080,
								TargetPort: 8081,
								Protocol:   "TCP",
							},
						},
						Certificate: "skupper",
						Issuer:      "skupper-site-ca",
					},
					annotations: map[string]string{
						"internal.skupper.io/controlled":   "true",
						"internal.skupper.io/routeraccess": "name",
						"foo":                              "bar",
					},
					refs: []metav1.OwnerReference{
						{
							Kind:       "SecuredAccess",
							APIVersion: "skupper.io/v2alpha1",
							Name:       "ownerRef",
							UID:        "8a96ffdf-403b-4e4a-83a8-97d3d459adb6",
						},
					},
				},
			},
		},
		{
			name: "no change",
			args: []args{
				{
					namespace: "test",
					name:      "skupper",
					spec: skupperv2alpha1.SecuredAccessSpec{
						AccessType: ACCESS_TYPE_LOADBALANCER,
						Ports: []skupperv2alpha1.SecuredAccessPort{
							{
								Name:       "port1",
								Port:       8080,
								TargetPort: 8081,
								Protocol:   "TCP",
							},
						},
						Certificate: "skupper",
						Issuer:      "skupper-site-ca",
					},
					annotations: map[string]string{
						"internal.skupper.io/controlled":   "true",
						"internal.skupper.io/routeraccess": "name",
					},
					refs: []metav1.OwnerReference{
						{
							Kind:       "SecuredAccess",
							APIVersion: "skupper.io/v2alpha1",
							Name:       "ownerRef",
							UID:        "8a96ffdf-403b-4e4a-83a8-97d3d459adb6",
						},
					},
				},
				{
					namespace: "test",
					name:      "skupper",
					spec: skupperv2alpha1.SecuredAccessSpec{
						AccessType: ACCESS_TYPE_LOADBALANCER,
						Ports: []skupperv2alpha1.SecuredAccessPort{
							{
								Name:       "port1",
								Port:       8080,
								TargetPort: 8081,
								Protocol:   "TCP",
							},
						},
						Certificate: "skupper",
						Issuer:      "skupper-site-ca",
					},
					annotations: map[string]string{
						"internal.skupper.io/controlled":   "true",
						"internal.skupper.io/routeraccess": "name",
					},
					refs: []metav1.OwnerReference{
						{
							Kind:       "SecuredAccess",
							APIVersion: "skupper.io/v2alpha1",
							Name:       "ownerRef",
							UID:        "8a96ffdf-403b-4e4a-83a8-97d3d459adb6",
						},
					},
				},
			},
		},
		{
			name: "error on create",
			args: []args{
				{
					namespace: "test",
					name:      "skupper",
					spec: skupperv2alpha1.SecuredAccessSpec{
						AccessType: ACCESS_TYPE_LOADBALANCER,
						Ports: []skupperv2alpha1.SecuredAccessPort{
							{
								Name:       "port1",
								Port:       8080,
								TargetPort: 8081,
								Protocol:   "TCP",
							},
						},
						Certificate: "skupper",
						Issuer:      "skupper-site-ca",
					},
					annotations: map[string]string{
						"internal.skupper.io/controlled":   "true",
						"internal.skupper.io/routeraccess": "name",
					},
					refs: []metav1.OwnerReference{
						{
							Kind:       "SecuredAccess",
							APIVersion: "skupper.io/v2alpha1",
							Name:       "ownerRef",
							UID:        "8a96ffdf-403b-4e4a-83a8-97d3d459adb6",
						},
					},
				},
			},
			skupperError: "create is blocked",
			expectedErrors: []string{
				"create is blocked",
			},
		},
		{
			name: "error on update",
			args: []args{
				{
					namespace: "test",
					name:      "skupper",
					spec: skupperv2alpha1.SecuredAccessSpec{
						AccessType: ACCESS_TYPE_LOADBALANCER,
						Ports: []skupperv2alpha1.SecuredAccessPort{
							{
								Name:       "port1",
								Port:       8080,
								TargetPort: 8081,
								Protocol:   "TCP",
							},
						},
						Certificate: "skupper",
						Issuer:      "skupper-site-ca",
					},
					annotations: map[string]string{
						"internal.skupper.io/controlled":   "true",
						"internal.skupper.io/routeraccess": "name",
					},
					refs: []metav1.OwnerReference{
						{
							Kind:       "SecuredAccess",
							APIVersion: "skupper.io/v2alpha1",
							Name:       "ownerRef",
							UID:        "8a96ffdf-403b-4e4a-83a8-97d3d459adb6",
						},
					},
				},
				{
					namespace: "test",
					name:      "skupper",
					spec: skupperv2alpha1.SecuredAccessSpec{
						AccessType: ACCESS_TYPE_LOADBALANCER,
						Ports: []skupperv2alpha1.SecuredAccessPort{
							{
								Name:       "port1",
								Port:       8086,
								TargetPort: 8081,
								Protocol:   "TCP",
							},
						},
						Certificate: "skupper",
						Issuer:      "skupper-site-ca",
					},
					annotations: map[string]string{
						"internal.skupper.io/controlled":   "true",
						"internal.skupper.io/routeraccess": "name",
					},
					refs: []metav1.OwnerReference{
						{
							Kind:       "SecuredAccess",
							APIVersion: "skupper.io/v2alpha1",
							Name:       "ownerRef",
							UID:        "8a96ffdf-403b-4e4a-83a8-97d3d459adb6",
						},
					},
				},
			},
			errors: []ClientError{
				SkupperClientError("update", "*", "update is blocked"),
			},
			expectedErrors: []string{
				"",
				"update is blocked",
			},
		},
	}
	for _, tt := range testTable {
		t.Run(tt.name, func(t *testing.T) {
			m, err := newSecureAccessManagerMocks("test", tt.k8sObjects, tt.skupperObjects, tt.skupperError)
			assert.Assert(t, err)

			for _, e := range tt.errors {
				e.Prepend(m.clients)
			}

			for i, args := range tt.args {
				err = m.Ensure(args.namespace, args.name, args.spec, args.annotations, args.refs)
				if len(tt.expectedErrors) > i && tt.expectedErrors[i] != "" {
					assert.ErrorContains(t, err, tt.expectedErrors[i])
				} else if err != nil {
					t.Error(err)
				} else {
					// retrieve SecuredAccess instance from API and verify it matches expectations
					actual, err := m.clients.GetSkupperClient().SkupperV2alpha1().SecuredAccesses(args.namespace).Get(context.Background(), args.name, metav1.GetOptions{})
					if err != nil {
						t.Error(err)
					} else {
						assert.DeepEqual(t, actual.Spec, args.spec)
						for key, value := range args.annotations {
							assert.Equal(t, actual.Annotations[key], value)
						}
						assert.DeepEqual(t, actual.OwnerReferences, args.refs)
					}
				}
			}
		})
	}
}

func TestSecuredAccessDeleted(t *testing.T) {
	type recreate struct {
		namespace string
		name      string
		spec      skupperv2alpha1.SecuredAccessSpec
	}
	testTable := []struct {
		name                 string
		config               Config
		k8sObjects           []runtime.Object
		skupperObjects       []runtime.Object
		errors               []ClientError
		recreate             []recreate
		expectedServices     []*corev1.Service
		expectedRoutes       []*routev1.Route
		expectedIngresses    []*networkingv1.Ingress
		expectedProxies      []ExpectedHttpProxy
		expectedCertificates []MockCertificate
		expectedStatuses     []*skupperv2alpha1.SecuredAccess
	}{
		{
			name: "simple",
			config: Config{
				EnabledAccessTypes: []string{"loadbalancer"},
			},
			k8sObjects: []runtime.Object{
				addLoadbalancerIP(service("mysvc", "test", selector(), corev1.ServiceTypeLoadBalancer, servicePorts()), "10.1.1.10"),
			},
			skupperObjects: []runtime.Object{
				securedAccess("mysvc", "test", selector(), securedAccessPorts()),
			},
			recreate: []recreate{
				{
					namespace: "test",
					name:      "mysvc",
					spec:      securedAccess("mysvc", "test", selector(), securedAccessPorts()).Spec,
				},
			},
			expectedServices: []*corev1.Service{
				service("mysvc", "test", selector(), corev1.ServiceTypeLoadBalancer, servicePorts()),
			},
			expectedStatuses: []*skupperv2alpha1.SecuredAccess{
				status("mysvc", "test", "OK", endpoint("a", "8080", "10.1.1.10"), endpoint("b", "9090", "10.1.1.10")),
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
			for i := 0; i < len(tt.k8sObjects)+len(tt.skupperObjects); i++ {
				controller.TestProcess()
			}

			for _, def := range tt.recreate {
				controller.TestProcess() //assume we have an event from the status update of each resource to recreate; need to process that before deleting
				err := client.GetSkupperClient().SkupperV2alpha1().SecuredAccesses(def.namespace).Delete(context.Background(), def.name, metav1.DeleteOptions{})
				assert.Assert(t, err)
				controller.TestProcess()
				err = m.Ensure(def.namespace, def.name, def.spec, nil, nil)
				assert.Assert(t, err)
				controller.TestProcess()
			}

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
				err = actual.readFromContourProxy(obj)
				assert.Assert(t, err)
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

func TestSecuredAccessManagerChangeDelete(t *testing.T) {
	type args struct {
		namespace   string
		name        string
		spec        *skupperv2alpha1.SecuredAccessSpec
		annotations *map[string]string
		refs        *[]metav1.OwnerReference
	}

	testTable := []struct {
		name                string
		args                args
		k8sObjects          []runtime.Object
		skupperObjects      []runtime.Object
		skupperErrorMessage string
		current             *skupperv2alpha1.SecuredAccess
	}{
		{
			name: "no existing secureAccess",
			args: args{
				namespace: "test",
				name:      "skupper",
				spec: &skupperv2alpha1.SecuredAccessSpec{
					AccessType: ACCESS_TYPE_LOADBALANCER,
					Ports: []skupperv2alpha1.SecuredAccessPort{
						{
							Name:       "port1",
							Port:       8080,
							TargetPort: 8081,
							Protocol:   "TCP",
						},
					},
					Certificate: "skupper",
					Issuer:      "skupper-site-ca",
				},
				annotations: &map[string]string{
					"internal.skupper.io/controlled":   "true",
					"internal.skupper.io/routeraccess": "name",
				},
				refs: &[]metav1.OwnerReference{
					{
						Kind:       "SecuredAccess",
						APIVersion: "skupper.io/v2alpha1",
						Name:       "ownerRef",
						UID:        "8a96ffdf-403b-4e4a-83a8-97d3d459adb6",
					},
				},
			},
			current: &skupperv2alpha1.SecuredAccess{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "skupper.io/v2alpha1",
					Kind:       "SecuredAccess",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "skupper",
					Namespace: "test",
					OwnerReferences: []metav1.OwnerReference{
						{
							Kind:       "SecuredAccess",
							APIVersion: "skupper.io/v2alpha1",
							Name:       "ownerRef",
							UID:        "8a96ffdf-403b-4e4a-83a8-97d3d459adb6",
						},
					},
					Annotations: map[string]string{
						"internal.skupper.io/controlled":   "true",
						"internal.skupper.io/routeraccess": "name",
					},
				},
				Spec: skupperv2alpha1.SecuredAccessSpec{
					AccessType: ACCESS_TYPE_LOCAL,
					Ports: []skupperv2alpha1.SecuredAccessPort{
						{
							Name:       "port1",
							Port:       8080,
							TargetPort: 8081,
							Protocol:   "TCP",
						},
					},
					Certificate: "skupper",
					Issuer:      "",
				},
			},
		},
	}

	for _, tt := range testTable {
		t.Run(tt.name, func(t *testing.T) {
			m, err := newSecureAccessManagerMocks("test", tt.k8sObjects, tt.skupperObjects, tt.skupperErrorMessage)
			assert.Assert(t, err)

			if err := m.Ensure(tt.args.namespace, tt.args.name, *tt.args.spec, *tt.args.annotations, *tt.args.refs); err != nil {
				t.Errorf("SecuredAccessManager.Ensure() error = %v", err)
			}
			// change Access Type
			if err = m.SecuredAccessChanged("test/skupper", tt.current); err != nil {
				t.Errorf("SecuredAccessManager.SecuredAccessChanged error = %v", err)
			}
			// expect one service and one definition
			if len(m.definitions) != 1 {
				t.Errorf("SecuredAccessManager.Ensure() expected one definition")
			}
			if len(m.services) != 1 {
				t.Errorf("SecuredAccessManager.Ensure() expected one service")
			}
			// delete secured access
			if err = m.SecuredAccessDeleted("test/skupper"); err != nil {
				t.Errorf("SecuredAccessManager.SecuredAccessDeleted() failed delete error= %v", err)
			}
			// expect no services or definitions
			if len(m.definitions) != 0 {
				t.Errorf("SecuredAccessManager.SecuredAccessDeleted() expected no definition")
			}
			hosts := getHosts(tt.current)
			if len(hosts) != 2 {
				t.Errorf("SecuredAccessManager.getHosts() expected %d", len(hosts))
			}
		})
	}
}

func TestSecuredAccessManagerCheckService(t *testing.T) {
	type args struct {
		namespace   string
		name        string
		spec        *skupperv2alpha1.SecuredAccessSpec
		annotations *map[string]string
		refs        *[]metav1.OwnerReference
	}

	testTable := []struct {
		name                string
		args                args
		k8sObjects          []runtime.Object
		skupperObjects      []runtime.Object
		skupperErrorMessage string
		svc                 *corev1.Service
		expectErr           bool
	}{
		{
			name: "no existing service",
			args: args{
				namespace: "test",
				name:      "skupper",
				spec: &skupperv2alpha1.SecuredAccessSpec{
					AccessType: ACCESS_TYPE_LOADBALANCER,
					Ports: []skupperv2alpha1.SecuredAccessPort{
						{
							Name:       "port1",
							Port:       8080,
							TargetPort: 8081,
							Protocol:   "TCP",
						},
					},
					Certificate: "skupper",
					Issuer:      "skupper-site-ca",
				},
				annotations: &map[string]string{
					"internal.skupper.io/controlled":   "true",
					"internal.skupper.io/routeraccess": "name",
				},
				refs: &[]metav1.OwnerReference{
					{
						Kind:       "SecuredAccess",
						APIVersion: "skupper.io/v2alpha1",
						Name:       "ownerRef",
						UID:        "8a96ffdf-403b-4e4a-83a8-97d3d459adb6",
					},
				},
			},
			expectErr: false,
		},
		{
			name: "existing service",
			args: args{
				namespace: "test",
				name:      "skupper",
				spec: &skupperv2alpha1.SecuredAccessSpec{
					AccessType: ACCESS_TYPE_LOADBALANCER,
					Selector: map[string]string{
						"skupper.io/component": "router",
					},
					Ports: []skupperv2alpha1.SecuredAccessPort{
						{
							Name:       "port1",
							Port:       8080,
							TargetPort: 8081,
							Protocol:   "TCP",
						},
					},
					Certificate: "skupper",
					Issuer:      "skupper-site-ca",
				},
				annotations: &map[string]string{
					"internal.skupper.io/controlled": "true",
				},
				refs: &[]metav1.OwnerReference{
					{
						Kind:       "SecuredAccess",
						APIVersion: "skupper.io/v2alpha1",
						Name:       "ownerRef",
						UID:        "8a96ffdf-403b-4e4a-83a8-97d3d459adb6",
					},
				},
			},
			svc: &corev1.Service{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "v1",
					Kind:       "Service",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "skupper",
					Namespace: "test",
					Labels: map[string]string{
						"internal.skupper.io/secured-access": "true",
					},
					Annotations: map[string]string{
						"internal.skupper.io/controlled": "true",
					},
				},
				Spec: corev1.ServiceSpec{
					Selector: map[string]string{
						"skupper.io/component": "router",
					},
					Type: corev1.ServiceTypeLoadBalancer, //ACCESS_TYPE_ROUTE,
					Ports: []corev1.ServicePort{
						{
							Name:       "port1",
							Port:       8080,
							TargetPort: intstr.IntOrString{IntVal: int32(8081)},
							Protocol:   "TCP",
						},
					},
				},
			},
			expectErr: false,
		},
		{
			name: "service parameter change",
			args: args{
				namespace: "test",
				name:      "skupper",
				spec: &skupperv2alpha1.SecuredAccessSpec{
					AccessType: ACCESS_TYPE_LOADBALANCER,
					Selector: map[string]string{
						"skupper.io/component": "router",
					},
					Ports: []skupperv2alpha1.SecuredAccessPort{
						{
							Name:       "port1",
							Port:       8080,
							TargetPort: 8081,
							Protocol:   "TCP",
						},
					},
					Certificate: "skupper",
					Issuer:      "skupper-site-ca",
				},
				annotations: &map[string]string{
					"internal.skupper.io/controlled": "true",
				},
				refs: &[]metav1.OwnerReference{
					{
						Kind:       "SecuredAccess",
						APIVersion: "skupper.io/v2alpha1",
						Name:       "ownerRef",
						UID:        "8a96ffdf-403b-4e4a-83a8-97d3d459adb6",
					},
				},
			},
			svc: &corev1.Service{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "v1",
					Kind:       "Service",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "skupper",
					Namespace: "test",
					Labels: map[string]string{
						"internal.skupper.io/secured-access": "true",
					},
					Annotations: map[string]string{
						"internal.skupper.io/controlled": "true",
					},
				},
				Spec: corev1.ServiceSpec{
					Selector: map[string]string{
						"skupper.io/component": "controller",
					},
					Type: corev1.ServiceTypeClusterIP,
					Ports: []corev1.ServicePort{
						{
							Name:       "port1",
							Port:       8880,
							TargetPort: intstr.IntOrString{IntVal: int32(8881)},
							Protocol:   "TCP",
						},
					},
				},
			},
			expectErr: true,
		},
	}

	for _, tt := range testTable {
		t.Run(tt.name, func(t *testing.T) {
			m, err := newSecureAccessManagerMocks("test", tt.k8sObjects, tt.skupperObjects, tt.skupperErrorMessage)
			assert.Assert(t, err)

			if err := m.Ensure(tt.args.namespace, tt.args.name, *tt.args.spec, *tt.args.annotations, *tt.args.refs); err != nil {
				t.Errorf("SecuredAccessManager.CheckServic() error = %v", err)
			}

			// check service
			if err = m.CheckService("test/skupper", tt.svc); err != nil {
				if tt.expectErr {
					// expected error since service doesn't exist so create one
					m.RecoverService(tt.svc)
				} else {
					t.Errorf("SecuredAccessManager.CheckService error = %v", err)
				}
			}
			// expect one service and one definition
			if len(m.definitions) != 1 {
				t.Errorf("SecuredAccessManager.CheckService() expected one definition")
			}
			if len(m.services) != 1 {
				t.Errorf("SecuredAccessManager.CheckService() expected one service")
			}
			// check values were modified
			if tt.svc != nil {
				skey := serviceKey(tt.svc)
				if !reflect.DeepEqual(tt.svc.Spec.Selector, m.services[skey].Spec.Selector) {
					t.Errorf("SecuredAccessManager.updateSelector() expected selector to be updated")
				}
				if tt.svc.Spec.Type != m.services[skey].Spec.Type {
					t.Errorf("SecuredAccessManager.updateType() expected Type to be updated")
				}
			}
		})
	}
}

func TestSecuredAccessManagerRecoverRoute(t *testing.T) {
	type args struct {
		route *routev1.Route
	}
	testTable := []struct {
		name                string
		args                args
		k8sObjects          []runtime.Object
		skupperObjects      []runtime.Object
		skupperErrorMessage string
	}{
		{
			name: "good route",
			args: args{
				route: &routev1.Route{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "v1",
						Kind:       "Route",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "route",
						Namespace: "test",
					},
					Spec: routev1.RouteSpec{
						Path: "",
						Host: "test/host",
						Port: &routev1.RoutePort{
							TargetPort: intstr.FromString("8080"),
						},
					},
				},
			},
		},
	}

	for _, tt := range testTable {
		t.Run(tt.name, func(t *testing.T) {
			m, err := newSecureAccessManagerMocks("test", tt.k8sObjects, tt.skupperObjects, tt.skupperErrorMessage)
			assert.Assert(t, err)

			m.RecoverRoute(tt.args.route)
			if len(m.routes) != 1 {
				t.Errorf("SecuredAccessManager.RecoverRoute() expect one route")
			}
		})
	}
}

func TestSecuredAccessManagerRecoverIngress(t *testing.T) {
	type args struct {
		ingress *networkingv1.Ingress
	}
	testTable := []struct {
		name                string
		args                args
		k8sObjects          []runtime.Object
		skupperObjects      []runtime.Object
		skupperErrorMessage string
	}{
		{
			name: "ingress",
			args: args{
				ingress: &networkingv1.Ingress{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "v1",
						Kind:       "Ingress",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "route1",
						Namespace: "test",
					},
				},
			},
		},
	}

	for _, tt := range testTable {
		t.Run(tt.name, func(t *testing.T) {
			m, err := newSecureAccessManagerMocks("test", tt.k8sObjects, tt.skupperObjects, tt.skupperErrorMessage)
			assert.Assert(t, err)

			m.RecoverIngress(tt.args.ingress)
			if len(m.ingresses) != 1 {
				t.Errorf("SecuredAccessManager.RecoverIngress() expect one route")
			}
		})
	}
}

// --- helper methods
func newSecureAccessManagerMocks(namespace string, k8sObjects []runtime.Object, skupperObjects []runtime.Object, fakeSkupperError string) (*SecuredAccessManager, error) {

	client, err := fakeclient.NewFakeClient(namespace, k8sObjects, skupperObjects, fakeSkupperError)
	if err != nil {
		return nil, err
	}

	securedAccessManager := &SecuredAccessManager{
		clients:     client,
		definitions: make(map[string]*skupperv2alpha1.SecuredAccess),
		services:    make(map[string]*corev1.Service),
		routes:      make(map[string]*routev1.Route),
		ingresses:   make(map[string]*networkingv1.Ingress),
		//httpProxies: make(map[string]*unstructured.Unstructured),
		certMgr: newMockCertificateManager(),
	}
	return securedAccessManager, nil
}

type MockCertificate struct {
	namespace string
	name      string
	ca        string
	subject   string
	hosts     []string
	client    bool
	server    bool
	refs      []metav1.OwnerReference
}

type MockCA struct {
	namespace string
	name      string
	subject   string
	refs      []metav1.OwnerReference
}

type MockCertificateManager struct {
	cas    map[string]MockCA
	certs  map[string]MockCertificate
	errors []string
}

func newMockCertificateManager() *MockCertificateManager {
	return &MockCertificateManager{
		cas:   map[string]MockCA{},
		certs: map[string]MockCertificate{},
	}
}

func (m *MockCertificateManager) checkCertificates(t *testing.T, expected []MockCertificate) {
	t.Helper()
	for _, desired := range expected {
		key := desired.namespace + "/" + desired.name
		assert.Equal(t, desired.namespace, m.certs[key].namespace)
		assert.Equal(t, desired.name, m.certs[key].name)
		assert.Equal(t, desired.subject, m.certs[key].subject)
		for _, host := range desired.hosts {
			assert.Assert(t, cmp.Contains(m.certs[key].hosts, host))
		}
		assert.Equal(t, len(desired.hosts), len(m.certs[key].hosts), "wrong number of hosts in certificate")
	}
}

func (m *MockCertificateManager) popError() error {
	if len(m.errors) > 0 {
		n := len(m.errors) - 1
		e := m.errors[n]
		m.errors = m.errors[:n]
		if e != "" {
			return errors.New(e)
		}
	}
	return nil
}

func (m *MockCertificateManager) pushError(e string) {
	m.errors = append(m.errors, e)
}

func (m *MockCertificateManager) EnsureCA(namespace string, name string, subject string, refs []metav1.OwnerReference) error {
	m.cas[namespace+"/"+name] = MockCA{
		namespace: namespace,
		name:      name,
		subject:   subject,
		refs:      refs,
	}
	return m.popError()
}

func (m *MockCertificateManager) Ensure(namespace string, name string, ca string, subject string, hosts []string, client bool, server bool, refs []metav1.OwnerReference) error {
	m.certs[namespace+"/"+name] = MockCertificate{
		namespace: namespace,
		name:      name,
		ca:        ca,
		subject:   subject,
		hosts:     hosts,
		client:    client,
		server:    server,
		refs:      refs,
	}
	return m.popError()
}

func gateway(name string, namespace string) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "gateway.networking.k8s.io",
		Version: "v1",
		Kind:    "Gateway",
	})
	obj.SetName(name)
	obj.SetNamespace(namespace)
	return obj
}

func tlsroute(name string, namespace string) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "gateway.networking.k8s.io",
		Version: "v1alpha2",
		Kind:    "TLSRoute",
	})
	obj.SetLabels(map[string]string{
		"internal.skupper.io/secured-access": "true",
	})
	obj.SetAnnotations(map[string]string{
		"internal.skupper.io/controlled": "true",
	})
	obj.SetName(name)
	obj.SetNamespace(namespace)
	return obj
}

type ServerSideApplyRecorder struct {
	objects   map[string]*unstructured.Unstructured
	modifiers map[string]func(*unstructured.Unstructured)
	errors    map[string]string
}

func newServerSideApplyRecorder() *ServerSideApplyRecorder {
	return &ServerSideApplyRecorder{
		objects:   map[string]*unstructured.Unstructured{},
		modifiers: map[string]func(*unstructured.Unstructured){},
		errors:    map[string]string{},
	}
}

func (recorder *ServerSideApplyRecorder) setGatewayIP(key string, ip string) *ServerSideApplyRecorder {
	recorder.modifiers[key] = func(obj *unstructured.Unstructured) {
		setGatewayAddress(obj, "IPAddress", ip)
	}
	return recorder
}

func (recorder *ServerSideApplyRecorder) setGatewayHostname(key string, hostname string) *ServerSideApplyRecorder {
	recorder.modifiers[key] = func(obj *unstructured.Unstructured) {
		setGatewayAddress(obj, "Hostname", hostname)
	}
	return recorder
}

func (recorder *ServerSideApplyRecorder) setError(key string, err string) *ServerSideApplyRecorder {
	recorder.errors[key] = err
	return recorder
}

func (recorder *ServerSideApplyRecorder) record(obj *unstructured.Unstructured) error {
	key := obj.GetNamespace() + "/" + obj.GetName()
	if err, ok := recorder.errors[key]; ok {
		return errors.New(err)
	}
	recorder.objects[key] = obj
	if modifier, ok := recorder.modifiers[key]; ok {
		modifier(obj)
	}
	return nil
}

func (recorder *ServerSideApplyRecorder) enable(client dynamic.Interface) bool {
	if fc, ok := client.(*fakedynamic.FakeDynamicClient); ok {
		fc.PrependReactor(
			"patch",
			"*",
			func(action k8stesting.Action) (bool, runtime.Object, error) {
				pa := action.(k8stesting.PatchAction)
				if pa.GetPatchType() != types.ApplyPatchType {
					return false, nil, nil
				}
				obj := &unstructured.Unstructured{}
				json.Unmarshal(pa.GetPatch(), obj)
				obj.SetNamespace(pa.GetNamespace())
				if err := recorder.record(obj); err != nil {
					return true, nil, err
				}
				return true, obj, nil
			},
		)
		return true
	}
	return false
}

func setGatewayAddress(obj *unstructured.Unstructured, typeName string, value string) *unstructured.Unstructured {
	addresses := []interface{}{
		map[string]interface{}{
			"type":  typeName,
			"value": value,
		},
	}
	unstructured.SetNestedSlice(obj.UnstructuredContent(), addresses, "status", "addresses")
	return obj
}

func serviceKey(svc *corev1.Service) string {
	return fmt.Sprintf("%s/%s", svc.Namespace, svc.Name)
}

func TestRecreateOnDelete(t *testing.T) {
	type Delete func(clients internalclient.Clients, manager *SecuredAccessManager) error
	testTable := []struct {
		name              string
		config            Config
		k8sObjects        []runtime.Object
		skupperObjects    []runtime.Object
		deletions         []Delete
		expectedServices  []*corev1.Service
		expectedRoutes    []*routev1.Route
		expectedIngresses []*networkingv1.Ingress
		expectedProxies   []ExpectedHttpProxy
		expectedSSA       map[string]*unstructured.Unstructured
	}{
		{
			name: "loadbalancer",
			config: Config{
				EnabledAccessTypes: []string{"loadbalancer"},
			},
			k8sObjects: []runtime.Object{
				service("mysvc", "test", selector(), corev1.ServiceTypeLoadBalancer, servicePorts()),
			},
			deletions: []Delete{
				func(clients internalclient.Clients, manager *SecuredAccessManager) error {
					if err := clients.GetKubeClient().CoreV1().Services("test").Delete(context.Background(), "mysvc", metav1.DeleteOptions{}); err != nil {
						return err
					}
					if err := manager.CheckService("test/mysvc", nil); err != nil {
						return err
					}
					return nil
				},
			},
			skupperObjects: []runtime.Object{
				securedAccess("mysvc", "test", selector(), securedAccessPorts()),
			},
			expectedServices: []*corev1.Service{
				service("mysvc", "test", selector(), corev1.ServiceTypeLoadBalancer, servicePorts()),
			},
		},
		{
			name: "redundant loadbalancer",
			config: Config{
				EnabledAccessTypes: []string{"loadbalancer"},
			},
			k8sObjects: []runtime.Object{
				service("mysvc", "test", selector(), corev1.ServiceTypeLoadBalancer, servicePorts()),
				service("foo", "test", selector(), corev1.ServiceTypeLoadBalancer, servicePorts()),
			},
			deletions: []Delete{
				func(clients internalclient.Clients, manager *SecuredAccessManager) error {
					if err := manager.CheckService("test/foo", service("foo", "test", selector(), corev1.ServiceTypeLoadBalancer, servicePorts())); err != nil {
						return err
					}
					if err := manager.CheckService("test/foo", nil); err != nil {
						return err
					}
					return nil
				},
			},
			skupperObjects: []runtime.Object{
				securedAccess("mysvc", "test", selector(), securedAccessPorts()),
			},
			expectedServices: []*corev1.Service{
				service("mysvc", "test", selector(), corev1.ServiceTypeLoadBalancer, servicePorts()),
			},
		},
		{
			name: "route",
			config: Config{
				EnabledAccessTypes: []string{"route"},
			},
			k8sObjects: []runtime.Object{
				service("mysvc", "test", selector(), "", servicePorts()),
				route("mysvc-a", "test", "mysvc", "a", ""),
				route("mysvc-b", "test", "mysvc", "b", ""),
			},
			deletions: []Delete{
				func(clients internalclient.Clients, manager *SecuredAccessManager) error {
					if err := clients.GetRouteClient().Routes("test").Delete(context.Background(), "mysvc-a", metav1.DeleteOptions{}); err != nil {
						return err
					}
					if err := manager.CheckRoute("test/mysvc-a", nil); err != nil {
						return err
					}
					return nil
				},
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
		},
		{
			name: "route",
			config: Config{
				EnabledAccessTypes: []string{"route"},
			},
			k8sObjects: []runtime.Object{
				service("mysvc", "test", selector(), "", servicePorts()),
				route("mysvc-a", "test", "mysvc", "a", ""),
				route("mysvc-b", "test", "mysvc", "b", ""),
				route("mysvc-c", "test", "mysvc", "c", ""),
			},
			deletions: []Delete{
				func(clients internalclient.Clients, manager *SecuredAccessManager) error {
					if err := manager.CheckRoute("test/mysvc-c", route("mysvc-c", "test", "mysvc", "c", "")); err != nil {
						return err
					}
					if err := manager.CheckRoute("test/mysvc-c", nil); err != nil {
						return err
					}
					return nil
				},
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
		},
		{
			name: "ingress",
			config: Config{
				EnabledAccessTypes: []string{"ingress-nginx"},
			},
			k8sObjects: []runtime.Object{
				service("mysvc", "test", selector(), "", servicePorts()),
				ingress("mysvc", "test", ingressRule("a.test", "mysvc", 8080), ingressRule("b.test", "mysvc", 9090)),
			},
			deletions: []Delete{
				func(clients internalclient.Clients, manager *SecuredAccessManager) error {
					if err := clients.GetKubeClient().NetworkingV1().Ingresses("test").Delete(context.Background(), "mysvc", metav1.DeleteOptions{}); err != nil {
						return err
					}
					if err := manager.CheckIngress("test/mysvc", nil); err != nil {
						return err
					}
					return nil
				},
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
		},
		{
			name: "redundant ingress",
			config: Config{
				EnabledAccessTypes: []string{"ingress-nginx"},
			},
			k8sObjects: []runtime.Object{
				service("mysvc", "test", selector(), "", servicePorts()),
				ingress("mysvc", "test", ingressRule("a.test", "mysvc", 8080), ingressRule("b.test", "mysvc", 9090)),
				ingress("foo", "test", ingressRule("a.test", "foo", 8080), ingressRule("b.test", "foo", 9090)),
			},
			deletions: []Delete{
				func(clients internalclient.Clients, manager *SecuredAccessManager) error {
					if err := manager.CheckIngress("test/foo", ingress("foo", "test", ingressRule("a.test", "foo", 8080), ingressRule("b.test", "foo", 9090))); err != nil {
						return err
					}
					if err := manager.CheckIngress("test/foo", nil); err != nil {
						return err
					}
					return nil
				},
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
		},
		{
			name: "http proxy",
			config: Config{
				EnabledAccessTypes: []string{"contour-http-proxy"},
			},
			k8sObjects: []runtime.Object{
				service("mysvc", "test", selector(), "", servicePorts()),
				httpProxy("mysvc-a", "test", "mysvc-a.test", "mysvc", 8080),
				httpProxy("mysvc-b", "test", "mysvc-b.test", "mysvc", 9090),
			},
			deletions: []Delete{
				func(clients internalclient.Clients, manager *SecuredAccessManager) error {
					if err := clients.GetDynamicClient().Resource(httpProxyResource).Namespace("test").Delete(context.TODO(), "mysvc-b", metav1.DeleteOptions{}); err != nil {
						return err
					}
					if err := manager.CheckHttpProxy("test/mysvc-b", nil); err != nil {
						return err
					}
					return nil
				},
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
		},
		{
			name: "redundant http proxy",
			config: Config{
				EnabledAccessTypes: []string{"contour-http-proxy"},
			},
			k8sObjects: []runtime.Object{
				service("mysvc", "test", selector(), "", servicePorts()),
				httpProxy("mysvc-a", "test", "mysvc-a.test", "mysvc", 8080),
				httpProxy("mysvc-b", "test", "mysvc-b.test", "mysvc", 9090),
				httpProxy("foo-bar", "test", "whatever", "bar", 9090),
			},
			deletions: []Delete{
				func(clients internalclient.Clients, manager *SecuredAccessManager) error {
					if err := manager.CheckHttpProxy("test/foo-bar", httpProxy("foo-bar", "test", "whatever", "bar", 9090)); err != nil {
						return err
					}
					if err := manager.CheckHttpProxy("test/foo-bar", nil); err != nil {
						return err
					}
					return nil
				},
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
		},
		{
			name: "tls route",
			config: Config{
				EnabledAccessTypes: []string{"gateway"},
				GatewayClass:       "contour",
				GatewayDomain:      "mygateway.org",
			},
			k8sObjects: []runtime.Object{
				service("mysvc", "test", selector(), "", servicePorts()),
				tlsroute("mysvc-a", "test"),
				tlsroute("mysvc-b", "test"),
			},
			deletions: []Delete{
				func(clients internalclient.Clients, manager *SecuredAccessManager) error {
					if err := manager.CheckTlsRoute("test/mysvc-a", nil); err != nil {
						return err
					}
					return nil
				},
			},
			skupperObjects: []runtime.Object{
				securedAccess("mysvc", "test", selector(), securedAccessPorts()),
			},
			expectedServices: []*corev1.Service{
				service("mysvc", "test", selector(), "", servicePorts()),
			},
			expectedSSA: map[string]*unstructured.Unstructured{
				"test/skupper": gateway("skupper", "test"),
				"test/mysvc-a": tlsroute("mysvc-a", "test"),
				"test/mysvc-b": tlsroute("mysvc-b", "test"),
			},
		},
		{
			name: "redundant tls route",
			config: Config{
				EnabledAccessTypes: []string{"gateway"},
				GatewayClass:       "contour",
				GatewayDomain:      "mygateway.org",
			},
			k8sObjects: []runtime.Object{
				service("mysvc", "test", selector(), "", servicePorts()),
				tlsroute("mysvc-a", "test"),
				tlsroute("mysvc-b", "test"),
				tlsroute("mysvc-c", "test"),
			},
			deletions: []Delete{
				func(clients internalclient.Clients, manager *SecuredAccessManager) error {
					if err := manager.CheckTlsRoute("test/mysvc-c", tlsroute("mysvc-c", "test")); err != nil {
						return err
					}
					if err := manager.CheckTlsRoute("test/mysvc-c", nil); err != nil {
						return err
					}
					return nil
				},
			},
			skupperObjects: []runtime.Object{
				securedAccess("mysvc", "test", selector(), securedAccessPorts()),
			},
			expectedServices: []*corev1.Service{
				service("mysvc", "test", selector(), "", servicePorts()),
			},
			expectedSSA: map[string]*unstructured.Unstructured{
				"test/skupper": gateway("skupper", "test"),
				"test/mysvc-a": tlsroute("mysvc-a", "test"),
				"test/mysvc-b": tlsroute("mysvc-b", "test"),
			},
		},
	}
	for _, tt := range testTable {
		t.Run(tt.name, func(t *testing.T) {
			client, err := fakeclient.NewFakeClient("test", tt.k8sObjects, tt.skupperObjects, "")
			if err != nil {
				assert.Assert(t, err)
			}
			ssaRecorder := newServerSideApplyRecorder()
			assert.Assert(t, ssaRecorder.enable(client.GetDynamicClient()))
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

			//delete:
			for _, d := range tt.deletions {
				assert.Assert(t, d(client, m))
			}

			// check any expected resources are recreated:
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
			if len(tt.expectedSSA) > 0 {
				assert.Equal(t, len(tt.expectedSSA), len(ssaRecorder.objects), "wrong number of objects in server side apply recorder")
				for key, desired := range tt.expectedSSA {
					actual, ok := ssaRecorder.objects[key]
					assert.Assert(t, ok, "No ssa object found for "+key)
					assert.Equal(t, desired.GetName(), actual.GetName())
					assert.Equal(t, desired.GetNamespace(), actual.GetNamespace())
					assert.Equal(t, desired.GroupVersionKind(), actual.GroupVersionKind())
				}
			}
		})
	}
}

type FakeControllerContext struct {
	name        string
	namespace   string
	uid         string
	labels      map[string]string
	annotations map[string]string
}

func (c *FakeControllerContext) IsControlled(namespace string) bool {
	return true
}

func (c *FakeControllerContext) SetLabels(namespace string, name string, kind string, labels map[string]string) bool {
	updated := false
	for k, v := range c.labels {
		if labels[k] != v {
			labels[k] = v
			updated = true
		}
	}
	return updated
}

func (c *FakeControllerContext) SetAnnotations(namespace string, name string, kind string, annotations map[string]string) bool {
	updated := false
	for k, v := range c.annotations {
		if annotations[k] != v {
			annotations[k] = v
			updated = true
		}
	}
	return updated
}

func (c *FakeControllerContext) Namespace() string {
	return c.namespace
}

func (c *FakeControllerContext) Name() string {
	return c.name
}

func (c *FakeControllerContext) UID() string {
	return c.uid
}

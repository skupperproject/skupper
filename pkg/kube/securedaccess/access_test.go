package securedaccess

import (
	"reflect"
	"testing"

	routev1 "github.com/openshift/api/route/v1"
	fakeclient "github.com/skupperproject/skupper/internal/kube/client/fake"
	skupperv1alpha1 "github.com/skupperproject/skupper/pkg/apis/skupper/v1alpha1"
	"gotest.tools/assert"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func TestSecuredAccessManagerEnsure(t *testing.T) {
	type args struct {
		namespace   string
		name        string
		spec        *skupperv1alpha1.SecuredAccessSpec
		annotations *map[string]string
		refs        *[]metav1.OwnerReference
	}

	testTable := []struct {
		name                string
		args                args
		k8sObjects          []runtime.Object
		skupperObjects      []runtime.Object
		skupperErrorMessage string
	}{
		{
			name: "no existing secureAccess",
			args: args{
				namespace: "test",
				name:      "skupper",
				spec: &skupperv1alpha1.SecuredAccessSpec{
					AccessType: ACCESS_TYPE_LOADBALANCER,
					Ports: []skupperv1alpha1.SecuredAccessPort{
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
						APIVersion: "skupper.io/v1alpha1",
						Name:       "ownerRef",
						UID:        "8a96ffdf-403b-4e4a-83a8-97d3d459adb6",
					},
				},
			},
		},
	}
	for _, tt := range testTable {
		t.Run(tt.name, func(t *testing.T) {
			m, err := newSecureAccessManagerMocks("test", tt.k8sObjects, tt.skupperObjects, tt.skupperErrorMessage)
			assert.Assert(t, err)

			if len(m.definitions) != 0 {
				t.Errorf("SecuredAccessManager.Ensure() expected no definitons")
			}
			if err := m.Ensure(tt.args.namespace, tt.args.name, *tt.args.spec, *tt.args.annotations, *tt.args.refs); err != nil {
				t.Errorf("SecuredAccessManager.Ensure() error = %v", err)
			}
			if m.definitions["test/skupper"].Annotations == nil {
				t.Errorf("Annotations not as expected")
			}
			if m.definitions["test/skupper"].OwnerReferences == nil {
				t.Errorf("OwnerReferences not as expected")
			}
			// issue with same values expect return with no changes
			if err := m.Ensure(tt.args.namespace, tt.args.name, *tt.args.spec, *tt.args.annotations, *tt.args.refs); err != nil {
				t.Errorf("SecuredAccessManager.Ensure() issue same, definiton error = %v", err)
			}
			// change key expect two definitions
			if err := m.Ensure(tt.args.namespace, "skupper2", *tt.args.spec, *tt.args.annotations, *tt.args.refs); err != nil {
				t.Errorf("SecuredAccessManager.Ensure() changing key, error = %v", err)
			}
			if len(m.definitions) != 2 {
				t.Errorf("SecuredAccessManager.Ensure() expected two definitions")
			}
			// change existing entry expect two definitions
			tt.args.spec.Ports[0].Name = "Port2"
			if err := m.Ensure(tt.args.namespace, tt.args.name, *tt.args.spec, *tt.args.annotations, *tt.args.refs); err != nil {
				t.Errorf("SecuredAccessManager.Ensure() modifying existing, error = %v", err)
			}
			// still two definitons
			if len(m.definitions) != 2 {
				t.Errorf("SecuredAccessManager.Ensure() expected two entries")
			}
		})
	}
}

func TestSecuredAccessManagerChangeDelete(t *testing.T) {
	type args struct {
		namespace   string
		name        string
		spec        *skupperv1alpha1.SecuredAccessSpec
		annotations *map[string]string
		refs        *[]metav1.OwnerReference
	}

	testTable := []struct {
		name                string
		args                args
		k8sObjects          []runtime.Object
		skupperObjects      []runtime.Object
		skupperErrorMessage string
		current             *skupperv1alpha1.SecuredAccess
	}{
		{
			name: "no existing secureAccess",
			args: args{
				namespace: "test",
				name:      "skupper",
				spec: &skupperv1alpha1.SecuredAccessSpec{
					AccessType: ACCESS_TYPE_LOADBALANCER,
					Ports: []skupperv1alpha1.SecuredAccessPort{
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
						APIVersion: "skupper.io/v1alpha1",
						Name:       "ownerRef",
						UID:        "8a96ffdf-403b-4e4a-83a8-97d3d459adb6",
					},
				},
			},
			current: &skupperv1alpha1.SecuredAccess{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "skupper.io/v1alpha1",
					Kind:       "SecuredAccess",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "skupper",
					Namespace: "test",
					OwnerReferences: []metav1.OwnerReference{
						{
							Kind:       "SecuredAccess",
							APIVersion: "skupper.io/v1alpha1",
							Name:       "ownerRef",
							UID:        "8a96ffdf-403b-4e4a-83a8-97d3d459adb6",
						},
					},
					Annotations: map[string]string{
						"internal.skupper.io/controlled":   "true",
						"internal.skupper.io/routeraccess": "name",
					},
				},
				Spec: skupperv1alpha1.SecuredAccessSpec{
					AccessType: ACCESS_TYPE_LOCAL,
					Ports: []skupperv1alpha1.SecuredAccessPort{
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
			if len(m.services) != 0 {
				t.Errorf("SecuredAccessManager.SecuredAccessDeleted() expected no service")
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
		spec        *skupperv1alpha1.SecuredAccessSpec
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
				spec: &skupperv1alpha1.SecuredAccessSpec{
					AccessType: ACCESS_TYPE_LOADBALANCER,
					Ports: []skupperv1alpha1.SecuredAccessPort{
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
						APIVersion: "skupper.io/v1alpha1",
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
				spec: &skupperv1alpha1.SecuredAccessSpec{
					AccessType: ACCESS_TYPE_LOADBALANCER,
					Selector: map[string]string{
						"skupper.io/component": "router",
					},
					Ports: []skupperv1alpha1.SecuredAccessPort{
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
					//"internal.skupper.io/routeraccess": "name",
				},
				refs: &[]metav1.OwnerReference{
					{
						Kind:       "SecuredAccess",
						APIVersion: "skupper.io/v1alpha1",
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
				spec: &skupperv1alpha1.SecuredAccessSpec{
					AccessType: ACCESS_TYPE_LOADBALANCER,
					Selector: map[string]string{
						"skupper.io/component": "router",
					},
					Ports: []skupperv1alpha1.SecuredAccessPort{
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
						APIVersion: "skupper.io/v1alpha1",
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
					//if err = m.createService(m.definitions["test/skupper"]); err != nil {
					//	t.Errorf("SecuredAccessManager.createService error = %v", err)
					//}
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

func TestSecuredAccessManagerCheckRoute(t *testing.T) {
	type args struct {
		routeKey string
		route    *routev1.Route
	}
	testTable := []struct {
		name                string
		args                args
		k8sObjects          []runtime.Object
		skupperObjects      []runtime.Object
		skupperErrorMessage string
		accessSpec          *skupperv1alpha1.SecuredAccessSpec
		expectRoutes        int
		expectErr           bool
	}{
		{
			name: "no route",
			args: args{
				routeKey: "test",
				route:    nil,
			},
			expectRoutes: 0,
			expectErr:    false,
		},
		{
			name: "maliformed route",
			args: args{
				routeKey: "test",
				route: &routev1.Route{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "v1",
						Kind:       "Route",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "route",
					},
					Spec: routev1.RouteSpec{
						Path: "",
						Host: "1.2.3.4-8080.test.host",
						Port: &routev1.RoutePort{
							TargetPort: intstr.FromString("8080"),
						},
					},
				},
			},
			expectRoutes: 1,
			expectErr:    false,
		},
		{
			name: "no ServiceAccess found",
			args: args{
				routeKey: "test/1.2.3.4-8080",
				route: &routev1.Route{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "v1",
						Kind:       "Route",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "route",
					},
					Spec: routev1.RouteSpec{
						Port: &routev1.RoutePort{
							TargetPort: intstr.FromString("8080"),
						},
					},
				},
			},
			expectRoutes: 1,
			expectErr:    true,
		},
		{
			name: "good route",
			args: args{
				routeKey: "test/1.2.3.4-8080",
				route: &routev1.Route{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "v1",
						Kind:       "Route",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "1.2.3.4",
						Labels: map[string]string{
							"internal.skupper.io/secured-access": "true",
						},
						Annotations: map[string]string{
							"internal.skupper.io/controlled": "true",
						},
						OwnerReferences: []metav1.OwnerReference{
							{
								Kind:       "SecuredAccess",
								APIVersion: "skupper.io/v1alpha1",
								Name:       "ownerRef",
								UID:        "8a96ffdf-403b-4e4a-83a8-97d3d459adb6",
							},
						},
					},
					Spec: routev1.RouteSpec{
						Path: "",
						Host: "1.2.3.4-8080.test.domain",
						Port: &routev1.RoutePort{
							TargetPort: intstr.FromString("8080"),
						},
						To: routev1.RouteTargetReference{
							Kind: "Service",
							Name: "4.4.4.4",
						},
						TLS: &routev1.TLSConfig{
							Termination:                   routev1.TLSTerminationPassthrough,
							InsecureEdgeTerminationPolicy: routev1.InsecureEdgeTerminationPolicyNone,
						},
					},
				},
			},
			accessSpec: &skupperv1alpha1.SecuredAccessSpec{
				AccessType: ACCESS_TYPE_ROUTE,
				Ports: []skupperv1alpha1.SecuredAccessPort{
					{
						Name:       "8080",
						Port:       8080,
						TargetPort: 8081,
						Protocol:   "TCP",
					},
				},
				Certificate: "skupper",
				Issuer:      "skupper-site-ca",
				Options: map[string]string{
					"domain": "domain",
				},
			},
			expectRoutes: 2,
			expectErr:    false,
		},
	}

	for _, tt := range testTable {
		t.Run(tt.name, func(t *testing.T) {
			m, err := newSecureAccessManagerMocks("test", tt.k8sObjects, tt.skupperObjects, tt.skupperErrorMessage)
			assert.Assert(t, err)

			if tt.accessSpec != nil {
				// setup so can test update route
				if err := m.Ensure("test", "1.2.3.4", *tt.accessSpec, nil, nil); err != nil {
					t.Errorf("SecuredAccessManager.Ensure() error = %v", err)
				}
				// add route
				if err, _ := m.ensureRoute("test", tt.args.route); err != nil {
					t.Errorf("SecuredAccessManager.ensureRoute() error = %v", err)
				}
				// change parameter in route
				tt.args.route.Spec.To.Name = "4.4.4.5"
				if err, _ := m.ensureRoute("test", tt.args.route); err != nil {
					t.Errorf("SecuredAccessManager.ensureRoute() error = %v", err)
				}
			}
			if err := m.CheckRoute(tt.args.routeKey, tt.args.route); err != nil && !tt.expectErr {
				t.Errorf("SecuredAccessManager.CheckRoute() error = %v", err)
			}

			numRoutes := len(m.routes)
			if numRoutes != tt.expectRoutes {
				t.Errorf("SecuredAccessManager.CheckRoute() incorrect number of routes installed expected %d found %d",
					tt.expectRoutes, numRoutes)
			}
		})
	}
}

func TestSecuredAccessTypes(t *testing.T) {
	var test struct {
		k8sObjects          []runtime.Object
		skupperObjects      []runtime.Object
		skupperErrorMessage string
	}

	m, err := newSecureAccessManagerMocks("test", test.k8sObjects, test.skupperObjects, test.skupperErrorMessage)
	assert.Assert(t, err)

	accessType := DefaultAccessType(m.clients)
	if accessType != ACCESS_TYPE_ROUTE {
		t.Errorf("SecuredAccessType.DefaultAccessType() error")
	}

	envAccessType := GetAccessTypeFromEnv()
	if envAccessType != "" {
		t.Errorf("SecuredAccessType.GetAccessTypeFromEnv() error")
	}

	accessType = getAccessType("", m.clients)
	if accessType != ACCESS_TYPE_ROUTE {
		t.Errorf("SecuredAccessType.getAccessType() error")
	}

	accessType = getAccessType(ACCESS_TYPE_LOADBALANCER, m.clients)
	if accessType != ACCESS_TYPE_LOADBALANCER {
		t.Errorf("SecuredAccessType.getAccessType() error")
	}

	serviceType := serviceType(ACCESS_TYPE_NODEPORT)
	if serviceType != corev1.ServiceTypeNodePort {
		t.Errorf("SecuredAccessType.serviceType() error")
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
		definitions: make(map[string]*skupperv1alpha1.SecuredAccess),
		services:    make(map[string]*corev1.Service),
		routes:      make(map[string]*routev1.Route),
		ingresses:   make(map[string]*networkingv1.Ingress),
		//httpProxies: make(map[string]*unstructured.Unstructured),
		//certMgr:     certificates.CertificateManager,
	}

	return securedAccessManager, nil
}

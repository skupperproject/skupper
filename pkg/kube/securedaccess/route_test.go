package securedaccess

import (
	"testing"

	routev1 "github.com/openshift/api/route/v1"
	fakeclient "github.com/skupperproject/skupper/internal/kube/client/fake"
	skupperv1alpha1 "github.com/skupperproject/skupper/pkg/apis/skupper/v1alpha1"
	"gotest.tools/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func TestRouteAccessTypeRealise(t *testing.T) {
	type args struct {
		access *skupperv1alpha1.SecuredAccess
	}
	tests := []struct {
		name                string
		args                args
		route               *routev1.Route
		want                bool
		k8sObjects          []runtime.Object
		skupperObjects      []runtime.Object
		skupperErrorMessage string
		numRoutes           int
	}{
		{
			name: "existing route",
			args: args{
				access: &skupperv1alpha1.SecuredAccess{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "v1",
						Kind:       "Route",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "1.2.3.4",
						Namespace: "test",
					},
					Spec: skupperv1alpha1.SecuredAccessSpec{
						AccessType: ACCESS_TYPE_ROUTE,
						Selector: map[string]string{
							"skupper.io/component": "router",
						},
						Ports: []skupperv1alpha1.SecuredAccessPort{
							{
								Name:       "8080",
								Port:       8080,
								TargetPort: 8081,
								Protocol:   "TCP",
							},
						},
					},
				},
			},
			route: &routev1.Route{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "v1",
					Kind:       "Route",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "1.2.3.4-8080",
				},
				Spec: routev1.RouteSpec{
					Port: &routev1.RoutePort{
						TargetPort: intstr.FromString("8080"),
					},
				},
			},
			numRoutes: 1,
			want:      false,
		},
		{
			name: "new route",
			args: args{
				access: &skupperv1alpha1.SecuredAccess{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "v1",
						Kind:       "Route",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "1.2.3.4",
						Namespace: "test",
					},
					Spec: skupperv1alpha1.SecuredAccessSpec{
						AccessType: ACCESS_TYPE_ROUTE,
						Selector: map[string]string{
							"skupper.io/component": "router",
						},
						Ports: []skupperv1alpha1.SecuredAccessPort{
							{
								Name:       "8080",
								Port:       8080,
								TargetPort: 8081,
								Protocol:   "TCP",
							},
						},
					},
				},
			},
			route: &routev1.Route{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "v1",
					Kind:       "Route",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "1.2.3.4-9999",
				},
				Spec: routev1.RouteSpec{
					Port: &routev1.RoutePort{
						TargetPort: intstr.FromString("9999"),
					},
				},
			},
			want:      false,
			numRoutes: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, err := newRouteSecureAccessManagerMocks("test", tt.k8sObjects, tt.skupperObjects, tt.skupperErrorMessage)
			assert.Assert(t, err)
			o := &RouteAccessType{manager: m}

			// add route
			if err, _ := o.manager.ensureRoute("test", tt.route); err != nil {
				t.Errorf("SecuredAccessManager.ensureRoute() error = %v", err)
			}
			if got := o.Realise(tt.args.access); got != tt.want {
				t.Errorf("RouteAccessType.Realise() = %v, want %v", got, tt.want)
			}
			numRoutes := len(m.routes)
			if numRoutes != tt.numRoutes {
				t.Errorf("RouteAccessType incorrect number of routes installed expected %d found %d",
					tt.numRoutes, numRoutes)
			}
		})
	}
}

func TestRouteAccessTypeResolve(t *testing.T) {
	type args struct {
		access *skupperv1alpha1.SecuredAccess
	}
	tests := []struct {
		name                string
		args                args
		route               *routev1.Route
		want                bool
		k8sObjects          []runtime.Object
		skupperObjects      []runtime.Object
		skupperErrorMessage string
		numRoutes           int
	}{
		{
			name: "no route",
			args: args{
				access: &skupperv1alpha1.SecuredAccess{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "v1",
						Kind:       "Route",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "1.2.3.4",
						Namespace: "test",
					},
					Spec: skupperv1alpha1.SecuredAccessSpec{
						AccessType: ACCESS_TYPE_LOADBALANCER,
						Selector: map[string]string{
							"skupper.io/component": "router",
						},
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
					},
				},
			},
			numRoutes: 0,
			want:      false,
		},
		{
			name: "one route",
			args: args{
				access: &skupperv1alpha1.SecuredAccess{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "v1",
						Kind:       "Route",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "1.2.3.4",
						Namespace: "test",
					},
					Spec: skupperv1alpha1.SecuredAccessSpec{
						AccessType: ACCESS_TYPE_LOADBALANCER,
						Selector: map[string]string{
							"skupper.io/component": "router",
						},
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
					},
				},
			},
			route: &routev1.Route{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "v1",
					Kind:       "Route",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "1.2.3.4-8080",
				},
				Spec: routev1.RouteSpec{
					Host: "1.2.3.4-8080.test.host",
					Port: &routev1.RoutePort{
						TargetPort: intstr.FromString("9999"),
					},
				},
			},
			want:      true,
			numRoutes: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, err := newRouteSecureAccessManagerMocks("test", tt.k8sObjects, tt.skupperObjects, tt.skupperErrorMessage)
			assert.Assert(t, err)
			o := &RouteAccessType{manager: m}

			if tt.want == true {
				// add route
				if err, _ := o.manager.ensureRoute("test", tt.route); err != nil {
					t.Errorf("SecuredAccessManager.ensureRoute() error = %v", err)
				}
			}
			if got := o.Resolve(tt.args.access); got != tt.want {
				t.Errorf("RouteAccessType.Realise() = %v, want %v", got, tt.want)
			}
			numRoutes := len(m.routes)
			if numRoutes != tt.numRoutes {
				t.Errorf("RouteAccessType.Realise() incorrect number of routes installed expected %d found %d",
					tt.numRoutes, numRoutes)
			}
			// check endpoints
			if tt.want == true {
				if len(tt.args.access.Status.Endpoints) != 1 &&
					tt.args.access.Status.Endpoints[0].Host != "1.2.3.4-8080.test.host" {
					t.Errorf("RouteAccessType.Realise() failure initializing endpoints")
				}
			} else {
				if len(tt.args.access.Status.Endpoints) != 0 {
					t.Errorf("RouteAccessType.Realise() unexpected endpoints")
				}
			}
		})
	}
}

// --- helper methods

func newRouteSecureAccessManagerMocks(namespace string, k8sObjects []runtime.Object, skupperObjects []runtime.Object, fakeSkupperError string) (*SecuredAccessManager, error) {

	client, err := fakeclient.NewFakeClient(namespace, k8sObjects, skupperObjects, fakeSkupperError)
	if err != nil {
		return nil, err
	}

	securedAccessManager := &SecuredAccessManager{
		clients: client,
		routes:  make(map[string]*routev1.Route),
	}

	return securedAccessManager, nil
}

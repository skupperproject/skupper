package securedaccess

import (
	"testing"

	fakeclient "github.com/skupperproject/skupper/internal/kube/client/fake"
	skupperv1alpha1 "github.com/skupperproject/skupper/pkg/apis/skupper/v1alpha1"
	"gotest.tools/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestLoadbalancerAccessTypeResolve(t *testing.T) {
	type args struct {
		access *skupperv1alpha1.SecuredAccess
	}
	tests := []struct {
		name                string
		args                args
		svc                 *corev1.Service
		want                bool
		k8sObjects          []runtime.Object
		skupperObjects      []runtime.Object
		skupperErrorMessage string
	}{
		{
			name: "no service",
			args: args{
				access: &skupperv1alpha1.SecuredAccess{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "v1",
						Kind:       "Service",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "skupper",
						Namespace: "test",
					},
				},
			},
			want: false,
		},
		{
			name: "no endpoints",
			args: args{
				access: &skupperv1alpha1.SecuredAccess{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "v1",
						Kind:       "Service",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "skupper",
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
					},
				},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, err := newLoadBalanceSecureAccessManagerMocks("test", tt.k8sObjects, tt.skupperObjects, tt.skupperErrorMessage)
			assert.Assert(t, err)
			o := &LoadbalancerAccessType{manager: m}

			if tt.name != "no service" {
				if err = m.createService(tt.args.access); err != nil {
					t.Errorf("LoadbalancerAccessType.Resolve() failed to create service %s", err)
				}
			}
			if got := o.Resolve(tt.args.access); got != tt.want {
				t.Errorf("LoadbalancerAccessType.Resolve() = %v, want %v", got, tt.want)
			}

			if result := o.Realise(tt.args.access); result != true ||
				tt.args.access.Status.StatusMessage != skupperv1alpha1.STATUS_OK {
				t.Errorf("LoadbalancerAccessType.Realise(), error setting status")
			}
		})
	}
}

// --- helper methods
func newLoadBalanceSecureAccessManagerMocks(namespace string, k8sObjects []runtime.Object, skupperObjects []runtime.Object, fakeSkupperError string) (*SecuredAccessManager, error) {

	client, err := fakeclient.NewFakeClient(namespace, k8sObjects, skupperObjects, fakeSkupperError)
	if err != nil {
		return nil, err
	}

	securedAccessManager := &SecuredAccessManager{
		clients:  client,
		services: make(map[string]*corev1.Service),
	}

	return securedAccessManager, nil
}

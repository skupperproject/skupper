package site

import (
	"context"
	"testing"

	fakeclient "github.com/skupperproject/skupper/internal/kube/client/fake"
	"github.com/skupperproject/skupper/pkg/apis/skupper/v2alpha1"
	skupperv2alpha1 "github.com/skupperproject/skupper/pkg/apis/skupper/v2alpha1"
	"github.com/skupperproject/skupper/pkg/kube"
	"github.com/skupperproject/skupper/pkg/kube/certificates"
	"github.com/skupperproject/skupper/pkg/kube/securedaccess"
	"github.com/skupperproject/skupper/pkg/qdr"
	site1 "github.com/skupperproject/skupper/pkg/site"
	"github.com/skupperproject/skupper/pkg/version"
	"gotest.tools/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"log/slog"
)

func TestSite_Recover(t *testing.T) {
	type args struct {
		site *skupperv2alpha1.Site
	}
	tests := []struct {
		name                string
		args                args
		wantErr             bool
		k8sObjects          []runtime.Object
		skupperObjects      []runtime.Object
		skupperErrorMessage string
	}{
		{
			name: "site inactive",
			args: args{
				site: &skupperv2alpha1.Site{
					ObjectMeta: v1.ObjectMeta{
						Name:      "siteInactive",
						Namespace: "test",
						UID:       "8a96ffdf-403b-4e4a-83a8-97d3d459adb6",
					},
				},
			},
			skupperErrorMessage: "NotFound",
			wantErr:             true,
		},
		{
			name: "site fail CA",
			args: args{
				site: &skupperv2alpha1.Site{
					ObjectMeta: v1.ObjectMeta{
						Name:      "site1",
						Namespace: "test",
						UID:       "8a96ffdf-403b-4e4a-83a8-97d3d459adb6",
					},
					Spec: skupperv2alpha1.SiteSpec{
						DefaultIssuer: "skupper-spec-issuer-ca",
						LinkAccess:    "loadbalancer",
					},
					Status: skupperv2alpha1.SiteStatus{
						DefaultIssuer: "skupper-status-issuer-ca",
					},
				},
			},
			skupperObjects: []runtime.Object{
				&skupperv2alpha1.RouterAccess{
					ObjectMeta: v1.ObjectMeta{
						Name:      "Router1",
						Namespace: "test",
					},
				},
			},
			skupperErrorMessage: "NotFound",
			wantErr:             true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, err := newSiteMocks("test", tt.k8sObjects, tt.skupperObjects, tt.skupperErrorMessage, false)
			assert.Assert(t, err)

			if err := s.Recover(tt.args.site); (err != nil) != tt.wantErr {
				t.Errorf("Site.Reconcile() error = %v", err)
			}
		})
	}
}

func TestSite_checkDefaultRouterAccess(t *testing.T) {
	type args struct {
		site *skupperv2alpha1.Site
	}
	tests := []struct {
		name                string
		args                args
		rtr                 *skupperv2alpha1.RouterAccess
		wantErr             bool
		wantLinkAccess      int
		k8sObjects          []runtime.Object
		skupperObjects      []runtime.Object
		skupperErrorMessage string
	}{
		{
			name: "no link access config",
			args: args{
				site: &skupperv2alpha1.Site{
					ObjectMeta: v1.ObjectMeta{
						Name:      "site1",
						Namespace: "test",
					},
				},
			},
			wantErr:        false,
			wantLinkAccess: 0,
		},
		{
			name: "default router config",
			args: args{
				site: &skupperv2alpha1.Site{
					ObjectMeta: v1.ObjectMeta{
						Name:      "site1",
						Namespace: "test",
					},
					Spec: skupperv2alpha1.SiteSpec{
						LinkAccess: "loadbalancer",
					},
				},
			},
			wantErr:        false,
			wantLinkAccess: 1,
		},
		{
			name: "default router config exists",
			args: args{
				site: &skupperv2alpha1.Site{
					ObjectMeta: v1.ObjectMeta{
						Name:      "site1",
						Namespace: "test",
					},
					Spec: skupperv2alpha1.SiteSpec{
						LinkAccess: "loadbalancer",
					},
				},
			},
			rtr: &skupperv2alpha1.RouterAccess{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "skupper.io/v2alpha1",
					Kind:       "RouterAccess",
				},
				ObjectMeta: v1.ObjectMeta{
					Name:      "skupper-router",
					Namespace: "test",
				},
				Spec: skupperv2alpha1.RouterAccessSpec{
					AccessType: "nodeport",
				},
			},
			skupperObjects: []runtime.Object{
				&skupperv2alpha1.RouterAccess{
					ObjectMeta: v1.ObjectMeta{
						Name:      "skupper-router",
						Namespace: "test",
					},
				},
			},
			wantErr:        false,
			wantLinkAccess: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, err := newSiteMocks("test", tt.k8sObjects, tt.skupperObjects, tt.skupperErrorMessage, false)
			assert.Assert(t, err)

			if tt.rtr != nil {
				// test case when there is already a router linkAccess defined
				s.linkAccess["skupper-router"] = tt.rtr
			}

			if err = s.checkDefaultRouterAccess(context.TODO(), tt.args.site); (err != nil) != tt.wantErr {
				t.Errorf("Site.checkDefaultRouterAccess() error = %v, wantErr %v", err, tt.wantErr)
			}

			numLinkAccess := len(s.linkAccess)
			if tt.wantLinkAccess != numLinkAccess {
				t.Errorf("Site.checkDefaultRouterAccess() expected link access not found expected %d, found %d", tt.wantLinkAccess, numLinkAccess)
			} else if tt.wantLinkAccess != 0 {
				if s.linkAccess["skupper-router"].Spec.Issuer != "skupper-site-ca" {
					t.Errorf("Site.checkDefaultRouterAccess() incorrect default values found")
				}
			}
		})
	}
}

func TestSite_ExposeUnexpose(t *testing.T) {
	type args struct {
		exposed  *ExposedPortSet
		exposed2 *ExposedPortSet
	}
	tests := []struct {
		name                string
		args                args
		wantErr             bool
		k8sObjects          []runtime.Object
		skupperObjects      []runtime.Object
		skupperErrorMessage string
	}{
		{
			name: "no existing exposed ports",
			args: args{
				exposed: &ExposedPortSet{
					Host: "backend",
					Ports: map[string]Port{
						"port1": {
							Name:       "port1",
							Port:       1234,
							TargetPort: 7890,
							Protocol:   "TCP",
						},
					},
				},
				exposed2: &ExposedPortSet{
					Host: "backend",
					Ports: map[string]Port{
						"port1": {
							Name:       "port1",
							Port:       2222,
							TargetPort: 7890,
							Protocol:   "TCP",
						},
					},
				},
			},
			k8sObjects: []runtime.Object{
				&corev1.Service{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "v1",
						Kind:       "Service",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "backend",
					},
					Status: corev1.ServiceStatus{
						Conditions: []v1.Condition{
							{
								Type:   "Configured",
								Status: "True",
							},
						},
					},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, err := newSiteMocks("test", tt.k8sObjects, tt.skupperObjects, tt.skupperErrorMessage, false)
			assert.Assert(t, err)

			s.Expose(tt.args.exposed)

			// modify ports exposed
			s.Expose(tt.args.exposed2)

			s.Unexpose(tt.args.exposed.Host)

			//TBD errors are not returned and nothing stored in Site
			//if (err != nil) != tt.wantErr {
			//	t.Errorf("Site.ExposeUnexpose() error = %v, wantErr %v", err, tt.wantErr)
			//}
		})
	}
}
func TestSite_CheckListener(t *testing.T) {
	type args struct {
		name     string
		listener *skupperv2alpha1.Listener
	}
	tests := []struct {
		name                string
		args                args
		wantErr             bool
		want                string
		wantListeners       uint
		k8sObjects          []runtime.Object
		skupperObjects      []runtime.Object
		skupperErrorMessage string
	}{
		{
			name: "no site",
			want: "not initialized",
			// code just silently ignores this failure
			wantErr:       false,
			wantListeners: 0,
		},
		{
			name: "one listener added",
			args: args{
				name: "listener1",
				listener: &skupperv2alpha1.Listener{
					ObjectMeta: v1.ObjectMeta{
						Name:      "listener1",
						Namespace: "test",
						UID:       "8a96ffdf-403b-4e4a-83a8-97d3d459adb6",
					},
					Spec: skupperv2alpha1.ListenerSpec{
						RoutingKey: "backend",
						Port:       8080,
						Type:       "tcp",
						Host:       "1.2.3.4",
					},
				},
			},
			skupperObjects: []runtime.Object{
				&skupperv2alpha1.Listener{
					ObjectMeta: v1.ObjectMeta{
						Name:      "listener1",
						Namespace: "test",
					},
				},
			},
			want:          "initialized",
			wantErr:       false,
			wantListeners: 1,
		},
		/* TBD updateListenerStatus if kube command err == nil it just
		   returns and doesn't update s.bindings.UpdateListener ???
			{
				name: "listener error",
				args: args{
					name: "listener1",
					listener: &skupperv2alpha1.Listener{
						ObjectMeta: v1.ObjectMeta{
							Name:      "listener1",
							Namespace: "test",
							UID:       "8a96ffdf-403b-4e4a-83a8-97d3d459adb6",
						},
						Spec: skupperv2alpha1.ListenerSpec{
							RoutingKey: "backend",
							Port:       8080,
							Type:       "tcp",
							Host:       "1.2.3.4",
						},
					},
				},
				want:                "initialized",
				wantErr:             false,
				wantListeners:       1,
				skupperErrorMessage: "NotFound",
			},
		*/
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, err := newSiteMocks("test", tt.k8sObjects, tt.skupperObjects, tt.skupperErrorMessage, false)
			assert.Assert(t, err)

			if tt.want == "initialized" {
				s.initialised = true
				err = createRouterConfigMock(s)
				assert.Assert(t, err)
			}

			if err := s.CheckListener(tt.args.name, tt.args.listener); (err != nil) != tt.wantErr {
				t.Errorf("Site.CheckListener() error = %v", err)
			}

			// check if listener is expected and has correct values
			listener := s.bindings.bindings.GetListener(tt.args.name)
			if tt.wantListeners != 0 {
				listenerConfigured := false
				if listener == nil {
					t.Errorf("Site.CheckListener() expected listener doesn't exist")
				} else {
					if listener.Spec.Port != tt.args.listener.Spec.Port {
						t.Errorf("Site.CheckListener() expected listener doesn't have correct values")
					}
					for _, condition := range listener.Status.Conditions {
						if condition.Type == "Configured" && condition.Status == "True" {
							listenerConfigured = true
						}
					}
					if listenerConfigured == false {
						t.Errorf("Site.CheckListener() link not in expected configured state")
					}
				}
			} else if listener != nil {
				t.Errorf("Site.CheckListener() unexpected listener exists")
			}
		})
	}
}

func TestSite_CheckConnector(t *testing.T) {
	type args struct {
		name      string
		connector *skupperv2alpha1.Connector
	}
	tests := []struct {
		name                string
		args                args
		wantErr             bool
		want                string
		wantConnectors      uint
		k8sObjects          []runtime.Object
		skupperObjects      []runtime.Object
		skupperErrorMessage string
	}{
		{
			name: "no connector",
			// code just silently ignores this failure
			wantErr:        false,
			wantConnectors: 0,
		},
		{
			name: "one connector added",
			args: args{
				name: "connector1",
				connector: &skupperv2alpha1.Connector{
					ObjectMeta: v1.ObjectMeta{
						Name:      "connector1",
						Namespace: "test",
						UID:       "8a96ffdf-403b-4e4a-83a8-97d3d459adb6",
					},
					Spec: skupperv2alpha1.ConnectorSpec{
						RoutingKey: "backend",
						Port:       8080,
						Type:       "tcp",
						Host:       "1.2.3.4",
					},
				},
			},
			skupperObjects: []runtime.Object{
				&skupperv2alpha1.Connector{
					ObjectMeta: v1.ObjectMeta{
						Name:      "connector1",
						Namespace: "test",
					},
				},
			},
			want:           "initialized",
			wantErr:        false,
			wantConnectors: 1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, err := newSiteMocks("test", tt.k8sObjects, tt.skupperObjects, tt.skupperErrorMessage, false)
			assert.Assert(t, err)

			if tt.want == "initialized" {
				s.initialised = true
				err = createRouterConfigMock(s)
				assert.Assert(t, err)
			}

			if err := s.CheckConnector(tt.args.name, tt.args.connector); (err != nil) != tt.wantErr {
				t.Errorf("Site.Checkconnector() error = %v", err)
			}

			// check if connector is expected and has correct values
			connector := s.bindings.bindings.GetConnector(tt.args.name)
			if tt.wantConnectors != 0 {
				connectorConfigured := false
				if connector == nil {
					t.Errorf("Site.Checkconnector() expected connector doesn't exist")
				} else {
					if connector.Spec.Port != tt.args.connector.Spec.Port {
						t.Errorf("Site.Checkconnector() expected connector doesn't have correct values")
					}
					for _, condition := range connector.Status.Conditions {
						if condition.Type == "Configured" && condition.Status == "True" {
							connectorConfigured = true
						}
					}
					if connectorConfigured == false {
						t.Errorf("Site.CheckConnector() link not in expected configured state")
					}
				}
			} else if connector != nil {
				t.Errorf("Site.Checkconnector() unexpected connector exists")
			}
		})
	}
}

func TestSite_CheckLink(t *testing.T) {
	type args struct {
		name       string
		linkconfig *skupperv2alpha1.Link
	}
	tests := []struct {
		name                string
		args                args
		wantErr             bool
		want                string
		wantLinks           int
		k8sObjects          []runtime.Object
		skupperObjects      []runtime.Object
		skupperErrorMessage string
	}{
		{
			name: "no link",
			args: args{
				name: "link1",
			},
			wantErr:   false,
			wantLinks: 0,
		},
		{
			name: "link - site not initialized",
			args: args{
				name: "link1",
				linkconfig: &skupperv2alpha1.Link{
					ObjectMeta: v1.ObjectMeta{
						Name:      "link1",
						Namespace: "test",
						UID:       "8a96ffdf-403b-4e4a-83a8-97d3d459adb6",
					},
					Spec: skupperv2alpha1.LinkSpec{
						Cost: 1,
						Endpoints: []skupperv2alpha1.Endpoint{
							{
								Name: string(qdr.RoleInterRouter),
								Host: "10.10.10.1",
								Port: "55671",
							},
						},
					},
				},
			},
			wantErr:   false,
			wantLinks: 1,
		},
		{
			name: "link - not found",
			args: args{
				name: "link1",
				linkconfig: &skupperv2alpha1.Link{
					ObjectMeta: v1.ObjectMeta{
						Name:      "link1",
						Namespace: "test",
						UID:       "8a96ffdf-403b-4e4a-83a8-97d3d459adb6",
					},
					Spec: skupperv2alpha1.LinkSpec{
						Cost: 1,
						Endpoints: []skupperv2alpha1.Endpoint{
							{
								Name: string(qdr.RoleInterRouter),
								Host: "10.10.10.1",
								Port: "55671",
							},
						},
					},
				},
			},
			want:                "initialized",
			wantErr:             true,
			wantLinks:           1,
			skupperErrorMessage: "NotFound",
		},
		{
			name: "link - ok",
			args: args{
				name: "link1",
				linkconfig: &skupperv2alpha1.Link{
					ObjectMeta: v1.ObjectMeta{
						Name:      "link1",
						Namespace: "test",
						UID:       "8a96ffdf-403b-4e4a-83a8-97d3d459adb6",
					},
					Spec: skupperv2alpha1.LinkSpec{
						Cost: 2,
						Endpoints: []skupperv2alpha1.Endpoint{
							{
								Name: string(qdr.RoleInterRouter),
								Host: "1.1.1.1",
								Port: "55671",
							},
						},
					},
				},
			},
			skupperObjects: []runtime.Object{
				&skupperv2alpha1.Link{
					ObjectMeta: v1.ObjectMeta{
						Name:      "link1",
						Namespace: "test",
					},
				},
			},
			want:      "initialized",
			wantErr:   false,
			wantLinks: 1,
		},
		{
			name: "link - error",
			args: args{
				name: "link1",
				linkconfig: &skupperv2alpha1.Link{
					ObjectMeta: v1.ObjectMeta{
						Name:      "link1",
						Namespace: "test",
						UID:       "8a96ffdf-403b-4e4a-83a8-97d3d459adb6",
					},
					Spec: skupperv2alpha1.LinkSpec{
						Cost: 2,
						Endpoints: []skupperv2alpha1.Endpoint{
							{
								Name: string(qdr.RoleInterRouter),
								Host: "1.1.1.1",
								Port: "55671",
							},
						},
					},
				},
			},
			want:                "initialized",
			wantErr:             true,
			wantLinks:           1,
			skupperErrorMessage: "NotFound",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, err := newSiteMocks("test", tt.k8sObjects, tt.skupperObjects, tt.skupperErrorMessage, false)
			assert.Assert(t, err)

			if tt.want == "initialized" {
				s.initialised = true
				err = createRouterConfigMock(s)
				assert.Assert(t, err)
			}

			if err := s.CheckLink(tt.args.name, tt.args.linkconfig); (err != nil) != tt.wantErr {
				t.Errorf("Site.CheckLink() error = %v, wantErr %v", err, tt.skupperErrorMessage)
			}

			if tt.want == "initialized" {
				linkConfigured := false
				if len(s.links) != tt.wantLinks {
					t.Errorf("Site.CheckLink() link not added")
				}
				link := s.links[tt.args.name].Definition()
				if link.Spec.Cost != tt.args.linkconfig.Spec.Cost {
					t.Errorf("Site.CheckLink() link not configured correctly")
				}

				for _, condition := range link.Status.Conditions {
					if condition.Type == "Configured" && condition.Status == "True" {
						linkConfigured = true
					}
				}
				if linkConfigured == false {
					t.Errorf("Site.CheckLink() link not in configured state")
				}
				// test unlinking
				if err := s.unlink(tt.args.name); err != nil {
					t.Errorf("Site.unlink() link remove failure")
				}
				if len(s.links) != 0 {
					t.Errorf("Site.CheckLink() link not removed")
				}
			} else {
				// router not initialized
				numLinks := len(s.links)
				if numLinks != tt.wantLinks {
					t.Errorf("Site.CheckLink() incorrect links expected: want %d found %d", numLinks, tt.wantLinks)
				}
				// expect link status not configured
				if numLinks != 0 {
					link := s.links[tt.args.name].Definition()
					for _, condition := range link.Status.Conditions {
						if condition.Type == "Configured" && condition.Status == "True" {
							t.Errorf("Site.CheckLink() link in configured state but should not be")
						}
					}
				}
			}
		})
	}
}

func TestSite_CheckRouterAccess(t *testing.T) {
	type args struct {
		name string
		la   *skupperv2alpha1.RouterAccess
	}
	tests := []struct {
		name                string
		args                args
		wantErr             bool
		wantLinkAccess      int
		want                string
		k8sObjects          []runtime.Object
		skupperObjects      []runtime.Object
		skupperErrorMessage string
	}{
		{
			name: "no router access config",
			args: args{
				name: "skupper-router",
			},
			// code silently ignores this failure
			wantErr:        false,
			wantLinkAccess: 0,
			want:           "initialized",
		},
		{
			name: "router access config",
			args: args{
				name: "skupper-router",
				la: &skupperv2alpha1.RouterAccess{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "skupper.io/v2alpha1",
						Kind:       "RouterAccess",
					},
					ObjectMeta: v1.ObjectMeta{
						Name:      "skupper-router",
						Namespace: "test",
					},
					Spec: skupperv2alpha1.RouterAccessSpec{
						AccessType: "nodeport",
					},
				},
			},
			skupperObjects: []runtime.Object{
				&skupperv2alpha1.RouterAccess{
					ObjectMeta: v1.ObjectMeta{
						Name:      "skupper-router",
						Namespace: "test",
					},
				},
			},
			wantErr:        false,
			wantLinkAccess: 1,
			want:           "initialized",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, err := newSiteMocks("test", tt.k8sObjects, tt.skupperObjects, tt.skupperErrorMessage, true)
			assert.Assert(t, err)

			if tt.want == "initialized" {
				s.initialised = true
				err = createRouterConfigMock(s)
				assert.Assert(t, err)
			}

			if err = s.CheckRouterAccess(tt.args.name, tt.args.la); (err != nil) != tt.wantErr {
				t.Errorf("Site.CheckRouterAccess() error = %v, wantErr %v", err, tt.wantErr)
			}

			numLinkAccess := len(s.linkAccess)
			if tt.wantLinkAccess != numLinkAccess {
				t.Errorf("Site.CheckRouterAccess() expected link access not found expected %d, found %d", tt.wantLinkAccess, numLinkAccess)
			} else if tt.wantLinkAccess != 0 {
				if s.linkAccess["skupper-router"].Spec.AccessType != "nodeport" {
					t.Errorf("Site.CheckRouterAccess() incorrect values found")
				}
			}
		})
	}
}

func Test_NetworkStatusUpdate(t *testing.T) {
	type args struct {
		siteRecord []skupperv2alpha1.SiteRecord
	}
	tests := []struct {
		name                string
		args                args
		linkconfig          *skupperv2alpha1.Link
		wantErr             bool
		k8sObjects          []runtime.Object
		skupperObjects      []runtime.Object
		skupperErrorMessage string
	}{
		{
			name: "no site",
			args: args{
				siteRecord: []skupperv2alpha1.SiteRecord{
					{
						Id:        "",
						Name:      "site1",
						Namespace: "test",
						Platform:  "kubernetes",
						Version:   "1.8.0",
					},
				},
			},
			wantErr:             true,
			skupperErrorMessage: "NotFound",
		},
		{
			name: "site1",
			args: args{
				siteRecord: []skupperv2alpha1.SiteRecord{
					{
						Id:        "8a96ffdf-403b-4e4a-83a8-97d3d459adb6",
						Name:      "site1",
						Namespace: "test",
						Platform:  "podman",
						Version:   "1.8.0",
						Links: []skupperv2alpha1.LinkRecord{
							{
								Name:           "link1",
								RemoteSiteName: "east",
							},
						},
					},
				},
			},
			linkconfig: &skupperv2alpha1.Link{
				ObjectMeta: v1.ObjectMeta{
					Name:      "link1",
					Namespace: "test",
					UID:       "8a96ffdf-403b-4e4a-83a8-97d3d459adb6",
				},
				Spec: skupperv2alpha1.LinkSpec{
					Cost: 2,
					Endpoints: []skupperv2alpha1.Endpoint{
						{
							Name: string(qdr.RoleInterRouter),
							Host: "1.1.1.1",
							Port: "55671",
						},
					},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, err := newSiteMocks("test", tt.k8sObjects, tt.skupperObjects, tt.skupperErrorMessage, false)
			assert.Assert(t, err)

			// add link
			if tt.linkconfig != nil {
				link := s.newLink(tt.linkconfig)
				s.links[tt.linkconfig.ObjectMeta.Name] = link
			}

			if err := s.NetworkStatusUpdated(tt.args.siteRecord); (err != nil) != tt.wantErr {
				t.Errorf("Site.NetworkStatusUpdated() error = %v, wantErr %v", err, tt.wantErr)
			}

			if tt.wantErr == false {
				// verify network was updated
				foundConfig := false
				for _, network := range s.site.Status.Network {
					if network.Platform == "podman" && network.Version == "1.8.0" {
						foundConfig = true
					}
				}
				if foundConfig == false {
					t.Errorf("Site.NetworkStatusUpdated() network not updated")
				}
			}
		})
	}
}

func Test_CheckSecuredAccess(t *testing.T) {
	type args struct {
		sa *skupperv2alpha1.SecuredAccess
	}
	tests := []struct {
		name                string
		args                args
		rtr                 *skupperv2alpha1.RouterAccess
		wantErr             bool
		k8sObjects          []runtime.Object
		skupperObjects      []runtime.Object
		skupperErrorMessage string
	}{
		{
			name: "no linkAccess name",
			args: args{
				sa: &skupperv2alpha1.SecuredAccess{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "skupper.io/v2alpha1",
						Kind:       "SecuredAccess",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "no existing linkAccess",
			args: args{
				sa: &skupperv2alpha1.SecuredAccess{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "skupper.io/v2alpha1",
						Kind:       "SecuredAccess",
					},
					ObjectMeta: v1.ObjectMeta{
						Name:      "skupper-router",
						Namespace: "test",
						Annotations: map[string]string{
							"internal.skupper.io/controlled":   "true",
							"internal.skupper.io/routeraccess": "skupper-router",
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "linkAccess modified",
			args: args{
				sa: &skupperv2alpha1.SecuredAccess{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "skupper.io/v2alpha1",
						Kind:       "SecuredAccess",
					},
					ObjectMeta: v1.ObjectMeta{
						Name:      "skupper-router",
						Namespace: "test",
						Annotations: map[string]string{
							"internal.skupper.io/controlled":   "true",
							"internal.skupper.io/routeraccess": "skupper-router",
						},
					},
					Spec: v2alpha1.SecuredAccessSpec{
						AccessType: "loadbalancer",
					},
					Status: v2alpha1.SecuredAccessStatus{
						Endpoints: []skupperv2alpha1.Endpoint{
							{
								Name: string(qdr.RoleInterRouter),
								Host: "10.10.10.1",
								Port: "55671",
							},
						},
					},
				},
			},
			rtr: &skupperv2alpha1.RouterAccess{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "skupper.io/v2alpha1",
					Kind:       "RouterAccess",
				},
				ObjectMeta: v1.ObjectMeta{
					Name:      "skupper-router",
					Namespace: "test",
				},
				Spec: skupperv2alpha1.RouterAccessSpec{
					AccessType: "nodeport",
				},
			},
			skupperObjects: []runtime.Object{
				&skupperv2alpha1.RouterAccess{
					ObjectMeta: v1.ObjectMeta{
						Name:      "skupper-router",
						Namespace: "test",
					},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, err := newSiteMocks("test", tt.k8sObjects, tt.skupperObjects, tt.skupperErrorMessage, false)
			assert.Assert(t, err)

			if tt.rtr != nil {
				// test case when there is already a router linkAccess defined
				s.linkAccess["skupper-router"] = tt.rtr
			}
			s.CheckSecuredAccess(tt.args.sa)

			if tt.wantErr == true {
				// verify no link access
				if len(s.linkAccess) != 0 {
					t.Errorf("Site.CheckSecuredAccess() unexpected linkAccess found")
				}
			} else {
				// expected updated linkAccess
				if len(s.linkAccess) != 1 {
					t.Errorf("Site.CheckSecuredAccess() linkAccess not found")
				}
				if s.linkAccess["skupper-router"].Spec.AccessType == "loadbalancer" {
					t.Errorf("Site.CheckSecuredAccess() linkAccess not updated")
				}
				// TBD adds to code coverge but doesn't return error if failed
				if err = s.checkSecuredAccess(); err != nil {
					t.Errorf("Site.CheckSecuredAccess() linkAccess not updated")
				}
			}
		})
	}
}

// --- helper

func newSiteMocks(namespace string, k8sObjects []runtime.Object, skupperObjects []runtime.Object, fakeSkupperError string, accessMgr bool) (*Site, error) {

	site := &skupperv2alpha1.Site{
		ObjectMeta: v1.ObjectMeta{
			Name:      "site1",
			Namespace: "test",
			UID:       "8a96ffdf-403b-4e4a-83a8-97d3d459adb6",
		},
		Spec: skupperv2alpha1.SiteSpec{
			DefaultIssuer: "skupper-spec-issuer-ca",
		},
		Status: skupperv2alpha1.SiteStatus{
			DefaultIssuer: "skupper-status-issuer-ca",
		},
	}
	skupperObjects = append(skupperObjects, site)
	client, err := fakeclient.NewFakeClient(namespace, k8sObjects, skupperObjects, fakeSkupperError)
	if err != nil {
		return nil, err
	}

	controller := kube.NewController("test", client)
	newSite := &Site{
		controller: controller,
		bindings:   NewExtendedBindings(controller, ""),
		links:      make(map[string]*site1.Link),
		errors:     make(map[string]string),
		linkAccess: make(map[string]*skupperv2alpha1.RouterAccess),
		certs:      certificates.NewCertificateManager(controller),
		access:     securedaccess.NewSecuredAccessManager(client, nil, &securedaccess.Config{DefaultAccessType: "loadbalancer"}, securedaccess.ControllerContext{}),
		adaptor:    BindingAdaptor{},
		routerPods: make(map[string]*corev1.Pod),
		logger: slog.New(slog.Default().Handler()).With(
			slog.String("component", "kube.site.site"),
		),
	}

	newSite.site = site
	newSite.name = site.ObjectMeta.Name
	newSite.namespace = site.ObjectMeta.Namespace

	return newSite, nil
}

func createRouterConfigMock(s *Site) error {
	rc := qdr.InitialConfig(s.name+"-${HOSTNAME}", s.site.GetSiteId(), version.Version, s.isEdge(), 3)
	rc.AddAddress(qdr.Address{
		Prefix:       "mc",
		Distribution: "multicast",
	})
	rc.SetNormalListeners(SSL_PROFILE_PATH)

	err := s.createRouterConfig(&rc)
	if err != nil {
		return err
	}
	return nil
}

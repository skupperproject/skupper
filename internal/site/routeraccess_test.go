package site

import (
	"reflect"
	"testing"

	"github.com/skupperproject/skupper/internal/qdr"
	skupperv2alpha1 "github.com/skupperproject/skupper/pkg/apis/skupper/v2alpha1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestRouterAccessConfig_Apply(t *testing.T) {
	id := "router-1"
	siteId := "site-1"
	version := "v2.0"
	notEdge := false
	helloAge := 10

	type fields struct {
		listeners   map[string]qdr.Listener
		connectors  []qdr.Connector
		profilePath string
	}
	type args struct {
		listeners   map[string]qdr.Listener
		connectors  []qdr.Connector
		sslProfiles map[string]qdr.SslProfile
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   bool
	}{
		{
			name: "no listeners, connectors, or profiles",
			fields: fields{
				listeners:   map[string]qdr.Listener{},
				connectors:  []qdr.Connector{},
				profilePath: "",
			},
			args: args{
				listeners:   map[string]qdr.Listener{},
				connectors:  []qdr.Connector{},
				sslProfiles: map[string]qdr.SslProfile{},
			},
			want: false,
		},
		{
			name: "a listener",
			fields: fields{
				listeners: map[string]qdr.Listener{
					"listener1": qdr.Listener{
						Name:       "listener1",
						Role:       qdr.RoleNormal,
						Host:       "10.10.10.1",
						Port:       9090,
						SslProfile: "skupper",
					},
				},
				connectors:  []qdr.Connector{},
				profilePath: "/etc/skupper/skupper",
			},
			args: args{
				listeners:  map[string]qdr.Listener{},
				connectors: []qdr.Connector{},
				sslProfiles: map[string]qdr.SslProfile{
					"skupper": qdr.SslProfile{
						Name:           "skupper",
						CertFile:       "/etc/skupper/skupper/tls.crt",
						PrivateKeyFile: "/etc/skupper/skupper/tls.key",
						CaCertFile:     "/etc/skupper/skupper/ca.crt",
					},
				},
			},
			want: true,
		},
		{
			name: "the same listener",
			fields: fields{
				listeners: map[string]qdr.Listener{
					"listener1": qdr.Listener{
						Name:       "listener1",
						Role:       qdr.RoleNormal,
						Host:       "10.10.10.1",
						Port:       9090,
						SslProfile: "skupper",
					},
				},
				connectors:  []qdr.Connector{},
				profilePath: "/etc/skupper/skupper",
			},
			args: args{
				listeners: map[string]qdr.Listener{
					"listener1": qdr.Listener{
						Name:       "listener1",
						Role:       qdr.RoleNormal,
						Host:       "10.10.10.1",
						Port:       9090,
						SslProfile: "skupper",
					},
				},
				connectors: []qdr.Connector{},
				sslProfiles: map[string]qdr.SslProfile{
					"skupper": qdr.SslProfile{
						Name:           "skupper",
						CertFile:       "/etc/skupper/skupper/tls.crt",
						PrivateKeyFile: "/etc/skupper/skupper/tls.key",
						CaCertFile:     "/etc/skupper/skupper/ca.crt",
					},
				},
			},
			want: false,
		},
		{
			name: "a listener with different sslProfile",
			fields: fields{
				listeners: map[string]qdr.Listener{
					"listener1": qdr.Listener{
						Name:       "listener1",
						Role:       qdr.RoleNormal,
						Host:       "10.10.10.1",
						Port:       9090,
						SslProfile: "skupper-other",
					},
				},
				connectors:  []qdr.Connector{},
				profilePath: "/etc/skupper/skupper-other",
			},
			args: args{
				listeners:  map[string]qdr.Listener{},
				connectors: []qdr.Connector{},
				sslProfiles: map[string]qdr.SslProfile{
					"skupper": qdr.SslProfile{
						Name:           "skupper",
						CertFile:       "/etc/skupper/skupper/tls.crt",
						PrivateKeyFile: "/etc/skupper/skupper/tls.key",
						CaCertFile:     "/etc/skupper/skupper/ca.crt",
					},
				},
			},
			want: true,
		},
		{
			name: "listener deleted",
			fields: fields{
				listeners:   map[string]qdr.Listener{},
				connectors:  []qdr.Connector{},
				profilePath: "",
			},
			args: args{
				listeners: map[string]qdr.Listener{
					"listener1": qdr.Listener{
						Name:       "listener1",
						Role:       qdr.RoleInterRouter,
						Host:       "10.10.10.1",
						Port:       9090,
						SslProfile: "skupper",
					},
				},
				connectors: []qdr.Connector{},
				sslProfiles: map[string]qdr.SslProfile{
					"skupper": qdr.SslProfile{
						Name:           "skupper",
						CertFile:       "/etc/skupper/skupper/tls.crt",
						PrivateKeyFile: "/etc/skupper/skupper/tls.key",
						CaCertFile:     "/etc/skupper/skupper/ca.crt",
					},
				},
			},
			want: true,
		},
		{
			name: "a connector",
			fields: fields{
				listeners: map[string]qdr.Listener{},
				connectors: []qdr.Connector{
					{
						Name:       "connector1",
						Role:       qdr.RoleNormal,
						Host:       "10.10.10.1",
						Port:       "9090",
						SslProfile: "skupper",
					},
				},
				profilePath: "/etc/skupper/skupper",
			},
			args: args{
				listeners:  map[string]qdr.Listener{},
				connectors: []qdr.Connector{},
				sslProfiles: map[string]qdr.SslProfile{
					"skupper": qdr.SslProfile{
						Name:           "skupper",
						CertFile:       "/etc/skupper/skupper/tls.crt",
						PrivateKeyFile: "/etc/skupper/skupper/tls.key",
						CaCertFile:     "/etc/skupper/skupper/ca.crt",
					},
				},
			},
			want: true,
		},
		{
			name: "the same connector",
			fields: fields{
				listeners: map[string]qdr.Listener{},
				connectors: []qdr.Connector{
					{
						Name:       "connector1",
						Role:       qdr.RoleNormal,
						Host:       "10.10.10.1",
						Port:       "9090",
						SslProfile: "skupper",
					},
				},
				profilePath: "/etc/skupper/skupper",
			},
			args: args{
				listeners: map[string]qdr.Listener{},
				connectors: []qdr.Connector{
					{
						Name:       "connector1",
						Role:       qdr.RoleNormal,
						Host:       "10.10.10.1",
						Port:       "9090",
						SslProfile: "skupper",
					},
				},
				sslProfiles: map[string]qdr.SslProfile{
					"skupper": qdr.SslProfile{
						Name:           "skupper",
						CertFile:       "/etc/skupper/skupper/tls.crt",
						PrivateKeyFile: "/etc/skupper/skupper/tls.key",
						CaCertFile:     "/etc/skupper/skupper/ca.crt",
					},
				},
			},
			want: false,
		},
		{
			name: "a different connector",
			fields: fields{
				listeners: map[string]qdr.Listener{},
				connectors: []qdr.Connector{
					{
						Name:       "connector2",
						Role:       qdr.RoleNormal,
						Host:       "10.10.10.2",
						Port:       "9091",
						SslProfile: "skupper",
					},
				},
				profilePath: "/etc/skupper/skupper",
			},
			args: args{
				listeners: map[string]qdr.Listener{},
				connectors: []qdr.Connector{
					{
						Name:       "connector1",
						Role:       qdr.RoleNormal,
						Host:       "10.10.10.1",
						Port:       "9090",
						SslProfile: "skupper",
					},
				},
				sslProfiles: map[string]qdr.SslProfile{
					"skupper": qdr.SslProfile{
						Name:           "skupper",
						CertFile:       "/etc/skupper/skupper/tls.crt",
						PrivateKeyFile: "/etc/skupper/skupper/tls.key",
						CaCertFile:     "/etc/skupper/skupper/ca.crt",
					},
				},
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := &RouterAccessConfig{}
			g.listeners = tt.fields.listeners
			g.connectors = tt.fields.connectors
			g.profilePath = ""

			argsConfig := qdr.InitialConfig(id, siteId, version, notEdge, helloAge)
			argsConfig.Listeners = tt.args.listeners
			for _, connector := range tt.args.connectors {
				argsConfig.Connectors[connector.Name] = connector
			}
			argsConfig.SslProfiles = tt.args.sslProfiles

			if got := g.Apply(&argsConfig); got != tt.want {
				t.Errorf("RouterAccessConfig.Apply() = %v, want %v (subtest: %s)", got, tt.want, tt.name)
			}
		})
	}
}

func TestRouterAccessMap_DesiredConfig(t *testing.T) {
	type args struct {
		targetGroups []string
		profilePath  string
	}
	tests := []struct {
		name string
		m    RouterAccessMap
		args args
		want RouterAccessConfig
	}{
		{
			name: "inter-router with target",
			m: map[string]*skupperv2alpha1.RouterAccess{
				"default": &skupperv2alpha1.RouterAccess{
					ObjectMeta: v1.ObjectMeta{
						Name:      "my-ra",
						Namespace: "test",
					},
					Spec: skupperv2alpha1.RouterAccessSpec{
						AccessType: "loadbalancer",
						Roles: []skupperv2alpha1.RouterAccessRole{
							{
								Name: "inter-router",
								Port: 55671,
							},
						},
						TlsCredentials: "skupper",
						BindHost:       "10.10.10.1",
					},
				},
			},
			args: args{
				targetGroups: []string{"my-target-group"},
				profilePath:  "",
			},
			want: RouterAccessConfig{
				listeners: map[string]qdr.Listener{
					"my-ra-inter-router": qdr.Listener{
						Name:             "my-ra-inter-router",
						Role:             "inter-router",
						Host:             "10.10.10.1",
						Port:             55671,
						RouteContainer:   false,
						Http:             false,
						Cost:             0,
						SslProfile:       "skupper",
						SaslMechanisms:   "EXTERNAL",
						AuthenticatePeer: true,
					},
				},
				connectors: []qdr.Connector{
					{
						Name:       "my-target-group",
						Host:       "my-target-group",
						Role:       qdr.RoleInterRouter,
						Port:       "55671",
						SslProfile: "skupper",
						Cost:       1,
					},
				},
			},
		},
		{
			name: "inter-router sans target",
			m: map[string]*skupperv2alpha1.RouterAccess{
				"default": &skupperv2alpha1.RouterAccess{
					ObjectMeta: v1.ObjectMeta{
						Name:      "my-ra",
						Namespace: "test",
					},
					Spec: skupperv2alpha1.RouterAccessSpec{
						AccessType: "loadbalancer",
						Roles: []skupperv2alpha1.RouterAccessRole{
							{
								Name: "inter-router",
								Port: 55671,
							},
						},
						TlsCredentials: "skupper",
						BindHost:       "10.10.10.1",
					},
				},
			},
			args: args{
				targetGroups: []string{},
				profilePath:  "",
			},
			want: RouterAccessConfig{
				listeners: map[string]qdr.Listener{
					"my-ra-inter-router": qdr.Listener{
						Name:             "my-ra-inter-router",
						Role:             "inter-router",
						Host:             "10.10.10.1",
						Port:             55671,
						RouteContainer:   false,
						Http:             false,
						Cost:             0,
						SslProfile:       "skupper",
						SaslMechanisms:   "EXTERNAL",
						AuthenticatePeer: true,
					},
				},
				connectors: nil,
			},
		},
		{
			name: "edge with target",
			m: map[string]*skupperv2alpha1.RouterAccess{
				"default": &skupperv2alpha1.RouterAccess{
					ObjectMeta: v1.ObjectMeta{
						Name:      "my-ra",
						Namespace: "test",
					},
					Spec: skupperv2alpha1.RouterAccessSpec{
						AccessType: "loadbalancer",
						Roles: []skupperv2alpha1.RouterAccessRole{
							{
								Name: "edge",
								Port: 45671,
							},
						},
						TlsCredentials: "skupper",
						BindHost:       "10.10.10.1",
					},
				},
			},
			args: args{
				targetGroups: []string{"my-target-group"},
				profilePath:  "",
			},
			want: RouterAccessConfig{
				listeners: map[string]qdr.Listener{
					"my-ra-edge": qdr.Listener{
						Name:             "my-ra-edge",
						Role:             "edge",
						Host:             "10.10.10.1",
						Port:             45671,
						RouteContainer:   false,
						Http:             false,
						Cost:             0,
						SslProfile:       "skupper",
						SaslMechanisms:   "EXTERNAL",
						AuthenticatePeer: true,
					},
				},
				connectors: nil,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.m.DesiredConfig(tt.args.targetGroups, tt.args.profilePath); !reflect.DeepEqual(*got, tt.want) {
				t.Errorf("RouterAccessMap.DesiredConfig() = %v, want %v", *got, tt.want)
			}
		})
	}
}

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
				autoLinks: map[string]qdr.AutoLink{},
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
				autoLinks:  map[string]qdr.AutoLink{},
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
				autoLinks:  map[string]qdr.AutoLink{},
			},
		},
		{
			name: "inter-network sans routingKeys",
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
								Name: "inter-network",
								Port: 35671,
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
					"my-ra-inter-network": qdr.Listener{
						Name:             "my-ra-inter-network",
						Role:             "inter-network",
						Host:             "10.10.10.1",
						Port:             35671,
						RouteContainer:   false,
						Http:             false,
						Cost:             0,
						SslProfile:       "skupper",
						SaslMechanisms:   "EXTERNAL",
						AuthenticatePeer: true,
					},
				},
				connectors: nil,
				autoLinks:  map[string]qdr.AutoLink{},
			},
		},
		{
			name: "inter-network with routingKeys",
			m: map[string]*skupperv2alpha1.RouterAccess{
				"my-ra": &skupperv2alpha1.RouterAccess{
					ObjectMeta: v1.ObjectMeta{
						Name:      "my-ra",
						Namespace: "test",
					},
					Spec: skupperv2alpha1.RouterAccessSpec{
						AccessType: "loadbalancer",
						Roles: []skupperv2alpha1.RouterAccessRole{
							{
								Name: "inter-network",
								Port: 35671,
							},
						},
						TlsCredentials: "skupper",
						BindHost:       "10.10.10.1",
						RoutingKeys:    []string{"key1", "key2"},
					},
				},
			},
			args: args{
				targetGroups: []string{"my-target-group"},
				profilePath:  "",
			},
			want: RouterAccessConfig{
				listeners: map[string]qdr.Listener{
					"my-ra-inter-network": qdr.Listener{
						Name:             "my-ra-inter-network",
						Role:             "inter-network",
						Host:             "10.10.10.1",
						Port:             35671,
						RouteContainer:   false,
						Http:             false,
						Cost:             0,
						SslProfile:       "skupper",
						SaslMechanisms:   "EXTERNAL",
						AuthenticatePeer: true,
					},
				},
				connectors: nil,
				autoLinks: map[string]qdr.AutoLink{
					"routerAccess/my-ra/key1": qdr.AutoLink{
						Name:       "routerAccess/my-ra/key1",
						Address:    "key1",
						Direction:  "in",
						Connection: "my-ra-inter-network",
					},
					"routerAccess/my-ra/key2": qdr.AutoLink{
						Name:       "routerAccess/my-ra/key2",
						Address:    "key2",
						Direction:  "in",
						Connection: "my-ra-inter-network",
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.m.DesiredConfig(tt.args.targetGroups, tt.args.profilePath); !reflect.DeepEqual(*got, tt.want) {
				t.Errorf("RouterAccessMap.DesiredConfig() = %+v, want %+v", *got, tt.want)
			}
		})
	}
}

func TestRouterAccessMap_DesiredConfigWithAvailableCredentials(t *testing.T) {
	ra := &skupperv2alpha1.RouterAccess{
		ObjectMeta: v1.ObjectMeta{Name: "ra", Namespace: "ns"},
		Spec: skupperv2alpha1.RouterAccessSpec{
			TlsCredentials: "missing-secret",
			BindHost:       "0.0.0.0",
			Roles: []skupperv2alpha1.RouterAccessRole{
				{Name: "inter-router", Port: 55671},
			},
		},
	}
	m := RouterAccessMap{"ra": ra}
	allow := func(name string) bool { return name != "missing-secret" }
	got := m.DesiredConfigWithAvailableCredentials([]string{"g1"}, "/certs", allow)
	if len(got.listeners) != 0 || len(got.connectors) != 0 {
		t.Fatalf("expected empty desired when TLS secret disallowed, got listeners=%d connectors=%d",
			len(got.listeners), len(got.connectors))
	}
	got2 := m.DesiredConfigWithAvailableCredentials([]string{"g1"}, "/certs", func(string) bool { return true })
	if len(got2.listeners) == 0 {
		t.Fatal("expected listeners when TLS secret allowed")
	}
}

func TestRouterAccessMap_HasPortConflict(t *testing.T) {
	tests := []struct {
		name             string
		m                RouterAccessMap
		ra               *skupperv2alpha1.RouterAccess
		wantConflict     bool
		wantConflictName string
		wantConflictPort int
	}{
		{
			name: "empty map — no conflict",
			m:    RouterAccessMap{},
			ra: &skupperv2alpha1.RouterAccess{
				ObjectMeta: v1.ObjectMeta{Name: "new-ra"},
				Spec: skupperv2alpha1.RouterAccessSpec{
					Roles: []skupperv2alpha1.RouterAccessRole{
						{Name: "inter-router", Port: 55671},
					},
				},
			},
			wantConflict: false,
		},
		{
			name: "new ra, distinct port from existing — no conflict",
			m: RouterAccessMap{
				"existing": &skupperv2alpha1.RouterAccess{
					ObjectMeta: v1.ObjectMeta{Name: "existing-ra"},
					Spec: skupperv2alpha1.RouterAccessSpec{
						Roles: []skupperv2alpha1.RouterAccessRole{
							{Name: "inter-router", Port: 55671},
						},
					},
				},
			},
			ra: &skupperv2alpha1.RouterAccess{
				ObjectMeta: v1.ObjectMeta{Name: "new-ra"},
				Spec: skupperv2alpha1.RouterAccessSpec{
					Roles: []skupperv2alpha1.RouterAccessRole{
						{Name: "edge", Port: 45671},
					},
				},
			},
			wantConflict: false,
		},
		{
			name: "new ra, same port as existing — conflict",
			m: RouterAccessMap{
				"existing": &skupperv2alpha1.RouterAccess{
					ObjectMeta: v1.ObjectMeta{Name: "existing-ra"},
					Spec: skupperv2alpha1.RouterAccessSpec{
						Roles: []skupperv2alpha1.RouterAccessRole{
							{Name: "inter-router", Port: 55671},
						},
					},
				},
			},
			ra: &skupperv2alpha1.RouterAccess{
				ObjectMeta: v1.ObjectMeta{Name: "new-ra"},
				Spec: skupperv2alpha1.RouterAccessSpec{
					Roles: []skupperv2alpha1.RouterAccessRole{
						{Name: "inter-router", Port: 55671},
					},
				},
			},
			wantConflict:     true,
			wantConflictName: "router access: existing-ra",
			wantConflictPort: 55671,
		},
		{
			name: "ra already in map updating its own port — self, no conflict",
			m: RouterAccessMap{
				"my-ra": &skupperv2alpha1.RouterAccess{
					ObjectMeta: v1.ObjectMeta{Name: "my-ra"},
					Spec: skupperv2alpha1.RouterAccessSpec{
						Roles: []skupperv2alpha1.RouterAccessRole{
							{Name: "inter-router", Port: 55671},
						},
					},
				},
			},
			ra: &skupperv2alpha1.RouterAccess{
				ObjectMeta: v1.ObjectMeta{Name: "my-ra"},
				Spec: skupperv2alpha1.RouterAccessSpec{
					Roles: []skupperv2alpha1.RouterAccessRole{
						{Name: "inter-router", Port: 55671},
					},
				},
			},
			wantConflict: false,
		},
		{
			name: "candidate role port 0 is skipped — no conflict",
			m: RouterAccessMap{
				"existing": &skupperv2alpha1.RouterAccess{
					ObjectMeta: v1.ObjectMeta{Name: "existing-ra"},
					Spec: skupperv2alpha1.RouterAccessSpec{
						Roles: []skupperv2alpha1.RouterAccessRole{
							{Name: "inter-router", Port: 55671},
						},
					},
				},
			},
			ra: &skupperv2alpha1.RouterAccess{
				ObjectMeta: v1.ObjectMeta{Name: "new-ra"},
				Spec: skupperv2alpha1.RouterAccessSpec{
					Roles: []skupperv2alpha1.RouterAccessRole{
						{Name: "inter-router", Port: 0},
					},
				},
			},
			wantConflict: false,
		},
		{
			name: "two existing ras both with unresolved ports — port-0 key bug, no spurious conflict",
			m: RouterAccessMap{
				"first": &skupperv2alpha1.RouterAccess{
					ObjectMeta: v1.ObjectMeta{Name: "first-ra"},
					Spec: skupperv2alpha1.RouterAccessSpec{
						Roles: []skupperv2alpha1.RouterAccessRole{
							{Name: "inter-router", Port: 0},
						},
					},
				},
				"second": &skupperv2alpha1.RouterAccess{
					ObjectMeta: v1.ObjectMeta{Name: "second-ra"},
					Spec: skupperv2alpha1.RouterAccessSpec{
						Roles: []skupperv2alpha1.RouterAccessRole{
							{Name: "edge", Port: 0},
						},
					},
				},
			},
			ra: &skupperv2alpha1.RouterAccess{
				ObjectMeta: v1.ObjectMeta{Name: "new-ra"},
				Spec: skupperv2alpha1.RouterAccessSpec{
					Roles: []skupperv2alpha1.RouterAccessRole{
						{Name: "inter-router", Port: 55671},
					},
				},
			},
			wantConflict: false,
		},
		{
			name: "existing ra has allocated port in Status — candidate wants same port — conflict",
			m: RouterAccessMap{
				"existing": &skupperv2alpha1.RouterAccess{
					ObjectMeta: v1.ObjectMeta{Name: "existing-ra"},
					Spec: skupperv2alpha1.RouterAccessSpec{
						Roles: []skupperv2alpha1.RouterAccessRole{
							{Name: "inter-router", Port: 0},
						},
					},
					Status: skupperv2alpha1.RouterAccessStatus{
						Roles: []skupperv2alpha1.RouterAccessRole{
							{Name: "inter-router", Port: 55671},
						},
					},
				},
			},
			ra: &skupperv2alpha1.RouterAccess{
				ObjectMeta: v1.ObjectMeta{Name: "new-ra"},
				Spec: skupperv2alpha1.RouterAccessSpec{
					Roles: []skupperv2alpha1.RouterAccessRole{
						{Name: "inter-router", Port: 55671},
					},
				},
			},
			wantConflict:     true,
			wantConflictName: "router access: existing-ra",
			wantConflictPort: 55671,
		},
		{
			name: "multiple existing ras — conflict with second one",
			m: RouterAccessMap{
				"first": &skupperv2alpha1.RouterAccess{
					ObjectMeta: v1.ObjectMeta{Name: "first-ra"},
					Spec: skupperv2alpha1.RouterAccessSpec{
						Roles: []skupperv2alpha1.RouterAccessRole{
							{Name: "inter-router", Port: 55671},
						},
					},
				},
				"second": &skupperv2alpha1.RouterAccess{
					ObjectMeta: v1.ObjectMeta{Name: "second-ra"},
					Spec: skupperv2alpha1.RouterAccessSpec{
						Roles: []skupperv2alpha1.RouterAccessRole{
							{Name: "edge", Port: 45671},
						},
					},
				},
			},
			ra: &skupperv2alpha1.RouterAccess{
				ObjectMeta: v1.ObjectMeta{Name: "new-ra"},
				Spec: skupperv2alpha1.RouterAccessSpec{
					Roles: []skupperv2alpha1.RouterAccessRole{
						{Name: "edge", Port: 45671},
					},
				},
			},
			wantConflict:     true,
			wantConflictName: "router access: second-ra",
			wantConflictPort: 45671,
		},
		{
			name: "candidate with two roles — only second conflicts",
			m: RouterAccessMap{
				"existing": &skupperv2alpha1.RouterAccess{
					ObjectMeta: v1.ObjectMeta{Name: "existing-ra"},
					Spec: skupperv2alpha1.RouterAccessSpec{
						Roles: []skupperv2alpha1.RouterAccessRole{
							{Name: "inter-router", Port: 55671},
						},
					},
				},
			},
			ra: &skupperv2alpha1.RouterAccess{
				ObjectMeta: v1.ObjectMeta{Name: "new-ra"},
				Spec: skupperv2alpha1.RouterAccessSpec{
					Roles: []skupperv2alpha1.RouterAccessRole{
						{Name: "edge", Port: 45671},
						{Name: "inter-router", Port: 55671},
					},
				},
			},
			wantConflict:     true,
			wantConflictName: "router access: existing-ra",
			wantConflictPort: 55671,
		},
		{
			name: "candidate with two roles — neither conflicts",
			m: RouterAccessMap{
				"existing": &skupperv2alpha1.RouterAccess{
					ObjectMeta: v1.ObjectMeta{Name: "existing-ra"},
					Spec: skupperv2alpha1.RouterAccessSpec{
						Roles: []skupperv2alpha1.RouterAccessRole{
							{Name: "inter-router", Port: 55671},
						},
					},
				},
			},
			ra: &skupperv2alpha1.RouterAccess{
				ObjectMeta: v1.ObjectMeta{Name: "new-ra"},
				Spec: skupperv2alpha1.RouterAccessSpec{
					Roles: []skupperv2alpha1.RouterAccessRole{
						{Name: "edge", Port: 45671},
						{Name: "inter-network", Port: 35671},
					},
				},
			},
			wantConflict: false,
		},
		{
			name: "candidate with modified port — no conflicts",
			m: RouterAccessMap{
				"my-ra": &skupperv2alpha1.RouterAccess{
					ObjectMeta: v1.ObjectMeta{Name: "my-ra"},
					Spec: skupperv2alpha1.RouterAccessSpec{
						Roles: []skupperv2alpha1.RouterAccessRole{
							{Name: "inter-router", Port: 55671},
						},
					},
				},
			},
			ra: &skupperv2alpha1.RouterAccess{
				ObjectMeta: v1.ObjectMeta{Name: "my-ra"},
				Spec: skupperv2alpha1.RouterAccessSpec{
					Roles: []skupperv2alpha1.RouterAccessRole{
						{Name: "inter-network", Port: 55673},
					},
				},
			},
			wantConflict: false,
		},
		{
			name: "candidate conflicts when modifying port",
			m: RouterAccessMap{
				"my-ra": &skupperv2alpha1.RouterAccess{
					ObjectMeta: v1.ObjectMeta{Name: "my-ra"},
					Spec: skupperv2alpha1.RouterAccessSpec{
						Roles: []skupperv2alpha1.RouterAccessRole{
							{Name: "inter-router", Port: 55671},
						},
					},
				},
				"other-ra": &skupperv2alpha1.RouterAccess{
					ObjectMeta: v1.ObjectMeta{Name: "other-ra"},
					Spec: skupperv2alpha1.RouterAccessSpec{
						Roles: []skupperv2alpha1.RouterAccessRole{
							{Name: "inter-router", Port: 55673},
						},
					},
				},
			},
			ra: &skupperv2alpha1.RouterAccess{
				ObjectMeta: v1.ObjectMeta{Name: "my-ra"},
				Spec: skupperv2alpha1.RouterAccessSpec{
					Roles: []skupperv2alpha1.RouterAccessRole{
						{Name: "inter-network", Port: 55673},
					},
				},
			},
			wantConflict:     true,
			wantConflictName: "router access: other-ra",
			wantConflictPort: 55673,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotConflict, gotName, gotPort := tt.m.HasPortConflict(tt.ra)
			if gotConflict != tt.wantConflict {
				t.Errorf("HasPortConflict() conflict = %v, want %v", gotConflict, tt.wantConflict)
			}
			if gotName != tt.wantConflictName {
				t.Errorf("HasPortConflict() conflicting name = %q, want %q", gotName, tt.wantConflictName)
			}
			if gotPort != tt.wantConflictPort {
				t.Errorf("HasPortConflict() conflicting port = %v, want %v", gotPort, tt.wantConflictPort)
			}
		})
	}
}

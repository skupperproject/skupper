package site

import (
	"testing"

	"github.com/skupperproject/skupper/internal/qdr"
	skupperv2alpha1 "github.com/skupperproject/skupper/pkg/apis/skupper/v2alpha1"
	"gotest.tools/v3/assert"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestBindings_Apply(t *testing.T) {
	vFalse := false
	type fields struct {
		path                   string
		SiteId                 string
		connectors             []*skupperv2alpha1.Connector
		listeners              []*skupperv2alpha1.Listener
		connectorConfiguration ConnectorConfiguration
		listenerConfiguration  ListenerConfiguration
		connectorFunction      ConnectorFunction
		listenerFunction       ListenerFunction
	}
	type expected struct {
		tcpListeners  qdr.TcpEndpointMap
		tcpConnectors qdr.TcpEndpointMap
		sslProfiles   map[string]qdr.SslProfile
	}
	tests := []struct {
		name     string
		fields   fields
		config   *qdr.RouterConfig
		expected expected
	}{
		{
			name: "bind a tcp listener",
			fields: fields{
				SiteId: "site-1",
				listeners: []*skupperv2alpha1.Listener{
					&skupperv2alpha1.Listener{
						ObjectMeta: v1.ObjectMeta{
							Name:      "listener1",
							Namespace: "test",
						},
						Spec: skupperv2alpha1.ListenerSpec{
							RoutingKey: "echo:9090",
							Host:       "10.10.10.1",
							Port:       9090,
							Type:       "tcp",
						},
					},
				},
			},
			config: &qdr.RouterConfig{
				Bridges: qdr.BridgeConfig{
					TcpListeners:  map[string]qdr.TcpEndpoint{},
					TcpConnectors: map[string]qdr.TcpEndpoint{},
				},
				SslProfiles: map[string]qdr.SslProfile{},
			},
			expected: expected{
				tcpListeners: qdr.TcpEndpointMap{
					"listener1": {
						Name:    "listener1",
						Host:    "10.10.10.1",
						Port:    "9090",
						Address: "echo:9090",
						SiteId:  "site-1",
					},
				},
				tcpConnectors: qdr.TcpEndpointMap{},
				sslProfiles:   map[string]qdr.SslProfile{},
			},
		},
		{
			name: "bind a tcp connector",
			fields: fields{
				SiteId: "site-1",
				connectors: []*skupperv2alpha1.Connector{
					&skupperv2alpha1.Connector{
						ObjectMeta: v1.ObjectMeta{
							Name:      "connector1",
							Namespace: "test",
						},
						Spec: skupperv2alpha1.ConnectorSpec{
							RoutingKey: "echo:9090",
							Host:       "10.10.10.1",
							Port:       9090,
							Type:       "tcp",
						},
					},
				},
			},
			config: &qdr.RouterConfig{
				Bridges: qdr.BridgeConfig{
					TcpListeners:  map[string]qdr.TcpEndpoint{},
					TcpConnectors: map[string]qdr.TcpEndpoint{},
				},
				SslProfiles: map[string]qdr.SslProfile{},
			},
			expected: expected{
				tcpListeners: qdr.TcpEndpointMap{},
				tcpConnectors: qdr.TcpEndpointMap{
					"connector1@10.10.10.1": {
						Name:    "connector1@10.10.10.1",
						Host:    "10.10.10.1",
						Port:    "9090",
						Address: "echo:9090",
						SiteId:  "site-1",
					},
				},
				sslProfiles: map[string]qdr.SslProfile{},
			},
		},
		{
			name: "unbind a tcp listener",
			fields: fields{
				SiteId: "site-1",
			},
			config: &qdr.RouterConfig{
				Bridges: qdr.BridgeConfig{
					TcpListeners: map[string]qdr.TcpEndpoint{
						"listener1": qdr.TcpEndpoint{
							Name:   "listener1",
							Host:   "10.10.10.1",
							Port:   "9090",
							SiteId: "site-1",
						},
					},
					TcpConnectors: map[string]qdr.TcpEndpoint{},
				},
				SslProfiles: map[string]qdr.SslProfile{},
			},
			expected: expected{
				tcpListeners:  qdr.TcpEndpointMap{},
				tcpConnectors: qdr.TcpEndpointMap{},
				sslProfiles:   map[string]qdr.SslProfile{},
			},
		},
		{
			name: "unbind a tcp connector",
			fields: fields{
				SiteId: "site-1",
			},
			config: &qdr.RouterConfig{
				Bridges: qdr.BridgeConfig{
					TcpListeners: map[string]qdr.TcpEndpoint{},
					TcpConnectors: map[string]qdr.TcpEndpoint{
						"connector1": qdr.TcpEndpoint{
							Name:   "connector1",
							Host:   "10.10.10.1",
							Port:   "9090",
							SiteId: "site-1",
						},
					},
				},
				SslProfiles: map[string]qdr.SslProfile{},
			},
			expected: expected{
				tcpListeners:  qdr.TcpEndpointMap{},
				tcpConnectors: qdr.TcpEndpointMap{},
				sslProfiles:   map[string]qdr.SslProfile{},
			},
		},
		{
			name: "bind a tcp listener with ssl enabled",
			fields: fields{
				path:   "/etc/foo/",
				SiteId: "site-1",
				listeners: []*skupperv2alpha1.Listener{
					&skupperv2alpha1.Listener{
						ObjectMeta: v1.ObjectMeta{
							Name:      "listener1",
							Namespace: "test",
						},
						Spec: skupperv2alpha1.ListenerSpec{
							RoutingKey:     "echo:9090",
							Host:           "10.10.10.1",
							Port:           9090,
							Type:           "tcp",
							TlsCredentials: "my-credentials",
						},
					},
				},
			},
			config: &qdr.RouterConfig{
				Bridges: qdr.BridgeConfig{
					TcpListeners:  map[string]qdr.TcpEndpoint{},
					TcpConnectors: map[string]qdr.TcpEndpoint{},
				},
				SslProfiles: map[string]qdr.SslProfile{},
			},
			expected: expected{
				tcpListeners: qdr.TcpEndpointMap{
					"listener1": {
						Name:       "listener1",
						Host:       "10.10.10.1",
						Port:       "9090",
						Address:    "echo:9090",
						SiteId:     "site-1",
						SslProfile: "my-credentials",
					},
				},
				tcpConnectors: qdr.TcpEndpointMap{},
				sslProfiles: map[string]qdr.SslProfile{
					"my-credentials": {
						Name:           "my-credentials",
						CertFile:       "/etc/foo/my-credentials/tls.crt",
						PrivateKeyFile: "/etc/foo/my-credentials/tls.key",
						CaCertFile:     "/etc/foo/my-credentials/ca.crt",
					},
				},
			},
		},
		{
			name: "bind a tcp connector with ssl enabled",
			fields: fields{
				SiteId: "site-1",
				connectors: []*skupperv2alpha1.Connector{
					&skupperv2alpha1.Connector{
						ObjectMeta: v1.ObjectMeta{
							Name:      "connector1",
							Namespace: "test",
						},
						Spec: skupperv2alpha1.ConnectorSpec{
							RoutingKey:     "echo:9090",
							Host:           "10.10.10.1",
							Port:           9090,
							Type:           "tcp",
							TlsCredentials: "a-secret",
							UseClientCert:  true,
						},
					},
				},
			},
			config: &qdr.RouterConfig{
				Bridges: qdr.BridgeConfig{
					TcpListeners:  map[string]qdr.TcpEndpoint{},
					TcpConnectors: map[string]qdr.TcpEndpoint{},
				},
				SslProfiles: map[string]qdr.SslProfile{},
			},
			expected: expected{
				tcpListeners: qdr.TcpEndpointMap{},
				tcpConnectors: qdr.TcpEndpointMap{
					"connector1@10.10.10.1": {
						Name:           "connector1@10.10.10.1",
						Host:           "10.10.10.1",
						Port:           "9090",
						Address:        "echo:9090",
						SiteId:         "site-1",
						SslProfile:     "a-secret",
						VerifyHostname: &vFalse,
					},
				},
				sslProfiles: map[string]qdr.SslProfile{
					"a-secret": {
						Name:           "a-secret",
						CertFile:       "a-secret/tls.crt",
						PrivateKeyFile: "a-secret/tls.key",
						CaCertFile:     "a-secret/ca.crt",
					},
				},
			},
		},
		{
			name: "bind a tcp connector with ssl enabled but no client auth",
			fields: fields{
				SiteId: "site-1",
				connectors: []*skupperv2alpha1.Connector{
					&skupperv2alpha1.Connector{
						ObjectMeta: v1.ObjectMeta{
							Name:      "connector1",
							Namespace: "test",
						},
						Spec: skupperv2alpha1.ConnectorSpec{
							RoutingKey:     "echo:9090",
							Host:           "10.10.10.1",
							Port:           9090,
							Type:           "tcp",
							TlsCredentials: "a-secret",
							UseClientCert:  false,
						},
					},
				},
			},
			config: &qdr.RouterConfig{
				Bridges: qdr.BridgeConfig{
					TcpListeners:  map[string]qdr.TcpEndpoint{},
					TcpConnectors: map[string]qdr.TcpEndpoint{},
				},
				SslProfiles: map[string]qdr.SslProfile{},
			},
			expected: expected{
				tcpListeners: qdr.TcpEndpointMap{},
				tcpConnectors: qdr.TcpEndpointMap{
					"connector1@10.10.10.1": {
						Name:           "connector1@10.10.10.1",
						Host:           "10.10.10.1",
						Port:           "9090",
						Address:        "echo:9090",
						SiteId:         "site-1",
						SslProfile:     "a-secret-profile",
						VerifyHostname: &vFalse,
					},
				},
				sslProfiles: map[string]qdr.SslProfile{
					"a-secret-profile": {
						Name:       "a-secret-profile",
						CaCertFile: "a-secret-profile/ca.crt",
					},
				},
			},
		},
		{
			name: "configure connector for pods",
			fields: fields{
				SiteId: "site-1",
				connectors: []*skupperv2alpha1.Connector{
					&skupperv2alpha1.Connector{
						ObjectMeta: v1.ObjectMeta{
							Name:      "connector1",
							Namespace: "test",
						},
						Spec: skupperv2alpha1.ConnectorSpec{
							RoutingKey: "echo:9090",
							Port:       9090,
							Type:       "tcp",
						},
					},
				},
				connectorConfiguration: getPodConnectorConfiguration([]skupperv2alpha1.PodDetails{
					{
						UID: "pod1",
						IP:  "11.5.6.21",
					},
					{
						UID: "pod2",
						IP:  "11.5.6.22",
					},
				}),
			},
			config: &qdr.RouterConfig{
				Bridges: qdr.BridgeConfig{
					TcpListeners:  map[string]qdr.TcpEndpoint{},
					TcpConnectors: map[string]qdr.TcpEndpoint{},
				},
				SslProfiles: map[string]qdr.SslProfile{},
			},
			expected: expected{
				tcpListeners: qdr.TcpEndpointMap{},
				tcpConnectors: qdr.TcpEndpointMap{
					"connector1@11.5.6.21": {
						Name:      "connector1@11.5.6.21",
						Host:      "11.5.6.21",
						Port:      "9090",
						Address:   "echo:9090",
						SiteId:    "site-1",
						ProcessID: "pod1",
					},
					"connector1@11.5.6.22": {
						Name:      "connector1@11.5.6.22",
						Host:      "11.5.6.22",
						Port:      "9090",
						Address:   "echo:9090",
						SiteId:    "site-1",
						ProcessID: "pod2",
					},
				},
				sslProfiles: map[string]qdr.SslProfile{},
			},
		},
		{
			name: "listener host and port override",
			fields: fields{
				SiteId: "site-1",
				listeners: []*skupperv2alpha1.Listener{
					&skupperv2alpha1.Listener{
						ObjectMeta: v1.ObjectMeta{
							Name:      "listener1",
							Namespace: "test",
						},
						Spec: skupperv2alpha1.ListenerSpec{
							RoutingKey: "echo:9090",
							Host:       "10.10.10.1",
							Port:       9090,
							Type:       "tcp",
						},
					},
				},
				listenerConfiguration: getListenerHostPortOverride("my-host", 5678),
			},
			config: &qdr.RouterConfig{
				Bridges: qdr.BridgeConfig{
					TcpListeners:  map[string]qdr.TcpEndpoint{},
					TcpConnectors: map[string]qdr.TcpEndpoint{},
				},
				SslProfiles: map[string]qdr.SslProfile{},
			},
			expected: expected{
				tcpListeners: qdr.TcpEndpointMap{
					"listener1": {
						Name:    "listener1",
						Host:    "my-host",
						Port:    "5678",
						Address: "echo:9090",
						SiteId:  "site-1",
					},
				},
				tcpConnectors: qdr.TcpEndpointMap{},
				sslProfiles:   map[string]qdr.SslProfile{},
			},
		},
		{
			name: "mapped listener",
			fields: fields{
				SiteId: "site-1",
				listeners: []*skupperv2alpha1.Listener{
					&skupperv2alpha1.Listener{
						ObjectMeta: v1.ObjectMeta{
							Name:      "listener1",
							Namespace: "test",
						},
						Spec: skupperv2alpha1.ListenerSpec{
							RoutingKey: "echo:9090",
							Host:       "10.10.10.1",
							Port:       9090,
							Type:       "tcp",
						},
					},
				},
				listenerFunction: changeListenerRoutingKey("foo"),
			},
			config: &qdr.RouterConfig{
				Bridges: qdr.BridgeConfig{
					TcpListeners:  map[string]qdr.TcpEndpoint{},
					TcpConnectors: map[string]qdr.TcpEndpoint{},
				},
				SslProfiles: map[string]qdr.SslProfile{},
			},
			expected: expected{
				tcpListeners: qdr.TcpEndpointMap{
					"listener1": {
						Name:    "listener1",
						Host:    "10.10.10.1",
						Port:    "9090",
						Address: "foo",
						SiteId:  "site-1",
					},
				},
				tcpConnectors: qdr.TcpEndpointMap{},
				sslProfiles:   map[string]qdr.SslProfile{},
			},
		},
		{
			name: "mapped connector",
			fields: fields{
				SiteId: "site-1",
				connectors: []*skupperv2alpha1.Connector{
					&skupperv2alpha1.Connector{
						ObjectMeta: v1.ObjectMeta{
							Name:      "connector1",
							Namespace: "test",
						},
						Spec: skupperv2alpha1.ConnectorSpec{
							RoutingKey: "echo:9090",
							Host:       "10.10.10.1",
							Port:       9090,
							Type:       "tcp",
						},
					},
				},
				connectorFunction: changeConnectorRoutingKey("foo"),
			},
			config: &qdr.RouterConfig{
				Bridges: qdr.BridgeConfig{
					TcpListeners:  map[string]qdr.TcpEndpoint{},
					TcpConnectors: map[string]qdr.TcpEndpoint{},
				},
				SslProfiles: map[string]qdr.SslProfile{},
			},
			expected: expected{
				tcpListeners: qdr.TcpEndpointMap{},
				tcpConnectors: qdr.TcpEndpointMap{
					"connector1@10.10.10.1": {
						Name:    "connector1@10.10.10.1",
						Host:    "10.10.10.1",
						Port:    "9090",
						Address: "foo",
						SiteId:  "site-1",
					},
				},
				sslProfiles: map[string]qdr.SslProfile{},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := NewBindings(tt.fields.path)
			b.SetSiteId(tt.fields.SiteId)
			if tt.fields.listenerConfiguration != nil {
				b.SetListenerConfiguration(tt.fields.listenerConfiguration)
			}
			if tt.fields.connectorConfiguration != nil {
				b.SetConnectorConfiguration(tt.fields.connectorConfiguration)
			}
			for _, listener := range tt.fields.listeners {
				b.UpdateListener(listener.Name, listener)
			}
			for _, connector := range tt.fields.connectors {
				b.UpdateConnector(connector.Name, connector)
			}
			if tt.fields.connectorFunction != nil || tt.fields.listenerFunction != nil {
				b.Map(tt.fields.connectorFunction, tt.fields.listenerFunction)
			}

			b.Apply(tt.config)
			assert.DeepEqual(t, tt.config.Bridges.TcpListeners, tt.expected.tcpListeners)
			assert.DeepEqual(t, tt.config.Bridges.TcpConnectors, tt.expected.tcpConnectors)
			assert.DeepEqual(t, tt.config.SslProfiles, tt.expected.sslProfiles)
		})
	}
}

func changeConnectorRoutingKey(key string) ConnectorFunction {
	return func(input *skupperv2alpha1.Connector) *skupperv2alpha1.Connector {
		copy := *input
		copy.Spec.RoutingKey = key
		return &copy
	}
}

func changeListenerRoutingKey(key string) ListenerFunction {
	return func(input *skupperv2alpha1.Listener) *skupperv2alpha1.Listener {
		copy := *input
		copy.Spec.RoutingKey = key
		return &copy
	}
}

func getListenerHostPortOverride(host string, port int) ListenerConfiguration {
	return func(siteId string, listener *skupperv2alpha1.Listener, config *qdr.BridgeConfig) {
		UpdateBridgeConfigForListenerWithHostAndPort(siteId, listener, host, port, config)
	}
}

func getPodConnectorConfiguration(pods []skupperv2alpha1.PodDetails) ConnectorConfiguration {
	return func(siteId string, connector *skupperv2alpha1.Connector, config *qdr.BridgeConfig) {
		for _, pod := range pods {
			UpdateBridgeConfigForConnectorToPod(siteId, connector, pod, false, config)
		}
	}
}

func TestBindings_UpdateListener(t *testing.T) {
	type fields struct {
		SiteId     string
		connectors map[string]*skupperv2alpha1.Connector
		listeners  map[string]*skupperv2alpha1.Listener
	}
	type args struct {
		name     string
		listener *skupperv2alpha1.Listener
	}
	type expected struct {
		listener  events
		connector events
	}
	tests := []struct {
		name     string
		fields   fields
		args     args
		expected expected
	}{
		{
			name: "add listener",
			fields: fields{
				SiteId:     "site-1",
				connectors: make(map[string]*skupperv2alpha1.Connector),
				listeners:  make(map[string]*skupperv2alpha1.Listener),
			},
			args: args{
				name: "listener1",
				listener: &skupperv2alpha1.Listener{
					ObjectMeta: v1.ObjectMeta{
						Name:      "listener1",
						Namespace: "test",
					},
					Spec: skupperv2alpha1.ListenerSpec{
						RoutingKey: "echo:9090",
						Host:       "10.10.10.1",
						Port:       9090,
						Type:       "tcp",
					},
				},
			},
			expected: expected{
				listener: events{
					Updated: []string{"listener1"},
				},
			},
		},
		{
			name: "delete listener",
			fields: fields{
				SiteId:     "site-1",
				connectors: make(map[string]*skupperv2alpha1.Connector),
				listeners: map[string]*skupperv2alpha1.Listener{
					"listener1": &skupperv2alpha1.Listener{
						ObjectMeta: v1.ObjectMeta{
							Name:      "listener1",
							Namespace: "test",
						},
						Spec: skupperv2alpha1.ListenerSpec{
							RoutingKey: "echo:9090",
							Host:       "10.10.10.1",
							Port:       9090,
							Type:       "tcp",
						},
					},
				},
			},
			args: args{
				name:     "listener1",
				listener: nil,
			},
			expected: expected{
				listener: events{
					Updated: []string{"listener1"},
					Deleted: []string{"listener1"},
				},
			},
		},
		{
			name: "delete listener that is not there",
			fields: fields{
				SiteId:     "site-1",
				connectors: make(map[string]*skupperv2alpha1.Connector),
				listeners:  make(map[string]*skupperv2alpha1.Listener),
			},
			args: args{
				name:     "listener1",
				listener: nil,
			},
		},
		{
			name: "update listener",
			fields: fields{
				SiteId:     "site-1",
				connectors: make(map[string]*skupperv2alpha1.Connector),
				listeners: map[string]*skupperv2alpha1.Listener{
					"listener1": &skupperv2alpha1.Listener{
						ObjectMeta: v1.ObjectMeta{
							Name:      "listener1",
							Namespace: "test",
						},
						Spec: skupperv2alpha1.ListenerSpec{
							RoutingKey: "echo:9090",
							Host:       "10.10.10.1",
							Port:       9090,
							Type:       "tcp",
						},
					},
				},
			},
			args: args{
				name: "listener1",
				listener: &skupperv2alpha1.Listener{
					ObjectMeta: v1.ObjectMeta{
						Name:      "listener1",
						Namespace: "test",
					},
					Spec: skupperv2alpha1.ListenerSpec{
						RoutingKey: "echo:9090",
						Host:       "10.10.10.1",
						Port:       9999,
						Type:       "tcp",
					},
				},
			},
			expected: expected{
				listener: events{
					Updated: []string{"listener1", "listener1"},
				},
			},
		},
		{
			name: "unchanged listener",
			fields: fields{
				SiteId:     "site-1",
				connectors: make(map[string]*skupperv2alpha1.Connector),
				listeners: map[string]*skupperv2alpha1.Listener{
					"listener1": &skupperv2alpha1.Listener{
						ObjectMeta: v1.ObjectMeta{
							Name:      "listener1",
							Namespace: "test",
						},
						Spec: skupperv2alpha1.ListenerSpec{
							RoutingKey: "echo:9090",
							Host:       "10.10.10.1",
							Port:       9090,
							Type:       "tcp",
						},
					},
				},
			},
			args: args{
				name: "listener1",
				listener: &skupperv2alpha1.Listener{
					ObjectMeta: v1.ObjectMeta{
						Name:      "listener1",
						Namespace: "test",
					},
					Spec: skupperv2alpha1.ListenerSpec{
						RoutingKey: "echo:9090",
						Host:       "10.10.10.1",
						Port:       9090,
						Type:       "tcp",
					},
				},
			},
			expected: expected{
				listener: events{
					Updated: []string{"listener1"},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := NewBindings("")
			b.SetSiteId(tt.fields.SiteId)
			for _, listener := range tt.fields.listeners {
				b.UpdateListener(listener.Name, listener)
			}
			for _, connector := range tt.fields.connectors {
				b.UpdateConnector(connector.Name, connector)
			}
			handler := &TestBindingEventHandler{}
			b.SetBindingEventHandler(handler)

			b.UpdateListener(tt.args.name, tt.args.listener)
			assert.Equal(t, b.GetListener(tt.args.name), tt.args.listener)
			assert.DeepEqual(t, handler.listener, tt.expected.listener)
			assert.DeepEqual(t, handler.connector, tt.expected.connector)
		})
	}
}

func TestBindings_UpdateConnector(t *testing.T) {
	type fields struct {
		SiteId     string
		connectors map[string]*skupperv2alpha1.Connector
		listeners  map[string]*skupperv2alpha1.Listener
		configure  struct {
			listener  ListenerConfiguration
			connector ConnectorConfiguration
		}
	}
	type args struct {
		name      string
		connector *skupperv2alpha1.Connector
	}
	type expected struct {
		listener  events
		connector events
	}
	tests := []struct {
		name     string
		fields   fields
		args     args
		expected expected
	}{
		{
			name: "add connector",
			fields: fields{
				SiteId:     "site-1",
				connectors: make(map[string]*skupperv2alpha1.Connector),
				listeners:  make(map[string]*skupperv2alpha1.Listener),
			},
			args: args{
				name: "connector1",
				connector: &skupperv2alpha1.Connector{
					ObjectMeta: v1.ObjectMeta{
						Name:      "connector1",
						Namespace: "test",
					},
					Spec: skupperv2alpha1.ConnectorSpec{
						RoutingKey: "echo:9090",
						Host:       "10.10.10.1",
						Port:       9090,
						Type:       "tcp",
					},
				},
			},
			expected: expected{
				connector: events{
					Updated: []string{"connector1"},
				},
			},
		},
		{
			name: "delete connector",
			fields: fields{
				SiteId: "site-1",
				connectors: map[string]*skupperv2alpha1.Connector{
					"connector1": &skupperv2alpha1.Connector{
						ObjectMeta: v1.ObjectMeta{
							Name:      "connector1",
							Namespace: "test",
						},
						Spec: skupperv2alpha1.ConnectorSpec{
							RoutingKey: "echo:9090",
							Host:       "10.10.10.1",
							Port:       9090,
							Type:       "tcp",
						},
					},
				},
				listeners: make(map[string]*skupperv2alpha1.Listener),
			},
			args: args{
				name:      "connector1",
				connector: nil,
			},
			expected: expected{
				connector: events{
					Updated: []string{"connector1"},
					Deleted: []string{"connector1"},
				},
			},
		},
		{
			name: "delete connector that is not there",
			fields: fields{
				SiteId:     "site-1",
				connectors: make(map[string]*skupperv2alpha1.Connector),
				listeners:  make(map[string]*skupperv2alpha1.Listener),
			},
			args: args{
				name:      "connector1",
				connector: nil,
			},
		},
		{
			name: "update connector",
			fields: fields{
				SiteId: "site-1",
				connectors: map[string]*skupperv2alpha1.Connector{
					"connector1": &skupperv2alpha1.Connector{
						ObjectMeta: v1.ObjectMeta{
							Name:      "connector1",
							Namespace: "test",
						},
						Spec: skupperv2alpha1.ConnectorSpec{
							RoutingKey: "echo:9090",
							Host:       "10.10.10.1",
							Port:       9090,
							Type:       "tcp",
						},
					},
				},
				listeners: make(map[string]*skupperv2alpha1.Listener),
			},
			args: args{
				name: "connector1",
				connector: &skupperv2alpha1.Connector{
					ObjectMeta: v1.ObjectMeta{
						Name:      "connector1",
						Namespace: "test",
					},
					Spec: skupperv2alpha1.ConnectorSpec{
						RoutingKey: "echo:9090",
						Host:       "10.10.10.1",
						Port:       9999,
						Type:       "tcp",
					},
				},
			},
			expected: expected{
				connector: events{
					Updated: []string{"connector1", "connector1"},
				},
			},
		},
		{
			name: "add connector that is already there",
			fields: fields{
				SiteId: "site-1",
				connectors: map[string]*skupperv2alpha1.Connector{
					"connector1": &skupperv2alpha1.Connector{
						ObjectMeta: v1.ObjectMeta{
							Name:      "connector1",
							Namespace: "test",
						},
						Spec: skupperv2alpha1.ConnectorSpec{
							RoutingKey: "echo:9090",
							Host:       "10.10.10.1",
							Port:       9090,
							Type:       "tcp",
						},
					},
				},
				listeners: make(map[string]*skupperv2alpha1.Listener),
			},
			args: args{
				name: "connector1",
				connector: &skupperv2alpha1.Connector{
					ObjectMeta: v1.ObjectMeta{
						Name:      "connector1",
						Namespace: "test",
					},
					Spec: skupperv2alpha1.ConnectorSpec{
						RoutingKey: "echo:9090",
						Host:       "10.10.10.1",
						Port:       9090,
						Type:       "tcp",
					},
				},
			},
			expected: expected{
				connector: events{
					Updated: []string{"connector1"},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := NewBindings("")
			b.SetSiteId(tt.fields.SiteId)
			for _, listener := range tt.fields.listeners {
				b.UpdateListener(listener.Name, listener)
			}
			for _, connector := range tt.fields.connectors {
				b.UpdateConnector(connector.Name, connector)
			}
			handler := &TestBindingEventHandler{}
			b.SetBindingEventHandler(handler)

			b.UpdateConnector(tt.args.name, tt.args.connector)
			assert.Equal(t, b.GetConnector(tt.args.name), tt.args.connector)
			assert.DeepEqual(t, handler.listener, tt.expected.listener)
			assert.DeepEqual(t, handler.connector, tt.expected.connector)
		})
	}
}

type events struct {
	Updated []string
	Deleted []string
}

func (e *events) updated(name string) {
	e.Updated = append(e.Updated, name)
}

func (e *events) deleted(name string) {
	e.Deleted = append(e.Deleted, name)
}

type TestBindingEventHandler struct {
	listener  events
	connector events
}

func (h *TestBindingEventHandler) ListenerUpdated(listener *skupperv2alpha1.Listener) {
	h.listener.updated(listener.Name)
}

func (h *TestBindingEventHandler) ListenerDeleted(listener *skupperv2alpha1.Listener) {
	h.listener.deleted(listener.Name)
}

func (h *TestBindingEventHandler) ConnectorUpdated(connector *skupperv2alpha1.Connector) bool {
	h.connector.updated(connector.Name)
	return false
}

func (h *TestBindingEventHandler) ConnectorDeleted(connector *skupperv2alpha1.Connector) {
	h.connector.deleted(connector.Name)
}

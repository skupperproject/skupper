package site

import (
	"testing"

	skupperv1alpha1 "github.com/skupperproject/skupper/pkg/apis/skupper/v1alpha1"
	"github.com/skupperproject/skupper/pkg/qdr"
	"gotest.tools/assert"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestBindings_Apply(t *testing.T) {
	type fields struct {
		SiteId     string
		connectors map[string]*skupperv1alpha1.Connector
		listeners  map[string]*skupperv1alpha1.Listener
	}
	type args struct {
		tcpListeners  map[string]qdr.TcpEndpoint
		tcpConnectors map[string]qdr.TcpEndpoint
		sslProfiles   map[string]qdr.SslProfile
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   bool
	}{
		{
			name: "bind a tcp listener",
			fields: fields{
				SiteId:     "site-1",
				connectors: make(map[string]*skupperv1alpha1.Connector),
				listeners: map[string]*skupperv1alpha1.Listener{
					"listener1": &skupperv1alpha1.Listener{
						ObjectMeta: v1.ObjectMeta{
							Name:      "listener1",
							Namespace: "test",
						},
						Spec: skupperv1alpha1.ListenerSpec{
							RoutingKey: "echo:9090",
							Host:       "10.10.10.1",
							Port:       9090,
							Type:       "tcp",
						},
					},
				},
			},
			args: args{
				tcpListeners:  map[string]qdr.TcpEndpoint{},
				tcpConnectors: map[string]qdr.TcpEndpoint{},
				sslProfiles:   map[string]qdr.SslProfile{},
			},
			want: true,
		},
		{
			name: "bind a tcp connector",
			fields: fields{
				SiteId: "site-1",
				connectors: map[string]*skupperv1alpha1.Connector{
					"connector1": &skupperv1alpha1.Connector{
						ObjectMeta: v1.ObjectMeta{
							Name:      "listener1",
							Namespace: "test",
						},
						Spec: skupperv1alpha1.ConnectorSpec{
							RoutingKey: "echo:9090",
							Host:       "10.10.10.1",
							Port:       9090,
							Type:       "tcp",
						},
					},
				},
				listeners: make(map[string]*skupperv1alpha1.Listener),
			},
			args: args{
				tcpListeners:  map[string]qdr.TcpEndpoint{},
				tcpConnectors: map[string]qdr.TcpEndpoint{},
				sslProfiles:   map[string]qdr.SslProfile{},
			},
			want: true,
		},
		{
			name: "unbind a tcp listener",
			fields: fields{
				SiteId:     "site-1",
				connectors: make(map[string]*skupperv1alpha1.Connector),
				listeners:  make(map[string]*skupperv1alpha1.Listener),
			},
			args: args{
				tcpListeners: map[string]qdr.TcpEndpoint{
					"listener1": qdr.TcpEndpoint{
						Name:   "listener1",
						Host:   "10.10.10.1",
						Port:   "9090",
						SiteId: "site-1",
					},
				},
				tcpConnectors: map[string]qdr.TcpEndpoint{},
				sslProfiles:   map[string]qdr.SslProfile{},
			},
			want: true,
		},
		{
			name: "unbind a tcp connector",
			fields: fields{
				SiteId:     "site-1",
				connectors: make(map[string]*skupperv1alpha1.Connector),
				listeners:  make(map[string]*skupperv1alpha1.Listener),
			},
			args: args{
				tcpListeners: map[string]qdr.TcpEndpoint{},
				tcpConnectors: map[string]qdr.TcpEndpoint{
					"connector1": qdr.TcpEndpoint{
						Name:   "connector1",
						Host:   "10.10.10.1",
						Port:   "9090",
						SiteId: "site-1",
					},
				},
				sslProfiles: map[string]qdr.SslProfile{},
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := NewBindings()
			b.SiteId = tt.fields.SiteId
			b.listeners = tt.fields.listeners
			b.connectors = tt.fields.connectors
			// redundant for coverage
			b.SetListenerConfiguration(b.configure.listener)
			b.SetConnectorConfiguration(b.configure.connector)

			argsConfig := qdr.InitialConfig("some-id", tt.fields.SiteId, "v2.0", false, 10)
			argsConfig.Bridges.TcpListeners = tt.args.tcpListeners
			argsConfig.Bridges.TcpConnectors = tt.args.tcpConnectors
			argsConfig.SslProfiles = tt.args.sslProfiles

			if got := b.Apply(&argsConfig); got != tt.want {
				t.Errorf("Bindings.Apply() = %v, want %v", got, tt.want)
			}
			assert.Assert(t, len(argsConfig.Bridges.TcpListeners) == len(b.listeners))
			assert.Assert(t, len(argsConfig.Bridges.TcpConnectors) == len(b.connectors))
		})
	}
}

func TestBindings_UpdateListener(t *testing.T) {
	type fields struct {
		SiteId     string
		connectors map[string]*skupperv1alpha1.Connector
		listeners  map[string]*skupperv1alpha1.Listener
		handler    BindingEventHandler
	}
	type args struct {
		name     string
		listener *skupperv1alpha1.Listener
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    qdr.ConfigUpdate
		wantErr bool
	}{
		{
			name: "add listener",
			fields: fields{
				SiteId:     "site-1",
				connectors: make(map[string]*skupperv1alpha1.Connector),
				listeners:  make(map[string]*skupperv1alpha1.Listener),
			},
			args: args{
				name: "listener1",
				listener: &skupperv1alpha1.Listener{
					ObjectMeta: v1.ObjectMeta{
						Name:      "listener1",
						Namespace: "test",
					},
					Spec: skupperv1alpha1.ListenerSpec{
						RoutingKey: "echo:9090",
						Host:       "10.10.10.1",
						Port:       9090,
						Type:       "tcp",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "delete listener",
			fields: fields{
				SiteId:     "site-1",
				connectors: make(map[string]*skupperv1alpha1.Connector),
				listeners: map[string]*skupperv1alpha1.Listener{
					"listener1": &skupperv1alpha1.Listener{
						ObjectMeta: v1.ObjectMeta{
							Name:      "listener1",
							Namespace: "test",
						},
						Spec: skupperv1alpha1.ListenerSpec{
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
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := NewBindings()
			b.SiteId = tt.fields.SiteId
			b.listeners = tt.fields.listeners
			b.connectors = tt.fields.connectors

			_, err := b.UpdateListener(tt.args.name, tt.args.listener)
			if (err != nil) != tt.wantErr {
				t.Errorf("Bindings.UpdateListener() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}

func TestBindings_UpdateConnector(t *testing.T) {
	type fields struct {
		SiteId     string
		connectors map[string]*skupperv1alpha1.Connector
		listeners  map[string]*skupperv1alpha1.Listener
		handler    BindingEventHandler
		configure  struct {
			listener  ListenerConfiguration
			connector ConnectorConfiguration
		}
	}
	type args struct {
		name      string
		connector *skupperv1alpha1.Connector
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    qdr.ConfigUpdate
		wantErr bool
	}{
		{
			name: "add connector",
			fields: fields{
				SiteId:     "site-1",
				connectors: make(map[string]*skupperv1alpha1.Connector),
				listeners:  make(map[string]*skupperv1alpha1.Listener),
			},
			args: args{
				name: "connector1",
				connector: &skupperv1alpha1.Connector{
					ObjectMeta: v1.ObjectMeta{
						Name:      "connector1",
						Namespace: "test",
					},
					Spec: skupperv1alpha1.ConnectorSpec{
						RoutingKey: "echo:9090",
						Host:       "10.10.10.1",
						Port:       9090,
						Type:       "tcp",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "delete connector",
			fields: fields{
				SiteId: "site-1",
				connectors: map[string]*skupperv1alpha1.Connector{
					"connector1": &skupperv1alpha1.Connector{
						ObjectMeta: v1.ObjectMeta{
							Name:      "connector1",
							Namespace: "test",
						},
						Spec: skupperv1alpha1.ConnectorSpec{
							RoutingKey: "echo:9090",
							Host:       "10.10.10.1",
							Port:       9090,
							Type:       "tcp",
						},
					},
				},
				listeners: make(map[string]*skupperv1alpha1.Listener),
			},
			args: args{
				name:      "connector1",
				connector: nil,
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := NewBindings()
			b.SiteId = tt.fields.SiteId
			b.listeners = tt.fields.listeners
			b.connectors = tt.fields.connectors

			_, err := b.UpdateConnector(tt.args.name, tt.args.connector)
			if (err != nil) != tt.wantErr {
				t.Errorf("Bindings.UpdateConnector() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}

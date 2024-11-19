package site

import (
	"testing"

	skupperv2alpha1 "github.com/skupperproject/skupper/pkg/apis/skupper/v2alpha1"
	"github.com/skupperproject/skupper/pkg/qdr"
	"gotest.tools/assert"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type MockBindingContext struct {
	selectors     map[string]TargetSelection
	exposed       ExposedPorts
	unexposedHost string
}

func NewMockBindingContext(selectors map[string]TargetSelection) *MockBindingContext {
	return &MockBindingContext{
		selectors: selectors,
		exposed:   map[string]*ExposedPortSet{},
	}
}

func (m *MockBindingContext) Select(connector *skupperv2alpha1.Connector) TargetSelection {
	if selector, ok := m.selectors[connector.Name]; ok {
		return selector
	}
	return nil
}

func (m *MockBindingContext) Expose(ports *ExposedPortSet) {
	portsCopy := *ports
	m.exposed[ports.Host] = &portsCopy
}

func (m *MockBindingContext) Unexpose(host string) {
	m.unexposedHost = host
}

type MockTargetSelection struct {
	selector   string
	podDetails []skupperv2alpha1.PodDetails
}

func NewMockTargetSelection(selector string, podDetails []skupperv2alpha1.PodDetails) *MockTargetSelection {
	return &MockTargetSelection{
		selector:   selector,
		podDetails: podDetails,
	}
}

func (m *MockTargetSelection) Selector() string {
	return m.selector
}

func (m *MockTargetSelection) Close() {
}

func (m *MockTargetSelection) List() []skupperv2alpha1.PodDetails {
	return m.podDetails
}

func TestBindingAdaptor_ConnectorUpdated(t *testing.T) {
	type fields struct {
		context   BindingContext
		mapping   *qdr.PortMapping
		exposed   ExposedPorts
		selectors map[string]TargetSelection
	}
	type args struct {
		connector *skupperv2alpha1.Connector
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   bool
	}{
		{
			name: "connector selector not populated, no matching pods",
			args: args{
				connector: &skupperv2alpha1.Connector{
					ObjectMeta: v1.ObjectMeta{
						Name:      "backend",
						Namespace: "test",
						UID:       "8a96ffdf-403b-4e4a-83a8-97d3d459adb6",
					},
				},
			},
			want: true,
		},
		{
			name: "connector selector populated, no matching pods",
			fields: fields{
				context:   NewMockBindingContext(nil),
				selectors: map[string]TargetSelection{},
			},
			args: args{
				connector: &skupperv2alpha1.Connector{
					ObjectMeta: v1.ObjectMeta{
						Name:      "backend",
						Namespace: "test",
						UID:       "8a96ffdf-403b-4e4a-83a8-97d3d459adb6",
					},
					Spec: skupperv2alpha1.ConnectorSpec{
						Selector: "app=backend",
					},
				},
			},
			want: false,
		},
		{
			name: "connector selector populated, pods match selector",
			fields: fields{
				context: NewMockBindingContext(map[string]TargetSelection{
					"backend": &TargetSelectionImpl{
						selector: "app=backend",
					},
				}),
				selectors: map[string]TargetSelection{
					"backend": &TargetSelectionImpl{
						selector: "app=backend",
					},
				},
			},
			args: args{
				connector: &skupperv2alpha1.Connector{
					ObjectMeta: v1.ObjectMeta{
						Name:      "backend",
						Namespace: "test",
						UID:       "8a96ffdf-403b-4e4a-83a8-97d3d459adb6",
					},
					Spec: skupperv2alpha1.ConnectorSpec{
						Selector: "app=backend",
					},
				},
			},
			want: true,
		},
		{
			name: "connector selector changed to empty",
			fields: fields{
				context: NewMockBindingContext(map[string]TargetSelection{
					"backend": &TargetSelectionImpl{
						selector: "app=backend",
					},
				}),
				selectors: map[string]TargetSelection{
					"backend": NewMockTargetSelection("app=backend", []skupperv2alpha1.PodDetails{}),
				},
			},
			args: args{
				connector: &skupperv2alpha1.Connector{
					ObjectMeta: v1.ObjectMeta{
						Name:      "backend",
						Namespace: "test",
						UID:       "8a96ffdf-403b-4e4a-83a8-97d3d459adb6",
					},
					Spec: skupperv2alpha1.ConnectorSpec{},
				},
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &BindingAdaptor{
				context:   tt.fields.context,
				mapping:   tt.fields.mapping,
				exposed:   tt.fields.exposed,
				selectors: tt.fields.selectors,
			}
			if got := a.ConnectorUpdated(tt.args.connector); got != tt.want {
				t.Errorf("BindingAdaptor.ConnectorUpdated() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBindingAdaptor_ConnectorDeleted(t *testing.T) {
	type fields struct {
		context   BindingContext
		mapping   *qdr.PortMapping
		exposed   ExposedPorts
		selectors map[string]TargetSelection
	}
	type args struct {
		connector *skupperv2alpha1.Connector
	}
	type expected struct {
		selectors map[string]TargetSelection
	}
	tests := []struct {
		name     string
		fields   fields
		args     args
		expected expected
	}{
		{
			name: "connector deleted",
			fields: fields{
				context: NewMockBindingContext(map[string]TargetSelection{}),
				selectors: map[string]TargetSelection{
					"backend": NewMockTargetSelection("app=backend", []skupperv2alpha1.PodDetails{}),
				},
			},
			args: args{
				connector: &skupperv2alpha1.Connector{
					ObjectMeta: v1.ObjectMeta{
						Name:      "backend",
						Namespace: "test",
						UID:       "8a96ffdf-403b-4e4a-83a8-97d3d459adb6",
					},
				},
			},
			expected: expected{
				// expected behavior: ConnectorDeleted deletes
				// "backend" from selectors map
				selectors: map[string]TargetSelection{},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &BindingAdaptor{
				context:   tt.fields.context,
				mapping:   tt.fields.mapping,
				exposed:   tt.fields.exposed,
				selectors: tt.fields.selectors,
			}
			a.ConnectorDeleted(tt.args.connector)

			assert.DeepEqual(t, a.selectors, tt.expected.selectors)
		})
	}
}

func TestBindingAdaptor_updateBridgeConfigForConnector(t *testing.T) {
	type fields struct {
		context   BindingContext
		mapping   *qdr.PortMapping
		exposed   ExposedPorts
		selectors map[string]TargetSelection
	}
	type args struct {
		siteId    string
		connector *skupperv2alpha1.Connector
		config    qdr.BridgeConfig
	}
	type expected struct {
		config qdr.BridgeConfig
	}
	tests := []struct {
		name     string
		fields   fields
		args     args
		expected expected
	}{
		{
			name: "connector spec host populated",
			fields: fields{
				context:   NewMockBindingContext(map[string]TargetSelection{}),
				selectors: map[string]TargetSelection{},
			},
			args: args{
				siteId: "00000000-0000-0000-0000-000000000001",
				connector: &skupperv2alpha1.Connector{
					ObjectMeta: v1.ObjectMeta{
						Name:      "backend",
						Namespace: "test",
						UID:       "8a96ffdf-403b-4e4a-83a8-97d3d459adb6",
					},
					Spec: skupperv2alpha1.ConnectorSpec{
						Host: "192.168.1.1",
						Port: 8080,
					},
				},
				config: qdr.NewBridgeConfig(),
			},
			expected: expected{
				config: qdr.BridgeConfig{
					TcpListeners: map[string]qdr.TcpEndpoint{},
					TcpConnectors: map[string]qdr.TcpEndpoint{
						"backend-192.168.1.1": qdr.TcpEndpoint{
							Name:   "backend-192.168.1.1",
							Host:   "192.168.1.1",
							Port:   "8080",
							SiteId: "00000000-0000-0000-0000-000000000001",
						},
					},
				},
			},
		},
		{
			name: "connector spec selector populated, tracking pods",
			fields: fields{
				context: NewMockBindingContext(map[string]TargetSelection{}),
				selectors: map[string]TargetSelection{
					"backend": NewMockTargetSelection("app=backend",
						[]skupperv2alpha1.PodDetails{
							{UID: "30af5279-be83-41e4-86fe-cc45396786f4",
								Name: "backend-8485574c8b-254ms",
								IP:   "10.244.0.9"},
						}),
				},
			},
			args: args{
				siteId: "00000000-0000-0000-0000-000000000001",
				connector: &skupperv2alpha1.Connector{
					ObjectMeta: v1.ObjectMeta{
						Name:      "backend",
						Namespace: "test",
						UID:       "8a96ffdf-403b-4e4a-83a8-97d3d459adb6",
					},
					Spec: skupperv2alpha1.ConnectorSpec{
						Selector: "app=backend",
					},
				},
				config: qdr.NewBridgeConfig(),
			},
			expected: expected{
				config: qdr.BridgeConfig{
					TcpListeners: map[string]qdr.TcpEndpoint{},
					TcpConnectors: map[string]qdr.TcpEndpoint{
						"backend-10.244.0.9": qdr.TcpEndpoint{
							Name:      "backend-10.244.0.9",
							Host:      "10.244.0.9",
							Port:      "0",
							SiteId:    "00000000-0000-0000-0000-000000000001",
							ProcessID: "30af5279-be83-41e4-86fe-cc45396786f4",
						},
					},
				},
			},
		},
		{
			name: "connector spec selector populated, not tracking pods yet",
			fields: fields{
				context:   NewMockBindingContext(map[string]TargetSelection{}),
				selectors: map[string]TargetSelection{},
			},
			args: args{
				siteId: "00000000-0000-0000-0000-000000000001",
				connector: &skupperv2alpha1.Connector{
					ObjectMeta: v1.ObjectMeta{
						Name:      "backend",
						Namespace: "test",
						UID:       "8a96ffdf-403b-4e4a-83a8-97d3d459adb6",
					},
					Spec: skupperv2alpha1.ConnectorSpec{
						Selector: "app=backend",
					},
				},
				config: qdr.NewBridgeConfig(),
			},
			expected: expected{
				config: qdr.BridgeConfig{
					TcpListeners:  map[string]qdr.TcpEndpoint{},
					TcpConnectors: map[string]qdr.TcpEndpoint{},
				},
			},
		},
		{
			name: "Error: connector has neither host nor selector set",
			fields: fields{
				context:   NewMockBindingContext(map[string]TargetSelection{}),
				selectors: map[string]TargetSelection{},
			},
			args: args{
				siteId: "00000000-0000-0000-0000-000000000001",
				connector: &skupperv2alpha1.Connector{
					ObjectMeta: v1.ObjectMeta{
						Name:      "backend",
						Namespace: "test",
						UID:       "8a96ffdf-403b-4e4a-83a8-97d3d459adb6",
					},
					Spec: skupperv2alpha1.ConnectorSpec{},
				},
				config: qdr.NewBridgeConfig(),
			},
			expected: expected{
				config: qdr.BridgeConfig{
					TcpListeners:  map[string]qdr.TcpEndpoint{},
					TcpConnectors: map[string]qdr.TcpEndpoint{},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &BindingAdaptor{
				context:   tt.fields.context,
				mapping:   tt.fields.mapping,
				exposed:   tt.fields.exposed,
				selectors: tt.fields.selectors,
			}
			a.updateBridgeConfigForConnector(tt.args.siteId, tt.args.connector, &tt.args.config)

			assert.DeepEqual(t, tt.args.config, tt.expected.config)
		})
	}
}

func TestBindingAdaptor_ListenerUpdated(t *testing.T) {
	type fields struct {
		context   BindingContext
		mapping   *qdr.PortMapping
		exposed   ExposedPorts
		selectors map[string]TargetSelection
	}
	type args struct {
		listener *skupperv2alpha1.Listener
	}
	type expected struct {
		exposed ExposedPorts
	}
	tests := []struct {
		name     string
		fields   fields
		args     args
		expected expected
	}{
		{
			name: "Successfully expose listener",
			fields: fields{
				context: NewMockBindingContext(nil),
				// TMPDBG mapping: &qdr.PortMapping{},
				mapping:   qdr.RecoverPortMapping(&qdr.RouterConfig{}),
				exposed:   ExposedPorts{},
				selectors: map[string]TargetSelection{},
			},
			args: args{
				listener: &skupperv2alpha1.Listener{
					ObjectMeta: v1.ObjectMeta{
						Name:      "backend",
						Namespace: "test",
						UID:       "8a96ffdf-403b-4e4a-83a8-97d3d459adb6",
					},
					Spec: skupperv2alpha1.ListenerSpec{
						Host:       "backend",
						Port:       8080,
						RoutingKey: "backend",
					},
				},
			},
			expected: expected{
				exposed: map[string]*ExposedPortSet{
					"backend": &ExposedPortSet{
						Host: "backend",
						Ports: map[string]Port{
							"backend": Port{
								Name:       "backend",
								Port:       8080,
								TargetPort: 1024,
								Protocol:   "TCP",
							},
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &BindingAdaptor{
				context:   tt.fields.context,
				mapping:   tt.fields.mapping,
				exposed:   tt.fields.exposed,
				selectors: tt.fields.selectors,
			}
			a.ListenerUpdated(tt.args.listener)

			assert.DeepEqual(t, a.exposed, tt.expected.exposed)
		})
	}
}

func TestBindingAdaptor_ListenerDeleted(t *testing.T) {
	type fields struct {
		context   BindingContext
		mapping   *qdr.PortMapping
		exposed   ExposedPorts
		selectors map[string]TargetSelection
	}
	type args struct {
		listener *skupperv2alpha1.Listener
	}
	type expected struct {
		unexposedHost string
	}
	tests := []struct {
		name     string
		fields   fields
		args     args
		expected expected
	}{
		{
			name: "Successfully delete listener",
			fields: fields{
				context:   NewMockBindingContext(nil),
				mapping:   qdr.RecoverPortMapping(&qdr.RouterConfig{}),
				exposed:   ExposedPorts{},
				selectors: map[string]TargetSelection{},
			},
			args: args{
				listener: &skupperv2alpha1.Listener{
					ObjectMeta: v1.ObjectMeta{
						Name:      "backend",
						Namespace: "test",
						UID:       "8a96ffdf-403b-4e4a-83a8-97d3d459adb6",
					},
					Spec: skupperv2alpha1.ListenerSpec{
						Host:       "backend",
						Port:       8080,
						RoutingKey: "backend",
					},
				},
			},
			expected: expected{
				unexposedHost: "backend",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &BindingAdaptor{
				context:   tt.fields.context,
				mapping:   tt.fields.mapping,
				exposed:   tt.fields.exposed,
				selectors: tt.fields.selectors,
			}
			a.ListenerDeleted(tt.args.listener)

			mockBindingContext, ok := a.context.(*MockBindingContext)
			assert.Equal(t, ok, true)
			assert.Equal(t, mockBindingContext.unexposedHost, tt.expected.unexposedHost)
		})
	}
}

func TestBindingAdaptor_updateBridgeConfigForListener(t *testing.T) {
	type fields struct {
		context   BindingContext
		mapping   *qdr.PortMapping
		exposed   ExposedPorts
		selectors map[string]TargetSelection
	}
	type args struct {
		siteId   string
		listener *skupperv2alpha1.Listener
		config   qdr.BridgeConfig
	}
	type expected struct {
		config qdr.BridgeConfig
	}
	tests := []struct {
		name     string
		fields   fields
		args     args
		expected expected
	}{
		{
			name: "listener updated successfully",
			fields: fields{
				context:   NewMockBindingContext(map[string]TargetSelection{}),
				mapping:   qdr.RecoverPortMapping(&qdr.RouterConfig{}),
				selectors: map[string]TargetSelection{},
			},
			args: args{
				siteId: "00000000-0000-0000-0000-000000000001",
				listener: &skupperv2alpha1.Listener{
					ObjectMeta: v1.ObjectMeta{
						Name:      "backend",
						Namespace: "test",
						UID:       "8a96ffdf-403b-4e4a-83a8-97d3d459adb6",
					},
					Spec: skupperv2alpha1.ListenerSpec{
						Host:       "backend",
						Port:       8080,
						RoutingKey: "backend",
					},
				},
				config: qdr.NewBridgeConfig(),
			},
			expected: expected{
				config: qdr.BridgeConfig{
					TcpListeners: map[string]qdr.TcpEndpoint{
						"backend": qdr.TcpEndpoint{

							Name:    "backend",
							Host:    "0.0.0.0",
							Port:    "1024",
							Address: "backend",
							SiteId:  "00000000-0000-0000-0000-000000000001",
						},
					},
					TcpConnectors: map[string]qdr.TcpEndpoint{},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &BindingAdaptor{
				context:   tt.fields.context,
				mapping:   tt.fields.mapping,
				exposed:   tt.fields.exposed,
				selectors: tt.fields.selectors,
			}
			a.updateBridgeConfigForListener(tt.args.siteId, tt.args.listener, &tt.args.config)

			assert.DeepEqual(t, tt.args.config, tt.expected.config)
		})
	}
}

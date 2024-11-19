package site

import (
	"testing"

	skupperv2alpha1 "github.com/skupperproject/skupper/pkg/apis/skupper/v2alpha1"
	"github.com/skupperproject/skupper/pkg/qdr"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"gotest.tools/assert"
)

type MockBindingContext struct {
	selectors map[string]TargetSelection
}

func NewMockBindingContext(selectors map[string]TargetSelection) *MockBindingContext {
	return &MockBindingContext{
		selectors: selectors,
	}
}

func (m *MockBindingContext) Select(connector *skupperv2alpha1.Connector) TargetSelection {
	if selector, ok := m.selectors[connector.Name]; ok {
		return selector
	}
	return nil
}

func (m *MockBindingContext) Expose(ports *ExposedPortSet) {
}

func (m *MockBindingContext) Unexpose(host string) {
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

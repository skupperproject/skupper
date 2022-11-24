package service

import (
	"log"
	"reflect"
	"testing"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/pkg/qdr"
	"gotest.tools/assert"
)

const (
	MIN_PORT = 1024
)

type DummyServiceBindingContext struct {
	hosts map[string][]string
}

func (c *DummyServiceBindingContext) NewTargetResolver(address string, selector string, skipStatusCheck bool, namespace string) (TargetResolver, error) {
	hosts := []string{}
	if c.hosts != nil {
		if value, ok := c.hosts[selector]; ok {
			hosts = value
		}
	}
	return NewNullTargetResolver(hosts), nil
}
func (*DummyServiceBindingContext) NewServiceIngress(def *types.ServiceInterface) ServiceIngress {
	return newDummyServiceIngress(def.ExposeIngress)
}

func newDummyServiceIngress(mode types.ServiceIngressMode) ServiceIngress {
	return &DummyServiceIngress{
		mode: mode,
	}
}

func (c *DummyServiceBindingContext) NewExternalBridge(def *types.ServiceInterface) ExternalBridge {
	return nil
}

type DummyServiceIngress struct {
	mode types.ServiceIngressMode
}

func (dsi *DummyServiceIngress) Realise(binding *ServiceBindings) error {
	return nil
}

func (dsi *DummyServiceIngress) Mode() types.ServiceIngressMode {
	return dsi.mode
}

func (dsi *DummyServiceIngress) Matches(def *types.ServiceInterface) bool {
	return dsi.mode == def.ExposeIngress
}

func TestNewServiceBindings(t *testing.T) {
	dummyContext := &DummyServiceBindingContext{}
	type scenario struct {
		name     string
		service  types.ServiceInterface
		expected *ServiceBindings
	}
	scenarios := []scenario{
		{
			name: "tcp-service",
			service: types.ServiceInterface{
				Address:  "test",
				Protocol: "tcp",
				Ports:    []int{8080},
			},
			expected: &ServiceBindings{
				protocol:     "tcp",
				Address:      "test",
				publicPorts:  []int{8080},
				ingressPorts: []int{MIN_PORT},
				targets:      map[string]*EgressBindings{},
			},
		},
		{
			name: "tcp-service-multi-port",
			service: types.ServiceInterface{
				Address:  "test-multi-port",
				Protocol: "tcp",
				Ports:    []int{8080, 9090},
			},
			expected: &ServiceBindings{
				protocol:     "tcp",
				Address:      "test-multi-port",
				publicPorts:  []int{8080, 9090},
				ingressPorts: []int{MIN_PORT + 1, MIN_PORT + 2},
				targets:      map[string]*EgressBindings{},
			},
		},
		{
			name: "tcp-service-add-labels",
			service: types.ServiceInterface{
				Address:  "test",
				Protocol: "tcp",
				Ports:    []int{8080},
				Labels: map[string]string{
					"app": "test",
				},
			},
			expected: &ServiceBindings{
				protocol:     "tcp",
				Address:      "test",
				publicPorts:  []int{8080},
				ingressPorts: []int{MIN_PORT},
				Labels: map[string]string{
					"app": "test",
				},
				targets: map[string]*EgressBindings{},
			},
		},
		{
			name: "tcp-service-add-target-upd-port-labels",
			service: types.ServiceInterface{
				Address:  "test",
				Protocol: "tcp",
				Ports:    []int{9090},
				Labels: map[string]string{
					"app": "test-updated",
				},
				Targets: []types.ServiceInterfaceTarget{
					{
						Name:        "test-target",
						Selector:    "app=test",
						TargetPorts: map[int]int{9090: 9090},
						Service:     "",
					},
				},
			},
			expected: &ServiceBindings{
				protocol:     "tcp",
				Address:      "test",
				publicPorts:  []int{9090},
				ingressPorts: []int{MIN_PORT},
				Labels: map[string]string{
					"app": "test-updated",
				},
				targets: map[string]*EgressBindings{
					"app=test": {
						name:        "test-target",
						Selector:    "app=test",
						service:     "",
						egressPorts: map[int]int{9090: 9090},
					},
				},
			},
		},
		{
			name: "tcp-service-add-headless-upd-targets",
			service: types.ServiceInterface{
				Address:  "test",
				Protocol: "tcp",
				Ports:    []int{9090},
				Labels: map[string]string{
					"app": "test-updated",
				},
				Headless: &types.Headless{
					Name:        "test-headless",
					Size:        2,
					TargetPorts: map[int]int{9090: 9090},
				},
				Targets: []types.ServiceInterfaceTarget{
					{
						Name:        "test-target",
						Selector:    "",
						TargetPorts: map[int]int{9090: 9090},
						Service:     "test-svc",
					},
				},
			},
			expected: &ServiceBindings{
				protocol:     "tcp",
				Address:      "test",
				publicPorts:  []int{9090},
				ingressPorts: []int{9090},
				Labels: map[string]string{
					"app": "test-updated",
				},
				headless: &types.Headless{
					Name:        "test-headless",
					Size:        2,
					TargetPorts: map[int]int{9090: 9090},
				},
				targets: map[string]*EgressBindings{
					"test-svc": {
						name:        "test-target",
						Selector:    "",
						service:     "test-svc",
						egressPorts: map[int]int{9090: 9090},
					},
				},
			},
		},
		{
			name: "tcp-headless",
			service: types.ServiceInterface{
				Address:  "tcp-headless",
				Protocol: "tcp",
				Ports:    []int{8080},
				Headless: &types.Headless{
					Name:        "headless",
					Size:        1,
					TargetPorts: map[int]int{8080: 9090},
				},
				Labels: map[string]string{
					"app": "no-head",
				},
				Targets: []types.ServiceInterfaceTarget{
					{Name: "tcp-headless", Selector: "", TargetPorts: map[int]int{8080: 9090}, Service: "test-headless"},
					{Name: "tcp-headless", Selector: "app=headless", TargetPorts: map[int]int{8080: 9090}, Service: ""},
				},
			},
			expected: &ServiceBindings{
				protocol:     "tcp",
				Address:      "tcp-headless",
				publicPorts:  []int{8080},
				ingressPorts: []int{8080},
				headless: &types.Headless{
					Name:        "headless",
					Size:        1,
					TargetPorts: map[int]int{8080: 9090},
				},
				Labels: map[string]string{
					"app": "no-head",
				},
				targets: map[string]*EgressBindings{
					"test-headless": {
						name:        "tcp-headless",
						Selector:    "",
						service:     "test-headless",
						egressPorts: map[int]int{8080: 9090},
					},
					"app=headless": {
						name:        "tcp-headless",
						Selector:    "app=headless",
						service:     "",
						egressPorts: map[int]int{8080: 9090},
					},
				},
			},
		},
		{
			name: "tcp-headless-add-aggregate-eventchannel-upd-protocol-headless-targetports",
			service: types.ServiceInterface{
				Address:      "tcp-headless",
				Protocol:     "http",
				Ports:        []int{8181},
				Aggregate:    "json",
				EventChannel: true,
				Headless: &types.Headless{
					Name:        "headless-upd",
					Size:        2,
					TargetPorts: map[int]int{8181: 8181},
				},
				Labels: map[string]string{
					"app": "no-head",
				},
				Targets: []types.ServiceInterfaceTarget{
					{Name: "tcp-headless", Selector: "", TargetPorts: map[int]int{8181: 9191}, Service: "test-headless"},
					{Name: "tcp-headless", Selector: "app=headless", TargetPorts: map[int]int{8181: 9191}, Service: ""},
				},
			},
			expected: &ServiceBindings{
				protocol:     "http",
				Address:      "tcp-headless",
				publicPorts:  []int{8181},
				ingressPorts: []int{8181},
				aggregation:  "json",
				eventChannel: true,
				headless: &types.Headless{
					Name:        "headless-upd",
					Size:        2,
					TargetPorts: map[int]int{8181: 8181},
				},
				Labels: map[string]string{
					"app": "no-head",
				},
				targets: map[string]*EgressBindings{
					"test-headless": {
						name:        "tcp-headless",
						Selector:    "",
						service:     "test-headless",
						egressPorts: map[int]int{8181: 9191},
					},
					"app=headless": {
						name:        "tcp-headless",
						Selector:    "app=headless",
						service:     "",
						egressPorts: map[int]int{8181: 9191},
					},
				},
			},
		},
		{
			name: "tcp-headless-del-headless",
			service: types.ServiceInterface{
				Address:      "tcp-headless",
				Protocol:     "http",
				Ports:        []int{8181},
				Aggregate:    "json",
				EventChannel: true,
				Labels: map[string]string{
					"app": "no-head",
				},
				Targets: []types.ServiceInterfaceTarget{
					{Name: "tcp-headless", Selector: "", TargetPorts: map[int]int{8181: 9090}, Service: "test-headless"},
					{Name: "tcp-headless", Selector: "app=headless", TargetPorts: map[int]int{8181: 9090}, Service: ""},
				},
			},
			expected: &ServiceBindings{
				protocol:     "http",
				Address:      "tcp-headless",
				publicPorts:  []int{8181},
				ingressPorts: []int{8181},
				aggregation:  "json",
				eventChannel: true,
				Labels: map[string]string{
					"app": "no-head",
				},
				targets: map[string]*EgressBindings{
					"test-headless": {
						name:        "tcp-headless",
						Selector:    "",
						service:     "test-headless",
						egressPorts: map[int]int{8181: 9090},
					},
					"app=headless": {
						name:        "tcp-headless",
						Selector:    "app=headless",
						service:     "",
						egressPorts: map[int]int{8181: 9090},
					},
				},
			},
		},
		{
			name: "tcp-headless-del-labels-targets-upd-aggregation",
			service: types.ServiceInterface{
				Address:      "tcp-headless",
				Protocol:     "http",
				Ports:        []int{8181},
				Aggregate:    "multipart",
				EventChannel: true,
			},
			expected: &ServiceBindings{
				protocol:     "http",
				Address:      "tcp-headless",
				publicPorts:  []int{8181},
				ingressPorts: []int{8181},
				aggregation:  "multipart",
				eventChannel: true,
			},
		},
		{
			name: "tcp-service-publish-not-ready-addresses",
			service: types.ServiceInterface{
				Address:                  "test",
				Protocol:                 "tcp",
				Ports:                    []int{8080},
				PublishNotReadyAddresses: true,
			},
			expected: &ServiceBindings{
				protocol:                 "tcp",
				Address:                  "test",
				publicPorts:              []int{8080},
				ingressPorts:             []int{MIN_PORT},
				targets:                  map[string]*EgressBindings{},
				PublishNotReadyAddresses: true,
			},
		},
		{
			name: "http2-headless-publish-not-ready-addresses",
			service: types.ServiceInterface{
				Address:  "http2-headless",
				Protocol: "http2",
				Ports:    []int{8080},
				Headless: &types.Headless{
					Name:        "headless",
					Size:        1,
					TargetPorts: map[int]int{8080: 9090},
				},
				Labels: map[string]string{
					"app": "no-head",
				},
				Targets: []types.ServiceInterfaceTarget{
					{Name: "http2-headless", Selector: "", TargetPorts: map[int]int{8080: 9090}, Service: "test-headless"},
					{Name: "http2-headless", Selector: "app=headless", TargetPorts: map[int]int{8080: 9090}, Service: ""},
				},
				PublishNotReadyAddresses: true,
			},
			expected: &ServiceBindings{
				protocol:     "http2",
				Address:      "http2-headless",
				publicPorts:  []int{8080},
				ingressPorts: []int{8080},
				headless: &types.Headless{
					Name:        "headless",
					Size:        1,
					TargetPorts: map[int]int{8080: 9090},
				},
				Labels: map[string]string{
					"app": "no-head",
				},
				targets: map[string]*EgressBindings{
					"test-headless": {
						name:        "http2-headless",
						Selector:    "",
						service:     "test-headless",
						egressPorts: map[int]int{8080: 9090},
					},
					"app=headless": {
						name:        "http2-headless",
						Selector:    "app=headless",
						service:     "",
						egressPorts: map[int]int{8080: 9090},
					},
				},
				PublishNotReadyAddresses: true,
			},
		},
		{
			name: "tcp-service-add-target-with-namespace",
			service: types.ServiceInterface{
				Address:  "test",
				Protocol: "tcp",
				Ports:    []int{9090},
				Labels: map[string]string{
					"app": "new-app",
				},
				Targets: []types.ServiceInterfaceTarget{
					{
						Name:        "test-target",
						Selector:    "app=test",
						TargetPorts: map[int]int{9090: 9090},
						Service:     "",
						Namespace:   "another-namespace",
					},
				},
			},
			expected: &ServiceBindings{
				protocol:     "tcp",
				Address:      "test",
				publicPorts:  []int{9090},
				ingressPorts: []int{MIN_PORT},
				Labels: map[string]string{
					"app": "new-app",
				},
				targets: map[string]*EgressBindings{
					"app=test": {
						name:        "test-target",
						Selector:    "app=test",
						service:     "",
						egressPorts: map[int]int{9090: 9090},
						namespace:   "another-namespace",
					},
				},
			},
		},
	}

	for _, s := range scenarios {
		t.Run(s.name, func(t *testing.T) {
			b := NewServiceBindings(s.service, s.expected.ingressPorts /*test port allocation elsewhere*/, dummyContext)
			assert.Equal(t, b.protocol, s.expected.protocol)
			assert.Equal(t, b.Address, s.expected.Address)
			assert.Assert(t, reflect.DeepEqual(b.publicPorts, s.expected.publicPorts))
			assert.Assert(t, reflect.DeepEqual(b.ingressPorts, s.expected.ingressPorts), "got: %v - expected: %v", b.ingressPorts, s.expected.ingressPorts)
			assert.Equal(t, b.aggregation, s.expected.aggregation)
			assert.Equal(t, b.eventChannel, s.expected.eventChannel)
			assert.Equal(t, b.headless == nil, s.expected.headless == nil)
			assert.Equal(t, b.PublishNotReadyAddresses, s.expected.PublishNotReadyAddresses)
			if s.expected.headless != nil {
				assert.Equal(t, b.headless.Name, s.expected.headless.Name)
				assert.Equal(t, b.headless.Size, s.expected.headless.Size)
				assert.Assert(t, reflect.DeepEqual(b.headless.TargetPorts, s.expected.headless.TargetPorts))
			}
			assert.DeepEqual(t, b.Labels, s.expected.Labels)
			assert.DeepEqual(t, b.Annotations, s.expected.Annotations)
			assert.Equal(t, len(b.targets), len(s.expected.targets))
			if len(s.expected.targets) > 0 {
				for k, v := range s.expected.targets {
					log.Println(b.targets)
					bv, ok := b.targets[k]
					assert.Equal(t, bv.name, v.name)
					assert.Assert(t, ok)
					assert.Equal(t, bv.service, v.service)
					assert.Assert(t, reflect.DeepEqual(bv.egressPorts, v.egressPorts))
					assert.Equal(t, bv.Selector, v.Selector)
					assert.Equal(t, bv.service, v.service)
					assert.Equal(t, bv.namespace, v.namespace)
				}
			}
			si := b.AsServiceInterface()
			copy := s.service
			copy.Targets = nil
			assert.DeepEqual(t, si, copy)
		})
	}

}

func TestUpdateServiceBindings(t *testing.T) {
	dummyContext := &DummyServiceBindingContext{}
	type scenario struct {
		name     string
		initial  types.ServiceInterface
		update   types.ServiceInterface
		expected *ServiceBindings
	}
	scenarios := []scenario{
		{
			name: "add port",
			initial: types.ServiceInterface{
				Address:  "test",
				Protocol: "tcp",
				Ports:    []int{8080},
			},
			update: types.ServiceInterface{
				Address:  "test",
				Protocol: "tcp",
				Ports:    []int{8080, 9090},
			},
			expected: &ServiceBindings{
				protocol:    "tcp",
				Address:     "test",
				publicPorts: []int{8080, 9090},
				targets:     map[string]*EgressBindings{},
			},
		},
		{
			name: "change protocol",
			initial: types.ServiceInterface{
				Address:  "test",
				Protocol: "tcp",
				Ports:    []int{8080},
			},
			update: types.ServiceInterface{
				Address:  "test",
				Protocol: "http",
				Ports:    []int{8080},
			},
			expected: &ServiceBindings{
				protocol:    "http",
				Address:     "test",
				publicPorts: []int{8080},
				targets:     map[string]*EgressBindings{},
			},
		},
		{
			name: "change ingress binding",
			initial: types.ServiceInterface{
				Address:  "test",
				Protocol: "tcp",
				Ports:    []int{8080},
			},
			update: types.ServiceInterface{
				Address:       "test",
				Protocol:      "http",
				Ports:         []int{8080},
				ExposeIngress: types.ServiceIngressModeNever,
			},
			expected: &ServiceBindings{
				protocol:       "http",
				Address:        "test",
				publicPorts:    []int{8080},
				targets:        map[string]*EgressBindings{},
				ingressBinding: newDummyServiceIngress(types.ServiceIngressModeNever),
			},
		},
		{
			name: "change aggregation",
			initial: types.ServiceInterface{
				Address:   "test",
				Protocol:  "tcp",
				Ports:     []int{8080},
				Aggregate: "multipart",
			},
			update: types.ServiceInterface{
				Address:   "test",
				Protocol:  "http",
				Ports:     []int{8080},
				Aggregate: "json",
			},
			expected: &ServiceBindings{
				protocol:    "http",
				Address:     "test",
				publicPorts: []int{8080},
				targets:     map[string]*EgressBindings{},
				aggregation: "json",
			},
		},
		{
			name: "make eventchannel",
			initial: types.ServiceInterface{
				Address:  "test",
				Protocol: "tcp",
				Ports:    []int{8080},
			},
			update: types.ServiceInterface{
				Address:      "test",
				Protocol:     "http",
				Ports:        []int{8080},
				EventChannel: true,
			},
			expected: &ServiceBindings{
				protocol:     "http",
				Address:      "test",
				publicPorts:  []int{8080},
				targets:      map[string]*EgressBindings{},
				eventChannel: true,
			},
		},
		{
			name: "add headless",
			initial: types.ServiceInterface{
				Address:  "test",
				Protocol: "tcp",
				Ports:    []int{8080},
			},
			update: types.ServiceInterface{
				Address:  "test",
				Protocol: "http",
				Ports:    []int{8080},
				Headless: &types.Headless{
					Name: "foo",
					Size: 3,
				},
			},
			expected: &ServiceBindings{
				protocol:    "http",
				Address:     "test",
				publicPorts: []int{8080},
				targets:     map[string]*EgressBindings{},
				headless: &types.Headless{
					Name: "foo",
					Size: 3,
				},
			},
		},
		{
			name: "change headless",
			initial: types.ServiceInterface{
				Address:  "test",
				Protocol: "tcp",
				Ports:    []int{8080},
				Headless: &types.Headless{
					Name: "foo",
					Size: 3,
				},
			},
			update: types.ServiceInterface{
				Address:  "test",
				Protocol: "http",
				Ports:    []int{8080},
				Headless: &types.Headless{
					Name: "bar",
					Size: 2,
					TargetPorts: map[int]int{
						8888: 9999,
					},
				},
			},
			expected: &ServiceBindings{
				protocol:    "http",
				Address:     "test",
				publicPorts: []int{8080},
				targets:     map[string]*EgressBindings{},
				headless: &types.Headless{
					Name: "bar",
					Size: 2,
					TargetPorts: map[int]int{
						8888: 9999,
					},
				},
			},
		},
		{
			name: "remove headless",
			initial: types.ServiceInterface{
				Address:  "test",
				Protocol: "tcp",
				Ports:    []int{8080},
				Headless: &types.Headless{
					Name: "foo",
					Size: 3,
				},
			},
			update: types.ServiceInterface{
				Address:  "test",
				Protocol: "tcp",
				Ports:    []int{8080},
			},
			expected: &ServiceBindings{
				protocol:    "tcp",
				Address:     "test",
				publicPorts: []int{8080},
				targets:     map[string]*EgressBindings{},
			},
		},
		{
			name: "add tls credentials",
			initial: types.ServiceInterface{
				Address:  "test",
				Protocol: "tcp",
				Ports:    []int{8080},
			},
			update: types.ServiceInterface{
				Address:        "test",
				Protocol:       "tcp",
				Ports:          []int{8080},
				TlsCredentials: "xyz",
			},
			expected: &ServiceBindings{
				protocol:       "tcp",
				Address:        "test",
				publicPorts:    []int{8080},
				TlsCredentials: "xyz",
			},
		},
		{
			name: "add targets",
			initial: types.ServiceInterface{
				Address:  "test",
				Protocol: "tcp",
				Ports:    []int{8080},
			},
			update: types.ServiceInterface{
				Address:  "test",
				Protocol: "tcp",
				Ports:    []int{8080},
				Targets: []types.ServiceInterfaceTarget{
					{
						Name:        "target1",
						Selector:    "app=test",
						TargetPorts: map[int]int{9090: 9090},
						Service:     "",
					},
					{
						Name:        "target2",
						Selector:    "",
						TargetPorts: map[int]int{9090: 9090},
						Service:     "foo",
					},
				},
			},
			expected: &ServiceBindings{
				protocol:    "tcp",
				Address:     "test",
				publicPorts: []int{8080},
				targets: map[string]*EgressBindings{
					"app=test": {
						name:        "target1",
						Selector:    "app=test",
						service:     "",
						egressPorts: map[int]int{9090: 9090},
					},
					"foo": {
						name:        "target2",
						Selector:    "",
						service:     "foo",
						egressPorts: map[int]int{9090: 9090},
					},
				},
			},
		},
		{
			name: "change target ports",
			initial: types.ServiceInterface{
				Address:  "test",
				Protocol: "tcp",
				Ports:    []int{8080},
				Targets: []types.ServiceInterfaceTarget{
					{
						Name:        "target1",
						Selector:    "app=test",
						TargetPorts: map[int]int{8080: 9090},
						Service:     "",
					},
					{
						Name:        "target2",
						Selector:    "",
						TargetPorts: map[int]int{8080: 9090},
						Service:     "foo",
					},
				},
			},
			update: types.ServiceInterface{
				Address:  "test",
				Protocol: "tcp",
				Ports:    []int{8080},
				Targets: []types.ServiceInterfaceTarget{
					{
						Name:        "target1",
						Selector:    "app=test",
						TargetPorts: map[int]int{8080: 9091},
						Service:     "",
					},
					{
						Name:        "target2",
						Selector:    "",
						TargetPorts: map[int]int{8080: 8888},
						Service:     "foo",
					},
				},
			},
			expected: &ServiceBindings{
				protocol:    "tcp",
				Address:     "test",
				publicPorts: []int{8080},
				targets: map[string]*EgressBindings{
					"app=test": {
						name:        "target1",
						Selector:    "app=test",
						service:     "",
						egressPorts: map[int]int{8080: 9091},
					},
					"foo": {
						name:        "target2",
						Selector:    "",
						service:     "foo",
						egressPorts: map[int]int{8080: 8888},
					},
				},
			},
		},
		{
			name: "remove targets",
			initial: types.ServiceInterface{
				Address:  "test",
				Protocol: "tcp",
				Ports:    []int{8080},
				Targets: []types.ServiceInterfaceTarget{
					{
						Name:        "target1",
						Selector:    "app=test",
						TargetPorts: map[int]int{9090: 9090},
						Service:     "",
					},
					{
						Name:        "target2",
						Selector:    "",
						TargetPorts: map[int]int{9090: 9090},
						Service:     "foo",
					},
					{
						Name:        "target3",
						Selector:    "app=whatever",
						TargetPorts: map[int]int{9090: 9090},
						Service:     "",
					},
					{
						Name:        "target4",
						Selector:    "",
						TargetPorts: map[int]int{9090: 9090},
						Service:     "bar",
					},
				},
			},
			update: types.ServiceInterface{
				Address:  "test",
				Protocol: "tcp",
				Ports:    []int{8080},
				Targets: []types.ServiceInterfaceTarget{
					{
						Name:        "target3",
						Selector:    "app=whatever",
						TargetPorts: map[int]int{9090: 9090},
						Service:     "",
					},
					{
						Name:        "target4",
						Selector:    "",
						TargetPorts: map[int]int{9090: 9090},
						Service:     "bar",
					},
				},
			},
			expected: &ServiceBindings{
				protocol:    "tcp",
				Address:     "test",
				publicPorts: []int{8080},
				targets: map[string]*EgressBindings{
					"app=whatever": {
						name:        "target3",
						Selector:    "app=whatever",
						service:     "",
						egressPorts: map[int]int{9090: 9090},
					},
					"bar": {
						name:        "target4",
						Selector:    "",
						service:     "bar",
						egressPorts: map[int]int{9090: 9090},
					},
				},
			},
		},
		{
			name: "add labels",
			initial: types.ServiceInterface{
				Address:  "test",
				Protocol: "tcp",
				Ports:    []int{8080},
			},
			update: types.ServiceInterface{
				Address:  "test",
				Protocol: "tcp",
				Ports:    []int{8080, 9090},
				Labels: map[string]string{
					"foo": "bar",
				},
			},
			expected: &ServiceBindings{
				protocol:    "tcp",
				Address:     "test",
				publicPorts: []int{8080, 9090},
				targets:     map[string]*EgressBindings{},
				Labels: map[string]string{
					"foo": "bar",
				},
			},
		},
		{
			name: "remove labels",
			initial: types.ServiceInterface{
				Address:  "test",
				Protocol: "tcp",
				Ports:    []int{8080},
				Labels: map[string]string{
					"foo": "bar",
				},
			},
			update: types.ServiceInterface{
				Address:  "test",
				Protocol: "tcp",
				Ports:    []int{8080, 9090},
			},
			expected: &ServiceBindings{
				protocol:    "tcp",
				Address:     "test",
				publicPorts: []int{8080, 9090},
				targets:     map[string]*EgressBindings{},
			},
		},
		{
			name: "add publish not ready addresses",
			initial: types.ServiceInterface{
				Address:                  "test",
				Protocol:                 "tcp",
				Ports:                    []int{8080},
				PublishNotReadyAddresses: false,
			},
			update: types.ServiceInterface{
				Address:                  "test",
				Protocol:                 "tcp",
				Ports:                    []int{8080},
				PublishNotReadyAddresses: true,
			},
			expected: &ServiceBindings{
				protocol:                 "tcp",
				Address:                  "test",
				publicPorts:              []int{8080},
				PublishNotReadyAddresses: true,
			},
		},
		{
			name: "add target namespace",
			initial: types.ServiceInterface{
				Address:  "test",
				Protocol: "tcp",
				Ports:    []int{8080},
				Targets: []types.ServiceInterfaceTarget{
					{
						Name:        "target1",
						Selector:    "app=test",
						TargetPorts: map[int]int{8080: 9090},
						Service:     "",
					},
				},
			},
			update: types.ServiceInterface{
				Address:  "test",
				Protocol: "tcp",
				Ports:    []int{8080},
				Targets: []types.ServiceInterfaceTarget{
					{
						Name:        "target1",
						Selector:    "app=test",
						TargetPorts: map[int]int{8080: 9090},
						Service:     "",
						Namespace:   "another-namespace",
					},
				},
			},
			expected: &ServiceBindings{
				protocol:    "tcp",
				Address:     "test",
				publicPorts: []int{8080},
				targets: map[string]*EgressBindings{
					"app=test": {
						name:        "target1",
						Selector:    "app=test",
						service:     "",
						egressPorts: map[int]int{8080: 9090},
						namespace:   "another-namespace",
					},
				},
			},
		},
		{
			name: "update target namespace",
			initial: types.ServiceInterface{
				Address:  "test",
				Protocol: "tcp",
				Ports:    []int{8080},
				Targets: []types.ServiceInterfaceTarget{
					{
						Name:        "target1",
						Selector:    "app=test",
						TargetPorts: map[int]int{8080: 9090},
						Service:     "",
						Namespace:   "another-namespace",
					},
				},
			},
			update: types.ServiceInterface{
				Address:  "test",
				Protocol: "tcp",
				Ports:    []int{8080},
				Targets: []types.ServiceInterfaceTarget{
					{
						Name:        "target1",
						Selector:    "app=test",
						TargetPorts: map[int]int{8080: 9090},
						Service:     "",
						Namespace:   "another-another-namespace",
					},
				},
			},
			expected: &ServiceBindings{
				protocol:    "tcp",
				Address:     "test",
				publicPorts: []int{8080},
				targets: map[string]*EgressBindings{
					"app=test": {
						name:        "target1",
						Selector:    "app=test",
						service:     "",
						egressPorts: map[int]int{8080: 9090},
						namespace:   "another-another-namespace",
					},
				},
			},
		},
	}

	for _, s := range scenarios {
		t.Run(s.name, func(t *testing.T) {
			b := NewServiceBindings(s.initial, s.initial.Ports, dummyContext)
			b.Update(s.update, dummyContext)
			assert.Equal(t, b.protocol, s.expected.protocol)
			assert.Equal(t, b.Address, s.expected.Address)
			assert.Assert(t, reflect.DeepEqual(b.publicPorts, s.expected.publicPorts))
			assert.Equal(t, b.aggregation, s.expected.aggregation)
			assert.Equal(t, b.eventChannel, s.expected.eventChannel)
			assert.Equal(t, b.TlsCredentials, s.expected.TlsCredentials)
			assert.Equal(t, b.PublishNotReadyAddresses, s.expected.PublishNotReadyAddresses)
			assert.Equal(t, b.headless == nil, s.expected.headless == nil)
			if s.expected.headless != nil {
				assert.Equal(t, b.headless.Name, s.expected.headless.Name)
				assert.Equal(t, b.headless.Size, s.expected.headless.Size)
				assert.Assert(t, reflect.DeepEqual(b.headless.TargetPorts, s.expected.headless.TargetPorts))
			}
			assert.DeepEqual(t, b.Labels, s.expected.Labels)
			assert.DeepEqual(t, b.Annotations, s.expected.Annotations)
			if s.expected.ingressBinding != nil {
				assert.Equal(t, b.ingressBinding.Mode(), s.expected.ingressBinding.Mode())
			}
			assert.Equal(t, len(b.targets), len(s.expected.targets))
			if len(s.expected.targets) > 0 {
				for k, v := range s.expected.targets {
					log.Println(b.targets)
					bv, ok := b.targets[k]
					assert.Assert(t, ok, "No target for "+k)
					assert.Equal(t, bv.name, v.name)
					assert.Equal(t, bv.service, v.service)
					assert.Assert(t, reflect.DeepEqual(bv.egressPorts, v.egressPorts))
					assert.Equal(t, bv.Selector, v.Selector)
					assert.Equal(t, bv.service, v.service)
					assert.Equal(t, bv.namespace, v.namespace)
				}
			}
		})
	}
}

func TestRequiredBridges(t *testing.T) {
	type scenario struct {
		name     string
		services []types.ServiceInterface
		siteId   string
		expected *qdr.BridgeConfig
	}
	myfalse := false
	scenarios := []scenario{
		{
			name: "simple",
			services: []types.ServiceInterface{
				{
					Address:  "foo",
					Protocol: "tcp",
					Ports:    []int{8080},
					Targets: []types.ServiceInterfaceTarget{
						{
							Name:        "target1",
							Selector:    "app=foo",
							TargetPorts: map[int]int{8080: 9090},
							Service:     "",
						},
					},
				},
				{
					Address:  "bar",
					Protocol: "http",
					Ports:    []int{8080, 9090},
					Targets: []types.ServiceInterfaceTarget{
						{
							Name:        "target1",
							Selector:    "app=bar",
							TargetPorts: map[int]int{8080: 8888, 9090: 9999},
							Service:     "",
						},
						{
							Name:        "target2",
							Selector:    "",
							TargetPorts: map[int]int{8080: 8888, 9090: 9999},
							Service:     "testservice",
						},
					},
				},
				{
					Address:  "baz",
					Protocol: "http2",
					Ports:    []int{8080},
					Targets: []types.ServiceInterfaceTarget{
						{
							Name:        "target1",
							Selector:    "app=foo",
							TargetPorts: map[int]int{8080: 9090},
							Service:     "",
						},
					},
				},
				{
					Address:  "ignoreme",
					Protocol: "carrier-pigeons",
					Ports:    []int{8080},
					Targets: []types.ServiceInterfaceTarget{
						{
							Name:        "target1",
							Selector:    "app=broken",
							TargetPorts: map[int]int{8080: 9090},
							Service:     "",
						},
						{
							Name:        "target2",
							Selector:    "app=foo",
							TargetPorts: map[int]int{8080: 9090},
							Service:     "",
						},
					},
				},
			},
			siteId: "abc",
			expected: &qdr.BridgeConfig{
				TcpConnectors: map[string]qdr.TcpEndpoint{
					"foo.target1@foo-pod-1:8080:9090": qdr.TcpEndpoint{
						Name:    "foo.target1@foo-pod-1:8080:9090",
						Address: "foo:8080",
						Host:    "foo-pod-1",
						Port:    "9090",
						SiteId:  "abc",
					},
				},
				TcpListeners: map[string]qdr.TcpEndpoint{
					"foo:8080": qdr.TcpEndpoint{
						Name:    "foo:8080",
						Address: "foo:8080",
						Port:    "8080",
						SiteId:  "abc",
					},
				},
				HttpConnectors: map[string]qdr.HttpEndpoint{
					"bar.target1@bar-pod-1:8080:8888": qdr.HttpEndpoint{
						Name:    "bar.target1@bar-pod-1:8080:8888",
						Address: "bar:8080",
						Host:    "bar-pod-1",
						Port:    "8888",
						SiteId:  "abc",
					},
					"bar.target1@bar-pod-1:9090:9999": qdr.HttpEndpoint{
						Name:    "bar.target1@bar-pod-1:9090:9999",
						Address: "bar:9090",
						Host:    "bar-pod-1",
						Port:    "9999",
						SiteId:  "abc",
					},
					"bar.target1@bar-pod-2:8080:8888": qdr.HttpEndpoint{
						Name:    "bar.target1@bar-pod-2:8080:8888",
						Address: "bar:8080",
						Host:    "bar-pod-2",
						Port:    "8888",
						SiteId:  "abc",
					},
					"bar.target1@bar-pod-2:9090:9999": qdr.HttpEndpoint{
						Name:    "bar.target1@bar-pod-2:9090:9999",
						Address: "bar:9090",
						Host:    "bar-pod-2",
						Port:    "9999",
						SiteId:  "abc",
					},
					"bar.target2@testservice:8080:8888": qdr.HttpEndpoint{
						Name:         "bar.target2@testservice:8080:8888",
						Address:      "bar:8080",
						Host:         "testservice",
						Port:         "8888",
						SiteId:       "abc",
						HostOverride: "testservice",
					},
					"bar.target2@testservice:9090:9999": qdr.HttpEndpoint{
						Name:         "bar.target2@testservice:9090:9999",
						Address:      "bar:9090",
						Host:         "testservice",
						Port:         "9999",
						SiteId:       "abc",
						HostOverride: "testservice",
					},
					"baz.target1@foo-pod-1:8080:9090": qdr.HttpEndpoint{
						Name:            "baz.target1@foo-pod-1:8080:9090",
						Address:         "baz:8080",
						Host:            "foo-pod-1",
						Port:            "9090",
						SiteId:          "abc",
						ProtocolVersion: "HTTP2",
					},
				},
				HttpListeners: map[string]qdr.HttpEndpoint{
					"bar:8080": qdr.HttpEndpoint{
						Name:    "bar:8080",
						Address: "bar:8080",
						Port:    "8080",
						SiteId:  "abc",
					},
					"bar:9090": qdr.HttpEndpoint{
						Name:    "bar:9090",
						Address: "bar:9090",
						Port:    "9090",
						SiteId:  "abc",
					},
					"baz:8080": qdr.HttpEndpoint{
						Name:            "baz:8080",
						Address:         "baz:8080",
						Port:            "8080",
						SiteId:          "abc",
						ProtocolVersion: "HTTP2",
					},
				},
			},
		},
		{
			name: "aggregation",
			services: []types.ServiceInterface{
				{
					Address:   "mc-service",
					Protocol:  "http",
					Ports:     []int{8080},
					Aggregate: "multipart",
					Targets: []types.ServiceInterfaceTarget{
						{
							Name:        "target1",
							Selector:    "app=bar",
							TargetPorts: map[int]int{8080: 8888},
							Service:     "",
						},
						{
							Name:        "target2",
							Selector:    "",
							TargetPorts: map[int]int{8080: 9999},
							Service:     "testservice",
						},
					},
				},
			},
			siteId: "xyz",
			expected: &qdr.BridgeConfig{
				TcpConnectors: map[string]qdr.TcpEndpoint{},
				TcpListeners:  map[string]qdr.TcpEndpoint{},
				HttpConnectors: map[string]qdr.HttpEndpoint{
					"mc-service.target1@bar-pod-1:8080:8888": qdr.HttpEndpoint{
						Name:        "mc-service.target1@bar-pod-1:8080:8888",
						Address:     "mc/mc-service:8080",
						Aggregation: "multipart",
						Host:        "bar-pod-1",
						Port:        "8888",
						SiteId:      "xyz",
					},
					"mc-service.target1@bar-pod-2:8080:8888": qdr.HttpEndpoint{
						Name:        "mc-service.target1@bar-pod-2:8080:8888",
						Address:     "mc/mc-service:8080",
						Aggregation: "multipart",
						Host:        "bar-pod-2",
						Port:        "8888",
						SiteId:      "xyz",
					},
					"mc-service.target2@testservice:8080:9999": qdr.HttpEndpoint{
						Name:         "mc-service.target2@testservice:8080:9999",
						Address:      "mc/mc-service:8080",
						Aggregation:  "multipart",
						Host:         "testservice",
						HostOverride: "testservice",
						Port:         "9999",
						SiteId:       "xyz",
					},
				},
				HttpListeners: map[string]qdr.HttpEndpoint{
					"mc-service:8080": qdr.HttpEndpoint{
						Name:        "mc-service:8080",
						Address:     "mc/mc-service:8080",
						Aggregation: "multipart",
						Port:        "8080",
						SiteId:      "xyz",
					},
				},
			},
		},
		{
			name: "eventchannel",
			services: []types.ServiceInterface{
				{
					Address:      "myevents",
					Protocol:     "http",
					EventChannel: true,
					Ports:        []int{8080},
					Targets: []types.ServiceInterfaceTarget{
						{
							Name:        "target1",
							Selector:    "app=foo",
							TargetPorts: map[int]int{8080: 8888},
							Service:     "",
						},
					},
				},
			},
			siteId: "xyz",
			expected: &qdr.BridgeConfig{
				TcpConnectors: map[string]qdr.TcpEndpoint{},
				TcpListeners:  map[string]qdr.TcpEndpoint{},
				HttpConnectors: map[string]qdr.HttpEndpoint{
					"myevents.target1@foo-pod-1:8080:8888": qdr.HttpEndpoint{
						Name:         "myevents.target1@foo-pod-1:8080:8888",
						Address:      "mc/myevents:8080",
						EventChannel: true,
						Host:         "foo-pod-1",
						Port:         "8888",
						SiteId:       "xyz",
					},
				},
				HttpListeners: map[string]qdr.HttpEndpoint{
					"myevents:8080": qdr.HttpEndpoint{
						Name:         "myevents:8080",
						Address:      "mc/myevents:8080",
						EventChannel: true,
						Port:         "8080",
						SiteId:       "xyz",
					},
				},
			},
		},
		{
			name: "tls",
			services: []types.ServiceInterface{
				{
					Address:          "special",
					Protocol:         "http2",
					Ports:            []int{8080},
					TlsCredentials:   "mysecret",
					TlsCertAuthority: types.ServiceClientSecret,
					Targets: []types.ServiceInterfaceTarget{
						{
							Name:        "target1",
							Selector:    "app=foo",
							TargetPorts: map[int]int{8080: 8888},
							Service:     "",
						},
					},
				},
			},
			siteId: "xyz",
			expected: &qdr.BridgeConfig{
				TcpConnectors: map[string]qdr.TcpEndpoint{},
				TcpListeners:  map[string]qdr.TcpEndpoint{},
				HttpConnectors: map[string]qdr.HttpEndpoint{
					"special.target1@foo-pod-1:8080:8888": qdr.HttpEndpoint{
						Name:            "special.target1@foo-pod-1:8080:8888",
						Address:         "special:8080",
						Host:            "foo-pod-1",
						Port:            "8888",
						SiteId:          "xyz",
						ProtocolVersion: "HTTP2",
						SslProfile:      "skupper-service-client",
						VerifyHostname:  &myfalse,
					},
				},
				HttpListeners: map[string]qdr.HttpEndpoint{
					"special:8080": qdr.HttpEndpoint{
						Name:            "special:8080",
						Address:         "special:8080",
						Port:            "8080",
						SiteId:          "xyz",
						ProtocolVersion: "HTTP2",
						SslProfile:      "mysecret",
					},
				},
			},
		},
	}
	for _, s := range scenarios {
		t.Run(s.name, func(t *testing.T) {
			context := &DummyServiceBindingContext{
				hosts: map[string][]string{
					"app=foo":    []string{"foo-pod-1"},
					"app=bar":    []string{"bar-pod-1", "bar-pod-2"},
					"app=broken": []string{""},
				},
			}
			bindings := map[string]*ServiceBindings{}
			for _, svc := range s.services {
				bindings[svc.Address] = NewServiceBindings(svc, svc.Ports, context)
			}
			actual := RequiredBridges(bindings, s.siteId)
			assert.Assert(t, reflect.DeepEqual(actual.TcpListeners, s.expected.TcpListeners), "Expected %v got %v", s.expected.TcpListeners, actual.TcpListeners)
			assert.Assert(t, reflect.DeepEqual(actual.HttpListeners, s.expected.HttpListeners), "Expected %v got %v", s.expected.HttpListeners, actual.HttpListeners)
			assert.Assert(t, reflect.DeepEqual(actual.TcpConnectors, s.expected.TcpConnectors), "Expected %v got %v", s.expected.TcpConnectors, actual.TcpConnectors)
			assert.Assert(t, reflect.DeepEqual(actual.HttpConnectors, s.expected.HttpConnectors), "Expected %v got %v", s.expected.HttpConnectors, actual.HttpConnectors)
		})
	}
}

func TestFindLocalTarget(t *testing.T) {
	type scenario struct {
		name                string
		service             types.ServiceInterface
		allocatedPorts      []int
		expectedTarget      bool
		expectedTargetName  string
		expectedTargetPorts map[int]int
		expectedPorts       map[int]int
	}
	scenarios := []scenario{
		{
			name: "one",
			service: types.ServiceInterface{
				Address:  "foo",
				Protocol: "tcp",
				Ports:    []int{8080},
				Targets: []types.ServiceInterfaceTarget{
					{
						Name:        "target1",
						Selector:    "app=ok",
						TargetPorts: map[int]int{8080: 9090},
					},
				},
			},
			allocatedPorts:      []int{1024},
			expectedTarget:      true,
			expectedTargetName:  "target1",
			expectedTargetPorts: map[int]int{8080: 9090},
			expectedPorts:       map[int]int{8080: 1024},
		},
		{
			name: "two",
			service: types.ServiceInterface{
				Address:  "bar",
				Protocol: "tcp",
				Ports:    []int{8080},
				Targets: []types.ServiceInterfaceTarget{
					{
						Name:        "target1",
						Selector:    "app=blah",
						TargetPorts: map[int]int{8080: 9090},
					},
				},
			},
			allocatedPorts: []int{1025},
			expectedTarget: false,
			expectedPorts:  map[int]int{8080: 1025},
		},
		{
			name: "three",
			service: types.ServiceInterface{
				Address:  "baz",
				Protocol: "tcp",
				Ports:    []int{8080},
				Targets: []types.ServiceInterfaceTarget{
					{
						Name:        "target1",
						Selector:    "app=blah",
						TargetPorts: map[int]int{8080: 9090},
					},
					{
						Name:     "target2",
						Selector: "app=ok",
					},
				},
			},
			allocatedPorts:      []int{1026},
			expectedTarget:      true,
			expectedTargetName:  "target2",
			expectedTargetPorts: map[int]int{8080: 8080},
			expectedPorts:       map[int]int{8080: 1026},
		},
	}
	context := &DummyServiceBindingContext{
		hosts: map[string][]string{
			"app=ok": []string{"mypod"},
		},
	}
	for _, s := range scenarios {
		t.Run(s.name, func(t *testing.T) {
			bindings := NewServiceBindings(s.service, s.allocatedPorts, context)
			target := bindings.FindLocalTarget()
			if s.expectedTarget {
				assert.Assert(t, target != nil)
				assert.Equal(t, target.name, s.expectedTargetName)
				t.Logf("for %s, target ports are %v, public ports are %v and allocated ports are %v", s.name, target.egressPorts, bindings.publicPorts, bindings.ingressPorts)
				assert.Assert(t, reflect.DeepEqual(target.GetLocalTargetPorts(bindings), s.expectedTargetPorts), "expected %v got %v", s.expectedTargetPorts, target.GetLocalTargetPorts(bindings))
			} else {
				assert.Assert(t, target == nil)
			}
			assert.Assert(t, reflect.DeepEqual(bindings.PortMap(), s.expectedPorts))
		})
	}
}

type StoppableResolver struct {
	stopped int
}

func (o *StoppableResolver) Close() {
	o.stopped += 1
}

func (o *StoppableResolver) List() []string {
	return []string{}
}

func (o *StoppableResolver) HasTarget() bool {
	return false
}

type StopTestBindingContext struct {
	resolvers []*StoppableResolver
}

func (c *StopTestBindingContext) NewTargetResolver(address string, selector string, skipStatusCheck bool, namespace string) (TargetResolver, error) {
	resolver := &StoppableResolver{}
	c.resolvers = append(c.resolvers, resolver)
	return resolver, nil
}
func (*StopTestBindingContext) NewServiceIngress(def *types.ServiceInterface) ServiceIngress {
	return nil
}
func (c *StopTestBindingContext) NewExternalBridge(def *types.ServiceInterface) ExternalBridge {
	return nil
}

func TestStop(t *testing.T) {
	context := &StopTestBindingContext{}
	service := types.ServiceInterface{
		Address:  "baz",
		Protocol: "tcp",
		Ports:    []int{8080},
		Targets: []types.ServiceInterfaceTarget{
			{
				Name:        "target1",
				Selector:    "app=blah",
				TargetPorts: map[int]int{8080: 9090},
			},
			{
				Name:     "target2",
				Selector: "app=ok",
			},
		},
	}
	bindings := NewServiceBindings(service, service.Ports, context)
	bindings.Stop()
	for _, r := range context.resolvers {
		assert.Equal(t, r.stopped, 1)
	}
}

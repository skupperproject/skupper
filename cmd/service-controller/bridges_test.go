package main

import (
	"log"
	"testing"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/client"
	"gotest.tools/assert"
	"k8s.io/client-go/kubernetes/fake"
)

func TestUpdateServiceBindings(t *testing.T) {
	// Controller's minimal (fake) initialization to unit test updateServiceBindings
	const NS = "test"
	vanClient := &client.VanClient{
		Namespace:  NS,
		KubeClient: fake.NewSimpleClientset(),
	}
	c := &Controller{
		vanClient: vanClient,
	}
	c.bindings = map[string]*ServiceBindings{}
	c.ports = newFreePorts()

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
				Port:     8080,
			},
			expected: &ServiceBindings{
				protocol:    "tcp",
				address:     "test",
				publicPort:  8080,
				ingressPort: MIN_PORT,
				targets:     map[string]*EgressBindings{},
			},
		},
		{
			name: "tcp-service-add-labels",
			service: types.ServiceInterface{
				Address:  "test",
				Protocol: "tcp",
				Port:     8080,
				Labels: map[string]string{
					"app": "test",
				},
			},
			expected: &ServiceBindings{
				protocol:    "tcp",
				address:     "test",
				publicPort:  8080,
				ingressPort: MIN_PORT,
				labels: map[string]string{
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
				Port:     9090,
				Labels: map[string]string{
					"app": "test-updated",
				},
				Targets: []types.ServiceInterfaceTarget{
					{
						Name:       "test-target",
						Selector:   "app=test",
						TargetPort: 9090,
						Service:    "",
					},
				},
			},
			expected: &ServiceBindings{
				protocol:    "tcp",
				address:     "test",
				publicPort:  9090,
				ingressPort: MIN_PORT,
				labels: map[string]string{
					"app": "test-updated",
				},
				targets: map[string]*EgressBindings{
					"app=test": {
						name:       "test-target",
						selector:   "app=test",
						service:    "",
						egressPort: 9090,
					},
				},
			},
		},
		{
			name: "tcp-service-add-headless-upd-targets",
			service: types.ServiceInterface{
				Address:  "test",
				Protocol: "tcp",
				Port:     9090,
				Labels: map[string]string{
					"app": "test-updated",
				},
				Headless: &types.Headless{
					Name:       "test-headless",
					Size:       2,
					TargetPort: 9090,
				},
				Targets: []types.ServiceInterfaceTarget{
					{
						Name:       "test-target",
						Selector:   "",
						TargetPort: 9090,
						Service:    "test-svc",
					},
				},
			},
			expected: &ServiceBindings{
				protocol:    "tcp",
				address:     "test",
				publicPort:  9090,
				ingressPort: 9090,
				labels: map[string]string{
					"app": "test-updated",
				},
				headless: &types.Headless{
					Name:       "test-headless",
					Size:       2,
					TargetPort: 9090,
				},
				targets: map[string]*EgressBindings{
					"test-svc": {
						name:       "test-target",
						selector:   "",
						service:    "test-svc",
						egressPort: 9090,
					},
				},
			},
		},
		{
			name: "tcp-headless",
			service: types.ServiceInterface{
				Address:  "tcp-headless",
				Protocol: "tcp",
				Port:     8080,
				Headless: &types.Headless{
					Name:       "headless",
					Size:       1,
					TargetPort: 8080,
				},
				Labels: map[string]string{
					"app": "no-head",
				},
				Targets: []types.ServiceInterfaceTarget{
					{Name: "tcp-headless", Selector: "", TargetPort: 9090, Service: "test-headless"},
					{Name: "tcp-headless", Selector: "app=headless", TargetPort: 9090, Service: ""},
				},
			},
			expected: &ServiceBindings{
				protocol:    "tcp",
				address:     "tcp-headless",
				publicPort:  8080,
				ingressPort: 8080,
				headless: &types.Headless{
					Name:       "headless",
					Size:       1,
					TargetPort: 8080,
				},
				labels: map[string]string{
					"app": "no-head",
				},
				targets: map[string]*EgressBindings{
					"test-headless": {
						name:       "tcp-headless",
						selector:   "",
						service:    "test-headless",
						egressPort: 9090,
					},
					"app=headless": {
						name:       "tcp-headless",
						selector:   "app=headless",
						service:    "",
						egressPort: 9090,
					},
				},
			},
		},
		{
			name: "tcp-headless-add-aggregate-eventchannel-upd-protocol-headless-targetports",
			service: types.ServiceInterface{
				Address:      "tcp-headless",
				Protocol:     "http",
				Port:         8181,
				Aggregate:    "json",
				EventChannel: true,
				Headless: &types.Headless{
					Name:       "headless-upd",
					Size:       2,
					TargetPort: 8181,
				},
				Labels: map[string]string{
					"app": "no-head",
				},
				Targets: []types.ServiceInterfaceTarget{
					{Name: "tcp-headless", Selector: "", TargetPort: 9091, Service: "test-headless"},
					{Name: "tcp-headless", Selector: "app=headless", TargetPort: 9091, Service: ""},
				},
			},
			expected: &ServiceBindings{
				protocol:     "http",
				address:      "tcp-headless",
				publicPort:   8181,
				ingressPort:  8181,
				aggregation:  "json",
				eventChannel: true,
				headless: &types.Headless{
					Name:       "headless-upd",
					Size:       2,
					TargetPort: 8181,
				},
				labels: map[string]string{
					"app": "no-head",
				},
				targets: map[string]*EgressBindings{
					"test-headless": {
						name:       "tcp-headless",
						selector:   "",
						service:    "test-headless",
						egressPort: 9091,
					},
					"app=headless": {
						name:       "tcp-headless",
						selector:   "app=headless",
						service:    "",
						egressPort: 9091,
					},
				},
			},
		},
		{
			name: "tcp-headless-del-headless",
			service: types.ServiceInterface{
				Address:      "tcp-headless",
				Protocol:     "http",
				Port:         8181,
				Aggregate:    "json",
				EventChannel: true,
				Labels: map[string]string{
					"app": "no-head",
				},
				Targets: []types.ServiceInterfaceTarget{
					{Name: "tcp-headless", Selector: "", TargetPort: 9090, Service: "test-headless"},
					{Name: "tcp-headless", Selector: "app=headless", TargetPort: 9090, Service: ""},
				},
			},
			expected: &ServiceBindings{
				protocol:     "http",
				address:      "tcp-headless",
				publicPort:   8181,
				ingressPort:  8181,
				aggregation:  "json",
				eventChannel: true,
				labels: map[string]string{
					"app": "no-head",
				},
				targets: map[string]*EgressBindings{
					"test-headless": {
						name:       "tcp-headless",
						selector:   "",
						service:    "test-headless",
						egressPort: 9090,
					},
					"app=headless": {
						name:       "tcp-headless",
						selector:   "app=headless",
						service:    "",
						egressPort: 9090,
					},
				},
			},
		},
		{
			name: "tcp-headless-del-labels-targets-upd-aggregation",
			service: types.ServiceInterface{
				Address:      "tcp-headless",
				Protocol:     "http",
				Port:         8181,
				Aggregate:    "multipart",
				EventChannel: true,
			},
			expected: &ServiceBindings{
				protocol:     "http",
				address:      "tcp-headless",
				publicPort:   8181,
				ingressPort:  8181,
				aggregation:  "multipart",
				eventChannel: true,
			},
		},
	}

	for _, s := range scenarios {
		t.Run(s.name, func(t *testing.T) {
			_ = c.updateServiceBindings(s.service, map[string]int{})
			b, ok := c.bindings[s.service.Address]
			assert.Assert(t, ok)
			assert.Equal(t, b.protocol, s.expected.protocol)
			assert.Equal(t, b.address, s.expected.address)
			assert.Equal(t, b.publicPort, s.expected.publicPort)
			assert.Equal(t, b.ingressPort, s.expected.ingressPort)
			assert.Equal(t, b.aggregation, s.expected.aggregation)
			assert.Equal(t, b.eventChannel, s.expected.eventChannel)
			assert.Equal(t, b.headless == nil, s.expected.headless == nil)
			if s.expected.headless != nil {
				assert.Equal(t, b.headless.Name, s.expected.headless.Name)
				assert.Equal(t, b.headless.Size, s.expected.headless.Size)
				assert.Equal(t, b.headless.TargetPort, s.expected.headless.TargetPort)
			}
			assert.DeepEqual(t, b.labels, s.expected.labels)
			assert.Equal(t, len(b.targets), len(s.expected.targets))
			if len(s.expected.targets) > 0 {
				for k, v := range s.expected.targets {
					log.Println(b.targets)
					bv, ok := b.targets[k]
					assert.Equal(t, bv.name, v.name)
					assert.Assert(t, ok)
					assert.Equal(t, bv.service, v.service)
					assert.Equal(t, bv.egressPort, v.egressPort)
					assert.Equal(t, bv.selector, v.selector)
					assert.Equal(t, bv.service, v.service)
				}
			}
		})
	}

}

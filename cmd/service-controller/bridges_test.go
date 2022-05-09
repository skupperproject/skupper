package main

import (
	"log"
	"reflect"
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
		policy:    client.NewClusterPolicyValidator(vanClient),
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
				Ports:    []int{8080},
			},
			expected: &ServiceBindings{
				protocol:     "tcp",
				address:      "test",
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
				address:      "test-multi-port",
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
				address:      "test",
				publicPorts:  []int{8080},
				ingressPorts: []int{MIN_PORT},
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
				address:      "test",
				publicPorts:  []int{9090},
				ingressPorts: []int{MIN_PORT},
				labels: map[string]string{
					"app": "test-updated",
				},
				targets: map[string]*EgressBindings{
					"app=test": {
						name:        "test-target",
						selector:    "app=test",
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
				address:      "test",
				publicPorts:  []int{9090},
				ingressPorts: []int{9090},
				labels: map[string]string{
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
						selector:    "",
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
				address:      "tcp-headless",
				publicPorts:  []int{8080},
				ingressPorts: []int{8080},
				headless: &types.Headless{
					Name:        "headless",
					Size:        1,
					TargetPorts: map[int]int{8080: 9090},
				},
				labels: map[string]string{
					"app": "no-head",
				},
				targets: map[string]*EgressBindings{
					"test-headless": {
						name:        "tcp-headless",
						selector:    "",
						service:     "test-headless",
						egressPorts: map[int]int{8080: 9090},
					},
					"app=headless": {
						name:        "tcp-headless",
						selector:    "app=headless",
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
				address:      "tcp-headless",
				publicPorts:  []int{8181},
				ingressPorts: []int{8181},
				aggregation:  "json",
				eventChannel: true,
				headless: &types.Headless{
					Name:        "headless-upd",
					Size:        2,
					TargetPorts: map[int]int{8181: 8181},
				},
				labels: map[string]string{
					"app": "no-head",
				},
				targets: map[string]*EgressBindings{
					"test-headless": {
						name:        "tcp-headless",
						selector:    "",
						service:     "test-headless",
						egressPorts: map[int]int{8181: 9191},
					},
					"app=headless": {
						name:        "tcp-headless",
						selector:    "app=headless",
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
				address:      "tcp-headless",
				publicPorts:  []int{8181},
				ingressPorts: []int{8181},
				aggregation:  "json",
				eventChannel: true,
				labels: map[string]string{
					"app": "no-head",
				},
				targets: map[string]*EgressBindings{
					"test-headless": {
						name:        "tcp-headless",
						selector:    "",
						service:     "test-headless",
						egressPorts: map[int]int{8181: 9090},
					},
					"app=headless": {
						name:        "tcp-headless",
						selector:    "app=headless",
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
				address:      "tcp-headless",
				publicPorts:  []int{8181},
				ingressPorts: []int{8181},
				aggregation:  "multipart",
				eventChannel: true,
			},
		},
	}

	for _, s := range scenarios {
		t.Run(s.name, func(t *testing.T) {
			_ = c.updateServiceBindings(s.service, map[string][]int{})
			b, ok := c.bindings[s.service.Address]
			assert.Assert(t, ok)
			assert.Equal(t, b.protocol, s.expected.protocol)
			assert.Equal(t, b.address, s.expected.address)
			assert.Assert(t, reflect.DeepEqual(b.publicPorts, s.expected.publicPorts))
			assert.Assert(t, reflect.DeepEqual(b.ingressPorts, s.expected.ingressPorts), "got: %v - expected: %v", b.ingressPorts, s.expected.ingressPorts)
			assert.Equal(t, b.aggregation, s.expected.aggregation)
			assert.Equal(t, b.eventChannel, s.expected.eventChannel)
			assert.Equal(t, b.headless == nil, s.expected.headless == nil)
			if s.expected.headless != nil {
				assert.Equal(t, b.headless.Name, s.expected.headless.Name)
				assert.Equal(t, b.headless.Size, s.expected.headless.Size)
				assert.Assert(t, reflect.DeepEqual(b.headless.TargetPorts, s.expected.headless.TargetPorts))
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
					assert.Assert(t, reflect.DeepEqual(bv.egressPorts, v.egressPorts))
					assert.Equal(t, bv.selector, v.selector)
					assert.Equal(t, bv.service, v.service)
				}
			}
		})
	}

}

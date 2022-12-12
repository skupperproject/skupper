package service_sync

import (
	vanClient "github.com/skupperproject/skupper/client"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/record"
	"reflect"
	"strings"
	"testing"
	"time"

	"gotest.tools/assert"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/pkg/event"
)

type updateRecord struct {
	changed []types.ServiceInterface
	deleted []string
	origin  string
}

type updateCollector struct {
	updates []updateRecord
}

type updateChannel struct {
	channel chan updateRecord
}

func newUpdateCollector() *updateCollector {
	return &updateCollector{}
}

func newUpdateChannel() *updateChannel {
	return &updateChannel{
		channel: make(chan updateRecord, 5),
	}
}

func (c *updateCollector) handler(changed []types.ServiceInterface, deleted []string, origin string) error {
	c.updates = append(c.updates, updateRecord{changed, deleted, origin})
	return nil
}

func NewMockClient(namespace string) *vanClient.VanClient {
	return &vanClient.VanClient{
		Namespace:     namespace,
		KubeClient:    fake.NewSimpleClientset(),
		EventRecorder: &record.FakeRecorder{},
	}
}

func (c *updateChannel) handler(changed []types.ServiceInterface, deleted []string, origin string) error {
	c.channel <- updateRecord{changed, deleted, origin}
	return nil
}

func TestServiceSync(t *testing.T) {
	scenarios := []struct {
		name  string
		site1 ServiceUpdate
		site2 ServiceUpdate
	}{
		{
			name: "simple",
			site1: ServiceUpdate{
				definitions: map[string]types.ServiceInterface{
					"a": types.ServiceInterface{
						Address:  "a",
						Protocol: "tcp",
						Ports:    []int{8080, 9090},
					},
				},
			},
			site2: ServiceUpdate{
				definitions: map[string]types.ServiceInterface{
					"d": types.ServiceInterface{
						Address:  "d",
						Protocol: "tcp",
						Ports:    []int{8080, 9090},
					},
					"b": types.ServiceInterface{
						Address:  "b",
						Protocol: "http",
						Ports:    []int{6666, 7777, 8888},
					},
				},
			},
		},
		{
			name: "labels and annotations",
			site1: ServiceUpdate{
				definitions: map[string]types.ServiceInterface{
					"a": types.ServiceInterface{
						Address:  "a",
						Protocol: "tcp",
						Ports:    []int{8080, 9090},
						Labels: map[string]string{
							"foo": "bar",
						},
					},
				},
			},
			site2: ServiceUpdate{
				definitions: map[string]types.ServiceInterface{
					"d": types.ServiceInterface{
						Address:  "d",
						Protocol: "tcp",
						Ports:    []int{8080, 9090},
					},
					"b": types.ServiceInterface{
						Address:  "b",
						Protocol: "http",
						Ports:    []int{6666, 7777, 8888},
						Annotations: map[string]string{
							"foo": "bar",
						},
					},
				},
			},
		},
	}

	for _, s := range scenarios {
		stopper := make(chan struct{})
		event.StartDefaultEventStore(stopper)
		t.Run(s.name, func(t *testing.T) {
			//hook up two instances of service sync
			factory := NewMockConnectionFactory("test-channel")
			updates1 := newUpdateChannel()
			updates2 := newUpdateChannel()
			site1 := NewServiceSync("foo", 0, "v1", factory, updates1.handler, NewMockClient("namespace"))
			site2 := NewServiceSync("bar", 0, "v1", factory, updates2.handler, NewMockClient("namespace"))
			site1.Start(stopper)
			site2.Start(stopper)
			factory.topics.newTopic(ServiceSyncAddress).waitForReceivers(2)
			site1.LocalDefinitionsUpdated(s.site1.definitions)
			site2.LocalDefinitionsUpdated(s.site2.definitions)
			site1Done := false
			site2Done := false
			for !(site1Done && site2Done) {
				select {
				case update := <-updates1.channel:
					assert.Equal(t, update.origin, "bar")
					assert.Equal(t, 0, len(update.deleted))
					assert.Equal(t, len(s.site2.definitions), len(update.changed))
					for _, actual := range update.changed {
						expected := s.site2.definitions[actual.Address]
						assert.Equal(t, expected.Address, actual.Address, "Wrong address for %s expected: %v - got: %v", actual.Address, expected.Address, actual.Address)
						assert.Equal(t, expected.Protocol, actual.Protocol, "Wrong protocol for %s expected: %v - got: %v", actual.Address, expected.Protocol, actual.Protocol)
						assert.Equal(t, actual.Origin, update.origin, "Wrong origin for %s expected: %v - got: %v", actual.Address, expected.Origin, actual.Origin)
						assert.Assert(t, reflect.DeepEqual(expected.Ports, actual.Ports), "Wrong ports for key %s expected: %v - got: %v", actual.Address, expected.Ports, actual.Ports)
						assert.Assert(t, reflect.DeepEqual(expected.Headless, actual.Headless), "Wrong headless for key %s expected: %v - got: %v", actual.Address, expected.Headless, actual.Headless)
						assert.Assert(t, reflect.DeepEqual(expected.Labels, actual.Labels), "Wrong labels for key %s expected: %v - got: %v", actual.Address, expected.Labels, actual.Labels)
						assert.Assert(t, reflect.DeepEqual(expected.Annotations, actual.Annotations), "Wrong annotations for key %s expected: %v - got: %v", actual.Address, expected.Annotations, actual.Annotations)
					}
					site1Done = true

				case update := <-updates2.channel:
					assert.Equal(t, update.origin, "foo")
					assert.Equal(t, 0, len(update.deleted))
					assert.Equal(t, len(s.site1.definitions), len(update.changed))
					for _, actual := range update.changed {
						expected := s.site1.definitions[actual.Address]
						assert.Equal(t, expected.Address, actual.Address, "Wrong address for %s expected: %v - got: %v", actual.Address, expected.Address, actual.Address)
						assert.Equal(t, expected.Protocol, actual.Protocol, "Wrong protocol for %s expected: %v - got: %v", actual.Address, expected.Protocol, actual.Protocol)
						assert.Equal(t, actual.Origin, update.origin, "Wrong origin for %s expected: %v - got: %v", actual.Address, expected.Origin, actual.Origin)
						assert.Assert(t, reflect.DeepEqual(expected.Ports, actual.Ports), "Wrong ports for key %s expected: %v - got: %v", actual.Address, expected.Ports, actual.Ports)
						assert.Assert(t, reflect.DeepEqual(expected.Headless, actual.Headless), "Wrong headless for key %s expected: %v - got: %v", actual.Address, expected.Headless, actual.Headless)
						assert.Assert(t, reflect.DeepEqual(expected.Labels, actual.Labels), "Wrong labels for key %s expected: %v - got: %v", actual.Address, expected.Labels, actual.Labels)
						assert.Assert(t, reflect.DeepEqual(expected.Annotations, actual.Annotations), "Wrong annotations for key %s expected: %v - got: %v", actual.Address, expected.Annotations, actual.Annotations)
					}
					site2Done = true

				}
			}
			close(stopper)
		})
	}
}

func TestRemoveStaleDefinitions(t *testing.T) {
	stopper := make(chan struct{})
	event.StartDefaultEventStore(stopper)

	updates := newUpdateCollector()
	factory := NewMockConnectionFactory("test-channel")
	site := NewServiceSync("foo", 0, "v1", factory, updates.handler, NewMockClient("namespace"))

	defs := map[string]types.ServiceInterface{
		"d": types.ServiceInterface{
			Address:  "d",
			Origin:   "bar",
			Protocol: "tcp",
			Ports:    []int{8080, 9090},
		},
		"c": types.ServiceInterface{
			Address:  "c",
			Protocol: "http2",
			Ports:    []int{12345},
		},
		"b": types.ServiceInterface{
			Address:  "b",
			Origin:   "bar",
			Protocol: "http",
			Ports:    []int{6666, 7777, 8888},
		},
		"a": types.ServiceInterface{
			Address:  "a",
			Origin:   "baz",
			Protocol: "tcp",
			Ports:    []int{8080},
		},
	}
	site.localDefinitionsUpdated(defs)

	//artificially age the definitions from bar
	site.heardFrom["bar"] = site.heardFrom["bar"].Add(-120 * time.Second)
	site.removeStaleDefinitions()
	assert.Equal(t, len(updates.updates), 1)
	expected := map[string]bool{"b": true, "d": true}
	assert.Equal(t, len(updates.updates[0].deleted), len(expected))
	for _, name := range updates.updates[0].deleted {
		_, ok := expected[name]
		assert.Assert(t, ok)
	}
}

func TestUpdateRemoteDefinitions(t *testing.T) {
	stopper := make(chan struct{})
	event.StartDefaultEventStore(stopper)

	updates := newUpdateCollector()
	factory := NewMockConnectionFactory("test-channel")
	site := NewServiceSync("foo", 0, "v1", factory, updates.handler, NewMockClient("namespace"))

	defs := map[string]types.ServiceInterface{
		"d": types.ServiceInterface{
			Address:  "d",
			Origin:   "bar",
			Protocol: "tcp",
			Ports:    []int{8080, 9090},
		},
		"c": types.ServiceInterface{
			Address:  "c",
			Protocol: "http2",
			Ports:    []int{12345},
		},
		"b": types.ServiceInterface{
			Address:  "b",
			Origin:   "bar",
			Protocol: "http",
			Ports:    []int{6666, 7777, 8888},
		},
		"a": types.ServiceInterface{
			Address:  "a",
			Origin:   "baz",
			Protocol: "tcp",
			Ports:    []int{8080},
		},
	}
	site.localDefinitionsUpdated(defs)

	update := map[string]types.ServiceInterface{
		"d": types.ServiceInterface{
			Address:  "d",
			Origin:   "bar",
			Protocol: "tcp",
			Ports:    []int{8080, 9091},
		},
		"e": types.ServiceInterface{
			Address:  "e",
			Origin:   "bar",
			Protocol: "http",
			Ports:    []int{6666, 7777, 8888},
		},
	}

	site.updateRemoteDefinitions("bar", update)
	assert.Equal(t, len(updates.updates), 1)
	assert.Equal(t, len(updates.updates[0].changed), 2)
	for _, actual := range updates.updates[0].changed {
		expected := update[actual.Address]
		assert.Equal(t, actual.Address, expected.Address)
		assert.Equal(t, actual.Origin, expected.Origin)
		assert.Equal(t, actual.Protocol, expected.Protocol)
		assert.Assert(t, reflect.DeepEqual(actual.Ports, expected.Ports))
	}
	assert.Equal(t, len(updates.updates[0].deleted), 1)
	assert.Equal(t, updates.updates[0].deleted[0], "b")
}

func TestLocalDefinitionsUpdated(t *testing.T) {
	stopper := make(chan struct{})
	event.StartDefaultEventStore(stopper)

	updates := newUpdateCollector()
	factory := NewMockConnectionFactory("test-channel")
	site := NewServiceSync("foo", 0, "v1", factory, updates.handler, NewMockClient("namespace"))

	defs := map[string]types.ServiceInterface{
		"svc-e": types.ServiceInterface{
			Address:  "svc-e",
			Protocol: "tcp",
			Ports:    []int{54321},
			Headless: &types.Headless{
				Name: "svc-e",
				Size: 1,
			},
		},
		"svc-d": types.ServiceInterface{
			Address:  "svc-d",
			Origin:   "bar",
			Protocol: "tcp",
			Ports:    []int{8080, 9090},
		},
		"svc-c": types.ServiceInterface{
			Address:  "svc-c",
			Protocol: "http2",
			Ports:    []int{12345},
		},
		"svc-b": types.ServiceInterface{
			Address:  "svc-b",
			Origin:   "bar",
			Protocol: "http",
			Ports:    []int{6666, 7777, 8888},
		},
		"svc-a": types.ServiceInterface{
			Address:  "svc-a",
			Origin:   "baz",
			Protocol: "tcp",
			Ports:    []int{8080},
		},
	}
	site.localDefinitionsUpdated(defs)

	update := map[string]types.ServiceInterface{
		"svc-e": types.ServiceInterface{
			Address:  "svc-e",
			Protocol: "tcp",
			Ports:    []int{54321},
			Headless: &types.Headless{
				Name: "svc-e",
				Size: 1,
			},
		},
		"svc-d": types.ServiceInterface{
			Address:  "svc-d",
			Origin:   "bar",
			Protocol: "tcp",
			Ports:    []int{8080, 9091},
		},
		"svc-c": types.ServiceInterface{
			Address:  "svc-c",
			Protocol: "http2",
			Ports:    []int{12345},
		},
		"svc-f": types.ServiceInterface{
			Address:  "svc-f",
			Origin:   "bar",
			Protocol: "http",
			Ports:    []int{6666, 7777, 8888},
		},
	}
	site.localDefinitionsUpdated(update)

	actual := event.Query()
	assert.Equal(t, len(actual), 1)
	assert.Equal(t, actual[0].Name, ServiceSyncEvent)
	assert.Equal(t, actual[0].Total, 4)
	assert.Equal(t, len(actual[0].Counts), 4)
	assert.Equal(t, actual[0].Counts[0].Key, "Service interface(s) modified svc-d")
	assert.Assert(t, strings.HasPrefix(actual[0].Counts[1].Key, "Service interface(s) removed")) //a and b
	assert.Assert(t, strings.Contains(actual[0].Counts[1].Key, "svc-a"))
	assert.Assert(t, strings.Contains(actual[0].Counts[1].Key, "svc-b"))
	assert.Assert(t, !strings.Contains(actual[0].Counts[1].Key, "svc-c"))
	assert.Assert(t, !strings.Contains(actual[0].Counts[1].Key, "svc-d"))
	assert.Assert(t, !strings.Contains(actual[0].Counts[1].Key, "svc-e"))
	assert.Assert(t, !strings.Contains(actual[0].Counts[1].Key, "svc-f"))
	assert.Equal(t, actual[0].Counts[2].Key, "Service interface(s) added svc-f")
	assert.Assert(t, strings.HasPrefix(actual[0].Counts[3].Key, "Service interface(s) added")) //a, b, c, d and e
	assert.Assert(t, strings.Contains(actual[0].Counts[3].Key, "svc-a"))
	assert.Assert(t, strings.Contains(actual[0].Counts[3].Key, "svc-b"))
	assert.Assert(t, strings.Contains(actual[0].Counts[3].Key, "svc-c"))
	assert.Assert(t, strings.Contains(actual[0].Counts[3].Key, "svc-d"))
	assert.Assert(t, strings.Contains(actual[0].Counts[3].Key, "svc-e"))
	assert.Assert(t, !strings.Contains(actual[0].Counts[1].Key, "svc-f"))

	update["svc-e"] = types.ServiceInterface{
		Address:  "svc-e",
		Protocol: "tcp",
		Ports:    []int{54321},
		Headless: &types.Headless{
			Name: "svc-e",
			Size: 3,
		},
	}
	site.localDefinitionsUpdated(update)
	actual = event.Query()
	assert.Equal(t, len(actual), 1)
	assert.Equal(t, actual[0].Counts[0].Key, "Service interface(s) modified svc-e")

	update["svc-e"] = types.ServiceInterface{
		Address:  "svc-e",
		Protocol: "tcp",
		Ports:    []int{54321},
		Headless: nil,
	}
	site.localDefinitionsUpdated(update)
	actual = event.Query()
	assert.Equal(t, len(actual), 1)
	assert.Equal(t, actual[0].Counts[0].Key, "Service interface(s) modified svc-e")

	//test removing origin, i.e. taking local ownership
	update["svc-f"] = types.ServiceInterface{
		Address:  "svc-f",
		Protocol: "http",
		Ports:    []int{6666, 7777, 8888},
	}
	_, ok := site.byOrigin["bar"]["svc-f"]
	assert.Assert(t, ok)
	_, ok = site.localServices["svc-f"]
	assert.Assert(t, !ok)
	site.localDefinitionsUpdated(update)
	_, ok = site.byOrigin["bar"]["svc-f"]
	assert.Assert(t, !ok)
	_, ok = site.localServices["svc-f"]
	assert.Assert(t, ok)
}

package service_sync

import (
	"reflect"
	"testing"

	"gotest.tools/assert"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/pkg/event"
)

func TestEncoding(t *testing.T) {
	stopper := make(chan struct{})
	event.StartDefaultEventStore(stopper)
	scenarios := []struct {
		name   string
		update ServiceUpdate
	}{
		{
			name: "simple",
			update: ServiceUpdate{
				origin:  "foo",
				version: "bar",
				definitions: map[string]types.ServiceInterface{
					"a": types.ServiceInterface{
						Address:  "a",
						Protocol: "tcp",
						Ports:    []int{8080, 9090},
					},
				},
			},
		},
		{
			name: "headless",
			update: ServiceUpdate{
				origin:  "foo",
				version: "bar",
				definitions: map[string]types.ServiceInterface{
					"a": types.ServiceInterface{
						Address:  "a",
						Protocol: "tcp",
						Ports:    []int{8080, 9090},
						Headless: &types.Headless{
							Name: "baz",
							Size: 3,
						},
					},
				},
			},
		},
		{
			name: "multiple",
			update: ServiceUpdate{
				origin:  "foo",
				version: "bar",
				definitions: map[string]types.ServiceInterface{
					"a": types.ServiceInterface{
						Address:  "a",
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
	}

	for _, s := range scenarios {
		t.Run(s.name, func(t *testing.T) {
			msg, err := encode(&s.update)
			assert.Assert(t, err)
			assert.Equal(t, msg.Properties.Subject, serviceSyncSubjectV2)
			assert.Equal(t, msg.ApplicationProperties["origin"], s.update.origin)
			assert.Equal(t, msg.ApplicationProperties["version"], s.update.version)
			update, err := decode(msg)
			assert.Assert(t, err)
			assert.Equal(t, update.origin, s.update.origin)
			assert.Equal(t, update.version, s.update.version)
			assert.Equal(t, len(update.definitions), len(s.update.definitions))
			for key, actual := range update.definitions {
				expected := s.update.definitions[key]
				assert.Equal(t, expected.Address, actual.Address, "Wrong address for %s expected: %v - got: %v", key, expected.Address, actual.Address)
				assert.Equal(t, expected.Protocol, actual.Protocol, "Wrong protocol for %s expected: %v - got: %v", key, expected.Protocol, actual.Protocol)
				assert.Equal(t, actual.Origin, s.update.origin, "Wrong origin for %s expected: %v - got: %v", key, s.update.origin, actual.Origin)
				assert.Assert(t, reflect.DeepEqual(expected.Ports, actual.Ports), "Wrong ports for key %s expected: %v - got: %v", key, expected.Ports, actual.Ports)
				assert.Assert(t, reflect.DeepEqual(expected.Headless, actual.Headless), "Wrong headless for key %s expected: %v - got: %v", key, expected.Ports, actual.Ports)
			}
		})
	}
}

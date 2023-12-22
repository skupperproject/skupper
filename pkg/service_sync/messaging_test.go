package service_sync

import (
	"reflect"
	"testing"

	"gotest.tools/assert"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/pkg/event"
	"github.com/skupperproject/skupper/pkg/messaging"
)

func TestSender(t *testing.T) {
	stopper := make(chan struct{})
	event.StartDefaultEventStore(stopper)
	scenarios := []struct {
		name    string
		updates []ServiceUpdate
	}{
		{
			name: "single",
			updates: []ServiceUpdate{
				{
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
		},
		{
			name: "multiple",
			updates: []ServiceUpdate{
				{
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
				{
					origin:  "baz",
					version: "bar",
					definitions: map[string]types.ServiceInterface{
						"b": types.ServiceInterface{
							Address:  "b",
							Protocol: "http",
							Ports:    []int{6666, 7777},
						},
					},
				},
			},
		},
	}

	for _, s := range scenarios {
		t.Run(s.name, func(t *testing.T) {
			factory := messaging.NewMockConnectionFactory(t, "test-channel")
			outgoing := make(chan ServiceUpdate)
			sender := newSender(factory, outgoing, event.NewDefaultEventLogger())
			sender.start()
			conn, err := factory.Connect()
			assert.Assert(t, err)
			receiver, err := conn.Receiver(ServiceSyncAddress, 1)
			assert.Assert(t, err)

			for _, update := range s.updates {
				outgoing <- update
				msg, err := receiver.Receive()
				assert.Assert(t, err)
				received, err := decode(msg)

				assert.Assert(t, err)
				assert.Equal(t, msg.Properties.Subject, serviceSyncSubjectV2)
				assert.Equal(t, msg.ApplicationProperties["origin"], received.origin)
				assert.Equal(t, msg.ApplicationProperties["version"], received.version)
				update, err := decode(msg)
				assert.Assert(t, err)
				assert.Equal(t, update.origin, received.origin)
				assert.Equal(t, update.version, received.version)
				assert.Equal(t, len(update.definitions), len(received.definitions))
				for key, actual := range received.definitions {
					expected := update.definitions[key]
					assert.Equal(t, expected.Address, actual.Address, "Wrong address for %s expected: %v - got: %v", key, expected.Address, actual.Address)
					assert.Equal(t, expected.Protocol, actual.Protocol, "Wrong protocol for %s expected: %v - got: %v", key, expected.Protocol, actual.Protocol)
					assert.Equal(t, actual.Origin, received.origin, "Wrong origin for %s expected: %v - got: %v", key, expected.Origin, actual.Origin)
					assert.Assert(t, reflect.DeepEqual(expected.Ports, actual.Ports), "Wrong ports for key %s expected: %v - got: %v", key, expected.Ports, actual.Ports)
					assert.Assert(t, reflect.DeepEqual(expected.Headless, actual.Headless), "Wrong headless for key %s expected: %v - got: %v", key, expected.Ports, actual.Ports)
				}
			}
			sender.stop()
		})
	}
}

func TestReceiver(t *testing.T) {
	stopper := make(chan struct{})
	event.StartDefaultEventStore(stopper)
	scenarios := []struct {
		name    string
		updates []ServiceUpdate
	}{
		{
			name: "single",
			updates: []ServiceUpdate{
				{
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
		},
		{
			name: "multiple",
			updates: []ServiceUpdate{
				{
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
				{
					origin:  "baz",
					version: "bar",
					definitions: map[string]types.ServiceInterface{
						"b": types.ServiceInterface{
							Address:  "b",
							Protocol: "http",
							Ports:    []int{6666, 7777},
						},
					},
				},
			},
		},
	}

	for _, s := range scenarios {
		t.Run(s.name, func(t *testing.T) {
			factory := messaging.NewMockConnectionFactory(t, "test-channel")
			incoming := make(chan ServiceUpdate)
			receiver := newReceiver(factory, incoming, event.NewDefaultEventLogger())
			receiver.start()
			factory.Broker.AwaitReceivers(ServiceSyncAddress, 1)

			tConn, err := factory.Connect()
			assert.Assert(t, err)
			serviceSyncSender, err := tConn.Sender(ServiceSyncAddress)
			assert.Assert(t, err)

			for _, update := range s.updates {
				msg, err := encode(&update)
				assert.Assert(t, err)
				serviceSyncSender.Send(msg)
				received := <-incoming

				assert.Assert(t, err)
				assert.Equal(t, msg.Properties.Subject, serviceSyncSubjectV2)
				assert.Equal(t, msg.ApplicationProperties["origin"], received.origin)
				assert.Equal(t, msg.ApplicationProperties["version"], received.version)
				update, err := decode(msg)
				assert.Assert(t, err)
				assert.Equal(t, update.origin, received.origin)
				assert.Equal(t, update.version, received.version)
				assert.Equal(t, len(update.definitions), len(received.definitions))
				for key, actual := range received.definitions {
					expected := update.definitions[key]
					assert.Equal(t, expected.Address, actual.Address, "Wrong address for %s expected: %v - got: %v", key, expected.Address, actual.Address)
					assert.Equal(t, expected.Protocol, actual.Protocol, "Wrong protocol for %s expected: %v - got: %v", key, expected.Protocol, actual.Protocol)
					assert.Equal(t, actual.Origin, received.origin, "Wrong origin for %s expected: %v - got: %v", key, expected.Origin, actual.Origin)
					assert.Assert(t, reflect.DeepEqual(expected.Ports, actual.Ports), "Wrong ports for key %s expected: %v - got: %v", key, expected.Ports, actual.Ports)
					assert.Assert(t, reflect.DeepEqual(expected.Headless, actual.Headless), "Wrong headless for key %s expected: %v - got: %v", key, expected.Ports, actual.Ports)
				}
			}
			receiver.stop()
		})
	}
}

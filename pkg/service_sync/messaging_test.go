package service_sync

import (
	"fmt"
	"reflect"
	"testing"

	amqp "github.com/interconnectedcloud/go-amqp"
	"gotest.tools/assert"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/pkg/event"
	"github.com/skupperproject/skupper/pkg/messaging"
)

type multicast struct {
	receivers []*MockReceiver
	channel   chan int
}

func newMulticast() *multicast {
	return &multicast{
		channel: make(chan int, 5),
	}
}

func (m *multicast) send(msg *amqp.Message) {
	for _, r := range m.receivers {
		r.send(msg)
	}
}

func (m *multicast) subscribe(r *MockReceiver) {
	m.receivers = append(m.receivers, r)
	m.channel <- len(m.receivers)
}

func (m *multicast) waitForReceivers(count int) {
	for len(m.receivers) < count {
		<-m.channel
	}
}

type broker struct {
	topics map[string]*multicast
}

func newBroker() *broker {
	return &broker{
		topics: map[string]*multicast{},
	}
}

func (b *broker) newTopic(address string) *multicast {
	t := newMulticast()
	b.topics[address] = t
	return t
}

func (b *broker) subscribe(address string, r *MockReceiver) {
	t, ok := b.topics[address]
	if !ok {
		t = newMulticast()
		b.topics[address] = t
	}
	t.subscribe(r)
}

func (b *broker) send(address string, msg *amqp.Message) {
	if t, ok := b.topics[address]; ok {
		t.send(msg)
	}
}

type MockConnectionFactory struct {
	url    string
	topics *broker
}

func NewMockConnectionFactory(url string) *MockConnectionFactory {
	return &MockConnectionFactory{
		url:    url,
		topics: newBroker(),
	}
}

func (f *MockConnectionFactory) Connect() (messaging.Connection, error) {
	c := &MockConnection{topics: f.topics, channel: make(chan bool)}
	return c, nil
}

func (f *MockConnectionFactory) Url() string {
	return f.url
}

type MockConnection struct {
	topics  *broker
	channel chan bool
	closed  bool
}

func (c *MockConnection) Sender(address string) (messaging.Sender, error) {
	return &MockSender{address: address, connection: c}, nil
}

func (c *MockConnection) Receiver(address string, credit uint32) (messaging.Receiver, error) {
	r := &MockReceiver{connection: c, channel: make(chan *amqp.Message, credit)}
	c.topics.subscribe(address, r)
	return r, nil
}

func (c *MockConnection) Close() {
	if !c.closed {
		c.closed = true
		close(c.channel)
	}
}

func (c *MockConnection) send(address string, msg *amqp.Message) error {
	if c.closed {
		return fmt.Errorf("Channel closed")
	}
	c.topics.send(address, msg)
	return nil
}

func (c *MockConnection) receive(r *MockReceiver) (*amqp.Message, error) {
	if c.closed {
		return nil, fmt.Errorf("Channel closed")
	}
	select {
	case msg := <-r.channel:
		if msg == nil {
			return nil, fmt.Errorf("Failed to receive")
		}
		return msg, nil
	case <-c.channel:
		return nil, fmt.Errorf("Channel closed")
	}
}

type MockSender struct {
	connection *MockConnection
	address    string
}

func (s *MockSender) Send(msg *amqp.Message) error {
	return s.connection.send(s.address, msg)
}

func (s *MockSender) Close() error {
	return nil
}

type MockReceiver struct {
	connection *MockConnection
	channel    chan *amqp.Message
	closed     bool
}

func (r *MockReceiver) send(msg *amqp.Message) {
	if !r.closed {
		r.channel <- msg
	}
}

func (r *MockReceiver) Receive() (*amqp.Message, error) {
	return r.connection.receive(r)
}

func (r *MockReceiver) Accept(msg *amqp.Message) error {
	return nil
}

func (r *MockReceiver) Close() error {
	if !r.closed {
		close(r.channel)
		r.closed = true
	}
	return nil
}

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
			factory := NewMockConnectionFactory("test-channel")
			outgoing := make(chan ServiceUpdate)
			sender := newSender(factory, outgoing, NewMockClient("test"))
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
			factory := NewMockConnectionFactory("test-channel")
			incoming := make(chan ServiceUpdate)
			receiver := newReceiver(factory, incoming, NewMockClient("test"))
			receiver.start()
			factory.topics.newTopic(ServiceSyncAddress).waitForReceivers(1)

			for _, update := range s.updates {
				msg, err := encode(&update)
				assert.Assert(t, err)
				factory.topics.send(ServiceSyncAddress, msg)
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

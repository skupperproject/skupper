package session

import (
	"context"
	"fmt"
	"sync"

	"github.com/Azure/go-amqp"
)

// NewMockContainer creates a mock container using the mock router
func NewMockContainer(router *MockRouter) Container {
	return &mockContainer{
		Router: router,
	}
}

type mockContainer struct {
	Router *MockRouter
}

func (c *mockContainer) Start(ctx context.Context) {
}

func (c *mockContainer) OnSessionError(func(error)) {
}

func (c *mockContainer) NewReceiver(address string, opts ReceiverOptions) Receiver {
	cred := opts.Credit
	if cred <= 0 {
		cred = 256
	}
	rcv := &mockReceiver{
		channel: make(chan *amqp.Message, cred),
		done:    make(chan struct{}),
	}
	c.Router.subscribe(address, rcv)
	return rcv
}

func (c *mockContainer) NewSender(address string, opts SenderOptions) Sender {
	return &mockSender{
		router:  c.Router,
		address: address,
		done:    make(chan struct{}),
	}
}

// NewMockRouter returns a mock router that can be used by containers to send
// and receive multicast messages. Behaves like a very rough approximation of
// the skupper router for routing vanflow messages.
func NewMockRouter() *MockRouter {
	return &MockRouter{
		topics: make(map[string]*multicast),
	}
}

type MockRouter struct {
	mu     sync.Mutex
	topics map[string]*multicast
}

func (router *MockRouter) subscribe(address string, r *mockReceiver) {
	router.get(address).subscribe(r)
}

func (router *MockRouter) send(ctx context.Context, address string, msg *amqp.Message) error {
	return router.get(address).send(ctx, msg)
}

func (router *MockRouter) get(address string) *multicast {
	router.mu.Lock()
	defer router.mu.Unlock()
	t, ok := router.topics[address]
	if !ok {
		t = newMulticast()
		router.topics[address] = t
	}
	return t
}

type multicast struct {
	mu        sync.Mutex
	receivers []*mockReceiver
	update    chan struct{}
}

func newMulticast() *multicast {
	return &multicast{update: make(chan struct{})}
}

func (m *multicast) send(ctx context.Context, msg *amqp.Message) error {
	m.mu.Lock()
	update := m.update
	var sent bool
	for _, r := range m.receivers {
		sent = true
		r.send(msg)
	}
	m.mu.Unlock()
	if !sent {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-update:
			return m.send(ctx, msg)
		}
	}
	return nil
}

func (m *multicast) subscribe(r *mockReceiver) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.receivers = append(m.receivers, r)
	close(m.update)
	m.update = make(chan struct{})
}

type mockReceiver struct {
	channel   chan *amqp.Message
	closeOnce sync.Once
	done      chan struct{}
}

func (r *mockReceiver) send(msg *amqp.Message) {
	select {
	case <-r.done:
		return
	default:
		r.channel <- msg
	}
}
func (r *mockReceiver) Next(ctx context.Context) (*amqp.Message, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-r.done:
		return nil, fmt.Errorf("closed")
	case msg := <-r.channel:
		return msg, nil
	}
}

func (r *mockReceiver) Accept(context.Context, *amqp.Message) error {
	return nil
}

func (r *mockReceiver) Close(context.Context) error {
	r.closeOnce.Do(func() { close(r.done) })
	return nil
}

type mockSender struct {
	router    *MockRouter
	address   string
	closeOnce sync.Once
	done      chan struct{}
}

func (s *mockSender) Send(ctx context.Context, msg *amqp.Message) error {
	select {
	case <-s.done:
		return fmt.Errorf("sender closed")
	case <-ctx.Done():
		return ctx.Err()
	default:
		return s.router.send(ctx, s.address, msg)
	}
}

func (s *mockSender) Close(context.Context) error {
	s.closeOnce.Do(func() { close(s.done) })
	return nil
}

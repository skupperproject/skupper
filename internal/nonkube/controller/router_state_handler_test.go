package controller

import (
	"fmt"
	"log/slog"
	"os"
	"sync/atomic"
	"testing"
	"time"

	amqp "github.com/interconnectedcloud/go-amqp"
	"github.com/skupperproject/skupper/internal/messaging"
	"github.com/skupperproject/skupper/internal/qdr"
	"github.com/skupperproject/skupper/internal/utils"
	"github.com/skupperproject/skupper/pkg/apis/skupper/v2alpha1"
	"github.com/skupperproject/skupper/pkg/nonkube/api"
	"gotest.tools/v3/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestRouterStateHandler(t *testing.T) {
	tempDir := t.TempDir()
	if os.Getuid() == 0 {
		api.DefaultRootDataHome = tempDir
	} else {
		t.Setenv("XDG_DATA_HOME", tempDir)
	}
	namespacesPath := api.GetDefaultOutputNamespacesPath()
	assert.Assert(t, os.MkdirAll(namespacesPath, 0755))
	namespace := "test-router-state-handler"
	runtimePath := api.GetInternalOutputPath(namespace, api.RuntimeSiteStatePath)
	assert.Assert(t, os.MkdirAll(runtimePath, 0755))

	// Site state
	siteState := &api.SiteState{
		SiteId: "site-id",
		Site: &v2alpha1.Site{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Site",
				APIVersion: "skupper.io/v2alpha1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: "site-name",
			},
			Spec: v2alpha1.SiteSpec{},
		},
		RouterAccesses: map[string]*v2alpha1.RouterAccess{
			"skupper-local": {
				TypeMeta: metav1.TypeMeta{
					Kind:       "RouterAccess",
					APIVersion: "skupper.io/v2alpha1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "skupper-local",
				},
				Spec: v2alpha1.RouterAccessSpec{
					Roles: []v2alpha1.RouterAccessRole{
						{
							Name: "normal",
							Port: 5671,
						},
					},
					TlsCredentials: "skupper-local",
					BindHost:       "127.0.0.1",
					SubjectAlternativeNames: []string{
						"localhost",
					},
				},
			},
		},
	}
	assert.Assert(t, api.MarshalSiteState(*siteState, runtimePath))

	routerStateHandler := NewRouterStateHandler(namespace)
	routerLogHandler := &testLogHandler{
		handler: routerStateHandler.logger.Handler(),
	}
	routerStateHandler.logger = slog.New(routerLogHandler)
	routerStateHandler.heartbeat = newHeartBeatsClient(namespace)
	heartbeatLogHandler := &testLogHandler{
		handler: routerStateHandler.heartbeat.logger.Handler(),
	}
	routerStateHandler.heartbeat.logger = slog.New(heartbeatLogHandler)
	var mockFactory *mockConnectionFactory
	routerStateHandler.heartbeat.factory = func(s string, tls qdr.TlsConfigRetriever) messaging.ConnectionFactory {
		mockFactory = &mockConnectionFactory{
			url: s,
			tls: tls,
		}
		return mockFactory
	}
	callback := &routerStateCallback{}
	routerStateHandler.AddCallback(callback)
	stopCh := make(chan struct{})

	t.Run("start-router-state-handler", func(t *testing.T) {
		routerStateHandler.Start(stopCh)
		assert.Assert(t, utils.Retry(delaySecs*time.Second, attempts, func() (bool, error) {
			return callback.started.Load() == int32(1), nil
		}), "callback Start() method not called")
		assert.Equal(t, len(mockFactory.connection.receivers), 1)
		assert.Equal(t, routerLogHandler.Count("Starting"), 1)
		assert.Equal(t, heartbeatLogHandler.Count("Starting"), 1)
	})

	t.Run("force-receiver-error", func(t *testing.T) {
		for _, r := range mockFactory.connection.receivers {
			mr := r.(*mockReceiver)
			mr.channel <- nil
		}
		assert.Assert(t, utils.Retry(delaySecs*time.Second, attempts, func() (bool, error) {
			return callback.stopped.Load() == int32(1), nil
		}))
	})

	t.Run("receiver-recovered", func(t *testing.T) {
		assert.Assert(t, utils.Retry(delaySecs*time.Second, attempts, func() (bool, error) {
			return callback.started.Load() == int32(2), nil
		}))
	})

	t.Run("stop-router-state-handler", func(t *testing.T) {
		routerStateHandler.Stop()
		assert.Assert(t, utils.Retry(delaySecs*time.Second, attempts, func() (bool, error) {
			h := routerStateHandler.heartbeat
			h.mutex.Lock()
			defer h.mutex.Unlock()
			return !h.running, nil
		}))
		assert.Assert(t, utils.Retry(delaySecs*time.Second, attempts, func() (bool, error) {
			return callback.stopped.Load() == int32(2), nil
		}))
		assert.Assert(t, utils.Retry(delaySecs*time.Second, attempts, func() (bool, error) {
			return routerLogHandler.Count("Stopping") == 1, nil
		}))
		assert.Assert(t, utils.Retry(delaySecs*time.Second, attempts, func() (bool, error) {
			return routerLogHandler.Count("Stopped") == 1, nil
		}))
		assert.Assert(t, utils.Retry(delaySecs*time.Second, attempts, func() (bool, error) {
			return heartbeatLogHandler.Count("Exiting") == 1, nil
		}))
	})

	t.Run("restart-router-state-handler", func(t *testing.T) {
		routerStateHandler.Start(stopCh)
		assert.Assert(t, utils.Retry(delaySecs*time.Second, attempts, func() (bool, error) {
			return callback.started.Load() == int32(3), nil
		}))
		assert.Equal(t, routerLogHandler.Count("Starting"), 2)
		assert.Equal(t, heartbeatLogHandler.Count("Starting"), 2)
	})

	t.Run("closing-stop-channel", func(t *testing.T) {
		assert.Equal(t, callback.stopped.Load(), int32(2))
		close(stopCh)
		assert.Assert(t, utils.Retry(delaySecs*time.Second, attempts, func() (bool, error) {
			h := routerStateHandler.heartbeat
			h.mutex.Lock()
			defer h.mutex.Unlock()
			return !h.running, nil
		}))
		assert.Assert(t, utils.Retry(delaySecs*time.Second, attempts, func() (bool, error) {
			return callback.stopped.Load() == int32(3), nil
		}))
		assert.Assert(t, utils.Retry(delaySecs*time.Second, attempts, func() (bool, error) {
			return routerLogHandler.Count("Parent channel closed") == 1, nil
		}))
		assert.Assert(t, utils.Retry(delaySecs*time.Second, attempts, func() (bool, error) {
			return routerLogHandler.Count("Stopping") == 2, nil
		}))
		assert.Assert(t, utils.Retry(delaySecs*time.Second, attempts, func() (bool, error) {
			return routerLogHandler.Count("Stopped") == 2, nil
		}))
		assert.Assert(t, utils.Retry(delaySecs*time.Second, attempts, func() (bool, error) {
			return heartbeatLogHandler.Count("Exiting") == 2, nil
		}))
	})
}

type mockConnectionFactory struct {
	url        string
	tls        qdr.TlsConfigRetriever
	connection *mockConnection
}

func (m *mockConnectionFactory) Connect() (messaging.Connection, error) {
	m.connection = &mockConnection{
		senders:   map[string]messaging.Sender{},
		receivers: map[string]messaging.Receiver{},
		closed:    false,
	}
	return m.connection, nil
}

func (m *mockConnectionFactory) Url() string {
	return m.url
}

type mockConnection struct {
	senders   map[string]messaging.Sender
	receivers map[string]messaging.Receiver
	closed    bool
}

func (m *mockConnection) Sender(address string) (messaging.Sender, error) {
	return nil, nil
}

func (m *mockConnection) Receiver(address string, credit uint32) (messaging.Receiver, error) {
	if r, ok := m.receivers[address]; ok {
		return r, nil
	}
	r := &mockReceiver{
		channel: make(chan *amqp.Message),
	}
	m.receivers[address] = r
	return r, nil
}

func (m *mockConnection) Close() {
}

type mockReceiver struct {
	channel chan *amqp.Message
	closed  bool
}

func (m *mockReceiver) Receive() (*amqp.Message, error) {
	msg := <-m.channel
	if msg == nil {
		return nil, fmt.Errorf("channel closed")
	}
	return msg, nil
}

func (m *mockReceiver) Accept(message *amqp.Message) error {
	return nil
}

func (m *mockReceiver) Close() error {
	if m.closed {
		return fmt.Errorf("receiver already closed")
	}
	close(m.channel)
	m.closed = true
	return nil
}

type routerStateCallback struct {
	started atomic.Int32
	stopped atomic.Int32
}

func (t *routerStateCallback) Start(stopCh <-chan struct{}) {
	t.started.Add(1)
}

func (t *routerStateCallback) Stop() {
	t.stopped.Add(1)
}

func (t *routerStateCallback) Id() string {
	return "router-state-callback"
}

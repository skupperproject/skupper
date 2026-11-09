package adaptor

import (
	"context"
	"fmt"
	"net"
	"testing"

	"gotest.tools/v3/assert"
	"k8s.io/apimachinery/pkg/api/meta"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	fakeclient "github.com/skupperproject/skupper/internal/kube/client/fake"
	"github.com/skupperproject/skupper/internal/kube/watchers"
	"github.com/skupperproject/skupper/internal/qdr"
	skupperv2alpha1 "github.com/skupperproject/skupper/pkg/apis/skupper/v2alpha1"
)

var healthCheckEnabled = map[string]string{"enable-health-check": "true"}

func healthConnector(name, host string, port int, settings map[string]string) *skupperv2alpha1.Connector {
	return &skupperv2alpha1.Connector{
		ObjectMeta: v1.ObjectMeta{Name: name, Namespace: "test"},
		Spec: skupperv2alpha1.ConnectorSpec{
			RoutingKey: name,
			Host:       host,
			Port:       port,
			Settings:   settings,
		},
	}
}

func newTestConnectorHealth(t *testing.T, onChange func(), connectors ...*skupperv2alpha1.Connector) *ConnectorHealth {
	t.Helper()
	objects := make([]runtime.Object, len(connectors))
	for i, connector := range connectors {
		objects[i] = connector
	}
	client, err := fakeclient.NewFakeClient("test", nil, objects, "")
	assert.NilError(t, err)
	h := newConnectorHealth(client, watchers.NewEventProcessor("test", client), "test", onChange)
	h.bindings.SetHealthChangeCallback(h.healthChanged)
	for _, connector := range connectors {
		assert.NilError(t, h.connectorEvent("test/"+connector.Name, connector))
	}
	return h
}

func listenLocal(t *testing.T, address string) net.Listener {
	t.Helper()
	listener, err := net.Listen("tcp", address)
	assert.NilError(t, err)
	t.Cleanup(func() { listener.Close() })
	return listener
}

func bridgeWith(names ...string) qdr.BridgeConfig {
	bridges := qdr.BridgeConfig{TcpConnectors: qdr.TcpEndpointMap{}}
	for _, name := range names {
		bridges.TcpConnectors[name] = qdr.TcpEndpoint{Name: name}
	}
	return bridges
}

func isKept(h *ConnectorHealth, tcpName string) bool {
	bridges := bridgeWith(tcpName)
	h.filter(&bridges)
	_, ok := bridges.TcpConnectors[tcpName]
	return ok
}

func TestConnectorHealth_Filter(t *testing.T) {
	reachable := listenLocal(t, "127.0.0.1:0").Addr().(*net.TCPAddr).Port
	closed := listenLocal(t, "127.0.0.1:0")
	unreachable := closed.Addr().(*net.TCPAddr).Port
	assert.NilError(t, closed.Close())

	tests := []struct {
		name      string
		connector *skupperv2alpha1.Connector
		check     bool
		expected  bool
	}{
		{
			name:      "health check enabled and not yet checked",
			connector: healthConnector("backend", "127.0.0.1", reachable, healthCheckEnabled),
			expected:  false,
		},
		{
			name:      "health check enabled and unreachable",
			connector: healthConnector("backend", "127.0.0.1", unreachable, healthCheckEnabled),
			check:     true,
			expected:  false,
		},
		{
			name:      "health check enabled and reachable",
			connector: healthConnector("backend", "127.0.0.1", reachable, healthCheckEnabled),
			check:     true,
			expected:  true,
		},
		{
			name:      "health check disabled",
			connector: healthConnector("backend", "127.0.0.1", unreachable, nil),
			check:     true,
			expected:  true,
		},
		{
			name: "connector with selector",
			connector: &skupperv2alpha1.Connector{
				ObjectMeta: v1.ObjectMeta{Name: "backend", Namespace: "test"},
				Spec:       skupperv2alpha1.ConnectorSpec{Selector: "app=backend", Port: unreachable, Settings: healthCheckEnabled},
			},
			check:    true,
			expected: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newTestConnectorHealth(t, nil, tt.connector)
			if tt.check {
				h.bindings.PerformHealthChecks()
			}
			tcpName := qdr.TcpConnectorNamePrefix + "backend@" + tt.connector.Spec.Host
			otherName := qdr.TcpConnectorNamePrefix + "other@other.host"
			bridges := bridgeWith(tcpName, otherName)
			h.filter(&bridges)
			_, kept := bridges.TcpConnectors[tcpName]
			assert.Equal(t, kept, tt.expected)
			_, otherKept := bridges.TcpConnectors[otherName]
			assert.Assert(t, otherKept)
		})
	}
}

func TestConnectorHealth_RemovesAndRestoresOnReachability(t *testing.T) {
	listener := listenLocal(t, "127.0.0.1:0")
	address := listener.Addr().String()
	port := listener.Addr().(*net.TCPAddr).Port
	changes := 0
	h := newTestConnectorHealth(t, func() { changes++ }, healthConnector("backend", "127.0.0.1", port, healthCheckEnabled))
	tcpName := qdr.TcpConnectorNamePrefix + "backend@127.0.0.1"
	assert.Equal(t, changes, 1)
	changes = 0

	assert.Assert(t, !isKept(h, tcpName))

	h.bindings.PerformHealthChecks()
	assert.Equal(t, changes, 1)
	assert.Assert(t, isKept(h, tcpName))

	assert.NilError(t, listener.Close())
	h.bindings.PerformHealthChecks()
	assert.Equal(t, changes, 2)
	assert.Assert(t, !isKept(h, tcpName))

	listenLocal(t, address)
	h.bindings.PerformHealthChecks()
	assert.Equal(t, changes, 3)
	assert.Assert(t, isKept(h, tcpName))
}

func TestConnectorHealth_DeletedConnectorIsNotFiltered(t *testing.T) {
	connector := healthConnector("backend", "backend.west.svc", 8080, healthCheckEnabled)
	h := newTestConnectorHealth(t, nil, connector)
	tcpName := qdr.TcpConnectorNamePrefix + "backend@backend.west.svc"
	assert.Assert(t, !isKept(h, tcpName))

	assert.NilError(t, h.connectorEvent("test/backend", nil))

	assert.Assert(t, isKept(h, tcpName))
}

func configuredCondition(t *testing.T, h *ConnectorHealth, name string) *v1.Condition {
	t.Helper()
	connector, err := h.clients.GetSkupperClient().SkupperV2alpha1().Connectors("test").Get(context.TODO(), name, v1.GetOptions{})
	assert.NilError(t, err)
	return meta.FindStatusCondition(connector.Status.Conditions, skupperv2alpha1.CONDITION_TYPE_CONFIGURED)
}

func TestConnectorHealth_StatusIsWrittenOnlyByLeader(t *testing.T) {
	listener := listenLocal(t, "127.0.0.1:0")
	address := listener.Addr().String()
	port := listener.Addr().(*net.TCPAddr).Port
	notReachable := fmt.Sprintf("Target host 127.0.0.1:%d is not reachable", port)
	h := newTestConnectorHealth(t, nil, healthConnector("backend", "127.0.0.1", port, healthCheckEnabled))

	h.bindings.PerformHealthChecks()
	assert.NilError(t, listener.Close())
	h.bindings.PerformHealthChecks()
	assert.Assert(t, configuredCondition(t, h, "backend") == nil, "non-leader must not write status")

	h.becomeLeader()
	condition := configuredCondition(t, h, "backend")
	assert.Assert(t, condition != nil, "new leader must reconcile unreachable connectors")
	assert.Equal(t, condition.Status, v1.ConditionFalse)
	assert.Equal(t, condition.Message, notReachable)

	listener = listenLocal(t, address)
	h.bindings.PerformHealthChecks()
	assert.Equal(t, configuredCondition(t, h, "backend").Status, v1.ConditionTrue)

	assert.NilError(t, listener.Close())
	h.bindings.PerformHealthChecks()
	assert.Equal(t, configuredCondition(t, h, "backend").Message, notReachable)

	h.leader.Store(false)
	listenLocal(t, address)
	h.bindings.PerformHealthChecks()
	assert.Equal(t, configuredCondition(t, h, "backend").Message, notReachable, "non-leader must not clear status")
}

func TestConnectorHealth_LeaderReassertsStatusOverwrittenByController(t *testing.T) {
	listener := listenLocal(t, "127.0.0.1:0")
	port := listener.Addr().(*net.TCPAddr).Port
	connector := healthConnector("backend", "127.0.0.1", port, healthCheckEnabled)
	h := newTestConnectorHealth(t, nil, connector)
	h.leader.Store(true)
	h.bindings.PerformHealthChecks()
	assert.NilError(t, listener.Close())
	h.bindings.PerformHealthChecks()
	assert.Equal(t, configuredCondition(t, h, "backend").Status, v1.ConditionFalse)

	stored, err := h.clients.GetSkupperClient().SkupperV2alpha1().Connectors("test").Get(context.TODO(), "backend", v1.GetOptions{})
	assert.NilError(t, err)
	assert.Assert(t, stored.SetConfigured(nil))
	_, err = h.clients.GetSkupperClient().SkupperV2alpha1().Connectors("test").UpdateStatus(context.TODO(), stored, v1.UpdateOptions{})
	assert.NilError(t, err)
	assert.Equal(t, configuredCondition(t, h, "backend").Status, v1.ConditionTrue)

	assert.NilError(t, h.connectorEvent("test/backend", stored))

	assert.Equal(t, configuredCondition(t, h, "backend").Status, v1.ConditionFalse)
}

func TestConnectorHealth_CreatedWhileUnreachableSetsStatus(t *testing.T) {
	closed := listenLocal(t, "127.0.0.1:0")
	port := closed.Addr().(*net.TCPAddr).Port
	assert.NilError(t, closed.Close())

	notReachable := fmt.Sprintf("Target host 127.0.0.1:%d is not reachable", port)
	connector := healthConnector("backend", "127.0.0.1", port, healthCheckEnabled)
	h := newTestConnectorHealth(t, nil, connector)
	h.leader.Store(true)

	h.bindings.PerformHealthChecks()
	condition := configuredCondition(t, h, "backend")
	assert.Assert(t, condition != nil, "first probe failure must write status")
	assert.Equal(t, condition.Status, v1.ConditionFalse)
	assert.Equal(t, condition.Message, notReachable)
}

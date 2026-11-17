package site

import (
	"net"
	"testing"

	"gotest.tools/v3/assert"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/skupperproject/skupper/internal/qdr"
	skupperv2alpha1 "github.com/skupperproject/skupper/pkg/apis/skupper/v2alpha1"
)

func TestBindings_UnhealthyConnectorIsNotConfigured(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	assert.NilError(t, err)
	defer listener.Close()
	port := listener.Addr().(*net.TCPAddr).Port

	newConnector := func(name string, settings map[string]string) *skupperv2alpha1.Connector {
		return &skupperv2alpha1.Connector{
			ObjectMeta: v1.ObjectMeta{Name: name, Namespace: "test"},
			Spec: skupperv2alpha1.ConnectorSpec{
				RoutingKey: name,
				Host:       "127.0.0.1",
				Port:       port,
				Type:       "tcp",
				Settings:   settings,
			},
		}
	}
	bindings := NewBindings("")
	bindings.SetSiteId("site-1")
	bindings.UpdateConnector("checked", newConnector("checked", map[string]string{"enable-health-check": "true"}))
	bindings.UpdateConnector("unchecked", newConnector("unchecked", nil))

	configured := func() qdr.TcpEndpointMap {
		return bindings.ToBridgeConfig().TcpConnectors
	}

	_, ok := configured()[qdr.TcpConnectorNamePrefix+"checked@127.0.0.1"]
	assert.Assert(t, !ok, "connector with health check must not be configured before the first check")
	_, ok = configured()[qdr.TcpConnectorNamePrefix+"unchecked@127.0.0.1"]
	assert.Assert(t, ok, "connector without health check must always be configured")

	bindings.PerformHealthChecks()
	_, ok = configured()[qdr.TcpConnectorNamePrefix+"checked@127.0.0.1"]
	assert.Assert(t, ok, "reachable connector must be configured")

	assert.NilError(t, listener.Close())
	bindings.PerformHealthChecks()
	_, ok = configured()[qdr.TcpConnectorNamePrefix+"checked@127.0.0.1"]
	assert.Assert(t, !ok, "unreachable connector must be removed")
	_, ok = configured()[qdr.TcpConnectorNamePrefix+"unchecked@127.0.0.1"]
	assert.Assert(t, ok, "connector without health check must stay configured")
}

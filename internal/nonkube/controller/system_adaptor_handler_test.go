package controller

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/internal/nonkube/common"
	"github.com/skupperproject/skupper/internal/qdr"
	"github.com/skupperproject/skupper/internal/site"
	skupperv2alpha1 "github.com/skupperproject/skupper/pkg/apis/skupper/v2alpha1"
	"github.com/skupperproject/skupper/pkg/nonkube/api"
	"gotest.tools/v3/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func writeMinimalRouterConfig(t *testing.T, namespace, routerID string) {
	t.Helper()
	dir := api.GetInternalOutputPath(namespace, api.RouterConfigPath)
	assert.Assert(t, os.MkdirAll(dir, 0755))
	cfg := qdr.RouterConfig{
		Metadata: qdr.RouterMetadata{Id: routerID},
		Bridges: qdr.BridgeConfig{
			TcpListeners:  qdr.TcpEndpointMap{},
			TcpConnectors: qdr.TcpEndpointMap{},
		},
		SslProfiles: map[string]qdr.SslProfile{},
	}
	data, err := qdr.MarshalRouterConfig(cfg)
	assert.Assert(t, err)
	assert.Assert(t, os.WriteFile(filepath.Join(dir, "skrouterd.json"), []byte(data), 0644))
}

func TestNewSystemAdaptorHandler_ManualReturnsNil(t *testing.T) {
	t.Setenv(types.ENV_SYSTEM_AUTO_RELOAD, types.SystemReloadTypeManual)
	h := NewSystemAdaptorHandler("ns")
	assert.Assert(t, h == nil)
}

func TestNewSystemAdaptorHandler_AutoReturnsHandler(t *testing.T) {
	t.Setenv(types.ENV_SYSTEM_AUTO_RELOAD, types.SystemReloadTypeAuto)
	h := NewSystemAdaptorHandler("ns")
	assert.Assert(t, h != nil)
	assert.Equal(t, h.namespace, "ns")
	assert.Assert(t, h.logger != nil)
}

func TestNewSystemAdaptorHandler_ErrorGettingLocalRouterAddress(t *testing.T) {
	t.Setenv(types.ENV_SYSTEM_AUTO_RELOAD, types.SystemReloadTypeAuto)
	handler := NewSystemAdaptorHandler("ns")
	assert.Assert(t, handler != nil)
	handler.Start(nil)
	assert.Assert(t, handler.systemAdaptor == nil)
}

func TestNewSystemAdaptorHandler_Stop(t *testing.T) {
	t.Setenv(types.ENV_SYSTEM_AUTO_RELOAD, types.SystemReloadTypeAuto)
	handler := NewSystemAdaptorHandler("ns")
	assert.Assert(t, handler != nil)
	handler.running = true
	handler.Stop()
	assert.Assert(t, handler.running == false)
}

func TestSystemAdaptorHandler_StopReleasesBindings(t *testing.T) {
	t.Setenv(types.ENV_SYSTEM_AUTO_RELOAD, types.SystemReloadTypeAuto)
	handler := NewSystemAdaptorHandler("ns")
	handler.running = true
	handler.bindings = site.NewBindings("")
	handler.Stop()
	assert.Assert(t, !handler.running)
	assert.Assert(t, handler.bindings == nil, "bindings must be released on Stop")
}

func TestReconcileBindings_PicksUpNewConnector(t *testing.T) {
	t.Setenv(types.ENV_SYSTEM_AUTO_RELOAD, types.SystemReloadTypeAuto)
	tempDir := t.TempDir()
	if os.Getuid() == 0 {
		api.DefaultRootDataHome = tempDir
	} else {
		t.Setenv("XDG_DATA_HOME", tempDir)
	}

	namespace := "test-reconcile-new-connector"
	writeMinimalRouterConfig(t, namespace, "router-reconcile")

	runtimePath := api.GetInternalOutputPath(namespace, api.RuntimeSiteStatePath)
	assert.Assert(t, os.MkdirAll(runtimePath, 0755))
	initialState := &api.SiteState{
		SiteId: "site-reconcile",
		Site: &skupperv2alpha1.Site{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Site",
				APIVersion: "skupper.io/v2alpha1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "site-reconcile",
				Namespace: namespace,
			},
		},
		Connectors: map[string]*skupperv2alpha1.Connector{},
	}
	assert.Assert(t, api.MarshalSiteState(*initialState, runtimePath))

	// Build bindings from the initial (connector-less) site state.
	handler := NewSystemAdaptorHandler(namespace)
	assert.Assert(t, handler != nil)
	bindings := site.NewBindings("")
	bindings.SetSiteId(initialState.SiteId)
	handler.bindings = bindings

	// Verify the connector is absent before reconcile.
	desired, err := common.LoadRouterConfig(namespace)
	assert.Assert(t, err)
	bindings.Apply(desired)
	_, present := desired.Bridges.TcpConnectors[qdr.TcpConnectorNamePrefix+"backend@127.0.0.1"]
	assert.Assert(t, !present, "connector must not be present before it is added")

	initialState.Connectors["backend"] = &skupperv2alpha1.Connector{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Connector",
			APIVersion: "skupper.io/v2alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "backend",
			Namespace: namespace,
		},
		Spec: skupperv2alpha1.ConnectorSpec{
			Host:       "127.0.0.1",
			Port:       9090,
			RoutingKey: "backend",
		},
	}

	entries, err := os.ReadDir(runtimePath)
	assert.Assert(t, err)
	for _, e := range entries {
		assert.Assert(t, os.Remove(filepath.Join(runtimePath, e.Name())))
	}
	assert.Assert(t, api.MarshalSiteState(*initialState, runtimePath))

	handler.reconcileBindings()

	desired2, err := common.LoadRouterConfig(namespace)
	assert.Assert(t, err)
	handler.bindings.Apply(desired2)
	_, present2 := desired2.Bridges.TcpConnectors[qdr.TcpConnectorNamePrefix+"backend@127.0.0.1"]
	assert.Assert(t, present2, "connector added after startup must appear after reconcileBindings")
}

func TestReconcileBindings_RemovesDeletedConnector(t *testing.T) {
	t.Setenv(types.ENV_SYSTEM_AUTO_RELOAD, types.SystemReloadTypeAuto)
	tempDir := t.TempDir()
	if os.Getuid() == 0 {
		api.DefaultRootDataHome = tempDir
	} else {
		t.Setenv("XDG_DATA_HOME", tempDir)
	}

	namespace := "test-reconcile-delete-connector"
	writeMinimalRouterConfig(t, namespace, "router-reconcile-del")

	conn := &skupperv2alpha1.Connector{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Connector",
			APIVersion: "skupper.io/v2alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "backend",
			Namespace: namespace,
		},
		Spec: skupperv2alpha1.ConnectorSpec{
			Host:       "10.0.0.1",
			Port:       9090,
			RoutingKey: "backend",
		},
	}

	runtimePath := api.GetInternalOutputPath(namespace, api.RuntimeSiteStatePath)
	assert.Assert(t, os.MkdirAll(runtimePath, 0755))
	stateWithConnector := &api.SiteState{
		SiteId: "site-reconcile-del",
		Site: &skupperv2alpha1.Site{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Site",
				APIVersion: "skupper.io/v2alpha1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "site-reconcile-del",
				Namespace: namespace,
			},
		},
		Connectors: map[string]*skupperv2alpha1.Connector{"backend": conn},
	}
	assert.Assert(t, api.MarshalSiteState(*stateWithConnector, runtimePath))

	handler := NewSystemAdaptorHandler(namespace)
	assert.Assert(t, handler != nil)
	bindings := site.NewBindings("")
	bindings.SetSiteId(stateWithConnector.SiteId)
	_ = bindings.UpdateConnector("backend", conn)
	handler.bindings = bindings

	stateWithConnector.Connectors = map[string]*skupperv2alpha1.Connector{}
	entries, err := os.ReadDir(runtimePath)
	assert.Assert(t, err)
	for _, e := range entries {
		assert.Assert(t, os.Remove(filepath.Join(runtimePath, e.Name())))
	}
	assert.Assert(t, api.MarshalSiteState(*stateWithConnector, runtimePath))

	handler.reconcileBindings()

	desired, err := common.LoadRouterConfig(namespace)
	assert.Assert(t, err)
	handler.bindings.Apply(desired)
	_, present := desired.Bridges.TcpConnectors[qdr.TcpConnectorNamePrefix+"backend@10.0.0.1"]
	assert.Assert(t, !present, "deleted connector must not appear after reconcileBindings")
}

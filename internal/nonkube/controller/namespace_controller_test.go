package controller

import (
	"fmt"
	"os"
	"path"
	"testing"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/internal/nonkube/bootstrap"
	"github.com/skupperproject/skupper/pkg/nonkube/api"
	"gotest.tools/v3/assert"
)

func TestNamespaceController_UpgradeDriftedRouter(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("XDG_DATA_HOME", tempDir)

	ns := "test-ns"
	sourcesPath := api.GetInternalOutputPath(ns, api.InputSiteStatePath)
	assert.Assert(t, os.MkdirAll(sourcesPath, 0755))

	// Write platform.yaml
	internalPath := api.GetInternalOutputPath(ns, api.InternalBasePath)
	assert.Assert(t, os.MkdirAll(internalPath, 0755))
	assert.Assert(t, os.WriteFile(path.Join(internalPath, "platform.yaml"), []byte("platform: podman\n"), 0644))

	t.Run("skips when reload-type is manual", func(t *testing.T) {
		t.Setenv(types.ENV_SYSTEM_AUTO_RELOAD, "manual")
		bootstrapCalled := false
		nsc, err := NewNamespaceController(ns)
		assert.Assert(t, err)

		nsc.checkRouterDrift = func(namespace string) (bool, error) {
			return true, nil
		}
		nsc.bootstrap = func(config *bootstrap.Config) (*api.SiteState, error) {
			bootstrapCalled = true
			return nil, nil
		}

		nsc.upgradeDriftedRouter()
		assert.Assert(t, !bootstrapCalled, "bootstrap should not be called when reload-type is manual")
	})

	t.Run("skips when no drift in auto mode", func(t *testing.T) {
		t.Setenv(types.ENV_SYSTEM_AUTO_RELOAD, "auto")
		bootstrapCalled := false
		nsc, err := NewNamespaceController(ns)
		assert.Assert(t, err)

		nsc.checkRouterDrift = func(namespace string) (bool, error) {
			return false, nil
		}
		nsc.bootstrap = func(config *bootstrap.Config) (*api.SiteState, error) {
			bootstrapCalled = true
			return nil, nil
		}

		nsc.upgradeDriftedRouter()
		assert.Assert(t, !bootstrapCalled, "bootstrap should not be called when no drift")
	})

	t.Run("re-bootstraps when drift detected in auto mode", func(t *testing.T) {
		t.Setenv(types.ENV_SYSTEM_AUTO_RELOAD, "auto")
		var capturedCfg *bootstrap.Config
		postBootstrapCalled := false

		nsc, err := NewNamespaceController(ns)
		assert.Assert(t, err)

		nsc.checkRouterDrift = func(namespace string) (bool, error) {
			return true, nil
		}
		nsc.bootstrap = func(config *bootstrap.Config) (*api.SiteState, error) {
			capturedCfg = config
			return &api.SiteState{}, nil
		}
		nsc.postBootstrap = func(config *bootstrap.Config, siteState *api.SiteState) {
			postBootstrapCalled = true
		}

		nsc.upgradeDriftedRouter()
		assert.Assert(t, capturedCfg != nil, "bootstrap should be called when drift detected in auto mode")
		assert.Equal(t, capturedCfg.Namespace, ns)
		assert.Equal(t, capturedCfg.Platform, types.PlatformPodman)
		assert.Assert(t, postBootstrapCalled, "postBootstrap should be called")
	})

	t.Run("handles bootstrap error gracefully in auto mode", func(t *testing.T) {
		t.Setenv(types.ENV_SYSTEM_AUTO_RELOAD, "auto")
		nsc, err := NewNamespaceController(ns)
		assert.Assert(t, err)

		nsc.checkRouterDrift = func(namespace string) (bool, error) {
			return true, nil
		}
		nsc.bootstrap = func(config *bootstrap.Config) (*api.SiteState, error) {
			return nil, fmt.Errorf("bootstrap error")
		}

		// Should not panic
		nsc.upgradeDriftedRouter()
	})
}

package bootstrap

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/pkg/nonkube/api"
	"gotest.tools/v3/assert"
)

func TestUpgradeRoutersWith_RebootsDriftedNamespace(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_DATA_HOME", tmp)
	nsPath := filepath.Join(tmp, "skupper", "namespaces", "mysite")
	_ = os.MkdirAll(nsPath, 0755)

	var capturedCfg *Config

	checkDrift := func(string) (bool, error) { return true, nil }
	bootstrapSite := func(cfg *Config) (*api.SiteState, error) {
		capturedCfg = cfg
		return nil, nil
	}

	upgradeRoutersWith("podman", checkDrift, bootstrapSite)

	assert.Assert(t, capturedCfg != nil, "Bootstrap was never called")
	assert.Equal(t, capturedCfg.Namespace, "mysite")
	assert.Equal(t, capturedCfg.Platform, types.Platform("podman"))

	expectedInputPath := filepath.Join(tmp, "skupper", "namespaces", "mysite", "input", "resources")
	assert.Equal(t, capturedCfg.InputPath, expectedInputPath,
		"InputPath must point to the namespace's stored input sources")
}

func TestUpgradeRoutersWith_SkipsNonDriftedNamespace(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_DATA_HOME", tmp)
	_ = os.MkdirAll(filepath.Join(tmp, "skupper", "namespaces", "up-to-date"), 0755)

	bootstrapCalled := false

	checkDrift := func(string) (bool, error) { return false, nil }
	bootstrapSite := func(cfg *Config) (*api.SiteState, error) { bootstrapCalled = true; return nil, nil }

	upgradeRoutersWith("podman", checkDrift, bootstrapSite)

	assert.Assert(t, !bootstrapCalled, "Bootstrap must not be called when there is no drift")
}

func TestUpgradeRoutersWith_ReturnsErrorOnBootstrapFailure(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_DATA_HOME", tmp)
	_ = os.MkdirAll(filepath.Join(tmp, "skupper", "namespaces", "broken"), 0755)

	checkDrift := func(string) (bool, error) { return true, nil }
	bootstrapSite := func(cfg *Config) (*api.SiteState, error) { return nil, fmt.Errorf("bootstrap failed") }

	err := upgradeRoutersWith("podman", checkDrift, bootstrapSite)
	assert.ErrorContains(t, err, "bootstrap failed")
}

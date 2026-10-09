package bootstrap

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/pkg/container"
	"github.com/skupperproject/skupper/pkg/nonkube/api"
	"gotest.tools/v3/assert"
)

func TestUpgradeNamespaces_BootstrapsBackedUpNamespaces(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_DATA_HOME", tmp)
	_ = os.MkdirAll(filepath.Join(tmp, "skupper", "namespaces", "ns-a"), 0755)
	_ = os.MkdirAll(filepath.Join(tmp, "skupper", "namespaces", "ns-b"), 0755)

	bootstrapped := map[string]bool{}
	bootstrapSite := func(cfg *Config) (*api.SiteState, error) {
		bootstrapped[cfg.Namespace] = true
		return nil, nil
	}

	backups := map[string]*container.Container{
		"ns-a": {Name: "ns-a-skupper-router", Image: "old"},
		"ns-b": {Name: "ns-b-skupper-router", Image: "old"},
	}

	err := upgradeNamespaces("podman", backups, bootstrapSite)
	assert.Assert(t, err)
	assert.Assert(t, bootstrapped["ns-a"], "ns-a must be bootstrapped")
	assert.Assert(t, bootstrapped["ns-b"], "ns-b must be bootstrapped")
}

func TestUpgradeNamespaces_ReturnsErrorOnBootstrapFailure(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_DATA_HOME", tmp)
	_ = os.MkdirAll(filepath.Join(tmp, "skupper", "namespaces", "broken"), 0755)

	backups := map[string]*container.Container{
		"broken": {Name: "broken-skupper-router", Image: "old"},
	}
	bootstrapSite := func(cfg *Config) (*api.SiteState, error) {
		return nil, fmt.Errorf("bootstrap failed")
	}

	err := upgradeNamespaces("podman", backups, bootstrapSite)
	assert.ErrorContains(t, err, "bootstrap failed")
}

func TestUpgradeNamespaces_EmptyBackupsSkipsBootstrap(t *testing.T) {
	bootstrapCalled := false
	bootstrapSite := func(cfg *Config) (*api.SiteState, error) {
		bootstrapCalled = true
		return nil, nil
	}

	err := upgradeNamespaces("podman", nil, bootstrapSite)
	assert.Assert(t, err)
	assert.Assert(t, !bootstrapCalled, "Bootstrap must not be called for an empty backup map")
}

func TestUpgradeNamespaces_PartialFailureCollectsAllErrors(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_DATA_HOME", tmp)

	bootstrapped := map[string]bool{}
	bootstrapSite := func(cfg *Config) (*api.SiteState, error) {
		if cfg.Namespace == "broken" {
			return nil, fmt.Errorf("bootstrap failed")
		}
		bootstrapped[cfg.Namespace] = true
		return nil, nil
	}

	backups := map[string]*container.Container{
		"broken": {Name: "broken-skupper-router", Image: "old"},
		"good":   {Name: "good-skupper-router", Image: "old"},
	}

	err := upgradeNamespaces("podman", backups, bootstrapSite)
	assert.ErrorContains(t, err, "bootstrap failed")
	assert.Assert(t, bootstrapped["good"], "good namespace must still be bootstrapped after broken fails")
}

func TestUpgradeNamespaces_ConfigPassedCorrectly(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_DATA_HOME", tmp)

	var capturedCfg *Config
	bootstrapSite := func(cfg *Config) (*api.SiteState, error) {
		capturedCfg = cfg
		return nil, nil
	}

	backups := map[string]*container.Container{
		"mysite": {Name: "mysite-skupper-router", Image: "old"},
	}

	err := upgradeNamespaces("podman", backups, bootstrapSite)
	assert.Assert(t, err)
	assert.Assert(t, capturedCfg != nil, "Bootstrap was never called")
	assert.Equal(t, capturedCfg.Namespace, "mysite")
	assert.Equal(t, capturedCfg.Platform, types.Platform("podman"))
	expectedInputPath := filepath.Join(tmp, "skupper", "namespaces", "mysite", "input", "resources")
	assert.Equal(t, capturedCfg.InputPath, expectedInputPath)
}

func TestAnyRouterDriftedWith_ReturnsTrueOnFirstDrift(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_DATA_HOME", tmp)
	_ = os.MkdirAll(filepath.Join(tmp, "skupper", "namespaces", "ns1"), 0755)
	_ = os.MkdirAll(filepath.Join(tmp, "skupper", "namespaces", "ns2"), 0755)

	calls := 0
	checkDrift := func(string) (bool, error) {
		calls++
		return true, nil
	}

	drifted, err := anyRouterDriftedWith(checkDrift)
	assert.Assert(t, err)
	assert.Assert(t, drifted, "should report drift")
	assert.Equal(t, calls, 1, "should stop after the first drifted namespace")
}

func TestAnyRouterDriftedWith_ReturnsFalseWhenNoneDrifted(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_DATA_HOME", tmp)
	_ = os.MkdirAll(filepath.Join(tmp, "skupper", "namespaces", "ns1"), 0755)
	_ = os.MkdirAll(filepath.Join(tmp, "skupper", "namespaces", "ns2"), 0755)

	checkDrift := func(string) (bool, error) { return false, nil }

	drifted, err := anyRouterDriftedWith(checkDrift)
	assert.Assert(t, err)
	assert.Assert(t, !drifted, "should report no drift")
}

func TestAnyRouterDriftedWith_PropagatesCheckError(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_DATA_HOME", tmp)
	_ = os.MkdirAll(filepath.Join(tmp, "skupper", "namespaces", "ns1"), 0755)

	checkDrift := func(ns string) (bool, error) {
		return false, fmt.Errorf("check failed for %s", ns)
	}

	_, err := anyRouterDriftedWith(checkDrift)
	assert.ErrorContains(t, err, "check failed for ns1")
}

func TestAnyRouterDriftedWith_IgnoresFiles(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_DATA_HOME", tmp)
	nsDir := filepath.Join(tmp, "skupper", "namespaces")
	_ = os.MkdirAll(nsDir, 0755)
	// Create a file (not a directory) alongside a real namespace dir.
	_ = os.WriteFile(filepath.Join(nsDir, "not-a-namespace.txt"), []byte{}, 0644)
	_ = os.MkdirAll(filepath.Join(nsDir, "real-ns"), 0755)

	var checkedNames []string
	checkDrift := func(ns string) (bool, error) {
		checkedNames = append(checkedNames, ns)
		return false, nil
	}

	_, err := anyRouterDriftedWith(checkDrift)
	assert.Assert(t, err)
	assert.Equal(t, len(checkedNames), 1, "only directories should be checked")
	assert.Equal(t, checkedNames[0], "real-ns")
}

func TestAnyRouterDriftedWith_EmptyNamespacesDir(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_DATA_HOME", tmp)
	_ = os.MkdirAll(filepath.Join(tmp, "skupper", "namespaces"), 0755)

	called := false
	checkDrift := func(string) (bool, error) { called = true; return true, nil }

	drifted, err := anyRouterDriftedWith(checkDrift)
	assert.Assert(t, err)
	assert.Assert(t, !drifted)
	assert.Assert(t, !called, "check must not be called when there are no namespaces")
}

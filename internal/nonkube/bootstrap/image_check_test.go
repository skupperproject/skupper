package bootstrap

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"gotest.tools/v3/assert"
)

func TestAnyRouterDrifted_NoDrift(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_DATA_HOME", tmp)
	_ = os.MkdirAll(filepath.Join(tmp, "skupper", "namespaces", "ns1"), 0755)
	_ = os.MkdirAll(filepath.Join(tmp, "skupper", "namespaces", "ns2"), 0755)

	drifted, err := anyRouterDriftedWith(func(string) (bool, error) { return false, nil })
	assert.NilError(t, err)
	assert.Assert(t, !drifted)
}

func TestAnyRouterDrifted_OneDrifted(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_DATA_HOME", tmp)
	_ = os.MkdirAll(filepath.Join(tmp, "skupper", "namespaces", "ns1"), 0755)
	_ = os.MkdirAll(filepath.Join(tmp, "skupper", "namespaces", "ns2"), 0755)

	called := map[string]bool{}
	drifted, err := anyRouterDriftedWith(func(ns string) (bool, error) {
		called[ns] = true
		return ns == "ns1", nil
	})
	assert.NilError(t, err)
	assert.Assert(t, drifted)
	assert.Assert(t, len(called) >= 1)
}

func TestAnyRouterDrifted_CheckError(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_DATA_HOME", tmp)
	_ = os.MkdirAll(filepath.Join(tmp, "skupper", "namespaces", "ns1"), 0755)

	_, err := anyRouterDriftedWith(func(ns string) (bool, error) {
		return false, os.ErrPermission
	})
	assert.ErrorContains(t, err, "router check for")
}

func TestAnyRouterDrifted_InspectErrorPropagates(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_DATA_HOME", tmp)
	_ = os.MkdirAll(filepath.Join(tmp, "skupper", "namespaces", "ns1"), 0755)

	inspectErr := fmt.Errorf("connection refused")
	_, err := anyRouterDriftedWith(func(string) (bool, error) {
		return false, inspectErr
	})
	assert.ErrorContains(t, err, "connection refused")
}

package networkobserver

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/skupperproject/skupper/pkg/nonkube/api"
	"gotest.tools/v3/assert"
)

func setTempPrometheusHome(t *testing.T) {
	t.Helper()
	if os.Getuid() == 0 {
		api.DefaultRootDataHome = t.TempDir()
	} else {
		t.Setenv("XDG_DATA_HOME", t.TempDir())
	}
}

func TestWriteAndReadPrometheusState(t *testing.T) {
	setTempPrometheusHome(t)

	if err := os.MkdirAll(api.GetHostPrometheusHome(), 0755); err != nil {
		t.Fatalf("setup: %v", err)
	}

	assert.NilError(t, WritePrometheusState(9090))

	port, err := ReadPrometheusPort()
	assert.NilError(t, err)
	assert.Equal(t, 9090, port)
}

func TestReadPrometheusPort_NotInstalled(t *testing.T) {
	setTempPrometheusHome(t)

	_, err := ReadPrometheusPort()
	assert.ErrorContains(t, err, "prometheus is not installed")
}

func TestReadPrometheusPort_Corrupt(t *testing.T) {
	setTempPrometheusHome(t)

	if err := os.MkdirAll(api.GetHostPrometheusHome(), 0755); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if err := os.WriteFile(prometheusStateFile(), []byte("not-json"), 0644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	_, err := ReadPrometheusPort()
	assert.ErrorContains(t, err, "failed to parse prometheus state file")
}

func TestIsPrometheusInstalled(t *testing.T) {
	setTempPrometheusHome(t)

	assert.Equal(t, false, IsPrometheusInstalled())

	if err := os.MkdirAll(api.GetHostPrometheusHome(), 0755); err != nil {
		t.Fatalf("setup: %v", err)
	}
	assert.NilError(t, WritePrometheusState(9091))
	assert.Equal(t, true, IsPrometheusInstalled())
}

func TestWriteAndRemoveTargetFile(t *testing.T) {
	setTempPrometheusHome(t)

	assert.NilError(t, WriteTargetFile("west", 9001))

	targetFile := filepath.Join(api.GetPrometheusTargetsDir(), "west.json")
	data, err := os.ReadFile(targetFile)
	assert.NilError(t, err)

	type entry struct {
		Targets []string          `json:"targets"`
		Labels  map[string]string `json:"labels"`
	}
	var entries []entry
	assert.NilError(t, json.Unmarshal(data, &entries))
	assert.Equal(t, 1, len(entries))
	assert.Equal(t, "localhost:9001", entries[0].Targets[0])
	assert.Equal(t, "west", entries[0].Labels["namespace"])

	assert.NilError(t, RemoveTargetFile("west"))
	_, err = os.Stat(targetFile)
	assert.Assert(t, os.IsNotExist(err))

	assert.NilError(t, RemoveTargetFile("west"))
}

func TestInstalledNetworkObservers(t *testing.T) {
	setTempPrometheusHome(t)

	assert.Equal(t, 0, len(installedNetworkObservers()))

	assert.NilError(t, WriteTargetFile("west", 9001))
	assert.NilError(t, WriteTargetFile("east", 9002))

	namespaces := installedNetworkObservers()
	assert.Equal(t, 2, len(namespaces))

	found := map[string]bool{}
	for _, ns := range namespaces {
		found[ns] = true
	}
	assert.Assert(t, found["west"])
	assert.Assert(t, found["east"])
}

func TestClaimedMetricsPorts(t *testing.T) {
	setTempPrometheusHome(t)

	assert.Equal(t, 0, len(claimedMetricsPorts()))

	assert.NilError(t, WriteTargetFile("west", 9001))
	assert.NilError(t, WriteTargetFile("east", 9002))

	claimed := claimedMetricsPorts()
	assert.Assert(t, claimed[9001])
	assert.Assert(t, claimed[9002])
	assert.Assert(t, !claimed[9003])
}

func TestNextFreeMetricsPort(t *testing.T) {
	setTempPrometheusHome(t)

	assert.NilError(t, WriteTargetFile("ns0", 9000))
	assert.NilError(t, WriteTargetFile("ns1", 9001))

	port, err := NextFreeMetricsPort(9000)
	assert.NilError(t, err)
	assert.Assert(t, port >= 9002, "expected port >= 9002, got %d", port)
}

func TestJoinStrings(t *testing.T) {
	tests := []struct {
		input    []string
		expected string
	}{
		{nil, ""},
		{[]string{"a"}, "a"},
		{[]string{"a", "b", "c"}, "a, b, c"},
	}
	for _, tc := range tests {
		got := joinStrings(tc.input)
		assert.Equal(t, tc.expected, got)
	}
}

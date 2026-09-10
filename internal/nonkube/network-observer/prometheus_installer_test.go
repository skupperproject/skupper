package networkobserver

import (
	"log/slog"
	"os"
	"testing"

	"github.com/skupperproject/skupper/internal/nonkube/client/compat"
	"github.com/skupperproject/skupper/pkg/container"
	"github.com/skupperproject/skupper/pkg/nonkube/api"
	"gotest.tools/v3/assert"
)

func newMockContainer(name string, running bool) *container.Container {
	return &container.Container{
		Name:    "/" + name,
		Running: running,
		Labels:  map[string]string{"application": container.AppName},
		Image:   "mock-image",
	}
}

func newPrometheusInstallerWithMock(containers []*container.Container) *PrometheusInstaller {
	return &PrometheusInstaller{
		Platform: "podman",
		logger:   slog.Default(),
		cli:      compat.NewCompatClientMock(containers),
	}
}

func TestValidatePrerequisitesForInstall_AlreadyRunning(t *testing.T) {
	installer := newPrometheusInstallerWithMock([]*container.Container{
		newMockContainer("skupper-prometheus", true),
	})

	err := installer.ValidatePrerequisitesForInstall()
	assert.ErrorContains(t, err, `"skupper-prometheus" is already running`)
}

func TestValidatePrerequisitesForInstall_NotRunning(t *testing.T) {
	installer := newPrometheusInstallerWithMock([]*container.Container{})

	assert.NilError(t, installer.ValidatePrerequisitesForInstall())
}

func TestValidatePrerequisitesForInstall_ContainerExistsButStopped(t *testing.T) {
	installer := newPrometheusInstallerWithMock([]*container.Container{
		newMockContainer("skupper-prometheus", false),
	})

	assert.NilError(t, installer.ValidatePrerequisitesForInstall())
}

func TestValidatePrerequisitesForUninstall_NotRunning(t *testing.T) {
	installer := newPrometheusInstallerWithMock([]*container.Container{})

	err := installer.ValidatePrerequisitesForUninstall()
	assert.ErrorContains(t, err, "is not running")
	assert.ErrorContains(t, err, "nothing to uninstall")
}

func TestValidatePrerequisitesForUninstall_NetworkObserversStillInstalled(t *testing.T) {
	setTempPrometheusHome(t)

	assert.NilError(t, WriteTargetFile("west", 9001))

	installer := newPrometheusInstallerWithMock([]*container.Container{
		newMockContainer("skupper-prometheus", true),
	})

	err := installer.ValidatePrerequisitesForUninstall()
	assert.ErrorContains(t, err, "network observers are still installed")
	assert.ErrorContains(t, err, "west")
}

func TestValidatePrerequisitesForUninstall_OK(t *testing.T) {
	setTempPrometheusHome(t)

	installer := newPrometheusInstallerWithMock([]*container.Container{
		newMockContainer("skupper-prometheus", true),
	})

	assert.NilError(t, installer.ValidatePrerequisitesForUninstall())
}

func TestPrometheusInstaller_isContainerRunning(t *testing.T) {
	tests := []struct {
		name       string
		containers []*container.Container
		query      string
		expected   bool
	}{
		{
			name:       "running container found",
			containers: []*container.Container{newMockContainer("skupper-prometheus", true)},
			query:      "skupper-prometheus",
			expected:   true,
		},
		{
			name:       "stopped container not considered running",
			containers: []*container.Container{newMockContainer("skupper-prometheus", false)},
			query:      "skupper-prometheus",
			expected:   false,
		},
		{
			name:       "unknown container name returns false",
			containers: []*container.Container{newMockContainer("skupper-prometheus", true)},
			query:      "other-container",
			expected:   false,
		},
		{
			name:       "empty container list returns false",
			containers: []*container.Container{},
			query:      "skupper-prometheus",
			expected:   false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			installer := newPrometheusInstallerWithMock(tc.containers)
			assert.Equal(t, tc.expected, installer.isContainerRunning(tc.query))
		})
	}
}

func TestPrometheusInstaller_Uninstall_RemovesStateAndDir(t *testing.T) {
	setTempPrometheusHome(t)

	prometheusHome := api.GetHostPrometheusHome()
	if err := os.MkdirAll(prometheusHome, 0755); err != nil {
		t.Fatalf("setup: %v", err)
	}
	assert.NilError(t, WritePrometheusState(9090))

	installer := newPrometheusInstallerWithMock([]*container.Container{
		newMockContainer("skupper-prometheus", true),
	})

	assert.NilError(t, installer.Uninstall())

	_, err := os.Stat(prometheusHome)
	assert.Assert(t, os.IsNotExist(err), "expected prometheus home to be removed")
}

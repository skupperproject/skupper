//go:build podman
// +build podman

package podman

import (
	"context"
	"testing"

	"github.com/skupperproject/skupper/api/types"
	clientpodman "github.com/skupperproject/skupper/client/podman"
	"gotest.tools/assert"
)

func TestSkupperNetworkStatusVolumeUpdate(t *testing.T) {
	cli := clientpodman.NewPodmanClientMock(mockContainers())
	mock := cli.RestClient.(*clientpodman.RestClientMock)
	localMockVolumes, localMockVolumeFiles := mockVolumes()
	// removing new volume to enforce update task to run
	delete(localMockVolumes, "skupper-network-status")
	assert.Assert(t, mock.MockVolumeFiles(localMockVolumes, localMockVolumeFiles))
	defer func() {
		_ = mock.CleanupMockVolumeDir()
	}()
	ch := NewRouterConfigHandlerPodman(cli)

	updateSkupperNetworkVolumeTask := new(SkupperNetworkStatusVolume).WithCli(cli)

	tests := []struct {
		name       string
		curVersion string
		version    string
		applies    bool
		changed    bool
	}{
		{
			name:       "newer-version",
			curVersion: "1.5.3",
			version:    "1.5.4",
			applies:    true,
			changed:    true,
		},
		{
			name:       "newer-version-volume-exists",
			curVersion: "1.5.3",
			version:    "1.5.4",
			applies:    true,
			changed:    false,
		},
		{
			name:       "older-version",
			curVersion: "1.5.4",
			version:    "1.5.3",
			applies:    false,
		},
		{
			name:       "invalid-old-version",
			curVersion: "invalid-old-version",
			version:    "1.5.4",
			applies:    false,
		},
		{
			name:       "invalid-versions",
			curVersion: "invalid-old-version",
			version:    "invalid-new-version",
			applies:    false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cfg, err := ch.GetRouterConfig()
			assert.Assert(t, err)
			siteMeta := cfg.GetSiteMetadata()
			siteMeta.Version = test.curVersion
			cfg.SetSiteMetadata(&siteMeta)
			assert.Assert(t, ch.SaveRouterConfig(cfg))
			// simulating new version
			t.Logf("New version: %s - current mock site version: %s", test.version, test.curVersion)

			// validating if task has to be executed or not
			assert.Assert(t, updateSkupperNetworkVolumeTask.AppliesTo(test.curVersion) == test.applies)

			// mocking task execution
			if test.applies {
				res := updateSkupperNetworkVolumeTask.Run(context.Background())
				assert.Assert(t, len(res.Errors) == 0)
				assert.Assert(t, res.Changed() == test.changed)
				if test.changed {
					assert.Assert(t, 1 == len(res.GetChanges()))
					// removing mounted volume from controller container
					volumeMounted := false
					for _, c := range mock.Containers {
						if c.Name != types.ControllerPodmanContainerName {
							continue
						}
						for _, m := range c.Mounts {
							if m.Name == types.NetworkStatusConfigMapName {
								volumeMounted = true
							}
						}
					}
					assert.Assert(t, volumeMounted)
				}
			}
		})
	}
}

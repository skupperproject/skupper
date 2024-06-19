//go:build podman
// +build podman

package podman

import (
	"context"
	_ "embed"
	"regexp"
	"testing"

	"github.com/skupperproject/skupper/client/podman"
	"github.com/skupperproject/skupper/pkg/container"
	"github.com/skupperproject/skupper/pkg/images"
	"gotest.tools/assert"
)

func TestContainerImagesTask(t *testing.T) {
	cli := podman.NewPodmanClientMock(mockUpdateContainers())
	mock := cli.RestClient.(*podman.RestClientMock)
	assert.Assert(t, mock.MockVolumeFiles(mockVolumes()))
	defer func() {
		_ = mock.CleanupMockVolumeDir()
	}()
	sh := NewSitePodmanHandlerFromCli(cli)
	site, err := sh.Get()
	assert.Assert(t, err)
	imagesTask := NewContainerImagesTask(cli)

	tests := []struct {
		name    string
		version string
		changed bool
	}{
		{
			name:    "newer-version",
			version: "1.5.0",
			changed: true,
		},
		{
			name:    "older-version",
			version: "1.4.0",
			changed: false,
		},
		{
			name:    "invalid-version",
			version: "invalid-version",
			changed: false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			imagesTask.version = test.version
			t.Logf("New version: %s - current mock site version: %s", test.version, site.GetVersion())
			assert.Assert(t, imagesTask.AppliesTo(site.GetVersion()) == test.changed)
			res := imagesTask.Run(context.Background())
			assert.Assert(t, len(res.Errors) == 0)
			assert.Assert(t, res.Changed() == test.changed)
			expectedChanges := 4
			if !test.changed {
				expectedChanges = 0
			}
			assert.Equal(t, expectedChanges, len(res.GetChanges()), "Changes performed: ", res.GetChanges())
		})
	}
}

func TestVersionUpdateTask(t *testing.T) {
	cli := podman.NewPodmanClientMock(mockContainers())
	mock := cli.RestClient.(*podman.RestClientMock)
	assert.Assert(t, mock.MockVolumeFiles(mockVolumes()))
	defer func() {
		_ = mock.CleanupMockVolumeDir()
	}()
	ch := NewRouterConfigHandlerPodman(cli)

	versionUpdTask := NewVersionUpdateTask(cli)
	tests := []struct {
		name       string
		curVersion string
		version    string
		changed    bool
	}{
		{
			name:       "newer-version",
			curVersion: "1.4.2",
			version:    "1.5.0",
			changed:    true,
		},
		{
			name:       "older-version",
			curVersion: "1.4.2",
			version:    "1.4.0",
			changed:    false,
		},
		{
			name:       "invalid-old-version",
			curVersion: "invalid-old-version",
			version:    "1.5.0",
			changed:    false,
		},
		{
			name:       "invalid-new-version",
			curVersion: "1.4.2",
			version:    "invalid-new-version",
			changed:    false,
		},
		{
			name:       "invalid-versions",
			curVersion: "invalid-old-version",
			version:    "invalid-new-version",
			changed:    false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// updating site version to curVersion
			cfg, err := ch.GetRouterConfig()
			assert.Assert(t, err)
			siteMeta := cfg.GetSiteMetadata()
			siteMeta.Version = test.curVersion
			cfg.SetSiteMetadata(&siteMeta)
			assert.Assert(t, ch.SaveRouterConfig(cfg))
			// simulating new version
			versionUpdTask.version = test.version
			t.Logf("New version: %s - current mock site version: %s", test.version, test.curVersion)

			// validating if task has to be executed or not
			assert.Assert(t, versionUpdTask.AppliesTo(test.curVersion) == test.changed)

			// mocking task execution
			if test.changed {
				res := versionUpdTask.Run(context.Background())
				assert.Assert(t, len(res.Errors) == 0)
				assert.Assert(t, res.Changed() == test.changed)
				assert.Assert(t, 1 == len(res.GetChanges()))
			}
		})
	}
}

func mockUpdateContainers() []*container.Container {
	imageNoTag := regexp.MustCompile(`(.*):.*`)
	containers := mockContainers()
	for _, c := range containers {
		switch c.Name {
		case "skupper-router":
			c.Image = imageNoTag.ReplaceAllString(images.GetRouterImageName(), "$1:old")
		case "skupper-controller-podman":
			c.Image = imageNoTag.ReplaceAllString(images.GetControllerPodmanImageName(), "$1:old")
		case "flow-collector":
			c.Image = imageNoTag.ReplaceAllString(images.GetFlowCollectorImageName(), "$1:old")
		case "nginx-service":
			c.Image = imageNoTag.ReplaceAllString(images.GetRouterImageName(), "$1:old")
		}
	}
	return containers
}

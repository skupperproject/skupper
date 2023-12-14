//go:build podman
// +build podman

package update

import (
	_ "embed"
	"flag"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/client/container"
	"github.com/skupperproject/skupper/client/podman"
	domainpodman "github.com/skupperproject/skupper/pkg/domain/podman"
	"github.com/skupperproject/skupper/pkg/images"
	"gotest.tools/assert"
)

// some podman tests require a cluster as well
var clusterRun = flag.Bool("use-cluster", false, "run tests against a configured cluster")

func TestMain(m *testing.M) {
	flag.Parse()
	os.Exit(m.Run())
}

var (
	//go:embed skrouterd.json
	skrouterdJson string
	//go:embed skupper-services.json
	skupperServicesJson string
)

func TestContainerImagesTask(t *testing.T) {
	cli := podman.NewPodmanClientMock(mockContainers())
	mock := cli.RestClient.(*podman.RestClientMock)
	assert.Assert(t, mock.MockVolumeFiles(mockVolumes()))
	defer func() {
		_ = mock.CleanupMockVolumeDir()
	}()
	sh := domainpodman.NewSitePodmanHandlerFromCli(cli)
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
			res := imagesTask.Run()
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
	ch := domainpodman.NewRouterConfigHandlerPodman(cli)

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
				res := versionUpdTask.Run()
				assert.Assert(t, len(res.Errors) == 0)
				assert.Assert(t, res.Changed() == test.changed)
				assert.Assert(t, 1 == len(res.GetChanges()))
			}
		})
	}
}

func mockContainers() []*container.Container {
	imageNoTag := regexp.MustCompile(`(.*):.*`)
	return []*container.Container{
		{
			ID:    strings.Replace(uuid.New().String(), "-", "", -1),
			Name:  "skupper-router",
			Image: imageNoTag.ReplaceAllString(images.GetRouterImageName(), "$1:old"),
			Labels: map[string]string{
				"application":          "skupper",
				"skupper.io/component": "skupper-router",
			},
			Networks: map[string]container.ContainerNetworkInfo{
				"skupper": {
					ID:        "skupper",
					IPAddress: "172.17.0.10",
					Gateway:   "172.17.0.1",
					Aliases:   []string{"skupper-router"},
					//Aliases:   []string{"skupper", "service-controller"},
				},
			},
			Ports: []container.Port{
				{Host: "55671", HostIP: "", Target: "55671", Protocol: "tcp"},
				{Host: "45671", HostIP: "", Target: "45671", Protocol: "tcp"},
			},
			Running:   true,
			CreatedAt: time.Now(),
			StartedAt: time.Now(),
		},
		{
			ID:    strings.Replace(uuid.New().String(), "-", "", -1),
			Name:  "skupper-controller-podman",
			Image: imageNoTag.ReplaceAllString(images.GetControllerPodmanImageName(), "$1:old"),
			Labels: map[string]string{
				"application":          "skupper",
				"skupper.io/component": "skupper-controller-podman",
			},
			Networks: map[string]container.ContainerNetworkInfo{
				"skupper": {
					ID:        "skupper",
					IPAddress: "172.17.0.11",
					Gateway:   "172.17.0.1",
					Aliases:   []string{"skupper", "service-controller-podman"},
				},
			},
			Running:   true,
			CreatedAt: time.Now(),
			StartedAt: time.Now(),
		}, {
			ID:    strings.Replace(uuid.New().String(), "-", "", -1),
			Name:  "flow-collector",
			Image: imageNoTag.ReplaceAllString(images.GetFlowCollectorImageName(), "$1:old"),
			Labels: map[string]string{
				"application":          "skupper",
				"skupper.io/component": "flow-collector",
			},
			Networks: map[string]container.ContainerNetworkInfo{
				"skupper": {
					ID:        "skupper",
					IPAddress: "172.17.0.12",
					Gateway:   "172.17.0.1",
					Aliases:   []string{"flow-collector"},
				},
			},
			Ports: []container.Port{
				{Host: "8010", HostIP: "", Target: "8010", Protocol: "tcp"},
			},
			Running:   true,
			CreatedAt: time.Now(),
			StartedAt: time.Now(),
		}, {
			ID:    strings.Replace(uuid.New().String(), "-", "", -1),
			Name:  "nginx-service",
			Image: imageNoTag.ReplaceAllString(images.GetRouterImageName(), "$1:old"),
			Labels: map[string]string{
				"application":        "skupper",
				"skupper.io/address": "nginx",
			},
			Networks: map[string]container.ContainerNetworkInfo{
				"skupper": {
					ID:        "skupper",
					IPAddress: "172.17.0.13",
					Gateway:   "172.17.0.1",
					Aliases:   []string{"nginx-service"},
				},
			},
			Ports: []container.Port{
				{Host: "8080", HostIP: "", Target: "8080", Protocol: "tcp"},
			},
			Running:   true,
			CreatedAt: time.Now(),
			StartedAt: time.Now(),
		}, {
			ID:     strings.Replace(uuid.New().String(), "-", "", -1),
			Name:   "nginx",
			Image:  "docker.io/nginxinc/nginx-unprivileged:stable-alpine",
			Labels: map[string]string{},
			Networks: map[string]container.ContainerNetworkInfo{
				"skupper": {
					ID:        "skupper",
					IPAddress: "172.17.0.14",
					Gateway:   "172.17.0.1",
					Aliases:   []string{"nginx"},
				},
			},
			Running:   true,
			CreatedAt: time.Now(),
			StartedAt: time.Now(),
		},
	}
}

func mockVolumes() (map[string]*container.Volume, map[string]map[string]string) {
	var volumes = map[string]*container.Volume{}
	var volumesFiles = map[string]map[string]string{}
	addSkupperVolume := func(name string, typeLabel ...string) {
		labels := map[string]string{"application": "skupper"}
		if len(typeLabel) == 1 {
			labels[types.SkupperTypeQualifier] = typeLabel[0]
		}
		volumes[name] = &container.Volume{Name: name, Labels: labels}
	}
	addSkupperVolume("skupper-console-certs", "Credential")
	addSkupperVolume("skupper-console-users")
	addSkupperVolume("skupper-internal")
	addSkupperVolume("skupper-local-ca", "CertAuthority")
	addSkupperVolume("skupper-local-client", "Credential")
	addSkupperVolume("skupper-local-server", "Credential")
	addSkupperVolume("skupper-router-certs")
	addSkupperVolume("skupper-service-ca", "CertAuthority")
	addSkupperVolume("skupper-service-client", "Credential")
	addSkupperVolume("skupper-services")
	addSkupperVolume("skupper-site-ca", "CertAuthority")
	addSkupperVolume("skupper-site-server", "Credential")

	// volumes content
	volumesFiles["skupper-internal"] = map[string]string{
		"skrouterd.json": skrouterdJson,
	}
	volumesFiles["skupper-console-users"] = map[string]string{
		"admin": "admin",
	}
	volumesFiles["skupper-services"] = map[string]string{
		"skupper-services.json": skupperServicesJson,
	}

	// defining skupper-services configmap
	return volumes, volumesFiles
}

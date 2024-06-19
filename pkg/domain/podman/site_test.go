//go:build podman
// +build podman

package podman

import (
	"context"
	_ "embed"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/client/podman"
	"github.com/skupperproject/skupper/pkg/container"
	"github.com/skupperproject/skupper/pkg/domain"
	"github.com/skupperproject/skupper/pkg/images"
	"github.com/skupperproject/skupper/pkg/utils"
	"gotest.tools/assert"
)

var (
	//go:embed skrouterd.json
	skrouterdJson string
	//go:embed skupper-services.json
	skupperServicesJson string
)

func TestSiteHandler(t *testing.T) {
	siteHandler, err := NewSitePodmanHandler(getEndpoint())
	assert.Assert(t, err)

	scenarios := []struct {
		name string
		site domain.Site
	}{{
		name: "basic-ingress-localhost",
		site: newBasicSite(),
	}, {
		name: "basic-ingress-none",
		site: &Site{
			SiteCommon: &domain.SiteCommon{
				Name: "site-podman-no-ingress",
			},
		},
	}, {
		name: "flow-collector-internal-auth-ingress-localhost",
		site: &Site{
			SiteCommon: &domain.SiteCommon{
				Name: "site-podman-fc-ingress",
			},
			IngressHosts:        []string{"127.0.0.1"},
			EnableFlowCollector: true,
			EnableConsole:       true,
			AuthMode:            "internal",
			ConsoleUser:         "internal",
			ConsolePassword:     "internal",
			PrometheusOpts: types.PrometheusServerOptions{
				ExternalServer: "http://10.0.0.1:8080/v1",
				AuthMode:       "internal",
				User:           "admin",
				Password:       "admin",
			},
		},
	}, {
		name: "flow-collector-internal-auth-ingress-none",
		site: &Site{
			SiteCommon: &domain.SiteCommon{
				Name: "site-podman-fc-no-ingress",
			},
			EnableFlowCollector: true,
			EnableConsole:       true,
			AuthMode:            "internal",
			ConsoleUser:         "internal",
			ConsolePassword:     "internal",
		},
	}}

	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			t.Logf("creating site")
			err = siteHandler.Create(context.Background(), scenario.site)
			assert.Assert(t, err)

			// remove site
			defer func() {
				t.Logf("removing site")
				err = siteHandler.Delete()
				assert.Assert(t, err)
				site, err := siteHandler.Get()
				assert.Assert(t, err != nil)
				assert.Assert(t, site == nil)
			}()

			// Verifying site
			t.Logf("retrieving site")
			site, err := siteHandler.Get()
			assert.Assert(t, err)
			podmanSite := site.(*Site)

			t.Logf("validating site info")
			scenarioSite := scenario.site.(*Site)
			assert.Equal(t, podmanSite.GetName(), scenarioSite.GetName())
			assert.Equal(t, podmanSite.GetMode(), utils.DefaultStr(scenarioSite.GetMode(), "interior"))
			assert.Equal(t, podmanSite.ContainerNetwork, utils.DefaultStr(scenarioSite.ContainerNetwork, container.ContainerNetworkName))
			// number of expected ingress hosts
			expIngHosts := 1 + len(scenarioSite.IngressHosts)
			expDeployments := 2
			if scenarioSite.EnableFlowCollector {
				expDeployments += 2
			}
			assert.Assert(t, len(podmanSite.IngressHosts) == expIngHosts)
			assert.Equal(t, len(podmanSite.Deployments), expDeployments)
			for _, dep := range podmanSite.GetDeployments() {
				assert.Assert(t, len(dep.GetComponents()) > 0, "no components found for %s", dep.GetName())
				for _, cmp := range dep.GetComponents() {
					var cmpContainer *container.Container
					err = utils.Retry(time.Second*6, 10, func() (bool, error) {
						cmpContainer, err = siteHandler.cli.ContainerInspect(cmp.Name())
						if err != nil {
							return true, err
						}
						return cmpContainer.Running, nil
					})
					assert.Assert(t, err, "error retrieving container info")
					assert.Assert(t, cmpContainer.Running, "component %s is not running - exit code: %d "+
						"- restarts: %d", cmpContainer.Name, cmpContainer.ExitCode, cmpContainer.RestartCount)
				}
			}
			assert.Assert(t, len(podmanSite.Credentials) > 0)
			assert.Assert(t, len(podmanSite.CertAuthorities) > 0)
			assert.Equal(t, scenarioSite.PrometheusOpts.ExternalServer, podmanSite.PrometheusOpts.ExternalServer)
			assert.Equal(t, scenarioSite.PrometheusOpts.AuthMode, podmanSite.PrometheusOpts.AuthMode)
			assert.Equal(t, scenarioSite.PrometheusOpts.User, podmanSite.PrometheusOpts.User)
			assert.Equal(t, scenarioSite.PrometheusOpts.Password, podmanSite.PrometheusOpts.Password)
		})
	}
}

func TestSiteHandlerDeleteBrokenSite(t *testing.T) {
	siteHandler, err := NewSitePodmanHandler(getEndpoint())
	assert.Assert(t, err)

	// counting containers and volumes
	countContainersAndVolumes := func() (int, int) {
		// Saving number of containers and volumes before initializing skupper
		// Asserting number of container and volumes are > 1
		containers, err := siteHandler.cli.ContainerList()
		containerCount := len(containers)
		assert.Assert(t, err)
		volumes, err := siteHandler.cli.VolumeList()
		volumeCount := len(volumes)
		assert.Assert(t, err)
		return containerCount, volumeCount
	}

	// retrieve totals before site creation
	containersBefore, volumesBefore := countContainersAndVolumes()

	// Initializing skupper
	site := &Site{
		SiteCommon: &domain.SiteCommon{
			Name: "site-podman-fc-ingress",
		},
		IngressHosts:        []string{"127.0.0.1"},
		EnableFlowCollector: true,
		EnableConsole:       true,
		AuthMode:            "internal",
		ConsoleUser:         "internal",
		ConsolePassword:     "internal",
		PrometheusOpts: types.PrometheusServerOptions{
			ExternalServer: "http://10.0.0.1:8080/v1",
			AuthMode:       "internal",
			User:           "admin",
			Password:       "admin",
		},
	}

	// Create a podman site
	err = siteHandler.Create(context.Background(), site)
	assert.Assert(t, err)

	// remove site
	defer func() {
		_ = siteHandler.Delete()
	}()

	// Validating container and volume counts after creation
	containersAfterCreate, volumesAfterCreate := countContainersAndVolumes()
	assert.Equal(t, containersAfterCreate, containersBefore+4)
	assert.Equal(t, volumesAfterCreate, volumesBefore+15)

	//
	// Removing mandatory volume
	err = siteHandler.cli.VolumeRemove(types.TransportConfigMapName)
	assert.Assert(t, err)

	// Forcing site to be in a bad state
	err = siteHandler.Delete()
	assert.Assert(t, err)

	// Asserting number of containers and volumes are back to original state
	containersAfterDelete, volumesAfterDelete := countContainersAndVolumes()
	assert.Equal(t, containersBefore, containersAfterDelete)
	assert.Equal(t, volumesBefore, volumesAfterDelete)
}

func TestSiteHandlerCreateSiteMock(t *testing.T) {
	var err error
	cli := podman.NewPodmanClientMock([]*container.Container{})
	mock := cli.RestClient.(*podman.RestClientMock)
	sh := NewSitePodmanHandlerFromCli(cli)

	// clean up mock volumes (created as temporary directories)
	defer func() {
		for volumeName, volume := range mock.Volumes {
			if strings.HasPrefix(volume.Source, os.TempDir()+"/") {
				t.Logf("removing tempdir used by mock volume %s: %s", volumeName, volume.Source)
				_ = os.RemoveAll(volume.Source)
			}
		}
	}()

	// Initializing skupper
	site := &Site{
		SiteCommon: &domain.SiteCommon{
			Name: "mock-site",
		},
		IngressHosts:        []string{"127.0.0.1"},
		EnableFlowCollector: true,
		EnableConsole:       true,
		AuthMode:            "internal",
		ConsoleUser:         "internal",
		ConsolePassword:     "internal",
		RouterOpts: types.RouterOptions{
			Tuning: types.Tuning{
				CpuLimit:    "4",
				MemoryLimit: "4096",
			},
		},
		ControllerOpts: types.ControllerOptions{
			Tuning: types.Tuning{
				CpuLimit:    "3",
				MemoryLimit: "3072",
			},
		},
		FlowCollectorOpts: types.FlowCollectorOptions{
			Tuning: types.Tuning{
				CpuLimit:    "2",
				MemoryLimit: "2048",
			},
		},
		PrometheusOpts: types.PrometheusServerOptions{
			Tuning: types.Tuning{
				CpuLimit:    "1",
				MemoryLimit: "1024",
			},
			ExternalServer: "http://10.0.0.1:8080/v1",
			AuthMode:       "internal",
			User:           "admin",
			Password:       "admin",
		},
	}
	err = sh.Create(context.Background(), site)
	assert.Assert(t, err)

	// validating mocked site creation
	siteCreated, err := sh.Get()
	assert.Assert(t, err)

	site = siteCreated.(*Site)
	assert.Equal(t, "mock-site", site.GetName())
	assert.Equal(t, "127.0.0.1", site.IngressHosts[1])
	assert.Equal(t, true, site.EnableFlowCollector)
	assert.Equal(t, true, site.EnableConsole)
	assert.Equal(t, "internal", site.AuthMode)
	assert.Equal(t, "internal", site.ConsoleUser)
	assert.Equal(t, "internal", site.ConsolePassword)
	assert.DeepEqual(t, types.RouterOptions{
		Tuning: types.Tuning{
			CpuLimit:    "4",
			MemoryLimit: "4096",
		},
		MaxFrameSize:     16384,
		MaxSessionFrames: 640,
	}, site.RouterOpts)
	assert.DeepEqual(t, types.ControllerOptions{
		Tuning: types.Tuning{
			CpuLimit:    "3",
			MemoryLimit: "3072",
		},
	}, site.ControllerOpts)
	assert.DeepEqual(t, types.FlowCollectorOptions{
		Tuning: types.Tuning{
			CpuLimit:    "2",
			MemoryLimit: "2048",
		},
	}, site.FlowCollectorOpts)
	assert.DeepEqual(t, types.PrometheusServerOptions{
		Tuning: types.Tuning{
			CpuLimit:    "1",
			MemoryLimit: "1024",
		},
		ExternalServer: "http://10.0.0.1:8080/v1",
		AuthMode:       "internal",
		User:           "admin",
		Password:       "admin",
		PodAnnotations: map[string]string{},
	}, site.PrometheusOpts)
}

func TestSiteHandlerDeleteBrokenSiteMock(t *testing.T) {
	cli := podman.NewPodmanClientMock(mockContainers())
	mock := cli.RestClient.(*podman.RestClientMock)
	mockedVolumes, mockedVolumeFiles := mockVolumes()
	origLenMockedVolumes := len(mockedVolumes)
	assert.Assert(t, mock.MockVolumeFiles(mockedVolumes, mockedVolumeFiles))
	defer func() {
		_ = mock.CleanupMockVolumeDir()
	}()

	// must NOT be removed after SiteHandler.Delete() is called
	mock.Volumes["my-volume"] = &container.Volume{
		Name: "my-volume",
		Labels: map[string]string{
			"my-label": "my-value",
		},
	}

	sh := NewSitePodmanHandlerFromCli(cli)
	site, err := sh.Get()
	assert.Assert(t, err)
	assert.Assert(t, site != nil)

	// verify mock site is in good state
	assert.Assert(t, err)

	// validating number of containers and volumes before removal
	containers, err := cli.ContainerList()
	assert.Assert(t, err)
	assert.Equal(t, len(containers), 5)
	volumes, err := cli.VolumeList()
	assert.Assert(t, err)
	assert.Equal(t, len(volumes), origLenMockedVolumes+1)

	// force a site get to be in a bad state
	delete(mock.Volumes, types.TransportConfigMapName)
	site, err = sh.Get()
	assert.Assert(t, err != nil)
	assert.Assert(t, site == nil)

	// validating delete removed remaining resources
	err = sh.Delete()
	assert.Assert(t, err)

	// assert container not owned by skupper remains
	containers, err = cli.ContainerList()
	assert.Assert(t, err)
	assert.Equal(t, len(containers), 1)

	// assert volume not owned by skupper remains
	volumes, err = cli.VolumeList()
	assert.Assert(t, err)
	assert.Equal(t, len(volumes), 1)

}

func mockContainers() []*container.Container {
	return []*container.Container{
		{
			ID:    strings.Replace(uuid.New().String(), "-", "", -1),
			Name:  "skupper-router",
			Image: images.GetRouterImageName(),
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
			Image: images.GetControllerPodmanImageName(),
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
			Image: images.GetFlowCollectorImageName(),
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
			Image: images.GetRouterImageName(),
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
	addSkupperVolume("skupper-network-status")
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

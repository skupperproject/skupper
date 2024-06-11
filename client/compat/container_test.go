package compat

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-openapi/runtime"
	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/client/container"
	"github.com/skupperproject/skupper/client/generated/libpod/client/containers_compat"
	"github.com/skupperproject/skupper/pkg/images"
	"github.com/skupperproject/skupper/pkg/utils"
	"gotest.tools/assert"
	"k8s.io/apimachinery/pkg/util/rand"
)

func RandomName(name string) string {
	res := name
	if !strings.HasSuffix(name, "-") {
		res = res + "-"
	}
	return res + rand.String(5)
}

func TestContainer(t *testing.T) {
	var err error
	ctx, cancel := context.WithCancel(context.Background())
	cli, wg := NewClientOrSkip(t, "", ctx)
	defer wg.Wait()
	defer cancel()

	name := RandomName("skupper-test")

	image := images.GetServiceControllerImageName()
	env := map[string]string{
		"VAR1": "VAL1",
		"VAR2": "VAL2",
	}
	labels := map[string]string{
		"label1": "value1",
		"label2": "value2",
	}
	annotations := map[string]string{
		"annotation1": "value1",
		"annotation2": "value2",
	}
	aliases := []string{"name1", "name2"}

	sourcePort := "12345"
	targetPort := "54321"
	hostIP := "0.0.0.0"
	protocol := "tcp"

	testFile := "test.txt"
	mountDestination := "/opt/volume"

	// Creating network
	t.Run("network-create-ipv6", func(t *testing.T) {
		network, err := cli.NetworkCreate(&container.Network{
			Name:     name,
			IPV6:     true,
			DNS:      false,
			Internal: true,
			Labels:   labels,
		})
		assert.Assert(t, err, "error creating network")
		assert.Equal(t, name, network.Name)
		assert.Equal(t, true, network.IPV6)
		assert.Equal(t, false, network.DNS)
		assert.Equal(t, true, network.Internal)
		ValidateMaps(t, labels, network.Labels)
		assert.Assert(t, network.Labels["application"] == types.AppName)
	})
	t.Run("network-remove", func(t *testing.T) {
		assert.Assert(t, cli.NetworkRemove(name), "error removing network")
	})
	t.Run("network-create", func(t *testing.T) {
		network, err := cli.NetworkCreate(&container.Network{
			Name:     name,
			DNS:      true,
			Internal: false,
			Labels:   labels,
		})
		assert.Assert(t, err, "error creating network")
		assert.Equal(t, name, network.Name)
		assert.Equal(t, false, network.IPV6)
		assert.Equal(t, true, network.DNS)
		assert.Equal(t, false, network.Internal)
		ValidateMaps(t, labels, network.Labels)
		assert.Assert(t, network.Labels["application"] == types.AppName)
	})

	// Creating volume
	t.Run("volume-create", func(t *testing.T) {
		vol, err := cli.VolumeCreate(&container.Volume{
			Name:   name,
			Labels: labels,
		})
		assert.Assert(t, err)
		assert.Equal(t, name, vol.Name)
		ValidateMaps(t, labels, vol.Labels)
		assert.Assert(t, vol.Labels["application"] == types.AppName)

		_, err = vol.CreateFile(testFile, []byte("test content"), false)
		assert.Assert(t, err, "error creating file test.txt inside volume")
	})

	// Pulling image
	t.Run("image-pull", func(t *testing.T) {
		assert.Assert(t, cli.ImagePull(ctx, image))
		invalidImage := strings.Replace(images.GetSiteControllerImageName(), ":main", ":invalid", 1)
		invalidImageErr := cli.ImagePull(ctx, invalidImage)
		assert.Assert(t, invalidImageErr != nil)
		assert.Assert(t, strings.Contains(invalidImageErr.Error(), "Recommendation:"))
	})
	t.Run("image-pull-timeout", func(t *testing.T) {
		expCtx, expCn := context.WithTimeout(context.Background(), time.Millisecond)
		time.Sleep(time.Millisecond)
		defer expCn()
		expErr := cli.ImagePull(expCtx, image)
		assert.ErrorContains(t, expErr, "context deadline exceeded")
	})

	// Creating container
	t.Run("container-create", func(t *testing.T) {
		err = cli.ContainerCreate(&container.Container{
			Name:   name,
			Image:  image,
			Env:    env,
			Labels: labels,
			Networks: map[string]container.ContainerNetworkInfo{
				name: {
					Aliases: aliases,
				},
			},
			Mounts: []container.Volume{
				{Name: name, Destination: mountDestination},
			},
			Ports: []container.Port{
				{Host: sourcePort, HostIP: hostIP, Target: targetPort, Protocol: protocol},
			},
			Command: []string{"tail", "-f", "/dev/null"},
		})
		assert.Assert(t, err, "error creating container")
		assert.Assert(t, cli.ContainerStart(name))
	})

	// Listing containers
	containerInspectTest := func(t *testing.T) {
		c, err := cli.ContainerInspect(name)
		assert.Assert(t, err, "error inspecting container")

		assert.Equal(t, name, c.Name)
		assert.Equal(t, image, c.Image)
		ValidateMaps(t, env, c.Env)
		ValidateMaps(t, labels, c.Labels)
		assert.Assert(t, c.Labels["application"] == types.AppName)
		ValidateStrings(t, aliases, c.NetworkAliases()[name])
		assert.Equal(t, 1, len(c.Mounts))
		mount := c.Mounts[0]
		assert.Equal(t, name, mount.Name)
		assert.Equal(t, mountDestination, mount.Destination)
		_, err = os.Stat(path.Join(mount.Source, testFile))
		assert.Assert(t, err, "test file has not been created under volume")
		assert.Equal(t, 1, len(c.Ports))
		p := c.Ports[0]
		assert.Equal(t, sourcePort, p.Host)
		assert.Equal(t, hostIP, p.HostIP)
		assert.Equal(t, targetPort, p.Target)
	}
	t.Run("container-inspect", containerInspectTest)

	t.Run("container-exec", func(t *testing.T) {
		out, err := cli.ContainerExec(name, strings.Split("cat /etc/services", " "))
		assert.Assert(t, err)
		assert.Assert(t, len(out) > 1)
		cmd := exec.CommandContext(ctx, "podman", strings.Split(fmt.Sprintf("exec %s cat /etc/services", name), " ")...)
		stdout := &bytes.Buffer{}
		cmd.Stdout = stdout
		assert.Assert(t, cmd.Run())
		assert.Assert(t, len(stdout.String()) > 1)
		assert.Equal(t, out, stdout.String())
	})

	// Disconnecting container from network
	t.Run("network-disconnect", func(t *testing.T) {
		assert.Assert(t, cli.NetworkDisconnect(name, name))
		c, err := cli.ContainerInspect(name)
		assert.Assert(t, err)
		assert.Equal(t, 0, len(c.NetworkNames()))
	})

	// Connecting container to network
	t.Run("network-connect", func(t *testing.T) {
		assert.Assert(t, cli.NetworkConnect(name, name, aliases...))
		c, err := cli.ContainerInspect(name)
		assert.Assert(t, err)
		assert.Equal(t, 1, len(c.NetworkNames()))
		assert.Equal(t, name, c.NetworkNames()[0])
	})

	// Updating container image
	image = strings.ReplaceAll(image, ":main", ":latest")
	t.Run("container-update-image", func(t *testing.T) {
		c, err := cli.ContainerUpdateImage(ctx, name, image)
		assert.Assert(t, err)
		assert.Equal(t, c.Image, image)
		containerInspectTest(t)
	})
	t.Run("container-inspect-after-image-update", containerInspectTest)

	t.Run("container-logs", func(t *testing.T) {
		clogsName := RandomName("skupper-test")
		assert.Assert(t, cli.ImagePull(ctx, images.GetRouterImageName()))
		err = cli.ContainerCreate(&container.Container{
			Name:        clogsName,
			Image:       images.GetRouterImageName(),
			Env:         env,
			Labels:      labels,
			Annotations: annotations,
		})
		assert.Assert(t, cli.ContainerStart(clogsName))
		time.Sleep(time.Second * 5)
		assert.Assert(t, cli.ContainerStop(clogsName))
		logs, err := cli.ContainerLogs(clogsName)
		assert.Assert(t, err)
		assert.Assert(t, len(logs) > 1)
		cmd := exec.CommandContext(ctx, "podman", strings.Split(fmt.Sprintf("logs %s", clogsName), " ")...)
		stderr := &bytes.Buffer{}
		cmd.Stderr = stderr
		assert.Assert(t, cmd.Run())
		assert.Assert(t, len(stderr.String()) > 1)
		assert.Equal(t, logs, stderr.String())
		assert.Assert(t, cli.ContainerRemove(clogsName))
	})

	// Cleaning up
	t.Run("cleanup", func(t *testing.T) {
		var cleanupErrors []error
		cleanupErrors = append(cleanupErrors, cli.ContainerStop(name))
		cleanupErrors = append(cleanupErrors, cli.ContainerRemove(name))
		cleanupErrors = append(cleanupErrors, cli.VolumeRemove(name))
		cleanupErrors = append(cleanupErrors, cli.NetworkRemove(name))
		for _, e := range cleanupErrors {
			assert.Assert(t, e)
		}
	})
}

func NewClientOrSkip(t *testing.T, endpoint string, ctx context.Context) (*CompatClient, *sync.WaitGroup) {
	var cli *CompatClient
	var err error
	var wg *sync.WaitGroup
	if endpoint == "" {
		endpoint, wg = StartPodmanService(t, ctx, false)
	} else {
		wg = new(sync.WaitGroup)
		wg.Done()
	}
	err = utils.RetryError(time.Second, 10, func() error {
		cli, err = NewCompatClient(endpoint, "")
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		t.Skipf("podman service is not available")
	}
	if _, err := cli.Version(); err != nil {
		t.Skipf("podman service is not available")
	}
	return cli, wg
}

func ValidateMaps(t *testing.T, originalMap map[string]string, finalMap map[string]string) {
	for k, v := range originalMap {
		assert.Equal(t, finalMap[k], v)
	}
}

func ValidateStrings(t *testing.T, original []string, final []string) {
	for _, v := range original {
		assert.Assert(t, utils.StringSliceContains(final, v), "string not found %s", v)
	}
}

func TestContainerCreateMock(t *testing.T) {
	cli := NewCompatClientMock([]*container.Container{})
	err := cli.ContainerCreate(&container.Container{
		Name:           "sample-container",
		Image:          "sample-image",
		MaxCpus:        2,
		MaxMemoryBytes: 1024 * 1024 * 1024,
	})
	assert.Assert(t, err)

	ci, err := cli.ContainerInspect("sample-container")
	assert.Assert(t, err)

	assert.Equal(t, 2, ci.MaxCpus)
	assert.Equal(t, int64(1024*1024*1024), ci.MaxMemoryBytes)
}

func TestContainerUpdateMock(t *testing.T) {
	image := images.GetServiceControllerImageName()
	cli := NewCompatClientMock(mockContainers(image))

	// starting the container
	assert.Assert(t, cli.ContainerStart("my-container"))

	// retrieving container
	cc, err := cli.ContainerInspect("my-container")
	assert.Assert(t, err)
	startedAt := cc.StartedAt
	cl, err := cli.ContainerList()
	assert.Assert(t, err)
	assert.Equal(t, len(cl), 1)

	newImage := strings.Replace(image, ":main", ":updated", -1)
	_, err = cli.ContainerUpdateImage(context.Background(), "my-container", newImage)
	assert.Assert(t, err)

	// validating that the container has the new image and has been restarted
	cc, err = cli.ContainerInspect("my-container")
	assert.Assert(t, err)
	assert.Assert(t, time.Now().After(startedAt))
	assert.Equal(t, cc.Image, newImage)

	// assert that there is no other container left
	cl, err = cli.ContainerList()
	assert.Assert(t, err)
	assert.Equal(t, len(cl), 1)
}

func TestContainerUpdateErrorMock(t *testing.T) {
	image := images.GetServiceControllerImageName()
	newImage := strings.Replace(image, ":main", ":updated", -1)
	cli := NewCompatClientMock(mockContainers(image))
	mock := cli.RestClient.(*RestClientMock)

	tests := []struct {
		name      string
		errorHook func(operation *runtime.ClientOperation) error
	}{{
		// must stop and remove the new container
		name: "error-stopping-old-container",
		errorHook: func(operation *runtime.ClientOperation) error {
			if operation.ID == "ContainerStop" {
				params := operation.Params.(*containers_compat.ContainerStopParams)
				// returns error when stopping the current container
				if params.Name == "my-container" {
					return fmt.Errorf("error stopping %q", params.Name)
				}
			}
			return nil
		},
	}, {
		// must start the old container, stop and remove the new one
		name: "error-renaming-old-container",
		errorHook: func(operation *runtime.ClientOperation) error {
			if operation.ID == "ContainerRename" {
				params := operation.Params.(*containers_compat.ContainerRenameParams)
				curName := params.PathName
				newName := params.QueryName
				// returns error when renaming current container to a backup name
				if curName == "my-container" {
					return fmt.Errorf("error renaming container %q to %q", curName, newName)
				}
			}
			return nil
		},
	}, {
		// must rename old container back, start the old container, stop and remove the new one
		name: "error-renaming-new-container-as-old-container",
		errorHook: func(operation *runtime.ClientOperation) error {
			if operation.ID == "ContainerRename" {
				params := operation.Params.(*containers_compat.ContainerRenameParams)
				curName := params.PathName
				newName := params.QueryName
				// return error when renaming new container to original name
				if strings.Contains(curName, "-new-") && newName == "my-container" {
					return fmt.Errorf("error renaming container %q to %q", curName, newName)
				}
			}
			return nil
		},
	}, {
		// must rename old container back, start the old container, stop and remove the new one
		name: "error-starting-new-container",
		errorHook: func(operation *runtime.ClientOperation) error {
			if operation.ID == "ContainerStart" {
				params := operation.Params.(*containers_compat.ContainerStartParams)
				hasBackupContainer := true
				for _, c := range mock.Containers {
					if strings.Contains(c.Name, "-new-") {
						hasBackupContainer = false
						break
					}
				}
				// only returns error when starting the updated container
				if len(mock.Containers) == 2 && hasBackupContainer && params.Name == "my-container" {
					return fmt.Errorf("error starting %q", params.Name)
				}
			}
			return nil
		},
	}}

	// iterate through test scenarios
	for _, test := range tests {
		mock.Containers = mockContainers(image)
		mock.ErrorHook = test.errorHook
		t.Run(test.name, func(t *testing.T) {
			_, err := cli.ContainerUpdate("my-container", func(newContainer *container.Container) {
				newContainer.Image = newImage
			})
			assert.Assert(t, err != nil)
			assert.Equal(t, 1, len(mock.Containers))
			assert.Equal(t, "my-container", mock.Containers[0].Name)
			assert.Equal(t, true, mock.Containers[0].Running)
		})
	}
}

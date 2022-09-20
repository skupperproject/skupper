package podman

import (
	"os"
	"path"
	"strings"
	"testing"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/client"
	"github.com/skupperproject/skupper/client/container"
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
	cli := NewClientOrSkip(t)

	name := RandomName("skupper-test")

	image := client.GetServiceControllerImageName()
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
	t.Run("network-create", func(t *testing.T) {
		network, err := cli.NetworkCreate(&container.Network{
			Name:     name,
			DNS:      true,
			Internal: false,
			Labels:   labels,
		})
		assert.Assert(t, err, "error creating network")
		assert.Equal(t, name, network.Name)
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
		assert.Assert(t, cli.ImagePull(image))
	})

	// Creating container
	t.Run("container-create", func(t *testing.T) {
		err = cli.ContainerCreate(&container.Container{
			Name:        name,
			Image:       image,
			Env:         env,
			Labels:      labels,
			Annotations: annotations,
			Networks: map[string]container.ContainerNetworkInfo{
				name: container.ContainerNetworkInfo{
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
	t.Run("container-inspect", func(t *testing.T) {
		c, err := cli.ContainerInspect(name)
		assert.Assert(t, err, "error inspecting container")

		assert.Equal(t, name, c.Name)
		assert.Equal(t, image, c.Image)
		ValidateMaps(t, env, c.Env)
		ValidateMaps(t, labels, c.Labels)
		assert.Assert(t, c.Labels["application"] == types.AppName)
		ValidateMaps(t, annotations, c.Annotations)
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
	})

	t.Run("container-exec", func(t *testing.T) {
		out, err := cli.ContainerExec(name, strings.Split("ls -1 /app", " "))
		assert.Assert(t, err)
		assert.Assert(t, strings.Contains(out, "service-controller"))
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

func NewClientOrSkip(t *testing.T) *PodmanRestClient {
	cli, err := NewPodmanClient("", "")
	if err != nil {
		t.Skipf("podman service is not available")
	}
	if _, err := cli.Version(); err != nil {
		t.Skipf("podman service is not available")
	}
	return cli
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

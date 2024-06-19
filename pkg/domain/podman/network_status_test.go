//go:build podman
// +build podman

package podman

import (
	_ "embed"
	"encoding/json"
	"os"
	"reflect"
	"testing"

	"github.com/skupperproject/skupper/client/podman"
	"github.com/skupperproject/skupper/pkg/container"
	"github.com/skupperproject/skupper/pkg/network"
	"gotest.tools/assert"
	"k8s.io/apimachinery/pkg/util/rand"
)

//go:embed network_status_sample_data.json
var sampleNetworkStatusData string

func TestNetworkStatusHandlerExternal(t *testing.T) {
	cli, err := podman.NewPodmanClient("", "")
	assert.Assert(t, err)
	handler := new(NetworkStatusHandler).WithClient(cli)

	suffix := rand.String(5)
	NetworkStatusVolume = "skupper-network-status-" + suffix

	volume, err := cli.VolumeCreate(&container.Volume{Name: NetworkStatusVolume})
	assert.Assert(t, err)
	defer func() {
		_ = cli.VolumeRemove(NetworkStatusVolume)
	}()

	// Assert volume is empty
	_, err = volume.ReadFile(NetworkStatusFile)
	assert.Assert(t, err != nil)

	// Loading sample data
	var sampleNetworkStatus = new(network.NetworkStatusInfo)
	assert.Assert(t, json.Unmarshal([]byte(sampleNetworkStatusData), sampleNetworkStatus))

	// Updating externally (through libpod)
	assert.Assert(t, handler.Update(sampleNetworkStatusData))

	// Assert data has been created and has the correct content inside the podman volume
	networkStatusData, err := volume.ReadFile(NetworkStatusFile)
	assert.Assert(t, err)
	assert.Assert(t, networkStatusData != "")
	assert.Equal(t, sampleNetworkStatusData, networkStatusData)
	networkStatus := new(network.NetworkStatusInfo)
	assert.Assert(t, json.Unmarshal([]byte(networkStatusData), networkStatus))
	assert.Assert(t, reflect.DeepEqual(networkStatus, sampleNetworkStatus))

	// Validating the Get method
	networkStatus, err = handler.Get()
	assert.Assert(t, err)
	assert.Assert(t, reflect.DeepEqual(networkStatus, sampleNetworkStatus))
}

func TestNetworkStatusHandlerInContainer(t *testing.T) {
	var err error
	NetworkStatusMountPoint, err = os.MkdirTemp(os.TempDir(), "skupper-network-status.")
	assert.Assert(t, err)
	defer os.RemoveAll(NetworkStatusMountPoint)

	handler := new(NetworkStatusHandler)

	// Creating the network status json file
	assert.Assert(t, handler.Update(sampleNetworkStatusData))
	networkStatusData, err := os.ReadFile(NetworkStatusFileInternal())
	assert.Assert(t, err)
	networkStatusJson := string(networkStatusData)
	assert.Equal(t, sampleNetworkStatusData, networkStatusJson)

	// Loading NetworkStatusInfo
	var sampleNetworkStatus = new(network.NetworkStatusInfo)
	assert.Assert(t, json.Unmarshal([]byte(sampleNetworkStatusData), sampleNetworkStatus))
	networkStatus, err := handler.Get()
	assert.Assert(t, err)
	assert.Assert(t, reflect.DeepEqual(sampleNetworkStatus, networkStatus))
}

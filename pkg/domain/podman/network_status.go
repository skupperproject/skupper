package podman

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/rogpeppe/go-internal/lockedfile"
	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/client/podman"
	"github.com/skupperproject/skupper/pkg/container"
	"github.com/skupperproject/skupper/pkg/network"
)

var (
	NetworkStatusVolume       = types.NetworkStatusConfigMapName
	NetworkStatusMountPoint   = "/etc/skupper-network-status"
	NetworkStatusFileInternal = func() string { return NetworkStatusMountPoint + "/" + NetworkStatusFile }
)

const (
	NetworkStatusFile = "skupper-network-status.json"
)

type NetworkStatusHandler struct {
	cli *podman.PodmanRestClient
}

func (n *NetworkStatusHandler) WithClient(cli *podman.PodmanRestClient) *NetworkStatusHandler {
	n.cli = cli
	return n
}

func (n *NetworkStatusHandler) isRunningInContainer() bool {
	return n.cli == nil || n.cli.IsRunningInContainer()
}

func (n *NetworkStatusHandler) Update(networkStatusJson string) error {
	if n.isRunningInContainer() {
		return n.updateInContainer([]byte(networkStatusJson))
	}
	// Updating through libpod
	networkStatusVol, err := n.cli.VolumeInspect(NetworkStatusVolume)
	if err != nil {
		return fmt.Errorf("error retrieving %s volume: %s", NetworkStatusVolume, err)
	}
	_, err = networkStatusVol.CreateFile(NetworkStatusFile, []byte(networkStatusJson), true)
	return err
}

func (n *NetworkStatusHandler) updateInContainer(networkStatusJson []byte) error {
	networkStatusLockFile := NetworkStatusFileInternal() + ".lock"
	unlockFn, err := lockedfile.MutexAt(networkStatusLockFile).Lock()
	if err != nil {
		return fmt.Errorf("unable to unlock %s: %s", networkStatusLockFile, err)
	}
	defer unlockFn()
	defer func() {
		_ = os.Remove(networkStatusLockFile)
	}()
	f, err := os.Create(NetworkStatusFileInternal())
	if err != nil {
		return fmt.Errorf("error opening file %s: %s", NetworkStatusFileInternal(), err)
	}
	defer f.Close()
	_, err = f.Write(networkStatusJson)
	if err != nil {
		return fmt.Errorf("error writing to %s: %s", NetworkStatusFileInternal(), err)
	}
	return nil
}

func (n *NetworkStatusHandler) Get() (*network.NetworkStatusInfo, error) {
	var networkStatusJson string
	var err error
	if n.isRunningInContainer() {
		networkStatusJson, err = n.getInContainer()
	} else {
		var networkStatusVol *container.Volume
		networkStatusVol, err = n.cli.VolumeInspect(NetworkStatusVolume)
		if err != nil {
			return nil, fmt.Errorf("error retrieving %s volume: %s", NetworkStatusVolume, err)
		}
		networkStatusJson, err = networkStatusVol.ReadFile(NetworkStatusFile)
	}
	if err != nil {
		return nil, fmt.Errorf("error reading %s: %s", NetworkStatusVolume, err)
	}
	var networkStatus network.NetworkStatusInfo
	err = json.Unmarshal([]byte(networkStatusJson), &networkStatus)
	return &networkStatus, err
}

func (n *NetworkStatusHandler) getInContainer() (string, error) {
	nsData, err := os.ReadFile(NetworkStatusFileInternal())
	if err != nil {
		return "", err
	}
	return string(nsData), nil
}

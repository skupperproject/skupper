package podman

import (
	"fmt"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/client/container"
	"github.com/skupperproject/skupper/client/generated/libpod/client/volumes"
	"github.com/skupperproject/skupper/client/podman"
	"github.com/skupperproject/skupper/pkg/qdr"
)

type RouterConfigHandler struct {
	cli *podman.PodmanRestClient
}

func NewRouterConfigHandlerPodman(cli *podman.PodmanRestClient) *RouterConfigHandler {
	return &RouterConfigHandler{
		cli: cli,
	}
}

func (r *RouterConfigHandler) GetRouterConfig() (*qdr.RouterConfig, error) {
	var configVolume *container.Volume
	configVolume, err := r.cli.VolumeInspect(types.TransportConfigMapName)
	if err != nil {
		return nil, fmt.Errorf("error retrieving volume %s - %v", types.TransportConfigMapName, err)
	}
	routerConfigStr, err := configVolume.ReadFile(types.TransportConfigFile)
	if err != nil {
		return nil, fmt.Errorf("error reading config file %s from volume %s - %v",
			types.TransportConfigFile, types.TransportConfigMapName, err)
	}
	routerConfig, err := qdr.UnmarshalRouterConfig(routerConfigStr)
	if err != nil {
		return nil, fmt.Errorf("error parsing config file %s from volume %s - %v",
			types.TransportConfigFile, types.TransportConfigMapName, err)
	}
	return &routerConfig, nil
}

func (r *RouterConfigHandler) SaveRouterConfig(config *qdr.RouterConfig) error {
	var configVolume *container.Volume
	configVolume, err := r.cli.VolumeInspect(types.TransportConfigMapName)
	if err != nil {
		if _, notFound := err.(*volumes.VolumeInspectLibpodNotFound); !notFound {
			return fmt.Errorf("error retrieving volume %s - %v", types.TransportConfigMapName, err)
		}
		// try to create volume since not found given
		if configVolume, err = r.cli.VolumeCreate(&container.Volume{Name: types.TransportConfigMapName}); err != nil {
			return fmt.Errorf("error creating volume %s - %v", types.TransportConfigMapName, err)
		}
	}
	routerConfig, err := qdr.MarshalRouterConfig(*config)
	if err != nil {
		return fmt.Errorf("error serializing router config - %v", err)
	}
	_, err = configVolume.CreateFile(types.TransportConfigFile, []byte(routerConfig), true)
	if err != nil {
		return fmt.Errorf("error creating router config - %v", err)
	}
	return nil
}

func (r *RouterConfigHandler) RemoveRouterConfig() error {
	err := r.cli.VolumeRemove(types.TransportConfigMapName)
	if err == nil {
		return nil
	}
	if _, notFound := err.(*volumes.VolumeInspectLibpodNotFound); notFound {
		return nil
	}
	return fmt.Errorf("error removing router config - %v", err)
}

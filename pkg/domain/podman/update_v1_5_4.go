package podman

import (
	"context"
	"fmt"

	"github.com/skupperproject/skupper/api/types"
	clientpodman "github.com/skupperproject/skupper/client/podman"
	"github.com/skupperproject/skupper/pkg/container"
	"github.com/skupperproject/skupper/pkg/domain"
	"github.com/skupperproject/skupper/pkg/utils"
)

type SkupperNetworkStatusVolume struct {
	cli *clientpodman.PodmanRestClient
}

func (m *SkupperNetworkStatusVolume) WithCli(cli *clientpodman.PodmanRestClient) *SkupperNetworkStatusVolume {
	m.cli = cli
	return m
}

func (m *SkupperNetworkStatusVolume) Info() string {
	return "Create and mount the skupper-network-status volume"
}

func (m *SkupperNetworkStatusVolume) AppliesTo(siteVersion string) bool {
	curVersion := utils.ParseVersion(siteVersion)
	return !(&curVersion).IsUndefined() && utils.LessRecentThanVersion(siteVersion, m.Version())
}

func (m *SkupperNetworkStatusVolume) Version() string {
	return "1.5.4"
}

func (m *SkupperNetworkStatusVolume) Priority() domain.UpdatePriority {
	return domain.PriorityNormal
}

func (m *SkupperNetworkStatusVolume) Run(ctx context.Context) *domain.UpdateResult {
	volumeName := types.NetworkStatusConfigMapName
	containerName := types.ControllerPodmanContainerName
	var result = &domain.UpdateResult{}

	_, err := m.cli.ContainerUpdate(containerName, func(newContainer *container.Container) {
		for _, mount := range newContainer.Mounts {
			if mount.Name == volumeName {
				return
			}
		}
		// volume not mounted, creating and mounting
		volume, err := m.cli.VolumeCreate(&container.Volume{Name: volumeName})
		if err != nil && volume == nil {
			result.AddErrors(fmt.Errorf("error creating volume %s: %s", volumeName, err))
		} else if volume == nil {
			result.AddChange(fmt.Sprintf("Volume has been created: %s", volumeName))
		}
		newContainer.Mounts = append(newContainer.Mounts, container.Volume{
			Name:        volumeName,
			Destination: "/etc/skupper-network-status",
		})
		result.AddChange(fmt.Sprintf("Mounted volume %s into %s", volumeName, containerName))
	})
	if err != nil {
		result.AddErrors(fmt.Errorf("error updating container %s: %s", containerName, err))
	}
	return result
}

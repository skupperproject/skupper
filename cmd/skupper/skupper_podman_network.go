package main

import (
	"context"
	"fmt"
	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/pkg/domain/podman"
	"github.com/skupperproject/skupper/pkg/network"
	"github.com/skupperproject/skupper/pkg/utils"
	"github.com/spf13/cobra"
)

type SkupperPodmanNetwork struct {
	podman               *SkupperPodman
	networkStatusHandler *podman.NetworkStatusHandler
}

func (s *SkupperPodmanNetwork) GetCurrentSite(ctx context.Context) (string, error) {

	if s.podman.currentSite == nil {
		return "", fmt.Errorf("Skupper is not enabled")
	}
	return s.podman.currentSite.Id, nil

}

func (s *SkupperPodmanNetwork) Status(cmd *cobra.Command, args []string, ctx context.Context) (*network.NetworkStatusInfo, error) {
	podmanSiteVersion := s.podman.currentSite.Version
	if podmanSiteVersion != "" && !utils.IsValidFor(podmanSiteVersion, network.MINIMUM_PODMAN_VERSION) {
		return nil, fmt.Errorf(network.MINIMUM_VERSION_MESSAGE, podmanSiteVersion, network.MINIMUM_PODMAN_VERSION)
	}

	return s.NetworkStatusHandler().Get()
}

func (s *SkupperPodmanNetwork) StatusFlags(cmd *cobra.Command) {}

func (s *SkupperPodmanNetwork) NewClient(cmd *cobra.Command, args []string) {
	s.podman.NewClient(cmd, args)
}

func (s *SkupperPodmanNetwork) Platform() types.Platform {
	return types.PlatformPodman
}

func (s *SkupperPodmanNetwork) NetworkStatusHandler() *podman.NetworkStatusHandler {
	if s.networkStatusHandler != nil {
		return s.networkStatusHandler
	}
	if s.podman.currentSite == nil {
		return nil
	}
	s.networkStatusHandler = new(podman.NetworkStatusHandler).WithClient(s.podman.cli)
	return s.networkStatusHandler
}

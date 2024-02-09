package main

import (
	"context"
	"fmt"
	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/pkg/network"
	"github.com/skupperproject/skupper/pkg/utils"
	"github.com/spf13/cobra"
)

type SkupperKubeNetwork struct {
	kube *SkupperKube
}

func (s *SkupperKubeNetwork) GetCurrentSite(ctx context.Context) (string, error) {

	siteConfig, err := s.kube.Cli.SiteConfigInspect(ctx, nil)
	if err != nil || siteConfig == nil {

		return "", fmt.Errorf("Skupper is not enabled in namespace: %s", s.kube.Cli.GetNamespace())
	}

	return siteConfig.Reference.UID, nil
}

func (s *SkupperKubeNetwork) NewClient(cmd *cobra.Command, args []string) {
	s.kube.NewClient(cmd, args)
}

func (s *SkupperKubeNetwork) Platform() types.Platform {
	return s.kube.Platform()
}

func (s *SkupperKubeNetwork) Status(cmd *cobra.Command, args []string, ctx context.Context) (*network.NetworkStatusInfo, error) {

	configSyncVersion := utils.GetVersionTag(s.kube.Cli.GetVersion(types.TransportContainerName, types.ConfigSyncContainerName))
	if configSyncVersion != "" && !utils.IsValidFor(configSyncVersion, network.MINIMUM_VERSION) {
		return nil, fmt.Errorf(network.MINIMUM_VERSION_MESSAGE, configSyncVersion, network.MINIMUM_VERSION)
	}

	return s.kube.Cli.NetworkStatus(ctx)

}

func (s *SkupperKubeNetwork) StatusFlags(cmd *cobra.Command) {}

package main

import (
	"context"
	"fmt"
	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/pkg/network"
	"github.com/skupperproject/skupper/pkg/utils"
	"github.com/skupperproject/skupper/pkg/utils/formatter"
	"github.com/spf13/cobra"
	"strconv"
	"strings"
)

type SkupperKubeNetwork struct {
	kube *SkupperKube
}

func (s *SkupperKubeNetwork) NewClient(cmd *cobra.Command, args []string) {
	s.kube.NewClient(cmd, args)
}

func (s *SkupperKubeNetwork) Platform() types.Platform {
	return s.kube.Platform()
}

func (s *SkupperKubeNetwork) Status(cmd *cobra.Command, args []string) error {

	silenceCobra(cmd)

	ctx, cancel := context.WithTimeout(context.Background(), types.DefaultTimeoutDuration)
	defer cancel()

	siteConfig, err := s.kube.Cli.SiteConfigInspect(ctx, nil)
	if err != nil || siteConfig == nil {
		fmt.Printf("The site configuration is not available: %s", err)
		fmt.Println()
		return nil
	}

	configSyncVersion := utils.GetVersionTag(s.kube.Cli.GetVersion(types.TransportContainerName, types.ConfigSyncContainerName))
	if configSyncVersion != "" && !utils.IsValidFor(configSyncVersion, network.MINIMUM_VERSION) {
		fmt.Printf("Site version is %s, but CLI requires %s", configSyncVersion, network.MINIMUM_VERSION)
		fmt.Println()
		return nil
	}

	currentSite := siteConfig.Reference.UID

	currentNetworkStatus, errStatus := s.kube.Cli.NetworkStatus(ctx)
	if errStatus != nil {
		return errStatus
	}

	sitesStatus := currentNetworkStatus.SiteStatus
	statusManager := network.SkupperStatus{NetworkStatus: currentNetworkStatus}

	if sitesStatus != nil && len(sitesStatus) > 0 {

		networkList := formatter.NewList()
		networkList.Item("Sites:")

		for _, siteStatus := range sitesStatus {
			if len(siteNetworkStatus) == 0 || siteNetworkStatus == siteStatus.Site.Name {

				siteVersion := siteStatus.Site.Version
				if len(siteStatus.Site.MinimumVersion) > 0 {
					siteVersion = fmt.Sprintf("%s (minimum version required %s)", siteStatus.Site.Version, siteStatus.Site.MinimumVersion)
				}

				detailsMap := map[string]string{"site name": siteStatus.Site.Name, "namespace": siteStatus.Site.Namespace, "version": siteVersion}

				location := "remote"
				if siteStatus.Site.Identity == currentSite {
					location = "local"
				}

				newItem := fmt.Sprintf("[%s] %s(%s) ", location, siteStatus.Site.Identity, siteStatus.Site.Namespace)
				newItem = newItem + fmt.Sprintln()

				siteLevel := networkList.NewChildWithDetail(newItem, detailsMap)

				if len(siteStatus.RouterStatus) > 0 {

					err, index := statusManager.GetRouterIndex(&siteStatus)
					if err != nil {
						return err
					}

					mapSiteLink := statusManager.GetSiteLinkMapPerRouter(&siteStatus.RouterStatus[index], &siteStatus.Site)

					if len(mapSiteLink) > 0 {
						siteLinks := siteLevel.NewChild("Linked sites:")
						for key, value := range mapSiteLink {
							siteLinks.NewChildWithDetail(fmt.Sprintln(key), map[string]string{"direction": value.Direction})
						}
					}

					if verboseNetworkStatus {
						routers := siteLevel.NewChild("Routers:")
						for _, routerStatus := range siteStatus.RouterStatus {
							routerId := strings.Split(routerStatus.Router.Name, "/")

							// skip routers that belong to headless services
							if len(routerId) > 1 && strings.HasPrefix(routerId[1], siteStatus.Site.Name) {
								routerItem := fmt.Sprintf("name: %s\n", routerId[1])
								detailsRouter := map[string]string{"image name": routerStatus.Router.ImageName, "image version": routerStatus.Router.ImageVersion}

								routerLevel := routers.NewChildWithDetail(routerItem, detailsRouter)

								printableLinks := statusManager.RemoveLinksFromSameSite(routerStatus, siteStatus.Site)

								if len(printableLinks) > 0 {
									links := routerLevel.NewChild("Links:")
									for _, link := range printableLinks {
										linkItem := fmt.Sprintf("name:  %s\n", link.Name)
										detailsLink := map[string]string{"direction": link.Direction}
										if link.LinkCost > 0 {
											detailsLink["cost"] = strconv.FormatUint(link.LinkCost, 10)
										}
										links.NewChildWithDetail(linkItem, detailsLink)
									}
								}
							}
						}
					}
				}
			}
		}

		networkList.Print()
	}

	return nil
}

func (s *SkupperKubeNetwork) StatusFlags(cmd *cobra.Command) {}

package main

import (
	"context"
	"fmt"
	"github.com/skupperproject/skupper/api/types"
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

	if !verboseNetworkStatus && zeroCostNetworkstatus {
		return fmt.Errorf("showing links with zero cost requires the verbose option")
	}
	silenceCobra(cmd)

	ctx, cancel := context.WithTimeout(context.Background(), types.DefaultFlowTimeoutDuration)
	defer cancel()

	siteConfig, err := s.kube.Cli.SiteConfigInspect(ctx, nil)
	if err != nil || siteConfig == nil {
		fmt.Printf("The site configuration is not available: %s", err)
		fmt.Println()
		return nil
	}
	currentSite := siteConfig.Reference.UID

	sitesStatus, errStatus := s.kube.Cli.NetworkStatus(ctx)
	if errStatus != nil {
		return errStatus
	}

	if sitesStatus != nil && len(*sitesStatus) > 0 {
		mapRouterSite := CreateRouterSiteMap(sitesStatus)

		network := formatter.NewList()
		network.Item("Sites:")

		for _, siteStatus := range *sitesStatus {
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

				newItem := fmt.Sprintf("[%s] %s-%s ", location, siteStatus.Site.Namespace, siteStatus.Site.Identity)
				newItem = newItem + fmt.Sprintln()

				siteLevel := network.NewChildWithDetail(newItem, detailsMap)

				if len(siteStatus.RouterStatus) > 0 {

					//to get the generic information about the links of a site, we can get the first router, because in case of multiple routers the information will be the same.
					mapSiteLink := CreateSiteLinkMap(&siteStatus.RouterStatus[0], &siteStatus.Site, mapRouterSite)

					if len(mapSiteLink) > 0 {
						siteLinks := siteLevel.NewChild("Linked with:")
						for key, value := range mapSiteLink {
							siteLinks.NewChildWithDetail(fmt.Sprintln(key), map[string]string{"direction": value.Direction})
						}
					}

					if verboseNetworkStatus {
						routers := siteLevel.NewChild("Routers:")
						for _, routerStatus := range siteStatus.RouterStatus {
							routerId := strings.Split(routerStatus.Router.Name, "/")
							routerItem := fmt.Sprintf("name: %s\n", routerId[1])
							detailsRouter := map[string]string{"image name": routerStatus.Router.ImageName, "image version": routerStatus.Router.ImageVersion}

							routerLevel := routers.NewChildWithDetail(routerItem, detailsRouter)

							printableLinks := FilterLinks(routerStatus, siteStatus.Site, mapRouterSite)

							if len(printableLinks) > 0 {
								links := routerLevel.NewChild("Links:")
								for _, link := range printableLinks {
									linkItem := fmt.Sprintf("name:  %s\n", link.Name)
									detailsLink := map[string]string{"direction": link.Direction, "cost": strconv.FormatUint(link.LinkCost, 10)}
									links.NewChildWithDetail(linkItem, detailsLink)

								}
							}

						}
					}
				}
			}
		}

		network.Print()
	}

	return nil
}

func (s *SkupperKubeNetwork) StatusFlags(cmd *cobra.Command) {}

func CreateRouterSiteMap(sitesStatus *[]types.SiteStatusInfo) map[string]string {
	mapRouterSite := make(map[string]string)
	for _, siteStatus := range *sitesStatus {
		if len(siteStatus.RouterStatus) > 0 {
			for _, routerStatus := range siteStatus.RouterStatus {

				// the name of the router has a "0/" as a prefix that it is needed to remove
				routerName := strings.Split(routerStatus.Router.Name, "/")
				mapRouterSite[routerName[1]] = strings.Join([]string{siteStatus.Site.Namespace, siteStatus.Site.Identity}, "-")
			}
		}
	}

	return mapRouterSite
}

func CreateSiteLinkMap(router *types.RouterStatusInfo, site *types.SiteInfo, mapRouterSite map[string]string) map[string]types.LinkInfo {

	siteLinkMap := make(map[string]types.LinkInfo)
	if len(router.Links) > 0 {
		for _, link := range router.Links {
			// the links between routers of the same site are not shown.
			if !LinkBelongsToSameSite(link.Name, site.Namespace, site.Identity, mapRouterSite) {
				//the attribute "Name" in the link matches the name of the router from the site that is linked with.
				siteIdentifier := mapRouterSite[link.Name]
				siteLinkMap[siteIdentifier] = link
			}
		}

	}

	return siteLinkMap
}

func LinkBelongsToSameSite(linkName string, namespace string, siteId string, routerSiteMap map[string]string) bool {

	return strings.EqualFold(routerSiteMap[linkName], strings.Join([]string{namespace, siteId}, "-"))

}

func FilterLinks(router types.RouterStatusInfo, site types.SiteInfo, mapRouterSite map[string]string) []types.LinkInfo {
	var filteredLinks []types.LinkInfo
	for _, link := range router.Links {
		// avoid showing the links between routers of the same site
		if !LinkBelongsToSameSite(link.Name, site.Namespace, site.Identity, mapRouterSite) {
			//avoid showing links with cost zero by default
			if link.LinkCost != 0 || zeroCostNetworkstatus {
				filteredLinks = append(filteredLinks, link)
			}
		}
	}

	return filteredLinks
}

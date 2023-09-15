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
		mapRouterSite := getRouterSiteMap(sitesStatus)

		network := formatter.NewList()
		network.Item("Sites:")

		for _, siteStatus := range *sitesStatus {
			if len(siteNameNetoworkStatus) == 0 || siteNameNetoworkStatus == siteStatus.Site.Name {

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
					mapSiteLink := getSiteLinkMap(&siteStatus.RouterStatus[0], mapRouterSite)

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

							if len(routerStatus.Links) > 0 {
								links := routerLevel.NewChild("Links:")

								for _, link := range routerStatus.Links {
									// avoid showing the links between routers of the same site
									if !strings.Contains(link.Name, routerStatus.Router.Namespace) {
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
		}

		network.Print()
	}

	return nil
}

func (s *SkupperKubeNetwork) StatusFlags(cmd *cobra.Command) {}

func getRouterSiteMap(sitesStatus *[]types.SiteStatusInfo) map[string]string {
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

func getSiteLinkMap(router *types.RouterStatusInfo, mapRouterSite map[string]string) map[string]*types.LinkInfo {

	siteLinkMap := make(map[string]*types.LinkInfo)
	if len(router.Links) > 0 {
		for _, link := range router.Links {
			// the links between routers of the same site are not shown. those links have the same namespace in their names.
			if !strings.Contains(link.Name, router.Router.Namespace) {
				//the attribute "Name" in the link matches the name of the router from the site that is linked with.
				siteIdentifier := mapRouterSite[link.Name]
				siteLinkMap[siteIdentifier] = &link
			}
		}

	}

	return siteLinkMap
}

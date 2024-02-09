package main

import (
	"context"
	"fmt"
	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/pkg/network"
	"github.com/skupperproject/skupper/pkg/utils/formatter"
	"github.com/spf13/cobra"
	"strconv"
	"strings"
)

func NewCmdNetwork() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "network [command]",
		Short: "Show information about the sites and services included in the network.",
	}
	return cmd
}

var verboseNetworkStatus bool
var siteNetworkStatus string

func NewCmdNetworkStatus(skupperClient SkupperNetworkClient) *cobra.Command {
	cmd := &cobra.Command{
		Use:    "status",
		Short:  "Shows information about the current site, and connected sites.",
		Args:   cobra.MaximumNArgs(1),
		PreRun: skupperClient.NewClient,
		RunE: func(cmd *cobra.Command, args []string) error {
			silenceCobra(cmd)

			ctx, cancel := context.WithTimeout(context.Background(), types.DefaultTimeoutDuration)
			defer cancel()

			currentSiteId, err := skupperClient.GetCurrentSite(ctx)
			if err != nil && strings.HasPrefix(err.Error(), "Skupper is not enabled") {
				fmt.Println(err.Error())
				return nil
			} else if err != nil {
				return err
			}
			currentNetworkStatus, errStatus := skupperClient.Status(cmd, args, ctx)
			if errStatus != nil && errStatus.Error() == "status not ready" {
				fmt.Println("Status pending...")
				return nil
			} else if errStatus != nil && strings.HasPrefix(errStatus.Error(), "Version incompatibility:") {
				fmt.Println(errStatus.Error())
				return nil
			} else if errStatus != nil {
				return errStatus
			}

			return printNetworkStatus(currentSiteId, currentNetworkStatus)
		},
	}

	cmd.Flags().BoolVarP(&verboseNetworkStatus, "verbose", "v", false, "More detailed output about the network topology")
	cmd.Flags().StringVarP(&siteNetworkStatus, "site", "s", "", "Filter by a specific site name")
	return cmd

}

func printNetworkStatus(currentSite string, currentNetworkStatus *network.NetworkStatusInfo) error {
	sitesStatus := currentNetworkStatus.SiteStatus
	statusManager := network.SkupperStatus{NetworkStatus: currentNetworkStatus}

	if sitesStatus != nil && len(sitesStatus) > 0 {

		networkList := formatter.NewList()
		networkList.Item("Sites:")

		for _, siteStatus := range sitesStatus {
			if len(siteNetworkStatus) == 0 || siteNetworkStatus == siteStatus.Site.Name {

				siteVersion := "-"
				if len(siteStatus.Site.Version) > 0 {
					siteVersion = siteStatus.Site.Version
				}

				if len(siteStatus.Site.MinimumVersion) > 0 {
					siteVersion = fmt.Sprintf("%s (minimum version required %s)", siteStatus.Site.Version, siteStatus.Site.MinimumVersion)
				}

				detailsMap := map[string]string{"site name": siteStatus.Site.Name, "namespace": siteStatus.Site.Namespace, "version": siteVersion}

				location := "[remote]"
				if siteStatus.Site.Identity == currentSite {
					location = "[local]"
				} else if strings.HasPrefix(siteStatus.Site.Identity, "gateway") {
					location = ""
				}

				newItem := fmt.Sprintf("%s %s(%s) ", location, siteStatus.Site.Identity, siteStatus.Site.Namespace)
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
							if network.PrintableRouter(routerStatus, siteStatus.Site.Name) {
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

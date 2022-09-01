package main

import (
	"fmt"

	"github.com/skupperproject/skupper/api/types"
	"github.com/spf13/cobra"
)

func NewCmdNetwork() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "network [command]",
		Short: "Show information about the sites and services included in the network.",
	}
	return cmd
}

var selectedSite string

func NewCmdNetworkStatus(skupperClient SkupperClient) *cobra.Command {
	cmd := &cobra.Command{
		Use:    "status",
		Short:  "Shows information about the current site, and connected sites.",
		Args:   cobra.MaximumNArgs(1),
		PreRun: skupperClient.NewClient,
		RunE:   skupperClient.NetworkStatus,
	}

	cmd.Flags().StringVarP(&selectedSite, "site", "s", "all", "Site identifier")

	return cmd

}

func printLocalStatus(readyreplicas int32, warnings []string, totalConnectedSites int, directConnectedSites int, exposedServices int) {

	if readyreplicas == 0 {
		fmt.Printf(" Status pending...")
	} else {
		if len(warnings) > 0 {
			for _, w := range warnings {
				fmt.Printf("Warning: %s", w)
				fmt.Println()
			}
		}
		if totalConnectedSites == 0 {
			fmt.Printf(" It is not connected to any other sites.")
		} else if totalConnectedSites == 1 {
			fmt.Printf(" It is connected to 1 other site.")
		} else if totalConnectedSites == directConnectedSites {
			fmt.Printf(" It is connected to %d other sites.", totalConnectedSites)
		} else {
			fmt.Printf(" It is connected to %d other sites (%d indirectly).", totalConnectedSites, directConnectedSites)
		}
	}
	fmt.Printf(" Number of exposed services: %d", exposedServices)
	fmt.Println()
}

func getLocalSiteInfo(serviceInterfaces []*types.ServiceInterface, siteId string, siteName string, namespace string, version string) []*types.SiteInfo {

	var localServices []types.ServiceInfo

	if len(serviceInterfaces) > 0 {
		for _, service := range serviceInterfaces {

			var localTargets []types.TargetInfo
			if len(service.Targets) > 0 {
				for _, target := range service.Targets {
					targetInfo := types.TargetInfo{
						Name:   target.Name,
						SiteId: siteId,
					}

					localTargets = append(localTargets, targetInfo)
				}
			}
			var portStr string
			if len(service.Ports) > 0 {
				for _, port := range service.Ports {
					portStr += fmt.Sprintf(" %d", port)
				}
			}

			serviceInfo := types.ServiceInfo{
				Name:     service.Address,
				Address:  service.Address + ":" + portStr,
				Protocol: service.Protocol,
				Targets:  localTargets,
			}

			localServices = append(localServices, serviceInfo)
		}
	}

	localSiteInfo := types.SiteInfo{
		Name:      siteName,
		Namespace: namespace,
		SiteId:    siteId,
		Version:   version,
		Services:  localServices,
	}

	var sites []*types.SiteInfo
	sites = append(sites, &localSiteInfo)

	return sites
}

package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/client"
	"github.com/skupperproject/skupper/pkg/utils"
	"github.com/skupperproject/skupper/pkg/utils/formatter"
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

func NewCmdNetworkStatus(newClient cobraFunc) *cobra.Command {
	cmd := &cobra.Command{
		Use:    "status",
		Short:  "Shows information about the current site, and connected sites.",
		Args:   cobra.MaximumNArgs(1),
		PreRun: newClient,
		RunE: func(cmd *cobra.Command, args []string) error {
			silenceCobra(cmd)

			var sites []*types.SiteInfo
			var errStatus error
			err := utils.RetryError(time.Second, 3, func() error {
				sites, errStatus = cli.NetworkStatus()

				if errStatus != nil {
					return errStatus
				}

				return nil
			})

			loadOnlyLocalInformation := false

			if err != nil {
				fmt.Printf("Unable to retrieve network information: %s", err)
				fmt.Println()
				fmt.Println()
				fmt.Println("Loading just local information:")
				loadOnlyLocalInformation = true
			}

			vir, err := cli.RouterInspect(context.Background())
			if err != nil || vir == nil {
				fmt.Printf("The router configuration is not available: %s", err)
				fmt.Println()
				return nil
			}

			siteConfig, err := cli.SiteConfigInspect(nil, nil)
			if err != nil || siteConfig == nil {
				fmt.Printf("The site configuration is not available: %s", err)
				fmt.Println()
				return nil
			}

			currentSite := siteConfig.Reference.UID

			if loadOnlyLocalInformation {
				printLocalStatus(vir.Status.TransportReadyReplicas, vir.Status.ConnectedSites.Warnings, vir.Status.ConnectedSites.Total, vir.Status.ConnectedSites.Direct, vir.ExposedServices)

				serviceInterfaces, err := cli.ServiceInterfaceList(context.Background())
				if err != nil {
					fmt.Printf("Service local configuration is not available: %s", err)
					fmt.Println()
					return nil
				}

				sites = getLocalSiteInfo(serviceInterfaces, currentSite, vir.Status.SiteName, cli.GetNamespace(), vir.TransportVersion)
			}

			if sites != nil && len(sites) > 0 {
				siteList := formatter.NewList()
				siteList.Item("Sites:")
				for _, site := range sites {

					if site.Name != selectedSite && selectedSite != "all" {
						continue
					}

					location := "remote"
					siteVersion := site.Version
					detailsMap := map[string]string{"name": site.Name, "namespace": site.Namespace, "URL": site.Url, "version": siteVersion}

					if len(site.MinimumVersion) > 0 {
						siteVersion = fmt.Sprintf("%s (minimum version required %s)", site.Version, site.MinimumVersion)
					}

					if site.SiteId == currentSite {
						location = "local"
						detailsMap["mode"] = vir.Status.Mode
					}

					newItem := fmt.Sprintf("[%s] %s - %s ", location, site.SiteId[:7], site.Name)

					newItem = newItem + fmt.Sprintln()

					if len(site.Links) > 0 {
						detailsMap["sites linked to"] = fmt.Sprint(strings.Join(site.Links, ", "))
					}

					serviceLevel := siteList.NewChildWithDetail(newItem, detailsMap)
					if len(site.Services) > 0 {
						services := serviceLevel.NewChild("Services:")
						var addresses []string
						svcAuth := map[string]bool{}
						for _, svc := range site.Services {
							addresses = append(addresses, svc.Name)
							svcAuth[svc.Name] = true
						}
						if vc, ok := cli.(*client.VanClient); ok && site.Namespace == cli.GetNamespace() {
							policy := client.NewPolicyValidatorAPI(vc)
							res, _ := policy.Services(addresses...)
							for addr, auth := range res {
								svcAuth[addr] = auth.Allowed
							}
						}
						for _, svc := range site.Services {
							authSuffix := ""
							if !svcAuth[svc.Name] {
								authSuffix = " - not authorized"
							}
							svcItem := "name: " + svc.Name + authSuffix + fmt.Sprintln()
							detailsSvc := map[string]string{"protocol": svc.Protocol, "address": svc.Address}
							targetLevel := services.NewChildWithDetail(svcItem, detailsSvc)

							if len(svc.Targets) > 0 {
								targets := targetLevel.NewChild("Targets:")
								for _, target := range svc.Targets {
									targets.NewChild("name: " + target.Name)

								}
							}

						}
					}
				}

				siteList.Print()
			}

			return nil
		},
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

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

	if networkStatusTimeout.Seconds() <= 0 {
		return fmt.Errorf(`invalid timeout value`)
	}

	ctx, cancel := context.WithTimeout(context.Background(), networkStatusTimeout)
	defer cancel()

	var sites []*types.SiteInfo
	var errStatus error
	err := utils.RetryErrorWithContext(ctx, time.Second, func() error {
		sites, errStatus = s.kube.Cli.NetworkStatus(ctx)

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

	vir, err := s.kube.Cli.RouterInspect(context.Background())
	if err != nil || vir == nil {
		fmt.Printf("The router configuration is not available: %s", err)
		fmt.Println()
		return nil
	}

	siteConfig, err := s.kube.Cli.SiteConfigInspect(ctx, nil)
	if err != nil || siteConfig == nil {
		fmt.Printf("The site configuration is not available: %s", err)
		fmt.Println()
		return nil
	}

	currentSite := siteConfig.Reference.UID

	if loadOnlyLocalInformation {
		printLocalStatus(vir.Status.TransportReadyReplicas, vir.Status.ConnectedSites.Warnings, vir.Status.ConnectedSites.Total, vir.Status.ConnectedSites.Direct, vir.ExposedServices)

		serviceInterfaces, err := s.kube.Cli.ServiceInterfaceList(context.Background())
		if err != nil {
			fmt.Printf("Service local configuration is not available: %s", err)
			fmt.Println()
			return nil
		}

		sites = getLocalSiteInfo(serviceInterfaces, currentSite, vir.Status.SiteName, s.kube.Cli.GetNamespace(), vir.TransportVersion)
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
				if vc, ok := s.kube.Cli.(*client.VanClient); ok && site.Namespace == s.kube.Cli.GetNamespace() {
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
}

func (s *SkupperKubeNetwork) StatusFlags(cmd *cobra.Command) {}

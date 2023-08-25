package main

import (
	"context"
	"fmt"
	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/client"
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
		network := formatter.NewList()
		network.Item("Sites:")
		for _, siteStatus := range *sitesStatus {

			if siteStatus.Site.Name != selectedSite && selectedSite != "all" {
				continue
			}

			location := "remote"

			siteVersion := siteStatus.Site.Version
			if len(siteStatus.Site.MinimumVersion) > 0 {
				siteVersion = fmt.Sprintf("%s (minimum version required %s)", siteStatus.Site.Version, siteStatus.Site.MinimumVersion)
			}

			detailsMap := map[string]string{"name": siteStatus.Site.Name, "namespace": siteStatus.Site.Namespace, "version": siteVersion}

			if siteStatus.Site.Identity == currentSite {
				location = "local"
			}

			newItem := fmt.Sprintf("[%s] %s - %s ", location, siteStatus.Site.Identity[:7], siteStatus.Site.Name)
			newItem = newItem + fmt.Sprintln()

			siteLevel := network.NewChildWithDetail(newItem, detailsMap)

			if len(siteStatus.RouterStatus) > 0 {
				routers := siteLevel.NewChild("Routers:")
				for _, routerStatus := range siteStatus.RouterStatus {
					routerItem := fmt.Sprintf("name: %s", routerStatus.Router.Name)
					detailsRouter := map[string]string{"image name": routerStatus.Router.ImageName, "image version": routerStatus.Router.ImageVersion}

					routerLevel := routers.NewChildWithDetail(routerItem, detailsRouter)
					if len(routerStatus.Listeners) > 0 {
						services := routerLevel.NewChild("Services:")
						var addresses []string
						svcAuth := map[string]bool{}
						for _, svc := range routerStatus.Listeners {
							addresses = append(addresses, svc.Name)
							svcAuth[svc.Name] = true
						}
						if vc, ok := s.kube.Cli.(*client.VanClient); ok && siteStatus.Site.Namespace == s.kube.Cli.GetNamespace() {
							policy := client.NewPolicyValidatorAPI(vc)
							res, _ := policy.Services(addresses...)
							for addr, auth := range res {
								svcAuth[addr] = auth.Allowed
							}
						}
						for _, svc := range routerStatus.Listeners {
							authSuffix := ""
							if !svcAuth[svc.Name] {
								authSuffix = " - not authorized"
							}
							svcItem := "name: " + svc.Name + authSuffix + fmt.Sprintln()
							detailsSvc := map[string]string{"protocol": svc.Protocol, "address": svc.Address}
							serviceLevel := services.NewChildWithDetail(svcItem, detailsSvc)

							if len(routerStatus.Connectors) > 0 {
								targets := serviceLevel.NewChild("Targets:")
								for _, target := range routerStatus.Connectors {
									targets.NewChild("name: " + target.Address)

								}
							}

						}
					}
				}
			}
		}
		//TODO: links are missing
		network.Print()
	}

	return nil
}

func (s *SkupperKubeNetwork) StatusFlags(cmd *cobra.Command) {}

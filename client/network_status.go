package client

import (
	"fmt"
	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/pkg/server"
)

func (cli *VanClient) NetworkStatus() ([]*types.SiteInfo, error) {

	sites, err := server.GetSiteInfo(cli.Namespace, cli.KubeClient, cli.RestConfig)
	if err != nil {
		return nil, err
	}
	services, err := server.GetServiceInfo(cli.Namespace, cli.KubeClient, cli.RestConfig)

	if err != nil {
		return nil, err
	}

	var listSites []*types.SiteInfo

	for _, site := range *sites {

		listLinksWithStatus, err := cli.getSiteLinksStatus(&site.Links, site.Namespace)
		if err != nil {
			return nil, err
		}

		listServicesAndTargets, err := cli.getServicesAndTargetsBySiteId(services, site.SiteId)
		if err != nil {
			return nil, err
		}

		newSite := types.SiteInfo{Name: site.Name, Namespace: site.Namespace, SiteId: site.SiteId, Url: site.Url, Links: listLinksWithStatus, Services: listServicesAndTargets}
		listSites = append(listSites, &newSite)

	}
	return listSites, nil
}

func (cli *VanClient) getSiteLinksStatus(siteLinks *[]string, namespace string) ([]string, error) {
	lightRed := "\033[1;31m"
	resetColor := "\033[0m"
	var listLinks []string

	mapLinkStatus, err := cli.getLinkStatusByNamespace(namespace)
	if err != nil {
		return nil, err
	}

	for _, link := range *siteLinks {
		var formattedLink string

		if mapLinkStatus[link].Connected {
			formattedLink = link
		} else {
			formattedLink = fmt.Sprintf("%s%s%s", lightRed, link, resetColor)
		}

		listLinks = append(listLinks, formattedLink)
	}

	return listLinks, nil
}

func (cli *VanClient) getServicesAndTargetsBySiteId(services *[]types.ServiceInfo, siteId string) ([]types.ServiceInfo, error) {
	var listServices []types.ServiceInfo

	for _, service := range *services {
		var listTargets []types.TargetInfo

		if len(service.Targets) > 0 {
			for _, target := range service.Targets {
				if target.SiteId == siteId {
					listTargets = append(listTargets, target)
				}
			}
		}

		serviceDetail, err := cli.ServiceInterfaceInspect(nil, service.Address)
		if err != nil {
			return nil, err
		}

		serviceHost := service.Address + ":"

		for _, port := range serviceDetail.Ports {
			serviceHost += fmt.Sprintf(" %d", port)
		}

		newService := types.ServiceInfo{Name: service.Address, Protocol: service.Protocol, Address: serviceHost, Targets: listTargets}
		listServices = append(listServices, newService)
	}

	return listServices, nil
}

package client

import (
	"fmt"
	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/pkg/server"
	"github.com/skupperproject/skupper/pkg/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"time"
)

func (cli *VanClient) NetworkStatus() ([]*types.SiteInfo, error) {

	//Checking if the router has been deployed
	_, err := cli.KubeClient.AppsV1().Deployments(cli.Namespace).Get(types.TransportDeploymentName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("skupper is not installed: %s", err)
	}

	var sites *[]types.SiteInfo
	err = utils.Retry(5*time.Second, 5, func() (bool, error) {
		sites, err = server.GetSiteInfo(cli.Namespace, cli.KubeClient, cli.RestConfig)
		if err != nil {
			return false, err
		}

		return true, nil
	})
	if err != nil {
		return nil, err
	}

	versionCheckedSites := cli.checkSiteVersion(sites)

	var services *[]types.ServiceInfo
	err = utils.Retry(5*time.Second, 5, func() (bool, error) {
		services, err = server.GetServiceInfo(cli.Namespace, cli.KubeClient, cli.RestConfig)
		if err != nil {
			return false, err
		}
		return true, nil
	})
	if err != nil {
		return nil, err
	}

	var listSites []*types.SiteInfo

	for _, site := range versionCheckedSites {

		if len(site.Namespace) == 0 {
			return nil, fmt.Errorf("unable to provide site information")
		}

		listLinks, err := cli.getSiteLinksStatus(site.Namespace)
		if err != nil {
			return nil, err
		}

		listServicesAndTargets, err := cli.getServicesAndTargetsBySiteId(services, site.SiteId)
		if err != nil {
			return nil, err
		}

		newSite := types.SiteInfo{Name: site.Name, Namespace: site.Namespace, SiteId: site.SiteId, Url: site.Url, Version: site.Version, MinimumVersion: site.MinimumVersion, Links: listLinks, Services: listServicesAndTargets}

		listSites = append(listSites, &newSite)

	}
	return listSites, nil
}

func (cli *VanClient) getSiteLinksStatus(namespace string) ([]string, error) {
	lightRed := "\033[1;31m"
	resetColor := "\033[0m"
	var listLinks []string

	if len(namespace) == 0 {
		return nil, fmt.Errorf("unspecified namespace")
	}

	mapLinkStatus, err := cli.getLinkStatusByNamespace(namespace)
	if err != nil {
		return nil, err
	}

	links := make([]string, 0, len(mapLinkStatus))
	for connection := range mapLinkStatus {
		links = append(links, connection)
	}

	for _, link := range links {
		var formattedLink string

		if mapLinkStatus[link].Connected {
			formattedLink = link
		} else {
			formattedLink = fmt.Sprintf("%s%s(link not active)%s", lightRed, link, resetColor)
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

		if serviceDetail != nil {
			for _, port := range serviceDetail.Ports {
				serviceHost += fmt.Sprintf(" %d", port)
			}
		}

		newService := types.ServiceInfo{Name: service.Address, Protocol: service.Protocol, Address: serviceHost, Targets: listTargets}
		listServices = append(listServices, newService)
	}

	return listServices, nil
}

func (cli *VanClient) checkSiteVersion(sites *[]types.SiteInfo) []types.SiteInfo {

	var listSites []types.SiteInfo

	localSiteVersion := cli.GetVersion(types.SiteVersion, types.SiteVersion)

	for _, site := range *sites {
		if utils.LessRecentThanVersion(site.Version, localSiteVersion) {
			if utils.IsValidFor(site.Version, cli.GetMinimumCompatibleVersion()) {
				site.MinimumVersion = cli.GetMinimumCompatibleVersion()
			}
		}

		listSites = append(listSites, site)
	}
	return listSites
}

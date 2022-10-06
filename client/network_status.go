package client

import (
	"context"
	"fmt"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/pkg/domain"
	"github.com/skupperproject/skupper/pkg/server"
	"github.com/skupperproject/skupper/pkg/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type GetLocalLinks func(*VanClient, string, map[string]string) (map[string]*types.LinkStatus, error)

func (cli *VanClient) NetworkStatus(ctx context.Context) ([]*types.SiteInfo, error) {

	// Checking if the router has been deployed
	_, err := cli.KubeClient.AppsV1().Deployments(cli.Namespace).Get(types.TransportDeploymentName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("Skupper is not installed: %s", err)
	}

	sites, err := server.GetSiteInfo(ctx, cli.Namespace, cli.KubeClient, cli.RestConfig)

	if err != nil {
		return nil, err
	}

	if sites == nil {
		return nil, fmt.Errorf("could not retrieve information about the sites from the service controller")
	}

	versionCheckedSites := cli.checkSiteVersion(sites)
	siteNameMap := getSiteNameMap(sites)

	services, err := server.GetServiceInfo(cli.Namespace, cli.KubeClient, cli.RestConfig)

	if err != nil {
		return nil, err
	}

	var listSites []*types.SiteInfo

	for _, site := range versionCheckedSites {

		if site.Gateway {
			// TODO: Define how gateways have to be shown
			continue
		}

		if len(site.Namespace) == 0 {
			return nil, fmt.Errorf("site %s: unable to get site namespace from service-controller", site.Name)
		}

		siteConfig, err := cli.SiteConfigInspect(nil, nil)
		if err != nil || siteConfig == nil {
			return nil, fmt.Errorf("skupper-site configuration not available")
		}

		currentSite := siteConfig.Reference.UID

		listLinks, err := GetFormattedLinks(GetLocalLinkStatus, cli, site, siteNameMap, site.SiteId == currentSite)
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

func GetFormattedLinks(getLocalLinks GetLocalLinks, cli *VanClient, site types.SiteInfo, siteNameMap map[string]string, isLocalSite bool) ([]string, error) {
	lightRed := "\033[1;31m"
	resetColor := "\033[0m"
	var listLinks []string

	if siteNameMap == nil || len(siteNameMap) == 0 {
		return nil, fmt.Errorf("the site name map used to format the links has no values or it is not initialized")
	}

	if len(site.Namespace) == 0 {
		return nil, fmt.Errorf("unspecified namespace in SiteInfo")
	}

	for _, link := range site.Links {
		if len(link) > 0 {

			trimmedLink := link
			if len(link) > 7 {
				trimmedLink = link[:7]
			}

			formattedLink := trimmedLink + "-" + siteNameMap[link]

			if isLocalSite {
				mapLinkStatus, err := getLocalLinks(cli, site.Namespace, siteNameMap)
				if err != nil {
					return nil, err
				}

				if _, ok := mapLinkStatus[formattedLink]; ok {
					if !mapLinkStatus[formattedLink].Connected {
						formattedLink = fmt.Sprintf("%s%s (link not active)%s", lightRed, formattedLink, resetColor)
					}
				}
			}
			listLinks = append(listLinks, formattedLink)
		}
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
			if utils.IsValidFor(site.Version, domain.MinimumCompatibleVersion) {
				site.MinimumVersion = domain.MinimumCompatibleVersion
			}
		}

		listSites = append(listSites, site)
	}
	return listSites
}

func getSiteNameMap(sites *[]types.SiteInfo) map[string]string {

	siteNameMap := make(map[string]string)
	for _, site := range *sites {
		siteNameMap[site.SiteId] = site.Name
	}

	return siteNameMap
}

func (cli *VanClient) GetRemoteLinks(ctx context.Context, siteConfig *types.SiteConfig) ([]*types.RemoteLinkInfo, error) {

	//Checking if the router has been deployed
	_, err := cli.KubeClient.AppsV1().Deployments(cli.Namespace).Get(types.TransportDeploymentName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("skupper is not installed: %s", err)
	}

	currentSiteId := siteConfig.Reference.UID

	sites, err := server.GetSiteInfo(ctx, cli.Namespace, cli.KubeClient, cli.RestConfig)

	if err != nil {
		return nil, err
	}

	var remoteLinks []*types.RemoteLinkInfo

	for _, site := range *sites {

		if site.SiteId == currentSiteId {
			continue
		}

		for _, link := range site.Links {
			if link == currentSiteId {
				newRemoteLink := types.RemoteLinkInfo{SiteName: site.Name, Namespace: site.Namespace, SiteId: site.SiteId}
				remoteLinks = append(remoteLinks, &newRemoteLink)
			}
		}
	}
	return remoteLinks, nil
}

package network

import (
	"github.com/skupperproject/skupper/api/types"
	"strconv"
)

func GetMapServiceLabels(services []*types.ServiceInterface) map[string]map[string]string {

	mapServiceLabels := make(map[string]map[string]string)

	for _, svc := range services {
		if svc.Labels != nil {
			for _, port := range svc.Ports {
				serviceName := svc.Address + ":" + strconv.Itoa(port)
				mapServiceLabels[serviceName] = svc.Labels
			}
		}

	}

	return mapServiceLabels
}

func GetMapServiceSites(vanStatus *types.VanStatusInfo) map[string][]types.SiteStatusInfo {

	mapServiceSites := make(map[string][]types.SiteStatusInfo)

	for _, site := range vanStatus.SiteStatus {

		for _, listener := range site.RouterStatus[0].Listeners {
			if mapServiceSites[listener.Name] != nil {
				serviceSites := mapServiceSites[listener.Name]

				serviceSites = append(serviceSites, site)
				mapServiceSites[listener.Name] = serviceSites
			} else {
				mapServiceSites[listener.Name] = []types.SiteStatusInfo{site}
			}
		}
	}

	return mapServiceSites
}

func GetMapSiteTargets(vanStatus *types.VanStatusInfo) map[string]map[string]types.ConnectorInfo {

	mapSiteTargets := make(map[string]map[string]types.ConnectorInfo)

	for _, site := range vanStatus.SiteStatus {

		for _, connector := range site.RouterStatus[0].Connectors {
			if mapSiteTargets[site.Site.Identity] == nil {
				mapSiteTargets[site.Site.Identity] = make(map[string]types.ConnectorInfo)
			}
			mapSiteTargets[site.Site.Identity][connector.Address] = connector
		}
	}

	return mapSiteTargets
}

package network

import (
	"encoding/json"
	"fmt"
	"strings"
)

type SkupperStatus struct {
	NetworkStatus *NetworkStatusInfo
}

func (s *SkupperStatus) GetServiceSitesMap() map[string][]SiteStatusInfo {

	mapServiceSites := make(map[string][]SiteStatusInfo)

	for _, site := range s.NetworkStatus.SiteStatus {

		for _, listener := range site.RouterStatus[0].Listeners {
			if mapServiceSites[listener.Name] != nil {
				serviceSites := mapServiceSites[listener.Name]

				serviceSites = append(serviceSites, site)
				mapServiceSites[listener.Name] = serviceSites
			} else {
				mapServiceSites[listener.Name] = []SiteStatusInfo{site}
			}
		}
	}

	return mapServiceSites
}

func (s *SkupperStatus) GetSiteTargetsMap() map[string]map[string]ConnectorInfo {

	mapSiteTargets := make(map[string]map[string]ConnectorInfo)

	for _, site := range s.NetworkStatus.SiteStatus {

		for _, connector := range site.RouterStatus[0].Connectors {
			if mapSiteTargets[site.Site.Identity] == nil {
				mapSiteTargets[site.Site.Identity] = make(map[string]ConnectorInfo)
			}
			mapSiteTargets[site.Site.Identity][connector.Address] = connector
		}
	}

	return mapSiteTargets
}

func (s *SkupperStatus) GetRouterSiteMap() map[string]SiteStatusInfo {
	mapRouterSite := make(map[string]SiteStatusInfo)
	for _, siteStatus := range s.NetworkStatus.SiteStatus {
		if len(siteStatus.RouterStatus) > 0 {
			for _, routerStatus := range siteStatus.RouterStatus {
				// the name of the router has a "0/" as a prefix that it is needed to remove
				routerName := strings.Split(routerStatus.Router.Name, "/")
				mapRouterSite[routerName[1]] = siteStatus
			}
		}
	}

	return mapRouterSite
}

func (s *SkupperStatus) GetSiteById(siteId string) *SiteStatusInfo {

	for _, siteStatus := range s.NetworkStatus.SiteStatus {
		if siteStatus.Site.Identity == siteId {
			return &siteStatus
		}
	}

	return nil
}

func (s *SkupperStatus) GetSiteLinkMapPerRouter(router *RouterStatusInfo, site *SiteInfo) map[string]LinkInfo {

	routerSiteMap := s.GetRouterSiteMap()
	siteLinkMap := make(map[string]LinkInfo)
	if len(router.Links) > 0 {
		for _, link := range router.Links {
			// the links between routers of the same site are not shown.
			if !s.LinkBelongsToSameSite(link.Name, site.Identity, routerSiteMap) {
				//the attribute "Name" in the link matches the name of the router from the site that is linked with.
				s := routerSiteMap[link.Name]
				siteIdentifier := fmt.Sprintf("%s(%s)", s.Site.Identity, s.Site.Namespace)
				siteLinkMap[siteIdentifier] = link
			}
		}

	}

	return siteLinkMap
}

func (s *SkupperStatus) LinkBelongsToSameSite(linkName string, siteId string, routerSiteMap map[string]SiteStatusInfo) bool {

	return strings.EqualFold(routerSiteMap[linkName].Site.Identity, siteId)

}

func (s *SkupperStatus) RemoveLinksFromSameSite(router RouterStatusInfo, site SiteInfo) []LinkInfo {
	routerSiteMap := s.GetRouterSiteMap()
	var filteredLinks []LinkInfo
	for _, link := range router.Links {
		// avoid showing the links between routers of the same site
		if !s.LinkBelongsToSameSite(link.Name, site.Identity, routerSiteMap) {
			filteredLinks = append(filteredLinks, link)
		}
	}

	return filteredLinks
}

func UnmarshalSkupperStatus(data map[string]string) (*NetworkStatusInfo, error) {

	var networkStatusInfo *NetworkStatusInfo

	err := json.Unmarshal([]byte(data["NetworkStatus"]), &networkStatusInfo)

	if err != nil {
		return nil, err
	}

	return networkStatusInfo, nil
}

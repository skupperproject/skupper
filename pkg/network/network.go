package network

import (
	"encoding/json"
	"fmt"
	"strings"
)

const MINIMUM_VERSION string = "1.5.0"
const MINIMUM_VERSION_MESSAGE string = "Detected that the skupper version installed in the namespace is version %s. The CLI requires version %s. To update the installation, please follow the instructions found here: https://skupper.io/docs/index.html"

type SkupperStatus struct {
	NetworkStatus *NetworkStatusInfo
}

func (s *SkupperStatus) GetServiceSitesMap() map[string][]SiteStatusInfo {

	mapServiceSites := make(map[string][]SiteStatusInfo)

	for _, site := range s.NetworkStatus.SiteStatus {

		if len(site.RouterStatus) > 0 {
			for _, router := range site.RouterStatus {
				for _, listener := range router.Listeners {
					if len(mapServiceSites[listener.Address]) == 0 || !sliceContainsSite(mapServiceSites[listener.Address], site) {
						mapServiceSites[listener.Address] = append(mapServiceSites[listener.Address], site)
					}
				}

				/* Checking for headless services:
				   the service can be available in the same site where the headless service was exposed, but in that site there is no
				   listeners for the statefulstet.
				*/
				for _, connector := range router.Connectors {
					if len(mapServiceSites[connector.Address]) == 0 || !sliceContainsSite(mapServiceSites[connector.Address], site) {
						mapServiceSites[connector.Address] = append(mapServiceSites[connector.Address], site)
					}
				}
			}

		}
	}

	return mapServiceSites
}

func (s *SkupperStatus) GetSiteTargetMap() map[string]map[string]ConnectorInfo {

	mapSiteTarget := make(map[string]map[string]ConnectorInfo)

	for _, site := range s.NetworkStatus.SiteStatus {

		if len(site.RouterStatus) > 0 {
			for _, router := range site.RouterStatus {
				for _, connector := range router.Connectors {
					if mapSiteTarget[site.Site.Identity] == nil {
						mapSiteTarget[site.Site.Identity] = make(map[string]ConnectorInfo)
					}
					mapSiteTarget[site.Site.Identity][connector.Address] = connector
				}
			}
		}
	}

	return mapSiteTarget
}

func (s *SkupperStatus) GetRouterSiteMap() map[string]SiteStatusInfo {
	mapRouterSite := make(map[string]SiteStatusInfo)
	for _, siteStatus := range s.NetworkStatus.SiteStatus {
		if len(siteStatus.RouterStatus) > 0 {
			for _, routerStatus := range siteStatus.RouterStatus {
				// the name of the router has a "0/" as a prefix that it is needed to remove
				routerName := strings.Split(routerStatus.Router.Name, "/")

				// Remove routers that belong to statefulsets for headless services
				if len(routerName) > 1 && strings.HasPrefix(routerName[1], siteStatus.Site.Name) {
					mapRouterSite[routerName[1]] = siteStatus
				}
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

func (s *SkupperStatus) GetRouterIndex(site *SiteStatusInfo) (error, int) {

	for index, router := range site.RouterStatus {
		// Ignore routers that belong to statefulsets for headless services and any other router
		routerId := strings.Split(router.Router.Name, "/")

		if len(routerId) > 1 && strings.HasPrefix(routerId[1], site.Site.Name) {
			return nil, index

		}
	}

	return fmt.Errorf("not valid router found"), -1
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

func sliceContainsSite(sites []SiteStatusInfo, site SiteStatusInfo) bool {
	for _, s := range sites {
		if site.Site.Identity == s.Site.Identity {
			return true
		}
	}

	return false
}

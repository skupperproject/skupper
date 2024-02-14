package network

import (
	"encoding/json"
	"fmt"
	"github.com/skupperproject/skupper/pkg/utils/formatter"
	"strings"
)

const MINIMUM_VERSION string = "1.5.0"
const MINIMUM_PODMAN_VERSION string = "1.6.0"
const MINIMUM_VERSION_MESSAGE string = "Version incompatibility:\n \tDetected that the skupper version installed in the namespace is version %s. The CLI requires at least version %s. To update the installation, please follow the instructions found here: https://skupper.io/install/index.html"

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
		if PrintableRouter(router, site.Site.Name) {
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

func PrintableRouter(router RouterStatusInfo, siteName string) bool {
	// Ignore routers that belong to statefulsets for headless services and any other router
	routerId := strings.Split(router.Router.Name, "/")

	isARegularSite := len(routerId) > 1 && strings.HasPrefix(routerId[1], siteName) && router.Router.ImageName == "skupper-router"
	isAGateway := len(routerId) > 1 && strings.HasPrefix(routerId[1], "skupper-gateway")

	return isARegularSite || isAGateway
}

func sliceContainsSite(sites []SiteStatusInfo, site SiteStatusInfo) bool {
	for _, s := range sites {
		if site.Site.Identity == s.Site.Identity {
			return true
		}
	}

	return false
}

func PrintServiceStatus(currentNetworkStatus *NetworkStatusInfo, mapServiceLabels map[string]map[string]string, verboseServiceStatus bool, showLabels bool) error {
	statusManager := SkupperStatus{
		NetworkStatus: currentNetworkStatus,
	}

	mapServiceSites := statusManager.GetServiceSitesMap()
	mapSiteTarget := statusManager.GetSiteTargetMap()

	if len(currentNetworkStatus.Addresses) == 0 {
		fmt.Println("No services defined")
	} else {
		l := formatter.NewList()
		l.Item("Services exposed through Skupper:")

		for _, si := range currentNetworkStatus.Addresses {
			svc := l.NewChild(fmt.Sprintf("%s (%s)", si.Name, si.Protocol))

			if verboseServiceStatus {
				sites := svc.NewChild("Sites:")

				if mapServiceSites[si.Name] != nil {
					for _, site := range mapServiceSites[si.Name] {
						item := site.Site.Identity + "(" + site.Site.Namespace + ")\n"
						policy := "-"
						if len(site.Site.Policy) > 0 {
							policy = site.Site.Policy
						}
						theSite := sites.NewChildWithDetail(item, map[string]string{"policy": policy})

						if si.ConnectorCount > 0 {
							t := mapSiteTarget[site.Site.Identity][si.Name]

							if len(t.Address) > 0 {
								targets := theSite.NewChild("Targets:")
								var name string
								if t.Target != "" {
									name = fmt.Sprintf("name=%s", t.Target)
								}
								targetInfo := fmt.Sprintf("%s %s", t.Address, name)
								targets.NewChild(targetInfo)
							}
						}
					}
				}
			}

			if showLabels && len(mapServiceLabels[si.Name]) > 0 {
				labels := svc.NewChild("Labels:")
				for k, v := range mapServiceLabels[si.Name] {
					labels.NewChild(fmt.Sprintf("%s=%s", k, v))
				}
			}
		}
		l.Print()
	}

	return nil
}

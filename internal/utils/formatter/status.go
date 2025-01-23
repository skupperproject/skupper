package formatter

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/skupperproject/skupper/internal/network"
)

type PlatformSupport struct {
	SupportType string
	SupportName string
}

type StatusData struct {
	EnabledIn           PlatformSupport
	Mode                string
	SiteName            string
	Policies            string
	Status              *string
	Warnings            []string
	TotalConnections    int
	DirectConnections   int
	IndirectConnections int
	ExposedServices     int
	ConsoleUrl          string
	Credentials         PlatformSupport
}

func PrintStatus(data StatusData) error {

	enabledIn := fmt.Sprintf("%q", data.EnabledIn.SupportName)
	if data.EnabledIn.SupportType == "kubernetes" {
		enabledIn = fmt.Sprintf("namespace %q", data.EnabledIn.SupportName)
	}

	siteName := ""
	if data.SiteName != "" && data.SiteName != data.EnabledIn.SupportName {
		siteName = siteName + fmt.Sprintf(" with site name %q", data.SiteName)
	}
	policyStr := ""

	if data.Policies == "enabled" {
		policyStr = " (with policies)"
	}

	fmt.Printf("Skupper is enabled for %s%s%s.", enabledIn, siteName, policyStr)
	if data.Status != nil {
		fmt.Printf(" Status pending...")
	} else {
		if len(data.Warnings) > 0 {
			for _, w := range data.Warnings {
				fmt.Printf("Warning: %s", w)
				fmt.Println()
			}
		}
		if data.TotalConnections == 0 {
			fmt.Printf(" It is not connected to any other sites.")
		} else if data.TotalConnections == 1 {
			fmt.Printf(" It is connected to 1 other site.")
		} else if data.TotalConnections == data.DirectConnections {
			fmt.Printf(" It is connected to %d other sites.", data.TotalConnections)
		} else {
			fmt.Printf(" It is connected to %d other sites (%d indirectly).", data.TotalConnections, data.IndirectConnections)
		}
	}
	if data.ExposedServices == 0 {
		fmt.Printf(" It has no exposed services.")
	} else if data.ExposedServices == 1 {
		fmt.Printf(" It has 1 exposed service.")
	} else {
		fmt.Printf(" It has %d exposed services.", data.ExposedServices)
	}
	fmt.Println()

	if len(data.ConsoleUrl) > 0 {
		fmt.Println("The site console url is: ", data.ConsoleUrl)
		if len(data.Credentials.SupportName) > 0 {
			fmt.Printf("The credentials for internal console-auth mode are held in %s: %s", data.Credentials.SupportType, data.Credentials.SupportName)
		}
	}

	fmt.Println()
	return nil
}

func PrintVerboseStatus(data StatusData) error {
	writer := tabwriter.NewWriter(os.Stdout, 1, 1, 1, ' ', 0)
	fmt.Fprintf(writer, "%s:\t %s \n", data.EnabledIn.SupportType, data.EnabledIn.SupportName)
	routerMode := "interior"
	if len(data.Mode) > 0 {
		routerMode = data.Mode
	}
	fmt.Fprintf(writer, "%s:\t %s \n", "mode", routerMode)
	fmt.Fprintf(writer, "%s:\t %s \n", "site name", data.SiteName)
	fmt.Fprintf(writer, "%s:\t %s \n", "policies", data.Policies)

	if data.Status != nil {
		fmt.Fprintf(writer, "%s:\t %s \n", "status", *data.Status)
	}

	for index, w := range data.Warnings {
		warningIndex := fmt.Sprintf("warning %d", index)
		fmt.Fprintf(writer, "%s:\t %s \n", warningIndex, w)
	}

	fmt.Fprintf(writer, "%s:\t %s \n", "total connections", strconv.Itoa(data.TotalConnections))
	fmt.Fprintf(writer, "%s:\t %s \n", "direct connections", strconv.Itoa(data.DirectConnections))
	fmt.Fprintf(writer, "%s:\t %s \n", "indirect connections", strconv.Itoa(data.IndirectConnections))

	fmt.Fprintf(writer, "%s:\t %s \n", "exposed services", strconv.Itoa(data.ExposedServices))

	if len(data.ConsoleUrl) > 0 {
		fmt.Fprintf(writer, "%s:\t %s \n", "site console url", data.ConsoleUrl)
	}

	if len(data.Credentials.SupportName) > 0 {
		fmt.Fprintf(writer, "%s:\t %s \n", "credentials", data.Credentials.SupportName)
	}

	err := writer.Flush()
	if err != nil {
		return err
	}

	return nil
}

func PrintNetworkStatus(currentSite string, currentNetworkStatus *network.NetworkStatusInfo, siteNetworkStatus string, verbose bool) error {
	sitesStatus := currentNetworkStatus.SiteStatus
	statusManager := network.SkupperStatus{NetworkStatus: currentNetworkStatus}

	if sitesStatus != nil && len(sitesStatus) > 0 {

		networkList := NewList()
		networkList.Item("Sites:")

		for _, siteStatus := range sitesStatus {
			if len(siteNetworkStatus) == 0 || siteNetworkStatus == siteStatus.Site.Name {

				siteVersion := "-"
				if len(siteStatus.Site.Version) > 0 {
					siteVersion = siteStatus.Site.Version
				}

				if len(siteStatus.Site.MinimumVersion) > 0 {
					siteVersion = fmt.Sprintf("%s (minimum version required %s)", siteStatus.Site.Version, siteStatus.Site.MinimumVersion)
				}

				detailsMap := map[string]string{"site name": siteStatus.Site.Name, "namespace": siteStatus.Site.Namespace, "version": siteVersion}

				location := "[remote]"
				if siteStatus.Site.Identity == currentSite {
					location = "[local]"
				} else if strings.HasPrefix(siteStatus.Site.Identity, "gateway") {
					location = ""
				}

				newItem := fmt.Sprintf("%s %s(%s) ", location, siteStatus.Site.Identity, siteStatus.Site.Namespace)
				newItem = newItem + fmt.Sprintln()

				siteLevel := networkList.NewChildWithDetail(newItem, detailsMap)

				if len(siteStatus.RouterStatus) > 0 {

					err, index := statusManager.GetRouterIndex(&siteStatus)
					if err != nil {
						return err
					}

					mapSiteLink := statusManager.GetSiteLinkMapPerRouter(&siteStatus.RouterStatus[index], &siteStatus.Site)

					if len(mapSiteLink) > 0 {
						siteLinks := siteLevel.NewChild("Linked sites:")
						for key := range mapSiteLink {
							siteLinks.NewChild(fmt.Sprintln(key))
						}
					}

					if verbose {
						routers := siteLevel.NewChild("Routers:")
						for _, routerStatus := range siteStatus.RouterStatus {
							routerId := strings.Split(routerStatus.Router.Name, "/")

							// skip routers that belong to headless services
							if network.DisplayableRouter(routerStatus, &siteStatus) {
								routerItem := fmt.Sprintf("name: %s\n", routerId[1])
								detailsRouter := map[string]string{"image name": routerStatus.Router.ImageName, "image version": routerStatus.Router.ImageVersion}

								routerLevel := routers.NewChildWithDetail(routerItem, detailsRouter)

								printableLinks := statusManager.RemoveLinksFromSameSite(routerStatus, siteStatus.Site)

								if len(printableLinks) > 0 {
									links := routerLevel.NewChild("Links:")
									for _, link := range printableLinks {
										linkItem := fmt.Sprintf("name:  %s\n", link.Name)
										detailsLink := map[string]string{}
										if link.LinkCost > 0 {
											detailsLink["cost"] = strconv.FormatUint(link.LinkCost, 10)
										}
										links.NewChildWithDetail(linkItem, detailsLink)
									}
								}
							}
						}
					}
				}
			}
		}

		networkList.Print()
	}
	return nil
}

func PrintServiceStatus(currentNetworkStatus *network.NetworkStatusInfo, mapServiceLabels map[string]map[string]string, verboseServiceStatus bool, showLabels bool, localSiteInfo *network.LocalSiteInfo) error {
	statusManager := network.SkupperStatus{
		NetworkStatus: currentNetworkStatus,
	}

	mapServiceSites := statusManager.GetServiceSitesMap()
	mapSiteTarget := statusManager.GetSiteTargetMap()

	if len(currentNetworkStatus.Addresses) == 0 {
		fmt.Println("No services defined")
	} else {
		l := NewList()
		l.Item("Services exposed through Skupper:")

		for _, si := range currentNetworkStatus.Addresses {
			svc := l.NewChild(fmt.Sprintf("%s (%s)", si.Name, si.Protocol))

			if verboseServiceStatus {
				sites := svc.NewChild("Sites:")

				if mapServiceSites[si.Name] != nil {
					for _, site := range mapServiceSites[si.Name] {
						siteNamespace := ""

						if len(site.Site.Namespace) > 0 {
							siteNamespace = "(" + site.Site.Namespace + ")"
						}
						item := site.Site.Identity + siteNamespace + "\n"
						policy := "-"
						if len(site.Site.Policy) > 0 {
							policy = site.Site.Policy
						}
						theSite := sites.NewChildWithDetail(item, map[string]string{"policy": policy})

						if si.ConnectorCount > 0 {
							serviceTargets := mapSiteTarget[site.Site.Identity][si.Name]

							if len(serviceTargets) > 0 {
								/* if the function has been provided with information about targets, it will be
								   printed, instead the information provided by the controller-lite*/
								if localSiteInfo != nil && localSiteInfo.SiteId == site.Site.Identity {
									for serviceAddress, serviceInfo := range localSiteInfo.ServiceInfo {
										if serviceAddress == si.Name {
											for section, data := range serviceInfo.Data {
												entrySection := theSite.NewChild(section)
												for _, sectionValue := range data {
													entrySection.NewChild(sectionValue)
												}
											}
										}
									}
								} else {
									targets := theSite.NewChild("Targets:")

									for _, t := range serviceTargets {
										if len(t.Address) > 0 {
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

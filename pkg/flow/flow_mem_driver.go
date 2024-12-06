package flow

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/pkg/config"
	"github.com/skupperproject/skupper/pkg/domain/podman"
	"github.com/skupperproject/skupper/pkg/kube"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/retry"
)

func (fc *FlowCollector) inferGatewaySite(siteId string) error {
	if _, ok := fc.Sites[siteId]; !ok {
		parts := strings.Split(siteId, "_")
		if len(parts) > 1 {
			if parts[0] == "gateway" {
				if _, ok := fc.Sites[siteId]; !ok {
					name := parts[1]
					namespace := parts[0] + "-" + parts[1]
					site := SiteRecord{
						Base: Base{
							RecType:   recordNames[Site],
							Identity:  siteId,
							StartTime: uint64(time.Now().UnixNano()) / uint64(time.Microsecond),
						},
						Name:      &name,
						NameSpace: &namespace,
					}
					log.Printf("FLOW_LOG: %s\n", prettyPrint(site))
					fc.Sites[siteId] = &site
				}
			}
		}
	}
	return nil
}

func (fc *FlowCollector) inferGatewayProcess(siteId string, host string) error {
	if site, ok := fc.Sites[siteId]; ok {
		groupName := *site.NameSpace + "-" + host
		groupIdentity := ""
		for _, pg := range fc.ProcessGroups {
			if *pg.Name == groupName {
				groupIdentity = pg.Identity
				break
			}
		}
		if groupIdentity == "" {
			groupIdentity = uuid.New().String()
			fc.ProcessGroups[groupIdentity] = &ProcessGroupRecord{
				Base: Base{
					RecType:   recordNames[ProcessGroup],
					Identity:  groupIdentity,
					StartTime: uint64(time.Now().UnixNano()) / uint64(time.Microsecond),
				},
				Name:             &groupName,
				ProcessGroupRole: &External,
			}
		}
		processName := *site.Name + "-" + host
		procFound := false
		for _, proc := range fc.Processes {
			if *proc.Name == processName {
				procFound = true
				break
			}
		}
		if !procFound {
			log.Printf("COLLECTOR: Inferring gateway process %s \n", host)
			procIdentity := uuid.New().String()
			fc.Processes[procIdentity] = &ProcessRecord{
				Base: Base{
					RecType:   recordNames[Process],
					Identity:  procIdentity,
					Parent:    siteId,
					StartTime: uint64(time.Now().UnixNano()) / uint64(time.Microsecond),
				},
				Name:          &processName,
				GroupName:     &groupName,
				GroupIdentity: &groupIdentity,
				HostName:      site.Name,
				SourceHost:    &host,
				ProcessRole:   &External,
			}
		}
	}
	return nil
}

func (c *FlowCollector) isGatewaySite(siteId string) bool {
	if site, ok := c.Sites[siteId]; ok {
		if site.NameSpace != nil {
			parts := strings.Split(*site.NameSpace, "-")
			if len(parts) > 1 {
				if parts[0] == "gateway" {
					return true
				}
			}
		}
	}
	return false
}

func (c *FlowCollector) getRecordSiteId(record interface{}) string {
	if record == nil {
		return ""
	}
	switch record.(type) {
	case SiteRecord:
		if site, ok := record.(SiteRecord); ok {
			return site.Identity
		}
	case RouterRecord:
		if router, ok := record.(RouterRecord); ok {
			return router.Parent
		}
	case LinkRecord:
		if link, ok := record.(LinkRecord); ok {
			if router, ok := c.Routers[link.Parent]; ok {
				return router.Parent
			}
		}
	case ListenerRecord:
		if listener, ok := record.(ListenerRecord); ok {
			if router, ok := c.Routers[listener.Parent]; ok {
				return router.Parent
			}
		}
	case ConnectorRecord:
		if connector, ok := record.(ConnectorRecord); ok {
			if router, ok := c.Routers[connector.Parent]; ok {
				return router.Parent
			}
		}
	case FlowRecord:
		if flow, ok := record.(FlowRecord); ok {
			if connector, ok := c.Connectors[flow.Parent]; ok {
				if router, ok := c.Routers[connector.Parent]; ok {
					return router.Parent
				}
			}
			if listener, ok := c.Listeners[flow.Parent]; ok {
				if router, ok := c.Routers[listener.Parent]; ok {
					return router.Parent
				}
			}
			if l4Flow, ok := c.Flows[flow.Parent]; ok {
				if connector, ok := c.Connectors[l4Flow.Parent]; ok {
					if router, ok := c.Routers[connector.Parent]; ok {
						return router.Parent
					}
				}
				if listener, ok := c.Listeners[l4Flow.Parent]; ok {
					if router, ok := c.Routers[listener.Parent]; ok {
						return router.Parent
					}
				}
			}
		}
	case ProcessRecord:
		if process, ok := record.(ProcessRecord); ok {
			return process.Parent
		}
	case HostRecord:
		if host, ok := record.(HostRecord); ok {
			return host.Parent
		}
	default:
		return ""
	}
	return ""
}

func (fc *FlowCollector) getRoutersForSite(site *SiteRecord) []*RouterRecord {
	var routers []*RouterRecord
	for _, router := range fc.Routers {
		if router.Parent == site.Identity {
			routers = append(routers, router)
		}
	}
	return routers
}

func (fc *FlowCollector) getLinksForRouter(router *RouterRecord) []*LinkRecord {
	var links []*LinkRecord
	for _, link := range fc.Links {
		if link.Parent == router.Identity {
			links = append(links, link)
		}
	}
	return links
}

func (fc *FlowCollector) getListenersForRouter(router *RouterRecord) []*ListenerRecord {
	var listeners []*ListenerRecord
	for _, listener := range fc.Listeners {
		if listener.Parent == router.Identity {
			listeners = append(listeners, listener)
		}
	}
	return listeners
}

func (fc *FlowCollector) getConnectorsForRouter(router *RouterRecord) []*ConnectorRecord {
	var connectors []*ConnectorRecord
	for _, connector := range fc.Connectors {
		if connector.Parent == router.Identity {
			connectors = append(connectors, connector)
		}
	}
	return connectors
}

func (fc *FlowCollector) getRouterForFlow(flow *FlowRecord) *RouterRecord {
	if connector, ok := fc.Connectors[flow.Parent]; ok {
		if router, ok := fc.Routers[connector.Parent]; ok {
			return router
		}
	}
	if listener, ok := fc.Listeners[flow.Parent]; ok {
		if router, ok := fc.Routers[listener.Parent]; ok {
			return router
		}
	}
	if l4Flow, ok := fc.Flows[flow.Parent]; ok {
		if connector, ok := fc.Connectors[l4Flow.Parent]; ok {
			if router, ok := fc.Routers[connector.Parent]; ok {
				return router
			}
		}
		if listener, ok := fc.Listeners[l4Flow.Parent]; ok {
			if router, ok := fc.Routers[listener.Parent]; ok {
				return router
			}
		}
	}
	return nil
}

func (fc *FlowCollector) getFlowProtocol(flow *FlowRecord) *string {
	if listener, ok := fc.Listeners[flow.Parent]; ok {
		return listener.Protocol
	} else if connector, ok := fc.Connectors[flow.Parent]; ok {
		return connector.Protocol
	} else if l4Flow, ok := fc.Flows[flow.Parent]; ok {
		if listener, ok := fc.Listeners[l4Flow.Parent]; ok {
			return listener.Protocol
		} else if connector, ok := fc.Connectors[l4Flow.Parent]; ok {
			return connector.Protocol
		}
	}
	return nil
}

func (fc *FlowCollector) getFlowLabels(flow *FlowRecord) map[string]string {
	labels := make(map[string]string)
	if flow.ProcessName != nil {
		labels["process"] = *flow.ProcessName
	}
	if listener, ok := fc.Listeners[flow.Parent]; ok {
		labels["direction"] = Incoming
		if listener.AddressId != nil {
			labels["addressId"] = *listener.AddressId
		}
		if listener.Protocol != nil {
			labels["protocol"] = *listener.Protocol
		}
		return labels
	} else if connector, ok := fc.Connectors[flow.Parent]; ok {
		labels["direction"] = Outgoing
		if connector.AddressId != nil {
			labels["addressId"] = *connector.AddressId
		}
		if connector.Protocol != nil {
			labels["protocol"] = *connector.Protocol
		}
		return labels
	} else if l4Flow, ok := fc.Flows[flow.Parent]; ok {
		if listener, ok := fc.Listeners[l4Flow.Parent]; ok {
			labels["direction"] = Incoming
			if listener.AddressId != nil {
				labels["addressId"] = *listener.AddressId
			}
			if listener.Protocol != nil {
				labels["protocol"] = *listener.Protocol
			}
		} else if connector, ok := fc.Connectors[l4Flow.Parent]; ok {
			labels["direction"] = Outgoing
			if connector.AddressId != nil {
				labels["addressId"] = *connector.AddressId
			}
			if connector.Protocol != nil {
				labels["protocol"] = *connector.Protocol
			}
			return labels
		}
	}
	return labels
}

func (fc *FlowCollector) getFlowPlace(flow *FlowRecord) FlowPlace {
	if _, ok := fc.Listeners[flow.Parent]; ok {
		return clientSide
	} else if _, ok := fc.Connectors[flow.Parent]; ok {
		return serverSide
	} else if _, ok := fc.recentConnectors[flow.Parent]; ok {
		return serverSide
	} else if l4Flow, ok := fc.Flows[flow.Parent]; ok {
		if _, ok := fc.Listeners[l4Flow.Parent]; ok {
			return clientSide
		} else if _, ok := fc.Connectors[l4Flow.Parent]; ok {
			return serverSide
		} else if _, ok := fc.recentConnectors[l4Flow.Parent]; ok {
			return serverSide
		}
	}
	return unknown
}

func (fc *FlowCollector) annotateFlowTrace(flow *FlowRecord) *string {
	if flow == nil || flow.Trace == nil {
		return nil
	}
	flowTrace := []string{}
	parts := strings.Split(*flow.Trace, "|")
	for _, part := range parts {
		for _, router := range fc.Routers {
			if *router.Name == part {
				if site, ok := fc.Sites[router.Parent]; ok {
					flowTrace = append(flowTrace, strings.TrimPrefix(*router.Name, "0/")+"@"+*site.Name)
				}
			}
		}
	}
	if connector, ok := fc.Connectors[flow.Parent]; ok {
		if router, ok := fc.Routers[connector.Parent]; ok {
			if site, ok := fc.Sites[router.Parent]; ok {
				flowTrace = append(flowTrace, strings.TrimPrefix(*router.Name, "0/")+"@"+*site.Name)
			}
		}
	}
	if listener, ok := fc.Listeners[flow.Parent]; ok {
		if router, ok := fc.Routers[listener.Parent]; ok {
			if site, ok := fc.Sites[router.Parent]; ok {
				flowTrace = append(flowTrace, strings.TrimPrefix(*router.Name, "0/")+"@"+*site.Name)
			}
		}
	}
	if l4Flow, ok := fc.Flows[flow.Parent]; ok {
		if connector, ok := fc.Connectors[l4Flow.Parent]; ok {
			if router, ok := fc.Routers[connector.Parent]; ok {
				if site, ok := fc.Sites[router.Parent]; ok {
					flowTrace = append(flowTrace, strings.TrimPrefix(*router.Name, "0/")+"@"+*site.Name)
				}
			}
		}
		if listener, ok := fc.Listeners[l4Flow.Parent]; ok {
			if router, ok := fc.Routers[listener.Parent]; ok {
				if site, ok := fc.Sites[router.Parent]; ok {
					flowTrace = append(flowTrace, strings.TrimPrefix(*router.Name, "0/")+"@"+*site.Name)
				}
			}
		}
	}
	annotatedTrace := strings.Join(flowTrace, "|")
	return &annotatedTrace
}

func (fc *FlowCollector) linkFlowPair(flow *FlowRecord) (*FlowPairRecord, bool) {
	var sourceSiteId, destSiteId, sourceSiteName, destSiteName string = "", "", "sourceSite", "destSite"
	var sourceFlow, destFlow *FlowRecord = nil, nil
	var ok bool

	if flow.CounterFlow == nil {
		// can't create a pair without a counter flow
		return nil, false
	}
	flow.Place = fc.getFlowPlace(flow)
	if flow.Place == clientSide {
		sourceFlow = flow
		if destFlow, ok = fc.Flows[*flow.CounterFlow]; !ok {
			return nil, ok
		}
	} else if flow.Place == serverSide {
		destFlow = flow
		if sourceFlow, ok = fc.Flows[*flow.CounterFlow]; !ok {
			return nil, ok
		}
	} else {
		return nil, false
	}

	fwdLabels := fc.getFlowLabels(sourceFlow)
	revLabels := fc.getFlowLabels(destFlow)
	if len(fwdLabels) != 4 || len(revLabels) != 4 {
		return nil, false
	}

	sourceSiteId = fc.getRecordSiteId(*sourceFlow)
	if sourceSite, ok := fc.Sites[sourceSiteId]; ok {
		sourceSiteName = *sourceSite.Name
	}
	destSiteId = fc.getRecordSiteId(*destFlow)
	if destSite, ok := fc.Sites[destSiteId]; ok {
		destSiteName = *destSite.Name
	}
	fwdLabels["sourceSite"] = sourceSiteName + "@_@" + sourceSiteId
	fwdLabels["destSite"] = destSiteName + "@_@" + destSiteId
	fwdLabels["sourceProcess"] = *sourceFlow.ProcessName
	fwdLabels["destProcess"] = *destFlow.ProcessName
	fwdLabels["sourceHost"] = *sourceFlow.SourceHost
	fwdLabels["destHost"] = *destFlow.SourceHost
	delete(fwdLabels, "process")
	revLabels["sourceSite"] = destSiteName + "@_@" + destSiteId
	revLabels["destSite"] = sourceSiteName + "@_@" + sourceSiteId
	revLabels["sourceProcess"] = *destFlow.ProcessName
	revLabels["destProcess"] = *sourceFlow.ProcessName
	revLabels["sourceHost"] = *destFlow.SourceHost
	fwdLabels["destHost"] = *sourceFlow.SourceHost
	delete(revLabels, "process")

	fp := &FlowPairRecord{
		Base: Base{
			RecType:   recordNames[FlowPair],
			Identity:  "fp-" + sourceFlow.Identity,
			StartTime: sourceFlow.StartTime,
			EndTime:   sourceFlow.EndTime,
		},
		SourceSiteId:      sourceSiteId,
		DestinationSiteId: destSiteId,
		ForwardFlow:       sourceFlow,
		CounterFlow:       destFlow,
	}
	if sourceFlow.EndTime != 0 {
		fp.Duration = sourceFlow.EndTime - sourceFlow.StartTime
	}
	if sourceSite, ok := fc.Sites[sourceSiteId]; ok {
		fp.SourceSiteName = sourceSite.Name
	} else {
		return fp, ok
	}
	if destSite, ok := fc.Sites[destSiteId]; ok {
		fp.DestinationSiteName = destSite.Name
	} else {
		return fp, ok
	}
	fp.FlowTrace = fc.annotateFlowTrace(destFlow)

	// setup flow metrics inc flow count, set octets, assign metric to flow, etc.
	addressId := fwdLabels["addressId"]
	if va, ok := fc.VanAddresses[addressId]; ok {
		fwdLabels["address"] = va.Name
		delete(fwdLabels, "addressId")
		revLabels["address"] = va.Name
		delete(revLabels, "addressId")
		err := fc.setupFlowMetrics(va, sourceFlow, fwdLabels)
		if err != nil {
			log.Println("COLLECTOR: metric setup error", err.Error())
		}
		err = fc.setupFlowMetrics(va, destFlow, revLabels)
		if err != nil {
			log.Println("COLLECTOR: metric setup error", err.Error())
		}
	}

	return fp, ok
}

func prettyPrint(i interface{}) string {
	s, _ := json.Marshal(i)
	return string(s)
}

var defaultRetry = wait.Backoff{
	Steps:    100,
	Duration: 10 * time.Millisecond,
	Factor:   1.0,
	Jitter:   0.1,
}

var netUpdateCt int

func (fc *FlowCollector) updateNetworkStatus() {
	var err error
	networkData := map[string]string{}
	platform := config.GetPlatform()
	var sites []*SiteStatus
	var addresses []*VanAddressRecord

	for _, address := range fc.VanAddresses {
		fc.getAddressAdaptorCounts(address)
		addresses = append(addresses, address)
	}
	for _, site := range fc.Sites {
		var routerStatus []RouterStatus
		routers := fc.getRoutersForSite(site)
		for _, router := range routers {
			links := fc.getLinksForRouter(router)
			listeners := fc.getListenersForRouter(router)
			connectors := fc.getConnectorsForRouter(router)
			routerStatus = append(routerStatus, RouterStatus{
				Router:     router,
				Links:      links,
				Listeners:  listeners,
				Connectors: connectors,
			})
		}
		siteStatus := &SiteStatus{
			Site:         site,
			RouterStatus: routerStatus,
		}
		sites = append(sites, siteStatus)
	}
	sort.SliceStable(addresses, func(i, j int) bool {
		return addresses[i].Name < addresses[j].Name
	})
	networkStatus := NetworkStatus{
		Addresses: addresses,
		Sites:     sites,
	}
	networkData["NetworkStatus"] = prettyPrint(networkStatus)

	if platform == "" || platform == types.PlatformKubernetes {
		if fc.kubeclient == nil { // errant configuration - means there is a bug in FlowCollector or how it was configured
			panic("FlowCollector was not configured with a kubernetes client")
		}
		err = func() error {
			err = retry.RetryOnConflict(defaultRetry, func() error {
				configMap, err := kube.GetConfigMap(types.NetworkStatusConfigMapName, fc.namespace, fc.kubeclient)
				if err != nil {
					return err
				}

				configMap.Data = networkData

				_, err = fc.kubeclient.CoreV1().ConfigMaps(fc.namespace).Update(context.TODO(), configMap, metav1.UpdateOptions{})
				if err != nil {
					return err
				} else {
					netUpdateCt++
					return nil
				}
			})
			if !fc.networkStatusUp && len(networkStatus.Sites) > 0 && len(networkStatus.Sites[0].RouterStatus) > 0 {
				fc.networkStatusUp = true
				log.Printf("COLLECTOR: First functional network status update written after %s and %d updates\n", time.Since(fc.begin), netUpdateCt)
			}
			return nil
		}()
	} else if platform == types.PlatformPodman {
		networkStatusHandler := &podman.NetworkStatusHandler{}
		err = networkStatusHandler.Update(networkData["NetworkStatus"])
	}

	if err != nil {
		log.Printf("COLLECTOR: Error writing network status update: %v", err)
	}
}

func (fc *FlowCollector) updateEntityCounts() {
	if fc.mode != RecordMetrics {
		return
	}
	fc.metrics.activeSites.Set(float64(len(fc.Sites)))
	fc.metrics.activeRouters.Set(float64(len(fc.Routers)))
	_, siteNodes := fc.graph()
	for _, node := range siteNodes {
		fc.metrics.activeLinks.WithLabelValues(node.ID, "outgoing").Set(float64(len(node.Forward)))
		fc.metrics.activeLinks.WithLabelValues(node.ID, "incoming").Set(float64(len(node.Backward)))
	}
}

func (fc *FlowCollector) addRecord(record interface{}) error {
	if record == nil {
		return fmt.Errorf("No record to add")
	}
	log.Printf("FLOW_LOG: %s\n", prettyPrint(record))

	switch record.(type) {
	case *SiteRecord:
		if site, ok := record.(*SiteRecord); ok {
			fc.Sites[site.Identity] = site
		}
	case *HostRecord:
		if host, ok := record.(*HostRecord); ok {
			fc.Hosts[host.Identity] = host
		}
	case *RouterRecord:
		if router, ok := record.(*RouterRecord); ok {
			fc.Routers[router.Identity] = router
		}
	case *LinkRecord:
		if link, ok := record.(*LinkRecord); ok {
			fc.Links[link.Identity] = link
		}
	case *ListenerRecord:
		if listener, ok := record.(*ListenerRecord); ok {
			fc.Listeners[listener.Identity] = listener
		}
	case *ConnectorRecord:
		if connector, ok := record.(*ConnectorRecord); ok {
			fc.Connectors[connector.Identity] = connector
		}
	case *FlowRecord:
		if flow, ok := record.(*FlowRecord); ok {
			fc.Flows[flow.Identity] = flow
		}
	case *FlowPairRecord:
		if flowPair, ok := record.(*FlowPairRecord); ok {
			fc.FlowPairs[flowPair.Identity] = flowPair
		}
	case *ProcessRecord:
		if process, ok := record.(*ProcessRecord); ok {
			fc.Processes[process.Identity] = process
		}
	case *ProcessGroupRecord:
		if processGroup, ok := record.(*ProcessGroupRecord); ok {
			fc.ProcessGroups[processGroup.Identity] = processGroup
		}
	case *FlowAggregateRecord:
		if aggregate, ok := record.(*FlowAggregateRecord); ok {
			fc.FlowAggregates[aggregate.Identity] = aggregate
		}
	case *VanAddressRecord:
		if va, ok := record.(*VanAddressRecord); ok {
			fc.VanAddresses[va.Identity] = va
		}
	default:
		return fmt.Errorf("Unknown record type to add")
	}
	if fc.mode == RecordStatus {
		fc.updateNetworkStatus()
	}
	return nil
}

func (fc *FlowCollector) deleteRecord(record interface{}) error {
	if record == nil {
		return fmt.Errorf("No record to delete")
	}
	log.Printf("FLOW_LOG: %s\n", prettyPrint(record))
	switch record.(type) {
	case *SiteRecord:
		if site, ok := record.(*SiteRecord); ok {
			delete(fc.Sites, site.Identity)
		}
	case *HostRecord:
		if host, ok := record.(*HostRecord); ok {
			delete(fc.Hosts, host.Identity)
		}
	case *RouterRecord:
		if router, ok := record.(*RouterRecord); ok {
			delete(fc.Routers, router.Identity)
		}
	case *LinkRecord:
		if link, ok := record.(*LinkRecord); ok {
			delete(fc.Links, link.Identity)
		}
	case *ListenerRecord:
		if listener, ok := record.(*ListenerRecord); ok {
			delete(fc.Listeners, listener.Identity)
		}
	case *ConnectorRecord:
		if connector, ok := record.(*ConnectorRecord); ok {
			// keep around for flows that will be terminated
			fc.recentConnectors[connector.Identity] = connector
			delete(fc.Connectors, connector.Identity)
		}
	case *FlowRecord:
		if flow, ok := record.(*FlowRecord); ok {
			delete(fc.Flows, flow.Identity)
		}
	case *FlowPairRecord:
		if flowPair, ok := record.(*FlowPairRecord); ok {
			delete(fc.FlowPairs, flowPair.Identity)
		}
	case *ProcessRecord:
		if process, ok := record.(*ProcessRecord); ok {
			delete(fc.Processes, process.Identity)
		}
	case *ProcessGroupRecord:
		if processGroup, ok := record.(*ProcessGroupRecord); ok {
			delete(fc.ProcessGroups, processGroup.Identity)
		}
	case *FlowAggregateRecord:
		if aggregate, ok := record.(*FlowAggregateRecord); ok {
			delete(fc.FlowAggregates, aggregate.Identity)
		}
	case *VanAddressRecord:
		if va, ok := record.(*VanAddressRecord); ok {
			delete(fc.VanAddresses, va.Identity)
		}
	default:
		return fmt.Errorf("Unknown record type to delete")
	}
	if fc.mode == RecordStatus {
		fc.updateNetworkStatus()
	}
	return nil
}

func (fc *FlowCollector) updateLastHeard(source string) error {
	if eventsource, ok := fc.eventSources[source]; ok {
		eventsource.LastHeard = uint64(time.Now().UnixNano()) / uint64(time.Microsecond)
		eventsource.Messages++
	}
	return nil
}

func (fc *FlowCollector) updateRecord(record interface{}) error {
	var updatesNetworkStatus bool
	switch record.(type) {
	case HeartbeatRecord:
		if heartbeat, ok := record.(HeartbeatRecord); ok {
			if eventsource, ok := fc.eventSources[heartbeat.Identity]; ok {
				eventsource.LastHeard = uint64(time.Now().UnixNano()) / uint64(time.Microsecond)
				eventsource.Heartbeats++
				eventsource.Messages++
			}
			if pending, ok := fc.pendingFlush[heartbeat.Source]; ok {
				pending.heartbeat = true
			}
		}
	case SiteRecord:
		if site, ok := record.(SiteRecord); ok {
			if current, ok := fc.Sites[site.Identity]; !ok {
				if site.StartTime > 0 && site.EndTime == 0 {
					fc.addRecord(&site)
				}
			} else {
				if site.EndTime > 0 {
					current.EndTime = site.EndTime
					for _, aggregate := range fc.FlowAggregates {
						if aggregate.PairType == recordNames[Site] {
							if current.Identity == *aggregate.SourceId || current.Identity == *aggregate.DestinationId {
								aggregate.EndTime = current.EndTime
								fc.deleteRecord(aggregate)
							}
						}
					}
					fc.deleteRecord(current)
				} else {
					updatesNetworkStatus = true
					if site.Policy != nil {
						current.Policy = site.Policy
					}
				}
			}
			fc.updateLastHeard(site.Source)
			fc.updateEntityCounts()
		}
	case HostRecord:
		if host, ok := record.(HostRecord); ok {
			if current, ok := fc.Hosts[host.Identity]; !ok {
				if host.StartTime > 0 && host.EndTime == 0 {
					fc.addRecord(&host)
				}
			} else {
				if host.EndTime > 0 {
					current.EndTime = host.EndTime
					fc.deleteRecord(current)
				} else {
					*current = host
				}
			}
			fc.updateLastHeard(host.Source)
		}
	case RouterRecord:
		if router, ok := record.(RouterRecord); ok {
			if current, ok := fc.Routers[router.Identity]; !ok {
				if router.StartTime > 0 && router.EndTime == 0 {
					fc.addRecord(&router)
					if router.Parent != "" {
						fc.inferGatewaySite(router.Parent)
					}
				}
			} else {
				if router.EndTime > 0 {
					current.EndTime = router.EndTime
					fc.deleteRecord(current)
				} else if router.Parent != "" && current.Parent == "" {
					current.Parent = router.Parent
					if _, ok := fc.Sites[current.Parent]; !ok {
						fc.inferGatewaySite(current.Parent)
					}
				}
			}
			fc.updateLastHeard(router.Source)
			fc.updateEntityCounts()
		}
	case LogEventRecord:
		if logEvent, ok := record.(LogEventRecord); ok {
			log.Printf("LOG_EVENT: %s \n", prettyPrint(logEvent))
		}
	case LinkRecord:
		if link, ok := record.(LinkRecord); ok {
			if current, ok := fc.Links[link.Identity]; !ok {
				if link.StartTime > 0 && link.EndTime == 0 {
					fc.addRecord(&link)
				}
			} else {
				if link.EndTime > 0 {
					// find and delete the corresponding link for the peer router
					if currentParentRouter, ok := fc.Routers[current.Parent]; ok {
						if current.Direction != nil && current.Name != nil && currentParentRouter.Name != nil {
							var peerRouterId string
							peerDirection := Outgoing
							if *current.Direction == Outgoing {
								peerDirection = Incoming
							}
							for _, router := range fc.Routers {
								if router.Name != nil && *current.Name == normalizeRouterName(*router.Name) {
									peerRouterId = router.Identity
									break
								}
							}
							for _, eLink := range fc.Links {
								if eLink.Direction != nil && *eLink.Direction == peerDirection && eLink.Parent == peerRouterId && eLink.Name != nil && *eLink.Name == normalizeRouterName(*currentParentRouter.Name) {
									eLink.EndTime = link.EndTime
									fc.deleteRecord(eLink)
									break
								}
							}
						}
					}
					current.EndTime = link.EndTime
					fc.deleteRecord(current)
				} else {
					if current.LinkCost == nil && link.LinkCost != nil {
						current.LinkCost = link.LinkCost
						if fc.mode == RecordStatus {
							fc.updateNetworkStatus()
						}
					}
				}
			}
			fc.updateLastHeard(link.Source)
			fc.updateEntityCounts()
		}
	case ListenerRecord:
		if listener, ok := record.(ListenerRecord); ok {
			if current, ok := fc.Listeners[listener.Identity]; !ok {
				if listener.StartTime > 0 && listener.EndTime == 0 {
					if listener.Address != nil {
						var va *VanAddressRecord
						for _, y := range fc.VanAddresses {
							if y.Name == *listener.Address {
								va = y
								break
							}
						}
						if va == nil {
							va = &VanAddressRecord{
								Base: Base{
									RecType:   recordNames[Address],
									Identity:  uuid.New().String(),
									StartTime: listener.StartTime,
								},
								Name:     *listener.Address,
								Protocol: *listener.Protocol,
							}
							va.flowCount = make(map[metricKey]prometheus.Counter)
							va.activeFlowCount = make(map[metricKey]prometheus.Gauge)
							va.octetCount = make(map[metricKey]prometheus.Counter)
							va.lastAccessed = make(map[metricKey]prometheus.Gauge)
							va.flowLatency = make(map[metricKey]prometheus.Observer)
							fc.addRecord(va)
						}
						listener.AddressId = &va.Identity
					}
					fc.addRecord(&listener)
				}
			} else {
				if listener.EndTime > 0 {
					current.EndTime = listener.EndTime
					if current.AddressId != nil {
						count := 0
						for id, l := range fc.Listeners {
							if id != current.Identity && l.AddressId != nil && *l.AddressId == *current.AddressId {
								count++
							}
						}
						if count == 0 {
							if va, ok := fc.VanAddresses[*current.AddressId]; ok {
								va.EndTime = listener.EndTime
								fc.deleteRecord(va)
							}
						}
					}
					fc.deleteRecord(current)
				} else {
					if current.Parent == "" && listener.Parent != "" {
						current.Parent = listener.Parent
					}
					if listener.FlowCountL4 != nil {
						current.FlowCountL4 = listener.FlowCountL4
					}
					if listener.FlowRateL4 != nil {
						current.FlowRateL4 = listener.FlowRateL4
					}
					if listener.FlowCountL7 != nil {
						current.FlowCountL7 = listener.FlowCountL7
					}
					if listener.FlowRateL7 != nil {
						current.FlowRateL7 = listener.FlowRateL7
					}
					if current.Address == nil && listener.Address != nil {
						current.Address = listener.Address
						var va *VanAddressRecord
						for _, y := range fc.VanAddresses {
							if y.Name == *listener.Address {
								va = y
								break
							}
						}
						if va == nil {
							t := time.Now()
							va = &VanAddressRecord{
								Base: Base{
									RecType:   recordNames[Address],
									Identity:  uuid.New().String(),
									StartTime: uint64(t.UnixNano()) / uint64(time.Microsecond),
								},
								Name:     *listener.Address,
								Protocol: *listener.Protocol,
							}
							va.flowCount = make(map[metricKey]prometheus.Counter)
							va.activeFlowCount = make(map[metricKey]prometheus.Gauge)
							va.octetCount = make(map[metricKey]prometheus.Counter)
							va.lastAccessed = make(map[metricKey]prometheus.Gauge)
							va.flowLatency = make(map[metricKey]prometheus.Observer)
							fc.addRecord(va)
						}
						current.AddressId = &va.Identity
					}
				}
			}
			fc.updateLastHeard(listener.Source)
		}
	case ConnectorRecord:
		if connector, ok := record.(ConnectorRecord); ok {
			if current, ok := fc.Connectors[connector.Identity]; !ok {
				if connector.StartTime > 0 && connector.EndTime == 0 {
					if connector.Parent != "" {
						if connector.Address != nil {
							var va *VanAddressRecord
							for _, y := range fc.VanAddresses {
								if y.Name == *connector.Address {
									va = y
									break
								}
							}
							if va == nil {
								t := time.Now()
								va = &VanAddressRecord{
									Base: Base{
										RecType:   recordNames[Address],
										Identity:  uuid.New().String(),
										StartTime: uint64(t.UnixNano()) / uint64(time.Microsecond),
									},
									Name:     *connector.Address,
									Protocol: *connector.Protocol,
								}
								va.flowCount = make(map[metricKey]prometheus.Counter)
								va.activeFlowCount = make(map[metricKey]prometheus.Gauge)
								va.octetCount = make(map[metricKey]prometheus.Counter)
								va.lastAccessed = make(map[metricKey]prometheus.Gauge)
								va.flowLatency = make(map[metricKey]prometheus.Observer)
								fc.VanAddresses[va.Identity] = va
								fc.addRecord(va)
							}
							connector.AddressId = &va.Identity
						}
						siteId := fc.getRecordSiteId(connector)
						if fc.isGatewaySite(siteId) && connector.DestHost != nil {
							fc.inferGatewayProcess(siteId, *connector.DestHost)
						}
					}
					fc.addRecord(&connector)
					fc.connectorsToReconcile[connector.Identity] = connector.Identity
				}
			} else {
				if connector.EndTime > 0 {
					current.EndTime = connector.EndTime
					if current.ProcessId != nil {
						if process, ok := fc.Processes[*current.ProcessId]; ok {
							if process.Name == nil || !strings.HasPrefix(*process.Name, "site-servers-") {
								// site servers can be reused by multiple connectors - do not unbind
								process.connector = nil
								process.ProcessBinding = &Unbound
							}
						}
					}
					// Note a new connector can create an address but does not delete it
					// removal of last listener will delete the address
					fc.deleteRecord(current)
				} else {
					if current.Parent == "" && connector.Parent != "" {
						current.Parent = connector.Parent
						siteId := fc.getRecordSiteId(current)
						if fc.isGatewaySite(siteId) && current.DestHost != nil {
							fc.inferGatewayProcess(siteId, *current.DestHost)
						}

					}
					if current.DestHost == nil && connector.DestHost != nil {
						current.DestHost = connector.DestHost
					}
					if connector.FlowCountL4 != nil {
						current.FlowCountL4 = connector.FlowCountL4
					}
					if connector.FlowRateL4 != nil {
						current.FlowRateL4 = connector.FlowRateL4
					}
					if connector.FlowCountL7 != nil {
						current.FlowCountL7 = connector.FlowCountL7
					}
					if connector.FlowRateL7 != nil {
						current.FlowRateL7 = connector.FlowRateL7
					}
					if current.Address == nil && connector.Address != nil {
						current.Address = connector.Address
						var va *VanAddressRecord
						for _, y := range fc.VanAddresses {
							if y.Name == *connector.Address {
								va = y
								break
							}
						}
						if va == nil {
							t := time.Now()
							va = &VanAddressRecord{
								Base: Base{
									RecType:   recordNames[Address],
									Identity:  uuid.New().String(),
									StartTime: uint64(t.UnixNano()) / uint64(time.Microsecond),
								},
								Name:     *connector.Address,
								Protocol: *connector.Protocol,
							}
							va.flowCount = make(map[metricKey]prometheus.Counter)
							va.activeFlowCount = make(map[metricKey]prometheus.Gauge)
							va.octetCount = make(map[metricKey]prometheus.Counter)
							va.lastAccessed = make(map[metricKey]prometheus.Gauge)
							va.flowLatency = make(map[metricKey]prometheus.Observer)
							fc.addRecord(va)
						}
						current.AddressId = &va.Identity
					}
				}
			}
			fc.updateLastHeard(connector.Source)
		}
	case FlowRecord:
		if flow, ok := record.(FlowRecord); ok {
			if current, ok := fc.Flows[flow.Identity]; !ok {
				if flow.StartTime != 0 && flow.EndTime != 0 {
					if flow.Parent == "" {
						log.Printf("COLLECTOR: Incomplete flow record for identity %s details %+v\n", flow.Identity, flow)
					}
				}
				if flow.StartTime != 0 {
					if flow.Parent != "" {
						flow.Protocol = fc.getFlowProtocol(&flow)
						flow.Place = fc.getFlowPlace(&flow)
						if listener, ok := fc.Listeners[flow.Parent]; ok {
							siteId := fc.getRecordSiteId(*listener)
							// TODO: workaround for gateway
							if fc.isGatewaySite(siteId) {
								fc.inferGatewayProcess(siteId, *flow.SourceHost)
							}
						}
					}
					fc.addRecord(&flow)
					if flow.CounterFlow != nil {
						fc.flowsToPairReconcile[flow.Identity] = &FlowToPairRecord{
							forwardId: *flow.CounterFlow,
							created:   uint64(time.Now().UnixNano()) / uint64(time.Microsecond)}
					}
					fc.flowsToProcessReconcile[flow.Identity] = flow.Identity
				}
			} else {
				if current.SourceHost == nil && flow.SourceHost != nil {
					current.SourceHost = flow.SourceHost
				}
				if current.SourcePort == nil && flow.SourcePort != nil {
					current.SourcePort = flow.SourcePort
				}
				if current.Parent == "" && flow.Parent != "" {
					current.Parent = flow.Parent
					current.Protocol = fc.getFlowProtocol(current)
					current.Place = fc.getFlowPlace(current)
					if listener, ok := fc.Listeners[flow.Parent]; ok {
						// TODO: workaround for gateway
						siteId := fc.getRecordSiteId(*listener)
						if fc.isGatewaySite(siteId) {
							fc.inferGatewayProcess(siteId, *flow.SourceHost)
						}
					}
				}
				if flow.Octets != nil {
					current.Octets = flow.Octets
					if current.octetMetric != nil {
						current.octetMetric.Add(float64(*current.Octets - current.lastOctets))
					}
					current.lastOctets = *current.Octets
				}
				if flow.OctetsOut != nil {
					current.OctetsOut = flow.OctetsOut
				}
				if flow.OctetsUnacked != nil {
					current.OctetsUnacked = flow.OctetsUnacked
				}
				if flow.WindowClosures != nil {
					current.WindowClosures = flow.WindowClosures
				}
				if flow.WindowSize != nil {
					current.WindowSize = flow.WindowSize
				}
				if flow.Latency != nil {
					current.Latency = flow.Latency
				}
				if flow.Trace != nil {
					current.Trace = flow.Trace
				}
				if flow.Reason != nil {
					current.Reason = flow.Reason
				}
				if flow.Method != nil {
					current.Method = flow.Method
				}
				if flow.Result != nil {
					current.Result = flow.Result
				}
				if flow.StreamIdentity != nil {
					current.StreamIdentity = flow.StreamIdentity
				}
				if flow.CounterFlow != nil && current.CounterFlow == nil {
					current.CounterFlow = flow.CounterFlow
				}
				if flow.EndTime > 0 && current.EndTime == 0 {
					current.EndTime = flow.EndTime
					if current.activeFlowMetric != nil {
						current.activeFlowMetric.Dec()
					}
					if fc.getFlowPlace(current) == clientSide {
						if flowpair, ok := fc.FlowPairs["fp-"+current.Identity]; ok {
							flowpair.EndTime = current.EndTime
							flowpair.Duration = flowpair.EndTime - flowpair.StartTime
						}
					}
				}
			}
		}
	case ProcessRecord:
		if process, ok := record.(ProcessRecord); ok {
			if current, ok := fc.Processes[process.Identity]; !ok {
				if process.StartTime > 0 && process.EndTime == 0 {
					if site, ok := fc.Sites[process.Parent]; ok {
						if site.Name != nil {
							process.ParentName = site.Name
						}
					}
					process.ProcessBinding = &Unbound
					for _, pg := range fc.ProcessGroups {
						if pg.EndTime == 0 && *process.GroupName == *pg.Name {
							process.GroupIdentity = &pg.Identity
							break
						}
					}
					if process.GroupIdentity == nil && process.GroupName != nil {
						pg := &ProcessGroupRecord{
							Base: Base{
								RecType:   recordNames[ProcessGroup],
								Identity:  uuid.New().String(),
								StartTime: uint64(time.Now().UnixNano()) / uint64(time.Microsecond),
							},
							Name:             process.GroupName,
							ProcessGroupRole: process.ProcessRole,
						}
						fc.updateRecord(*pg)
						process.GroupIdentity = &pg.Identity
					}
					fc.addRecord(&process)
				}
			} else {
				if process.EndTime > 0 {
					current.EndTime = process.EndTime
					// check if there are any process pairs active
					for _, aggregate := range fc.FlowAggregates {
						if aggregate.PairType == recordNames[Process] {
							if current.Identity == *aggregate.SourceId || current.Identity == *aggregate.DestinationId {
								aggregate.EndTime = current.EndTime
								fc.deleteRecord(aggregate)
							}
						}
					}
					if current.GroupIdentity != nil {
						count := 0
						for id, p := range fc.Processes {
							if id != current.Identity && *p.GroupIdentity == *current.GroupIdentity {
								count++
							}
						}
						if count == 0 {
							if processGroup, ok := fc.ProcessGroups[*current.GroupIdentity]; ok {
								processGroup.EndTime = process.EndTime
								fc.updateRecord(*processGroup)
							}
						}
					}
					fc.deleteRecord(current)
				}
			}
			fc.updateLastHeard(process.Source)
		}
	case ProcessGroupRecord:
		if processGroup, ok := record.(ProcessGroupRecord); ok {
			if current, ok := fc.ProcessGroups[processGroup.Identity]; !ok {
				if processGroup.StartTime > 0 && processGroup.EndTime == 0 {
					fc.addRecord(&processGroup)
				}
			} else {
				if processGroup.EndTime > 0 {
					current.EndTime = processGroup.EndTime
					// check if there are an processgroup pairs active
					for _, aggregate := range fc.FlowAggregates {
						if aggregate.PairType == recordNames[ProcessGroup] {
							if current.Identity == *aggregate.SourceId || current.Identity == *aggregate.DestinationId {
								aggregate.EndTime = current.EndTime
								fc.deleteRecord(aggregate)
							}
						}
					}
					fc.deleteRecord(current)
				}
			}
			fc.updateLastHeard(processGroup.Source)
		}
	default:
		return fmt.Errorf("Unrecognized record type %T", record)
	}

	if updatesNetworkStatus && fc.mode == RecordStatus {
		fc.updateNetworkStatus()
	}
	return nil
}

type linkResponseHandler struct {
	siteByRouterID   map[string]string
	siteByRouterName map[string]string
}

func newLinkResponseHandler(sites map[string]*SiteRecord, routers map[string]*RouterRecord) linkResponseHandler {
	builder := linkResponseHandler{
		siteByRouterID:   make(map[string]string, len(routers)),
		siteByRouterName: make(map[string]string, len(routers)),
	}
	for _, router := range routers {
		_, ok := sites[router.Parent]
		if !ok || router.Name == nil {
			continue
		}

		builder.siteByRouterName[normalizeRouterName(*router.Name)] = router.Parent
		builder.siteByRouterID[router.Identity] = router.Parent
	}
	return builder
}

func normalizeRouterName(name string) string {
	// router names are prefixed with a routing area - so far always `0/`
	if delim := strings.IndexRune(name, '/'); delim > -1 {
		return name[delim+1:]
	}
	return name
}

func (b linkResponseHandler) handle(l LinkRecord) (linkRecordResponse, bool) {
	var (
		ok   bool
		resp = linkRecordResponse{LinkRecord: l}
	)
	if l.Name == nil || l.Direction == nil {
		return resp, false
	}

	if resp.SourceSiteId, ok = b.siteByRouterID[l.Parent]; !ok {
		return resp, false
	}

	if resp.DestinationSiteId, ok = b.siteByRouterName[*l.Name]; !ok {
		return resp, false
	}
	if *l.Direction == Incoming {
		resp.SourceSiteId, resp.DestinationSiteId = resp.DestinationSiteId, resp.SourceSiteId
	}

	return resp, true
}

func (fc *FlowCollector) retrieve(request ApiRequest) (*string, error) {
	vars := mux.Vars(request.Request)
	url := request.Request.URL
	queryParams := getQueryParams(url)
	var retrieveError error = nil

	p := Payload{
		Results:        nil,
		Status:         "",
		Count:          0,
		TimeRangeCount: 0,
		TotalCount:     0,
		timestamp:      uint64(time.Now().UnixNano()) / uint64(time.Microsecond),
		elapsed:        0,
	}

	switch request.RecordType {
	case Site:
		switch request.HandlerName {
		case "list":
			sites := []SiteRecord{}
			for _, site := range fc.Sites {
				if filterRecord(*site, queryParams) && site.Base.TimeRangeValid(queryParams) {
					sites = append(sites, *site)
				}
			}
			p.TotalCount = len(fc.Sites)
			retrieveError = sortAndSlice(sites, &p, queryParams)
		case "item":
			if id, ok := vars["id"]; ok {
				if site, ok := fc.Sites[id]; ok {
					p.Count = 1
					p.Results = site
				}
			}
		case "processes":
			processes := []ProcessRecord{}
			if id, ok := vars["id"]; ok {
				if site, ok := fc.Sites[id]; ok {
					for _, process := range fc.Processes {
						if process.Parent == site.Identity {
							p.TotalCount++
							if filterRecord(*process, queryParams) && process.Base.TimeRangeValid(queryParams) {
								processes = append(processes, *process)
							}
						}
					}
				}
			}
			retrieveError = sortAndSlice(processes, &p, queryParams)
		case "routers":
			routers := []RouterRecord{}
			if id, ok := vars["id"]; ok {
				if site, ok := fc.Sites[id]; ok {
					for _, router := range fc.Routers {
						if router.Parent == site.Identity {
							p.TotalCount++
							if filterRecord(*router, queryParams) && router.Base.TimeRangeValid(queryParams) {
								routers = append(routers, *router)
							}
						}
					}
				}
			}
			retrieveError = sortAndSlice(routers, &p, queryParams)
		case "links":
			links := []linkRecordResponse{}
			linkHandler := newLinkResponseHandler(fc.Sites, fc.Routers)
			if id, ok := vars["id"]; ok {
				if site, ok := fc.Sites[id]; ok {
					for _, link := range fc.Links {
						if fc.getRecordSiteId(*link) == site.Identity {
							lr, ok := linkHandler.handle(*link)
							if !ok {
								continue
							}
							p.TotalCount++
							if filterRecord(lr, queryParams) && link.Base.TimeRangeValid(queryParams) {
								links = append(links, lr)
							}
						}
					}
				}
			}
			retrieveError = sortAndSlice(links, &p, queryParams)
		case "hosts":
			hosts := []HostRecord{}
			if id, ok := vars["id"]; ok {
				if site, ok := fc.Sites[id]; ok {
					for _, host := range fc.Hosts {
						if host.Parent == site.Identity {
							p.TotalCount++
							if filterRecord(*host, queryParams) && host.Base.TimeRangeValid(queryParams) {
								hosts = append(hosts, *host)
							}
						}
					}
				}
			}
			retrieveError = sortAndSlice(hosts, &p, queryParams)
		}
	case Host:
		switch request.HandlerName {
		case "list":
			hosts := []HostRecord{}
			for _, host := range fc.Hosts {
				if filterRecord(*host, queryParams) && host.Base.TimeRangeValid(queryParams) {
					hosts = append(hosts, *host)
				}
			}
			p.TotalCount = len(fc.Hosts)
			retrieveError = sortAndSlice(hosts, &p, queryParams)
		case "item":
			if id, ok := vars["id"]; ok {
				if host, ok := fc.Hosts[id]; ok {
					p.Count = 1
					p.Results = host
				}
			}
		}
	case Router:
		switch request.HandlerName {
		case "list":
			routers := []RouterRecord{}
			for _, router := range fc.Routers {
				if filterRecord(*router, queryParams) && router.Base.TimeRangeValid(queryParams) {
					routers = append(routers, *router)
				}
			}
			p.TotalCount = len(fc.Routers)
			retrieveError = sortAndSlice(routers, &p, queryParams)
		case "item":
			if id, ok := vars["id"]; ok {
				if router, ok := fc.Routers[id]; ok {
					p.Count = 1
					p.Results = router
				}
			}
		case "flows":
			flows := []FlowRecord{}
			if id, ok := vars["id"]; ok {
				if _, ok := fc.Routers[id]; ok {
					for connId, connector := range fc.Connectors {
						if connector.Parent == id {
							for _, flow := range fc.Flows {
								if flow.Parent == connId {
									p.TotalCount++
									if filterRecord(*flow, queryParams) && flow.Base.TimeRangeValid(queryParams) {
										flows = append(flows, *flow)
									}
								} else if l4Flow, ok := fc.Flows[flow.Parent]; ok {
									if l4Flow.Parent == connId {
										p.TotalCount++
										if filterRecord(*flow, queryParams) && flow.Base.TimeRangeValid(queryParams) {
											flows = append(flows, *flow)
										}
									}
								}
							}
						}
					}
					for listenerId, listener := range fc.Listeners {
						if listener.Parent == id {
							for _, flow := range fc.Flows {
								if flow.Parent == listenerId {
									p.TotalCount++
									if filterRecord(*flow, queryParams) && flow.Base.TimeRangeValid(queryParams) {
										flows = append(flows, *flow)
									}
								} else if l4Flow, ok := fc.Flows[flow.Parent]; ok {
									if l4Flow.Parent == listenerId {
										p.TotalCount++
										if filterRecord(*flow, queryParams) && flow.Base.TimeRangeValid(queryParams) {
											flows = append(flows, *flow)
										}
									}
								}
							}
						}
					}
				}
			}
			retrieveError = sortAndSlice(flows, &p, queryParams)
		case "links":
			links := []linkRecordResponse{}
			linkHandler := newLinkResponseHandler(fc.Sites, fc.Routers)
			if id, ok := vars["id"]; ok {
				if router, ok := fc.Routers[id]; ok {
					for _, link := range fc.Links {
						if link.Parent == router.Identity {
							lr, ok := linkHandler.handle(*link)
							if !ok {
								continue
							}
							p.TotalCount++
							if filterRecord(lr, queryParams) && link.Base.TimeRangeValid(queryParams) {
								links = append(links, lr)
							}
						}
					}
				}
			}
			retrieveError = sortAndSlice(links, &p, queryParams)
		case "listeners":
			listeners := []ListenerRecord{}
			if id, ok := vars["id"]; ok {
				if router, ok := fc.Routers[id]; ok {
					for _, listener := range fc.Listeners {
						if listener.Parent == router.Identity {
							p.TotalCount++
							if filterRecord(*listener, queryParams) && listener.Base.TimeRangeValid(queryParams) {
								listeners = append(listeners, *listener)
							}
						}
					}
				}
			}
			retrieveError = sortAndSlice(listeners, &p, queryParams)
		case "connectors":
			connectors := []ConnectorRecord{}
			if id, ok := vars["id"]; ok {
				if router, ok := fc.Routers[id]; ok {
					for _, connector := range fc.Connectors {
						if connector.Parent == router.Identity {
							p.TotalCount++
							if filterRecord(*connector, queryParams) && connector.Base.TimeRangeValid(queryParams) {
								connectors = append(connectors, *connector)
							}
						}
					}
				}
			}
			retrieveError = sortAndSlice(connectors, &p, queryParams)
		}
	case Link:
		linkHandler := newLinkResponseHandler(fc.Sites, fc.Routers)
		switch request.HandlerName {
		case "list":
			links := []linkRecordResponse{}
			for _, link := range fc.Links {
				lr, ok := linkHandler.handle(*link)
				if !ok {
					continue
				}
				if filterRecord(lr, queryParams) && link.Base.TimeRangeValid(queryParams) {
					links = append(links, lr)
				}
			}
			p.TotalCount = len(fc.Links)
			retrieveError = sortAndSlice(links, &p, queryParams)
		case "item":
			if id, ok := vars["id"]; ok {
				if link, ok := fc.Links[id]; ok {
					lr, ok := linkHandler.handle(*link)
					if ok {
						p.Count = 1
						p.Results = lr
					}
				}
			}
		}
	case Listener:
		switch request.HandlerName {
		case "list":
			listeners := []ListenerRecord{}
			for _, listener := range fc.Listeners {
				if filterRecord(*listener, queryParams) && listener.Base.TimeRangeValid(queryParams) {
					listeners = append(listeners, *listener)
				}
			}
			p.TotalCount = len(fc.Listeners)
			retrieveError = sortAndSlice(listeners, &p, queryParams)
		case "item":
			if id, ok := vars["id"]; ok {
				if listener, ok := fc.Listeners[id]; ok {
					p.Count = 1
					p.Results = listener
				}
			}
		case "flows":
			flows := []FlowRecord{}
			if id, ok := vars["id"]; ok {
				if _, ok := fc.Listeners[id]; ok {
					for _, flow := range fc.Flows {
						if flow.Parent == id {
							p.TotalCount++
							if filterRecord(*flow, queryParams) && flow.Base.TimeRangeValid(queryParams) {
								flows = append(flows, *flow)
							}
						} else if l4Flow, ok := fc.Flows[flow.Parent]; ok {
							if l4Flow.Parent == id {
								p.TotalCount++
								if filterRecord(*flow, queryParams) && flow.Base.TimeRangeValid(queryParams) {
									flows = append(flows, *flow)
								}
							}
						}
					}
				}
			}
			retrieveError = sortAndSlice(flows, &p, queryParams)
		}
	case Connector:
		switch request.HandlerName {
		case "list":
			connectors := []ConnectorRecord{}
			for _, connector := range fc.Connectors {
				if filterRecord(*connector, queryParams) && connector.Base.TimeRangeValid(queryParams) {
					connectors = append(connectors, *connector)
				}
			}
			p.TotalCount = len(fc.Connectors)
			retrieveError = sortAndSlice(connectors, &p, queryParams)
		case "item":
			if id, ok := vars["id"]; ok {
				if connector, ok := fc.Connectors[id]; ok {
					p.Count = 1
					p.Results = connector
				}
			}
		case "flows":
			flows := []FlowRecord{}
			if id, ok := vars["id"]; ok {
				if _, ok := fc.Connectors[id]; ok {
					for _, flow := range fc.Flows {
						if flow.Parent == id {
							p.TotalCount++
							if filterRecord(*flow, queryParams) && flow.Base.TimeRangeValid(queryParams) {
								flows = append(flows, *flow)
							}
						} else if l4Flow, ok := fc.Flows[flow.Parent]; ok {
							if l4Flow.Parent == id {
								p.TotalCount++
								if filterRecord(*flow, queryParams) && flow.Base.TimeRangeValid(queryParams) {
									flows = append(flows, *flow)
								}
							}
						}
					}
				}
			}
			retrieveError = sortAndSlice(flows, &p, queryParams)
		case "process":
			if id, ok := vars["id"]; ok {
				if connector, ok := fc.Connectors[id]; ok {
					if connector.ProcessId != nil {
						if process, ok := fc.Processes[*connector.ProcessId]; ok {
							p.Count = 1
							p.Results = *process
						}
					}
				}
			}
		}
	case Address:
		switch request.HandlerName {
		case "list":
			addresses := []VanAddressRecord{}
			for _, address := range fc.VanAddresses {
				if filterRecord(*address, queryParams) {
					fc.getAddressAdaptorCounts(address)
					addresses = append(addresses, *address)
				}
			}
			p.TotalCount = len(fc.VanAddresses)
			retrieveError = sortAndSlice(addresses, &p, queryParams)
		case "item":
			if id, ok := vars["id"]; ok {
				if address, ok := fc.VanAddresses[id]; ok {
					fc.getAddressAdaptorCounts(address)
					p.Count = 1
					p.Results = address
				}
			}
		case "flows":
			flows := []FlowRecord{}
			if id, ok := vars["id"]; ok {
				if vanaddr, ok := fc.VanAddresses[id]; ok {
					for connId, connector := range fc.Connectors {
						if *connector.Address == vanaddr.Name {
							for _, flow := range fc.Flows {
								if flow.Parent == connId && *connector.Protocol == "tcp" {
									p.TotalCount++
									if filterRecord(*flow, queryParams) && flow.Base.TimeRangeValid(queryParams) {
										flows = append(flows, *flow)
									}
								} else if l4Flow, ok := fc.Flows[flow.Parent]; ok {
									if l4Flow.Parent == connId {
										p.TotalCount++
										if filterRecord(*flow, queryParams) && flow.Base.TimeRangeValid(queryParams) {
											flows = append(flows, *flow)
										}
									}
								}
							}
						}
					}
					for listenerId, listener := range fc.Listeners {
						if *listener.Address == vanaddr.Name {
							for _, flow := range fc.Flows {
								if flow.Parent == listenerId && *listener.Protocol == "tcp" {
									p.TotalCount++
									if filterRecord(*flow, queryParams) && flow.Base.TimeRangeValid(queryParams) {
										flows = append(flows, *flow)
									}
								} else if l4Flow, ok := fc.Flows[flow.Parent]; ok {
									if l4Flow.Parent == listenerId {
										p.TotalCount++
										if filterRecord(*flow, queryParams) && flow.Base.TimeRangeValid(queryParams) {
											flows = append(flows, *flow)
										}
									}
								}
							}
						}
					}
				}
			}
			retrieveError = sortAndSlice(flows, &p, queryParams)
		case "flowpairs":
			flowPairs := []FlowPairRecord{}
			if id, ok := vars["id"]; ok {
				if vanaddr, ok := fc.VanAddresses[id]; ok {
					// forward flow for a flow pair is indexed by listener flow id
					for listenerId, listener := range fc.Listeners {
						if *listener.Address == vanaddr.Name {
							for flowId, flow := range fc.Flows {
								if flow.Parent == listenerId && flow.CounterFlow != nil {
									if flowpair, ok := fc.FlowPairs["fp-"+flowId]; ok {
										p.TotalCount++
										if filterRecord(*flowpair, queryParams) && flowpair.Base.TimeRangeValid(queryParams) {
											flowPairs = append(flowPairs, *flowpair)
										}
									}
								} else if l4Flow, ok := fc.Flows[flow.Parent]; ok {
									if l4Flow.Parent == listenerId {
										if flowpair, ok := fc.FlowPairs["fp-"+flowId]; ok {
											p.TotalCount++
											if filterRecord(*flowpair, queryParams) && flowpair.Base.TimeRangeValid(queryParams) {
												flowPairs = append(flowPairs, *flowpair)
											}
										}
									}
								}
							}
						}
					}
				}
			}
			retrieveError = sortAndSlice(flowPairs, &p, queryParams)
		case "processes":
			processes := []ProcessRecord{}
			unique := make(map[string]*ProcessRecord)
			if id, ok := vars["id"]; ok {
				if vanaddr, ok := fc.VanAddresses[id]; ok {
					for _, connector := range fc.Connectors {
						if *connector.Address == vanaddr.Name && connector.ProcessId != nil {
							if process, ok := fc.Processes[*connector.ProcessId]; ok {
								if filterRecord(*process, queryParams) && process.Base.TimeRangeValid(queryParams) {
									unique[process.Identity] = process
								}
							}
						}
					}
					for _, process := range unique {
						processes = append(processes, *process)
					}
				}
			}
			p.TotalCount = len(unique)
			retrieveError = sortAndSlice(processes, &p, queryParams)
		case "processpairs":
			processPairs := []FlowAggregateRecord{}
			if id, ok := vars["id"]; ok {
				if vanaddr, ok := fc.VanAddresses[id]; ok {
					for _, connector := range fc.Connectors {
						if *connector.Address == vanaddr.Name && connector.ProcessId != nil {
							for _, aggregate := range fc.FlowAggregates {
								if aggregate.PairType == recordNames[Process] {
									if *connector.ProcessId == *aggregate.DestinationId {
										p.TotalCount++
										if filterRecord(*aggregate, queryParams) {
											processPairs = append(processPairs, *aggregate)
										}
									}
								}
							}
						}
					}
				}
			}
			retrieveError = sortAndSlice(processPairs, &p, queryParams)
		case "listeners":
			listeners := []ListenerRecord{}
			if id, ok := vars["id"]; ok {
				if vanaddr, ok := fc.VanAddresses[id]; ok {
					for _, listener := range fc.Listeners {
						if *listener.Address == vanaddr.Name {
							p.TotalCount++
							if filterRecord(*listener, queryParams) && listener.Base.TimeRangeValid(queryParams) {
								listeners = append(listeners, *listener)
							}
						}
					}
				}
			}
			retrieveError = sortAndSlice(listeners, &p, queryParams)
		case "connectors":
			connectors := []ConnectorRecord{}
			if id, ok := vars["id"]; ok {
				if vanaddr, ok := fc.VanAddresses[id]; ok {
					for _, connector := range fc.Connectors {
						if *connector.Address == vanaddr.Name {
							p.TotalCount++
							if filterRecord(*connector, queryParams) && connector.Base.TimeRangeValid(queryParams) {
								connectors = append(connectors, *connector)
							}
						}
					}
				}
			}
			retrieveError = sortAndSlice(connectors, &p, queryParams)
		}
	case Process:
		switch request.HandlerName {
		case "list":
			processes := []ProcessRecord{}
			for _, process := range fc.Processes {
				if filterRecord(*process, queryParams) && process.Base.TimeRangeValid(queryParams) {
					if process.connector != nil {
						process.Addresses = nil
						if connector, ok := fc.Connectors[*process.connector]; ok {
							if connector.Address != nil && connector.AddressId != nil {
								addrDetails := *connector.Address + "@" + *connector.AddressId + "@" + *connector.Protocol
								process.Addresses = append(process.Addresses, &addrDetails)
							}
						}
					}
					processes = append(processes, *process)
				}
			}
			p.TotalCount = len(fc.Processes)
			retrieveError = sortAndSlice(processes, &p, queryParams)
		case "item":
			if id, ok := vars["id"]; ok {
				if process, ok := fc.Processes[id]; ok {
					p.Count = 1
					p.Results = process
				}
			}
		case "flows":
			flows := []FlowRecord{}
			if id, ok := vars["id"]; ok {
				for _, flow := range fc.Flows {
					if flow.Process != nil && *flow.Process == id {
						p.TotalCount++
						if filterRecord(*flow, queryParams) && flow.Base.TimeRangeValid(queryParams) {
							flows = append(flows, *flow)
						}
					}
				}
			}
			retrieveError = sortAndSlice(flows, &p, queryParams)
		case "addresses":
			addresses := []VanAddressRecord{}
			if id, ok := vars["id"]; ok {
				if _, ok := fc.Processes[id]; ok {
					for _, connector := range fc.Connectors {
						if connector.ProcessId != nil && *connector.ProcessId == id {
							for _, address := range fc.VanAddresses {
								if *connector.Address == address.Name {
									if filterRecord(*address, queryParams) {
										fc.getAddressAdaptorCounts(address)
										addresses = append(addresses, *address)
									}
								}
							}
						}
					}
				}
			}
			p.TotalCount = len(fc.VanAddresses)
			retrieveError = sortAndSlice(addresses, &p, queryParams)
		case "connector":
			if id, ok := vars["id"]; ok {
				if process, ok := fc.Processes[id]; ok {
					if process.connector != nil {
						if connector, ok := fc.Connectors[*process.connector]; ok {
							p.Count = 1
							p.Results = *connector
						}
					}
				}
			}
		}
	case ProcessGroup:
		switch request.HandlerName {
		case "list":
			processGroups := []ProcessGroupRecord{}
			for _, processGroup := range fc.ProcessGroups {
				count := 0
				for _, process := range fc.Processes {
					if *process.GroupIdentity == processGroup.Identity {
						count++
					}
				}
				processGroup.ProcessCount = count
				p.TotalCount++
				if filterRecord(*processGroup, queryParams) && processGroup.Base.TimeRangeValid(queryParams) {
					processGroups = append(processGroups, *processGroup)
				}
			}
			retrieveError = sortAndSlice(processGroups, &p, queryParams)
		case "item":
			if id, ok := vars["id"]; ok {
				if processGroup, ok := fc.ProcessGroups[id]; ok {
					count := 0
					for _, process := range fc.Processes {
						if *process.GroupIdentity == processGroup.Identity {
							count++
						}
					}
					processGroup.ProcessCount = count
					p.Count = 1
					p.Results = processGroup
				}
			}
		case "processes":
			processes := []ProcessRecord{}
			if id, ok := vars["id"]; ok {
				if processGroup, ok := fc.ProcessGroups[id]; ok {
					for _, process := range fc.Processes {
						if *process.GroupIdentity == processGroup.Identity {
							p.TotalCount++
							if filterRecord(*process, queryParams) && process.Base.TimeRangeValid(queryParams) {
								processes = append(processes, *process)
							}
						}
					}
				}
			}
			retrieveError = sortAndSlice(processes, &p, queryParams)
		}
	case Flow:
		switch request.HandlerName {
		case "list":
			flows := []FlowRecord{}
			for _, flow := range fc.Flows {
				if filterRecord(*flow, queryParams) && flow.Base.TimeRangeValid(queryParams) {
					flows = append(flows, *flow)
				}
			}
			p.TotalCount = len(fc.Flows)
			retrieveError = sortAndSlice(flows, &p, queryParams)
		case "item":
			if id, ok := vars["id"]; ok {
				if flow, ok := fc.Flows[id]; ok {
					p.Count = 1
					p.Results = flow
				}
			}
		case "process":
			if id, ok := vars["id"]; ok {
				if flow, ok := fc.Flows[id]; ok {
					if flow.Process != nil {
						if process, ok := fc.Processes[*flow.Process]; ok {
							p.Count = 1
							p.Results = process
						}
					}
				}
			}
		}
	case FlowPair:
		switch request.HandlerName {
		case "list":
			flowPairs := []FlowPairRecord{}
			for _, flowPair := range fc.FlowPairs {
				if filterRecord(*flowPair, queryParams) && flowPair.Base.TimeRangeValid(queryParams) {
					flowPairs = append(flowPairs, *flowPair)
				}
			}
			p.TotalCount = len(fc.FlowPairs)
			retrieveError = sortAndSlice(flowPairs, &p, queryParams)
		case "item":
			if id, ok := vars["id"]; ok {
				if flowPair, ok := fc.FlowPairs[id]; ok {
					p.Count = 1
					p.Results = flowPair
				}
			}
		}
	case SitePair:
		sourceId := url.Query().Get("sourceId")
		destinationId := url.Query().Get("destinationId")
		switch request.HandlerName {
		case "list":
			aggregates := []FlowAggregateRecord{}
			for _, aggregate := range fc.FlowAggregates {
				if aggregate.PairType == recordNames[Site] {
					p.TotalCount++
					if sourceId == "" && destinationId == "" ||
						sourceId == *aggregate.SourceId && destinationId == "" ||
						sourceId == "" && destinationId == *aggregate.DestinationId ||
						sourceId == *aggregate.SourceId && destinationId == *aggregate.DestinationId {
						if filterRecord(*aggregate, queryParams) {
							aggregates = append(aggregates, *aggregate)
						}
					}
				}
			}
			retrieveError = sortAndSlice(aggregates, &p, queryParams)
		case "item":
			if id, ok := vars["id"]; ok {
				if flowAggregate, ok := fc.FlowAggregates[id]; ok {
					if flowAggregate.PairType == recordNames[Site] {
						p.Count = 1
						p.Results = flowAggregate
					}
				}
			}
		}
	case ProcessGroupPair:
		sourceId := url.Query().Get("sourceId")
		destinationId := url.Query().Get("destinationId")
		switch request.HandlerName {
		case "list":
			aggregates := []FlowAggregateRecord{}
			for _, aggregate := range fc.FlowAggregates {
				if aggregate.PairType == recordNames[ProcessGroup] {
					p.TotalCount++
					if sourceId == "" && destinationId == "" ||
						sourceId == *aggregate.SourceId && destinationId == "" ||
						sourceId == "" && destinationId == *aggregate.DestinationId ||
						sourceId == *aggregate.SourceId && destinationId == *aggregate.DestinationId {
						if filterRecord(*aggregate, queryParams) {
							aggregates = append(aggregates, *aggregate)
						}
					}
				}
			}
			retrieveError = sortAndSlice(aggregates, &p, queryParams)
		case "item":
			if id, ok := vars["id"]; ok {
				if flowAggregate, ok := fc.FlowAggregates[id]; ok {
					if flowAggregate.PairType == recordNames[ProcessGroup] {
						p.Count = 1
						p.Results = flowAggregate
					}
				}
			}
		}
	case ProcessPair:
		sourceId := url.Query().Get("sourceId")
		destinationId := url.Query().Get("destinationId")
		switch request.HandlerName {
		case "list":
			aggregates := []FlowAggregateRecord{}
			for _, aggregate := range fc.FlowAggregates {
				if aggregate.PairType == recordNames[Process] {
					p.TotalCount++
					if sourceId == "" && destinationId == "" ||
						sourceId == *aggregate.SourceId && destinationId == "" ||
						sourceId == "" && destinationId == *aggregate.DestinationId ||
						sourceId == *aggregate.SourceId && destinationId == *aggregate.DestinationId {
						// try to associate a protocol to the process pair
						if process, ok := fc.Processes[*aggregate.DestinationId]; ok {
							if process.connector != nil {
								if connector, ok := fc.Connectors[*process.connector]; ok {
									aggregate.Protocol = connector.Protocol
								}
							}
						}
						if filterRecord(*aggregate, queryParams) {
							aggregates = append(aggregates, *aggregate)
						}
					}
				}
			}
			retrieveError = sortAndSlice(aggregates, &p, queryParams)
		case "item":
			if id, ok := vars["id"]; ok {
				if flowAggregate, ok := fc.FlowAggregates[id]; ok {
					if flowAggregate.PairType == recordNames[Process] {
						if process, ok := fc.Processes[*flowAggregate.DestinationId]; ok {
							if process.connector != nil {
								if connector, ok := fc.Connectors[*process.connector]; ok {
									flowAggregate.Protocol = connector.Protocol
								}
							}
						}
						p.Count = 1
						p.Results = flowAggregate
					}
				}
			}
		}
	case FlowAggregate:
		sourceId := url.Query().Get("sourceId")
		destinationId := url.Query().Get("destinationId")
		switch request.HandlerName {
		case "sitepair-list":
			aggregates := []FlowAggregateRecord{}
			for _, aggregate := range fc.FlowAggregates {
				if aggregate.PairType == recordNames[Site] {
					p.TotalCount++
					if sourceId == "" && destinationId == "" ||
						sourceId == *aggregate.SourceId && destinationId == "" ||
						sourceId == "" && destinationId == *aggregate.DestinationId ||
						sourceId == *aggregate.SourceId && destinationId == *aggregate.DestinationId {
						aggregates = append(aggregates, *aggregate)
					}
				}
			}
			retrieveError = sortAndSlice(aggregates, &p, queryParams)
		case "sitepair-item":
			if id, ok := vars["id"]; ok {
				if flowAggregate, ok := fc.FlowAggregates[id]; ok {
					if flowAggregate.PairType == recordNames[Site] {
						p.Count = 1
						p.Results = flowAggregate
					}
				}
			}
		case "processpair-list":
			aggregates := []FlowAggregateRecord{}
			for _, aggregate := range fc.FlowAggregates {
				if aggregate.PairType == recordNames[Process] {
					p.TotalCount++
					if sourceId == "" && destinationId == "" ||
						sourceId == *aggregate.SourceId && destinationId == "" ||
						sourceId == "" && destinationId == *aggregate.DestinationId ||
						sourceId == *aggregate.SourceId && destinationId == *aggregate.DestinationId {
						aggregates = append(aggregates, *aggregate)
					}
				}
			}
			retrieveError = sortAndSlice(aggregates, &p, queryParams)
		case "processpair-item":
			if id, ok := vars["id"]; ok {
				if flowAggregate, ok := fc.FlowAggregates[id]; ok {
					if flowAggregate.PairType == recordNames[Process] {
						p.Count = 1
						p.Results = flowAggregate
					}
				}
			}
		case "processgrouppair-list":
			aggregates := []FlowAggregateRecord{}
			for _, aggregate := range fc.FlowAggregates {
				if aggregate.PairType == recordNames[ProcessGroup] {
					p.TotalCount++
					if sourceId == "" && destinationId == "" ||
						sourceId == *aggregate.SourceId && destinationId == "" ||
						sourceId == "" && destinationId == *aggregate.DestinationId ||
						sourceId == *aggregate.SourceId && destinationId == *aggregate.DestinationId {
						aggregates = append(aggregates, *aggregate)
					}
				}
			}
			retrieveError = sortAndSlice(aggregates, &p, queryParams)
		case "processgrouppair-item":
			if id, ok := vars["id"]; ok {
				if flowAggregate, ok := fc.FlowAggregates[id]; ok {
					if flowAggregate.PairType == recordNames[ProcessGroup] {
						p.Count = 1
						p.Results = flowAggregate
					}
				}
			}
		}
	case EventSource:
		switch request.HandlerName {
		case "list":
			eventSources := []EventSourceRecord{}
			for _, eventSource := range fc.eventSources {
				if filterRecord(eventSource.EventSourceRecord, queryParams) && eventSource.EventSourceRecord.Base.TimeRangeValid(queryParams) {
					eventSources = append(eventSources, eventSource.EventSourceRecord)
				}
			}
			p.TotalCount = len(fc.eventSources)
			retrieveError = sortAndSlice(eventSources, &p, queryParams)
		case "item":
			if id, ok := vars["id"]; ok {
				if eventSource, ok := fc.eventSources[id]; ok {
					p.Count = 1
					p.Results = eventSource.EventSourceRecord
				}
			}
		}
	case Collector:
		// TODO: emit and collect Collector records
		switch request.HandlerName {
		case "list":
			collectors := []CollectorRecord{}
			collectors = append(collectors, fc.Collector)
			p.TotalCount = len(collectors)
			retrieveError = sortAndSlice(collectors, &p, queryParams)
		case "item":
			p.Count = 1
			p.Results = &fc.Collector
		case "connectors-to-process":
			connectors := []string{}
			for _, connId := range fc.connectorsToReconcile {
				if _, ok := fc.Connectors[connId]; ok {
					p.TotalCount++
					connectors = append(connectors, connId)
				}
			}
			retrieveError = sortAndSlice(connectors, &p, queryParams)
		case "flows-to-pair":
			flows := []string{}
			for reverseId, ftpr := range fc.flowsToPairReconcile {
				p.TotalCount++
				flows = append(flows, ftpr.forwardId+"-"+reverseId)
			}
			retrieveError = sortAndSlice(flows, &p, queryParams)
		case "flows-to-process":
			flows := []string{}
			for _, flowId := range fc.flowsToProcessReconcile {
				if _, ok := fc.Flows[flowId]; ok {
					p.TotalCount++
					flows = append(flows, flowId)
				}
			}
			retrieveError = sortAndSlice(flows, &p, queryParams)
		case "pair-to-aggregate":
			flowPairs := []string{}
			for flowPairId := range fc.aggregatesToReconcile {
				if _, ok := fc.FlowPairs[flowPairId]; ok {
					p.TotalCount++
					flowPairs = append(flowPairs, flowPairId)
				}
			}
			retrieveError = sortAndSlice(flowPairs, &p, queryParams)
		}
	default:
		log.Println("COLLECTOR: Unrecognize record request", request.RecordType)
	}
	if retrieveError != nil {
		p.Status = retrieveError.Error()
	}
	p.elapsed = uint64(time.Now().UnixNano())/uint64(time.Microsecond) - p.timestamp
	apiQueryLatencyMetric, err := fc.metrics.apiQueryLatency.GetMetricWith(map[string]string{"recordType": recordNames[request.RecordType], "handler": request.HandlerName})
	if err == nil {
		apiQueryLatencyMetric.Observe(float64(p.elapsed))
	}
	data, err := json.MarshalIndent(p, "", " ")
	if err != nil {
		log.Println("COLLECTOR: Error marshalling results", err.Error())
		return nil, err
	}
	sd := string(data)
	return &sd, nil
}

func (fc *FlowCollector) getFlowProcess(id string) (ProcessRecord, bool) {
	if flow, ok := fc.Flows[id]; ok {
		if flow.Process != nil {
			if process, ok := fc.Processes[*flow.Process]; ok {
				return *process, ok
			}
		}
	}
	return ProcessRecord{}, false
}

func (fc *FlowCollector) setupFlowMetrics(va *VanAddressRecord, flow *FlowRecord, metricLabel prometheus.Labels) error {
	var flowMetric prometheus.Counter
	var octetMetric prometheus.Counter
	var flowLatencyMetric prometheus.Observer
	var lastAccessedMetric prometheus.Gauge
	var activeFlowMetric prometheus.Gauge
	var err error
	var ok bool

	key := metricKey{}
	if key.sourceSite, ok = metricLabel["sourceSite"]; !ok {
		return fmt.Errorf("Metric label missing source site key")
	}
	if key.sourceProcess, ok = metricLabel["sourceProcess"]; !ok {
		return fmt.Errorf("Metric label missing source process key")
	}
	if key.sourceHost, ok = metricLabel["sourceHost"]; !ok {
		return fmt.Errorf("Metric label missing source host key")
	}
	if key.sourceHost, ok = metricLabel["destHost"]; !ok {
		return fmt.Errorf("Metric label missing dest host key")
	}
	if key.destSite, ok = metricLabel["destSite"]; !ok {
		return fmt.Errorf("Metric label missing dest site key")
	}
	if key.destProcess, ok = metricLabel["destProcess"]; !ok {
		return fmt.Errorf("Metric label missing dest process key")
	}

	if flowMetric, ok = va.flowCount[key]; !ok {
		flowMetric, err = fc.metrics.flows.GetMetricWith(metricLabel)
		if err != nil {
			return err
		} else {
			va.flowCount[key] = flowMetric
		}
	}
	flowMetric.Inc()

	if octetMetric, ok = va.octetCount[key]; !ok {
		octetMetric, err = fc.metrics.octets.GetMetricWith(metricLabel)
		if err != nil {
			return err
		} else {
			va.octetCount[key] = octetMetric
		}
	}
	flow.octetMetric = octetMetric
	if flow.Octets != nil {
		octetMetric.Add(float64(*flow.Octets))
		flow.lastOctets = *flow.Octets
	}

	if flowLatencyMetric, ok = va.flowLatency[key]; !ok {
		flowLatencyMetric, err = fc.metrics.flowLatency.GetMetricWith(metricLabel)
		if err != nil {
			return err
		} else {
			va.flowLatency[key] = flowLatencyMetric
		}
	}
	if flow.Latency != nil {
		flowLatencyMetric.Observe(float64(*flow.Latency))
	}

	if lastAccessedMetric, ok = va.lastAccessed[key]; !ok {
		lastAccessedMetric, err = fc.metrics.lastAccessed.GetMetricWith(metricLabel)
		if err != nil {
			return err
		} else {
			va.lastAccessed[key] = lastAccessedMetric
		}
	}
	lastAccessedMetric.Set(float64((flow.StartTime / uint64(time.Microsecond))))

	if activeFlowMetric, ok = va.activeFlowCount[key]; !ok {
		activeFlowMetric, err = fc.metrics.activeFlows.GetMetricWith(metricLabel)
		if err != nil {
			return err
		} else {
			va.activeFlowCount[key] = activeFlowMetric
		}
	}
	flow.activeFlowMetric = activeFlowMetric
	if flow.EndTime == 0 {
		activeFlowMetric.Inc()
	}

	if direction, ok := metricLabel["direction"]; ok {
		if direction == Incoming {
			if flow.Method != nil {
				metricLabel["method"] = *flow.Method
				httpReqsMethod, err := fc.metrics.httpReqsMethod.GetMetricWith(metricLabel)
				if err != nil {
					return err
				} else {
					httpReqsMethod.Inc()
				}
				delete(metricLabel, "method")
			}
		}
		// note: observed code to be present in both forward in reverse flows (e.g. directions)
		if flow.Result != nil {
			metricLabel["code"] = *flow.Result
			httpReqsResult, err := fc.metrics.httpReqsResult.GetMetricWith(metricLabel)
			if err != nil {
				return err
			} else {
				httpReqsResult.Inc()
			}
			delete(metricLabel, "code")
		}
	}
	return nil
}

func (fc *FlowCollector) reconcileFlowRecords() error {
	age := uint64(time.Now().UnixNano())/uint64(time.Microsecond) - uint64(fc.recordTtl.Microseconds())

	m, err := fc.metrics.activeReconcile.GetMetricWith(prometheus.Labels{"reconcileTask": "flowsToProcess"})
	if err == nil {
		m.Set(float64(len(fc.flowsToProcessReconcile)))
	}
	for _, flowId := range fc.flowsToProcessReconcile {
		if flow, ok := fc.Flows[flowId]; ok {
			siteId := fc.getRecordSiteId(*flow)
			if siteId == "" {
				continue
			}
			if flow.SourceHost != nil {
				if connector, ok := fc.Connectors[flow.Parent]; ok {
					flow.Place = serverSide
					flow.Protocol = connector.Protocol
					if connector.ProcessId != nil && connector.AddressId != nil {
						flow.Process = connector.ProcessId
						if process, ok := fc.Processes[*flow.Process]; ok {
							flow.ProcessName = process.Name
						}
						delete(fc.flowsToProcessReconcile, flowId)
					} else {
						if fc.needForSiteProcess(flow, siteId, connector.StartTime, false) {
							delete(fc.flowsToProcessReconcile, flowId)
						}
					}
				} else if listener, ok := fc.Listeners[flow.Parent]; ok {
					flow.Place = clientSide
					flow.Protocol = listener.Protocol
					if listener.AddressId != nil {
						found := false
						for _, process := range fc.Processes {
							if siteId == process.Parent && process.SourceHost != nil {
								if *flow.SourceHost == *process.SourceHost {
									flow.Process = &process.Identity
									flow.ProcessName = process.Name
									found = true
									delete(fc.flowsToProcessReconcile, flowId)
								}
							}
						}
						if !found {
							if fc.needForSiteProcess(flow, siteId, listener.StartTime, true) {
								delete(fc.flowsToProcessReconcile, flowId)
							}
						}
					}
				}
			} else if l4Flow, ok := fc.Flows[flow.Parent]; ok {
				if l4Flow.Process != nil {
					flow.Process = l4Flow.Process
					flow.ProcessName = l4Flow.ProcessName
					flow.Protocol = fc.getFlowProtocol(l4Flow)
					delete(fc.flowsToProcessReconcile, flowId)
				}
			}
		} else {
			delete(fc.flowsToProcessReconcile, flowId)
		}
	}
	m, err = fc.metrics.activeReconcile.GetMetricWith(prometheus.Labels{"reconcileTask": "flowsToPair"})
	if err == nil {
		m.Set(float64(len(fc.flowsToPairReconcile)))
	}
	for reverseId, ftpr := range fc.flowsToPairReconcile {
		if age > ftpr.created {
			delete(fc.flowsToPairReconcile, reverseId)
		} else if reverseFlow, ok := fc.Flows[reverseId]; ok {
			if forwardFlow, ok := fc.Flows[ftpr.forwardId]; ok {
				forwardFlow.CounterFlow = &reverseFlow.Identity
				flowPair, ok := fc.linkFlowPair(forwardFlow)
				if ok {
					flowPair.Protocol = forwardFlow.Protocol
					fc.FlowPairs[flowPair.Identity] = flowPair
					fc.aggregatesToReconcile[flowPair.Identity] = flowPair
					delete(fc.flowsToPairReconcile, reverseId)
				}
			}
		}
	}
	m, err = fc.metrics.activeReconcile.GetMetricWith(prometheus.Labels{"reconcileTask": "pairToAggregate"})
	if err == nil {
		m.Set(float64(len(fc.aggregatesToReconcile)))
	}
	for flowPairId, flowPair := range fc.aggregatesToReconcile {
		if flowPair.ForwardFlow != nil && flowPair.CounterFlow != nil {
			ffp, ffpOk := fc.getFlowProcess(flowPair.ForwardFlow.Identity)
			cfp, cfpOk := fc.getFlowProcess(flowPair.CounterFlow.Identity)
			if ffpOk && cfpOk {
				siteAggregateId := flowPair.SourceSiteId + "-to-" + flowPair.DestinationSiteId
				if _, ok := fc.FlowAggregates[siteAggregateId]; !ok {
					sfa := &FlowAggregateRecord{
						Base: Base{
							RecType:   recordNames[FlowAggregate],
							Identity:  siteAggregateId,
							StartTime: uint64(time.Now().UnixNano()) / uint64(time.Microsecond),
						},
						PairType:      recordNames[Site],
						SourceId:      &flowPair.SourceSiteId,
						DestinationId: &flowPair.DestinationSiteId,
					}
					if sourceSite, ok := fc.Sites[flowPair.SourceSiteId]; ok {
						sfa.SourceName = sourceSite.Name
					}
					if destinationSite, ok := fc.Sites[flowPair.DestinationSiteId]; ok {
						sfa.DestinationName = destinationSite.Name
					}
					fc.FlowAggregates[siteAggregateId] = sfa
				}
				flowPair.SiteAggregateId = &siteAggregateId
				// next process pairs
				processAggregateId := ffp.Identity + "-to-" + cfp.Identity
				if _, ok := fc.FlowAggregates[processAggregateId]; !ok {
					pfa := &FlowAggregateRecord{
						Base: Base{
							RecType:   recordNames[FlowAggregate],
							Identity:  processAggregateId,
							StartTime: uint64(time.Now().UnixNano()) / uint64(time.Microsecond),
						},
						PairType:      recordNames[Process],
						SourceId:      &ffp.Identity,
						DestinationId: &cfp.Identity,
					}
					if sourceProcess, ok := fc.Processes[ffp.Identity]; ok {
						pfa.SourceName = sourceProcess.Name
						sourceSiteId := fc.getRecordSiteId(*sourceProcess)
						if sourceSite, ok := fc.Sites[sourceSiteId]; ok {
							pfa.SourceSiteId = &sourceSiteId
							pfa.SourceSiteName = sourceSite.Name
						}
					}
					if destinationProcess, ok := fc.Processes[cfp.Identity]; ok {
						if destinationProcess.Name != nil {
							pfa.DestinationName = destinationProcess.Name
						}
						destinationSiteId := fc.getRecordSiteId(*destinationProcess)
						if destinationSite, ok := fc.Sites[destinationSiteId]; ok {
							pfa.DestinationSiteId = &destinationSiteId
							pfa.DestinationSiteName = destinationSite.Name
						}
					}
					fc.FlowAggregates[processAggregateId] = pfa
				}
				flowPair.ProcessAggregateId = &processAggregateId
				// next process group pairs
				processGroupAggregateId := *ffp.GroupIdentity + "-to-" + *cfp.GroupIdentity
				if _, ok := fc.FlowAggregates[processGroupAggregateId]; !ok {
					pgfa := &FlowAggregateRecord{
						Base: Base{
							RecType:   recordNames[FlowAggregate],
							Identity:  processGroupAggregateId,
							StartTime: uint64(time.Now().UnixNano()) / uint64(time.Microsecond),
						},
						PairType:      recordNames[ProcessGroup],
						SourceId:      ffp.GroupIdentity,
						DestinationId: cfp.GroupIdentity,
					}
					if sourceProcessGroup, ok := fc.ProcessGroups[*ffp.GroupIdentity]; ok {
						pgfa.SourceName = sourceProcessGroup.Name
					}
					if destinationProcessGroup, ok := fc.ProcessGroups[*cfp.GroupIdentity]; ok {
						pgfa.DestinationName = destinationProcessGroup.Name
					}
					fc.FlowAggregates[processGroupAggregateId] = pgfa
				}
				flowPair.ProcessGroupAggregateId = &processGroupAggregateId
				delete(fc.aggregatesToReconcile, flowPairId)
			}
		}
	}
	return nil
}

func (fc *FlowCollector) reconcileConnectorRecords() error {
	for _, connId := range fc.connectorsToReconcile {
		t := time.Now()
		if connector, ok := fc.Connectors[connId]; ok {
			if connector.EndTime > 0 {
				delete(fc.connectorsToReconcile, connId)
			} else if connector.DestHost != nil {
				siteId := fc.getRecordSiteId(*connector)
				var matchHost *string
				found := false
				if net.ParseIP(*connector.DestHost) != nil {
					matchHost = connector.DestHost
				}
				for _, process := range fc.Processes {
					if siteId == process.Parent {
						if process.SourceHost != nil && matchHost != nil {
							if *matchHost == *process.SourceHost {
								found = true
							}
						} else if process.HostName != nil {
							if *process.HostName == *connector.DestHost {
								found = true
							}
						}
						if found {
							connector.ProcessId = &process.Identity
							connector.Target = process.Name
							process.connector = &connector.Identity
							process.ProcessBinding = &Bound
							if fc.mode == RecordStatus {
								fc.updateNetworkStatus()
							}
							log.Printf("COLLECTOR: Connector %s/%s associated to process %s\n", connector.Identity, *connector.Address, *process.Name)
							delete(fc.connectorsToReconcile, connId)
							break
						}
					}
				}
				if !found {
					parts := strings.Split(siteId, "-")
					processName := "site-servers-" + parts[0]
					diffTime := connector.StartTime
					wait := 30 * oneSecond
					if fc.startTime > connector.StartTime {
						diffTime = fc.startTime
						wait = 120 * oneSecond
					}
					diff := uint64(t.UnixNano())/uint64(time.Microsecond) - diffTime
					if diff > wait {
						for _, process := range fc.Processes {
							if process.Name != nil && *process.Name == processName {
								log.Printf("COLLECTOR: Associating connector %s to external process %s\n", connector.Identity, processName)
								process.ProcessBinding = &Bound
								process.connector = &connector.Identity
								connector.ProcessId = &process.Identity
								delete(fc.connectorsToReconcile, connId)
								break
							}
						}
					}
				}
			}
		} else {
			delete(fc.connectorsToReconcile, connId)
		}
	}
	return nil
}

func (fc *FlowCollector) ageAndPurgeRecords() error {
	age := uint64(time.Now().UnixNano())/uint64(time.Microsecond) - uint64(fc.recordTtl.Microseconds())

	for flowId, flow := range fc.Flows {
		router := fc.getRouterForFlow(flow)
		if flow.EndTime != 0 && age > flow.EndTime || router == nil {
			fc.deleteRecord(flow)
			if flowPair, ok := fc.FlowPairs["fp-"+flowId]; ok {
				fc.deleteRecord(flowPair)
			}
		}
	}

	t := time.Now()
	for _, source := range fc.eventSources {
		diff := uint64(t.UnixNano())/uint64(time.Microsecond) - source.EventSourceRecord.LastHeard
		if diff > 60*oneSecond {
			log.Printf("COLLECTOR: Purging event source %s of type %s \n", source.Beacon.Identity, source.Beacon.SourceType)
			fc.purgeEventSource(source.EventSourceRecord)
		}
	}

	// recentConnectors for flows after the fact
	for _, connector := range fc.recentConnectors {
		diff := uint64(t.UnixNano())/uint64(time.Microsecond) - connector.EndTime
		if diff > 120*oneSecond {
			delete(fc.recentConnectors, connector.Identity)
		}
	}

	return nil
}

func (fc *FlowCollector) getAddressAdaptorCounts(addr *VanAddressRecord) error {
	listenerCount := 0
	connectorCount := 0
	for _, listener := range fc.Listeners {
		if listener.Address != nil && *listener.Address == addr.Name {
			listenerCount++
		}
	}
	for _, connector := range fc.Connectors {
		if connector.Address != nil && *connector.Address == addr.Name {
			connectorCount++
		}
	}
	addr.ListenerCount = listenerCount
	addr.ConnectorCount = connectorCount

	return nil
}

func (fc *FlowCollector) purgeEventSource(eventSource EventSourceRecord) error {
	// it would be good to indicate the reason
	t := time.Now()
	now := uint64(t.UnixNano()) / uint64(time.Microsecond)

	switch eventSource.Beacon.SourceType {
	case recordNames[Router]:
		for _, listener := range fc.Listeners {
			if listener.Parent == eventSource.Identity {
				listener.EndTime = now
				listener.Purged = true
				fc.updateRecord(*listener)
			}
		}
		for _, connector := range fc.Connectors {
			if connector.Parent == eventSource.Identity {
				connector.EndTime = now
				fc.updateRecord(*connector)
			}
		}
		for _, link := range fc.Links {
			if link.Parent == eventSource.Identity {
				link.EndTime = now
				link.Purged = true
				fc.updateRecord(*link)
			}
		}
		if router, ok := fc.Routers[eventSource.Identity]; ok {
			// workaround for gateway site
			if fc.isGatewaySite(router.Parent) {
				for _, process := range fc.Processes {
					if process.Parent == router.Parent {
						process.EndTime = now
						process.Purged = true
						fc.updateRecord(*process)
					}
				}
				if site, ok := fc.Sites[router.Parent]; ok {
					site.EndTime = now
					site.Purged = true
					fc.updateRecord(*site)
				}
			}
			router.EndTime = now
			router.Purged = true
			fc.updateRecord(*router)
		}
	case recordNames[Controller]:
		for _, process := range fc.Processes {
			if process.Parent == eventSource.Identity {
				process.EndTime = now
				process.Purged = true
				fc.updateRecord(*process)
			}
		}
		for _, host := range fc.Hosts {
			if host.Parent == eventSource.Identity {
				host.EndTime = now
				host.Purged = true
				fc.updateRecord(*host)
			}
		}
		for _, site := range fc.Sites {
			if site.Identity == eventSource.Identity {
				site.EndTime = now
				site.Purged = true
				fc.updateRecord(*site)
			}
		}
	}
	for id, es := range fc.eventSources {
		if id == eventSource.Identity {
			for _, receiver := range es.receivers {
				receiver.stop()
			}
			es.send.sender.stop()
		}
	}
	eventSource.Purged = true
	eventSource.EndTime = now
	log.Printf("COLLECTOR: %s \n", prettyPrint(eventSource))
	delete(fc.eventSources, eventSource.Identity)

	return nil
}

func (fc *FlowCollector) createSiteProcess(name string, site SiteRecord) ProcessRecord {
	parts := strings.Split(site.Identity, "-")
	processName := name + "-" + parts[0]
	processGroupName := name
	process := ProcessRecord{}
	process.RecType = recordNames[Process]
	process.Identity = uuid.New().String()
	process.Parent = site.Identity
	process.ParentName = site.Name
	process.StartTime = site.StartTime
	process.Name = &processName
	process.GroupName = &processGroupName
	process.ProcessRole = &Remote
	fc.updateRecord(process)
	return process
}

func (fc *FlowCollector) needForSiteProcess(flow *FlowRecord, siteId string, startTime uint64, client bool) bool {
	if fc.getFlowPlace(flow) == unknown {
		return false
	}
	parts := strings.Split(siteId, "-")
	name := "site-servers"
	if client {
		name = "site-clients"
	}
	processName := name + "-" + parts[0]
	diffTime := startTime
	wait := 120 * oneSecond
	if fc.startTime > startTime {
		diffTime = fc.startTime
		wait = 120 * oneSecond
	}
	diff := uint64(time.Now().UnixNano())/uint64(time.Microsecond) - diffTime
	found := false
	if diff > wait && flow.Process == nil {
		log.Printf("COLLECTOR: Associating flow %s to external process %s\n", flow.Identity, processName)
		for _, process := range fc.Processes {
			if process.Name != nil && *process.Name == processName {
				flow.Process = &process.Identity
				flow.ProcessName = process.Name
				found = true
				break
			}
		}
		if !found {
			if site, ok := fc.Sites[siteId]; ok {
				process := fc.createSiteProcess(name, *site)
				flow.Process = &process.Identity
				flow.ProcessName = &processName
				found = true
			}
		}
	}
	return found
}

// graph relations between routers and sites using LinkRecords
//
// Collector state is fallible, as such this graph is meant to be conservative
// in the edges it includes between nodes. It de-duplicates links (see
// skupperproject/skupper-router issue #1456) and ensures both the listener and
// connector sides of inter-router links are present representing either.
func (fc *FlowCollector) graph() (routers, sites map[string]*node) {
	siteByRouterID := make(map[string]string, len(fc.Routers))
	routerIDByName := make(map[string]string, len(fc.Routers))
	for _, router := range fc.Routers {
		if _, ok := fc.Sites[router.Parent]; !ok || router.Name == nil {
			continue
		}
		siteByRouterID[router.Identity] = router.Parent
		routerIDByName[normalizeRouterName(*router.Name)] = router.Identity
	}
	setLink := func(set map[string]map[string]string, local, peer, role string) {
		s, ok := set[local]
		if !ok {
			set[local] = make(map[string]string)
			s = set[local]
		}
		s[peer] = role
	}
	// find unique links by localRotuerID and peerRouterID
	outgoing := make(map[string]map[string]string)
	incoming := make(map[string]map[string]string)
	for _, link := range fc.Links {
		if link.Direction == nil || link.Name == nil {
			continue
		}
		localRouterID, peerRouterName := link.Parent, *link.Name

		linkRole := "inter-router"
		if link.Mode != nil && *link.Mode != "" {
			linkRole = *link.Mode
		}

		if _, ok := fc.Routers[localRouterID]; !ok {
			continue
		}
		peerRouterID, ok := routerIDByName[peerRouterName]
		if !ok {
			continue
		}
		switch *link.Direction {
		case "incoming":
			setLink(incoming, localRouterID, peerRouterID, linkRole)
		case "outgoing":
			setLink(outgoing, localRouterID, peerRouterID, linkRole)
		}
	}

	routerNodes := make(map[string]*node, len(fc.Routers))
	for id := range fc.Routers {
		routerNodes[id] = &node{ID: id}
	}
	siteNodes := make(map[string]*node, len(fc.Sites))
	for id := range fc.Sites {
		siteNodes[id] = &node{ID: id}
	}

	for router, links := range outgoing {
		for peerRouter, role := range links {
			switch role {
			// Since the router does not presently include Link records for
			// edge router connections on the listener side, for edge router
			// links we include all unique links on the connector (Outgoing)
			// side.
			case "edge":
				site, peerSite := siteByRouterID[router], siteByRouterID[peerRouter]
				rNode, siteNode := routerNodes[router], siteNodes[site]
				if rNode == nil || siteNode == nil {
					continue
				}
				rNode.Forward = append(rNode.Forward, peerRouter)
				if site != peerSite {
					siteNode.Forward = append(siteNode.Forward, peerSite)
				}
			default:
				fallthrough
			case "inter-router":
				// Ignore inter-router links only when the connector side
				// exists but not the listener side or vice-versa. When one or
				// the other of these records is missing it is likely that the
				// connection is down.
				peerLinks, ok := incoming[peerRouter]
				if !ok {
					continue
				}
				if _, ok := peerLinks[router]; !ok {
					continue
				}
				site, peerSite := siteByRouterID[router], siteByRouterID[peerRouter]
				rNode, peerNode := routerNodes[router], routerNodes[peerRouter]
				rNode.Forward = append(rNode.Forward, peerRouter)
				peerNode.Backward = append(peerNode.Backward, router)
				if site != peerSite {
					siteNode, peerSiteNode := siteNodes[site], siteNodes[peerSite]
					siteNode.Forward = append(siteNode.Forward, peerSite)
					peerSiteNode.Backward = append(peerSiteNode.Backward, site)
				}
			}
		}
	}
	return routerNodes, siteNodes
}

type node struct {
	ID       string
	Forward  []string
	Backward []string
}

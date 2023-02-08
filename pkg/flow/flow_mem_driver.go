package flow

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
)

func (c *FlowCollector) inferGatewayProcess(siteId string, flow FlowRecord, connector bool) error {
	sourceHost := flow.SourceHost
	if connector {
		if connector, ok := c.Connectors[flow.Parent]; ok {
			sourceHost = connector.DestHost
		}
	}

	if site, ok := c.Sites[siteId]; ok {
		groupName := *site.NameSpace + "-" + *sourceHost
		groupIdentity := ""
		for _, pg := range c.ProcessGroups {
			if *pg.Name == groupName {
				groupIdentity = pg.Identity
				break
			}
		}
		if groupIdentity == "" {
			groupIdentity = uuid.New().String()
			c.ProcessGroups[groupIdentity] = &ProcessGroupRecord{
				Base: Base{
					RecType:   recordNames[ProcessGroup],
					Identity:  groupIdentity,
					StartTime: uint64(time.Now().UnixNano()) / uint64(time.Microsecond),
				},
				Name: &groupName,
			}
		}
		processName := *site.Name + "-" + *sourceHost
		procFound := false
		for _, proc := range c.Processes {
			if *proc.Name == processName {
				procFound = true
				break
			}
		}
		if !procFound {
			log.Println("Inferring gateway process for flow: ", *sourceHost)
			procIdentity := uuid.New().String()
			c.Processes[procIdentity] = &ProcessRecord{
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
				SourceHost:    sourceHost,
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

func (fc *FlowCollector) getFlowPlace(flow *FlowRecord) FlowPlace {
	if _, ok := fc.Listeners[flow.Parent]; ok {
		return clientSide
	} else if _, ok := fc.Connectors[flow.Parent]; ok {
		return serverSide
	} else if l4Flow, ok := fc.Flows[flow.Parent]; ok {
		if _, ok := fc.Listeners[l4Flow.Parent]; ok {
			return clientSide
		} else if _, ok := fc.Connectors[l4Flow.Parent]; ok {
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
	var sourceSiteId, destSiteId string = "", ""
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
	sourceSiteId = fc.getRecordSiteId(*sourceFlow)
	destSiteId = fc.getRecordSiteId(*destFlow)
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
	return fp, ok
}

func (fc *FlowCollector) updateRecord(record interface{}) error {
	switch record.(type) {
	case HeartbeatRecord:
		if heartbeat, ok := record.(HeartbeatRecord); ok {
			if eventsource, ok := fc.eventSources[heartbeat.Identity]; ok {
				eventsource.LastHeard = heartbeat.Now
				eventsource.Heartbeats++
			}
			if pending, ok := fc.pendingFlush[heartbeat.Source]; ok {
				pending.heartbeat = true
			}
		}
	case SiteRecord:
		if site, ok := record.(SiteRecord); ok {
			if site.StartTime > 0 && site.EndTime == 0 {
				if current, ok := fc.Sites[site.Identity]; !ok {
					fc.Sites[site.Identity] = &site
				} else {
					*current = site
				}
			}
		}
	case HostRecord:
		if host, ok := record.(HostRecord); ok {
			if host.StartTime > 0 && host.EndTime == 0 {
				if current, ok := fc.Hosts[host.Identity]; !ok {
					fc.Hosts[host.Identity] = &host
				} else {
					*current = host
				}
			}
		}
	case RouterRecord:
		if router, ok := record.(RouterRecord); ok {
			if router.StartTime > 0 && router.EndTime == 0 {
				if _, ok := fc.Routers[router.Identity]; !ok {
					fc.Routers[router.Identity] = &router
					if router.Parent != "" {
						if _, ok := fc.Sites[router.Parent]; !ok {
							fc.routersToSiteReconcile[router.Identity] = router.Parent
						}
					}
				}
			}
		}
	case LinkRecord:
		if link, ok := record.(LinkRecord); ok {
			if current, ok := fc.Links[link.Identity]; !ok {
				fc.Links[link.Identity] = &link
			} else {
				if link.EndTime > 0 {
					current.EndTime = link.EndTime
				}
			}
		}
	case ListenerRecord:
		if listener, ok := record.(ListenerRecord); ok {
			if current, ok := fc.Listeners[listener.Identity]; !ok {
				if listener.StartTime > 0 && listener.EndTime == 0 {
					fc.Listeners[listener.Identity] = &listener
					if listener.Address != nil {
						var addr *VanAddressRecord
						for _, y := range fc.VanAddresses {
							if y.Name == *listener.Address {
								addr = y
								break
							}
						}
						if addr == nil {
							va := &VanAddressRecord{
								Base: Base{
									RecType:   attributeNames[VanAddress],
									Identity:  uuid.New().String(),
									StartTime: listener.StartTime,
								},
								Name:     *listener.Address,
								Protocol: *listener.Protocol,
							}
							fc.VanAddresses[va.Identity] = va
						}
					}
				}
			} else {
				if current.EndTime == 0 && listener.EndTime > 0 {
					current.EndTime = listener.EndTime
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
						var addr *VanAddressRecord
						for _, y := range fc.VanAddresses {
							if y.Name == *listener.Address {
								addr = y
								break
							}
						}
						if addr == nil {
							t := time.Now()
							va := &VanAddressRecord{
								Base: Base{
									RecType:   attributeNames[VanAddress],
									Identity:  uuid.New().String(),
									StartTime: uint64(t.UnixNano()) / uint64(time.Microsecond),
								},
								Name:     *listener.Address,
								Protocol: *listener.Protocol,
							}
							fc.VanAddresses[va.Identity] = va
						}
					}
				}
			}
		}
	case ConnectorRecord:
		if connector, ok := record.(ConnectorRecord); ok {
			if current, ok := fc.Connectors[connector.Identity]; !ok {
				if connector.StartTime > 0 && connector.EndTime == 0 {
					fc.Connectors[connector.Identity] = &connector
					if connector.Parent != "" {
						if connector.Address != nil {
							var addr *VanAddressRecord
							for _, y := range fc.VanAddresses {
								if y.Name == *connector.Address {
									addr = y
									break
								}
							}
							if addr == nil {
								t := time.Now()
								va := &VanAddressRecord{
									Base: Base{
										RecType:   attributeNames[VanAddress],
										Identity:  uuid.New().String(),
										StartTime: uint64(t.UnixNano()) / uint64(time.Microsecond),
									},
									Name:     *connector.Address,
									Protocol: *connector.Protocol,
								}
								fc.VanAddresses[va.Identity] = va
							}
						}
					}
					fc.connectorsToReconcile[connector.Identity] = connector.Identity
				}
			} else {
				if current.EndTime == 0 && connector.EndTime > 0 {
					current.EndTime = connector.EndTime
					if current.process != nil {
						if process, ok := fc.Processes[*current.process]; ok {
							process.connector = nil
						}
					}
				} else {
					if current.Parent == "" && connector.Parent != "" {
						current.Parent = connector.Parent
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
						var addr *VanAddressRecord
						for _, y := range fc.VanAddresses {
							if y.Name == *connector.Address {
								addr = y
								break
							}
						}
						if addr == nil {
							t := time.Now()
							va := &VanAddressRecord{
								Base: Base{
									RecType:   attributeNames[VanAddress],
									Identity:  uuid.New().String(),
									StartTime: uint64(t.UnixNano()) / uint64(time.Microsecond),
								},
								Name:     *connector.Address,
								Protocol: *connector.Protocol,
							}
							fc.VanAddresses[va.Identity] = va
						}
					}
				}
			}
		}
	case FlowRecord:
		if flow, ok := record.(FlowRecord); ok {
			if current, ok := fc.Flows[flow.Identity]; !ok {
				if flow.StartTime != 0 && flow.EndTime != 0 {
					if flow.Parent == "" {
						log.Printf("Incomplete flow record for identity: %s details %+v\n", flow.Identity, flow)
					}
				}
				if flow.StartTime != 0 {
					fc.Flows[flow.Identity] = &flow
					if flow.Parent != "" {
						flow.Protocol = fc.getFlowProtocol(&flow)
						flow.Place = fc.getFlowPlace(&flow)
						if listener, ok := fc.Listeners[flow.Parent]; ok {
							// TODO: workaround for gateway
							siteId := fc.getRecordSiteId(*listener)
							if fc.isGatewaySite(siteId) {
								fc.inferGatewayProcess(siteId, flow, false)
							}
						} else if connector, ok := fc.Connectors[flow.Parent]; ok {
							// TODO: workaround for gateway
							siteId := fc.getRecordSiteId(*connector)
							if fc.isGatewaySite(siteId) {
								fc.inferGatewayProcess(siteId, flow, true)
								fc.connectorsToReconcile[connector.Identity] = connector.Identity
							}
						}
					}
					if flow.CounterFlow != nil {
						fc.flowsToPairReconcile[flow.Identity] = *flow.CounterFlow
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
							fc.inferGatewayProcess(siteId, flow, false)
						}
					} else if connector, ok := fc.Connectors[flow.Parent]; ok {
						// TODO: workaround for gateway
						siteId := fc.getRecordSiteId(*connector)
						if fc.isGatewaySite(siteId) {
							fc.inferGatewayProcess(siteId, flow, true)
							fc.connectorsToReconcile[connector.Identity] = connector.Identity
						}
					}
				}
				if flow.Octets != nil {
					current.Octets = flow.Octets
				}
				if flow.OctetRate != nil {
					current.OctetRate = flow.OctetRate
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
					// this should trigger pairing
					current.CounterFlow = flow.CounterFlow
				}
				if flow.EndTime > 0 {
					current.EndTime = flow.EndTime
					if fc.getFlowPlace(current) == clientSide {
						if flowpair, ok := fc.FlowPairs["fp-"+current.Identity]; ok {
							flowpair.EndTime = current.EndTime
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
					fc.Processes[process.Identity] = &process
				}
				for _, pg := range fc.ProcessGroups {
					if *process.GroupName == *pg.Name {
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
			} else {
				if process.EndTime > 0 {
					current.EndTime = process.EndTime
				}
			}
		}
	case ProcessGroupRecord:
		if processGroup, ok := record.(ProcessGroupRecord); ok {
			if processGroup.StartTime > 0 && processGroup.EndTime == 0 {
				if _, ok := fc.ProcessGroups[processGroup.Identity]; !ok {
					fc.ProcessGroups[processGroup.Identity] = &processGroup
				}
			}
		}
	default:
		return fmt.Errorf("Unrecognized record type %T", record)
	}
	return nil
}

func (fc *FlowCollector) retrieve(request ApiRequest) (*string, error) {
	vars := mux.Vars(request.Request)
	url := request.Request.URL
	queryParams := getQueryParams(url)
	var retrieveError error = nil

	p := Payload{
		Results:        nil,
		QueryParams:    queryParams,
		Status:         "",
		Timestamp:      uint64(time.Now().UnixNano()) / uint64(time.Microsecond),
		Elapsed:        0,
		Count:          0,
		TimeRangeCount: 0,
		TotalCount:     0,
	}

	switch request.RecordType {
	case Site:
		switch request.HandlerName {
		case "list":
			sites := []SiteRecord{}
			for _, site := range fc.Sites {
				if filterRecord(*site, p.QueryParams.Filter) && site.Base.TimeRangeValid(queryParams) {
					fc.getSiteMetrics(site)
					sites = append(sites, *site)
				}
			}
			p.TotalCount = len(fc.Sites)
			retrieveError = sortAndSlice(sites, &p)
		case "item":
			if id, ok := vars["id"]; ok {
				if site, ok := fc.Sites[id]; ok {
					fc.getSiteMetrics(site)
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
							if filterRecord(*process, p.QueryParams.Filter) && process.Base.TimeRangeValid(queryParams) {
								fc.getProcessMetrics(process)
								processes = append(processes, *process)
							}
						}
					}
				}
			}
			retrieveError = sortAndSlice(processes, &p)
		case "routers":
			routers := []RouterRecord{}
			if id, ok := vars["id"]; ok {
				if site, ok := fc.Sites[id]; ok {
					for _, router := range fc.Routers {
						if router.Parent == site.Identity {
							p.TotalCount++
							if filterRecord(*router, p.QueryParams.Filter) && router.Base.TimeRangeValid(queryParams) {
								routers = append(routers, *router)
							}
						}
					}
				}
			}
			retrieveError = sortAndSlice(routers, &p)
		case "links":
			links := []LinkRecord{}
			if id, ok := vars["id"]; ok {
				if site, ok := fc.Sites[id]; ok {
					for _, link := range fc.Links {
						if fc.getRecordSiteId(*link) == site.Identity {
							p.TotalCount++
							if filterRecord(*link, p.QueryParams.Filter) && link.Base.TimeRangeValid(queryParams) {
								links = append(links, *link)
							}
						}
					}
				}
			}
			retrieveError = sortAndSlice(links, &p)
		case "hosts":
			hosts := []HostRecord{}
			if id, ok := vars["id"]; ok {
				if site, ok := fc.Sites[id]; ok {
					for _, host := range fc.Hosts {
						if host.Parent == site.Identity {
							p.TotalCount++
							if filterRecord(*host, p.QueryParams.Filter) && host.Base.TimeRangeValid(queryParams) {
								hosts = append(hosts, *host)
							}
						}
					}
				}
			}
			retrieveError = sortAndSlice(hosts, &p)
		}
	case Host:
		switch request.HandlerName {
		case "list":
			hosts := []HostRecord{}
			for _, host := range fc.Hosts {
				if filterRecord(*host, p.QueryParams.Filter) && host.Base.TimeRangeValid(queryParams) {
					hosts = append(hosts, *host)
				}
			}
			p.TotalCount = len(fc.Hosts)
			retrieveError = sortAndSlice(hosts, &p)
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
				if filterRecord(*router, p.QueryParams.Filter) && router.Base.TimeRangeValid(queryParams) {
					routers = append(routers, *router)
				}
			}
			p.TotalCount = len(fc.Routers)
			retrieveError = sortAndSlice(routers, &p)
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
									if filterRecord(*flow, p.QueryParams.Filter) && flow.Base.TimeRangeValid(queryParams) {
										flows = append(flows, *flow)
									}
								} else if l4Flow, ok := fc.Flows[flow.Parent]; ok {
									if l4Flow.Parent == connId {
										p.TotalCount++
										if filterRecord(*flow, p.QueryParams.Filter) && flow.Base.TimeRangeValid(queryParams) {
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
									if filterRecord(*flow, p.QueryParams.Filter) && flow.Base.TimeRangeValid(queryParams) {
										flows = append(flows, *flow)
									}
								} else if l4Flow, ok := fc.Flows[flow.Parent]; ok {
									if l4Flow.Parent == listenerId {
										p.TotalCount++
										if filterRecord(*flow, p.QueryParams.Filter) && flow.Base.TimeRangeValid(queryParams) {
											flows = append(flows, *flow)
										}
									}
								}
							}
						}
					}
				}
			}
			retrieveError = sortAndSlice(flows, &p)
		case "links":
			links := []LinkRecord{}
			if id, ok := vars["id"]; ok {
				if router, ok := fc.Routers[id]; ok {
					for _, link := range fc.Links {
						if link.Parent == router.Identity {
							p.TotalCount++
							if filterRecord(*link, p.QueryParams.Filter) && link.Base.TimeRangeValid(queryParams) {
								links = append(links, *link)
							}
						}
					}
				}
			}
			retrieveError = sortAndSlice(links, &p)
		case "listeners":
			listeners := []ListenerRecord{}
			if id, ok := vars["id"]; ok {
				if router, ok := fc.Routers[id]; ok {
					for _, listener := range fc.Listeners {
						if listener.Parent == router.Identity {
							p.TotalCount++
							if filterRecord(*listener, p.QueryParams.Filter) && listener.Base.TimeRangeValid(queryParams) {
								listeners = append(listeners, *listener)
							}
						}
					}
				}
			}
			retrieveError = sortAndSlice(listeners, &p)
		case "connectors":
			connectors := []ConnectorRecord{}
			if id, ok := vars["id"]; ok {
				if router, ok := fc.Routers[id]; ok {
					for _, connector := range fc.Connectors {
						if connector.Parent == router.Identity {
							p.TotalCount++
							if filterRecord(*connector, p.QueryParams.Filter) && connector.Base.TimeRangeValid(queryParams) {
								connectors = append(connectors, *connector)
							}
						}
					}
				}
			}
			retrieveError = sortAndSlice(connectors, &p)
		}
	case Link:
		switch request.HandlerName {
		case "list":
			links := []LinkRecord{}
			for _, link := range fc.Links {
				if filterRecord(*link, p.QueryParams.Filter) && link.Base.TimeRangeValid(queryParams) {
					links = append(links, *link)
				}
			}
			p.TotalCount = len(fc.Links)
			retrieveError = sortAndSlice(links, &p)
		case "item":
			if id, ok := vars["id"]; ok {
				if link, ok := fc.Links[id]; ok {
					p.Count = 1
					p.Results = link
				}
			}
		}
	case Listener:
		switch request.HandlerName {
		case "list":
			listeners := []ListenerRecord{}
			for _, listener := range fc.Listeners {
				if filterRecord(*listener, p.QueryParams.Filter) && listener.Base.TimeRangeValid(queryParams) {
					listeners = append(listeners, *listener)
				}
			}
			p.TotalCount = len(fc.Listeners)
			retrieveError = sortAndSlice(listeners, &p)
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
							if filterRecord(*flow, p.QueryParams.Filter) && flow.Base.TimeRangeValid(queryParams) {
								flows = append(flows, *flow)
							}
						} else if l4Flow, ok := fc.Flows[flow.Parent]; ok {
							if l4Flow.Parent == id {
								p.TotalCount++
								if filterRecord(*flow, p.QueryParams.Filter) && flow.Base.TimeRangeValid(queryParams) {
									flows = append(flows, *flow)
								}
							}
						}
					}
				}
			}
			retrieveError = sortAndSlice(flows, &p)
		}
	case Connector:
		switch request.HandlerName {
		case "list":
			connectors := []ConnectorRecord{}
			for _, connector := range fc.Connectors {
				if filterRecord(*connector, p.QueryParams.Filter) && connector.Base.TimeRangeValid(queryParams) {
					connectors = append(connectors, *connector)
				}
			}
			p.TotalCount = len(fc.Connectors)
			retrieveError = sortAndSlice(connectors, &p)
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
							if filterRecord(*flow, p.QueryParams.Filter) && flow.Base.TimeRangeValid(queryParams) {
								flows = append(flows, *flow)
							}
						} else if l4Flow, ok := fc.Flows[flow.Parent]; ok {
							if l4Flow.Parent == id {
								p.TotalCount++
								if filterRecord(*flow, p.QueryParams.Filter) && flow.Base.TimeRangeValid(queryParams) {
									flows = append(flows, *flow)
								}
							}
						}
					}
				}
			}
			retrieveError = sortAndSlice(flows, &p)
		case "process":
			if id, ok := vars["id"]; ok {
				if connector, ok := fc.Connectors[id]; ok {
					if connector.process != nil {
						if process, ok := fc.Processes[*connector.process]; ok {
							fc.getProcessMetrics(process)
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
				if filterRecord(*address, p.QueryParams.Filter) {
					fc.getAddressMetrics(address)
					addresses = append(addresses, *address)
				}
			}
			p.TotalCount = len(fc.VanAddresses)
			retrieveError = sortAndSlice(addresses, &p)
		case "item":
			if id, ok := vars["id"]; ok {
				if addr, ok := fc.VanAddresses[id]; ok {
					fc.getAddressMetrics(addr)
					p.Count = 1
					p.Results = addr
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
									if filterRecord(*flow, p.QueryParams.Filter) && flow.Base.TimeRangeValid(queryParams) {
										flows = append(flows, *flow)
									}
								} else if l4Flow, ok := fc.Flows[flow.Parent]; ok {
									if l4Flow.Parent == connId {
										p.TotalCount++
										if filterRecord(*flow, p.QueryParams.Filter) && flow.Base.TimeRangeValid(queryParams) {
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
									if filterRecord(*flow, p.QueryParams.Filter) && flow.Base.TimeRangeValid(queryParams) {
										flows = append(flows, *flow)
									}
								} else if l4Flow, ok := fc.Flows[flow.Parent]; ok {
									if l4Flow.Parent == listenerId {
										p.TotalCount++
										if filterRecord(*flow, p.QueryParams.Filter) && flow.Base.TimeRangeValid(queryParams) {
											flows = append(flows, *flow)
										}
									}
								}
							}
						}
					}
				}
			}
			retrieveError = sortAndSlice(flows, &p)
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
										if filterRecord(*flowpair, p.QueryParams.Filter) && flowpair.Base.TimeRangeValid(queryParams) {
											flowPairs = append(flowPairs, *flowpair)
										}
									}
								} else if l4Flow, ok := fc.Flows[flow.Parent]; ok {
									if l4Flow.Parent == listenerId {
										if flowpair, ok := fc.FlowPairs["fp-"+flowId]; ok {
											p.TotalCount++
											if filterRecord(*flowpair, p.QueryParams.Filter) && flowpair.Base.TimeRangeValid(queryParams) {
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
			retrieveError = sortAndSlice(flowPairs, &p)
		case "processes":
			processes := []ProcessRecord{}
			unique := make(map[string]*ProcessRecord)
			if id, ok := vars["id"]; ok {
				if vanaddr, ok := fc.VanAddresses[id]; ok {
					for _, connector := range fc.Connectors {
						if *connector.Address == vanaddr.Name && connector.process != nil {
							if process, ok := fc.Processes[*connector.process]; ok {
								if filterRecord(*process, p.QueryParams.Filter) && process.Base.TimeRangeValid(queryParams) {
									unique[process.Identity] = process
								}
							}
						}
					}
					for _, process := range unique {
						fc.getProcessMetrics(process)
						processes = append(processes, *process)
					}
				}
			}
			p.TotalCount = len(unique)
			retrieveError = sortAndSlice(processes, &p)
		case "listeners":
			listeners := []ListenerRecord{}
			if id, ok := vars["id"]; ok {
				if vanaddr, ok := fc.VanAddresses[id]; ok {
					for _, listener := range fc.Listeners {
						if *listener.Address == vanaddr.Name {
							p.TotalCount++
							if filterRecord(*listener, p.QueryParams.Filter) && listener.Base.TimeRangeValid(queryParams) {
								listeners = append(listeners, *listener)
							}
						}
					}
				}
			}
			retrieveError = sortAndSlice(listeners, &p)
		case "connectors":
			connectors := []ConnectorRecord{}
			if id, ok := vars["id"]; ok {
				if vanaddr, ok := fc.VanAddresses[id]; ok {
					for _, connector := range fc.Connectors {
						if *connector.Address == vanaddr.Name {
							p.TotalCount++
							if filterRecord(*connector, p.QueryParams.Filter) && connector.Base.TimeRangeValid(queryParams) {
								connectors = append(connectors, *connector)
							}
						}
					}
				}
			}
			retrieveError = sortAndSlice(connectors, &p)
		}
	case Process:
		switch request.HandlerName {
		case "list":
			processes := []ProcessRecord{}
			for _, process := range fc.Processes {
				if filterRecord(*process, p.QueryParams.Filter) && process.Base.TimeRangeValid(queryParams) {
					fc.getProcessMetrics(process)
					processes = append(processes, *process)
				}
			}
			p.TotalCount = len(fc.Processes)
			retrieveError = sortAndSlice(processes, &p)
		case "item":
			if id, ok := vars["id"]; ok {
				if process, ok := fc.Processes[id]; ok {
					fc.getProcessMetrics(process)
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
						if filterRecord(*flow, p.QueryParams.Filter) && flow.Base.TimeRangeValid(queryParams) {
							flows = append(flows, *flow)
						}
					}
				}
			}
			retrieveError = sortAndSlice(flows, &p)
		case "addresses":
			addresses := []VanAddressRecord{}
			if id, ok := vars["id"]; ok {
				if _, ok := fc.Processes[id]; ok {
					for _, connector := range fc.Connectors {
						if connector.process != nil && *connector.process == id {
							for _, address := range fc.VanAddresses {
								if *connector.Address == address.Name {
									if filterRecord(*address, p.QueryParams.Filter) {
										fc.getAddressMetrics(address)
										addresses = append(addresses, *address)
									}
								}
							}
						}
					}
				}
			}
			p.TotalCount = len(fc.VanAddresses)
			retrieveError = sortAndSlice(addresses, &p)
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
				p.TotalCount++
				if filterRecord(*processGroup, p.QueryParams.Filter) && processGroup.Base.TimeRangeValid(queryParams) {
					fc.getProcessGroupMetrics(processGroup)
					processGroups = append(processGroups, *processGroup)
				}
			}
			retrieveError = sortAndSlice(processGroups, &p)
		case "item":
			if id, ok := vars["id"]; ok {
				if processGroup, ok := fc.ProcessGroups[id]; ok {
					fc.getProcessGroupMetrics(processGroup)
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
							if filterRecord(*process, p.QueryParams.Filter) && process.Base.TimeRangeValid(queryParams) {
								fc.getProcessMetrics(process)
								processes = append(processes, *process)
							}
						}
					}
				}
			}
			retrieveError = sortAndSlice(processes, &p)
		}
	case Flow:
		switch request.HandlerName {
		case "list":
			flows := []FlowRecord{}
			for _, flow := range fc.Flows {
				if filterRecord(*flow, p.QueryParams.Filter) && flow.Base.TimeRangeValid(queryParams) {
					flows = append(flows, *flow)
				}
			}
			p.TotalCount = len(fc.Flows)
			retrieveError = sortAndSlice(flows, &p)
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
							fc.getProcessMetrics(process)
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
				if filterRecord(*flowPair, p.QueryParams.Filter) && flowPair.Base.TimeRangeValid(queryParams) {
					flowPairs = append(flowPairs, *flowPair)
				}
			}
			p.TotalCount = len(fc.FlowPairs)
			retrieveError = sortAndSlice(flowPairs, &p)
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
						if filterRecord(*aggregate, p.QueryParams.Filter) {
							fc.getFlowAggregateMetrics(Site, aggregate.Identity)
							aggregates = append(aggregates, *aggregate)
						}
					}
				}
			}
			retrieveError = sortAndSlice(aggregates, &p)
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
						if filterRecord(*aggregate, p.QueryParams.Filter) {
							fc.getFlowAggregateMetrics(ProcessGroup, aggregate.Identity)
							aggregates = append(aggregates, *aggregate)
						}
					}
				}
			}
			retrieveError = sortAndSlice(aggregates, &p)
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
						if filterRecord(*aggregate, p.QueryParams.Filter) {
							fc.getFlowAggregateMetrics(Process, aggregate.Identity)
							aggregates = append(aggregates, *aggregate)
						}
					}
				}
			}
			retrieveError = sortAndSlice(aggregates, &p)
		case "item":
			if id, ok := vars["id"]; ok {
				if flowAggregate, ok := fc.FlowAggregates[id]; ok {
					if flowAggregate.PairType == recordNames[Process] {
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
						fc.getFlowAggregateMetrics(Site, aggregate.Identity)
						aggregates = append(aggregates, *aggregate)
					}
				}
			}
			retrieveError = sortAndSlice(aggregates, &p)
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
						fc.getFlowAggregateMetrics(Process, aggregate.Identity)
						aggregates = append(aggregates, *aggregate)
					}
				}
			}
			retrieveError = sortAndSlice(aggregates, &p)
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
						fc.getFlowAggregateMetrics(ProcessGroup, aggregate.Identity)
						aggregates = append(aggregates, *aggregate)
					}
				}
			}
			retrieveError = sortAndSlice(aggregates, &p)
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
				if filterRecord(eventSource.EventSourceRecord, p.QueryParams.Filter) && eventSource.EventSourceRecord.Base.TimeRangeValid(queryParams) {
					eventSources = append(eventSources, eventSource.EventSourceRecord)
				}
			}
			p.TotalCount = len(fc.eventSources)
			retrieveError = sortAndSlice(eventSources, &p)
		case "item":
			if id, ok := vars["id"]; ok {
				if eventSource, ok := fc.eventSources[id]; ok {
					p.Count = 1
					p.Results = eventSource.EventSourceRecord
				}
			}
		}
	default:
		log.Println("Unrecognize record request", request.RecordType)
	}
	if retrieveError != nil {
		p.Status = retrieveError.Error()
	}
	p.Elapsed = uint64(time.Now().UnixNano())/uint64(time.Microsecond) - p.Timestamp
	data, err := json.MarshalIndent(p, "", " ")
	if err != nil {
		log.Println("Error marshalling results", err.Error())
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

func (fc *FlowCollector) reconcileRecords() error {
	for routerId, siteId := range fc.routersToSiteReconcile {
		// TODO: This is temporary workaround until gateway or CE site emits events
		parts := strings.Split(siteId, "_")
		if len(parts) > 1 {
			if parts[0] == "gateway" {
				if _, ok := fc.Sites[siteId]; !ok {
					name := parts[1]
					namespace := parts[0] + "-" + parts[1]
					fc.Sites[siteId] = &SiteRecord{
						Base: Base{
							RecType:   recordNames[Site],
							Identity:  siteId,
							StartTime: uint64(time.Now().UnixNano()) / uint64(time.Microsecond),
						},
						Name:      &name,
						NameSpace: &namespace,
					}
				}
			}
		}
		delete(fc.routersToSiteReconcile, routerId)
	}
	for _, flowId := range fc.flowsToProcessReconcile {
		if flow, ok := fc.Flows[flowId]; ok {
			if flow.SourceHost != nil {
				if connector, ok := fc.Connectors[flow.Parent]; ok {
					if connector.process != nil {
						flow.Process = connector.process
						if process, ok := fc.Processes[*flow.Process]; ok {
							flow.ProcessName = process.Name
						}
						delete(fc.flowsToProcessReconcile, flowId)
					}
				} else if _, ok := fc.Listeners[flow.Parent]; ok {
					siteId := fc.getRecordSiteId(*flow)
					for _, process := range fc.Processes {
						if siteId == process.Parent && process.SourceHost != nil {
							if *flow.SourceHost == *process.SourceHost {
								flow.Process = &process.Identity
								flow.ProcessName = process.Name
								delete(fc.flowsToProcessReconcile, flowId)
							}
						}
					}
				}
			} else if l4Flow, ok := fc.Flows[flow.Parent]; ok {
				if l4Flow.Process != nil {
					flow.Process = l4Flow.Process
					flow.ProcessName = l4Flow.ProcessName
					delete(fc.flowsToProcessReconcile, flowId)
				}
			}
		} else {
			delete(fc.flowsToProcessReconcile, flowId)
		}
	}
	for reverseId, forwardId := range fc.flowsToPairReconcile {
		if reverseFlow, ok := fc.Flows[reverseId]; ok {
			if forwardFlow, ok := fc.Flows[forwardId]; ok {
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
	for _, connId := range fc.connectorsToReconcile {
		if connector, ok := fc.Connectors[connId]; ok {
			if connector.EndTime > 0 {
				delete(fc.connectorsToReconcile, connId)
			} else if connector.DestHost != nil {
				siteId := fc.getRecordSiteId(*connector)
				// TODO: workaround for gateway
				// maybe the site should indicate it is a gateway
				parts := strings.Split(siteId, "_")
				gateway := false
				if len(parts) > 1 && parts[0] == "gateway" {
					gateway = true
				}
				for _, process := range fc.Processes {
					if siteId == process.Parent {
						if gateway {
							if process.SourceHost != nil && *process.SourceHost == *connector.DestHost {
								connector.process = &process.Identity
								process.connector = &connector.Identity
								log.Printf("Gateway connector %s/%s associated to process %s\n", connector.Identity, *connector.Address, *process.Name)
								delete(fc.connectorsToReconcile, connId)
							}
						} else if process.SourceHost != nil {
							if *connector.DestHost == *process.SourceHost {
								connector.process = &process.Identity
								process.connector = &connector.Identity
								log.Printf("Connector %s/%s associated to process %s\n", connector.Identity, *connector.Address, *process.Name)
								delete(fc.connectorsToReconcile, connId)
							}
						}
					}
				}
			}
		} else {
			delete(fc.connectorsToReconcile, connId)
		}
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
					}
					if destinationProcess, ok := fc.Processes[cfp.Identity]; ok {
						pfa.DestinationName = destinationProcess.Name
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

func (fc *FlowCollector) getSiteMetrics(site *SiteRecord) error {
	//TODO: active flows, flows since, flows in interval

	octetsSent := uint64(0)
	octetsSentRate := uint64(0)
	octetsReceived := uint64(0)
	octetsReceivedRate := uint64(0)
	for _, fp := range fc.FlowPairs {

		if site.Identity == fp.SourceSiteId {
			if fp.ForwardFlow.Octets != nil {
				octetsSent += *fp.ForwardFlow.Octets
			}
			if fp.ForwardFlow.OctetRate != nil {
				octetsSentRate = octetsSentRate + *fp.ForwardFlow.OctetRate
			}
			if fp.CounterFlow.Octets != nil {
				octetsReceived += *fp.CounterFlow.Octets
			}
			if fp.CounterFlow.OctetRate != nil {
				octetsReceivedRate = octetsReceivedRate + *fp.CounterFlow.OctetRate
			}
		} else if site.Identity == fp.DestinationSiteId {
			if fp.ForwardFlow.Octets != nil {
				octetsReceived += *fp.ForwardFlow.Octets
			}
			if fp.ForwardFlow.OctetRate != nil {
				octetsReceivedRate = octetsSentRate + *fp.ForwardFlow.OctetRate
			}
			if fp.CounterFlow.Octets != nil {
				octetsSent += *fp.CounterFlow.Octets
			}
			if fp.CounterFlow.OctetRate != nil {
				octetsSentRate = octetsReceivedRate + *fp.CounterFlow.OctetRate
			}
		}
	}
	site.OctetsSent = octetsSent
	site.OctetsSentRate = octetsSentRate
	site.OctetsReceived = octetsReceived
	site.OctetsReceivedRate = octetsReceivedRate

	return nil
}

func (fc *FlowCollector) getAddressMetrics(addr *VanAddressRecord) error {
	listenerCount := 0
	connectorCount := 0
	totalFlows := 0
	currentFlows := 0
	sourceOctets := uint64(0)
	destOctets := uint64(0)
	for listenerId, listener := range fc.Listeners {
		if *listener.Address == addr.Name {
			listenerCount++
			for _, flow := range fc.Flows {
				if flow.Parent == listenerId && *listener.Protocol == "tcp" && flow.StartTime != 0 {
					totalFlows++
					if flow.EndTime == 0 {
						currentFlows++
					}
					if flow.Octets != nil {
						sourceOctets += *flow.Octets
					}
				} else if l4Flow, ok := fc.Flows[flow.Parent]; ok {
					if l4Flow.Parent == listenerId && flow.StartTime != 0 {
						totalFlows++
						if flow.EndTime == 0 {
							currentFlows++
						}
						if flow.Octets != nil {
							sourceOctets += *flow.Octets
						}
					}
				}
			}
		}
	}
	for connectorId, connector := range fc.Connectors {
		if *connector.Address == addr.Name {
			connectorCount++
			for _, flow := range fc.Flows {
				if flow.Parent == connectorId && *connector.Protocol == "tcp" && flow.StartTime != 0 {
					totalFlows++
					if flow.EndTime == 0 {
						currentFlows++
					}
					if flow.Octets != nil {
						destOctets += *flow.Octets
					}
				} else if l4Flow, ok := fc.Flows[flow.Parent]; ok {
					if l4Flow.Parent == connectorId && flow.StartTime != 0 {
						totalFlows++
						if flow.EndTime == 0 {
							currentFlows++
						}
						if flow.Octets != nil {
							destOctets += *flow.Octets
						}
					}
				}
			}
		}
	}
	addr.ListenerCount = listenerCount
	addr.ConnectorCount = connectorCount
	addr.TotalFlows = totalFlows
	addr.CurrentFlows = currentFlows
	addr.SourceOctets = sourceOctets
	addr.DestOctets = destOctets

	return nil
}

func (fc *FlowCollector) getProcessGroupMetrics(pg *ProcessGroupRecord) error {
	octetsSent := uint64(0)
	octetsSentRate := uint64(0)
	octetsReceived := uint64(0)
	octetsReceivedRate := uint64(0)
	for _, fp := range fc.FlowPairs {
		if fp.ProcessGroupAggregateId != nil {
			parts := strings.Split(*fp.ProcessGroupAggregateId, "-to-")
			if len(parts) == 2 {
				if pg.Identity == parts[0] {
					if fp.ForwardFlow.Octets != nil {
						octetsSent += *fp.ForwardFlow.Octets
					}
					if fp.ForwardFlow.OctetRate != nil {
						octetsSentRate = octetsSentRate + *fp.ForwardFlow.OctetRate
					}
					if fp.CounterFlow.Octets != nil {
						octetsReceived += *fp.CounterFlow.Octets
					}
					if fp.CounterFlow.OctetRate != nil {
						octetsReceivedRate = octetsReceivedRate + *fp.CounterFlow.OctetRate
					}
				} else if pg.Identity == parts[1] {
					if fp.ForwardFlow.Octets != nil {
						octetsReceived += *fp.ForwardFlow.Octets
					}
					if fp.ForwardFlow.OctetRate != nil {
						octetsReceivedRate = octetsReceivedRate + *fp.ForwardFlow.OctetRate
					}
					if fp.CounterFlow.Octets != nil {
						octetsSent += *fp.CounterFlow.Octets
					}
					if fp.CounterFlow.OctetRate != nil {
						octetsSentRate = octetsSentRate + *fp.CounterFlow.OctetRate
					}
				}
			}
		}
	}
	pg.OctetsSent = octetsSent
	pg.OctetsSentRate = octetsSentRate
	pg.OctetsReceived = octetsReceived
	pg.OctetsReceivedRate = octetsReceivedRate

	return nil
}

func (fc *FlowCollector) getProcessMetrics(proc *ProcessRecord) error {
	//TODO: active flows, flows in time interval, flows since time

	octetsSent := uint64(0)
	octetsSentRate := uint64(0)
	octetsReceived := uint64(0)
	octetsReceivedRate := uint64(0)
	for _, fp := range fc.FlowPairs {
		if fp.ProcessAggregateId != nil {
			parts := strings.Split(*fp.ProcessAggregateId, "-to-")
			if len(parts) == 2 {
				if proc.Identity == parts[0] {
					if fp.ForwardFlow.Octets != nil {
						octetsSent += *fp.ForwardFlow.Octets
					}
					if fp.ForwardFlow.OctetRate != nil {
						octetsSentRate = octetsSentRate + *fp.ForwardFlow.OctetRate
					}
					if fp.CounterFlow.Octets != nil {
						octetsReceived += *fp.CounterFlow.Octets
					}
					if fp.CounterFlow.OctetRate != nil {
						octetsReceivedRate = octetsReceivedRate + *fp.CounterFlow.OctetRate
					}
				} else if proc.Identity == parts[1] {
					if fp.ForwardFlow.Octets != nil {
						octetsReceived += *fp.ForwardFlow.Octets
					}
					if fp.ForwardFlow.OctetRate != nil {
						octetsReceivedRate = octetsReceivedRate + *fp.ForwardFlow.OctetRate
					}
					if fp.CounterFlow.Octets != nil {
						octetsSent += *fp.CounterFlow.Octets
					}
					if fp.CounterFlow.OctetRate != nil {
						octetsSentRate = octetsSentRate + *fp.CounterFlow.OctetRate
					}
				}
			}
		}
	}
	proc.OctetsSent = octetsSent
	proc.OctetsSentRate = octetsSentRate
	proc.OctetsReceived = octetsReceived
	proc.OctetsReceivedRate = octetsReceivedRate

	return nil
}

func (fc *FlowCollector) getFlowAggregateMetrics(itemType int, identity string) (*FlowAggregateRecord, error) {
	if aggregate, ok := fc.FlowAggregates[identity]; ok {
		// todo determine way to prime latency calcs
		for _, flowPair := range fc.FlowPairs {
			if flowPair.ForwardFlow.Latency != nil && flowPair.CounterFlow.Latency != nil {
				aggregate.SourceMaxLatency = *flowPair.ForwardFlow.Latency
				aggregate.SourceMinLatency = *flowPair.ForwardFlow.Latency
				aggregate.DestinationMaxLatency = *flowPair.CounterFlow.Latency
				aggregate.DestinationMinLatency = *flowPair.CounterFlow.Latency
				break
			}
		}
		sumSourceLatency := uint64(0)
		sumDestinationLatency := uint64(0)
		aggregate.RecordCount = uint64(0)
		sourceOctets := uint64(0)
		sourceOctetRate := uint64(0)
		destinationOctets := uint64(0)
		destinationOctetRate := uint64(0)

		for _, flowPair := range fc.FlowPairs {
			found := false
			switch itemType {
			case Site:
				if aggregate.PairType == recordNames[Site] {
					if flowPair.SiteAggregateId != nil && *flowPair.SiteAggregateId == aggregate.Identity {
						found = true
					}
				}
			case ProcessGroup:
				if aggregate.PairType == recordNames[ProcessGroup] {
					if flowPair.ProcessGroupAggregateId != nil && *flowPair.ProcessGroupAggregateId == aggregate.Identity {
						found = true
					}
				}
			case Process:
				if aggregate.PairType == recordNames[Process] {
					if flowPair.ProcessAggregateId != nil && *flowPair.ProcessAggregateId == aggregate.Identity {
						found = true
					}
				}
			}
			if found {
				aggregate.RecordCount++
				if flowPair.ForwardFlow.Octets != nil {
					sourceOctets += *flowPair.ForwardFlow.Octets
				}
				if flowPair.ForwardFlow.OctetRate != nil {
					sourceOctetRate += *flowPair.ForwardFlow.OctetRate
				}
				if flowPair.ForwardFlow.Latency != nil {
					aggregate.SourceMinLatency = min(aggregate.SourceMinLatency, *flowPair.ForwardFlow.Latency)
					aggregate.SourceMaxLatency = max(aggregate.SourceMaxLatency, *flowPair.ForwardFlow.Latency)
					sumSourceLatency += *flowPair.ForwardFlow.Latency
				}
				if flowPair.CounterFlow.Octets != nil {
					destinationOctets += *flowPair.CounterFlow.Octets
				}
				if flowPair.CounterFlow.OctetRate != nil {
					destinationOctetRate += *flowPair.CounterFlow.OctetRate
				}
				if flowPair.CounterFlow.Latency != nil {
					aggregate.DestinationMinLatency = min(aggregate.DestinationMinLatency, *flowPair.CounterFlow.Latency)
					aggregate.DestinationMaxLatency = max(aggregate.DestinationMaxLatency, *flowPair.CounterFlow.Latency)
					sumDestinationLatency += *flowPair.CounterFlow.Latency
				}
			}
		}
		aggregate.SourceOctets = sourceOctets
		aggregate.SourceOctetRate = sourceOctetRate
		aggregate.DestinationOctets = destinationOctets
		aggregate.DestinationOctetRate = destinationOctetRate
		if aggregate.RecordCount > 0 {
			aggregate.SourceAverageLatency = sumSourceLatency / aggregate.RecordCount
			aggregate.DestinationAverageLatency = sumDestinationLatency / aggregate.RecordCount
		}
		return aggregate, nil
	}
	return nil, fmt.Errorf("Aggregate not found %s", identity)
}

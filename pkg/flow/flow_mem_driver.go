package flow

import (
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
)

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
			if _, ok := c.Connectors[flow.Parent]; ok {
				connector := c.Connectors[flow.Parent]
				if router, ok := c.Routers[connector.Parent]; ok {
					return router.Parent
				}
			}
			if _, ok := c.Listeners[flow.Parent]; ok {
				listener := c.Listeners[flow.Parent]
				if router, ok := c.Routers[listener.Parent]; ok {
					return router.Parent
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

func (fc *FlowCollector) updateRecord(record interface{}) error {
	switch record.(type) {
	case HeartbeatRecord:
		if heartbeat, ok := record.(HeartbeatRecord); ok {
			if eventsource, ok := fc.eventSources[heartbeat.Identity]; ok {
				eventsource.CurrentTime = heartbeat.Now
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
				}
				// to do router update handling
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
					if listener.Parent != "" {
						if rtr, ok := fc.Routers[listener.Parent]; ok {
							rtr.addListener(listener.Identity)
						}
					}
					if listener.Address != nil {
						var addr *VanAddressRecord
						for _, y := range fc.VanAddresses {
							if y.Name == *listener.Address {
								addr = y
							}
						}
						if addr == nil {
							t := time.Now()
							va := &VanAddressRecord{
								Base: Base{
									RecType:   attributeNames[VanAddress],
									Identity:  uuid.New().String(),
									StartTime: uint64(t.UnixNano()),
								},
								Name:           *listener.Address,
								ListenerCount:  1,
								listeners:      []string{listener.Identity},
								ConnectorCount: 0,
								connectors:     []string{},
							}
							fc.VanAddresses[va.Identity] = va
							listener.vanIdentity = &va.Identity
						} else {
							addr.addListener(listener.Identity)
							listener.vanIdentity = &addr.Identity
						}
					}
				}
			} else {
				if current.EndTime == 0 && listener.EndTime > 0 {
					current.EndTime = listener.EndTime
					if router, ok := fc.Routers[current.Parent]; ok {
						router.removeListener(current.Identity)
					}
					if addr, ok := fc.VanAddresses[*current.vanIdentity]; ok {
						addr.removeListener(current.Identity)
					}
				} else {
					if current.Parent == "" && listener.Parent != "" {
						current.Parent = listener.Parent
						if router, ok := fc.Routers[listener.Parent]; ok {
							router.addListener(current.Identity)
						}
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
							}
						}
						if addr == nil {
							t := time.Now()
							va := &VanAddressRecord{
								Base: Base{
									RecType:   attributeNames[VanAddress],
									Identity:  uuid.New().String(),
									StartTime: uint64(t.UnixNano()),
								},
								Name:           *listener.Address,
								ListenerCount:  1,
								listeners:      []string{listener.Identity},
								ConnectorCount: 0,
								connectors:     []string{},
							}
							fc.VanAddresses[va.Identity] = va
							listener.vanIdentity = &va.Identity
						} else {
							addr.addListener(listener.Identity)
							listener.vanIdentity = &addr.Identity
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
						if rtr, ok := fc.Routers[connector.Parent]; ok {
							rtr.addConnector(connector.Identity)
						}
						if connector.Address != nil {
							var addr *VanAddressRecord
							for _, y := range fc.VanAddresses {
								if y.Name == *connector.Address {
									addr = y
								}
							}
							if addr == nil {
								t := time.Now()
								va := &VanAddressRecord{
									Base: Base{
										RecType:   attributeNames[VanAddress],
										Identity:  uuid.New().String(),
										StartTime: uint64(t.UnixNano()),
									},
									Name:           *connector.Address,
									ListenerCount:  0,
									listeners:      []string{},
									ConnectorCount: 1,
									connectors:     []string{connector.Identity},
								}
								fc.VanAddresses[va.Identity] = va
								connector.vanIdentity = &va.Identity
							} else {
								addr.addConnector(connector.Identity)
								connector.vanIdentity = &addr.Identity
							}
						}
					}
					fc.connectorsToReconcile[connector.Identity] = connector.Identity
				}
			} else {
				if current.EndTime == 0 && connector.EndTime > 0 {
					current.EndTime = connector.EndTime
					if rtr, ok := fc.Routers[current.Parent]; ok {
						rtr.removeConnector(current.Identity)
					}
					if current.process != nil {
						if process, ok := fc.Processes[*current.process]; ok {
							process.connector = nil
						}
					}
					if current.vanIdentity != nil {
						if addr, ok := fc.VanAddresses[*current.vanIdentity]; ok {
							addr.removeConnector(current.Identity)
						}
					}
				} else {
					if current.Parent == "" && connector.Parent != "" {
						current.Parent = connector.Parent
						if rtr, ok := fc.Routers[connector.Parent]; ok {
							rtr.addConnector(connector.Identity)
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
						var addr *VanAddressRecord
						for _, y := range fc.VanAddresses {
							if y.Name == *connector.Address {
								addr = y
							}
						}
						if addr == nil {
							t := time.Now()
							va := &VanAddressRecord{
								Base: Base{
									RecType:   attributeNames[VanAddress],
									Identity:  uuid.New().String(),
									StartTime: uint64(t.UnixNano()),
								},
								Name:           *connector.Address,
								ListenerCount:  0,
								listeners:      []string{},
								ConnectorCount: 1,
								connectors:     []string{connector.Identity},
							}
							fc.VanAddresses[va.Identity] = va
							connector.vanIdentity = &va.Identity
						} else {
							addr.addConnector(connector.Identity)
							connector.vanIdentity = &addr.Identity
						}
					}
				}
			}
		}
	case FlowRecord:
		if flow, ok := record.(FlowRecord); ok {
			if current, ok := fc.Flows[flow.Identity]; !ok {
				if flow.StartTime > 0 && flow.EndTime == 0 {
					fc.Flows[flow.Identity] = &flow
					if listener, ok := fc.Listeners[flow.Parent]; ok {
						listener.addFlow(flow.Identity)
						for _, addr := range fc.VanAddresses {
							if addr.Name == *listener.Address {
								addr.flowBegin()
							}
						}
					} else if connector, ok := fc.Connectors[flow.Parent]; ok {
						connector.addFlow(flow.Identity)
						for _, addr := range fc.VanAddresses {
							if addr.Name == *connector.Address {
								addr.flowBegin()
							}
						}
					}
					if flow.CounterFlow != nil {
						if forwardFlow, ok := fc.Flows[*flow.CounterFlow]; ok {
							forwardFlow.CounterFlow = &flow.Identity
							sourceSite := fc.getRecordSiteId(*forwardFlow)
							destSite := fc.getRecordSiteId(flow)
							fp := &FlowPairRecord{
								Base: Base{
									RecType:   recordNames[FlowPair],
									Identity:  "fp-" + forwardFlow.Identity,
									StartTime: uint64(time.Now().UnixNano()),
								},
								SourceSiteId:      sourceSite,
								DestinationSiteId: destSite,
								ForwardFlow:       forwardFlow,
								CounterFlow:       &flow,
							}
							fc.FlowPairs["fp-"+forwardFlow.Identity] = fp
							fc.aggregatesToReconcile["fp-"+forwardFlow.Identity] = fp
						} else {
							fc.flowsToPairReconcile[flow.Identity] = *flow.CounterFlow
						}
					}
					fc.flowsToProcessReconcile[flow.Identity] = flow.Identity
				}
			} else {
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
				if current.SourceHost == nil && flow.SourceHost != nil {
					log.Println("TODO: Reconcile flow to site processes")
				}
				if flow.CounterFlow != nil && current.CounterFlow == nil {
					current.CounterFlow = flow.CounterFlow
					sourceSite := fc.getRecordSiteId(*current)
					destSite := fc.getRecordSiteId(flow)
					if forwardFlow, ok := fc.Flows[*flow.CounterFlow]; ok {
						forwardFlow.CounterFlow = &flow.Identity
						fp := &FlowPairRecord{
							Base: Base{
								RecType:   recordNames[FlowPair],
								Identity:  "fp-" + forwardFlow.Identity,
								StartTime: uint64(time.Now().UnixNano()),
							},
							SourceSiteId:      sourceSite,
							DestinationSiteId: destSite,
							ForwardFlow:       forwardFlow,
							CounterFlow:       &flow,
						}
						fc.FlowPairs["fp-"+forwardFlow.Identity] = fp
						fc.aggregatesToReconcile["fp-"+forwardFlow.Identity] = fp
					} else {
						fc.flowsToPairReconcile[flow.Identity] = *flow.CounterFlow
					}
				}
				if flow.EndTime > 0 {
					current.EndTime = flow.EndTime
					if listener, ok := fc.Listeners[current.Parent]; ok {
						listener.removeFlow(current.Identity)
						for _, addr := range fc.VanAddresses {
							if addr.Name == *listener.Address {
								addr.flowEnd()
							}
						}
						// listener is forward flow identity for pairs
						if flowpair, ok := fc.FlowPairs["fp-"+current.Identity]; ok {
							// should we remove from pair table or wait for an aging process
							flowpair.EndTime = current.EndTime
						}
					} else if connector, ok := fc.Connectors[current.Parent]; ok {
						connector.removeFlow(current.Identity)
						for _, addr := range fc.VanAddresses {
							if addr.Name == *connector.Address {
								addr.flowEnd()
							}
						}
					}
					if current.Process != nil {
						if process, ok := fc.Processes[*current.Process]; ok {
							process.flows = removeIdentity(process.flows, current.Identity)
						}
					}
				}
			}
		}
	case ProcessRecord:
		if process, ok := record.(ProcessRecord); ok {
			if current, ok := fc.Processes[process.Identity]; !ok {
				if process.StartTime > 0 && process.EndTime == 0 {
					fc.Processes[process.Identity] = &process
				}
				for _, pg := range fc.ProcessGroups {
					if *process.Group == *pg.Name {
						process.GroupIdentity = &pg.Identity
					}
				}
				if process.GroupIdentity == nil && process.Group != nil {
					pg := &ProcessGroupRecord{
						Base: Base{
							RecType:   recordNames[ProcessGroup],
							Identity:  uuid.New().String(),
							StartTime: uint64(time.Now().UnixNano()),
						},
						Name: process.Group,
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
	handlerName := mux.CurrentRoute(request.Request).GetName()
	url := request.Request.URL
	offset, err := strconv.Atoi(url.Query().Get("offset"))
	if err != nil {
		offset = -1
	}
	limit, err := strconv.Atoi(url.Query().Get("limit"))
	if err != nil {
		limit = -1
	}
	sortBy := url.Query().Get("sortBy")
	if sortBy == "" {
		// identity.asc is the default sort query
		sortBy = "identity.asc"
	}

	switch request.RecordType {
	case Site:
		switch handlerName {
		case "list":
			sites := []SiteRecord{}
			for _, site := range fc.Sites {
				fc.getSiteMetrics(site)
				sites = append(sites, *site)
			}
			snf, _ := sortAndFilter(sites, sortBy, offset, limit)
			return listToJSON(snf), nil
		case "item":
			if id, ok := vars["id"]; ok {
				if site, ok := fc.Sites[id]; ok {
					fc.getSiteMetrics(site)
					return itemToJSON(site), nil
				}
			}
		case "processes":
			processes := []ProcessRecord{}
			if id, ok := vars["id"]; ok {
				if site, ok := fc.Sites[id]; ok {
					for _, process := range fc.Processes {
						if process.Parent == site.Identity {
							processes = append(processes, *process)
						}
					}
				}
			}
			snf, _ := sortAndFilter(processes, sortBy, offset, limit)
			return listToJSON(snf), nil
		case "routers":
			routers := []RouterRecord{}
			if id, ok := vars["id"]; ok {
				if site, ok := fc.Sites[id]; ok {
					for _, router := range fc.Routers {
						if router.Parent == site.Identity {
							routers = append(routers, *router)
						}
					}
				}
			}
			snf, _ := sortAndFilter(routers, sortBy, offset, limit)
			return listToJSON(snf), nil
		case "links":
			links := []LinkRecord{}
			if id, ok := vars["id"]; ok {
				if site, ok := fc.Sites[id]; ok {
					for _, link := range fc.Links {
						if fc.getRecordSiteId(*link) == site.Identity {
							links = append(links, *link)
						}
					}
				}
			}
			snf, _ := sortAndFilter(links, sortBy, offset, limit)
			return listToJSON(snf), nil
		case "hosts":
			hosts := []HostRecord{}
			if id, ok := vars["id"]; ok {
				if site, ok := fc.Sites[id]; ok {
					for _, host := range fc.Hosts {
						if host.Parent == site.Identity {
							hosts = append(hosts, *host)
						}
					}
				}
			}
			snf, _ := sortAndFilter(hosts, sortBy, offset, limit)
			return listToJSON(snf), nil
		}
	case Host:
		switch handlerName {
		case "list":
			hosts := []HostRecord{}
			for _, host := range fc.Hosts {
				hosts = append(hosts, *host)
			}
			snf, _ := sortAndFilter(hosts, sortBy, offset, limit)
			return listToJSON(snf), nil
		case "item":
			if id, ok := vars["id"]; ok {
				if host, ok := fc.Hosts[id]; ok {
					return itemToJSON(host), nil
				}
			}
		}
	case Router:
		switch handlerName {
		case "list":
			routers := []RouterRecord{}
			for _, router := range fc.Routers {
				routers = append(routers, *router)
			}
			snf, _ := sortAndFilter(routers, sortBy, offset, limit)
			return listToJSON(snf), nil
		case "item":
			if id, ok := vars["id"]; ok {
				if router, ok := fc.Routers[id]; ok {
					return itemToJSON(router), nil
				}
			}
		case "flows":
			flows := []FlowRecord{}
			if id, ok := vars["id"]; ok {
				if router, ok := fc.Routers[id]; ok {
					for _, connId := range router.connectors {
						if connector, ok := fc.Connectors[connId]; ok {
							for _, flowId := range connector.flows {
								if flow, ok := fc.Flows[flowId]; ok {
									flows = append(flows, *flow)
								}
							}
						}
					}
					for _, listenerId := range router.listeners {
						if listener, ok := fc.Listeners[listenerId]; ok {
							for _, flowId := range listener.flows {
								if flow, ok := fc.Flows[flowId]; ok {
									flows = append(flows, *flow)
								}
							}
						}
					}
				}
			}
			snf, _ := sortAndFilter(flows, sortBy, offset, limit)
			return listToJSON(snf), nil
		case "links":
			links := []LinkRecord{}
			if id, ok := vars["id"]; ok {
				if router, ok := fc.Routers[id]; ok {
					for _, link := range fc.Links {
						if link.Parent == router.Identity {
							links = append(links, *link)
						}
					}
				}
			}
			snf, _ := sortAndFilter(links, sortBy, offset, limit)
			return listToJSON(snf), nil
		case "listeners":
			listeners := []ListenerRecord{}
			if id, ok := vars["id"]; ok {
				if router, ok := fc.Routers[id]; ok {
					for _, listener := range fc.Listeners {
						if listener.Parent == router.Identity {
							listeners = append(listeners, *listener)
						}
					}
				}
			}
			snf, _ := sortAndFilter(listeners, sortBy, offset, limit)
			return listToJSON(snf), nil
		case "connectors":
			connectors := []ConnectorRecord{}
			if id, ok := vars["id"]; ok {
				if router, ok := fc.Routers[id]; ok {
					for _, connector := range fc.Connectors {
						if connector.Parent == router.Identity {
							connectors = append(connectors, *connector)
						}
					}
				}
			}
			snf, _ := sortAndFilter(connectors, sortBy, offset, limit)
			return listToJSON(snf), nil
		}
	case Link:
		switch handlerName {
		case "list":
			links := []LinkRecord{}
			for _, link := range fc.Links {
				links = append(links, *link)
			}
			snf, _ := sortAndFilter(links, sortBy, offset, limit)
			return listToJSON(snf), nil
		case "item":
			if id, ok := vars["id"]; ok {
				if link, ok := fc.Links[id]; ok {
					return itemToJSON(link), nil
				}
			}
		}
	case Listener:
		switch handlerName {
		case "list":
			listeners := []ListenerRecord{}
			for _, listener := range fc.Listeners {
				listeners = append(listeners, *listener)
			}
			snf, _ := sortAndFilter(listeners, sortBy, offset, limit)
			return listToJSON(snf), nil
		case "item":
			if id, ok := vars["id"]; ok {
				if listener, ok := fc.Listeners[id]; ok {
					return itemToJSON(listener), nil
				}
			}
		case "flows":
			flows := []FlowRecord{}
			if id, ok := vars["id"]; ok {
				if listener, ok := fc.Listeners[id]; ok {
					for _, flowId := range listener.flows {
						if flow, ok := fc.Flows[flowId]; ok {
							flows = append(flows, *flow)
						}
					}
				}
			}
			snf, _ := sortAndFilter(flows, sortBy, offset, limit)
			return listToJSON(snf), nil
		}
	case Connector:
		switch handlerName {
		case "list":
			connectors := []ConnectorRecord{}
			for _, connector := range fc.Connectors {
				connectors = append(connectors, *connector)
			}
			snf, _ := sortAndFilter(connectors, sortBy, offset, limit)
			return listToJSON(snf), nil
		case "item":
			if id, ok := vars["id"]; ok {
				if connector, ok := fc.Connectors[id]; ok {
					return itemToJSON(connector), nil
				}
			}
		case "flows":
			flows := []FlowRecord{}
			if id, ok := vars["id"]; ok {
				if connector, ok := fc.Connectors[id]; ok {
					for _, flowId := range connector.flows {
						if flow, ok := fc.Flows[flowId]; ok {
							flows = append(flows, *flow)
						}
					}
				}
			}
			snf, _ := sortAndFilter(flows, sortBy, offset, limit)
			return listToJSON(snf), nil
		case "process":
			if id, ok := vars["id"]; ok {
				if connector, ok := fc.Connectors[id]; ok {
					if connector.process != nil {
						if process, ok := fc.Processes[*connector.process]; ok {
							return itemToJSON(*process), nil
						}
					}
				}
			}
		}
	case Address:
		switch handlerName {
		case "list":
			addresses := []VanAddressRecord{}
			for _, address := range fc.VanAddresses {
				addresses = append(addresses, *address)
			}
			snf, _ := sortAndFilter(addresses, sortBy, offset, limit)
			return listToJSON(snf), nil
		case "item":
			if id, ok := vars["id"]; ok {
				if addr, ok := fc.VanAddresses[id]; ok {
					return itemToJSON(addr), nil
				}
			}
		case "flows":
			flows := []FlowRecord{}
			if id, ok := vars["id"]; ok {
				if vanaddr, ok := fc.VanAddresses[id]; ok {
					for _, connId := range vanaddr.connectors {
						if connector, ok := fc.Connectors[connId]; ok {
							for _, flowId := range connector.flows {
								if flow, ok := fc.Flows[flowId]; ok {
									flows = append(flows, *flow)
								}
							}
						}
					}
					for _, listenerId := range vanaddr.listeners {
						if listener, ok := fc.Listeners[listenerId]; ok {
							for _, flowId := range listener.flows {
								if flow, ok := fc.Flows[flowId]; ok {
									flows = append(flows, *flow)
								}
							}
						}
					}
				}
			}
			snf, _ := sortAndFilter(flows, sortBy, offset, limit)
			return listToJSON(snf), nil
		case "flowpairs":
			flowPairs := []FlowPairRecord{}
			if id, ok := vars["id"]; ok {
				if vanaddr, ok := fc.VanAddresses[id]; ok {
					// forward flow for a flow pair is indexed by listener flow id
					for _, listenerId := range vanaddr.listeners {
						if listener, ok := fc.Listeners[listenerId]; ok {
							for _, flowId := range listener.flows {
								if flowpair, ok := fc.FlowPairs["fp-"+flowId]; ok {
									flowPairs = append(flowPairs, *flowpair)
								}
							}
						}
					}
				}
			}
			snf, _ := sortAndFilter(flowPairs, sortBy, offset, limit)
			return listToJSON(snf), nil
		case "processes":
			processes := []ProcessRecord{}
			unique := make(map[string]*ProcessRecord)
			if id, ok := vars["id"]; ok {
				if vanaddr, ok := fc.VanAddresses[id]; ok {
					for _, connId := range vanaddr.connectors {
						if connector, ok := fc.Connectors[connId]; ok {
							if connector.process != nil {
								if process, ok := fc.Processes[*connector.process]; ok {
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
			snf, _ := sortAndFilter(processes, sortBy, offset, limit)
			return listToJSON(snf), nil
		case "listeners":
			listeners := []ListenerRecord{}
			if id, ok := vars["id"]; ok {
				if vanaddr, ok := fc.VanAddresses[id]; ok {
					for _, listenerId := range vanaddr.listeners {
						if listener, ok := fc.Listeners[listenerId]; ok {
							listeners = append(listeners, *listener)
						}
					}
				}
			}
			snf, _ := sortAndFilter(listeners, sortBy, offset, limit)
			return listToJSON(snf), nil
		case "connectors":
			connectors := []ConnectorRecord{}
			if id, ok := vars["id"]; ok {
				if vanaddr, ok := fc.VanAddresses[id]; ok {
					for _, connId := range vanaddr.connectors {
						if connector, ok := fc.Connectors[connId]; ok {
							connectors = append(connectors, *connector)
						}
					}
				}
			}
			snf, _ := sortAndFilter(connectors, sortBy, offset, limit)
			return listToJSON(snf), nil
		}
	case Process:
		switch handlerName {
		case "list":
			processes := []ProcessRecord{}
			for _, process := range fc.Processes {
				fc.getProcessMetrics(process)
				processes = append(processes, *process)
			}
			snf, _ := sortAndFilter(processes, sortBy, offset, limit)
			return listToJSON(snf), nil
		case "item":
			if id, ok := vars["id"]; ok {
				if process, ok := fc.Processes[id]; ok {
					fc.getProcessMetrics(process)
					return itemToJSON(process), nil
				}
			}
		case "flows":
			flows := []FlowRecord{}
			if id, ok := vars["id"]; ok {
				if process, ok := fc.Processes[id]; ok {
					for _, flowId := range process.flows {
						if flow, ok := fc.Flows[flowId]; ok {
							flows = append(flows, *flow)
						}
					}
				}
			}
			snf, _ := sortAndFilter(flows, sortBy, offset, limit)
			return listToJSON(snf), nil
		case "connector":
			if id, ok := vars["id"]; ok {
				if process, ok := fc.Processes[id]; ok {
					if process.connector != nil {
						if connector, ok := fc.Connectors[*process.connector]; ok {
							return itemToJSON(*connector), nil
						}
					}
				}
			}
		}
	case ProcessGroup:
		switch handlerName {
		case "list":
			processGroups := []ProcessGroupRecord{}
			for _, processGroup := range fc.ProcessGroups {
				fc.getProcessGroupMetrics(processGroup)
				processGroups = append(processGroups, *processGroup)
			}
			snf, _ := sortAndFilter(processGroups, sortBy, offset, limit)
			return listToJSON(snf), nil
		case "item":
			if id, ok := vars["id"]; ok {
				if processGroup, ok := fc.ProcessGroups[id]; ok {
					fc.getProcessGroupMetrics(processGroup)
					return itemToJSON(processGroup), nil
				}
			}
		case "processes":
			processes := []ProcessRecord{}
			if id, ok := vars["id"]; ok {
				if processGroup, ok := fc.ProcessGroups[id]; ok {
					for _, process := range fc.Processes {
						if *process.GroupIdentity == processGroup.Identity {
							processes = append(processes, *process)
						}
					}
				}
			}
			snf, _ := sortAndFilter(processes, sortBy, offset, limit)
			return listToJSON(snf), nil
		}
	case Flow:
		switch handlerName {
		case "list":
			flows := []FlowRecord{}
			for _, flow := range fc.Flows {
				flows = append(flows, *flow)
			}
			snf, _ := sortAndFilter(flows, sortBy, offset, limit)
			return listToJSON(snf), nil
		case "item":
			if id, ok := vars["id"]; ok {
				if flow, ok := fc.Flows[id]; ok {
					return itemToJSON(flow), nil
				}
			}
		case "process":
			if id, ok := vars["id"]; ok {
				if flow, ok := fc.Flows[id]; ok {
					if flow.Process != nil {
						if process, ok := fc.Processes[*flow.Process]; ok {
							return itemToJSON(*process), nil
						}
					}
				}
			}
		}
	case FlowPair:
		switch handlerName {
		case "list":
			flowPairs := []FlowPairRecord{}
			for _, flowPair := range fc.FlowPairs {
				flowPairs = append(flowPairs, *flowPair)
			}
			snf, _ := sortAndFilter(flowPairs, sortBy, offset, limit)
			return listToJSON(snf), nil
		case "item":
			if id, ok := vars["id"]; ok {
				if flowPair, ok := fc.FlowPairs[id]; ok {
					return itemToJSON(flowPair), nil
				}
			}
		}
	case SitePair:
		sourceId := url.Query().Get("sourceId")
		destinationId := url.Query().Get("destinationId")
		switch handlerName {
		case "list":
			aggregates := []FlowAggregateRecord{}
			for _, aggregate := range fc.FlowAggregates {
				if aggregate.PairType == recordNames[Site] {
					if sourceId == "" && destinationId == "" {
						fc.getFlowAggregateMetrics(Site, aggregate.Identity)
						aggregates = append(aggregates, *aggregate)
					} else if sourceId == *aggregate.SourceId {
						fc.getFlowAggregateMetrics(Site, aggregate.Identity)
						aggregates = append(aggregates, *aggregate)
					} else if destinationId == *aggregate.DestinationId {
						fc.getFlowAggregateMetrics(Site, aggregate.Identity)
						aggregates = append(aggregates, *aggregate)
					}
				}
			}
			snf, _ := sortAndFilter(aggregates, sortBy, offset, limit)
			return listToJSON(snf), nil
		case "item":
			if id, ok := vars["id"]; ok {
				if flowAggregate, ok := fc.FlowAggregates[id]; ok {
					if flowAggregate.PairType == recordNames[Site] {
						return itemToJSON(flowAggregate), nil
					}
				}
			}
		}
	case ProcessGroupPair:
		sourceId := url.Query().Get("sourceId")
		destinationId := url.Query().Get("destinationId")
		switch handlerName {
		case "list":
			aggregates := []FlowAggregateRecord{}
			for _, aggregate := range fc.FlowAggregates {
				if aggregate.PairType == recordNames[ProcessGroup] {
					if sourceId == "" && destinationId == "" {
						fc.getFlowAggregateMetrics(ProcessGroup, aggregate.Identity)
						aggregates = append(aggregates, *aggregate)
					} else if sourceId == *aggregate.SourceId {
						fc.getFlowAggregateMetrics(ProcessGroup, aggregate.Identity)
						aggregates = append(aggregates, *aggregate)
					} else if destinationId == *aggregate.DestinationId {
						fc.getFlowAggregateMetrics(ProcessGroup, aggregate.Identity)
						aggregates = append(aggregates, *aggregate)
					}
				}
			}
			snf, _ := sortAndFilter(aggregates, sortBy, offset, limit)
			return listToJSON(snf), nil
		case "item":
			if id, ok := vars["id"]; ok {
				if flowAggregate, ok := fc.FlowAggregates[id]; ok {
					if flowAggregate.PairType == recordNames[ProcessGroup] {
						return itemToJSON(flowAggregate), nil
					}
				}
			}
		}
	case ProcessPair:
		sourceId := url.Query().Get("sourceId")
		destinationId := url.Query().Get("destinationId")
		switch handlerName {
		case "list":
			aggregates := []FlowAggregateRecord{}
			for _, aggregate := range fc.FlowAggregates {
				if aggregate.PairType == recordNames[Process] {
					if sourceId == "" && destinationId == "" {
						fc.getFlowAggregateMetrics(Process, aggregate.Identity)
						aggregates = append(aggregates, *aggregate)
					} else if sourceId == *aggregate.SourceId {
						fc.getFlowAggregateMetrics(Process, aggregate.Identity)
						aggregates = append(aggregates, *aggregate)
					} else if destinationId == *aggregate.DestinationId {
						fc.getFlowAggregateMetrics(Process, aggregate.Identity)
						aggregates = append(aggregates, *aggregate)
					}
				}
			}
			snf, _ := sortAndFilter(aggregates, sortBy, offset, limit)
			return listToJSON(snf), nil
		case "item":
			if id, ok := vars["id"]; ok {
				if flowAggregate, ok := fc.FlowAggregates[id]; ok {
					if flowAggregate.PairType == recordNames[Process] {
						return itemToJSON(flowAggregate), nil
					}
				}
			}
		}
	case FlowAggregate:
		sourceId := url.Query().Get("sourceId")
		destinationId := url.Query().Get("destinationId")
		switch handlerName {
		case "sitepair-list":
			aggregates := []FlowAggregateRecord{}
			for _, aggregate := range fc.FlowAggregates {
				if aggregate.PairType == recordNames[Site] {
					if sourceId == "" && destinationId == "" {
						fc.getFlowAggregateMetrics(Site, aggregate.Identity)
						aggregates = append(aggregates, *aggregate)
					} else if sourceId == *aggregate.SourceId {
						fc.getFlowAggregateMetrics(Site, aggregate.Identity)
						aggregates = append(aggregates, *aggregate)
					} else if destinationId == *aggregate.DestinationId {
						fc.getFlowAggregateMetrics(Site, aggregate.Identity)
						aggregates = append(aggregates, *aggregate)
					}
				}
			}
			snf, _ := sortAndFilter(aggregates, sortBy, offset, limit)
			return listToJSON(snf), nil
		case "sitepair-item":
			if id, ok := vars["id"]; ok {
				if flowAggregate, ok := fc.FlowAggregates[id]; ok {
					if flowAggregate.PairType == recordNames[Site] {
						return itemToJSON(flowAggregate), nil
					}
				}
			}
		case "processpair-list":
			aggregates := []FlowAggregateRecord{}
			for _, aggregate := range fc.FlowAggregates {
				if aggregate.PairType == recordNames[Process] {
					if sourceId == "" && destinationId == "" {
						fc.getFlowAggregateMetrics(Process, aggregate.Identity)
						aggregates = append(aggregates, *aggregate)
					} else if sourceId == *aggregate.SourceId {
						fc.getFlowAggregateMetrics(Process, aggregate.Identity)
						aggregates = append(aggregates, *aggregate)
					} else if destinationId == *aggregate.DestinationId {
						fc.getFlowAggregateMetrics(Process, aggregate.Identity)
						aggregates = append(aggregates, *aggregate)
					}
				}
			}
			snf, _ := sortAndFilter(aggregates, sortBy, offset, limit)
			return listToJSON(snf), nil
		case "processpair-item":
			if id, ok := vars["id"]; ok {
				if flowAggregate, ok := fc.FlowAggregates[id]; ok {
					if flowAggregate.PairType == recordNames[Process] {
						return itemToJSON(flowAggregate), nil
					}
				}
			}
		case "processgrouppair-list":
			aggregates := []FlowAggregateRecord{}
			for _, aggregate := range fc.FlowAggregates {
				if aggregate.PairType == recordNames[ProcessGroup] {
					if sourceId == "" && destinationId == "" {
						fc.getFlowAggregateMetrics(ProcessGroup, aggregate.Identity)
						aggregates = append(aggregates, *aggregate)
					} else if sourceId == *aggregate.SourceId {
						fc.getFlowAggregateMetrics(ProcessGroup, aggregate.Identity)
						aggregates = append(aggregates, *aggregate)
					} else if destinationId == *aggregate.DestinationId {
						fc.getFlowAggregateMetrics(ProcessGroup, aggregate.Identity)
						aggregates = append(aggregates, *aggregate)
					}
				}
			}
			snf, _ := sortAndFilter(aggregates, sortBy, offset, limit)
			return listToJSON(snf), nil
		case "processgrouppair-item":
			if id, ok := vars["id"]; ok {
				if flowAggregate, ok := fc.FlowAggregates[id]; ok {
					if flowAggregate.PairType == recordNames[ProcessGroup] {
						return itemToJSON(flowAggregate), nil
					}
				}
			}
		}
	case EventSource:
		switch handlerName {
		case "list":
			eventSources := []eventSource{}
			for _, eventSource := range fc.eventSources {
				eventSources = append(eventSources, *eventSource)
			}
			snf, _ := sortAndFilter(eventSources, sortBy, offset, limit)
			return listToJSON(snf), nil
		case "item":
			if id, ok := vars["id"]; ok {
				if eventSource, ok := fc.eventSources[id]; ok {
					return itemToJSON(eventSource), nil
				}
			}
		}
	default:
		log.Println("Unrecognize record request", request.RecordType)
	}
	return nil, nil
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
	for _, flowId := range fc.flowsToProcessReconcile {
		if flow, ok := fc.Flows[flowId]; ok {
			if flow.SourceHost != nil {
				if connector, ok := fc.Connectors[flow.Parent]; ok {
					if connector.process != nil {
						flow.Process = connector.process
						if process, ok := fc.Processes[*flow.Process]; ok {
							process.flows = addIdentity(process.flows, flow.Identity)
						}
						delete(fc.flowsToProcessReconcile, flowId)
					}
				} else if _, ok := fc.Listeners[flow.Parent]; ok {
					siteId := fc.getRecordSiteId(*flow)
					for _, process := range fc.Processes {
						if siteId == process.Parent && process.SourceHost != nil {
							if *flow.SourceHost == *process.SourceHost {
								flow.Process = &process.Identity
								process.flows = addIdentity(process.flows, flow.Identity)
								delete(fc.flowsToProcessReconcile, flowId)
							}
						}
					}
				}
			}
		} else {
			delete(fc.flowsToProcessReconcile, flowId)
		}
	}
	for reverseId, forwardId := range fc.flowsToPairReconcile {
		if reverseFlow, ok := fc.Flows[reverseId]; !ok {
			delete(fc.flowsToPairReconcile, reverseId)
		} else {
			if forwardFlow, ok := fc.Flows[forwardId]; ok {
				forwardFlow.CounterFlow = &reverseFlow.Identity
				sourceSite := fc.getRecordSiteId(*forwardFlow)
				destSite := fc.getRecordSiteId(*reverseFlow)
				fp := &FlowPairRecord{
					Base: Base{
						RecType:   recordNames[FlowPair],
						Identity:  "fp-" + forwardFlow.Identity,
						StartTime: uint64(time.Now().UnixNano()),
					},
					SourceSiteId:      sourceSite,
					DestinationSiteId: destSite,
					ForwardFlow:       forwardFlow,
					CounterFlow:       reverseFlow,
				}
				fc.FlowPairs["fp-"+forwardFlow.Identity] = fp
				fc.aggregatesToReconcile["fp-"+forwardFlow.Identity] = fp
				delete(fc.flowsToPairReconcile, reverseId)
			}
		}
	}
	for _, connId := range fc.connectorsToReconcile {
		if connector, ok := fc.Connectors[connId]; ok {
			if connector.EndTime > 0 {
				delete(fc.connectorsToReconcile, connId)
			} else if connector.DestHost != nil {
				siteId := fc.getRecordSiteId(*connector)
				for _, process := range fc.Processes {
					if siteId == process.Parent && process.SourceHost != nil {
						if *connector.DestHost == *process.SourceHost {
							connector.process = &process.Identity
							process.connector = &connector.Identity
							delete(fc.connectorsToReconcile, connId)
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
							StartTime: uint64(time.Now().UnixNano()),
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
							StartTime: uint64(time.Now().UnixNano()),
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
							StartTime: uint64(time.Now().UnixNano()),
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

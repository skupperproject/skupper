// Package view implements a mapping layer between vanflow records and the
// collector api.
package views

import (
	"sort"
	"strings"

	"github.com/skupperproject/skupper/cmd/network-observer/internal/api"
	"github.com/skupperproject/skupper/cmd/network-observer/internal/collector"
	"github.com/skupperproject/skupper/pkg/vanflow"
	"github.com/skupperproject/skupper/pkg/vanflow/store"
)

const unknownStr = "unknown"

func NewSitePairSliceProvider(graph collector.Graph) func(entries []store.Entry) []api.FlowAggregateRecord {
	provider := NewSitePairProvider(graph)
	return func(entries []store.Entry) []api.FlowAggregateRecord {
		results := make([]api.FlowAggregateRecord, 0, len(entries))
		for _, e := range entries {
			record, ok := e.Record.(collector.SitePairRecord)
			if !ok {
				continue
			}
			results = append(results, provider(record))
		}
		return results
	}
}
func NewSitePairProvider(graph collector.Graph) func(collector.SitePairRecord) api.FlowAggregateRecord {
	return func(record collector.SitePairRecord) api.FlowAggregateRecord {
		out := defaultFlowAggregate(record.ID)
		out.StartTime = uint64(record.Start.UnixMicro())
		out.PairType = api.SITE
		out.SourceId = record.Source
		out.DestinationId = record.Dest
		out.Protocol = record.Protocol

		if proc, ok := graph.Site(record.Source).GetRecord(); ok {
			setOpt(&out.SourceName, proc.Name)
		}
		if proc, ok := graph.Site(record.Dest).GetRecord(); ok {
			setOpt(&out.DestinationName, proc.Name)
		}
		return out
	}
}
func NewProcessPairSliceProvider(graph collector.Graph) func(entries []store.Entry) []api.FlowAggregateRecord {
	provider := NewProcessPairProvider(graph)
	return func(entries []store.Entry) []api.FlowAggregateRecord {
		results := make([]api.FlowAggregateRecord, 0, len(entries))
		for _, e := range entries {
			record, ok := e.Record.(collector.ProcPairRecord)
			if !ok {
				continue
			}
			results = append(results, provider(record))
		}
		return results
	}
}
func NewProcessPairProvider(graph collector.Graph) func(collector.ProcPairRecord) api.FlowAggregateRecord {
	return func(record collector.ProcPairRecord) api.FlowAggregateRecord {
		out := defaultFlowAggregate(record.ID)
		out.StartTime = uint64(record.Start.UnixMicro())
		out.PairType = api.PROCESS
		out.SourceId = record.Source
		out.DestinationId = record.Dest
		out.Protocol = record.Protocol

		if proc, ok := graph.Process(record.Source).GetRecord(); ok {
			setOpt(&out.SourceName, proc.Name)
		}
		if proc, ok := graph.Process(record.Dest).GetRecord(); ok {
			setOpt(&out.DestinationName, proc.Name)
		}
		if site, ok := graph.Process(record.Source).Parent().GetRecord(); ok {
			out.SourceSiteId = &site.ID
			out.SourceSiteName = site.Name
		}
		if site, ok := graph.Process(record.Dest).Parent().GetRecord(); ok {
			out.DestinationSiteId = &site.ID
			out.DestinationSiteName = site.Name
		}
		return out
	}
}
func NewProcessGroupPairSliceProvider() func(entries []store.Entry) []api.FlowAggregateRecord {
	provider := NewProcessGroupPairProvider()
	return func(entries []store.Entry) []api.FlowAggregateRecord {
		results := make([]api.FlowAggregateRecord, 0, len(entries))
		for _, e := range entries {
			record, ok := e.Record.(collector.ProcGroupPairRecord)
			if !ok {
				continue
			}
			results = append(results, provider(record))
		}
		return results
	}
}
func NewProcessGroupPairProvider() func(collector.ProcGroupPairRecord) api.FlowAggregateRecord {
	return func(record collector.ProcGroupPairRecord) api.FlowAggregateRecord {
		out := defaultFlowAggregate(record.ID)
		out.StartTime = uint64(record.Start.UnixMicro())
		out.PairType = api.PROCESSGROUP
		out.SourceId = record.Source
		out.SourceName = record.SourceName
		out.DestinationId = record.Dest
		out.DestinationName = record.DestName
		out.Protocol = record.Protocol
		return out
	}
}

func defaultFlowAggregate(id string) api.FlowAggregateRecord {
	return api.FlowAggregateRecord{
		Identity: id,
		Protocol: unknownStr,
	}
}
func NewListenerSliceProvider(graph collector.Graph) func(entries []store.Entry) []api.ListenerRecord {
	provider := NewListenerProvider(graph)
	return func(entries []store.Entry) []api.ListenerRecord {
		results := make([]api.ListenerRecord, 0, len(entries))
		for _, e := range entries {
			record, ok := e.Record.(vanflow.ListenerRecord)
			if !ok {
				continue
			}
			results = append(results, provider(record))
		}
		return results
	}
}

func NewListenerProvider(graph collector.Graph) func(vanflow.ListenerRecord) api.ListenerRecord {
	return func(record vanflow.ListenerRecord) api.ListenerRecord {
		out := defaultListener(record.ID)
		out.StartTime, out.EndTime = vanflowTimes(record.BaseRecord)
		setOpt(&out.Name, record.Name)
		setOpt(&out.Parent, record.Parent)
		setOpt(&out.Protocol, record.Protocol)
		setOpt(&out.DestHost, record.DestHost)
		setOpt(&out.DestPort, record.DestPort)
		setOpt(&out.Address, record.Address)

		node := graph.Listener(record.ID)
		if addressID := node.Address().ID(); addressID != "" {
			out.AddressId = &addressID
		}
		if site, ok := node.Parent().Parent().GetRecord(); ok {
			out.SiteId = site.ID
			setOpt(&out.SiteName, site.Name)
		}
		return out
	}
}

func defaultListener(id string) api.ListenerRecord {
	return api.ListenerRecord{
		Identity: id,
		Name:     unknownStr,
		Parent:   unknownStr,
		Protocol: unknownStr,
		Address:  unknownStr,
		DestHost: unknownStr,
		DestPort: unknownStr,
		SiteId:   unknownStr,
		SiteName: unknownStr,
	}
}

func NewConnectorSliceProvider(graph collector.Graph) func(entries []store.Entry) []api.ConnectorRecord {
	provider := NewConnectorProvider(graph)
	return func(entries []store.Entry) []api.ConnectorRecord {
		results := make([]api.ConnectorRecord, 0, len(entries))
		for _, e := range entries {
			record, ok := e.Record.(vanflow.ConnectorRecord)
			if !ok {
				continue
			}
			results = append(results, provider(record))
		}
		return results
	}
}

func NewConnectorProvider(graph collector.Graph) func(vanflow.ConnectorRecord) api.ConnectorRecord {
	return func(record vanflow.ConnectorRecord) api.ConnectorRecord {
		out := defaultConnector(record.ID)
		out.StartTime, out.EndTime = vanflowTimes(record.BaseRecord)
		setOpt(&out.Name, record.Name)
		setOpt(&out.Parent, record.Parent)
		setOpt(&out.Protocol, record.Protocol)
		setOpt(&out.DestHost, record.DestHost)
		setOpt(&out.DestPort, record.DestPort)
		setOpt(&out.ProcessId, record.ProcessID)
		setOpt(&out.Address, record.Address)

		node := graph.Connector(record.ID)
		if addressID := node.Address().ID(); addressID != "" {
			out.AddressId = &addressID
		}
		if proc, ok := node.Target().GetRecord(); ok {
			out.Target = proc.Name
			out.ProcessId = proc.ID
		}
		if site, ok := node.Parent().Parent().GetRecord(); ok {
			out.SiteId = site.ID
			setOpt(&out.SiteName, site.Name)
		}
		return out
	}
}

func defaultConnector(id string) api.ConnectorRecord {
	return api.ConnectorRecord{
		Identity: id,
		Name:     unknownStr,
		Parent:   unknownStr,
		Protocol: unknownStr,
		Address:  unknownStr,
		DestHost: unknownStr,
		DestPort: unknownStr,
		SiteId:   unknownStr,
		SiteName: unknownStr,
	}
}

func NewProcessGroupSliceProvider(stor store.Interface) func(entries []store.Entry) []api.ProcessGroupRecord {
	provider := NewProcessGroupProvider(stor)
	return func(entries []store.Entry) []api.ProcessGroupRecord {
		results := make([]api.ProcessGroupRecord, 0, len(entries))
		for _, e := range entries {
			link, ok := e.Record.(collector.ProcessGroupRecord)
			if !ok {
				continue
			}
			results = append(results, provider(link))
		}
		return results
	}
}

func NewProcessGroupProvider(stor store.Interface) func(collector.ProcessGroupRecord) api.ProcessGroupRecord {
	// todo(ck) not v efficient
	allProcesses := stor.Index(store.TypeIndex, store.Entry{Record: vanflow.ProcessRecord{}})
	return func(record collector.ProcessGroupRecord) api.ProcessGroupRecord {
		group := defaultProcessGroup(record.ID)
		group.StartTime = uint64(record.Start.UnixMicro())
		group.Name = record.Name
		var (
			pCount int
			role   string
		)
		for _, p := range allProcesses {
			if proc := p.Record.(vanflow.ProcessRecord); proc.Group != nil && *proc.Group == group.Name {
				pCount++
				if role == "" && proc.Mode != nil {
					role = *proc.Mode
				}
			}
		}
		group.ProcessGroupRole = role
		group.ProcessCount = pCount
		return group
	}
}

func defaultProcessGroup(id string) api.ProcessGroupRecord {
	return api.ProcessGroupRecord{
		Identity:         id,
		Name:             unknownStr,
		ProcessGroupRole: string(api.External),
	}
}

func NewProcessSliceProvider(stor store.Interface, graph collector.Graph) func(entries []store.Entry) []api.ProcessRecord {
	provider := NewProcessProvider(stor, graph)
	return func(entries []store.Entry) []api.ProcessRecord {
		results := make([]api.ProcessRecord, 0, len(entries))
		for _, e := range entries {
			record, ok := e.Record.(vanflow.ProcessRecord)
			if !ok {
				continue
			}
			if result, ok := provider(record); ok {
				results = append(results, result)
			}
		}
		return results
	}
}

func NewProcessProvider(stor store.Interface, graph collector.Graph) func(vanflow.ProcessRecord) (api.ProcessRecord, bool) {
	return func(record vanflow.ProcessRecord) (api.ProcessRecord, bool) {
		out := defaultProcess(record.ID)

		if record.Parent == nil {
			return out, false
		}
		site, ok := graph.Site(*record.Parent).GetRecord()
		if !ok {
			return out, false
		}
		setOpt(&out.ParentName, site.Name)
		out.StartTime, out.EndTime = vanflowTimes(record.BaseRecord)
		out.ImageName = record.ImageName
		out.HostName = record.Hostname
		setOpt(&out.Name, record.Name)
		setOpt(&out.Parent, record.Parent)
		setOpt(&out.SourceHost, record.SourceHost)
		if record.Mode != nil {
			mode := *record.Mode
			switch {
			case strings.EqualFold(mode, "internal"):
				out.ProcessRole = api.Internal
			case strings.EqualFold(mode, "remote"):
				out.ProcessRole = api.Remote
			}
		}

		setOpt(&out.GroupName, record.Group)
		if record.Group != nil {
			group := *record.Group
			groups := stor.Index(collector.IndexByTypeName, store.Entry{Record: collector.ProcessGroupRecord{Name: group}})
			if len(groups) > 0 {
				gid := groups[0].Record.Identity()
				out.GroupIdentity = gid
			}
		}

		node := graph.Process(record.ID)

		addresses := make(map[api.AtmarkDelimitedString]struct{})
		hasConnectors := false
		hasListeners := false
		for _, cNode := range node.Connectors() {
			hasConnectors = true
			addressNode := cNode.Address()
			if !hasListeners {
				if len(addressNode.RoutingKey().Listeners()) > 0 {
					hasListeners = true
				}
			}
			if address, ok := addressNode.GetRecord(); ok {
				addresses[api.NewAtmarkDelimitedString(address.Name, address.ID, address.Protocol)] = struct{}{}
			}
		}

		if len(addresses) > 0 {
			if hasListeners && hasConnectors {
				out.ProcessBinding = api.Bound
			}
			addressList := make([]api.AtmarkDelimitedString, 0, len(addresses))
			for addr := range addresses {
				addressList = append(addressList, addr)
			}
			sort.Slice(addressList, func(i, j int) bool { return addressList[i] < addressList[j] })
			out.Addresses = &addressList
		}

		return out, true
	}
}

func defaultProcess(id string) api.ProcessRecord {
	return api.ProcessRecord{
		Identity:       id,
		Name:           unknownStr,
		Parent:         unknownStr,
		ParentName:     unknownStr,
		GroupIdentity:  unknownStr,
		GroupName:      unknownStr,
		SourceHost:     unknownStr,
		ProcessBinding: api.Unbound,
		ProcessRole:    api.External,
	}
}
func RouterAccessList(entries []store.Entry) []api.RouterAccessRecord {
	results := make([]api.RouterAccessRecord, 0, len(entries))
	for _, e := range entries {
		record, ok := e.Record.(vanflow.RouterAccessRecord)
		if !ok {
			continue
		}
		results = append(results, RouterAccess(record))
	}
	return results
}

func RouterAccess(record vanflow.RouterAccessRecord) api.RouterAccessRecord {
	out := defaultRouterAccess(record.ID)
	out.StartTime, out.EndTime = vanflowTimes(record.BaseRecord)
	setOpt(&out.RouterId, record.Parent)
	setOpt(&out.Name, record.Name)
	setOpt(&out.Role, record.Role)
	setOpt(&out.LinkCount, record.LinkCount)

	return out
}

func defaultRouterAccess(id string) api.RouterAccessRecord {
	return api.RouterAccessRecord{
		Identity: id,
		Name:     unknownStr,
		Role:     unknownStr,
		RouterId: unknownStr,
	}
}
func NewRotuerLinkSliceProvider(graph collector.Graph) func(entries []store.Entry) []api.RouterLinkRecord {
	provider := NewRouterLinkProvider(graph)
	return func(entries []store.Entry) []api.RouterLinkRecord {
		results := make([]api.RouterLinkRecord, 0, len(entries))
		for _, e := range entries {
			link, ok := e.Record.(vanflow.LinkRecord)
			if !ok {
				continue
			}
			if l, ok := provider(link); ok {
				results = append(results, l)
			}
		}
		return results
	}
}

func NewRouterLinkProvider(graph collector.Graph) func(vanflow.LinkRecord) (api.RouterLinkRecord, bool) {
	return func(link vanflow.LinkRecord) (api.RouterLinkRecord, bool) {
		out := defaultRouterLink(link.ID)
		out.StartTime, out.EndTime = vanflowTimes(link.BaseRecord)
		if link.Parent == nil {
			return out, false
		}
		routerNode := graph.Link(link.ID).Parent()
		sourceRouter, found := routerNode.GetRecord()
		if !found {
			return out, false
		}
		siteNode := routerNode.Parent()
		sourceSite, found := siteNode.GetRecord()
		if !found {
			return out, false
		}

		out.RouterId = sourceRouter.ID
		out.SourceSiteId = sourceSite.ID
		setOpt(&out.RouterName, sourceRouter.Name)
		setOpt(&out.SourceSiteName, sourceSite.Name)
		setOpt(&out.Name, link.Name)

		if link.Role != nil {
			role := *link.Role
			switch {
			case strings.EqualFold(role, string(api.LinkRoleTypeInterRouter)):
				out.Role = api.LinkRoleTypeInterRouter
			case strings.EqualFold(role, string(api.LinkRoleTypeEdge)):
				out.Role = api.LinkRoleTypeEdge
			}
		}
		if link.Status != nil && strings.EqualFold(*link.Status, string(api.Up)) {
			out.Status = api.Up
		}

		if link.Peer == nil {
			return out, true
		}
		out.RouterAccessId = link.Peer
		out.Cost = link.LinkCost

		raN := graph.RouterAccess(*link.Peer)
		raRouterN := raN.Parent()
		raSiteN := raRouterN.Parent()
		if destRouter, found := raRouterN.GetRecord(); found {
			out.DestinationRouterId = &destRouter.ID
			if destRouter.Name == nil {
				tmp := unknownStr
				destRouter.Name = &tmp
			}
			out.DestinationRouterName = destRouter.Name
		}
		if destSite, found := raSiteN.GetRecord(); found {
			out.DestinationSiteId = &destSite.ID
			if destSite.Name == nil {
				tmp := unknownStr
				destSite.Name = &tmp

			}
			out.DestinationSiteName = destSite.Name
		}

		return out, true
	}
}

func defaultRouterLink(id string) api.RouterLinkRecord {
	return api.RouterLinkRecord{
		Identity:       id,
		Name:           unknownStr,
		Role:           api.LinkRoleTypeUnknown,
		RouterId:       unknownStr,
		RouterName:     unknownStr,
		SourceSiteId:   unknownStr,
		SourceSiteName: unknownStr,
		Status:         api.Down,
	}
}

func NewAddressSliceProvider(stor store.Interface, graph collector.Graph) func(entries []store.Entry) []api.AddressRecord {
	provider := NewAddressProvider(stor, graph)
	return func(entries []store.Entry) []api.AddressRecord {
		results := make([]api.AddressRecord, 0, len(entries))
		for _, e := range entries {
			record, ok := e.Record.(collector.AddressRecord)
			if !ok {
				continue
			}
			results = append(results, provider(record))
		}
		return results
	}
}

func NewAddressProvider(stor store.Interface, graph collector.Graph) func(collector.AddressRecord) api.AddressRecord {
	requestRecordType := collector.RequestRecord{}.GetTypeMeta().String()
	flowIndex := stor.IndexValues(collector.IndexFlowByAddress)
	addressAppProtocols := make(map[string]map[string]struct{})
	for _, value := range flowIndex {
		addressProtocol, found := strings.CutPrefix(value, requestRecordType+"/")
		if !found {
			continue
		}
		last := strings.LastIndex(addressProtocol, "/")
		if last < 0 {
			continue
		}
		address, protocol := addressProtocol[:last], addressProtocol[last+1:]
		if addressAppProtocols[address] == nil {
			addressAppProtocols[address] = make(map[string]struct{})
		}
		addressAppProtocols[address][protocol] = struct{}{}
	}
	return func(record collector.AddressRecord) api.AddressRecord {
		node := graph.Address(record.ID).RoutingKey()
		listenerCt := len(node.Listeners())
		connectorCt := len(node.Connectors())

		protocols := make([]string, 0, 2)
		for proto := range addressAppProtocols[record.Name] {
			protocols = append(protocols, proto)
		}
		return api.AddressRecord{
			Identity:                     record.ID,
			StartTime:                    uint64(record.Start.UnixMicro()),
			Protocol:                     record.Protocol,
			ObservedApplicationProtocols: protocols,
			Name:                         record.Name,
			ListenerCount:                listenerCt,
			HasListener:                  listenerCt > 0,
			ConnectorCount:               connectorCt,
			IsBound:                      listenerCt > 0 && connectorCt > 0,
		}
	}
}

func NewLinkSliceProvider(graph collector.Graph) func(entries []store.Entry) []api.LinkRecord {
	provider := NewLinkProvider(graph)
	return func(entries []store.Entry) []api.LinkRecord {
		results := make([]api.LinkRecord, 0, len(entries))
		for _, e := range entries {
			link, ok := e.Record.(vanflow.LinkRecord)
			if !ok {
				continue
			}
			if l, ok := provider(link); ok {
				results = append(results, l)
			}
		}
		return results
	}
}

func NewLinkProvider(graph collector.Graph) func(vanflow.LinkRecord) (api.LinkRecord, bool) {
	return func(link vanflow.LinkRecord) (api.LinkRecord, bool) {
		out := defaultLink(link.ID)
		out.StartTime, out.EndTime = vanflowTimes(link.BaseRecord)
		if link.Status == nil || !strings.EqualFold(*link.Status, "up") ||
			link.Parent == nil || link.Peer == nil {
			return out, false
		}
		siteNode := graph.Link(link.ID).Parent().Parent()
		if !siteNode.IsKnown() {
			return out, false
		}
		out.SourceSiteId = siteNode.ID()

		destSiteNode := graph.RouterAccess(*link.Peer).Parent().Parent()
		if !destSiteNode.IsKnown() {
			return out, false
		}
		out.DestinationSiteId = destSiteNode.ID()

		setOpt(&out.LinkCost, link.LinkCost)
		setOpt(&out.Mode, link.Role)
		setOpt(&out.Name, link.Name)
		setOpt(&out.Parent, link.Parent)
		return out, true
	}
}

func defaultLink(id string) api.LinkRecord {
	return api.LinkRecord{
		Identity:  id,
		Mode:      unknownStr,
		Name:      unknownStr,
		Direction: "outgoing",
	}
}

func Routers(entries []store.Entry) []api.RouterRecord {
	results := make([]api.RouterRecord, 0, len(entries))
	for _, e := range entries {
		record, ok := e.Record.(vanflow.RouterRecord)
		if !ok {
			continue
		}
		results = append(results, Router(record))
	}
	return results
}

func Router(record vanflow.RouterRecord) api.RouterRecord {
	out := defaultRouter(record.ID)
	out.StartTime, out.EndTime = vanflowTimes(record.BaseRecord)
	out.Namespace = record.Namespace
	setOpt(&out.Parent, record.Parent)
	setOpt(&out.HostName, record.Hostname)
	setOpt(&out.ImageName, record.ImageName)
	setOpt(&out.ImageVersion, record.ImageVersion)
	setOpt(&out.Mode, record.Mode)
	setOpt(&out.Name, record.Name)

	return out
}

func defaultRouter(id string) api.RouterRecord {
	return api.RouterRecord{
		Identity:     id,
		HostName:     unknownStr,
		ImageName:    unknownStr,
		ImageVersion: unknownStr,
		Mode:         unknownStr,
		Name:         unknownStr,
		Parent:       unknownStr,
	}
}

func NewSiteSliceProvider(graph collector.Graph) func(entries []store.Entry) []api.SiteRecord {
	provider := NewSiteProvider(graph)
	return func(entries []store.Entry) []api.SiteRecord {
		results := make([]api.SiteRecord, 0, len(entries))
		for _, e := range entries {
			site, ok := e.Record.(vanflow.SiteRecord)
			if !ok {
				continue
			}
			results = append(results, provider(site))
		}
		return results
	}
}

func NewSiteProvider(graph collector.Graph) func(vanflow.SiteRecord) api.SiteRecord {
	return func(site vanflow.SiteRecord) api.SiteRecord {
		s := defaultSite(site.ID)
		s.StartTime, s.EndTime = vanflowTimes(site.BaseRecord)
		s.NameSpace = site.Namespace

		setOpt(&s.Name, site.Name)
		setOpt(&s.Provider, site.Provider)
		setOpt(&s.SiteVersion, site.Version)
		if site.Platform != nil {
			platform := *site.Platform
			switch {
			case strings.EqualFold(platform, string(api.SitePlatformTypeKubernetes)):
				s.Platform = api.SitePlatformTypeKubernetes
			case strings.EqualFold(platform, string(api.SitePlatformTypeDocker)):
				s.Platform = api.SitePlatformTypeDocker
			case strings.EqualFold(platform, string(api.SitePlatformTypePodman)):
				s.Platform = api.SitePlatformTypePodman
			case strings.EqualFold(platform, string(api.SitePlatformTypeLinux)):
				s.Platform = api.SitePlatformTypeLinux
			}
		}
		s.RouterCount = len(graph.Site(site.ID).Routers())
		return s
	}
}

func defaultSite(id string) api.SiteRecord {
	return api.SiteRecord{
		Identity:    id,
		Name:        unknownStr,
		Platform:    api.SitePlatformTypeUnknown,
		SiteVersion: unknownStr,
	}
}

func vanflowTimes(b vanflow.BaseRecord) (start, end uint64) {
	if b.StartTime != nil {
		start = uint64(b.StartTime.UnixMicro())
	}
	if b.EndTime != nil {
		end = uint64(b.EndTime.UnixMicro())
	}
	return
}

func setOpt[T any](target *T, val *T) {
	if val == nil {
		return
	}
	*target = *val
}

package views

import (
	"fmt"
	"strings"

	"github.com/skupperproject/skupper/cmd/network-console-collector/internal/api"
	"github.com/skupperproject/skupper/cmd/network-console-collector/internal/collector"
	"github.com/skupperproject/skupper/pkg/vanflow"
	"github.com/skupperproject/skupper/pkg/vanflow/store"
)

const unknownStr = "unknown"

func NewProcessesProvider(stor store.Interface, graph *collector.Graph) func(entries []store.Entry) []api.ProcessRecord {
	provider := NewProcessProvider(stor, graph)
	return func(entries []store.Entry) []api.ProcessRecord {
		results := make([]api.ProcessRecord, 0, len(entries))
		for _, e := range entries {
			link, ok := e.Record.(vanflow.ProcessRecord)
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

func NewProcessProvider(stor store.Interface, graph *collector.Graph) func(vanflow.ProcessRecord) (api.ProcessRecord, bool) {
	return func(record vanflow.ProcessRecord) (api.ProcessRecord, bool) {
		out := defaultProcess(record.ID)
		out.StartTime, out.EndTime = vanflowTimes(record.BaseRecord)
		out.ImageName = record.ImageName
		out.HostName = record.Hostname
		setOpt(&out.Name, record.Name)
		setOpt(&out.Parent, record.Parent)
		setOpt(&out.SourceHost, record.SourceHost)

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

		var addresses []string
		for _, cNode := range node.Connectors() {
			if addressEntry, ok := cNode.Address().Get(); ok {
				address, ok := addressEntry.Record.(collector.AddressRecord)
				if !ok {
					continue
				}
				addresses = append(addresses, fmt.Sprintf("%s@%s@%s", address.Name, address.ID, address.Protocol))
			}
		}
		if site, ok := node.Parent().Get(); ok {
			setOpt(&out.ParentName, site.Record.(vanflow.SiteRecord).Name)
		}

		if len(addresses) > 0 {
			out.ProcessBinding = api.Bound
			out.Addresses = &addresses
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
func NewRotuerLinksProvider(graph *collector.Graph) func(entries []store.Entry) []api.RouterLinkRecord {
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

func NewRouterLinkProvider(graph *collector.Graph) func(vanflow.LinkRecord) (api.RouterLinkRecord, bool) {
	return func(link vanflow.LinkRecord) (api.RouterLinkRecord, bool) {
		out := defaultRouterLink(link.ID)
		out.StartTime, out.EndTime = vanflowTimes(link.BaseRecord)
		if link.Parent == nil {
			return out, false
		}
		siteNode := graph.Link(link.ID).Parent().Parent()
		if !siteNode.IsKnown() {
			return out, false
		}
		out.SourceSiteId = siteNode.ID()

		setOpt(&out.RouterId, link.Parent)
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
		out.Peer = link.Peer
		out.Cost = link.LinkCost

		raN := graph.RouterAccess(*link.Peer)
		raSiteN := raN.Parent().Parent()
		if raSiteN.IsKnown() {
			destSiteID := raSiteN.ID()
			out.DestinationSiteId = &destSiteID
		}

		return out, true
	}
}

func defaultRouterLink(id string) api.RouterLinkRecord {
	return api.RouterLinkRecord{
		Identity:     id,
		Name:         unknownStr,
		Role:         api.LinkRoleTypeUnknown,
		RouterId:     unknownStr,
		SourceSiteId: unknownStr,
		Status:       api.Down,
	}
}

func NewAddressesProvider(graph *collector.Graph) func(entries []store.Entry) []api.AddressRecord {
	provider := NewAddressProvider(graph)
	return func(entries []store.Entry) []api.AddressRecord {
		results := make([]api.AddressRecord, 0, len(entries))
		for _, e := range entries {
			record, ok := e.Record.(collector.AddressRecord)
			if !ok {
				continue
			}
			if l, ok := provider(record); ok {
				results = append(results, l)
			}
		}
		return results
	}
}

func NewAddressProvider(graph *collector.Graph) func(collector.AddressRecord) (api.AddressRecord, bool) {
	return func(record collector.AddressRecord) (api.AddressRecord, bool) {
		node := graph.Address(record.ID)
		return api.AddressRecord{
			Identity:       record.ID,
			StartTime:      uint64(record.Start.UnixMicro()),
			Protocol:       record.Protocol,
			Name:           record.Name,
			ListenerCount:  len(node.Listeners()),
			ConnectorCount: len(node.Connectors()),
		}, true
	}
}

func NewLinksProvider(graph *collector.Graph) func(entries []store.Entry) []api.LinkRecord {
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

func NewLinkProvider(graph *collector.Graph) func(vanflow.LinkRecord) (api.LinkRecord, bool) {
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

func Sites(entries []store.Entry) []api.SiteRecord {
	results := make([]api.SiteRecord, 0, len(entries))
	for _, e := range entries {
		site, ok := e.Record.(vanflow.SiteRecord)
		if !ok {
			continue
		}
		results = append(results, Site(site))
	}
	return results
}

func Site(site vanflow.SiteRecord) api.SiteRecord {
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
		}
	}
	return s
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
		start = uint64(b.EndTime.UnixMicro())
	}
	return
}

func setOpt[T any](target *T, val *T) {
	if val == nil {
		return
	}
	*target = *val
}

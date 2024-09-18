package views

import (
	"strings"
	"time"

	"github.com/skupperproject/skupper/cmd/network-console-collector/internal/api"
	"github.com/skupperproject/skupper/cmd/network-console-collector/internal/collector"
	"github.com/skupperproject/skupper/pkg/vanflow"
	"github.com/skupperproject/skupper/pkg/vanflow/store"
)

func NewConnectionsSliceProvider(stor store.Interface) func([]store.Entry) []api.ConnectionRecord {
	provider := NewConnectionsProvider(stor)
	return func(entries []store.Entry) []api.ConnectionRecord {
		results := make([]api.ConnectionRecord, 0, len(entries))
		for _, e := range entries {
			record, ok := e.Record.(collector.ConnectionRecord)
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
func NewConnectionsProvider(stor store.Interface) func(collector.ConnectionRecord) (api.ConnectionRecord, bool) {
	memo := make(map[string]struct {
		Router string
		Site   string
	})
	routerAndSiteByTracePart := func(name string) (router string, site string, ok bool) {
		if m, ok := memo[name]; ok {
			return m.Router, m.Site, true
		}
		routers := stor.Index(collector.IndexByTypeName, store.Entry{
			Record: vanflow.RouterRecord{Name: &name},
		})
		if len(routers) == 0 {
			return router, site, false
		}
		re := routers[0]
		rr := re.Record.(vanflow.RouterRecord)
		if rr.Name == nil || rr.Parent == nil {
			return router, site, false
		}

		se, ok := stor.Get(*rr.Parent)
		if !ok {
			return router, site, false
		}
		sr, ok := se.Record.(vanflow.SiteRecord)
		if !ok || sr.Name == nil {
			return router, site, false
		}

		memo[name] = struct {
			Router string
			Site   string
		}{*rr.Name, *sr.Name}
		return *rr.Name, *sr.Name, true
	}
	return func(conn collector.ConnectionRecord) (api.ConnectionRecord, bool) {
		out := defaultConnection(conn.ID)

		record, ok := conn.GetFlow()

		if !ok {
			return out, false
		}

		out.StartTime, out.EndTime = vanflowTimes(record.BaseRecord)
		out.ConnectorError = record.ErrorConnector
		out.ListenerError = record.ErrorListener
		setOpt(&out.Octets, record.Octets)
		setOpt(&out.OctetsReverse, record.OctetsReverse)
		setOpt(&out.Latency, record.Latency)
		setOpt(&out.LatencyReverse, record.LatencyReverse)
		setOpt(&out.SourceHost, record.SourceHost)
		setOpt(&out.SourcePort, record.SourcePort)
		setOpt(&out.ProxyHost, record.ProxyHost)
		setOpt(&out.ProxyPort, record.ProxyPort)

		if record.EndTime != nil && record.StartTime != nil && record.EndTime.After(record.StartTime.Time) {
			if record.EndTime != nil && record.StartTime != nil {
				duration := uint64(record.EndTime.Sub(record.StartTime.Time) / time.Microsecond)
				out.Duration = &duration
			}
		}

		out.TraceSites = append(out.TraceSites, conn.SourceSite.Name)
		out.TraceRouters = append(out.TraceRouters, conn.SourceRouter.Name)
		if record.Trace != nil {
			parts := strings.Split(*record.Trace, "|")
			for _, routerName := range parts {
				router, site, ok := routerAndSiteByTracePart(routerName)
				if !ok {
					continue
				}
				if last := out.TraceRouters[len(out.TraceRouters)-1]; last != router {
					out.TraceRouters = append(out.TraceRouters, router)
				}
				if last := out.TraceSites[len(out.TraceSites)-1]; last != site {
					out.TraceSites = append(out.TraceSites, site)
				}
			}
		}
		if last := out.TraceSites[len(out.TraceSites)-1]; last != conn.DestSite.Name {
			out.TraceSites = append(out.TraceSites, conn.DestSite.Name)
		}
		if last := out.TraceRouters[len(out.TraceRouters)-1]; last != conn.DestRouter.Name {
			out.TraceRouters = append(out.TraceRouters, conn.DestRouter.Name)
		}

		out.Protocol = conn.Protocol
		out.RoutingKey = conn.RoutingKey
		out.SourceProcessId = conn.Source.ID
		out.SourceProcessName = conn.Source.Name
		out.SourceSiteId = conn.SourceSite.ID
		out.SourceSiteName = conn.SourceSite.Name
		out.DestProcessId = conn.Dest.ID
		out.DestProcessName = conn.Dest.Name
		out.DestSiteId = conn.DestSite.ID
		out.DestSiteName = conn.DestSite.Name
		out.ListenerId = conn.Listener.ID
		out.ConnectorId = conn.Connector.ID
		out.DestHost = conn.ConnectorHost
		out.DestPort = conn.ConnectorPort

		return out, true
	}
}

func defaultConnection(id string) api.ConnectionRecord {
	return api.ConnectionRecord{
		Identity:     id,
		TraceRouters: []string{},
		TraceSites:   []string{},
	}
}

func NewRequestSliceProvider() func([]store.Entry) []api.RequestRecord {
	provider := NewRequestProvider()
	return func(entries []store.Entry) []api.RequestRecord {
		results := make([]api.RequestRecord, 0, len(entries))
		for _, e := range entries {
			record, ok := e.Record.(collector.RequestRecord)
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
func NewRequestProvider() func(collector.RequestRecord) (api.RequestRecord, bool) {
	return func(conn collector.RequestRecord) (api.RequestRecord, bool) {
		out := defaultRequest(conn.ID)

		record, ok := conn.GetFlow()

		if !ok {
			return out, false
		}

		out.StartTime, out.EndTime = vanflowTimes(record.BaseRecord)
		setOpt(&out.Method, record.Method)
		setOpt(&out.Result, record.Result)

		if record.EndTime != nil && record.StartTime != nil && record.EndTime.After(record.StartTime.Time) {
			out.Active = false
			if record.EndTime != nil && record.StartTime != nil {
				duration := uint64(record.EndTime.Sub(record.StartTime.Time) / time.Microsecond)
				out.Duration = &duration
			}
		}

		out.ConnectionId = conn.TransportID
		out.Protocol = conn.Protocol
		out.RoutingKey = conn.RoutingKey
		out.SourceProcessId = conn.Source.ID
		out.SourceProcessName = conn.Source.Name
		out.SourceSiteId = conn.SourceSite.ID
		out.SourceSiteName = conn.SourceSite.Name
		out.DestProcessId = conn.Dest.ID
		out.DestProcessName = conn.Dest.Name
		out.DestSiteId = conn.DestSite.ID
		out.DestSiteName = conn.DestSite.Name
		out.ListenerId = conn.Listener.ID

		return out, true
	}
}

func defaultRequest(id string) api.RequestRecord {
	return api.RequestRecord{
		Identity: id,
		Active:   true,
	}
}

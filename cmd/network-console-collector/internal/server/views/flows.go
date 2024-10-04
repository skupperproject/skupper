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
	traceProvider := newTraceProvider(stor)
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

		var trace string
		if record.Trace != nil {
			trace = *record.Trace
		}
		out.TraceRouters, out.TraceSites = traceProvider(conn.SourceSite.Name, conn.SourceRouter.Name, conn.DestSite.Name, conn.DestRouter.Name, trace)

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

func newTraceProvider(stor store.Interface) func(sourceSite, sourceRouter, destSite, destRouter, trace string) ([]string, []string) {
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
	return func(sourceSite, sourceRouter, destSite, destRouter, trace string) ([]string, []string) {
		traceSites := []string{sourceSite}
		traceRouters := []string{sourceRouter}
		if trace != "" {
			parts := strings.Split(trace, "|")
			for _, routerName := range parts {
				router, site, ok := routerAndSiteByTracePart(routerName)
				if !ok {
					continue
				}
				if last := traceRouters[len(traceRouters)-1]; last != router {
					traceRouters = append(traceRouters, router)
				}
				if last := traceSites[len(traceSites)-1]; last != site {
					traceSites = append(traceSites, site)
				}
			}
		}
		if last := traceSites[len(traceSites)-1]; last != destSite {
			traceSites = append(traceSites, destSite)
		}
		if last := traceRouters[len(traceRouters)-1]; last != destRouter {
			traceRouters = append(traceRouters, destRouter)
		}
		return traceRouters, traceSites
	}
}

func NewRequestSliceProvider(stor store.Interface) func([]store.Entry) []api.ApplicationFlowRecord {
	provider := NewRequestProvider(stor)
	return func(entries []store.Entry) []api.ApplicationFlowRecord {
		results := make([]api.ApplicationFlowRecord, 0, len(entries))
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
func NewRequestProvider(stor store.Interface) func(collector.RequestRecord) (api.ApplicationFlowRecord, bool) {
	traceProvider := newTraceProvider(stor)

	memo := make(map[string]vanflow.TransportBiflowRecord)
	getTransport := func(request collector.RequestRecord) (vanflow.TransportBiflowRecord, bool) {
		if t, ok := memo[request.TransportID]; ok {
			return t, true
		}
		t, ok := request.GetTransport()
		if ok {
			memo[request.TransportID] = t
		}
		return t, ok
	}
	return func(request collector.RequestRecord) (api.ApplicationFlowRecord, bool) {
		out := defaultRequest(request.ID)

		record, ok := request.GetFlow()

		if !ok {
			return out, false
		}
		conn, ok := getTransport(request)
		if !ok {
			return out, false
		}

		out.StartTime, out.EndTime = vanflowTimes(record.BaseRecord)
		setOpt(&out.Method, record.Method)
		setOpt(&out.Status, record.Result)

		if record.EndTime != nil && record.StartTime != nil && record.EndTime.After(record.StartTime.Time) {
			if record.EndTime != nil && record.StartTime != nil {
				duration := uint64(record.EndTime.Sub(record.StartTime.Time) / time.Microsecond)
				out.Duration = &duration
			}
		}

		var trace string
		if conn.Trace != nil {
			trace = *conn.Trace
		}
		out.TraceRouters, out.TraceSites = traceProvider(request.SourceSite.Name, request.SourceRouter.Name, request.DestSite.Name, request.DestRouter.Name, trace)
		out.ConnectionId = request.TransportID
		out.Protocol = request.Protocol
		out.RoutingKey = request.RoutingKey
		out.SourceProcessId = request.Source.ID
		out.SourceProcessName = request.Source.Name
		out.SourceSiteId = request.SourceSite.ID
		out.SourceSiteName = request.SourceSite.Name
		out.DestProcessId = request.Dest.ID
		out.DestProcessName = request.Dest.Name
		out.DestSiteId = request.DestSite.ID
		out.DestSiteName = request.DestSite.Name

		return out, true
	}
}

func defaultRequest(id string) api.ApplicationFlowRecord {
	return api.ApplicationFlowRecord{
		Identity: id,
	}
}

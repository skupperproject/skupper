package views

import (
	"time"

	"github.com/skupperproject/skupper/cmd/network-console-collector/internal/api"
	"github.com/skupperproject/skupper/cmd/network-console-collector/internal/collector"
	"github.com/skupperproject/skupper/pkg/vanflow/store"
)

func NewConnectionsSliceProvider() func([]store.Entry) []api.ConnectionRecord {
	provider := NewConnectionsProvider()
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
func NewConnectionsProvider() func(collector.ConnectionRecord) (api.ConnectionRecord, bool) {
	return func(conn collector.ConnectionRecord) (api.ConnectionRecord, bool) {
		out := defaultConnection(conn.ID)

		record, ok := conn.GetFlow()

		if !ok {
			return out, false
		}

		out.StartTime, out.EndTime = vanflowTimes(record.BaseRecord)
		out.ConnectorError = record.ErrorConnector
		out.ListenerError = record.ErrorListener
		setOpt(&out.Trace, record.Trace)
		setOpt(&out.Octets, record.Octets)
		setOpt(&out.OctetsReverse, record.OctetsReverse)
		setOpt(&out.Latency, record.Latency)
		setOpt(&out.LatencyReverse, record.LatencyReverse)
		setOpt(&out.SourceHost, record.SourceHost)
		setOpt(&out.SourcePort, record.SourcePort)
		setOpt(&out.ProxyHost, record.ProxyHost)
		setOpt(&out.ProxyPort, record.ProxyPort)

		if record.EndTime != nil && record.StartTime != nil && record.EndTime.After(record.StartTime.Time) {
			out.Active = false
			if record.EndTime != nil && record.StartTime != nil {
				duration := uint64(record.EndTime.Sub(record.StartTime.Time) / time.Microsecond)
				out.Duration = &duration
			}
		}

		out.Protocol = conn.Protocol
		out.Address = conn.Address
		out.SourceProcessId = conn.Source.ID
		out.SourceProcessName = conn.Source.Name
		out.SourceSiteId = conn.SourceSite.ID
		out.SourceSiteName = conn.SourceSite.Name
		out.DestProcessId = conn.Dest.ID
		out.DestProcessName = conn.Dest.Name
		out.DestSiteId = conn.DestSite.ID
		out.DestSiteName = conn.DestSite.Name
		out.ListenerId = conn.Listener.ID
		out.ConnectorId = conn.Listener.ID

		return out, true
	}
}

func defaultConnection(id string) api.ConnectionRecord {
	return api.ConnectionRecord{
		Identity: id,
		Active:   true,
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
		out.Address = conn.Address
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

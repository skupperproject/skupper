package views

import (
	"time"

	"github.com/skupperproject/skupper/cmd/network-console-collector/internal/api"
	"github.com/skupperproject/skupper/cmd/network-console-collector/internal/collector"
	"github.com/skupperproject/skupper/pkg/vanflow"
	"github.com/skupperproject/skupper/pkg/vanflow/store"
)

func NewConnectionsSliceProvider(fs collector.FlowStateAccess) func([]store.Entry) []api.ConnectionRecord {
	provider := NewConnectionsProvider(fs)
	return func(entries []store.Entry) []api.ConnectionRecord {
		results := make([]api.ConnectionRecord, 0, len(entries))
		for _, e := range entries {
			record, ok := e.Record.(vanflow.TransportBiflowRecord)
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
func NewConnectionsProvider(fs collector.FlowStateAccess) func(vanflow.TransportBiflowRecord) (api.ConnectionRecord, bool) {
	return func(record vanflow.TransportBiflowRecord) (api.ConnectionRecord, bool) {
		out := defaultConnection(record.ID)

		state, ok := fs.Get(record.ID)
		if !ok || !state.Conditions.FullyQualified() {
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

		if state.Conditions.Terminated {
			out.Active = false
			if record.EndTime != nil && record.StartTime != nil {
				duration := uint64(record.EndTime.Sub(record.StartTime.Time) / time.Microsecond)
				out.Duration = &duration
			}
		}

		out.Protocol = state.Connector.Protocol
		out.Address = state.Connector.Address
		out.SourceProcessId = state.Source.ID
		out.SourceProcessName = state.Source.Name
		out.SourceSiteId = state.Source.SiteID
		out.SourceSiteName = state.Source.SiteName
		out.DestProcessId = state.Dest.ID
		out.DestProcessName = state.Dest.Name
		out.DestSiteId = state.Dest.SiteID
		out.DestSiteName = state.Dest.SiteName

		return out, true
	}
}

func defaultConnection(id string) api.ConnectionRecord {
	return api.ConnectionRecord{
		Identity: id,
		Active:   true,
	}
}

package data

import (
	"github.com/skupperproject/skupper/pkg/qdr"
)

type HttpRequestsHandledList []HttpRequestsHandled
type HttpRequestsReceivedList []HttpRequestsReceived

type HttpService struct {
	Service
	RequestsReceived HttpRequestsReceivedList `json:"requests_received"`
	RequestsHandled  HttpRequestsHandledList  `json:"requests_handled"`
}

type HttpServiceMap map[string]HttpService
type HttpRequestStatsMap map[string]HttpRequestStats

type HttpRequestsReceived struct {
	SiteId   string              `json:"site_id"`
	ByClient HttpRequestStatsMap `json:"by_client,omitempty"`
}

type HttpRequestsHandled struct {
	SiteId            string              `json:"site_id"`
	ByServer          HttpRequestStatsMap `json:"by_server,omitempty"`
	ByOriginatingSite HttpRequestStatsMap `json:"by_originating_site,omitempty"`
}

type DetailsMap map[string]int

type HttpRequestStats struct {
	Requests       int                 `json:"requests"`
	BytesIn        int                 `json:"bytes_in"`
	BytesOut       int                 `json:"bytes_out"`
	Details        DetailsMap          `json:"details"`
	LatencyMax     int                 `json:"latency_max"`
	ByHandlingSite HttpRequestStatsMap `json:"by_handling_site,omitempty"`
}

func max(a int, b int) int {
	if b > a {
		return b
	} else {
		return a
	}
}

func (a DetailsMap) merge(b DetailsMap) DetailsMap {
	c := DetailsMap{}
	for k, v := range a {
		c[k] = v
	}
	for k, v := range b {
		if s, ok := c[k]; ok {
			c[k] = s + v
		} else {
			c[k] = v
		}
	}
	return c
}

func (a *HttpRequestStats) merge(b *HttpRequestStats) {
	a.Requests += b.Requests
	a.BytesIn += b.BytesIn
	a.BytesOut += b.BytesOut
	a.LatencyMax = max(a.LatencyMax, b.LatencyMax)
	if a.Details == nil {
		a.Details = b.Details
	} else if b.Details != nil {
		a.Details = a.Details.merge(b.Details)
	}
	if a.ByHandlingSite == nil {
		a.ByHandlingSite = b.ByHandlingSite
	} else if b.ByHandlingSite != nil {
		a.ByHandlingSite.merge(b.ByHandlingSite)
	}
}

func (a HttpRequestStatsMap) merge(b HttpRequestStatsMap) {
	for k, v := range b {
		if s, ok := a[k]; ok {
			s.merge(&v)
			a[k] = s
		} else {
			a[k] = v
		}
	}
}

func (a *HttpRequestsReceived) merge(b *HttpRequestsReceived) {
	if a.ByClient == nil {
		a.ByClient = b.ByClient
	} else if b.ByClient != nil {
		a.ByClient.merge(b.ByClient)
	}
}

func (a *HttpRequestsHandled) merge(b *HttpRequestsHandled) {
	if a.ByServer == nil {
		a.ByServer = b.ByServer
	} else if b.ByServer != nil {
		a.ByServer.merge(b.ByServer)
	}
	if a.ByOriginatingSite == nil {
		a.ByOriginatingSite = b.ByOriginatingSite
	} else if b.ByOriginatingSite != nil {
		a.ByOriginatingSite.merge(b.ByOriginatingSite)
	}
}

func (service *HttpService) mergeReceived(r *HttpRequestsReceived) {
	found := false
	for _, entry := range service.RequestsReceived {
		if entry.SiteId == r.SiteId {
			entry.merge(r)
			found = true
		}
	}
	if !found {
		service.RequestsReceived = append(service.RequestsReceived, *r)
	}
}

func (service *HttpService) mergeHandled(r *HttpRequestsHandled) {
	found := false
	for _, entry := range service.RequestsHandled {
		if entry.SiteId == r.SiteId {
			entry.merge(r)
			found = true
		}
	}
	if !found {
		service.RequestsHandled = append(service.RequestsHandled, *r)
	}
}

func getHttpProtocol(protocolVersion string) string {
	if protocolVersion == qdr.HttpVersion2 {
		return "http2"
	} else {
		return "http"
	}
}

func asHttpRequestStats(r *qdr.HttpRequestInfo) HttpRequestStats {
	stats := HttpRequestStats{
		Requests:   r.Requests,
		LatencyMax: r.MaxLatency,
		BytesIn:    r.BytesIn,
		BytesOut:   r.BytesOut,
		Details:    r.Details,
	}
	if r.Direction == qdr.DirectionIn {
		stats.ByHandlingSite = HttpRequestStatsMap{
			r.Site: stats,
		}
	}
	return stats
}

func (index HttpServiceMap) merge(services []HttpService) {
	for _, s := range services {
		if service, ok := index[s.Address]; ok {
			if s.Targets != nil {
				service.Targets = append(service.Targets, s.Targets...)
			}
			for _, received := range s.RequestsReceived {
				service.mergeReceived(&received)
			}
			for _, handled := range s.RequestsHandled {
				service.mergeHandled(&handled)
			}
			index[s.Address] = service
		} else {
			index[s.Address] = s
		}
	}
}

func (index HttpServiceMap) Update(siteId string, requests []qdr.HttpRequestInfo, mapping NameMapping) {
	for _, r := range requests {
		host := mapping.Lookup(r.Host)
		stats := asHttpRequestStats(&r)
		if service, ok := index[r.Address]; ok {
			if r.Direction == qdr.DirectionIn {
				received := HttpRequestsReceived{
					SiteId: siteId,
					ByClient: HttpRequestStatsMap{
						host: stats,
					},
				}
				service.mergeReceived(&received)
			} else {
				handled := HttpRequestsHandled{
					SiteId: siteId,
					ByServer: HttpRequestStatsMap{
						host: stats,
					},
					ByOriginatingSite: HttpRequestStatsMap{
						r.Site: stats,
					},
				}
				service.mergeHandled(&handled)
			}
			index[r.Address] = service
		}
	}
}

func (index HttpServiceMap) AddTargets(connectors []qdr.HttpEndpoint, mapping NameMapping) {
	for _, c := range connectors {
		service, ok := index[c.Address]
		if !ok {
			service = HttpService{
				Service: Service{
					Address:  c.Address,
					Protocol: getHttpProtocol(c.ProtocolVersion),
				},
			}
		}
		service.AddTarget(c.Name, c.Host, c.SiteId, mapping)
		index[c.Address] = service
	}
}

func (index HttpServiceMap) AddServices(listeners []qdr.HttpEndpoint) {
	for _, l := range listeners {
		if _, ok := index[l.Address]; !ok {
			service := HttpService{
				Service: Service{
					Address:  l.Address,
					Protocol: getHttpProtocol(l.ProtocolVersion),
				},
			}
			index[l.Address] = service
		}
	}
}

func (index HttpServiceMap) AsList() []HttpService {
	list := []HttpService{}
	for _, v := range index {
		list = append(list, v)
	}
	return list
}

func GetHttpServices(siteId string, info [][]qdr.HttpRequestInfo, targets []qdr.HttpEndpoint, listeners []qdr.HttpEndpoint, lookup NameMapping) []HttpService {
	flattened := []qdr.HttpRequestInfo{}
	for _, l := range info {
		flattened = append(flattened, l...)
	}
	services := HttpServiceMap{}
	services.AddServices(listeners)
	services.Update(siteId, flattened, lookup)
	services.AddTargets(targets, lookup)
	return services.AsList()
}

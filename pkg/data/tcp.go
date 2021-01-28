package data

import (
	"strings"

	"github.com/skupperproject/skupper/pkg/qdr"
)

type TcpService struct {
	Service
	ConnectionsIngress TcpServiceEndpointsList `json:"connections_ingress,omitempty"`
	ConnectionsEgress  TcpServiceEndpointsList `json:"connections_egress,omitempty"`
}

type TcpServiceMap map[string]TcpService

type TcpServiceEndpointsList []TcpServiceEndpoints

type TcpServiceEndpoints struct {
	SiteId      string                        `json:"site_id"`
	Connections map[string]TcpConnectionStats `json:"connections"`
}

type TcpConnectionStats struct {
	Id        string `json:"id"`
	StartTime uint64 `json:"start_time"`
	LastOut   uint64 `json:"last_out"`
	LastIn    uint64 `json:"last_in"`
	BytesIn   int    `json:"bytes_in"`
	BytesOut  int    `json:"bytes_out"`
	Client    string `json:"client,omitempty"`
	Server    string `json:"server,omitempty"`
}

func asTcpConnectionStats(connection *qdr.TcpConnection, mapping NameMapping) TcpConnectionStats {
	stats := TcpConnectionStats{
		Id:        connection.Name,
		StartTime: connection.Uptime,
		LastOut:   connection.LastOut,
		LastIn:    connection.LastIn,
		BytesIn:   connection.BytesIn,
		BytesOut:  connection.BytesOut,
	}
	peer := mapping.Lookup(strings.Split(connection.Host, ":")[0])
	if connection.Direction == qdr.DirectionIn {
		stats.Client = peer
	} else {
		stats.Server = peer
	}
	return stats
}

func (a *TcpServiceEndpoints) merge(b *TcpServiceEndpoints) {
	for k, v := range b.Connections {
		a.Connections[k] = v
	}
}

func (service *TcpService) mergeIngress(record *TcpServiceEndpoints) {
	found := false
	for _, entry := range service.ConnectionsIngress {
		if entry.SiteId == record.SiteId {
			entry.merge(record)
			found = true
		}
	}
	if !found {
		service.ConnectionsIngress = append(service.ConnectionsIngress, *record)
	}
}

func (service *TcpService) mergeEgress(record *TcpServiceEndpoints) {
	found := false
	for _, entry := range service.ConnectionsEgress {
		if entry.SiteId == record.SiteId {
			entry.merge(record)
			found = true
		}
	}
	if !found {
		service.ConnectionsEgress = append(service.ConnectionsEgress, *record)
	}
}

func (index TcpServiceMap) merge(services []TcpService) {
	for _, s := range services {
		if service, ok := index[s.Address]; ok {
			if s.Targets != nil {
				service.Targets = append(service.Targets, s.Targets...)
			}
			for _, ingress := range s.ConnectionsIngress {
				service.mergeIngress(&ingress)
			}
			for _, egress := range s.ConnectionsEgress {
				service.mergeEgress(&egress)
			}
			index[s.Address] = service
		} else {
			index[s.Address] = s
		}
	}
}

func (index TcpServiceMap) Update(siteId string, connections []qdr.TcpConnection, mapping NameMapping) {
	for _, c := range connections {
		record := TcpServiceEndpoints{
			SiteId: siteId,
			Connections: map[string]TcpConnectionStats{
				c.Name: asTcpConnectionStats(&c, mapping),
			},
		}
		service, ok := index[c.Address]
		if !ok {
			service = TcpService{
				Service: Service{
					Address:  c.Address,
					Protocol: "tcp",
				},
			}
		}
		if c.Direction == qdr.DirectionIn {
			service.mergeIngress(&record)
		} else {
			service.mergeEgress(&record)
		}
		index[c.Address] = service
	}
}

func (s TcpServiceMap) AddTargets(connectors []qdr.TcpEndpoint, mapping NameMapping) {
	for _, c := range connectors {
		service, ok := s[c.Address]
		if !ok {
			service = TcpService{
				Service: Service{
					Address:  c.Address,
					Protocol: "tcp",
				},
			}
		}
		service.AddTarget(c.Name, c.Host, c.SiteId, mapping)
		s[c.Address] = service
	}
}

func (s TcpServiceMap) AsList() []TcpService {
	list := []TcpService{}
	for _, v := range s {
		list = append(list, v)
	}
	return list
}

func GetTcpServices(siteId string, info [][]qdr.TcpConnection, targets []qdr.TcpEndpoint, lookup NameMapping) []TcpService {
	flattened := []qdr.TcpConnection{}
	for _, l := range info {
		flattened = append(flattened, l...)
	}
	services := TcpServiceMap{}
	services.Update(siteId, flattened, lookup)
	services.AddTargets(targets, lookup)
	return services.AsList()
}

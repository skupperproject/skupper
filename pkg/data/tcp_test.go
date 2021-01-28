package data

import (
	"github.com/skupperproject/skupper/pkg/qdr"
	"reflect"
	"testing"
)

func TestGetTcpServices(t *testing.T) {
	siteId := "mysite"
	connections := [][]qdr.TcpConnection{
		[]qdr.TcpConnection{
			qdr.TcpConnection{
				Name:      "a1",
				Host:      "1.1.1.1",
				Address:   "a",
				Direction: "in",
				BytesIn:   10,
				BytesOut:  20,
				Uptime:    60,
				LastIn:    5,
				LastOut:   4,
			},
			qdr.TcpConnection{
				Name:      "b1",
				Host:      "1.1.1.1",
				Address:   "b",
				Direction: "out",
				BytesIn:   15,
				BytesOut:  15,
				Uptime:    120,
				LastIn:    1,
				LastOut:   1,
			},
		},
		[]qdr.TcpConnection{
			qdr.TcpConnection{
				Name:      "a2",
				Host:      "2.2.2.2",
				Address:   "a",
				Direction: "in",
				BytesIn:   40,
				BytesOut:  80,
				Uptime:    90,
				LastIn:    10,
				LastOut:   9,
			},
		},
		[]qdr.TcpConnection{
			qdr.TcpConnection{
				Name:      "a1",
				Host:      "3.3.3.3",
				Address:   "a",
				Direction: "out",
				BytesIn:   20,
				BytesOut:  10,
				Uptime:    60,
				LastIn:    5,
				LastOut:   4,
			},
			qdr.TcpConnection{
				Name:      "a2",
				Host:      "4.4.4.4",
				Address:   "a",
				Direction: "out",
				BytesIn:   80,
				BytesOut:  40,
				Uptime:    90,
				LastIn:    9,
				LastOut:   10,
			},
			qdr.TcpConnection{
				Name:      "b1",
				Host:      "5.5.5.5",
				Address:   "b",
				Direction: "in",
				BytesIn:   15,
				BytesOut:  15,
				Uptime:    120,
				LastIn:    1,
				LastOut:   1,
			},
		},
	}
	targets := []qdr.TcpEndpoint{
		qdr.TcpEndpoint{
			Name:    "c1",
			Host:    "1.1.1.1",
			Address: "b",
			SiteId:  siteId,
		},
		qdr.TcpEndpoint{
			Name:    "c2",
			Host:    "3.3.3.3",
			Address: "a",
			SiteId:  siteId,
		},
		qdr.TcpEndpoint{
			Name:    "c3",
			Host:    "4.4.4.4",
			Address: "a",
			SiteId:  siteId,
		},
		qdr.TcpEndpoint{
			Name:    "c4",
			Host:    "6.6.6.6",
			Address: "c",
			SiteId:  siteId,
		},
	}
	mapping := GetTestMapping(map[string]string{})

	services := GetTcpServices(siteId, connections, targets, mapping)
	if services == nil {
		t.Errorf("Got nil services list")
	}
	expected := map[string]TcpService{
		"a": TcpService{
			Service: Service{
				Address:  "a",
				Protocol: "tcp",
				Targets: []ServiceTarget{
					ServiceTarget{
						Target: "c2",
						Name:   "3.3.3.3",
						SiteId: siteId,
					},
					ServiceTarget{
						Target: "c3",
						Name:   "4.4.4.4",
						SiteId: siteId,
					},
				},
			},
			ConnectionsIngress: []TcpServiceEndpoints{
				TcpServiceEndpoints{
					SiteId: siteId,
					Connections: map[string]TcpConnectionStats{
						"a1": TcpConnectionStats{
							Id:        "a1",
							StartTime: 60,
							LastOut:   4,
							LastIn:    5,
							BytesIn:   10,
							BytesOut:  20,
							Client:    "1.1.1.1",
						},
						"a2": TcpConnectionStats{
							Id:        "a2",
							StartTime: 90,
							LastOut:   9,
							LastIn:    10,
							BytesIn:   40,
							BytesOut:  80,
							Client:    "2.2.2.2",
						},
					},
				},
			},
			ConnectionsEgress: []TcpServiceEndpoints{
				TcpServiceEndpoints{
					SiteId: siteId,
					Connections: map[string]TcpConnectionStats{
						"a1": TcpConnectionStats{
							Id:        "a1",
							StartTime: 60,
							LastOut:   4,
							LastIn:    5,
							BytesIn:   20,
							BytesOut:  10,
							Server:    "3.3.3.3",
						},
						"a2": TcpConnectionStats{
							Id:        "a2",
							StartTime: 90,
							LastOut:   10,
							LastIn:    9,
							BytesIn:   80,
							BytesOut:  40,
							Server:    "4.4.4.4",
						},
					},
				},
			},
		},
		"b": TcpService{
			Service: Service{
				Address:  "b",
				Protocol: "tcp",
				Targets: []ServiceTarget{
					ServiceTarget{
						Target: "c1",
						Name:   "1.1.1.1",
						SiteId: siteId,
					},
				},
			},
			ConnectionsIngress: []TcpServiceEndpoints{
				TcpServiceEndpoints{
					SiteId: siteId,
					Connections: map[string]TcpConnectionStats{
						"b1": TcpConnectionStats{
							Id:        "b1",
							StartTime: 120,
							LastOut:   1,
							LastIn:    1,
							BytesIn:   15,
							BytesOut:  15,
							Client:    "5.5.5.5",
						},
					},
				},
			},
			ConnectionsEgress: []TcpServiceEndpoints{
				TcpServiceEndpoints{
					SiteId: siteId,
					Connections: map[string]TcpConnectionStats{
						"b1": TcpConnectionStats{
							Id:        "b1",
							StartTime: 120,
							LastOut:   1,
							LastIn:    1,
							BytesIn:   15,
							BytesOut:  15,
							Server:    "1.1.1.1",
						},
					},
				},
			},
		},
		"c": TcpService{
			Service: Service{
				Address:  "c",
				Protocol: "tcp",
				Targets: []ServiceTarget{
					ServiceTarget{
						Target: "c4",
						Name:   "6.6.6.6",
						SiteId: siteId,
					},
				},
			},
		},
	}
	if len(services) != len(expected) {
		t.Errorf("Expected %d services, got %d", len(expected), len(services))
	}
	for _, s := range services {
		e := expected[s.Address]
		if !reflect.DeepEqual(e.Service, s.Service) {
			t.Errorf("Incorrect service definition for %s; expected %v, got %v", s.Service.Address, e.Service, s.Service)
		}
		if !reflect.DeepEqual(e.ConnectionsIngress, s.ConnectionsIngress) {
			t.Errorf("Incorrect ingress connections for %s; expected %v, got %v", s.Service.Address, e.ConnectionsIngress, s.ConnectionsIngress)
		}
		if !reflect.DeepEqual(e.ConnectionsEgress, s.ConnectionsEgress) {
			t.Errorf("Incorrect egress connections for %s; expected %v, got %v", s.Service.Address, e.ConnectionsEgress, s.ConnectionsEgress)
		}
	}

}

package flow

import (
	"reflect"
	"strconv"
	"testing"

	"github.com/google/uuid"
	amqp "github.com/interconnectedcloud/go-amqp"
	"gotest.tools/assert"
)

func TestRecordDecoding(t *testing.T) {
	scenarios := []struct {
		name   string
		stype  reflect.Type
		fields map[int]interface{}
	}{
		{
			name:  "site",
			stype: reflect.TypeOf(SiteRecord{}),
			fields: map[int]interface{}{
				TypeOfRecord: uint32(Site),
				Identity:     uuid.New().String(),
				Name:         "skupper-site",
				Namespace:    "van-namespace",
			},
		},
		{
			name:  "router",
			stype: reflect.TypeOf(RouterRecord{}),
			fields: map[int]interface{}{
				TypeOfRecord: uint32(Router),
				Name:         "skupper-router",
				Namespace:    "van-namespace",
				ImageName:    "quay.io/skupper/skupper-router",
				ImageVersion: "main",
				HostName:     "172.56.92.10",
				BuildVersion: "2.0.2",
			},
		},
		{
			name:  "link",
			stype: reflect.TypeOf(LinkRecord{}),
			fields: map[int]interface{}{
				TypeOfRecord: uint32(Link),
				Mode:         "interior",
				Name:         "private-to-public-link",
				LinkCost:     uint64(20),
				Direction:    "incoming",
			},
		},
		{
			name:  "listener",
			stype: reflect.TypeOf(ListenerRecord{}),
			fields: map[int]interface{}{
				TypeOfRecord: uint32(Listener),
				Name:         "tcp-go-echo-listener",
				DestHost:     "172.21.122.169",
				DestPort:     "9090",
				Protocol:     "tcp",
				VanAddress:   " tcp-go-echo",
			},
		},
		{
			name:  "connector",
			stype: reflect.TypeOf(ConnectorRecord{}),
			fields: map[int]interface{}{
				TypeOfRecord: uint32(Connector),
				DestHost:     "172.21.122.169",
				DestPort:     "9090",
				Protocol:     "tcp",
				VanAddress:   " tcp-go-echo",
			},
		},
		{
			name:  "flow",
			stype: reflect.TypeOf(FlowRecord{}),
			fields: map[int]interface{}{
				TypeOfRecord:    uint32(Flow),
				CounterFlow:     uuid.New().String(),
				SourceHost:      "172.21.122.169",
				SourcePort:      "9090",
				Octets:          uint64(1234),
				Latency:         uint64(15),
				Trace:           "some-trace-string",
				ProcessIdentity: uuid.New().String(),
			},
		},
		{
			name:  "process",
			stype: reflect.TypeOf(ProcessRecord{}),
			fields: map[int]interface{}{
				TypeOfRecord: uint32(Process),
				Name:         "tcp-go-echo",
				ImageName:    "quay.io/skupper/tcp-go-echo",
				SourceHost:   "172.17.53.67",
				Group:        "tcp-go-echo",
				HostName:     "10.17.0.4",
			},
		},
		{
			name:  "host",
			stype: reflect.TypeOf(HostRecord{}),
			fields: map[int]interface{}{
				TypeOfRecord: uint32(Host),
				Name:         "10.17.0.5",
				Provider:     "ibm",
			},
		},
	}

	for _, s := range scenarios {
		var record []interface{}
		var msg amqp.Message
		var properties amqp.MessageProperties
		properties.Subject = "RECORD"
		msg.Properties = &properties

		m := make(map[interface{}]interface{})

		for k, v := range s.fields {
			m[uint32(k)] = v
		}

		record = append(record, m)
		msg.Value = record

		records := decode(&msg)
		assert.Equal(t, len(records), 1)
		for _, record := range records {
			assert.Equal(t, reflect.TypeOf(record), s.stype)
			switch record.(type) {
			case SiteRecord:
				site, ok := record.(SiteRecord)
				assert.Assert(t, ok)
				assert.Equal(t, site.Identity, s.fields[Identity])
				assert.Equal(t, *site.Name, s.fields[Name])
				assert.Equal(t, *site.NameSpace, s.fields[Namespace])
			case RouterRecord:
				router, ok := record.(RouterRecord)
				assert.Assert(t, ok)
				assert.Equal(t, *router.Name, s.fields[Name])
				assert.Equal(t, *router.Namespace, s.fields[Namespace])
				assert.Equal(t, *router.ImageName, s.fields[ImageName])
				assert.Equal(t, *router.ImageVersion, s.fields[ImageVersion])
				assert.Equal(t, *router.Hostname, s.fields[HostName])
				assert.Equal(t, *router.BuildVersion, s.fields[BuildVersion])
			case LinkRecord:
				link, ok := record.(LinkRecord)
				assert.Assert(t, ok)
				assert.Equal(t, *link.Name, s.fields[Name])
				assert.Equal(t, *link.Mode, s.fields[Mode])
				assert.Equal(t, *link.LinkCost, s.fields[LinkCost])
				assert.Equal(t, *link.Direction, s.fields[Direction])
			case ListenerRecord:
				listener, ok := record.(ListenerRecord)
				assert.Assert(t, ok)
				assert.Equal(t, *listener.Name, s.fields[Name])
				assert.Equal(t, *listener.DestHost, s.fields[DestHost])
				assert.Equal(t, *listener.DestPort, s.fields[DestPort])
				assert.Equal(t, *listener.Protocol, s.fields[Protocol])
				assert.Equal(t, *listener.Address, s.fields[VanAddress])
			case ConnectorRecord:
				connector, ok := record.(ConnectorRecord)
				assert.Assert(t, ok)
				assert.Equal(t, *connector.DestHost, s.fields[DestHost])
				assert.Equal(t, *connector.DestPort, s.fields[DestPort])
				assert.Equal(t, *connector.Protocol, s.fields[Protocol])
				assert.Equal(t, *connector.Address, s.fields[VanAddress])
			case FlowRecord:
				flow, ok := record.(FlowRecord)
				assert.Assert(t, ok)
				assert.Equal(t, *flow.CounterFlow, s.fields[CounterFlow])
				assert.Equal(t, *flow.SourceHost, s.fields[SourceHost])
				assert.Equal(t, *flow.SourcePort, s.fields[SourcePort])
				assert.Equal(t, *flow.Octets, s.fields[Octets])
				assert.Equal(t, *flow.Latency, s.fields[Latency])
				assert.Equal(t, *flow.Trace, s.fields[Trace])
			case HostRecord:
				host, ok := record.(HostRecord)
				assert.Assert(t, ok)
				assert.Equal(t, *host.Name, s.fields[Name])
				assert.Equal(t, *host.Provider, s.fields[Provider])
			case ProcessRecord:
				process, ok := record.(ProcessRecord)
				assert.Assert(t, ok)
				assert.Equal(t, *process.Name, s.fields[Name])
				assert.Equal(t, *process.ImageName, s.fields[ImageName])
				assert.Equal(t, *process.Group, s.fields[Group])
				assert.Equal(t, *process.HostName, s.fields[HostName])
				assert.Equal(t, *process.SourceHost, s.fields[SourceHost])
			}

		}

	}
}

func TestRecordEncoding(t *testing.T) {
	scenarios := []struct {
		name       string
		subject    string
		to         string
		properties map[string]string
		stype      reflect.Type
		fields     map[int]interface{}
	}{
		{
			name:    "router-beacon",
			subject: "BEACON",
			to:      "mc/sfe.all",
			stype:   reflect.TypeOf(BeaconRecord{}),
			properties: map[string]string{
				"Version":    "1",
				"SourceType": "ROUTER",
				"Address":    "sfe.gnjdr:0",
			},
		},
		{
			name:    "controller-beacon",
			subject: "BEACON",
			to:      "mc/sfe.all",
			stype:   reflect.TypeOf(BeaconRecord{}),
			properties: map[string]string{
				"Version":    "1",
				"SourceType": "CONTROLLER",
				"Address":    "sfe.gnjdr:0",
			},
		},
		{
			name:    "heartbeat",
			subject: "HEARTBEAT",
			to:      "mc/sfe.heartbeatid",
			stype:   reflect.TypeOf(HeartbeatRecord{}),
			properties: map[string]string{
				"Version":  "1",
				"Now":      "1662648292000000000",
				"Identity": "heartbeatid",
			},
		},
		{
			name:    "flush",
			subject: "FLUSH",
			to:      "mc/sfe.flushid",
			stype:   reflect.TypeOf(FlushRecord{}),
			properties: map[string]string{
				"ReplyTo": "replyToId",
			},
		},
		{
			name:  "site",
			stype: reflect.TypeOf(SiteRecord{}),
			fields: map[int]interface{}{
				TypeOfRecord: uint32(Site),
				Name:         "skupper-site",
				Namespace:    "van-namespace",
			},
		},
		{
			name:  "host",
			stype: reflect.TypeOf(HostRecord{}),
			fields: map[int]interface{}{
				TypeOfRecord: uint32(Host),
				Name:         "10.17.0.5",
				Provider:     "ibm",
			},
		},
		{
			name:  "process",
			stype: reflect.TypeOf(ProcessRecord{}),
			fields: map[int]interface{}{
				TypeOfRecord: uint32(Process),
				Name:         "tcp-go-echo",
				ImageName:    "quay.io/skupper/tcp-go-echo",
				SourceHost:   "172.17.53.67",
				Group:        "tcp-go-echo",
				HostName:     "10.17.0.4",
			},
		},
	}

	for _, s := range scenarios {
		switch s.stype {
		case reflect.TypeOf(BeaconRecord{}):
			scenarioBeacon := &BeaconRecord{}
			version, _ := strconv.Atoi(s.properties["Version"])
			scenarioBeacon.Version = uint32(version)
			scenarioBeacon.SourceType = s.properties["SourceType"]
			scenarioBeacon.Address = s.properties["Address"]

			msg, err := encodeBeacon(scenarioBeacon)
			assert.Assert(t, err)
			assert.Equal(t, msg.Properties.Subject, s.subject)
			assert.Equal(t, msg.Properties.To, s.to)
			assert.Equal(t, msg.ApplicationProperties["v"], uint32(version))
			assert.Equal(t, msg.ApplicationProperties["address"], s.properties["Address"])

			records := decode(msg)
			assert.Equal(t, len(records), 1)
			for _, record := range records {
				beacon, ok := record.(BeaconRecord)
				assert.Assert(t, ok)
				assert.Equal(t, beacon.Version, uint32(version))
				assert.Equal(t, beacon.SourceType, s.properties["SourceType"])
				assert.Equal(t, beacon.Address, s.properties["Address"])
			}
		case reflect.TypeOf(HeartbeatRecord{}):
			scenarioHeartbeat := &HeartbeatRecord{}
			version, _ := strconv.Atoi(s.properties["Version"])
			now, _ := strconv.Atoi(s.properties["Now"])
			scenarioHeartbeat.Version = uint32(version)
			scenarioHeartbeat.Now = uint64(now)
			scenarioHeartbeat.Identity = (s.properties["Identity"])

			msg, err := encodeHeartbeat(scenarioHeartbeat)
			assert.Assert(t, err)
			assert.Equal(t, msg.Properties.Subject, s.subject)
			assert.Equal(t, msg.Properties.To, s.to)
			assert.Equal(t, msg.ApplicationProperties["v"], uint32(version))
			assert.Equal(t, msg.ApplicationProperties["now"], uint64(now))
			assert.Equal(t, msg.ApplicationProperties["id"], s.properties["Identity"])

			records := decode(msg)
			assert.Equal(t, len(records), 1)
			for _, record := range records {
				heartbeat, ok := record.(HeartbeatRecord)
				assert.Assert(t, ok)
				assert.Equal(t, heartbeat.Version, uint32(version))
				assert.Equal(t, heartbeat.Now, uint64(now))
				assert.Equal(t, heartbeat.Identity, s.properties["Identity"])
			}
		case reflect.TypeOf(FlushRecord{}):
			scenarioFlush := &FlushRecord{}
			scenarioFlush.Address = s.to
			scenarioFlush.Source = s.properties["ReplyTo"]

			msg, err := encodeFlush(scenarioFlush)
			assert.Assert(t, err)
			assert.Equal(t, msg.Properties.Subject, s.subject)
			assert.Equal(t, msg.Properties.To, s.to)

			records := decode(msg)
			assert.Equal(t, len(records), 1)
		case reflect.TypeOf(SiteRecord{}):
			scenarioSite := &SiteRecord{
				Base: Base{
					RecType:  recordNames[Site],
					Identity: uuid.New().String(),
					Parent:   uuid.New().String(),
				},
			}
			if v, ok := s.fields[Name].(string); ok {
				scenarioSite.Name = &v
			}
			if v, ok := s.fields[Namespace].(string); ok {
				scenarioSite.NameSpace = &v
			}

			msg, err := encodeSite(scenarioSite)
			assert.Assert(t, err)
			assert.Equal(t, msg.Properties.Subject, "RECORD")
			sites, ok := msg.Value.([]interface{})
			assert.Assert(t, ok)
			assert.Equal(t, len(sites), 1)
			for _, site := range sites {
				_, ok := site.(map[interface{}]interface{})
				assert.Assert(t, ok)
				m := make(map[string]interface{})
				if r, ok := site.(map[interface{}]interface{}); ok {
					for k, v := range r {
						m[attributeNames[k.(uint32)]] = v
					}
					assert.Equal(t, int(m["TypeOfRecord"].(uint32)), Site)
					assert.Equal(t, m["Identity"].(string), scenarioSite.Identity)
					assert.Equal(t, m["Name"].(string), *scenarioSite.Name)
					assert.Equal(t, m["Namespace"].(string), *scenarioSite.NameSpace)
				}
			}
		case reflect.TypeOf(HostRecord{}):
			scenarioHost := &HostRecord{
				Base: Base{
					RecType:  recordNames[Host],
					Identity: uuid.New().String(),
					Parent:   uuid.New().String(),
				},
			}
			if v, ok := s.fields[Name].(string); ok {
				scenarioHost.Name = &v
			}
			if v, ok := s.fields[Provider].(string); ok {
				scenarioHost.Provider = &v
			}

			msg, err := encodeHost(scenarioHost)
			assert.Assert(t, err)
			assert.Equal(t, msg.Properties.Subject, "RECORD")
			hosts, ok := msg.Value.([]interface{})
			assert.Assert(t, ok)
			assert.Equal(t, len(hosts), 1)
			for _, host := range hosts {
				_, ok := host.(map[interface{}]interface{})
				assert.Assert(t, ok)
				m := make(map[string]interface{})
				if r, ok := host.(map[interface{}]interface{}); ok {
					for k, v := range r {
						m[attributeNames[k.(uint32)]] = v
					}
					assert.Equal(t, int(m["TypeOfRecord"].(uint32)), Host)
					assert.Equal(t, m["Identity"].(string), scenarioHost.Identity)
					assert.Equal(t, m["Parent"].(string), scenarioHost.Parent)
					assert.Equal(t, m["Name"].(string), *scenarioHost.Name)
					assert.Equal(t, m["Provider"].(string), *scenarioHost.Provider)
				}
			}
		case reflect.TypeOf(ProcessRecord{}):
			scenarioProcess := &ProcessRecord{
				Base: Base{
					RecType:  recordNames[Process],
					Identity: uuid.New().String(),
					Parent:   uuid.New().String(),
				},
			}
			if v, ok := s.fields[Name].(string); ok {
				scenarioProcess.Name = &v
			}
			if v, ok := s.fields[ImageName].(string); ok {
				scenarioProcess.ImageName = &v
			}
			if v, ok := s.fields[SourceHost].(string); ok {
				scenarioProcess.SourceHost = &v
			}
			if v, ok := s.fields[Group].(string); ok {
				scenarioProcess.Group = &v
			}
			if v, ok := s.fields[HostName].(string); ok {
				scenarioProcess.HostName = &v
			}

			msg, err := encodeProcess(scenarioProcess)
			assert.Assert(t, err)
			assert.Equal(t, msg.Properties.Subject, "RECORD")
			processes, ok := msg.Value.([]interface{})
			assert.Assert(t, ok)
			assert.Equal(t, len(processes), 1)
			for _, process := range processes {
				_, ok := process.(map[interface{}]interface{})
				assert.Assert(t, ok)
				m := make(map[string]interface{})
				if r, ok := process.(map[interface{}]interface{}); ok {
					for k, v := range r {
						m[attributeNames[k.(uint32)]] = v
					}
					assert.Equal(t, int(m["TypeOfRecord"].(uint32)), Process)
					assert.Equal(t, m["Identity"].(string), scenarioProcess.Identity)
					assert.Equal(t, m["Parent"].(string), scenarioProcess.Parent)
					assert.Equal(t, m["Name"].(string), *scenarioProcess.Name)
					assert.Equal(t, m["ImageName"].(string), *scenarioProcess.ImageName)
					assert.Equal(t, m["SourceHost"].(string), *scenarioProcess.SourceHost)
				}
			}
		}
	}
}

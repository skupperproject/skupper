package flow

import (
	"encoding/json"
	"log"
	"reflect"
	"strings"

	amqp "github.com/interconnectedcloud/go-amqp"
)

func asBeaconMessage(msg *amqp.Message) BeaconRecord {
	result := BeaconRecord{}
	if version, ok := msg.ApplicationProperties["v"].(uint32); ok {
		result.Version = version
	}
	if sourceType, ok := msg.ApplicationProperties["sourceType"].(string); ok {
		result.SourceType = sourceType
	}
	if address, ok := msg.ApplicationProperties["address"].(string); ok {
		result.Address = address
	}
	if direct, ok := msg.ApplicationProperties["direct"].(string); ok {
		result.Direct = direct
	}
	if identity, ok := msg.ApplicationProperties["id"].(string); ok {
		result.Identity = identity
	}
	return result
}

func encodeBeacon(beacon *BeaconRecord) (*amqp.Message, error) {
	encoded, err := json.Marshal(beacon)
	if err != nil {
		return nil, err
	}
	var request amqp.Message
	var properties amqp.MessageProperties
	properties.To = RecordPrefix + "all"
	properties.Subject = "BEACON"
	request.Properties = &properties
	request.ApplicationProperties = make(map[string]interface{})
	request.ApplicationProperties["v"] = beacon.Version
	request.ApplicationProperties["sourceType"] = beacon.SourceType
	request.ApplicationProperties["address"] = beacon.Address
	request.ApplicationProperties["direct"] = beacon.Direct
	request.ApplicationProperties["id"] = beacon.Identity

	request.Value = string(encoded)

	return &request, nil
}

func asHeartbeatMessage(msg *amqp.Message) HeartbeatRecord {
	result := HeartbeatRecord{}
	result.Source = strings.TrimPrefix(msg.Properties.To, "mc/")
	if version, ok := msg.ApplicationProperties["v"].(uint32); ok {
		result.Version = version
	}
	if now, ok := msg.ApplicationProperties["now"].(uint64); ok {
		result.Now = now
	}
	if identity, ok := msg.ApplicationProperties["id"].(string); ok {
		result.Identity = identity
	}
	return result
}

func encodeHeartbeat(heartbeat *HeartbeatRecord) (*amqp.Message, error) {
	var request amqp.Message
	var properties amqp.MessageProperties
	properties.To = RecordPrefix + heartbeat.Identity
	properties.Subject = "HEARTBEAT"
	request.Properties = &properties
	request.ApplicationProperties = make(map[string]interface{})
	request.ApplicationProperties["v"] = heartbeat.Version
	request.ApplicationProperties["now"] = heartbeat.Now
	request.ApplicationProperties["id"] = heartbeat.Identity

	return &request, nil
}

func asFlushMessage(msg *amqp.Message) FlushRecord {
	result := FlushRecord{Address: msg.Properties.To, Source: msg.Properties.ReplyTo}
	return result
}

func encodeFlush(fr *FlushRecord) (*amqp.Message, error) {
	var request amqp.Message
	var properties amqp.MessageProperties
	properties.To = fr.Address
	properties.Subject = "FLUSH"
	request.Properties = &properties

	return &request, nil
}

func encodeSite(site *SiteRecord) (*amqp.Message, error) {
	var record []interface{}
	var request amqp.Message
	var properties amqp.MessageProperties
	properties.Subject = "RECORD"
	properties.To = RecordPrefix + site.Identity
	request.Properties = &properties

	m := make(map[interface{}]interface{})
	m[uint32(TypeOfRecord)] = uint32(Site)
	m[uint32(Identity)] = site.Identity
	m[uint32(StartTime)] = site.StartTime
	if site.Name != nil {
		m[uint32(Name)] = *site.Name
	}
	if site.NameSpace != nil {
		m[uint32(Namespace)] = *site.NameSpace
	}
	if site.Platform != nil {
		m[uint32(Platform)] = *site.Platform
	}
	if site.Version != nil {
		m[uint32(Version)] = *site.Version
	}
	if site.Policy != nil {
		m[uint32(Policy)] = *site.Policy
	}
	record = append(record, m)

	request.Value = record

	return &request, nil
}

func encodeProcess(process *ProcessRecord) (*amqp.Message, error) {
	var record []interface{}
	var request amqp.Message
	var properties amqp.MessageProperties
	properties.Subject = "RECORD"
	properties.To = RecordPrefix + process.Parent
	request.Properties = &properties

	m := make(map[interface{}]interface{})
	m[uint32(TypeOfRecord)] = uint32(Process)
	m[uint32(Identity)] = process.Identity
	m[uint32(Parent)] = process.Parent
	m[uint32(StartTime)] = process.StartTime
	m[uint32(EndTime)] = process.EndTime
	if process.Name != nil {
		m[uint32(Name)] = *process.Name
	}
	if process.ImageName != nil {
		m[uint32(ImageName)] = *process.ImageName

	}
	if process.HostName != nil {
		m[uint32(HostName)] = *process.HostName
	}
	if process.SourceHost != nil {
		m[uint32(SourceHost)] = *process.SourceHost
	}
	if process.GroupName != nil {
		m[uint32(Group)] = *process.GroupName
	}
	if process.ProcessRole != nil {
		// note mapping ProcessRole to Mode
		m[uint32(Mode)] = *process.ProcessRole
	}

	record = append(record, m)

	request.Value = record

	return &request, nil
}

func encodeHost(host *HostRecord) (*amqp.Message, error) {
	var record []interface{}
	var request amqp.Message
	var properties amqp.MessageProperties
	properties.Subject = "RECORD"
	properties.To = RecordPrefix + host.Parent
	request.Properties = &properties

	m := make(map[interface{}]interface{})
	m[uint32(TypeOfRecord)] = uint32(Host)
	m[uint32(Identity)] = host.Identity
	m[uint32(Parent)] = host.Parent
	m[uint32(StartTime)] = host.StartTime
	m[uint32(EndTime)] = host.EndTime
	if host.Name != nil {
		m[uint32(Name)] = *host.Name
	}
	if host.Provider != nil {
		m[uint32(Provider)] = *host.Provider
	}
	if host.Platform != nil {
		m[uint32(Platform)] = *host.Platform
	}
	record = append(record, m)

	request.Value = record

	return &request, nil
}

func decode(msg *amqp.Message) []interface{} {
	var result []interface{}

	source := strings.TrimPrefix(msg.Properties.To, RecordPrefix)

	switch msg.Properties.Subject {
	case "BEACON":
		result = append(result, asBeaconMessage(msg))
	case "HEARTBEAT":
		result = append(result, asHeartbeatMessage(msg))
	case "FLUSH":
		result = append(result, asFlushMessage(msg))
	case "RECORD":
		if records, ok := msg.Value.([]interface{}); !ok {
			log.Printf("COLLECTOR: Unable to convert message of type %d to record list \n", reflect.TypeOf(msg.Value))
		} else {
			for _, record := range records {
				m := make(map[string]interface{})
				if r, ok := record.(map[interface{}]interface{}); ok {
					for k, v := range r {
						if k.(uint32) < uint32(len(attributeNames)) {
							m[attributeNames[k.(uint32)]] = v
						} else {
							log.Printf("COLLECTOR: Detected flow attribute out of range for record conversion %d \n ", k.(uint32))
						}
					}
				}
				var rt int
				if _, ok := m["TypeOfRecord"].(uint32); ok {
					rt = int(m["TypeOfRecord"].(uint32))
				}
				base := Base{
					RecType: recordNames[rt],
					Source:  source,
				}
				if v, ok := m["Identity"].(string); ok {
					base.Identity = v
				}
				if v, ok := m["Parent"].(string); ok {
					base.Parent = v
				}
				if v, ok := m["StartTime"].(uint64); ok {
					base.StartTime = v
				}
				if v, ok := m["EndTime"].(uint64); ok {
					base.EndTime = v
				}

				switch rt {
				case Site:
					site := SiteRecord{
						Base: base,
					}
					if v, ok := m["Location"].(string); ok {
						site.Location = &v
					}
					if v, ok := m["Provider"].(string); ok {
						site.Provider = &v
					}
					if v, ok := m["Platform"].(string); ok {
						site.Platform = &v
					}
					if v, ok := m["Name"].(string); ok {
						site.Name = &v
					}
					if v, ok := m["Namespace"].(string); ok {
						site.NameSpace = &v
					}
					if v, ok := m["Version"].(string); ok {
						site.Version = &v
					}
					if v, ok := m["Policy"].(string); ok {
						site.Policy = &v
					}
					result = append(result, site)
				case Host:
					host := HostRecord{
						Base: base,
					}
					if v, ok := m["Name"].(string); ok {
						host.Name = &v
					}
					if v, ok := m["Provider"].(string); ok {
						host.Provider = &v
					}
					result = append(result, host)
				case Router:
					router := RouterRecord{
						Base: base,
					}
					if v, ok := m["Mode"].(string); ok {
						router.Mode = &v
					}
					if v, ok := m["Name"].(string); ok {
						router.Name = &v
					}
					if v, ok := m["Namespace"].(string); ok {
						router.Namespace = &v
					}
					if v, ok := m["ImageName"].(string); ok {
						router.ImageName = &v
					}
					if v, ok := m["ImageVersion"].(string); ok {
						router.ImageVersion = &v
					}
					if v, ok := m["HostName"].(string); ok {
						router.Hostname = &v
					}
					if v, ok := m["BuildVersion"].(string); ok {
						router.BuildVersion = &v
					}
					result = append(result, router)
				case Link:
					link := LinkRecord{
						Base: base,
					}
					if v, ok := m["Mode"].(string); ok {
						link.Mode = &v
					}
					if v, ok := m["Name"].(string); ok {
						link.Name = &v
					}
					if v, ok := m["LinkCost"].(uint64); ok {
						link.LinkCost = &v
					}
					if v, ok := m["Direction"].(string); ok {
						link.Direction = &v
					}
					if v, ok := m["LinkName"].(string); ok {
						link.LinkName = &v
					}

					result = append(result, link)
				case Listener:
					listener := ListenerRecord{
						Base: base,
					}
					if v, ok := m["Name"].(string); ok {
						listener.Name = &v
					}
					if v, ok := m["DestHost"].(string); ok {
						listener.DestHost = &v
					}
					if v, ok := m["DestPort"].(string); ok {
						listener.DestPort = &v
					}
					if v, ok := m["Protocol"].(string); ok {
						listener.Protocol = &v
					}
					if v, ok := m["VanAddress"].(string); ok {
						listener.Address = &v
					}
					if v, ok := m["FlowCountL4"].(uint64); ok {
						listener.FlowCountL4 = &v
					}
					if v, ok := m["FlowRateL4"].(uint64); ok {
						listener.FlowRateL4 = &v
					}
					if v, ok := m["FlowCountL7"].(uint64); ok {
						listener.FlowCountL7 = &v
					}
					if v, ok := m["FlowRateL7"].(uint64); ok {
						listener.FlowRateL7 = &v
					}
					result = append(result, listener)
				case Connector:
					connector := ConnectorRecord{
						Base: base,
					}
					if v, ok := m["DestHost"].(string); ok {
						connector.DestHost = &v
					}
					if v, ok := m["DestPort"].(string); ok {
						connector.DestPort = &v
					}
					if v, ok := m["Protocol"].(string); ok {
						connector.Protocol = &v
					}
					if v, ok := m["VanAddress"].(string); ok {
						connector.Address = &v
					}
					if v, ok := m["FlowCountL4"].(uint64); ok {
						connector.FlowCountL4 = &v
					}
					if v, ok := m["FlowRateL4"].(uint64); ok {
						connector.FlowRateL4 = &v
					}
					if v, ok := m["FlowCountL7"].(uint64); ok {
						connector.FlowCountL7 = &v
					}
					if v, ok := m["FlowRateL7"].(uint64); ok {
						connector.FlowRateL7 = &v
					}
					if v, ok := m["Target"].(string); ok {
						connector.Target = &v
					}
					result = append(result, connector)
				case LogEvent:
					logEvent := LogEventRecord{
						Base: base,
					}
					if v, ok := m["LogSeverity"].(uint64); ok {
						logEvent.LogSeverity = &v
					}
					if v, ok := m["LogText"].(string); ok {
						logEvent.LogText = &v
					}
					if v, ok := m["SourceFile"].(string); ok {
						logEvent.SourceFile = &v
					}
					if v, ok := m["SourceLine"].(uint64); ok {
						logEvent.SourceLine = &v
					}
					result = append(result, logEvent)
				case Flow:
					flow := FlowRecord{
						Base: base,
					}
					if v, ok := m["SourceHost"].(string); ok {
						flow.SourceHost = &v
					}
					if v, ok := m["SourcePort"].(string); ok {
						flow.SourcePort = &v
					}
					if v, ok := m["CounterFlow"].(string); ok {
						flow.CounterFlow = &v
					}
					if v, ok := m["Trace"].(string); ok {
						flow.Trace = &v
					}
					if v, ok := m["Latency"].(uint64); ok {
						flow.Latency = &v
					}
					if v, ok := m["Octets"].(uint64); ok {
						flow.Octets = &v
					}
					if v, ok := m["OctetsOut"].(uint64); ok {
						flow.OctetsOut = &v
					}
					if v, ok := m["OctetsUnacked"].(uint64); ok {
						flow.OctetsUnacked = &v
					}
					if v, ok := m["WindowClosures"].(uint64); ok {
						flow.WindowClosures = &v
					}
					if v, ok := m["WindowSize"].(uint64); ok {
						flow.WindowSize = &v
					}
					if v, ok := m["Reason"].(string); ok {
						flow.Reason = &v
					}
					if v, ok := m["Method"].(string); ok {
						flow.Method = &v
					}
					if v, ok := m["Result"].(string); ok {
						flow.Result = &v
					}
					if v, ok := m["StreamIdentity"].(uint64); ok {
						flow.StreamIdentity = &v
					}
					result = append(result, flow)
				case Process:
					process := ProcessRecord{
						Base: base,
					}
					if v, ok := m["Name"].(string); ok {
						process.Name = &v
					}
					if v, ok := m["ImageName"].(string); ok {
						process.ImageName = &v
					}
					if v, ok := m["Image"].(string); ok {
						process.Image = &v
					}
					if v, ok := m["Group"].(string); ok {
						process.GroupName = &v
					}
					if v, ok := m["GroupIdentity"].(string); ok {
						process.GroupIdentity = &v
					}
					if v, ok := m["HostName"].(string); ok {
						process.HostName = &v
					}
					if v, ok := m["SourceHost"].(string); ok {
						process.SourceHost = &v
					}
					if v, ok := m["Mode"].(string); ok {
						// note mapping Mode to ProcessRole
						process.ProcessRole = &v
					}
					result = append(result, process)
				default:
					log.Println("Unrecognized record type", rt)
				}
			}
		}
	default:
		log.Println("Unrecognized message subject")
	}
	return result
}

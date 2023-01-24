package flow

import (
	"fmt"
	"reflect"
	"sort"
	"strings"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

// RecordTypes
const (
	Site             = iota // 0
	Router                  // 1
	Link                    // 2
	Controller              // 3
	Listener                // 4
	Connector               // 5
	Flow                    // 6
	Process                 // 7
	Image                   // 8
	Ingress                 // 9
	Egress                  // 10
	Collector               // 11
	ProcessGroup            // 12
	Host                    // 13
	FlowPair                // 14 (generated)
	FlowAggregate           // 15
	EventSource             // 16
	SitePair                // 17
	ProcessGroupPair        // 18
	ProcessPair             // 19
	Address                 // 20
)

var recordNames = []string{
	"SITE",
	"ROUTER",
	"LINK",
	"CONTROLLER",
	"LISTENER",
	"CONNECTOR",
	"FLOW",
	"PROCESS",
	"IMAGE",
	"INGRESS",
	"EGRESS",
	"COLLECTOR",
	"PROCESS_GROUP",
	"HOST",
	"FLOWPAIR",
	"FLOWAGGREGATE",
	"EVENTSOURCE",
	"SITEPAIR",
	"PROCESSGROUPPAIR",
	"PROCESSPAIR",
	"ADDRESS",
}

// Attribute Types
const (
	TypeOfRecord    = iota //0
	Identity               // 1
	Parent                 // 2
	StartTime              // 3
	EndTime                // 4
	CounterFlow            // 5
	PeerIdentity           // 6
	ProcessIdentity        // 7
	SiblingOrdinal         // 8
	Location               // 9
	Provider               // 10
	Platform               // 11
	Namespace              // 12
	Mode                   // 13
	SourceHost             // 14
	DestHost               // 15
	Protocol               // 16
	SourcePort             // 17
	DestPort               // 18
	VanAddress             // 19
	ImageName              // 20
	ImageVersion           // 21
	HostName               // 22
	Octets                 // 23
	Latency                // 24
	TransitLatency         // 25
	Backlog                // 26
	Method                 // 27
	Result                 // 28
	Reason                 // 29
	Name                   // 30
	Trace                  // 31
	BuildVersion           // 32
	LinkCost               // 33
	Direction              // 34
	OctetRate              // 35
	OctetsOut              // 36
	OctetsUnacked          // 37
	WindowClosures         // 38
	WindowSize             // 39
	FlowCountL4            // 40
	FlowCountL7            // 41
	FlowRateL4             // 42
	FlowRateL7             // 43
	Duration               // 44
	ImageAttr              // 45
	Group                  // 46
	StreamIdentity         // 47
)

var attributeNames = []string{
	"TypeOfRecord",
	"Identity",
	"Parent",
	"StartTime",
	"EndTime",         // 4
	"CounterFlow",     // 5
	"PeerIdentity",    // 6
	"ProcessIdentity", // 7
	"SiblingOrdinal",  // 8
	"Location",        // 9
	"Provider",        // 10
	"Platform",        // 11
	"Namespace",       // 12
	"Mode",            // 13
	"SourceHost",      // 14
	"DestHost",        // 15
	"Protocol",        // 16
	"SourcePort",      // 17
	"DestPort",        // 18
	"VanAddress",      // 19
	"ImageName",       // 20
	"ImageVersion",    // 21
	"HostName",        // 22
	"Octets",          // 23
	"Latency",         // 24
	"TransitLatency",  // 25
	"Backlog",         // 26
	"Method",          // 27
	"Result",          // 28
	"Reason",          // 29
	"Name",            // 30
	"Trace",           // 31
	"BuildVersion",    // 32
	"LinkCost",        // 33
	"Direction",       // 34
	"OctetRate",       // 35
	"OctetsOut",       // 36
	"OctetsUnacked",   // 37
	"WindowClosures",  // 38
	"WindowSize",      // 39
	"FlowCountL4",     // 40
	"FlowCountL7",     // 41
	"FlowRateL4",      // 42
	"FlowRateL7",      // 43
	"Duration",        // 44
	"Image",           // 45
	"Group",           // 46
	"StreamIdentity",  // 47
}

type Payload struct {
	Results       interface{} `json:"results,omitempty"`
	Timestamp     uint64      `json:"timestamp,omitempty"`
	LengthTotal   int         `json:"lengthTotal,omitempty"`
	LengthResults int         `json:"lengthResults,omitempty"`
}

type Base struct {
	RecType   string `json:"recType,omitempty"`
	Identity  string `json:"identity,omitempty"`
	Parent    string `json:"parent,omitempty"`
	StartTime uint64 `json:"startTime,omitempty"`
	EndTime   uint64 `json:"endTime,omitempty"`
}

type BeaconRecord struct {
	Version    uint32 `json:"version,omitempty"`
	SourceType string `json:"sourceType,omitempty"`
	Address    string `json:"address,omitempty"`
	Direct     string `json:"direct,omitempty"`
	Now        uint64 `json:"now,omitempty"`
	Identity   string `json:"identity,omitempty"`
}

type HeartbeatRecord struct {
	Source   string `json:"source,omityempty"`
	Identity string `json:"identity,omitempty"`
	Version  uint32 `json:"version,omitempty"`
	Now      uint64 `json:"now,omitempty"`
}

type FlushRecord struct {
	Address string `json:"address,omitempty"`
	Source  string `json:"source,omitempty"`
}

type SiteRecord struct {
	Base
	Location           *string `json:"location,omitempty"`
	Provider           *string `json:"provider,omitempty"`
	Platform           *string `json:"platform,omitempty"`
	Name               *string `json:"name,omitempty"`
	NameSpace          *string `json:"nameSpace,omitempty"`
	OctetsSent         uint64  `json:"octetsSent"`
	OctetsSentRate     uint64  `json:"octetSentRate"`
	OctetsReceived     uint64  `json:"octetsReceived"`
	OctetsReceivedRate uint64  `json:"octetReceivedRate"`
}

type HostRecord struct {
	Base
	Location          *string `json:"location,omitempty"`
	Provider          *string `json:"provider,omitempty"`
	Platform          *string `json:"platform,omitempty"`
	Name              *string `json:"name,omitempty"`
	Arch              *string `json:"arch,omitempty"`
	OperatingSystem   *string `json:"operatingSystem,omitempty"`
	OperatingSystemId *string `json:"operatingSystemId,omitempty"`
	Region            *string `json:"region,omitempty"`
	Zone              *string `json:"zone,omitempty"`
	ContainerRuntime  *string `json:"containerRuntime,omitempty"`
	KernelVersion     *string `json:"kernelVersion,omitempty"`
	KubeProxyVersion  *string `json:"kubeProxyVersion,omitempty"`
	KubeletVersion    *string `json:"kubeletVersion,omitempty"`
}

type RouterRecord struct {
	Base
	Name         *string `json:"name,omitempty"`
	Namespace    *string `json:"namespace,omitempty"`
	Mode         *string `json:"mode,omitempty"`
	ImageName    *string `json:"imageName,omitempty"`
	ImageVersion *string `json:"imageVersion,omitempty"`
	Hostname     *string `json:"hostname,omitempty"`
	BuildVersion *string `json:"buildVersion,omitempty"`
}

type LinkRecord struct {
	Base
	Mode      *string `json:"mode,omitempty"`
	Name      *string `json:"name,omitempty"`
	LinkCost  *uint64 `json:"linkCost,omitempty"`
	Direction *string `json:"direction,omitempty"`
}

type ListenerRecord struct {
	Base
	Name        *string `json:"name,omitempty"`
	DestHost    *string `json:"destHost,omitempty"`
	DestPort    *string `json:"destPort,omitempty"`
	Protocol    *string `json:"protocol,omitempty"`
	Address     *string `json:"address,omitempty"`
	FlowCountL4 *uint64 `json:"flowCountL4,omitempty"`
	FlowRateL4  *uint64 `json:"flowRateL4,omitempty"`
	FlowCountL7 *uint64 `json:"flowCountL7,omitempty"`
	FlowRateL7  *uint64 `json:"flowRateL7,omitempty"`
}

type ConnectorRecord struct {
	Base
	DestHost    *string `json:"destHost,omitempty"`
	DestPort    *string `json:"destPort,omitempty"`
	Protocol    *string `json:"protocol,omitempty"`
	Address     *string `json:"address,omitempty"`
	FlowCountL4 *uint64 `json:"flowCountL4,omitempty"`
	FlowRateL4  *uint64 `json:"flowRateL4,omitempty"`
	FlowCountL7 *uint64 `json:"flowCountL7,omitempty"`
	FlowRateL7  *uint64 `json:"flowRateL7,omitempty"`
	process     *string
}

// Van Address represents a service that is attached to the application network
type VanAddressRecord struct {
	Base
	Name           string `json:"name,omitempty"`
	ListenerCount  int    `json:"listenerCount"`
	ConnectorCount int    `json:"connectorCount"`
	TotalFlows     int    `json:"totalFlows"`
	CurrentFlows   int    `json:"currentFlows"`
}

type ProcessRecord struct {
	Base
	Name               *string `json:"name,omitempty"`
	ParentName         *string `json:"parentName,omitempty"`
	ImageName          *string `json:"imageName,omitempty"`
	Image              *string `json:"image,omitempty"`
	GroupName          *string `json:"groupName,omitempty"`
	GroupIdentity      *string `json:"groupIdentity,omitempty"`
	HostName           *string `json:"hostName,omitempty"`
	SourceHost         *string `json:"sourceHost,omitempty"`
	OctetsSent         uint64  `json:"octetsSent"`
	OctetsSentRate     uint64  `json:"octetSentRate"`
	OctetsReceived     uint64  `json:"octetsReceived"`
	OctetsReceivedRate uint64  `json:"octetReceivedRate"`
	connector          *string
}

type ProcessGroupRecord struct {
	Base
	Name               *string `json:"name,omitempty"`
	OctetsSent         uint64  `json:"octetsSent"`
	OctetsSentRate     uint64  `json:"octetSentRate"`
	OctetsReceived     uint64  `json:"octetsReceived"`
	OctetsReceivedRate uint64  `json:"octetReceivedRate"`
}

type FlowRecord struct {
	Base
	SourceHost     *string `json:"sourceHost,omitempty"`
	SourcePort     *string `json:"sourcePort,omitempty"`
	CounterFlow    *string `json:"counterFlow,omitempty"`
	Trace          *string `json:"trace,omitempty"`
	Latency        *uint64 `json:"latency,omitempty"`
	Octets         *uint64 `json:"octets"`
	OctetRate      *uint64 `json:"octetRate"`
	OctetsOut      *uint64 `json:"octetsOut,omitempty"`
	OctetsUnacked  *uint64 `json:"octetsUnacked,omitempty"`
	WindowClosures *uint64 `json:"windowClosures,omitempty"`
	WindowSize     *uint64 `json:"windowSize,omitempty"`
	Reason         *string `json:"reason,omitempty"`
	Method         *string `json:"method,omitempty"`
	Result         *string `json:"result,omitempty"`
	Process        *string `json:"process,omitempty"`
	ProcessName    *string `json:"processName,omitempty"`
}

// Note a flowpair does not have a defined parent relationship through Base
type FlowPairRecord struct {
	Base
	SourceSiteId            string      `json:"sourceSiteId,omitempty"`
	DestinationSiteId       string      `json:"destinationSiteId,omitempty"`
	ForwardFlow             *FlowRecord `json:"forwardFlow,omitempty"`
	CounterFlow             *FlowRecord `json:"counterFlow,omitempty"`
	SiteAggregateId         *string     `json:"siteAggregateId,omitempty"`
	ProcessGroupAggregateId *string     `json:"processGroupAggregateId,omitempty"`
	ProcessAggregateId      *string     `json:"processAggregateId,omitempty"`
}

type FlowAggregateRecord struct {
	Base
	PairType                  string  `json:"pairType,omitempty"`
	RecordCount               uint64  `json:"recordCount,omitempty"`
	SourceId                  *string `json:"sourceId,omitempty"`
	SourceName                *string `json:"sourceName,omitempty"`
	DestinationId             *string `json:"destinationId,omitempty"`
	DestinationName           *string `json:"destinationName,omitempty"`
	SourceOctets              uint64  `json:"sourceOctets,omitempty"`
	SourceOctetRate           uint64  `json:"sourceOctetRate,omitempty"`
	SourceMinLatency          uint64  `json:"sourceMinLatency,omitempty"`
	SourceMaxLatency          uint64  `json:"sourceMaxLatency,omitempty"`
	SourceAverageLatency      uint64  `json:"sourceAverageLatency,omitempty"`
	DestinationOctets         uint64  `json:"destinationOctets,omitempty"`
	DestinationOctetRate      uint64  `json:"destinationOctetRate,omitempty"`
	DestinationMinLatency     uint64  `json:"destinationMinLatency,omitempty"`
	DestinationMaxLatency     uint64  `json:"destinationMaxLatency,omitempty"`
	DestinationAverageLatency uint64  `json:"destinationAverageLatency,omitempty"`
}

func min(a, b uint64) uint64 {
	if a < b {
		return a
	}
	return b
}

func max(a, b uint64) uint64 {
	if a > b {
		return a
	}
	return b
}

type ControllerRecord struct {
	base
	ImageName    string `json:"imageName,omitempty"`
	ImageVersion string `json:"imageVersion,omitempty"`
	Hostname     string `json:"hostame,omitempty"`
	Name         string `json:"name,omitempty"`
	BuildVersion string `json:"buildVersion,omitempty"`
}

type ImageRecord struct {
	Base
	Name    string `json:"name,omitempty"`
	Version string `json:"version,omitempty"`
	// signature, url/rep, id??
}

type IngressRecord struct {
	Base
}

type EgressRecord struct {
	Base
}

type CollectorRecord struct {
	Base
	//	name, kind, process
}

// Convert a slice or array of a specific type to array of interface{}
func ToIntf(s interface{}) []interface{} {
	v := reflect.ValueOf(s)
	// There is no need to check, we want to panic if it's not slice or array
	intf := make([]interface{}, v.Len())
	for i := 0; i < v.Len(); i++ {
		intf[i] = v.Index(i).Interface()
	}
	return intf
}

func paginate(offset int, limit int, length int) (int, int) {
	start := offset
	if start < 0 {
		start = 0
	} else if start > length {
		start = length
	}

	if limit < 0 {
		limit = length
	}
	end := start + limit
	if end > length {
		end = length
	}

	return start, end
}

func validateAndReturnSortQuery(sortBy string) (string, string, error) {
	parts := strings.Split(sortBy, ".")
	if len(parts) != 2 {
		return "", "", fmt.Errorf("Malformed sortBy query parameter")
	}
	field, order := parts[0], parts[1]
	if order != "asc" && order != "desc" {
		return "", "", fmt.Errorf("Malformed order direction in sortBy query parameter, should be asc or desc")
	}
	return field, order, nil
}

func getField(field string, record interface{}) interface{} {
	x := reflect.ValueOf(record)
	if x.Kind() == reflect.Struct {
		switch x.FieldByName(field).Kind() {
		case reflect.String:
			return x.FieldByName(field).String()
		case reflect.Ptr:
			elem := x.FieldByName(field).Elem()
			switch elem.Kind() {
			case reflect.String:
				return fmt.Sprintf("%s", (x.FieldByName(field).Elem().Interface()))
			case reflect.Uint64:
				return x.FieldByName(field).Elem().Uint()
			}
		case reflect.Int:
			return x.FieldByName(field).Int()
		case reflect.Uint64:
			return x.FieldByName(field).Uint()
		default:
			return nil
		}
	} else {
		return nil
	}
	return nil
}

func compareFields(x, y interface{}, order string) bool {
	if x != nil && y != nil {
		switch x.(type) {
		case string:
			if order == "asc" {
				return x.(string) < y.(string)
			} else {
				return x.(string) > y.(string)
			}
		case uint64:
			if order == "asc" {
				return x.(uint64) < y.(uint64)
			} else {
				return x.(uint64) > y.(uint64)
			}
		case int32:
			if order == "asc" {
				return x.(int32) < y.(int32)
			} else {
				return x.(int32) > y.(int32)
			}
		case int64:
			if order == "asc" {
				return x.(int64) < y.(int64)
			} else {
				return x.(int64) > y.(int64)
			}
		case int:
			if order == "asc" {
				return x.(int) < y.(int)
			} else {
				return x.(int) > y.(int)
			}
		default:
			return false
		}
	} else {
		return false
	}
}

func sortAndFilter(list interface{}, sortBy string, offset, limit int) ([]interface{}, error) {
	// tag to field name
	sortBy = cases.Title(language.Und, cases.NoLower).String(sortBy)
	field, order, err := validateAndReturnSortQuery(sortBy)
	if err != nil {
		return nil, err
	}
	switch list.(type) {
	case []SiteRecord:
		sites := list.([]SiteRecord)
		sort.Slice(sites, func(i, j int) bool {
			return compareFields(getField(field, sites[i]), getField(field, sites[j]), order)
		})
		start, end := paginate(offset, limit, len(sites))
		return ToIntf(sites[start:end]), nil
	case []HostRecord:
		hosts := list.([]HostRecord)
		sort.Slice(hosts, func(i, j int) bool {
			return compareFields(getField(field, hosts[i]), getField(field, hosts[j]), order)
		})
		start, end := paginate(offset, limit, len(hosts))
		return ToIntf(hosts[start:end]), nil
	case []RouterRecord:
		routers := list.([]RouterRecord)
		sort.Slice(routers, func(i, j int) bool {
			return compareFields(getField(field, routers[i]), getField(field, routers[j]), order)
		})
		start, end := paginate(offset, limit, len(routers))
		return ToIntf(routers[start:end]), nil
	case []LinkRecord:
		links := list.([]LinkRecord)
		sort.Slice(links, func(i, j int) bool {
			return compareFields(getField(field, links[i]), getField(field, links[j]), order)
		})
		start, end := paginate(offset, limit, len(links))
		return ToIntf(links[start:end]), nil
	case []ListenerRecord:
		listeners := list.([]ListenerRecord)
		sort.Slice(listeners, func(i, j int) bool {
			return compareFields(getField(field, listeners[i]), getField(field, listeners[j]), order)
		})
		start, end := paginate(offset, limit, len(listeners))
		return ToIntf(listeners[start:end]), nil
	case []ConnectorRecord:
		connectors := list.([]ConnectorRecord)
		sort.Slice(connectors, func(i, j int) bool {
			return compareFields(getField(field, connectors[i]), getField(field, connectors[j]), order)
		})
		start, end := paginate(offset, limit, len(connectors))
		return ToIntf(connectors[start:end]), nil
	case []VanAddressRecord:
		addresses := list.([]VanAddressRecord)
		sort.Slice(addresses, func(i, j int) bool {
			return compareFields(getField(field, addresses[i]), getField(field, addresses[j]), order)
		})
		start, end := paginate(offset, limit, len(addresses))
		return ToIntf(addresses[start:end]), nil
	case []ProcessRecord:
		processes := list.([]ProcessRecord)
		sort.Slice(processes, func(i, j int) bool {
			return compareFields(getField(field, processes[i]), getField(field, processes[j]), order)
		})
		start, end := paginate(offset, limit, len(processes))
		return ToIntf(processes[start:end]), nil
	case []ProcessGroupRecord:
		processGroups := list.([]ProcessGroupRecord)
		sort.Slice(processGroups, func(i, j int) bool {
			return compareFields(getField(field, processGroups[i]), getField(field, processGroups[j]), order)
		})
		start, end := paginate(offset, limit, len(processGroups))
		return ToIntf(processGroups[start:end]), nil
	case []FlowRecord:
		flows := list.([]FlowRecord)
		sort.Slice(flows, func(i, j int) bool {
			return compareFields(getField(field, flows[i]), getField(field, flows[j]), order)
		})
		start, end := paginate(offset, limit, len(flows))
		return ToIntf(flows[start:end]), nil
	case []FlowPairRecord:
		flowPairs := list.([]FlowPairRecord)
		sort.Slice(flowPairs, func(i, j int) bool {
			return compareFields(getField(field, flowPairs[i]), getField(field, flowPairs[j]), order)
		})
		start, end := paginate(offset, limit, len(flowPairs))
		return ToIntf(flowPairs[start:end]), nil
	case []FlowAggregateRecord:
		flowAggregates := list.([]FlowAggregateRecord)
		sort.Slice(flowAggregates, func(i, j int) bool {
			return compareFields(getField(field, flowAggregates[i]), getField(field, flowAggregates[j]), order)
		})
		start, end := paginate(offset, limit, len(flowAggregates))
		return ToIntf(flowAggregates[start:end]), nil
	case []eventSource:
		eventSources := list.([]eventSource)
		sort.Slice(eventSources, func(i, j int) bool {
			return eventSources[i].Beacon.Address < eventSources[j].Beacon.Address
		})
		start, end := paginate(offset, limit, len(eventSources))
		return ToIntf(eventSources[start:end]), nil
	}
	return nil, fmt.Errorf("Unrecognized list type to filter %T", list)
}

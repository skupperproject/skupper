package flow

import (
	"fmt"
	"net/url"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
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

var Internal string = "internal"
var External string = "external"
var Incoming string = "incoming"
var Outgoing string = "outgoing"

type Base struct {
	RecType   string `json:"recType,omitempty"`
	Identity  string `json:"identity,omitempty"`
	Parent    string `json:"parent,omitempty"`
	StartTime uint64 `json:"startTime"`
	EndTime   uint64 `json:"endTime"`
}

type BeaconRecord struct {
	Version    uint32 `json:"version,omitempty"`
	SourceType string `json:"sourceType,omitempty"`
	Address    string `json:"address,omitempty"`
	Direct     string `json:"direct,omitempty"`
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

type EventSourceRecord struct {
	Base
	Beacon     *BeaconRecord `json:"beacon,omitempty"`
	LastHeard  uint64        `json:"lastHeard,omitempty"`
	Heartbeats int           `json:"heartbeats,omitempty"`
	Beacons    int           `json:"beacons,omitempty"`
}

type SiteRecord struct {
	Base
	Location  *string `json:"location,omitempty"`
	Provider  *string `json:"provider,omitempty"`
	Platform  *string `json:"platform,omitempty"`
	Name      *string `json:"name,omitempty"`
	NameSpace *string `json:"nameSpace,omitempty"`
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
	AddressId   *string `json:"addressId,omitempty"`
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
	AddressId   *string `json:"addressId,omitempty"`
}

type metricKey struct {
	sourceSite    string
	sourceProcess string
	destSite      string
	destProcess   string
}

// Van Address represents a service that is attached to the application network
type VanAddressRecord struct {
	Base
	Name            string `json:"name,omitempty"`
	Protocol        string `json:"protocol,omitempty"`
	ListenerCount   int    `json:"listenerCount"`
	ConnectorCount  int    `json:"connectorCount"`
	flowCount       map[metricKey]prometheus.Counter
	octetCount      map[metricKey]prometheus.Counter
	lastAccessed    map[metricKey]prometheus.Gauge
	flowLatency     map[metricKey]prometheus.Observer
	activeFlowCount map[metricKey]prometheus.Gauge
}

type ProcessRecord struct {
	Base
	Name          *string `json:"name,omitempty"`
	ParentName    *string `json:"parentName,omitempty"`
	ImageName     *string `json:"imageName,omitempty"`
	Image         *string `json:"image,omitempty"`
	GroupName     *string `json:"groupName,omitempty"`
	GroupIdentity *string `json:"groupIdentity,omitempty"`
	HostName      *string `json:"hostName,omitempty"`
	SourceHost    *string `json:"sourceHost,omitempty"`
	ProcessRole   *string `json:"processRole,omitempty"`
	connector     *string
}

type ProcessGroupRecord struct {
	Base
	Name             *string `json:"name,omitempty"`
	ProcessGroupRole *string `json:"processGroupRole,omitempty"`
}

type FlowPlace int

const (
	unknown    FlowPlace = iota
	clientSide           // forward flow
	serverSide           // counter flow
)

type FlowRecord struct {
	Base
	SourceHost       *string   `json:"sourceHost,omitempty"`
	SourcePort       *string   `json:"sourcePort,omitempty"`
	CounterFlow      *string   `json:"counterFlow,omitempty"`
	Trace            *string   `json:"trace,omitempty"`
	Latency          *uint64   `json:"latency,omitempty"`
	Octets           *uint64   `json:"octets"`
	OctetRate        *uint64   `json:"octetRate"`
	OctetsOut        *uint64   `json:"octetsOut,omitempty"`
	OctetsUnacked    *uint64   `json:"octetsUnacked,omitempty"`
	WindowClosures   *uint64   `json:"windowClosures,omitempty"`
	WindowSize       *uint64   `json:"windowSize,omitempty"`
	Reason           *string   `json:"reason,omitempty"`
	Method           *string   `json:"method,omitempty"`
	Result           *string   `json:"result,omitempty"`
	StreamIdentity   *uint64   `json:"streamIdentity,omitempty"`
	Process          *string   `json:"process,omitempty"`
	ProcessName      *string   `json:"processName,omitempty"`
	Protocol         *string   `json:"protocol,omitempty"`
	Place            FlowPlace `json:"place"`
	lastOctets       uint64
	octetMetric      prometheus.Counter
	activeFlowMetric prometheus.Gauge
	httpReqsMetric   prometheus.Counter
}

// Note a flowpair does not have a defined parent relationship through Base
type FlowPairRecord struct {
	Base
	Protocol                *string     `json:"protocol,omitempty"`
	SourceSiteId            string      `json:"sourceSiteId,omitempty"`
	SourceSiteName          *string     `json:"sourceSiteName,omitempty"`
	DestinationSiteId       string      `json:"destinationSiteId,omitempty"`
	DestinationSiteName     *string     `json:"destinationSiteName,omitempty"`
	FlowTrace               *string     `json:"flowTrace,omitempty"`
	ForwardFlow             *FlowRecord `json:"forwardFlow,omitempty"`
	CounterFlow             *FlowRecord `json:"counterFlow,omitempty"`
	SiteAggregateId         *string     `json:"siteAggregateId,omitempty"`
	ProcessGroupAggregateId *string     `json:"processGroupAggregateId,omitempty"`
	ProcessAggregateId      *string     `json:"processAggregateId,omitempty"`
}

type FlowAggregateRecord struct {
	Base
	PairType        string  `json:"pairType,omitempty"`
	RecordCount     uint64  `json:"recordCount,omitempty"`
	SourceId        *string `json:"sourceId,omitempty"`
	SourceName      *string `json:"sourceName,omitempty"`
	DestinationId   *string `json:"destinationId,omitempty"`
	DestinationName *string `json:"destinationName,omitempty"`
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
	PrometheusHost       string
	PrometheusAuthMethod string
	PrometheusUser       string
	PrometheusPassword   string
}

type Payload struct {
	Results        interface{} `json:"results"`
	QueryParams    QueryParams `json:"queryParams"`
	Status         string      `json:"status"`
	Count          int         `json:"count"`
	TimeRangeCount int         `json:"timeRangeCount"`
	TotalCount     int         `json:"totalCount"`
	timestamp      uint64
	elapsed        uint64
}

type QueryParams struct {
	Offset             int               `json:"offset"`
	Limit              int               `json:"limit"`
	SortBy             string            `json:"sortBy"`
	Filter             string            `json:"filter"`
	FilterFields       map[string]string `json:"filterFields"`
	TimeRangeStart     uint64            `json:"timeRangeStart"`
	TimeRangeEnd       uint64            `json:"timeRangeEnd"`
	TimeRangeOperation TimeRangeRelation `json:"timeRangeOperation"`
}

func getQueryParams(url *url.URL) QueryParams {
	now := uint64(time.Now().UnixNano()) / uint64(time.Microsecond)
	qp := QueryParams{
		Offset:             -1,
		Limit:              -1,
		SortBy:             "identity.asc",
		Filter:             "",
		FilterFields:       make(map[string]string),
		TimeRangeStart:     now - (15 * oneMinute),
		TimeRangeEnd:       now,
		TimeRangeOperation: intersects,
	}

	for k, v := range url.Query() {
		switch k {
		case "offset":
			offset, err := strconv.Atoi(v[0])
			if err == nil {
				qp.Offset = offset
			}
		case "limit":
			limit, err := strconv.Atoi(v[0])
			if err == nil {
				qp.Limit = limit
			}
		case "sortBy":
			if v[0] != "" {
				qp.SortBy = v[0]
			}
		case "filter":
			if v[0] != "" {
				qp.Filter = v[0]
			}
		case "timeRangeStart":
			if v[0] != "" {
				v, err := strconv.Atoi(v[0])
				if err == nil {
					qp.TimeRangeStart = uint64(v)
				}
			}
		case "timeRangeEnd":
			if v[0] != "" {
				v, err := strconv.Atoi(v[0])
				if err == nil {
					qp.TimeRangeEnd = uint64(v)
				}
			}
		case "timeRangeOperation":
			timeRangeOperation := v[0]
			switch timeRangeOperation {
			case "contains":
				qp.TimeRangeOperation = contains
			case "within":
				qp.TimeRangeOperation = within
			default:
				qp.TimeRangeOperation = intersects
			}
		default:
			qp.FilterFields[cases.Title(language.Und, cases.NoLower).String(k)] = v[0]
		}
	}
	return qp
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

const oneMinute uint64 = 60000000
const oneHour uint64 = 3600000000
const oneDay uint64 = 86400000000

type TimeRangeRelation int

const (
	intersects TimeRangeRelation = iota
	contains
	within
)

func (base *Base) TimeRangeValid(qp QueryParams) bool {
	switch qp.TimeRangeOperation {
	case intersects:
		return !(base.EndTime != 0 && base.EndTime < qp.TimeRangeStart || base.StartTime > qp.TimeRangeEnd)
	case contains:
		return base.StartTime <= qp.TimeRangeStart && (base.EndTime == 0 || base.EndTime >= qp.TimeRangeEnd)
	case within:
		return base.StartTime >= qp.TimeRangeStart && (base.EndTime != 0 && base.EndTime <= qp.TimeRangeEnd)
	default:
		return false
	}
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

func validateAndReturnSortQuery(sortBy string) (string, string, string, error) {
	sortBy = cases.Title(language.Und, cases.NoLower).String(sortBy)
	parts := strings.Split(sortBy, ".")
	if len(parts) < 2 {
		return "", "", "", fmt.Errorf("Malformed sortBy query parameter")
	}
	order := parts[len(parts)-1]
	field := parts[0]
	subField := cases.Title(language.Und, cases.NoLower).String(strings.Join(parts[1:len(parts)-1], "."))
	if order != "asc" && order != "desc" {
		return "", "", "", fmt.Errorf("Malformed order direction in sortBy query parameter, should be asc or desc")
	}
	return field, subField, order, nil
}

func validateAndReturnFilterQuery(filter string) (string, string, error) {
	parts := strings.Split(filter, ".")
	if len(parts) == 1 {
		return "", "", fmt.Errorf("Missing filter query value parameter")
	}
	field := cases.Title(language.Und, cases.NoLower).String(parts[0])
	match := strings.Join(parts[1:], ".")
	return field, match, nil
}

func validateAndReturnFilterFieldQuery(filterField string) (string, string, error) {
	parts := strings.Split(filterField, ".")
	if len(parts) > 2 {
		return "", "", fmt.Errorf("Malformed filter field query value parameter")
	}
	field := cases.Title(language.Und, cases.NoLower).String(parts[0])
	subField := ""
	if len(parts) == 2 {
		subField = cases.Title(language.Und, cases.NoLower).String(parts[1])
	}
	return field, subField, nil
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
			case reflect.Struct:
				return x.FieldByName(field).Elem().Interface()
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

func matchFieldValue(x interface{}, y string) bool {
	if x != nil && y != "" {
		switch x.(type) {
		case string:
			return x.(string) == y
		case uint64:
			i, err := strconv.ParseInt(y, 10, 64)
			if err == nil {
				return x.(uint64) == uint64(i)
			}
		case int32:
			i, err := strconv.ParseInt(y, 10, 32)
			if err == nil {
				return x.(int32) == int32(i)
			}
		case int64:
			i, err := strconv.ParseInt(y, 10, 64)
			if err == nil {
				return x.(int64) == int64(i)
			}
		case int:
			i, err := strconv.ParseInt(y, 10, 64)
			if err == nil {
				return x.(int) == int(i)
			}
		default:
			return false
		}
	}
	return false
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

// type any = interface{}
func filterRecord[T any](item T, qp QueryParams) bool {
	filter := true
	if qp.Filter != "" {
		field, match, err := validateAndReturnFilterQuery(qp.Filter)
		// todo propagate error or log
		if err != nil {
			return false
		}
		value := getField(field, item)
		if value == nil {
			filter = false
		} else {
			x := reflect.ValueOf(value)
			if x.Kind() == reflect.Struct {
				if !filterSubRecord(value, match) {
					filter = false
				}
			} else if !matchFieldValue(value, match) {
				filter = false
			}
		}
	} else if len(qp.FilterFields) > 0 {
		for field, match := range qp.FilterFields {
			field, subField, err := validateAndReturnFilterFieldQuery(field)
			if err != nil {
				return false
			}
			value := getField(field, item)
			if value == nil {
				filter = false
			} else {
				x := reflect.ValueOf(value)
				if x.Kind() == reflect.Struct {
					if !filterSubRecord(value, subField+"."+match) {
						filter = false
					}
				} else if !matchFieldValue(value, match) {
					filter = false
				}
			}
		}
	}
	return filter
}

// type any = interface{}
func filterSubRecord[T any](item T, filter string) bool {
	if filter == "" {
		return true
	}
	field, match, err := validateAndReturnFilterQuery(filter)
	// todo propagate error or log
	if err != nil {
		return false
	}
	value := getField(field, item)
	x := reflect.ValueOf(value)
	if x.Kind() == reflect.Struct {
		return filterSubRecord(value, match)
	}
	return matchFieldValue(value, match)
}

func sortAndSlice[T any](list []T, payload *Payload) error {
	offset := payload.QueryParams.Offset
	limit := payload.QueryParams.Limit
	start := 0
	end := 0
	field, subField, order, err := validateAndReturnSortQuery(payload.QueryParams.SortBy)
	if err != nil {
		return err
	}
	payload.TimeRangeCount = len(list)
	sort.Slice(list, func(i, j int) bool {
		v1 := getField(field, list[i])
		v2 := getField(field, list[j])
		x := reflect.ValueOf(v1)
		// todo: embedded all the way down
		if x.Kind() == reflect.Struct {
			v1 = getField(subField, v1)
			v2 = getField(subField, v2)
		}
		return compareFields(v1, v2, order)
	})
	start, end = paginate(offset, limit, len(list))
	payload.Count = end - start
	payload.Results = (list[start:end])
	return nil
}

package types

import (
         "fmt"
         "os"
         "runtime"
         "time"
       )

type VanConnectorCreateOptions struct {
	Name string
	Cost int32
}

type VanSiteConfig struct {
	Spec      VanSiteConfigSpec
	Reference VanSiteConfigReference
}

type VanSiteConfigSpec struct {
	SkupperName         string
	IsEdge              bool
	EnableController    bool
	EnableServiceSync   bool
	EnableRouterConsole bool
	EnableConsole       bool
	AuthMode            string
	User                string
	Password            string
	ClusterLocal        bool
	Replicas            int32
	SiteControlled      bool
}

type VanSiteConfigReference struct {
	UID        string
	Name       string
	APIVersion string
	Kind       string
}

type VanServiceInterfaceCreateOptions struct {
	Protocol   string
	Address    string
	Port       int
	TargetPort int
	Headless   bool
}

type VanRouterInspectResponse struct {
	Status            VanRouterStatusSpec
	TransportVersion  string
	ControllerVersion string
	ExposedServices   int
}

type VanConnectorInspectResponse struct {
	Connector *Connector
	Connected bool
}

const (
	EventCodeNone = iota
	EventCodeInfo
	EventCodeWarning
	EventCodeError
)

type Result struct {
	Function  string
	File      string
	Line      int
	Code      int
	Message   string
	Timestamp float64
}

type Results []Result

func EventCode2Str(code int) string {
	switch code {
	case EventCodeNone:
		return "No_Event_Code"
	case EventCodeInfo:
		return "Info"
	case EventCodeWarning:
		return "Warning"
	case EventCodeError:
		return "Error"

	default:
		return "Unknown Event Code"
	}
}

func (rs *Results) addResult(code int, format string, args ...interface{}) {
	pc, file, line, _ := runtime.Caller(1)
	r := Result{Timestamp: float64(time.Now().UnixNano()) / 1000000000.0,
		Function: runtime.FuncForPC(pc).Name(),
		File:     file,
		Line:     line,
		Code:     code,
		Message:  fmt.Sprintf(format, args...),
	}
	*rs = append(*rs, r)
}

func (rs *Results) AddInfo(format string, args ...interface{}) {
	rs.addResult(EventCodeInfo, format, args)
}

func (rs *Results) AddWarning(format string, args ...interface{}) {
	rs.addResult(EventCodeWarning, format, args)
}

func (rs *Results) AddError(format string, args ...interface{}) {
	rs.addResult(EventCodeError, format, args)
}

func (rs *Results) Latest() Result {
	return (*rs)[len(*rs)-1]
}

func (rs *Results) IsError() bool {
	return rs.Latest().Code == EventCodeError
}

func (rs *Results) Print(f *os.File) {
	for _, r := range *rs {
		fmt.Fprintf(f, "%.6f %s : file %s function %s line %d : %s\n",
			r.Timestamp,
			EventCode2Str(r.Code),
			r.File,
			r.Function,
			r.Line,
			r.Message)
	}
}

func (rs *Results) ContainsError() bool {
	for _, r := range *rs {
		if r.Code == EventCodeError {
			return true
		}
	}
	return false
}


package common

var (
	LinkAccessTypes = []string{"route", "loadbalancer", "default"}
	OutputTypes     = []string{"json", "yaml"}
	ListenerTypes   = []string{"tcp"}
	ConnectorTypes  = []string{"tcp"}
	WorkloadTypes   = []string{"deployment", "service", "daemonset", "statefulset"}
	WaitStatusTypes = []string{"ready", "configured"}
)

const (
	Connectors     string = "connectors"
	Listeners      string = "listeners"
	Sites          string = "sites"
	RouterAccesses string = "routerAccesses"
)

package common

var (
	LinkAccessTypes = []string{"route", "loadbalancer", "default"}
	OutputTypes     = []string{"json", "yaml"}
	ListenerTypes   = []string{"tcp"}
	ConnectorTypes  = []string{"tcp"}
	WorkloadTypes   = []string{"deployment", "service", "daemonset", "statefulset"}
	WaitStatusTypes = []string{"ready", "configured", "none"}
)

const (
	Connectors     string = "connectors"
	Listeners      string = "listeners"
	Sites          string = "site"
	RouterAccesses string = "routerAccesses"
)

const (
	SiteConfigNameKey string = "name"
)

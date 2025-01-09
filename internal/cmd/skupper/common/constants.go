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
	Sites          string = "sites"
	RouterAccesses string = "routerAccesses"
	Links          string = "links"
)

const (
	SiteConfigNameKey string = "name"
)

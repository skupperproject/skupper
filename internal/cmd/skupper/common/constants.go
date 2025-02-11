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
	Connectors     string = "Connector"
	Listeners      string = "Listener"
	Sites          string = "Site"
	RouterAccesses string = "RouterAccess"
	Links          string = "Link"
)

const (
	SiteConfigNameKey string = "name"
)

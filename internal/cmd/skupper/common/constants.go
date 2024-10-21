package common

var (
	LinkAccessTypes = []string{"route", "loadbalancer", "default"}
	OutputTypes     = []string{"json", "yaml"}
	ListenerTypes   = []string{"tcp"}
	ConnectorTypes  = []string{"tcp"}
	WorkloadTypes   = []string{"deployment", "service", "daemonset", "statefulset"}
)

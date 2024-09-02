package securedaccess

import (
	"flag"
	"fmt"

	iflag "github.com/skupperproject/skupper/internal/flag"

	internalclient "github.com/skupperproject/skupper/internal/kube/client"
)

const ACCESS_TYPE_LOADBALANCER = "loadbalancer"
const ACCESS_TYPE_ROUTE = "route"
const ACCESS_TYPE_NODEPORT = "nodeport"
const ACCESS_TYPE_INGRESS_NGINX = "ingress-nginx"
const ACCESS_TYPE_CONTOUR_HTTP_PROXY = "contour-http-proxy"
const ACCESS_TYPE_GATEWAY = "gateway"
const ACCESS_TYPE_LOCAL = "local"

type Config struct {
	EnabledAccessTypes []string
	DefaultAccessType  string
	ClusterHost        string
	IngressDomain      string
	HttpProxyDomain    string
	GatewayPort        int
	GatewayClass       string
	GatewayDomain      string
}

func (c *Config) isEnabled(accessType string) bool {
	for _, a := range c.EnabledAccessTypes {
		if a == accessType {
			return true
		}
	}
	return false
}

func (c *Config) Verify() error {
	// check that the default access type is included in those enabled
	if c.DefaultAccessType != "" && !c.isEnabled(c.DefaultAccessType) {
		return fmt.Errorf("Default access type %q is not in enabled list.", c.DefaultAccessType)
	}
	// if nodeport is in enabled list, check that clusterhost is set
	if c.isEnabled("nodeport") && c.ClusterHost == "" {
		return fmt.Errorf("Cluster host must be set to enable nodeport access type.")
	}
	// if gateway is in enabled list, check that the class is set
	if c.isEnabled("gateway") && c.GatewayClass == "" {
		return fmt.Errorf("Gateway class must be set to enable gateway access type.")
	}
	return nil
}

func (c *Config) getDefaultAccessType(clients internalclient.Clients) string {
	if c.DefaultAccessType == "" {
		if clients.GetRouteClient() != nil && c.isEnabled(ACCESS_TYPE_ROUTE) {
			return ACCESS_TYPE_ROUTE
		}
		if c.isEnabled(ACCESS_TYPE_LOADBALANCER) {
			return ACCESS_TYPE_LOADBALANCER
		}
		if len(c.EnabledAccessTypes) > 0 {
			return c.EnabledAccessTypes[0]
		}
	}
	return c.DefaultAccessType
}

func BoundConfig(flags *flag.FlagSet) (*Config, error) {
	c := &Config{}
	iflag.MultiStringVar(flags, &c.EnabledAccessTypes, "enabled-access-types", "SKUPPER_ENABLED_ACCESS_TYPES", defaultEnabledAccessTypes(), "The access types which should be enabled for sites to choose from.")
	iflag.StringVar(flags, &c.DefaultAccessType, "default-access-type", "SKUPPER_DEFAULT_ACCESS_TYPE", "", "The default access type.")
	iflag.StringVar(flags, &c.ClusterHost, "cluster-host", "SKUPPER_CLUSTER_HOST", "", "The hostname or IP address through which the cluster can be reached. Required for configuring nodeport as an access type.")
	iflag.StringVar(flags, &c.IngressDomain, "ingress-domain", "SKUPPER_INGRESS_DOMAIN", "", "The domain to use in constructing the fully qualified hostname for Ingress resources, through which the ingress controller can be reached. Only used when selecting ingress-nginx as an access type.")
	iflag.StringVar(flags, &c.HttpProxyDomain, "http-proxy-domain", "SKUPPER_HTTP_PROXY_DOMAIN", "", "The domain to use in constructing the fully qualified hostname for contour HttpProxy resources, through which the contour controller can be reached. Only used when selecting contour-http-proxy as an access type.")
	iflag.StringVar(flags, &c.GatewayDomain, "gateway-domain", "SKUPPER_GATEWAY_DOMAIN", "", "The domain to use in constructing the fully qualified hostname for TLSRoutes resources. Only used when selecting gateway as an access type.")
	iflag.StringVar(flags, &c.GatewayClass, "gateway-class", "SKUPPER_GATEWAY_CLASS", "", "The class of Gateway to use. This is required to enable gateway as an access type.")
	iflag.IntVar(flags, &c.GatewayPort, "gateway-port", "SKUPPER_GATEWAY_PORT", 8443, "The port the Gateway should be configured to listen on. This is only used if gateway is enabled as an access type.")
	return c, nil
}

func defaultEnabledAccessTypes() []string {
	return []string{
		"local",
		"loadbalancer",
		"route",
	}
}

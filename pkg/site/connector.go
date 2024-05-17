package site

import (
	"log"
	"strconv"

	skupperv1alpha1 "github.com/skupperproject/skupper/pkg/apis/skupper/v1alpha1"
	"github.com/skupperproject/skupper/pkg/qdr"
)

type Connector struct {
	resource *skupperv1alpha1.Connector
	selection TargetSelection
}

func (c *Connector) init(context BindingContext) bool {
	if c.selection != nil {
		c.selection.Close()
	}
	if c.resource.Spec.Selector != "" && context != nil {
		c.selection = context.Select(c.resource)
		return false
	}
	return true
}

func (c *Connector) resourceUpdated(resource *skupperv1alpha1.Connector) {
	c.resource = resource
	if c.selection != nil {
		c.selection.Update(c.resource)
	}
}

func (c *Connector) updateBridges_(name string, host string, siteId string, config *qdr.BridgeConfig) {
	if c.resource.Spec.Type == "http" {
		config.HttpConnectors[name] = c.AsHttpEndpoint(name, host, siteId)
	} else if c.resource.Spec.Type == "http2" {
		config.HttpConnectors[name] = c.AsHttp2Endpoint(name, host, siteId)
	} else if c.resource.Spec.Type == "tcp" || c.resource.Spec.Type == "" {
		config.TcpConnectors[name] = c.AsTcpEndpoint(name, host, siteId)
		log.Printf("Updated tcp-connectors for %s: %v", name, config)
	}
}

func (c *Connector) updateBridges(siteId string, config *qdr.BridgeConfig) {
	if c.selection == nil {
		c.updateBridges_(c.resource.Name, c.resource.Spec.Host, siteId, config)
	} else {
		hosts := c.selection.List()
		for _, host := range hosts {
			name := c.resource.Name + "_" + host
			c.updateBridges_(name, host, siteId, config)
		}
	}
}

func (c *Connector) AsTcpEndpoint(name string, host string, siteId string) qdr.TcpEndpoint {
	return qdr.TcpEndpoint {
		Name:       name,
		Host:       host,
		Port:       strconv.Itoa(c.resource.Spec.Port),
		Address:    c.resource.Spec.RoutingKey,
		SiteId:     siteId,
		SslProfile: c.resource.Spec.TlsCredentials,
		//TODO:
		//VerifyHostname
	}
}

func (c *Connector) AsHttpEndpoint(name string, host string, siteId string) qdr.HttpEndpoint {
	return qdr.HttpEndpoint {
		Name:       name,
		Host:       host,
		Port:       strconv.Itoa(c.resource.Spec.Port), //TODO: should port be a string to allow for wll known service names in binding definitions?
		Address:    c.resource.Spec.RoutingKey,
		SiteId:     siteId,
		SslProfile: c.resource.Spec.TlsCredentials,
	        //TODO:
	        //Aggregation
	        //EventChannel
	        //HostOverride
		//VerifyHostname
	}
}

func (c *Connector) AsHttp2Endpoint(name string, host string, siteId string) qdr.HttpEndpoint {
	endpoint := c.AsHttpEndpoint(name, host, siteId)
	endpoint.ProtocolVersion = qdr.HttpVersion2
	return endpoint
}

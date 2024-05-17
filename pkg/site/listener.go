package site

import (
	"log"
	"strconv"

	corev1 "k8s.io/api/core/v1"

	skupperv1alpha1 "github.com/skupperproject/skupper/pkg/apis/skupper/v1alpha1"
	"github.com/skupperproject/skupper/pkg/qdr"
)

type Listener struct {
	resource *skupperv1alpha1.Listener
}

func (l *Listener) updateBridges(siteId string, mapping *qdr.PortMapping, config *qdr.BridgeConfig) {
	if l.resource.Spec.Type == "http" {
		config.HttpListeners[l.resource.Name] = l.AsHttpEndpoint(siteId, mapping)
	} else if l.resource.Spec.Type == "http2" {
		config.HttpListeners[l.resource.Name] = l.AsHttp2Endpoint(siteId, mapping)
	} else if l.resource.Spec.Type == "tcp" || l.resource.Spec.Type == "" {
		config.TcpListeners[l.resource.Name] = l.AsTcpEndpoint(siteId, mapping)
	}
}

func (l *Listener) AsTcpEndpoint(siteId string, mapping *qdr.PortMapping) qdr.TcpEndpoint {
	port, err := mapping.GetPortForKey(l.resource.Name)
	if err != nil {
		log.Printf("Could not allocate port for %s/%s: %s", l.resource.Namespace, l.resource.Name, err)
	}
	return qdr.TcpEndpoint {
		Name:    l.resource.Name,
		Host:    "0.0.0.0",
		Port:    strconv.Itoa(port),
		Address: l.resource.Spec.RoutingKey,
		SiteId:  siteId,
		SslProfile: l.resource.Spec.TlsCredentials,
		//TODO:
		//VerifyHostname
	}
}

func (l *Listener) AsHttpEndpoint(siteId string, mapping *qdr.PortMapping) qdr.HttpEndpoint {
	port, err := mapping.GetPortForKey(l.resource.Name)
	if err != nil {
		log.Printf("Could not allocate port for %s/%s: %s", l.resource.Namespace, l.resource.Name, err)
	}
	return qdr.HttpEndpoint {
		Name:       l.resource.Name,
		Host:       "0.0.0.0",
		Port:       strconv.Itoa(port), //TODO: should port be a string to allow for wll known service names in binding definitions?
		Address:    l.resource.Spec.RoutingKey,
		SiteId:     siteId,
		SslProfile: l.resource.Spec.TlsCredentials,
	        //TODO:
	        //Aggregation
	        //EventChannel
	        //HostOverride
		//VerifyHostname
	}
}

func (l *Listener) AsHttp2Endpoint(siteId string, mapping *qdr.PortMapping) qdr.HttpEndpoint {
	endpoint := l.AsHttpEndpoint(siteId, mapping)
	endpoint.ProtocolVersion = qdr.HttpVersion2
	return endpoint
}

func (l *Listener) protocol() corev1.Protocol {
	if l.resource.Spec.Type == "udp" {
		return corev1.ProtocolUDP
	}
	return corev1.ProtocolTCP
}

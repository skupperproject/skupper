package site

import (
	"strconv"

	skupperv1alpha1 "github.com/skupperproject/skupper/pkg/apis/skupper/v1alpha1"
	"github.com/skupperproject/skupper/pkg/qdr"
)

func UpdateBridgeConfigForConnector(siteId string, connector *skupperv1alpha1.Connector, config *qdr.BridgeConfig) {
	if connector.Spec.Host != "" {
		UpdateBridgeConfigForConnectorWithHostProcess(siteId, connector, connector.Spec.Host, "", config)
	}
}

func UpdateBridgeConfigForConnectorWithHost(siteId string, connector *skupperv1alpha1.Connector, host string, config *qdr.BridgeConfig) {
	UpdateBridgeConfigForConnectorWithHostProcess(siteId, connector, host, "", config)
}

func UpdateBridgeConfigForConnectorWithHostProcess(siteId string, connector *skupperv1alpha1.Connector, host string, processID string, config *qdr.BridgeConfig) {
	name := connector.Name + "-" + host
	if connector.Spec.Type == "tcp" || connector.Spec.Type == "" {
		config.AddTcpConnector(qdr.TcpEndpoint{
			Name:       name,
			SiteId:     siteId,
			Host:       host,
			Port:       strconv.Itoa(connector.Spec.Port),
			Address:    connector.Spec.RoutingKey,
			SslProfile: connector.Spec.TlsCredentials,
			ProcessID:  processID,
			//TODO:
			//VerifyHostname
		})
	} else if connector.Spec.Type == "http" || connector.Spec.Type == "http2" {
		endpoint := qdr.HttpEndpoint{
			Name:       name,
			SiteId:     siteId,
			Host:       host,
			Port:       strconv.Itoa(connector.Spec.Port),
			Address:    connector.Spec.RoutingKey,
			SslProfile: connector.Spec.TlsCredentials,
			//TODO:
			//Aggregation
			//EventChannel
			//HostOverride
			//VerifyHostname
		}
		if connector.Spec.Type == "http2" {
			endpoint.ProtocolVersion = qdr.HttpVersion2
		}
		config.AddHttpConnector(endpoint)
	}
}

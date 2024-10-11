package site

import (
	"strconv"

	skupperv2alpha1 "github.com/skupperproject/skupper/pkg/apis/skupper/v2alpha1"
	"github.com/skupperproject/skupper/pkg/qdr"
)

func UpdateBridgeConfigForConnector(siteId string, connector *skupperv2alpha1.Connector, config *qdr.BridgeConfig) {
	if connector.Spec.Host != "" {
		updateBridgeConfigForConnector(siteId, connector, connector.Spec.Host, "", config)
	}
}

func UpdateBridgeConfigForConnectorToPod(siteId string, connector *skupperv2alpha1.Connector, pod skupperv2alpha1.PodDetails, config *qdr.BridgeConfig) {
	updateBridgeConfigForConnector(siteId, connector, pod.IP, pod.UID, config)
}

func updateBridgeConfigForConnector(siteId string, connector *skupperv2alpha1.Connector, host string, processID string, config *qdr.BridgeConfig) {
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
	}
}

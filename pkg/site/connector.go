package site

import (
	"strconv"

	skupperv2alpha1 "github.com/skupperproject/skupper/pkg/apis/skupper/v2alpha1"
	"github.com/skupperproject/skupper/pkg/qdr"
)

func UpdateBridgeConfigForConnector(siteId string, connector *skupperv2alpha1.Connector, config *qdr.BridgeConfig) {
	if connector.Spec.Host != "" {
		updateBridgeConfigForConnector(connector.Name+"-"+connector.Spec.Host, siteId, connector, connector.Spec.Host, "", connector.Spec.RoutingKey, config)
	}
}

func UpdateBridgeConfigForConnectorToPod(siteId string, connector *skupperv2alpha1.Connector, pod skupperv2alpha1.PodDetails, addQualifiedAddress bool, config *qdr.BridgeConfig) {
	updateBridgeConfigForConnector(connector.Name+"-"+pod.IP, siteId, connector, pod.IP, pod.UID, connector.Spec.RoutingKey, config)
	if addQualifiedAddress {
		updateBridgeConfigForConnector(connector.Name+"-"+pod.Name, siteId, connector, pod.IP, pod.UID, connector.Spec.RoutingKey+"."+pod.Name, config)
	}
}

func updateBridgeConfigForConnector(name string, siteId string, connector *skupperv2alpha1.Connector, host string, processID string, address string, config *qdr.BridgeConfig) {
	if connector.Spec.Type == "tcp" || connector.Spec.Type == "" {
		config.AddTcpConnector(qdr.TcpEndpoint{
			Name:           name,
			SiteId:         siteId,
			Host:           host,
			Port:           strconv.Itoa(connector.Spec.Port),
			Address:        address,
			SslProfile:     getSslProfileName(connector),
			ProcessID:      processID,
			VerifyHostname: getVerifyHostname(connector),
		})
	}
}

func getSslProfileName(connector *skupperv2alpha1.Connector) string {
	if connector.Spec.TlsCredentials == "" {
		return ""
	}
	if connector.Spec.NoClientAuth {
		return connector.Spec.TlsCredentials + "-profile"
	}
	return connector.Spec.TlsCredentials
}

func getVerifyHostname(connector *skupperv2alpha1.Connector) *bool {
	if connector.Spec.TlsCredentials == "" {
		return nil
	}
	value := connector.Spec.VerifyHostname
	return &value
}

package site

import (
	"strconv"

	"github.com/skupperproject/skupper/internal/qdr"
	skupperv2alpha1 "github.com/skupperproject/skupper/pkg/apis/skupper/v2alpha1"
)

func UpdateBridgeConfigForConnector(siteId string, connector *skupperv2alpha1.Connector, config *qdr.BridgeConfig) {
	if connector.Spec.Host != "" {
		updateBridgeConfigForConnector(connector.Name+"@"+connector.Spec.Host, siteId, connector, connector.Spec.Host, "", connector.Spec.RoutingKey, config)
	}
}

func UpdateBridgeConfigForConnectorToPod(siteId string, connector *skupperv2alpha1.Connector, pod skupperv2alpha1.PodDetails, addQualifiedAddress bool, config *qdr.BridgeConfig) bool {
	updated := false
	if updateBridgeConfigForConnector(connector.Name+"@"+pod.IP, siteId, connector, pod.IP, pod.UID, connector.Spec.RoutingKey, config) {
		updated = true
	}
	if addQualifiedAddress {
		if updateBridgeConfigForConnector(connector.Name+"@"+pod.Name, siteId, connector, pod.IP, pod.UID, connector.Spec.RoutingKey+"."+pod.Name, config) {
			updated = true
		}
	}
	return updated
}

func updateBridgeConfigForConnector(name string, siteId string, connector *skupperv2alpha1.Connector, host string, processID string, address string, config *qdr.BridgeConfig) bool {
	if connector.Spec.Type == "tcp" || connector.Spec.Type == "" {
		return config.AddTcpConnector(qdr.TcpEndpoint{
			Name:           name,
			SiteId:         siteId,
			Host:           host,
			Port:           strconv.Itoa(connector.Spec.Port),
			Address:        address,
			SslProfile:     GetSslProfileName(connector.Spec.TlsCredentials, connector.Spec.UseClientCert),
			ProcessID:      processID,
			VerifyHostname: getVerifyHostname(connector),
		})
	}
	return false
}

func GetSslProfileName(tlsCredentials string, useClientCert bool) string {
	if tlsCredentials == "" {
		return ""
	}
	if !useClientCert {
		return tlsCredentials + "-profile"
	}
	return tlsCredentials
}

func getVerifyHostname(connector *skupperv2alpha1.Connector) *bool {
	if connector.Spec.TlsCredentials == "" {
		return nil
	}
	value := connector.Spec.VerifyHostname
	return &value
}

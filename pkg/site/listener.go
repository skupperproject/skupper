package site

import (
	"strconv"

	skupperv1alpha1 "github.com/skupperproject/skupper/pkg/apis/skupper/v1alpha1"
	"github.com/skupperproject/skupper/pkg/qdr"
)

func UpdateBridgeConfigForListener(siteId string, listener *skupperv1alpha1.Listener, config *qdr.BridgeConfig) {
	UpdateBridgeConfigForListenerWithHostAndPort(siteId, listener, listener.Spec.Host, listener.Spec.Port, config)
}

func UpdateBridgeConfigForListenerWithHostAndPort(siteId string, listener *skupperv1alpha1.Listener, host string, port int, config *qdr.BridgeConfig) {
	name := listener.Name
	if listener.Spec.Type == "tcp" || listener.Spec.Type == "" {
		config.AddTcpListener(qdr.TcpEndpoint{
			Name:       name,
			SiteId:     siteId,
			Host:       host,
			Port:       strconv.Itoa(port),
			Address:    listener.Spec.RoutingKey,
			SslProfile: listener.Spec.TlsCredentials,
			//TODO:
			//VerifyHostname
		})
	}
}

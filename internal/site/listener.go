package site

import (
	"strconv"

	"github.com/skupperproject/skupper/internal/qdr"
	skupperv2alpha1 "github.com/skupperproject/skupper/pkg/apis/skupper/v2alpha1"
)

func UpdateBridgeConfigForListener(siteId string, listener *skupperv2alpha1.Listener, config *qdr.BridgeConfig) {
	UpdateBridgeConfigForListenerWithHostAndPort(siteId, listener, listener.Spec.Host, listener.Spec.Port, config)
}

func UpdateBridgeConfigForListenerWithHostAndPort(siteId string, listener *skupperv2alpha1.Listener, host string, port int, config *qdr.BridgeConfig) {
	name := listener.Name
	if listener.Spec.Type == "tcp" || listener.Spec.Type == "" {
		config.AddTcpListener(qdr.TcpEndpoint{
			Name:       name,
			SiteId:     siteId,
			Host:       host,
			Port:       strconv.Itoa(port),
			Address:    listener.Spec.RoutingKey,
			SslProfile: listener.Spec.TlsCredentials,
			Observer:   listener.Spec.Observer,
		})
	}
}

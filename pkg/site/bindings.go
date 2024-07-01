package site

import (
	"log"

	skupperv1alpha1 "github.com/skupperproject/skupper/pkg/apis/skupper/v1alpha1"
	"github.com/skupperproject/skupper/pkg/qdr"
)

type ListenerConfiguration func (siteId string, listener *skupperv1alpha1.Listener, config *qdr.BridgeConfig)
type ConnectorConfiguration func (siteId string, connector *skupperv1alpha1.Connector, config *qdr.BridgeConfig)

type BindingEventHandler interface {
	ListenerUpdated(listener *skupperv1alpha1.Listener)
	ListenerDeleted(listener *skupperv1alpha1.Listener)
	ConnectorUpdated(connector *skupperv1alpha1.Connector, specChanged bool) bool
	ConnectorDeleted(connector *skupperv1alpha1.Connector)
}

type Bindings struct {
	SiteId     string
	connectors map[string]*skupperv1alpha1.Connector
	listeners  map[string]*skupperv1alpha1.Listener
	handler    BindingEventHandler
	configure  struct{
		listener  ListenerConfiguration
		connector ConnectorConfiguration
	}
}

func NewBindings() *Bindings {
	bindings := &Bindings{
		connectors: map[string]*skupperv1alpha1.Connector{},
		listeners:  map[string]*skupperv1alpha1.Listener{},
	}
	bindings.configure.listener = UpdateBridgeConfigForListener
	bindings.configure.connector = UpdateBridgeConfigForConnector
	return bindings
}

func (b *Bindings) SetSiteId(siteId string) {
	b.SiteId = siteId
}

func (b *Bindings) SetListenerConfiguration(configuration ListenerConfiguration) {
	b.configure.listener = configuration
}

func (b *Bindings) SetConnectorConfiguration(configuration ConnectorConfiguration) {
	b.configure.connector = configuration
}

func (b *Bindings) SetBindingEventHandler(handler BindingEventHandler) {
	b.handler = handler
	for _, c := range b.connectors {
		b.handler.ConnectorUpdated(c, true)
	}
	for _, l := range b.listeners {
		b.handler.ListenerUpdated(l)
	}
}

func (b *Bindings) UpdateConnector(name string, connector *skupperv1alpha1.Connector) (qdr.ConfigUpdate, error) {
	if connector == nil {
		return b.deleteConnector(name), nil
	}
	return b.updateConnector(connector)
}

func (b *Bindings) updateConnector(connector *skupperv1alpha1.Connector) (qdr.ConfigUpdate, error) {
	name := connector.ObjectMeta.Name
	existing, ok := b.connectors[name]
	b.connectors[name] = connector
	if b.handler == nil {
		if !ok || existing.Spec != connector.Spec {
			return b, nil
		}
	} else {
		if b.handler.ConnectorUpdated(connector, !ok || existing.Spec == connector.Spec) {
			return b, nil
		}
	}
	return nil, nil
}

func (b *Bindings) deleteConnector(name string) qdr.ConfigUpdate {
	if existing, ok := b.connectors[name]; ok {
		delete(b.connectors, name)
		if b.handler != nil {
			b.handler.ConnectorDeleted(existing)
		}
		return b
	}
	return nil
}

func (b *Bindings) UpdateListener(name string, listener *skupperv1alpha1.Listener) (qdr.ConfigUpdate, error) {
	if listener == nil {
		return b.deleteListener(name), nil
	}
	return b.updateListener(listener)
}

func (b *Bindings) updateListener(latest *skupperv1alpha1.Listener) (qdr.ConfigUpdate, error) {
	log.Printf("updating listener %s/%s...", latest.Namespace, latest.Name)
	name := latest.ObjectMeta.Name
	existing, ok := b.listeners[name]
	b.listeners[name] = latest
	if b.handler != nil {
		b.handler.ListenerUpdated(latest)
	}

	if !ok || existing.Spec != latest.Spec {
		return b, nil
	}
	return nil, nil
}

func (b *Bindings) deleteListener(name string) qdr.ConfigUpdate {
	if existing, ok := b.listeners[name]; ok {
		delete(b.listeners, name)
		if b.handler != nil {
			b.handler.ListenerDeleted(existing)
		}
		return b
	}
	return nil
}

func (b *Bindings) ToBridgeConfig() qdr.BridgeConfig {
	config := qdr.BridgeConfig{
		TcpListeners:   qdr.TcpEndpointMap{},
		TcpConnectors:  qdr.TcpEndpointMap{},
		HttpListeners:  qdr.HttpEndpointMap{},
		HttpConnectors: qdr.HttpEndpointMap{},
	}
	for _, c := range b.connectors {
		b.configure.connector(b.SiteId, c, &config)
	}
	for _, l := range b.listeners {
		b.configure.listener(b.SiteId, l, &config)
	}

	return config
}

func (b *Bindings) Apply(config *qdr.RouterConfig) bool {
	//TODO: add/remove SslProfiles as necessary
	config.UpdateBridgeConfig(b.ToBridgeConfig())
	return true //TODO: can optimise by indicating if no change was required
}

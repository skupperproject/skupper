package site

import (
	"log"
	"reflect"

	skupperv2alpha1 "github.com/skupperproject/skupper/pkg/apis/skupper/v2alpha1"
	"github.com/skupperproject/skupper/pkg/qdr"
)

type ListenerConfiguration func(siteId string, listener *skupperv2alpha1.Listener, config *qdr.BridgeConfig)
type ConnectorConfiguration func(siteId string, connector *skupperv2alpha1.Connector, config *qdr.BridgeConfig)

type BindingEventHandler interface {
	ListenerUpdated(listener *skupperv2alpha1.Listener)
	ListenerDeleted(listener *skupperv2alpha1.Listener)
	ConnectorUpdated(connector *skupperv2alpha1.Connector) bool
	ConnectorDeleted(connector *skupperv2alpha1.Connector)
}

type ConnectorFunction func(*skupperv2alpha1.Connector) *skupperv2alpha1.Connector
type ListenerFunction func(*skupperv2alpha1.Listener) *skupperv2alpha1.Listener

type Bindings struct {
	SiteId     string
	connectors map[string]*skupperv2alpha1.Connector
	listeners  map[string]*skupperv2alpha1.Listener
	handler    BindingEventHandler
	configure  struct {
		listener  ListenerConfiguration
		connector ConnectorConfiguration
	}
}

func NewBindings() *Bindings {
	bindings := &Bindings{
		connectors: map[string]*skupperv2alpha1.Connector{},
		listeners:  map[string]*skupperv2alpha1.Listener{},
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
		b.handler.ConnectorUpdated(c)
	}
	for _, l := range b.listeners {
		b.handler.ListenerUpdated(l)
	}
}

func (b *Bindings) Map(cf ConnectorFunction, lf ListenerFunction) {
	if cf != nil {
		for key, connector := range b.connectors {
			if updated := cf(connector); updated != nil {
				b.connectors[key] = updated
			}
		}
	}
	if lf != nil {
		for key, listener := range b.listeners {
			if updated := lf(listener); updated != nil {
				b.listeners[key] = updated
			}
		}
	}
}

func (b *Bindings) GetConnector(name string) *skupperv2alpha1.Connector {
	if existing, ok := b.connectors[name]; ok {
		return existing
	}
	return nil
}

func (b *Bindings) GetListener(name string) *skupperv2alpha1.Listener {
	if existing, ok := b.listeners[name]; ok {
		return existing
	}
	return nil
}

func (b *Bindings) UpdateConnector(name string, connector *skupperv2alpha1.Connector) qdr.ConfigUpdate {
	if connector == nil {
		return b.deleteConnector(name)
	}
	return b.updateConnector(connector)
}

func (b *Bindings) updateConnector(connector *skupperv2alpha1.Connector) qdr.ConfigUpdate {
	name := connector.ObjectMeta.Name
	existing, ok := b.connectors[name]
	b.connectors[name] = connector // always update pointer, even if spec has not changed
	if ok && reflect.DeepEqual(existing.Spec, connector.Spec) {
		return nil
	}
	if b.handler == nil || b.handler.ConnectorUpdated(connector) {
		return b
	}
	return nil
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

func (b *Bindings) UpdateListener(name string, listener *skupperv2alpha1.Listener) qdr.ConfigUpdate {
	if listener == nil {
		return b.deleteListener(name)
	}
	return b.updateListener(listener)
}

func (b *Bindings) updateListener(latest *skupperv2alpha1.Listener) qdr.ConfigUpdate {
	log.Printf("updating listener %s/%s...", latest.Namespace, latest.Name)
	name := latest.ObjectMeta.Name
	existing, ok := b.listeners[name]
	b.listeners[name] = latest
	if b.handler != nil {
		b.handler.ListenerUpdated(latest)
	}

	if !ok || !reflect.DeepEqual(existing.Spec, latest.Spec) {
		return b
	}
	return nil
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
		TcpListeners:  qdr.TcpEndpointMap{},
		TcpConnectors: qdr.TcpEndpointMap{},
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

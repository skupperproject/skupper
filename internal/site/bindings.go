package site

import (
	"reflect"

	"github.com/skupperproject/skupper/internal/qdr"
	skupperv2alpha1 "github.com/skupperproject/skupper/pkg/apis/skupper/v2alpha1"
)

type ListenerConfiguration func(siteId string, listener *skupperv2alpha1.Listener, config *qdr.BridgeConfig)
type ConnectorConfiguration func(siteId string, connector *skupperv2alpha1.Connector, config *qdr.BridgeConfig)
type MultiKeyListenerConfiguration func(siteId string, mkl *skupperv2alpha1.MultiKeyListener, config *qdr.BridgeConfig)

type BindingEventHandler interface {
	ListenerUpdated(listener *skupperv2alpha1.Listener)
	ListenerDeleted(listener *skupperv2alpha1.Listener)
	ConnectorUpdated(connector *skupperv2alpha1.Connector) bool
	ConnectorDeleted(connector *skupperv2alpha1.Connector)
}

type ConnectorFunction func(*skupperv2alpha1.Connector) *skupperv2alpha1.Connector
type ListenerFunction func(*skupperv2alpha1.Listener) *skupperv2alpha1.Listener
type MultiKeyListenerFunction func(*skupperv2alpha1.MultiKeyListener) *skupperv2alpha1.MultiKeyListener

type Bindings struct {
	SiteId            string
	ProfilePath       string
	connectors        map[string]*skupperv2alpha1.Connector
	listeners         map[string]*skupperv2alpha1.Listener
	multiKeyListeners map[string]*skupperv2alpha1.MultiKeyListener
	handler           BindingEventHandler
	configure         struct {
		listener         ListenerConfiguration
		connector        ConnectorConfiguration
		multiKeyListener MultiKeyListenerConfiguration
	}
}

func NewBindings(profilePath string) *Bindings {
	bindings := &Bindings{
		ProfilePath:       profilePath,
		connectors:        map[string]*skupperv2alpha1.Connector{},
		listeners:         map[string]*skupperv2alpha1.Listener{},
		multiKeyListeners: map[string]*skupperv2alpha1.MultiKeyListener{},
	}
	bindings.configure.listener = UpdateBridgeConfigForListener
	bindings.configure.connector = UpdateBridgeConfigForConnector
	bindings.configure.multiKeyListener = UpdateBridgeConfigForMultiKeyListener
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

func (b *Bindings) MapOverMultiKeyListeners(mkf MultiKeyListenerFunction) {
	if mkf != nil {
		for key, mkl := range b.multiKeyListeners {
			if updated := mkf(mkl); updated != nil {
				b.multiKeyListeners[key] = updated
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
	name := latest.ObjectMeta.Name
	existing, ok := b.listeners[name]
	b.listeners[name] = latest

	if !ok || !reflect.DeepEqual(existing.Spec, latest.Spec) {
		if b.handler != nil {
			b.handler.ListenerUpdated(latest)
		}
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

func (b *Bindings) GetMultiKeyListener(name string) *skupperv2alpha1.MultiKeyListener {
	if existing, ok := b.multiKeyListeners[name]; ok {
		return existing
	}
	return nil
}

func (b *Bindings) UpdateMultiKeyListener(name string, mkl *skupperv2alpha1.MultiKeyListener) qdr.ConfigUpdate {
	if mkl == nil {
		return b.deleteMultiKeyListener(name)
	}
	return b.updateMultiKeyListener(mkl)
}

func (b *Bindings) updateMultiKeyListener(mkl *skupperv2alpha1.MultiKeyListener) qdr.ConfigUpdate {
	name := mkl.ObjectMeta.Name
	existing, ok := b.multiKeyListeners[name]
	b.multiKeyListeners[name] = mkl
	if ok && reflect.DeepEqual(existing.Spec, mkl.Spec) {
		return nil
	}
	return b
}

func (b *Bindings) deleteMultiKeyListener(name string) qdr.ConfigUpdate {
	if _, ok := b.multiKeyListeners[name]; !ok {
		return nil
	}
	delete(b.multiKeyListeners, name)
	return b
}

func (b *Bindings) SetMultiKeyListenerConfiguration(configuration MultiKeyListenerConfiguration) {
	b.configure.multiKeyListener = configuration
}

func (b *Bindings) ToBridgeConfig() qdr.BridgeConfig {
	config := qdr.BridgeConfig{
		TcpListeners:      qdr.TcpEndpointMap{},
		TcpConnectors:     qdr.TcpEndpointMap{},
		ListenerAddresses: qdr.ListenerAddressMap{},
	}
	for _, c := range b.connectors {
		b.configure.connector(b.SiteId, c, &config)
	}
	for _, l := range b.listeners {
		b.configure.listener(b.SiteId, l, &config)
	}
	for _, mkl := range b.multiKeyListeners {
		b.configure.multiKeyListener(b.SiteId, mkl, &config)
	}

	return config
}

func (b *Bindings) AddSslProfiles(config *qdr.RouterConfig) bool {
	profiles := map[string]qdr.SslProfile{}
	for _, c := range b.connectors {
		if c.Spec.TlsCredentials != "" {
			if !c.Spec.UseClientCert {
				//if only ca is used, need to qualify the profile to ensure that it does not collide with
				// use of the same secret where client auth *is* required
				name := GetSslProfileName(c.Spec.TlsCredentials, c.Spec.UseClientCert)
				if _, ok := profiles[name]; !ok {
					profiles[name] = qdr.ConfigureSslProfile(name, b.ProfilePath, false)
				}
			} else {
				if _, ok := profiles[c.Spec.TlsCredentials]; !ok {
					profiles[c.Spec.TlsCredentials] = qdr.ConfigureSslProfile(c.Spec.TlsCredentials, b.ProfilePath, true)
				}
			}
		}
	}
	for _, l := range b.listeners {
		if _, ok := profiles[l.Spec.TlsCredentials]; l.Spec.TlsCredentials != "" && !ok {
			profiles[l.Spec.TlsCredentials] = qdr.ConfigureSslProfile(l.Spec.TlsCredentials, b.ProfilePath, true)
		}
	}
	for _, mkl := range b.multiKeyListeners {
		if _, ok := profiles[mkl.Spec.TlsCredentials]; mkl.Spec.TlsCredentials != "" && !ok {
			profiles[mkl.Spec.TlsCredentials] = qdr.ConfigureSslProfile(mkl.Spec.TlsCredentials, b.ProfilePath, true)
		}
	}
	changed := false
	for _, profile := range profiles {
		if config.AddSslProfile(profile) {
			changed = true
		}
	}
	return changed
}

func (b *Bindings) Apply(config *qdr.RouterConfig) bool {
	b.AddSslProfiles(config)
	config.UpdateBridgeConfig(b.ToBridgeConfig())
	config.RemoveUnreferencedSslProfiles()
	return true //TODO: can optimise by indicating if no change was required
}

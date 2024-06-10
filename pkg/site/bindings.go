package site

import (
	"log"

	skupperv1alpha1 "github.com/skupperproject/skupper/pkg/apis/skupper/v1alpha1"
	"github.com/skupperproject/skupper/pkg/qdr"
)

type TargetSelection interface {
	Update(connector *skupperv1alpha1.Connector)
	List() []string
	Close()
}

type BindingContext interface {
	Select(connector *skupperv1alpha1.Connector) TargetSelection
	Expose(ports *ExposedPortSet)
	Unexpose(host string)
}

type Bindings struct {
	SiteId     string
	context    BindingContext
	mapping    *qdr.PortMapping
	connectors map[string]*Connector
	listeners  map[string]*Listener
	exposed    ExposedPorts
}

func NewBindings() *Bindings {
	return &Bindings{
		connectors: map[string]*Connector{},
		listeners:  map[string]*Listener{},
		exposed:    ExposedPorts{},
	}
}

func (b *Bindings) SetBindingContext(context BindingContext) {
	b.context = context
	for _, sc := range b.connectors {
		sc.init(context)
	}
	for _, l := range b.listeners {
		b.expose(l)
	}
}

func (b *Bindings) CloseAllSelectedConnectors() {
	for _, c := range b.connectors {
		if c.selection != nil {
			c.selection.Close()
		}
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
	c, ok := b.connectors[name]
	if !ok {
		c = &Connector{
			resource: connector,
		}
		b.connectors[name] = c
		if c.init(b.context) {
			return b, nil
		}
	} else {
		c.resourceUpdated(connector)
		if c.resource.Spec != connector.Spec &&c.init(b.context) {
			return b, nil
		}
	}
	return nil, nil
}

func (b *Bindings) deleteConnector(name string) qdr.ConfigUpdate {
	if existing, ok := b.connectors[name]; ok {
		if existing.selection != nil {
			existing.selection.Close()
		}
		delete(b.connectors, name)
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
	if existing, ok := b.listeners[name]; !ok || existing.resource.Spec != latest.Spec {
		if !ok {
			existing = &Listener{
				resource: latest,
			}
			b.listeners[name] = existing
		} else {
			existing.resource = latest
		}
		b.expose(existing)
		log.Printf("Updating router config for listener %s/%s", latest.Namespace, latest.Name)
		return b, nil
	}
	log.Printf("No need to update router config for listener %s/%s", latest.Namespace, latest.Name)
	return nil, nil
}

func (b *Bindings) deleteListener(name string) qdr.ConfigUpdate {
	if _, ok := b.listeners[name]; ok {
		delete(b.listeners, name)
		if b.context != nil {
			b.context.Unexpose(name)
		}
		if b.mapping != nil {
			b.mapping.ReleasePortForKey(name)
		}
		return b
	}
	return nil
}

func (b *Bindings) ToBridgeConfig(mapping *qdr.PortMapping) qdr.BridgeConfig {
	config := qdr.BridgeConfig {
		TcpListeners:   qdr.TcpEndpointMap{},
		TcpConnectors:  qdr.TcpEndpointMap{},
		HttpListeners:  qdr.HttpEndpointMap{},
		HttpConnectors: qdr.HttpEndpointMap{},
	}
	for _, c := range b.connectors {
		c.updateBridges(b.SiteId, &config)
	}
	for _, l := range b.listeners {
		l.updateBridges(b.SiteId, mapping, &config)
	}

	return config
}

func (b *Bindings) RecoverPortMapping(config *qdr.RouterConfig) {
	if b.mapping == nil {
		b.mapping = qdr.RecoverPortMapping(config)
	}
}

func (b *Bindings) Apply(config *qdr.RouterConfig) bool {
	//TODO: add/remove SslProfiles as necessary
	config.UpdateBridgeConfig(b.ToBridgeConfig(b.mapping))
	return true //TODO: can optimise by indicating if no change was required
}

func (b *Bindings) expose(l *Listener)  {
	if b.mapping != nil {
		allocatedRouterPort, err := b.mapping.GetPortForKey(l.resource.Name)
		if err != nil {
			log.Printf("Unable to get port for listener %s/%s: %s", l.resource.Namespace, l.resource.Name, err)
		} else {
			port := Port {
				Name:       l.resource.Name,
				Port:       l.resource.Spec.Port,
				TargetPort: allocatedRouterPort,
				Protocol:   l.protocol(),
			}
			exposed := b.exposed.Expose(l.resource.Spec.Host, port)
			if exposed != nil && b.context != nil{
				b.context.Expose(exposed)
			}
		}
	}
}

func (b *Bindings) unexpose(name string, l *Listener)  {
	exposed := b.exposed.Unexpose(l.resource.Spec.Host, name)
	if exposed != nil && b.context != nil {
		if len(exposed.Ports) > 0 {
			b.context.Expose(exposed)
		} else {
			b.context.Unexpose(exposed.Host)
		}
	}
}

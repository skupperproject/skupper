package site

import (
	"errors"
	"log/slog"

	skupperv2alpha1 "github.com/skupperproject/skupper/pkg/apis/skupper/v2alpha1"
	"github.com/skupperproject/skupper/pkg/kube"
	"github.com/skupperproject/skupper/pkg/qdr"
	"github.com/skupperproject/skupper/pkg/site"
)

type ExtendedBindings struct {
	context            BindingContext
	mapping            *qdr.PortMapping
	exposed            ExposedPorts
	selectors          map[string]TargetSelection
	bindings           *site.Bindings
	connectors         map[string]*AttachedConnector
	perTargetListeners map[string]*PerTargetListener
	controller         *kube.Controller
	site               *Site
	logger             *slog.Logger
}

func NewExtendedBindings(controller *kube.Controller, profilePath string) *ExtendedBindings {
	eb := &ExtendedBindings{
		bindings:           site.NewBindings(profilePath),
		connectors:         map[string]*AttachedConnector{},
		perTargetListeners: map[string]*PerTargetListener{},
		controller:         controller,
		logger: slog.New(slog.Default().Handler()).With(
			slog.String("component", "kube.site.attached_connector"),
		),
	}
	eb.bindings.SetBindingEventHandler(eb)
	eb.bindings.SetConnectorConfiguration(eb.updateBridgeConfigForConnector)
	eb.bindings.SetListenerConfiguration(eb.updateBridgeConfigForListener)

	return eb
}

func (a *ExtendedBindings) init(context BindingContext, config *qdr.RouterConfig) {
	a.context = context
	if a.mapping == nil {
		a.mapping = qdr.RecoverPortMapping(config)
	}
	a.exposed = ExposedPorts{}
	a.selectors = map[string]TargetSelection{}
}

func (a *ExtendedBindings) cleanup() {
	for _, s := range a.selectors {
		s.Close()
	}
}

func (a *ExtendedBindings) ConnectorUpdated(connector *skupperv2alpha1.Connector) bool {
	if selector, ok := a.selectors[connector.Name]; ok {
		if selector.Selector() == connector.Spec.Selector {
			// don't need to change the pod watcher, but may need to reconfigure for other change to spec
			return true
		} else {
			// selector has changed so need to close current pod watcher
			selector.Close()
			if connector.Spec.Selector == "" {
				// no longer using a selector, so just delete the old watcher
				delete(a.selectors, connector.Name)
				return true
			}
			// else create a new watcher below
		}
	} else if connector.Spec.Selector == "" {
		return true
	}
	a.selectors[connector.Name] = a.context.Select(connector)
	// can't yet update configuration; need to wait for the new
	// watcher to return any matching pods and update config at
	// that point
	return false
}

func (a *ExtendedBindings) ConnectorDeleted(connector *skupperv2alpha1.Connector) {
	if current, ok := a.selectors[connector.Name]; ok {
		current.Close()
		delete(a.selectors, connector.Name)
	}
}

func (a *ExtendedBindings) ListenerUpdated(listener *skupperv2alpha1.Listener) {
	allocatedRouterPort, err := a.mapping.GetPortForKey(listener.Name)
	if err != nil {
		bindings_logger.Error("Unable to get port for listener",
			slog.String("namespace", listener.Namespace),
			slog.String("name", listener.Name))
		slog.Any("error", err)
	} else {
		port := Port{
			Name:       listener.Name,
			Port:       listener.Spec.Port,
			TargetPort: allocatedRouterPort,
			Protocol:   listener.Protocol(),
		}
		if exposed := a.exposed.Expose(listener.Spec.Host, port); exposed != nil {
			if err := a.context.Expose(exposed); err != nil {
				//TODO: write error to listener status
			}
		}
	}
}

func (a *ExtendedBindings) ListenerDeleted(listener *skupperv2alpha1.Listener) {
	if exposed := a.exposed.Unexpose(listener.Spec.Host, listener.Name); exposed != nil {
		a.mapping.ReleasePortForKey(listener.Name)
		if exposed.empty() {
			if err := a.context.Unexpose(listener.Spec.Host); err != nil {
				//TODO: write error to listener status
			}
		} else {
			if err := a.context.Expose(exposed); err != nil {
				//TODO: write error to listener status
			}
		}
	}
}

func (a *ExtendedBindings) updateBridgeConfigForConnector(siteId string, connector *skupperv2alpha1.Connector, config *qdr.BridgeConfig) {
	if connector.Spec.Host != "" {
		site.UpdateBridgeConfigForConnector(siteId, connector, config)
	} else if connector.Spec.Selector != "" {
		if selector, ok := a.selectors[connector.Name]; ok {
			for _, pod := range selector.List() {
				site.UpdateBridgeConfigForConnectorToPod(siteId, connector, pod, connector.Spec.ExposePodsByName, config)
			}
		} else {
			bindings_logger.Error("Not yet tracking pods for connector with selector set",
				slog.String("namespace", connector.Namespace),
				slog.String("name", connector.Name))
		}
	} else {
		bindings_logger.Error("Connector has neither host nor selector set",
			slog.String("namespace", connector.Namespace),
			slog.String("name", connector.Name))
	}
}

func (a *ExtendedBindings) updateBridgeConfigForListener(siteId string, listener *skupperv2alpha1.Listener, config *qdr.BridgeConfig) {
	if port, err := a.mapping.GetPortForKey(listener.Name); err == nil {
		site.UpdateBridgeConfigForListenerWithHostAndPort(siteId, listener, "", port, config)
	} else {
		bindings_logger.Error("Could not allocate port for %s/%s: %s",
			slog.String("namespace", listener.Namespace),
			slog.String("name", listener.Name))
	}
}

func (b *ExtendedBindings) SetListenerConfiguration(configuration site.ListenerConfiguration) {
	b.bindings.SetListenerConfiguration(configuration)
}

func (b *ExtendedBindings) SetConnectorConfiguration(configuration site.ConnectorConfiguration) {
	b.bindings.SetConnectorConfiguration(configuration)
}

func (b *ExtendedBindings) SetBindingEventHandler(handler site.BindingEventHandler) {
	b.bindings.SetBindingEventHandler(handler)
}

func (b *ExtendedBindings) UpdateConnector(name string, connector *skupperv2alpha1.Connector) qdr.ConfigUpdate {
	return b.bindings.UpdateConnector(name, connector)
}

func (b *ExtendedBindings) UpdateListener(name string, listener *skupperv2alpha1.Listener) (qdr.ConfigUpdate, error) {
	var errs []error
	updateConfig := false
	if listener != nil && listener.Spec.ExposePodsByName {
		if existing, ok := b.perTargetListeners[name]; ok {
			if existing.updateListener(listener) {
				if err := existing.expose(b.mapping, b.exposed, b.context); err != nil {
					errs = append(errs, err)
				}
				updateConfig = true
			}
		} else {
			b.perTargetListeners[name] = newPerTargetListener(listener)
		}
	} else {
		if existing, ok := b.perTargetListeners[name]; ok {
			delete(b.perTargetListeners, name)
			if err := existing.unexposeAll(b.mapping, b.exposed, b.context); err != nil {
				errs = append(errs, err)
			}

			updateConfig = true
		}
	}
	if b.bindings.UpdateListener(name, listener) != nil {
		updateConfig = true
	}
	if !updateConfig {
		return nil, errors.Join(errs...)
	}
	return b, errors.Join(errs...)
}

func (b *ExtendedBindings) GetConnector(name string) *skupperv2alpha1.Connector {
	return b.bindings.GetConnector(name)
}

func (b *ExtendedBindings) Map(cf site.ConnectorFunction, lf site.ListenerFunction) {
	b.bindings.Map(cf, lf)
}

type AttachedConnectorFunction func(*AttachedConnector)

func (b *ExtendedBindings) MapOverAttachedConnectors(cf AttachedConnectorFunction) {
	for _, value := range b.connectors {
		cf(value)
	}
}

func (b *ExtendedBindings) Apply(config *qdr.RouterConfig) bool {
	desired := b.bindings.ToBridgeConfig()
	for _, connector := range b.connectors {
		connector.updateBridgeConfig(b.bindings.SiteId, &desired)
	}
	for _, ptl := range b.perTargetListeners {
		ptl.updateBridgeConfig(b.bindings.SiteId, &desired)
	}
	b.bindings.AddSslProfiles(config)
	config.UpdateBridgeConfig(desired)
	config.RemoveUnreferencedSslProfiles()
	return true //TODO: can optimise by indicating if no change was required
}

func (b *ExtendedBindings) SetSite(site *Site) {
	b.bindings.SetSiteId(site.site.GetSiteId())
	b.site = site
}

func (b *ExtendedBindings) checkAttachedConnectorBinding(namespace string, name string, binding *skupperv2alpha1.AttachedConnectorBinding) error {
	connector, ok := b.connectors[name]
	if !ok {
		connector = NewAttachedConnector(name, namespace, b)
		b.connectors[name] = connector
	}
	if (binding == nil && connector.bindingDeleted()) || (binding != nil && connector.bindingUpdated(binding)) {
		if b.site != nil {
			if err := b.site.updateRouterConfigForGroups(b.site.bindings); err != nil {
				return connector.configurationError(err)
			} else {
				return connector.updateStatus()
			}
		}
	}
	return nil
}

func (b *ExtendedBindings) attachedConnectorUpdated(name string, definition *skupperv2alpha1.AttachedConnector) error {
	connector, ok := b.connectors[name]
	if !ok {
		connector = NewAttachedConnector(name, definition.Spec.SiteNamespace, b)
		b.connectors[name] = connector
	}
	if connector.definitionUpdated(definition) {
		if b.site != nil {
			if err := b.site.updateRouterConfigForGroups(b.site.bindings); err != nil {
				return connector.configurationError(err)
			} else {
				return connector.updateStatus()
			}
		}
	}
	return nil
}

func (b *ExtendedBindings) attachedConnectorDeleted(namespace string, name string) error {
	if connector, ok := b.connectors[name]; ok && connector.definitionDeleted(namespace) {
		if b.site != nil {
			if err := b.site.updateRouterConfigForGroups(b.site.bindings); err != nil {
				return connector.configurationError(err)
			} else {
				return connector.updateStatus()
			}
		}
	}
	return nil
}

func (b *ExtendedBindings) networkUpdated(network []skupperv2alpha1.SiteRecord) qdr.ConfigUpdate {
	changed := false
	for _, ptl := range b.perTargetListeners {
		update, err := ptl.extractTargets(network, b.mapping, b.exposed, b.context)
		if err != nil {
			if err := b.site.updateListenerStatus(ptl.definition, err); err != nil {
				bindings_logger.Error("Error handling network update for listener",
					slog.String("namespace", ptl.definition.Namespace),
					slog.String("name", ptl.definition.Name))
				slog.Any("error", err)
			}
		}
		if update {
			changed = true
		}
	}
	if !changed {
		return nil
	}
	return b
}

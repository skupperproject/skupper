package site

import (
	"context"
	"fmt"
	"log/slog"
	"reflect"
	"strings"

	skupperv2alpha1 "github.com/skupperproject/skupper/pkg/apis/skupper/v2alpha1"
	"github.com/skupperproject/skupper/pkg/kube"
	"github.com/skupperproject/skupper/pkg/qdr"
	"github.com/skupperproject/skupper/pkg/site"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type ExtendedBindings struct {
	bindings   *site.Bindings
	connectors map[string]*AttachedConnector
	controller *kube.Controller
	site       *Site
	logger     *slog.Logger
}

func NewExtendedBindings(controller *kube.Controller) *ExtendedBindings {
	return &ExtendedBindings{
		bindings:   site.NewBindings(),
		connectors: map[string]*AttachedConnector{},
		controller: controller,
		logger: slog.New(slog.Default().Handler()).With(
			slog.String("component", "kube.site.attached_connector"),
		),
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

func (b *ExtendedBindings) UpdateListener(name string, listener *skupperv2alpha1.Listener) qdr.ConfigUpdate {
	return b.bindings.UpdateListener(name, listener)
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
	//TODO: add/remove SslProfiles as necessary
	config.UpdateBridgeConfig(desired)
	return true //TODO: can optimise by indicating if no change was required
}

func (b *ExtendedBindings) SetSite(site *Site) {
	b.bindings.SetSiteId(site.site.GetSiteId())
	b.site = site
}

func (b *ExtendedBindings) checkAttachedConnectorAnchor(namespace string, name string, anchor *skupperv2alpha1.AttachedConnectorAnchor) error {
	connector, ok := b.connectors[name]
	if !ok {
		connector = NewAttachedConnector(name, namespace, b)
		b.connectors[name] = connector
	}
	if (anchor == nil && connector.anchorDeleted()) || (anchor != nil && connector.anchorUpdated(anchor)) {
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

type AttachedConnector struct {
	name        string
	namespace   string
	definitions map[string]*skupperv2alpha1.AttachedConnector
	anchor      *skupperv2alpha1.AttachedConnectorAnchor
	watcher     *PodWatcher
	parent      *ExtendedBindings
}

func NewAttachedConnector(name string, namespace string, parent *ExtendedBindings) *AttachedConnector {
	return &AttachedConnector{
		name:        name,
		namespace:   namespace,
		parent:      parent,
		definitions: map[string]*skupperv2alpha1.AttachedConnector{},
	}
}

func (a *AttachedConnector) activeDefinition() *skupperv2alpha1.AttachedConnector {
	if a.anchor == nil {
		return nil
	}
	if definition, ok := a.definitions[a.anchor.Spec.ConnectorNamespace]; ok {
		return definition
	}
	return nil
}

func (a *AttachedConnector) Selector() string {
	if definition := a.activeDefinition(); definition != nil {
		return definition.Spec.Selector
	}
	return ""
}

func (a *AttachedConnector) IncludeNotReady() bool {
	if definition := a.activeDefinition(); definition != nil {
		return definition.Spec.IncludeNotReady
	}
	return false
}

func (a *AttachedConnector) Attr() slog.Attr {
	if definition := a.activeDefinition(); definition != nil {
		return slog.Group("AttachedConnector",
			slog.Bool("Active", true),
			slog.String("Name", definition.Name),
			slog.String("Namespace", definition.Namespace))
	}
	return slog.Group("AttachedConnector",
		slog.Bool("Active", false),
		slog.String("Name", a.name),
		slog.String("Namespace", a.namespace))
}

func error_(errors []string) error {
	if len(errors) > 0 {
		return fmt.Errorf("Error(s) encountered: %s", strings.Join(errors, ", "))
	}
	return nil
}

func (a *AttachedConnector) updateStatus() error {
	if a.anchor == nil {
		return a.updateStatusNoAnchor()
	}
	if active := a.activeDefinition(); active != nil {
		if a.watcher == nil {
			return a.updateStatusTo(fmt.Errorf("Not ready"), active)
		} else if len(a.watcher.pods()) == 0 {
			return a.updateStatusTo(fmt.Errorf("No matches for selector"), active)
		} else {
			return a.updateStatusTo(nil, active)
		}
	} else {
		return a.updateStatusTo(fmt.Errorf("No matching AttachedConnector"), nil)
	}
}

func (a *AttachedConnector) updateStatusNoAnchor() error {
	var errors []string
	for _, definition := range a.definitions {
		if definition.SetConfigured(fmt.Errorf("No matching AttachedConnectorAnchor in site namespace")) {
			if err := a.updateDefinitionStatus(definition); err != nil {
				errors = append(errors, err.Error())
			}
		}
	}
	return error_(errors)
}

func (a *AttachedConnector) updateStatusTo(err error, activeDefinition *skupperv2alpha1.AttachedConnector) error {
	var errors []string
	if a.anchor.SetConfigured(err) {
		if err := a.updateAnchorStatus(); err != nil {
			errors = append(errors, err.Error())
		}
	}
	if activeDefinition != nil && activeDefinition.SetConfigured(err) {
		if err := a.updateDefinitionStatus(activeDefinition); err != nil {
			errors = append(errors, err.Error())
		}
	}
	for _, definition := range a.definitions {
		if definition.Namespace == a.anchor.Spec.ConnectorNamespace {
			continue
		}
		if definition.SetConfigured(fmt.Errorf("AttachedConnectorAnchor %s/%s does not allow AttachedConnector in %s (only %s)", a.anchor.Namespace, a.anchor.Name, definition.Namespace, a.anchor.Spec.ConnectorNamespace)) {
			if err := a.updateDefinitionStatus(definition); err != nil {
				errors = append(errors, err.Error())
			}
		}
	}
	return error_(errors)
}

func (a *AttachedConnector) setMatchingListenerCount(count int) {
	if a.anchor.SetMatchingListenerCount(count) {
		if err := a.updateAnchorStatus(); err != nil {
			a.parent.logger.Error("Failed to update AttachedConnectorAnchor",
				slog.String("namespace", a.anchor.Namespace),
				slog.String("name", a.anchor.Name),
				slog.Any("error", err))
		}
	}
}

func (a *AttachedConnector) Updated(pods []skupperv2alpha1.PodDetails) error {
	if a.anchor == nil {
		return a.updateStatusNoAnchor()
	}
	definition := a.activeDefinition()
	if definition == nil {
		return a.updateStatusTo(fmt.Errorf("No matching AttachedConnector"), nil)
	}
	err := a.parent.site.updateRouterConfigForGroups(a.parent.site.bindings)
	if err != nil {
		return a.updateStatusTo(err, definition)
	}
	if len(pods) == 0 {
		a.parent.logger.Info("No pods available for selector",
			slog.String("namespace", definition.Namespace),
			slog.String("name", definition.Name))
		return a.updateStatusTo(fmt.Errorf("No matches for selector"), definition)
	}
	a.parent.logger.Info("Pods are available for selector",
		slog.String("namespace", definition.Namespace),
		slog.String("name", definition.Name))
	return a.updateStatusTo(nil, definition)
}

func (a *AttachedConnector) configurationError(err error) error {
	if a.activeDefinition() == nil || a.anchor == nil {
		return nil
	}
	return err
}

func (a *AttachedConnector) updateDefinitionStatus(definition *skupperv2alpha1.AttachedConnector) error {
	updated, err := a.parent.controller.GetSkupperClient().SkupperV2alpha1().AttachedConnectors(definition.ObjectMeta.Namespace).UpdateStatus(context.TODO(), definition, metav1.UpdateOptions{})
	if err != nil {
		a.parent.logger.Error("Failed to update status for selector",
			slog.String("namespace", definition.Namespace),
			slog.String("name", definition.Name),
			slog.Any("error", err))
		return err
	}
	a.definitions[updated.Namespace] = updated
	return nil
}

func (a *AttachedConnector) updateAnchorStatus() error {
	if a.anchor == nil {
		return nil
	}
	updated, err := a.parent.controller.GetSkupperClient().SkupperV2alpha1().AttachedConnectorAnchors(a.anchor.ObjectMeta.Namespace).UpdateStatus(context.TODO(), a.anchor, metav1.UpdateOptions{})
	if err != nil {
		a.parent.logger.Error("Failed to update status for AttachedConnectorAnchor",
			slog.String("namespace", a.anchor.Namespace),
			slog.String("name", a.anchor.Name),
			slog.Any("error", err))
		return err
	}
	a.anchor = updated
	return nil
}

func (a *AttachedConnector) watchPods() {
	if a.watcher != nil {
		a.watcher.Close()
	}
	if a.parent.site != nil {
		if active := a.activeDefinition(); active != nil {
			a.watcher = a.parent.site.WatchPods(a, active.Namespace)
		}
	}
}

func (a *AttachedConnector) definitionUpdated(definition *skupperv2alpha1.AttachedConnector) bool {
	specChanged := true
	selectorChanged := true
	if existing, ok := a.definitions[definition.Namespace]; ok {
		if reflect.DeepEqual(existing.Spec, definition.Spec) {
			specChanged = false
			selectorChanged = false
			slog.Debug("Spec has not changed for AttachedConnector",
				slog.String("namespace", definition.Namespace),
				slog.String("name", definition.Name))
		} else if existing.Spec.Selector == definition.Spec.Selector {
			selectorChanged = false
			slog.Debug("Selector has not changed for AttachedConnector",
				slog.String("namespace", definition.Namespace),
				slog.String("name", definition.Name))
		}
	}
	a.definitions[definition.Namespace] = definition
	if a.anchor != nil && a.anchor.Spec.ConnectorNamespace == definition.Namespace {
		if selectorChanged || a.watcher == nil {
			a.parent.logger.Info("Watching pods for AttachedConnector",
				slog.String("namespace", definition.Namespace),
				slog.String("name", definition.Name))
			a.watchPods()
			return false // not ready to configure until selector returns pods
		}
		return specChanged && a.watcher != nil
	} else if a.anchor == nil {
		if definition.SetConfigured(fmt.Errorf("No matching AttachedConnectorAnchor in site namespace")) {
			if err := a.updateDefinitionStatus(definition); err != nil {
				a.parent.logger.Error("Error updating status for AttachedConnector",
					slog.String("namespace", definition.Namespace),
					slog.String("name", definition.Name),
					slog.Any("error", err))
			}
		}
		return false
	} else {
		if definition.SetConfigured(fmt.Errorf("AttachedConnectorAnchor %s/%s does not allow AttachedConnector in %s (only %s)", a.anchor.Namespace, a.anchor.Name, definition.Namespace, a.anchor.Spec.ConnectorNamespace)) {
			if err := a.updateDefinitionStatus(definition); err != nil {
				a.parent.logger.Error("Error updating status for AttachedConnector",
					slog.String("namespace", definition.Namespace),
					slog.String("name", definition.Name),
					slog.Any("error", err))
			}
		}
		return false
	}
}

func (a *AttachedConnector) anchorUpdated(anchor *skupperv2alpha1.AttachedConnectorAnchor) bool {
	changed := a.anchor == nil || !reflect.DeepEqual(a.anchor.Spec, anchor.Spec)
	a.anchor = anchor
	return changed
}

func (a *AttachedConnector) definitionDeleted(namespace string) bool {
	if _, ok := a.definitions[namespace]; ok {
		if a.watcher != nil {
			a.watcher.Close()
		}
		delete(a.definitions, namespace)
		return true
	}
	return false
}

func (a *AttachedConnector) anchorDeleted() bool {
	if a.anchor == nil {
		return false
	}
	a.anchor = nil
	return true
}

func (a *AttachedConnector) updateBridgeConfig(siteId string, config *qdr.BridgeConfig) {
	definition := a.activeDefinition()
	if definition == nil || a.watcher == nil {
		return
	}
	connector := &skupperv2alpha1.Connector{
		ObjectMeta: metav1.ObjectMeta{
			Name: definition.Name,
		},
		Spec: skupperv2alpha1.ConnectorSpec{
			RoutingKey:     a.anchor.Spec.RoutingKey,
			Type:           definition.Spec.Type,
			Port:           definition.Spec.Port,
			TlsCredentials: definition.Spec.TlsCredentials,
		},
	}
	for _, pod := range a.watcher.pods() {
		site.UpdateBridgeConfigForConnectorToPod(siteId, connector, pod, config)
	}
}

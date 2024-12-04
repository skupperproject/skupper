package site

import (
	"context"
	"fmt"
	"log/slog"
	"reflect"
	"strings"

	skupperv2alpha1 "github.com/skupperproject/skupper/pkg/apis/skupper/v2alpha1"
	"github.com/skupperproject/skupper/pkg/qdr"
	"github.com/skupperproject/skupper/pkg/site"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type AttachedConnector struct {
	name        string
	namespace   string
	definitions map[string]*skupperv2alpha1.AttachedConnector
	binding     *skupperv2alpha1.AttachedConnectorBinding
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
	if a.binding == nil {
		return nil
	}
	if definition, ok := a.definitions[a.binding.Spec.ConnectorNamespace]; ok {
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

func (a *AttachedConnector) IncludeNotReadyPods() bool {
	if definition := a.activeDefinition(); definition != nil {
		return definition.Spec.IncludeNotReadyPods
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
	if a.binding == nil {
		return a.updateStatusNoBinding()
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

func (a *AttachedConnector) updateStatusNoBinding() error {
	var errors []string
	for _, definition := range a.definitions {
		if definition.SetConfigured(fmt.Errorf("No matching AttachedConnectorBinding in site namespace")) {
			if err := a.updateDefinitionStatus(definition); err != nil {
				errors = append(errors, err.Error())
			}
		}
	}
	return error_(errors)
}

func (a *AttachedConnector) updateStatusTo(err error, activeDefinition *skupperv2alpha1.AttachedConnector) error {
	var errors []string
	if a.binding.SetConfigured(err) {
		if err := a.updateBindingStatus(); err != nil {
			errors = append(errors, err.Error())
		}
	}
	if activeDefinition != nil && activeDefinition.SetConfigured(err) {
		if err := a.updateDefinitionStatus(activeDefinition); err != nil {
			errors = append(errors, err.Error())
		}
	}
	for _, definition := range a.definitions {
		if definition.Namespace == a.binding.Spec.ConnectorNamespace {
			continue
		}
		if definition.SetConfigured(fmt.Errorf("AttachedConnectorBinding %s/%s does not allow AttachedConnector in %s (only %s)", a.binding.Namespace, a.binding.Name, definition.Namespace, a.binding.Spec.ConnectorNamespace)) {
			if err := a.updateDefinitionStatus(definition); err != nil {
				errors = append(errors, err.Error())
			}
		}
	}
	return error_(errors)
}

func (a *AttachedConnector) setMatchingListenerCount(count int) {
	if a.binding.SetHasMatchingListener(count > 0) {
		if err := a.updateBindingStatus(); err != nil {
			a.parent.logger.Error("Failed to update AttachedConnectorBinding",
				slog.String("namespace", a.binding.Namespace),
				slog.String("name", a.binding.Name),
				slog.Any("error", err))
		}
	}
}

func (a *AttachedConnector) Updated(pods []skupperv2alpha1.PodDetails) error {
	if a.binding == nil {
		return a.updateStatusNoBinding()
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
	if a.activeDefinition() == nil || a.binding == nil {
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

func (a *AttachedConnector) updateBindingStatus() error {
	if a.binding == nil {
		return nil
	}
	updated, err := a.parent.controller.GetSkupperClient().SkupperV2alpha1().AttachedConnectorBindings(a.binding.ObjectMeta.Namespace).UpdateStatus(context.TODO(), a.binding, metav1.UpdateOptions{})
	if err != nil {
		a.parent.logger.Error("Failed to update status for AttachedConnectorBinding",
			slog.String("namespace", a.binding.Namespace),
			slog.String("name", a.binding.Name),
			slog.Any("error", err))
		return err
	}
	a.binding = updated
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
	if a.binding != nil && a.binding.Spec.ConnectorNamespace == definition.Namespace {
		if selectorChanged || a.watcher == nil {
			a.parent.logger.Info("Watching pods for AttachedConnector",
				slog.String("namespace", definition.Namespace),
				slog.String("name", definition.Name))
			a.watchPods()
			return false // not ready to configure until selector returns pods
		}
		return specChanged && a.watcher != nil
	} else if a.binding == nil {
		if definition.SetConfigured(fmt.Errorf("No matching AttachedConnectorBinding in site namespace")) {
			if err := a.updateDefinitionStatus(definition); err != nil {
				a.parent.logger.Error("Error updating status for AttachedConnector",
					slog.String("namespace", definition.Namespace),
					slog.String("name", definition.Name),
					slog.Any("error", err))
			}
		}
		return false
	} else {
		if definition.SetConfigured(fmt.Errorf("AttachedConnectorBinding %s/%s does not allow AttachedConnector in %s (only %s)", a.binding.Namespace, a.binding.Name, definition.Namespace, a.binding.Spec.ConnectorNamespace)) {
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

func (a *AttachedConnector) bindingUpdated(binding *skupperv2alpha1.AttachedConnectorBinding) bool {
	changed := a.binding == nil || !reflect.DeepEqual(a.binding.Spec, binding.Spec)
	a.binding = binding
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

func (a *AttachedConnector) bindingDeleted() bool {
	if a.binding == nil {
		return false
	}
	a.binding = nil
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
			RoutingKey:     a.binding.Spec.RoutingKey,
			Type:           definition.Spec.Type,
			Port:           definition.Spec.Port,
			TlsCredentials: definition.Spec.TlsCredentials,
		},
	}
	for _, pod := range a.watcher.pods() {
		site.UpdateBridgeConfigForConnectorToPod(siteId, connector, pod, a.binding.Spec.ExposePodsByName, config)
	}
}

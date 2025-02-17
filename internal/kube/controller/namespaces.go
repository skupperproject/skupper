package controller

import (
	"log/slog"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	internalclient "github.com/skupperproject/skupper/internal/kube/client"
)

const (
	namespaceConfigName  = "skupper"
	controllerSettingKey = "controller"
)

type NamespaceConfig struct {
	config                 map[string]*corev1.ConfigMap
	watcher                *internalclient.ConfigMapWatcher
	controllerName         string
	requireExplicitControl bool
	logging                ControlLogging
}

func newNamespaceConfig(controllerName string, requireExplicitControl bool, logging ControlLogging) *NamespaceConfig {
	return &NamespaceConfig{
		config:                 map[string]*corev1.ConfigMap{},
		controllerName:         controllerName,
		requireExplicitControl: requireExplicitControl,
		logging:                logging,
	}
}

func (c *NamespaceConfig) update(key string, cm *corev1.ConfigMap) error {
	if cm == nil {
		delete(c.config, key)
		return nil
	}
	c.config[key] = cm
	return nil
}

func (c *NamespaceConfig) controller(namespace string) (string, bool) {
	if controller, ok := c.get(namespace, controllerSettingKey); ok {
		if strings.Contains(controller, "/") {
			return controller, true
		}
		return namespace + "/" + controller, true
	}
	return "", false
}

func (c *NamespaceConfig) isControlled(namespace string) bool {
	if controller, ok := c.controller(namespace); ok {
		if controller != c.controllerName {
			c.logging.NamespaceNotControlled(namespace, controller)
			return false
		}
		return true
	}
	if c.requireExplicitControl {
		c.logging.NamespaceNotControlled(namespace, "")
		return false
	}
	return true
}

func (c *NamespaceConfig) get(namespace string, setting string) (string, bool) {
	key := namespace + "/" + namespaceConfigName
	cm, ok := c.config[key]
	if !ok {
		return "", false
	}
	value, ok := cm.Data[setting]
	return value, ok
}

func (c *NamespaceConfig) watch(controller *internalclient.Controller, namespace string) {
	options := func(options *metav1.ListOptions) {
		options.FieldSelector = "metadata.name=" + namespaceConfigName
	}
	c.watcher = controller.WatchConfigMaps(options, namespace, c.update)
}

func (c *NamespaceConfig) recover() {
	if c.watcher != nil {
		for _, config := range c.watcher.List() {
			c.update(config.Namespace+"/"+config.Name, config)
		}
	}
}

type ControlLogging interface {
	NamespaceNotControlled(namespace string, actualControllerName string)
}

type OneTimeControlLogging struct {
	logged map[string]string
	impl   ControlLogging
}

type ClusterScopedControlLogging struct {
	log *slog.Logger
}

type NamespaceScopedControlLogging struct {
	log *slog.Logger
}

func (l *ClusterScopedControlLogging) NamespaceNotControlled(namespace string, actualControllerName string) {
	if actualControllerName != "" {
		l.log.Info("Ignoring resources in namespace controlled by another controller",
			slog.String("namespace", namespace),
			slog.String("controlled-by", actualControllerName),
		)
	} else {
		l.log.Info("Ignoring resources in uncontrolled namespace",
			slog.String("namespace", namespace),
		)
	}
}

func (l *NamespaceScopedControlLogging) NamespaceNotControlled(namespace string, actualControllerName string) {
	if actualControllerName != "" {
		l.log.Warn("Ignoring all resources as this namespace is controlled by another controller",
			slog.String("namespace", namespace),
			slog.String("controlled-by", actualControllerName),
		)
	} else {
		l.log.Warn("Ignoring all resources as this namespace has no controller assigned",
			slog.String("namespace", namespace),
		)
	}
}

func (l *OneTimeControlLogging) NamespaceNotControlled(namespace string, actualControllerName string) {
	if controller, ok := l.logged[namespace]; !ok || controller != actualControllerName {
		l.logged[namespace] = actualControllerName
		l.impl.NamespaceNotControlled(namespace, actualControllerName)
	}
}

func newControlLogging(clusterScoped bool, log *slog.Logger) ControlLogging {
	var impl ControlLogging
	if clusterScoped {
		impl = &ClusterScopedControlLogging{
			log: log,
		}
	} else {
		impl = &NamespaceScopedControlLogging{
			log: log,
		}
	}
	return &OneTimeControlLogging{
		logged: map[string]string{},
		impl:   impl,
	}
}

package site

import (
	"fmt"
	"log/slog"

	corev1 "k8s.io/api/core/v1"

	skupperv2alpha1 "github.com/skupperproject/skupper/pkg/apis/skupper/v2alpha1"
	"github.com/skupperproject/skupper/pkg/kube"
	"github.com/skupperproject/skupper/pkg/qdr"
	"github.com/skupperproject/skupper/pkg/site"
)

var bindings_logger *slog.Logger

func init() {
	bindings_logger = slog.New(slog.Default().Handler()).With(
		slog.String("component", "kube.site.bindings"),
	)
}

type BindingContext interface {
	Select(connector *skupperv2alpha1.Connector) TargetSelection
	Expose(ports *ExposedPortSet)
	Unexpose(host string)
}

type BindingAdaptor struct {
	context   BindingContext
	mapping   *qdr.PortMapping
	exposed   ExposedPorts
	selectors map[string]TargetSelection
}

func (a *BindingAdaptor) init(context BindingContext, config *qdr.RouterConfig) {
	a.context = context
	if a.mapping == nil {
		a.mapping = qdr.RecoverPortMapping(config)
	}
	a.exposed = ExposedPorts{}
	a.selectors = map[string]TargetSelection{}
}

func (a *BindingAdaptor) cleanup() {
	for _, s := range a.selectors {
		s.Close()
	}
}

func (a *BindingAdaptor) ConnectorUpdated(connector *skupperv2alpha1.Connector) bool {
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

func (a *BindingAdaptor) ConnectorDeleted(connector *skupperv2alpha1.Connector) {
	if current, ok := a.selectors[connector.Name]; ok {
		current.Close()
		delete(a.selectors, connector.Name)
	}
}

func (a *BindingAdaptor) ListenerUpdated(listener *skupperv2alpha1.Listener) {
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
			a.context.Expose(exposed)
		}
	}
}

func (a *BindingAdaptor) ListenerDeleted(listener *skupperv2alpha1.Listener) {
	a.context.Unexpose(listener.Spec.Host)
	a.mapping.ReleasePortForKey(listener.Name)
}

func (a *BindingAdaptor) updateBridgeConfigForConnector(siteId string, connector *skupperv2alpha1.Connector, config *qdr.BridgeConfig) {
	if connector.Spec.Host != "" {
		site.UpdateBridgeConfigForConnector(siteId, connector, config)
	} else if connector.Spec.Selector != "" {
		if selector, ok := a.selectors[connector.Name]; ok {
			for _, pod := range selector.List() {
				site.UpdateBridgeConfigForConnectorToPod(siteId, connector, pod, config)
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

func (a *BindingAdaptor) updateBridgeConfigForListener(siteId string, listener *skupperv2alpha1.Listener, config *qdr.BridgeConfig) {
	if port, err := a.mapping.GetPortForKey(listener.Name); err == nil {
		site.UpdateBridgeConfigForListenerWithHostAndPort(siteId, listener, "0.0.0.0", port, config)
	} else {
		bindings_logger.Error("Could not allocate port for %s/%s: %s",
			slog.String("namespace", listener.Namespace),
			slog.String("name", listener.Name))
	}
}

type TargetSelection interface {
	Selector() string
	Close()
	List() []skupperv2alpha1.PodDetails
}

type TargetSelectionImpl struct {
	watcher         *PodWatcher
	site            *Site
	selector        string
	name            string
	namespace       string
	includeNotReady bool
}

func (w *TargetSelectionImpl) Selector() string {
	return w.selector
}

func (w *TargetSelectionImpl) Close() {
	w.watcher.Close()
}

func (w *TargetSelectionImpl) IncludeNotReady() bool {
	return w.includeNotReady
}

func (w *TargetSelectionImpl) Attr() slog.Attr {
	return slog.Group("Connector",
		slog.String("Name", w.name),
		slog.String("Namespace", w.namespace))
}

func (w *TargetSelectionImpl) List() []skupperv2alpha1.PodDetails {
	return w.watcher.pods()
}

func (w *TargetSelectionImpl) Updated(pods []skupperv2alpha1.PodDetails) error {
	err := w.site.updateRouterConfigForGroups(w.site.bindings)
	connector := w.site.bindings.GetConnector(w.name)
	if connector == nil {
		return fmt.Errorf("Error looking up connector for %s/%s", w.namespace, w.name)
	}
	if err != nil {
		return w.site.updateConnectorConfiguredStatus(connector, err)
	}
	if len(pods) == 0 {
		bindings_logger.Debug("No pods available for target selection", w.Attr())
		return w.site.updateConnectorConfiguredStatus(connector, fmt.Errorf("No matches for selector"))
	}
	return w.site.updateConnectorConfiguredStatus(connector, nil)
}

type PodWatchingContext interface {
	Selector() string
	IncludeNotReady() bool
	Attr() slog.Attr
	Updated(pods []skupperv2alpha1.PodDetails) error
}

type PodWatcher struct {
	watcher *kube.PodWatcher
	stopCh  chan struct{}
	context PodWatchingContext
}

func (w *PodWatcher) pods() []skupperv2alpha1.PodDetails {
	var targets []skupperv2alpha1.PodDetails
	for _, pod := range w.watcher.List() {
		if kube.IsPodReady(pod) || w.context.IncludeNotReady() {
			if kube.IsPodRunning(pod) && pod.DeletionTimestamp == nil {
				bindings_logger.Debug("Pod selected for connector",
					slog.String("pod", pod.ObjectMeta.Name),
					w.context.Attr())
				targets = append(targets, skupperv2alpha1.PodDetails{
					UID:  string(pod.UID),
					Name: pod.Name,
					IP:   pod.Status.PodIP,
				})
			} else {
				bindings_logger.Debug("Pod not running for connector",
					slog.String("pod", pod.ObjectMeta.Name),
					w.context.Attr())
			}
		} else {
			bindings_logger.Debug("Pod not ready for connector",
				slog.String("pod", pod.ObjectMeta.Name),
				w.context.Attr())
		}
	}
	return targets

}

func (w *PodWatcher) handle(key string, pod *corev1.Pod) error {
	return w.context.Updated(w.pods())
}

func (w *PodWatcher) Close() {
	close(w.stopCh)
}

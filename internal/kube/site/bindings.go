package site

import (
	"fmt"
	"log/slog"

	corev1 "k8s.io/api/core/v1"

	"github.com/skupperproject/skupper/internal/kube/watchers"
	skupperv2alpha1 "github.com/skupperproject/skupper/pkg/apis/skupper/v2alpha1"
)

var bindings_logger *slog.Logger

func init() {
	bindings_logger = slog.New(slog.Default().Handler()).With(
		slog.String("component", "kube.site.bindings"),
	)
}

type BindingContext interface {
	Select(connector *skupperv2alpha1.Connector) TargetSelection
	Expose(ports *ExposedPortSet) error
	Unexpose(host string) error
}

type TargetSelection interface {
	Selector() string
	Close()
	List() []skupperv2alpha1.PodDetails
}

type TargetSelectionImpl struct {
	watcher             *PodWatcher
	site                *Site
	selector            string
	name                string
	namespace           string
	includeNotReadyPods bool
}

func (w *TargetSelectionImpl) Selector() string {
	return w.selector
}

func (w *TargetSelectionImpl) Close() {
	w.watcher.Close()
}

func (w *TargetSelectionImpl) IncludeNotReadyPods() bool {
	return w.includeNotReadyPods
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
	err := w.site.updateRouterConfig(w.site.bindings)
	connector := w.site.bindings.GetConnector(w.name)
	if connector == nil {
		bindings_logger.Error("Error looking up connector for pod event", w.Attr())
		return nil
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
	IncludeNotReadyPods() bool
	Attr() slog.Attr
	Updated(pods []skupperv2alpha1.PodDetails) error
}

type PodWatcher struct {
	watcher *watchers.PodWatcher
	stopCh  chan struct{}
	context PodWatchingContext
}

func (w *PodWatcher) pods() []skupperv2alpha1.PodDetails {
	var targets []skupperv2alpha1.PodDetails
	for _, pod := range w.watcher.List() {
		if isPodReady(pod) || w.context.IncludeNotReadyPods() {
			if isPodRunning(pod) && pod.DeletionTimestamp == nil {
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
	bindings_logger.Debug("Stopping pod watcher", w.context.Attr(), slog.String("selector", w.context.Selector()))
	close(w.stopCh)
}

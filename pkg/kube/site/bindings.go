package site

import (
	"fmt"
	"log"

	corev1 "k8s.io/api/core/v1"

	skupperv1alpha1 "github.com/skupperproject/skupper/pkg/apis/skupper/v1alpha1"
	"github.com/skupperproject/skupper/pkg/kube"
	"github.com/skupperproject/skupper/pkg/qdr"
	"github.com/skupperproject/skupper/pkg/site"
)

type BindingContext interface {
	Select(connector *skupperv1alpha1.Connector) TargetSelection
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

func (a *BindingAdaptor) ConnectorUpdated(connector *skupperv1alpha1.Connector) bool {
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

func (a *BindingAdaptor) ConnectorDeleted(connector *skupperv1alpha1.Connector) {
	if current, ok := a.selectors[connector.Name]; ok {
		current.Close()
		delete(a.selectors, connector.Name)
	}
}

func (a *BindingAdaptor) ListenerUpdated(listener *skupperv1alpha1.Listener) {
	allocatedRouterPort, err := a.mapping.GetPortForKey(listener.Name)
	if err != nil {
		log.Printf("Unable to get port for listener %s/%s: %s", listener.Namespace, listener.Name, err)
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

func (a *BindingAdaptor) ListenerDeleted(listener *skupperv1alpha1.Listener) {
	a.context.Unexpose(listener.Spec.Host)
	a.mapping.ReleasePortForKey(listener.Name)
}

func (a *BindingAdaptor) updateBridgeConfigForConnector(siteId string, connector *skupperv1alpha1.Connector, config *qdr.BridgeConfig) {
	if connector.Spec.Host != "" {
		site.UpdateBridgeConfigForConnector(siteId, connector, config)
	} else if connector.Spec.Selector != "" {
		if selector, ok := a.selectors[connector.Name]; ok {
			for _, pod := range selector.List() {
				site.UpdateBridgeConfigForConnectorToPod(siteId, connector, pod, config)
			}
		} else {
			log.Printf("Error: not yet tracking pods for connector %s/%s with selector set", connector.Namespace, connector.Name)
		}
	} else {
		log.Printf("Error: connector %s/%s has neither host nor selector set", connector.Namespace, connector.Name)
	}
}

func (a *BindingAdaptor) updateBridgeConfigForListener(siteId string, listener *skupperv1alpha1.Listener, config *qdr.BridgeConfig) {
	if port, err := a.mapping.GetPortForKey(listener.Name); err == nil {
		site.UpdateBridgeConfigForListenerWithHostAndPort(siteId, listener, "0.0.0.0", port, config)
	} else {
		log.Printf("Could not allocate port for %s/%s: %s", listener.Namespace, listener.Name, err)
	}
}

type TargetSelection interface {
	Selector() string
	Close()
	List() []skupperv1alpha1.PodDetails
}

type TargetSelectionImpl struct {
	watcher         *PodWatcher
	stopCh          chan struct{}
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
	//w.watcher.Close()
	close(w.stopCh)
}

func (w *TargetSelectionImpl) IncludeNotReady() bool {
	return w.includeNotReady
}

func (w *TargetSelectionImpl) Id() string {
	return fmt.Sprintf("Connector %s/%s", w.name, w.namespace)
}

func (w *TargetSelectionImpl) List() []skupperv1alpha1.PodDetails {
	return w.watcher.pods()
}

func (w *TargetSelectionImpl) Updated(pods []skupperv1alpha1.PodDetails) error {
	err := w.site.updateRouterConfigForGroups(w.site.bindings)
	connector := w.site.bindings.GetConnector(w.name)
	if connector == nil {
		return fmt.Errorf("Error looking up connector for %s/%s", w.namespace, w.name)
	}
	if err != nil {
		return w.site.updateConnectorConfiguredStatus(connector, err)
	}
	if len(pods) == 0 {
		log.Printf("No pods available for %s", w.Id())
		return w.site.updateConnectorConfiguredStatus(connector, fmt.Errorf("No matches for selector"))
	}
	log.Printf("Pods are available for %s", w.Id())
	return w.site.updateConnectorConfiguredStatus(connector, nil)
}

type PodWatchingContext interface {
	Selector() string
	IncludeNotReady() bool
	Id() string
	Updated(pods []skupperv1alpha1.PodDetails) error
}

type PodWatcher struct {
	watcher *kube.PodWatcher
	stopCh  chan struct{}
	context PodWatchingContext
}

func (w *PodWatcher) pods() []skupperv1alpha1.PodDetails {
	var targets []skupperv1alpha1.PodDetails
	for _, pod := range w.watcher.List() {
		if kube.IsPodReady(pod) || w.context.IncludeNotReady() {
			if kube.IsPodRunning(pod) && pod.DeletionTimestamp == nil {
				log.Printf("Pod %s selected for connector %s", pod.ObjectMeta.Name, w.context.Id())
				targets = append(targets, skupperv1alpha1.PodDetails{
					UID:  string(pod.UID),
					Name: pod.Name,
					IP:   pod.Status.PodIP,
				})
			} else {
				log.Printf("Pod %s not running for connector %s", pod.ObjectMeta.Name, w.context.Id())
			}
		} else {
			log.Printf("Pod %s not ready for connector %s", pod.ObjectMeta.Name, w.context.Id())
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

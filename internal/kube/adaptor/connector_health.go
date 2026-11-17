package adaptor

import (
	"log/slog"
	"sync"

	"k8s.io/client-go/tools/cache"

	"github.com/skupperproject/skupper/internal/kube/watchers"
	"github.com/skupperproject/skupper/internal/qdr"
	"github.com/skupperproject/skupper/internal/site"
	skupperv2alpha1 "github.com/skupperproject/skupper/pkg/apis/skupper/v2alpha1"
)

type ConnectorHealth struct {
	bindings   *site.Bindings
	watcher    *watchers.ConnectorWatcher
	mu         sync.RWMutex
	connectors map[string]*skupperv2alpha1.Connector
	onChange   func()
	logger     *slog.Logger
}

func newConnectorHealth(controller *watchers.EventProcessor, namespace string, onChange func()) *ConnectorHealth {
	h := &ConnectorHealth{
		bindings:   site.NewBindings(""),
		connectors: map[string]*skupperv2alpha1.Connector{},
		onChange:   onChange,
		logger:     slog.New(slog.Default().Handler()).With(slog.String("component", "kube.adaptor.connectorHealth")),
	}
	h.watcher = controller.WatchConnectors(namespace, h.connectorEvent)
	return h
}

func (h *ConnectorHealth) start() {
	h.mu.Lock()
	for _, connector := range h.watcher.List() {
		h.update(connector.Name, connector)
	}
	h.mu.Unlock()
	h.bindings.SetHealthChangeCallback(h.healthChanged)
	h.bindings.StartHealthCheckLoop(site.HealthCheckLoopInterval)
}

func (h *ConnectorHealth) stop() {
	h.bindings.Stop()
}

func (h *ConnectorHealth) connectorEvent(key string, connector *skupperv2alpha1.Connector) error {
	h.mu.Lock()
	var changed bool
	var name string
	if connector != nil {
		name = connector.Name
		changed = h.update(name, connector)
	} else {
		var err error
		_, name, err = cache.SplitMetaNamespaceKey(key)
		if err != nil {
			h.mu.Unlock()
			return err
		}
		changed = h.update(name, nil)
	}
	h.mu.Unlock()
	if changed && h.onChange != nil {
		h.onChange()
	}
	return nil
}

func (h *ConnectorHealth) update(name string, connector *skupperv2alpha1.Connector) bool {
	if connector == nil {
		delete(h.connectors, name)
	} else {
		h.connectors[name] = connector
	}
	return h.bindings.UpdateConnector(name, connector) != nil
}

func (h *ConnectorHealth) healthChanged(changed []site.ConnectorHealthChange) {
	h.logger.Info("Connector health status changed", slog.Any("connectors", changed))
	if h.onChange != nil {
		h.onChange()
	}
}

func (h *ConnectorHealth) filter(bridges *qdr.BridgeConfig) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for name, connector := range h.connectors {
		if connector.Spec.Host == "" || !connector.Spec.IsHealthCheckEnabled() || h.bindings.IsConnectorHealthy(name) {
			continue
		}
		if removed, _ := bridges.RemoveTcpConnector(qdr.TcpConnectorNamePrefix + name + "@" + connector.Spec.Host); removed {
			h.logger.Info("Not adding connector bridge config because host is unhealthy",
				slog.String("name", name),
				slog.String("host", connector.Spec.Host))
		}
	}
}

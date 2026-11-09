package adaptor

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"sync/atomic"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"k8s.io/client-go/util/retry"

	internalclient "github.com/skupperproject/skupper/internal/kube/client"
	"github.com/skupperproject/skupper/internal/kube/watchers"
	"github.com/skupperproject/skupper/internal/qdr"
	"github.com/skupperproject/skupper/internal/site"
	skupperv2alpha1 "github.com/skupperproject/skupper/pkg/apis/skupper/v2alpha1"
)

const connectorHealthLeaderLockName = "skupper-connector-health-leader"

type ConnectorHealth struct {
	clients     internalclient.Clients
	namespace   string
	bindings    *site.Bindings
	watcher     *watchers.ConnectorWatcher
	mu          sync.RWMutex
	connectors  map[string]*skupperv2alpha1.Connector
	unreachable map[string]struct{}
	leader      atomic.Bool
	onChange    func()
	logger      *slog.Logger
}

func newConnectorHealth(clients internalclient.Clients, controller *watchers.EventProcessor, namespace string, onChange func()) *ConnectorHealth {
	h := &ConnectorHealth{
		clients:     clients,
		namespace:   namespace,
		bindings:    site.NewBindings(""),
		connectors:  map[string]*skupperv2alpha1.Connector{},
		unreachable: map[string]struct{}{},
		onChange:    onChange,
		logger:      slog.New(slog.Default().Handler()).With(slog.String("component", "kube.adaptor.connectorHealth")),
	}
	h.watcher = controller.WatchConnectors(namespace, h.connectorEvent)
	return h
}

func (h *ConnectorHealth) start(stopCh <-chan struct{}) {
	h.mu.Lock()
	for _, connector := range h.watcher.List() {
		h.update(connector.Name, connector)
	}
	h.mu.Unlock()
	h.bindings.SetHealthChangeCallback(h.healthChanged)
	h.bindings.StartHealthCheckLoop(site.HealthCheckLoopInterval)
	go h.runLeaderElection(wait.ContextForChannel(stopCh))
}

func (h *ConnectorHealth) stop() {
	h.bindings.Stop()
}

func (h *ConnectorHealth) runLeaderElection(ctx context.Context) {
	identity, _ := os.Hostname()
	lock := &resourcelock.LeaseLock{
		LeaseMeta: metav1.ObjectMeta{
			Name:      connectorHealthLeaderLockName,
			Namespace: h.namespace,
		},
		Client: h.clients.GetKubeClient().CoordinationV1(),
		LockConfig: resourcelock.ResourceLockConfig{
			Identity: identity,
		},
	}
	for ctx.Err() == nil {
		leaderelection.RunOrDie(ctx, leaderelection.LeaderElectionConfig{
			Lock:            lock,
			ReleaseOnCancel: true,
			LeaseDuration:   15 * time.Second,
			RenewDeadline:   10 * time.Second,
			RetryPeriod:     2 * time.Second,
			Callbacks: leaderelection.LeaderCallbacks{
				OnStartedLeading: func(context.Context) {
					h.logger.Info("Became leader for connector status updates")
					h.becomeLeader()
				},
				OnStoppedLeading: func() {
					h.logger.Info("Lost leader lock for connector status updates")
					h.leader.Store(false)
				},
			},
		})
	}
}

func (h *ConnectorHealth) becomeLeader() {
	h.leader.Store(true)
	h.mu.RLock()
	names := make([]string, 0, len(h.unreachable))
	for name := range h.unreachable {
		names = append(names, name)
	}
	h.mu.RUnlock()
	for _, name := range names {
		h.setReachable(name, false)
	}
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
	_, unreachable := h.unreachable[name]
	h.mu.Unlock()
	if unreachable && h.leader.Load() {
		h.setReachable(name, false)
	}
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
	if connector == nil || connector.Spec.Host == "" || !connector.Spec.IsHealthCheckEnabled() {
		delete(h.unreachable, name)
	}
	return h.bindings.UpdateConnector(name, connector) != nil
}

func (h *ConnectorHealth) healthChanged(changed []site.ConnectorHealthChange) {
	h.logger.Info("Connector health status changed", slog.Any("connectors", changed))
	h.mu.Lock()
	for _, change := range changed {
		if change.Healthy {
			delete(h.unreachable, change.Name)
		} else {
			h.unreachable[change.Name] = struct{}{}
		}
	}
	h.mu.Unlock()
	if h.onChange != nil {
		h.onChange()
	}
	if h.leader.Load() {
		for _, change := range changed {
			h.setReachable(change.Name, change.Healthy)
		}
	}
}

func (h *ConnectorHealth) setReachable(name string, reachable bool) {
	connectors := h.clients.GetSkupperClient().SkupperV2alpha1().Connectors(h.namespace)
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		connector, err := connectors.Get(context.TODO(), name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		var unreachableErr error
		if !reachable {
			unreachableErr = fmt.Errorf("Target host %s:%d is not reachable", connector.Spec.Host, connector.Spec.Port)
		}
		if !connector.SetConfigured(unreachableErr) {
			return nil
		}
		_, err = connectors.UpdateStatus(context.TODO(), connector, metav1.UpdateOptions{})
		return err
	})
	if err != nil && !apierrors.IsNotFound(err) {
		h.logger.Error("Failed to update connector status",
			slog.String("connector", name),
			slog.Any("error", err))
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

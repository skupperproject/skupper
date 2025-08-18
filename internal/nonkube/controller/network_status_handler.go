package controller

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync"

	"github.com/skupperproject/skupper/internal/network"
	"github.com/skupperproject/skupper/internal/nonkube/common"
	"github.com/skupperproject/skupper/pkg/apis/skupper/v2alpha1"
	"github.com/skupperproject/skupper/pkg/nonkube/api"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/yaml"
)

type NetworkStatusHandler struct {
	Namespace string
	basePath  string
	logger    *slog.Logger
	doneCh    chan struct{}
	events    chan network.NetworkStatusInfo
	mutex     sync.RWMutex
}

func NewNetworkStatusHandler(namespace string) *NetworkStatusHandler {
	logger := slog.Default().
		With("component", "network.status.handler").
		With("namespace", namespace)

	return &NetworkStatusHandler{
		Namespace: namespace,
		logger:    logger,
		events:    make(chan network.NetworkStatusInfo),
	}
}

func (n *NetworkStatusHandler) OnUpdate(name string) {
	n.startProcessingEvents()
	n.processConfigMapUpdate(name)
}

func (n *NetworkStatusHandler) processConfigMapUpdate(name string) {
	networkStatusInfo, err := n.loadNetworkStatusInfo(name)
	if err != nil {
		n.logger.Warn("ignoring network status update", slog.Any("error", err))
		return
	}
	n.logger.Debug("Dispatching network status info event")
	n.events <- *networkStatusInfo
}

func (n *NetworkStatusHandler) processEvents(done chan struct{}) {
	n.resetStatus(true)
	for {
		select {
		case networkStatusInfo := <-n.events:
			n.logger.Debug("Processing network status event", slog.Any("event", networkStatusInfo))
			n.updateRuntimeSiteState(networkStatusInfo, true)
		case <-done:
			n.resetStatus(false)
			n.logger.Info("Stop event processing")
			return
		}
	}
}

func (n *NetworkStatusHandler) updateRuntimeSiteState(networkStatusInfo network.NetworkStatusInfo, ready bool) {
	runtimeSiteStatePath := api.GetInternalOutputPath(n.Namespace, api.RuntimeSiteStatePath)
	siteStateLoader := &common.FileSystemSiteStateLoader{
		Path: runtimeSiteStatePath,
	}
	siteState, err := siteStateLoader.Load()
	if err != nil {
		n.logger.Warn("Error loading runtime site state", slog.Any("error", err))
		return
	}
	delete(siteState.ConfigMaps, "skupper-network-status")
	siteState.UpdateStatus(networkStatusInfo)
	if ready {
		siteState.Site.SetRunning(v2alpha1.ReadyCondition())
	} else {
		siteState.Site.SetRunning(v2alpha1.PendingCondition("No router pod is ready"))
	}
	if err = api.MarshalSiteState(*siteState, runtimeSiteStatePath); err != nil {
		n.logger.Error("Error marshaling runtime site state", slog.Any("error", err))
		return
	}
	n.logger.Debug("Runtime site state updated")
}

func (n *NetworkStatusHandler) resetStatus(ready bool) {
	n.updateRuntimeSiteState(network.NetworkStatusInfo{}, ready)
}

func (n *NetworkStatusHandler) loadNetworkStatusInfo(name string) (*network.NetworkStatusInfo, error) {
	cm, err := n.loadCm(name)
	if err != nil {
		return nil, err
	}
	if cm.Data == nil {
		return nil, fmt.Errorf("skupper-network-status ConfigMap has no data")
	}
	networkStatus, ok := cm.Data["NetworkStatus"]
	if !ok {
		return nil, fmt.Errorf("skupper-network-status ConfigMap has no 'NetworkStatus' key")
	}
	nsi := &network.NetworkStatusInfo{}
	err = json.Unmarshal([]byte(networkStatus), nsi)
	if err != nil {
		n.logger.Error("Failed to unmarshal network status info", slog.Any("networkStatus", networkStatus))
		return nil, fmt.Errorf("error unmarshalling NetworkStatusInfo: %v", err)
	}
	return nsi, nil
}

func (n *NetworkStatusHandler) loadCm(name string) (*corev1.ConfigMap, error) {
	data, err := os.ReadFile(name)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %w", name, err)
	}
	cm := &corev1.ConfigMap{}
	err = yaml.Unmarshal(data, cm)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal configmap %s: %w", name, err)
	}
	return cm, nil

}

func (n *NetworkStatusHandler) OnBasePathAdded(basePath string) {
}

func (n *NetworkStatusHandler) startProcessingEvents() {
	n.mutex.Lock()
	defer n.mutex.Unlock()
	if n.doneCh != nil {
		return
	}
	n.logger.Info("Start event processing")
	n.doneCh = make(chan struct{})
	go n.processEvents(n.doneCh)
}

func (n *NetworkStatusHandler) OnRemove(name string) {
	n.mutex.Lock()
	defer n.mutex.Unlock()

	if n.doneCh != nil {
		close(n.doneCh)
		n.doneCh = nil
	}
}

func (n *NetworkStatusHandler) Filter(name string) bool {
	return strings.HasSuffix(name, "/ConfigMap-skupper-network-status.yaml")
}

func (n *NetworkStatusHandler) OnCreate(name string) {
	n.OnUpdate(name)
}

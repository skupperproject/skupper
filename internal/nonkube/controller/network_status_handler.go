package controller

import (
	"log/slog"
	"strings"
)

type NetworkStatusHandler struct {
	Namespace string
	basePath  string
	logger    *slog.Logger
}

func NewNetworkStatusHandler(namespace string) *NetworkStatusHandler {
	logger := slog.Default().
		With("namespace", namespace).
		With("component", "network.status.handler")

	return &NetworkStatusHandler{
		Namespace: namespace,
		logger:    logger,
	}
}

func (n *NetworkStatusHandler) OnAdd(basePath string) {
	n.logger.Info("network status has been added", slog.Any("basePath", basePath))
}

func (n *NetworkStatusHandler) OnCreate(name string) {
	n.logger.Info("network status has been created")
}

func (n *NetworkStatusHandler) OnUpdate(name string) {
	n.logger.Info("network status has been updated")
}

func (n *NetworkStatusHandler) OnRemove(name string) {
	n.logger.Info("network status has been removed")
}

func (n *NetworkStatusHandler) Filter(name string) bool {
	return strings.HasSuffix(name, "/ConfigMap-skupper-network-status.yaml")
}

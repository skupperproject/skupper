package controller

import (
	"log/slog"
	"os"
	"path"
	"strings"
)

type RouterConfigHandler struct {
	logger    *slog.Logger
	stopCh    chan struct{}
	callbacks []ActivationCallback
	namespace string
}

func NewRouterConfigHandler(stopCh chan struct{}, namespace string) *RouterConfigHandler {
	handler := &RouterConfigHandler{
		stopCh:    stopCh,
		namespace: namespace,
	}
	handler.logger = slog.Default().With("component", "router.config.handler", "namespace", namespace)
	return handler
}

func (r *RouterConfigHandler) AddCallback(callback ActivationCallback) {
	r.callbacks = append(r.callbacks, callback)
}

func (r *RouterConfigHandler) OnCreate(name string) {
	r.logger.Info("Router config has been created, starting callbacks")
	for _, cb := range r.callbacks {
		cb.Start(r.stopCh)
	}
}

func (r *RouterConfigHandler) OnRemove(name string) {
	r.logger.Info("Router config has been removed, stopping callbacks")
	for _, cb := range r.callbacks {
		cb.Stop()
	}
}

func (r *RouterConfigHandler) OnBasePathAdded(basePath string) {
	routerConfigFile := path.Join(basePath, "skrouterd.json")
	if _, err := os.Stat(routerConfigFile); err == nil {
		r.OnCreate(routerConfigFile)
	}
}

func (r *RouterConfigHandler) OnUpdate(name string) {
}

func (r *RouterConfigHandler) Filter(name string) bool {
	return strings.HasSuffix(name, "/skrouterd.json")
}

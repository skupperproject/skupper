package controller

import (
	"fmt"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/internal/nonkube/client/runtime"
	"github.com/skupperproject/skupper/internal/nonkube/common"
	"github.com/skupperproject/skupper/internal/qdr"
	"github.com/skupperproject/skupper/internal/site"
	"github.com/skupperproject/skupper/internal/utils"
	"github.com/skupperproject/skupper/pkg/nonkube/api"
)

type SystemAdaptorHandler struct {
	running       bool
	logger        *slog.Logger
	namespace     string
	lock          sync.Mutex
	systemAdaptor *SystemAdaptor
	callback      ActivationCallback
	bindings      *site.Bindings
}

func NewSystemAdaptorHandler(namespace string) *SystemAdaptorHandler {

	systemReloadType := utils.DefaultStr(os.Getenv(types.ENV_SYSTEM_AUTO_RELOAD),
		types.SystemReloadTypeManual)

	if systemReloadType == types.SystemReloadTypeManual {
		slog.Default().Debug("Automatic reloading is not configured.")
		return nil
	}

	handler := &SystemAdaptorHandler{
		namespace: namespace,
	}
	handler.logger = slog.Default().With("component", "system.adaptor.handler", "namespace", namespace)
	return handler
}

func (s *SystemAdaptorHandler) SetCallback(callback ActivationCallback) {
	s.callback = callback
}

func (s *SystemAdaptorHandler) Start(stopCh <-chan struct{}) {
	s.lock.Lock()
	defer s.lock.Unlock()
	if s.running {
		return
	}
	s.logger.Info("Starting")
	s.running = true

	tls := runtime.GetRuntimeTlsCert(s.namespace, "skupper-local-client")
	address, err := runtime.GetLocalRouterAddress(s.namespace)
	if err != nil {
		s.logger.Error(fmt.Sprintf("Error getting local router address: %s", err))
		return
	}

	agentPool := qdr.NewAgentPool(address, tls)
	s.systemAdaptor = NewSystemAdaptor(s.namespace, agentPool)

	if bindings, err := s.buildBindings(); err != nil {
		s.logger.Warn("Cannot build bindings at start, will retry", slog.Any("error", err))
	} else {
		s.bindings = bindings
		bindings.StartHealthCheckLoop(5 * time.Second)
	}

	go s.processRouterConfig(stopCh)
}

func (s *SystemAdaptorHandler) Stop() {
	s.lock.Lock()
	defer s.lock.Unlock()
	if s.running {
		s.logger.Info("Stopping")
		s.running = false
		if s.bindings != nil {
			s.bindings.Stop()
			s.bindings = nil
		}
	}
}

func (s *SystemAdaptorHandler) Id() string {
	return "system.adaptor.handler"
}

func (s *SystemAdaptorHandler) buildBindings() (*site.Bindings, error) {
	siteState, err := common.LoadCurrentSiteState(s.namespace)
	if err != nil || siteState == nil {
		return nil, fmt.Errorf("cannot load current site state: %w", err)
	}
	profilePath := api.GetInternalOutputPath(s.namespace, api.CertificatesPath)
	bindings := site.NewBindings(profilePath)
	bindings.SetSiteId(siteState.SiteId)
	for name, connector := range siteState.Connectors {
		_ = bindings.UpdateConnector(name, connector)
	}
	for name, listener := range siteState.Listeners {
		_ = bindings.UpdateListener(name, listener)
	}
	for name, mkl := range siteState.MultiKeyListeners {
		bindings.UpdateMultiKeyListener(name, mkl)
	}
	return bindings, nil
}

func (s *SystemAdaptorHandler) processRouterConfig(stopCh <-chan struct{}) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-stopCh:
			return
		case <-ticker.C:
			s.lock.Lock()
			if !s.running {
				s.lock.Unlock()
				return
			}

			if s.bindings == nil {
				if bindings, err := s.buildBindings(); err != nil {
					s.logger.Debug("Bindings not yet available, will retry", slog.Any("error", err))
				} else {
					s.bindings = bindings
					bindings.StartHealthCheckLoop(5 * time.Second)
				}
			} else {
				s.reconcileBindings()
			}

			desired, err := common.LoadRouterConfig(s.namespace)
			if err != nil {
				s.logger.Error(err.Error())
				s.lock.Unlock()
				continue
			}

			bindings := s.bindings
			s.lock.Unlock()

			if bindings != nil {
				bindings.Apply(desired)
			}

			if err := s.systemAdaptor.syncWithRouter(desired); err != nil {
				s.logger.Debug(err.Error())
			}
		}
	}
}

func (s *SystemAdaptorHandler) reconcileBindings() {
	siteState, err := common.LoadCurrentSiteState(s.namespace)
	if err != nil || siteState == nil {
		s.logger.Debug("Cannot load current site state for reconciliation", slog.Any("error", err))
		return
	}

	seen := make(map[string]struct{}, len(siteState.Connectors))
	for name, connector := range siteState.Connectors {
		seen[name] = struct{}{}
		_ = s.bindings.UpdateConnector(name, connector)
	}

	for _, name := range s.bindings.ConnectorNames() {
		if _, ok := seen[name]; !ok {
			_ = s.bindings.UpdateConnector(name, nil)
		}
	}

	seenListeners := make(map[string]struct{}, len(siteState.Listeners))
	for name, listener := range siteState.Listeners {
		seenListeners[name] = struct{}{}
		_ = s.bindings.UpdateListener(name, listener)
	}

	for _, name := range s.bindings.ListenerNames() {
		if _, ok := seenListeners[name]; !ok {
			_ = s.bindings.UpdateListener(name, nil)
		}
	}

	seenMkl := make(map[string]struct{}, len(siteState.MultiKeyListeners))
	for name, mkl := range siteState.MultiKeyListeners {
		seenMkl[name] = struct{}{}
		s.bindings.UpdateMultiKeyListener(name, mkl)
	}

	for _, name := range s.bindings.MultiKeyListenerNames() {
		if _, ok := seenMkl[name]; !ok {
			s.bindings.UpdateMultiKeyListener(name, nil)
		}
	}
}

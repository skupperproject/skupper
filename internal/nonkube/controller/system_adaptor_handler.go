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
	"github.com/skupperproject/skupper/internal/utils"
)

type SystemAdaptorHandler struct {
	running       bool
	logger        *slog.Logger
	namespace     string
	lock          sync.Mutex
	systemAdaptor *SystemAdaptor
	callback      ActivationCallback
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
	go s.processRouterConfig(stopCh)

}

func (s *SystemAdaptorHandler) Stop() {
	s.lock.Lock()
	defer s.lock.Unlock()
	if s.running {
		s.logger.Info("Stopping")
		s.running = false
	}
}

func (s *SystemAdaptorHandler) Id() string {
	return "system.adaptor.handler"
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

			desired, err := common.LoadRouterConfig(s.namespace)
			if err != nil {
				s.logger.Error(err.Error())
				s.lock.Unlock()
				continue
			}

			s.lock.Unlock()

			err = s.systemAdaptor.syncWithRouter(desired)
			if err != nil {
				s.logger.Debug(err.Error())
			}

		}

	}
}

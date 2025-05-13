package controller

import (
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/skupperproject/skupper/internal/messaging"
	"github.com/skupperproject/skupper/internal/nonkube/client/runtime"
	"github.com/skupperproject/skupper/internal/nonkube/common"
	"github.com/skupperproject/skupper/internal/qdr"
	"github.com/skupperproject/skupper/pkg/nonkube/api"
)

type RouterStateHandler struct {
	running   bool
	namespace string
	siteId    string
	logger    *slog.Logger
	mux       sync.Mutex
	callback  ActivationCallback
	heartbeat *heartBeatsClient
}

func NewRouterStateHandler(namespace string) *RouterStateHandler {
	handler := &RouterStateHandler{
		namespace: namespace,
		heartbeat: newHeartBeatsClient(namespace),
	}
	handler.logger = slog.Default().
		With("component", handler.Id()).
		With("namespace", namespace)
	return handler
}

func (h *RouterStateHandler) SetCallback(callback ActivationCallback) {
	h.callback = callback
}

func (h *RouterStateHandler) Start(stopCh <-chan struct{}) {
	h.mux.Lock()
	defer h.mux.Unlock()
	if h.running {
		return
	}
	h.logger.Info("Starting")
	h.running = true
	go h.heartbeat.Start(stopCh, h.callback)
	go h.handleParentStop(stopCh)
}

func (h *RouterStateHandler) Stop() {
	h.mux.Lock()
	defer h.mux.Unlock()
	if h.running {
		h.logger.Info("Stopping")
		h.heartbeat.Stop()
		h.running = false
	}
}

func (h *RouterStateHandler) Id() string {
	return "router.state.handler"
}

func (h *RouterStateHandler) handleParentStop(stopCh <-chan struct{}) {
	t := time.NewTicker(time.Second)
	for {
		h.mux.Lock()
		if !h.running {
			h.mux.Unlock()
			break
		}
		h.mux.Unlock()
		select {
		case <-stopCh:
			h.logger.Debug("Parent channel closed")
			h.Stop()
		case <-t.C:
		}
	}
	h.logger.Debug("Stopped")
}

func newHeartBeatsClient(namespace string) *heartBeatsClient {
	c := &heartBeatsClient{
		Namespace: namespace,
	}
	c.logger = slog.Default().
		With("component", "heartbeat.client").
		With("namespace", namespace)
	return c
}

type heartBeatsClient struct {
	Namespace  string
	logger     *slog.Logger
	siteId     string
	url        string
	address    string
	mutex      sync.Mutex
	running    bool
	isRouterUp bool
	callback   ActivationCallback
	receiver   messaging.Receiver
	factory    func(string, qdr.TlsConfigRetriever) messaging.ConnectionFactory
}

func (h *heartBeatsClient) Start(stopCh <-chan struct{}, callback ActivationCallback) {
	h.mutex.Lock()
	defer h.mutex.Unlock()

	if h.running {
		return
	}

	h.logger.Info("Starting")
	h.running = true
	h.callback = callback
	go h.run(stopCh)
}

func (h *heartBeatsClient) Stop() {
	h.mutex.Lock()
	defer h.mutex.Unlock()
	h.running = false
	if h.receiver != nil {
		_ = h.receiver.Close()
	}
}

func (h *heartBeatsClient) routerDown(reason string) {
	h.mutex.Lock()
	defer h.mutex.Unlock()
	if h.isRouterUp {
		h.logger.Info("Router is DOWN", slog.Any("reason", reason))
		h.isRouterUp = false
		h.callback.Stop()
	}
}

func (h *heartBeatsClient) routerUp(stopCh <-chan struct{}) {
	h.mutex.Lock()
	defer h.mutex.Unlock()
	if !h.isRouterUp {
		h.logger.Info("Router is UP")
		h.isRouterUp = true
		h.callback.Start(stopCh)
	}
}

func (h *heartBeatsClient) run(stopCh <-chan struct{}) {
	h.logger.Debug("Watching for router availability")
	ticker := time.NewTicker(time.Second)
	for {
		h.mutex.Lock()
		if !h.running {
			h.mutex.Unlock()
			h.routerDown("Stopped")
			break
		}
		h.mutex.Unlock()
		<-ticker.C

		// connection info
		url, err := h.getUrl()
		if err != nil {
			h.routerDown(fmt.Sprintf("Unable to retrieve heartbeat url: %s", err))
			continue
		}
		address, err := h.getAddress()
		if err != nil {
			h.routerDown(fmt.Sprintf("Unable to retrieve heartbeat address: %s", err))
			continue
		}
		tls := runtime.GetRuntimeTlsCert(h.Namespace, "skupper-local-client")

		// connect
		var connFactory messaging.ConnectionFactory
		if h.factory == nil {
			connFactory = qdr.NewConnectionFactory(url, tls)
		} else {
			connFactory = h.factory(url, tls)
		}
		conn, err := connFactory.Connect()
		if err != nil {
			h.routerDown(fmt.Sprintf("unable to connect with router through: %s", url))
			continue
		}
		h.mutex.Lock()
		h.receiver, err = conn.Receiver(address, 1)
		receiver := h.receiver
		h.mutex.Unlock()
		if err != nil {
			h.logger.Error(err.Error())
			h.routerDown("unable to create receiver")
			continue
		}
		h.routerUp(stopCh)
		for {
			h.mutex.Lock()
			if !h.running {
				h.mutex.Unlock()
				_ = receiver.Close()
				conn.Close()
				break
			}
			h.mutex.Unlock()
			_, err = receiver.Receive()
			if err == nil {
				h.routerUp(stopCh)
				continue
			}
			h.routerDown(fmt.Sprintf("receive error: %s", err))
			_ = receiver.Close()
			conn.Close()
			break
		}
	}
	h.logger.Debug("Exiting")
}

func (h *heartBeatsClient) getSiteId() (string, error) {
	if h.siteId != "" {
		return h.siteId, nil
	}
	// Loading runtime state
	siteStateLoader := &common.FileSystemSiteStateLoader{
		Path: api.GetInternalOutputPath(h.Namespace, api.RuntimeSiteStatePath),
	}
	siteState, err := siteStateLoader.Load()
	if err != nil {
		return "", fmt.Errorf("unable to load site state: %w", err)
	}
	h.siteId = siteState.SiteId
	return h.siteId, nil
}

func (h *heartBeatsClient) getUrl() (string, error) {
	h.mutex.Lock()
	defer h.mutex.Unlock()
	if h.url != "" {
		return h.url, nil
	}
	port, err := runtime.GetLocalRouterPort(h.Namespace)
	if err != nil {
		return "", fmt.Errorf("unable to determine local router url: %w", err)
	}
	h.url = fmt.Sprintf("amqps://127.0.0.1:%d", port)
	return h.url, nil

}

func (h *heartBeatsClient) getAddress() (string, error) {
	siteId, err := h.getSiteId()
	if err != nil {
		return "", fmt.Errorf("unable to determine siteId: %w", err)
	}
	address := fmt.Sprintf("/mc/sfe.%s.heartbeats", siteId)
	return address, nil
}

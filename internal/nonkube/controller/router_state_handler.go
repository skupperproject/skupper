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
	stopCh    <-chan struct{}
	namespace string
	siteId    string
	logger    *slog.Logger
	runningCh chan bool
	callback  ActivationCallback
}

func NewRouterStateHandler(namespace string) *RouterStateHandler {
	handler := &RouterStateHandler{
		namespace: namespace,
	}
	handler.logger = slog.Default().
		With("namespace", namespace).
		With("component", handler.Id())
	return handler
}

func (h *RouterStateHandler) SetCallback(callback ActivationCallback) {
	h.callback = callback
}

func (h *RouterStateHandler) Start(stopCh <-chan struct{}) {
	if h.runningCh != nil {
		return
	}
	h.logger.Info("Starting router state handler")
	h.runningCh = make(chan bool)
	h.stopCh = stopCh
	go h.run()
}

func (h *RouterStateHandler) Stop() {
	h.logger.Info("Stopping router state handler")
	if h.runningCh != nil {
		close(h.runningCh)
		h.runningCh = nil
	}
}

func (h *RouterStateHandler) Id() string {
	return "router.state.handler"
}

func (h *RouterStateHandler) run() {
	hbClient := newHeartBeatsClient(h.namespace)
	hbClient.Start(h.stopCh, h.callback)
	select {
	case <-h.stopCh:
		h.logger.Debug("exiting router state handler (parent stopped)")
		return
	case <-h.runningCh:
		h.logger.Debug("exiting router state handler (user request)")
		return
	}
}

func newHeartBeatsClient(namespace string) *heartBeatsClient {
	c := &heartBeatsClient{
		Namespace: namespace,
	}
	c.logger = slog.Default().
		With("namespace", namespace).
		With("component", "heartbeat.client")
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
}

func (h *heartBeatsClient) Start(stopCh <-chan struct{}, callback ActivationCallback) {
	h.mutex.Lock()
	defer h.mutex.Unlock()

	if h.running {
		return
	}

	h.running = true
	h.callback = callback
	go h.run(stopCh)
	go h.handleShutdown(stopCh)
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
	h.logger.Info("watching for router availability")
	ticker := time.NewTicker(5 * time.Second)
	for h.running {
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
		connFactory := qdr.NewConnectionFactory(url, tls)
		conn, err := connFactory.Connect()
		if err != nil {
			h.routerDown(fmt.Sprintf("unable to connect with router through: %s", url))
			continue
		}
		h.receiver, err = conn.Receiver(address, 1)
		if err != nil {
			h.logger.Error(err.Error())
			h.callback.Stop()
			continue
		}
		h.routerUp(stopCh)
		for {
			_, err = h.receiver.Receive()
			if err == nil {
				h.routerUp(stopCh)
				continue
			}
			h.routerDown(fmt.Sprintf("receive error: %s", err))
			_ = h.receiver.Close()
			conn.Close()
			break
		}
	}
}

func (h *heartBeatsClient) handleShutdown(stopCh <-chan struct{}) {
	<-stopCh
	if h.receiver != nil {
		h.logger.Info("stopped watching for router availability")
		_ = h.receiver.Close()
		h.reset()
	}
}

func (h *heartBeatsClient) reset() {
	h.running = false
	h.siteId = ""
	h.url = ""
	h.address = ""
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

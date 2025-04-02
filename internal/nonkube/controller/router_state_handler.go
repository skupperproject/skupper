package controller

import (
	"fmt"
	"log/slog"
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
	running   chan bool
	callback  ActivationCallback
}

func NewRouterStateHandler(namespace string) *RouterStateHandler {
	handler := &RouterStateHandler{
		namespace: namespace,
	}
	handler.logger = slog.Default().With("component", handler.Id(), "namespace", namespace)
	return handler
}

func (h *RouterStateHandler) SetCallback(callback ActivationCallback) {
	h.callback = callback
}

func (h *RouterStateHandler) Start(stopCh <-chan struct{}) {
	if h.running != nil {
		return
	}
	h.logger.Info("Starting router state handler")
	h.running = make(chan bool)
	h.stopCh = stopCh
	go h.run()
}

func (h *RouterStateHandler) Stop() {
	h.logger.Info("Stopping router state handler")
	close(h.running)
}

func (h *RouterStateHandler) Id() string {
	return "router.state.handler"
}

func (h *RouterStateHandler) run() {
	hbClient := newHeartBeatsClient(h.namespace)
	hbClient.Start(h.stopCh, h.callback)
	select {
	case <-h.stopCh:
		fmt.Println("exiting router state handler (parent stopped)")
		return
	case <-h.running:
		fmt.Println("exiting router state handler (user request)")
		return
	}
}

func newHeartBeatsClient(namespace string) *heartBeatsClient {
	c := &heartBeatsClient{
		Namespace: namespace,
	}
	c.logger = slog.Default().With("component", "heartbeat.client", "namespace", namespace)
	return c
}

type heartBeatsClient struct {
	Namespace  string
	logger     *slog.Logger
	running    bool
	isRouterUp bool
	callback   ActivationCallback
	receiver   messaging.Receiver
}

func (h *heartBeatsClient) Start(stopCh <-chan struct{}, callback ActivationCallback) {
	if h.running {
		return
	}
	h.running = true
	h.callback = callback
	go h.run(stopCh)
	go h.handleShutdown(stopCh)
}

func (h *heartBeatsClient) routerDown(reason string) {
	h.isRouterUp = false
	h.callback.Stop()
	if h.isRouterUp {
		h.logger.Warn("router is down", slog.Any("reason", reason))
	} else {
		time.Sleep(5 * time.Second)
	}
}

func (h *heartBeatsClient) routerUp(stopCh <-chan struct{}) {
	if !h.isRouterUp {
		h.logger.Warn("router is up")
	}
	h.isRouterUp = true
	h.callback.Start(stopCh)
}

func (h *heartBeatsClient) run(stopCh <-chan struct{}) {
	h.logger.Info("watching for router availability")
	for h.running {
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
		_ = h.receiver.Close()
		h.running = false
	}
}

func (h *heartBeatsClient) getSiteId() (string, error) {
	// Loading runtime state
	siteStateLoader := &common.FileSystemSiteStateLoader{
		Path: api.GetInternalOutputPath(h.Namespace, api.RuntimeSiteStatePath),
	}
	siteState, err := siteStateLoader.Load()
	if err != nil {
		return "", fmt.Errorf("unable to load site state: %w", err)
	}
	return siteState.SiteId, nil
}

func (h *heartBeatsClient) getUrl() (string, error) {
	port, err := runtime.GetLocalRouterPort(h.Namespace)
	if err != nil {
		return "", fmt.Errorf("unable to determine local router url: %w", err)
	}
	return fmt.Sprintf("amqps://127.0.0.1:%d", port), nil
}

func (h *heartBeatsClient) getAddress() (string, error) {
	siteId, err := h.getSiteId()
	if err != nil {
		return "", fmt.Errorf("unable to determine siteId: %w", err)
	}
	address := fmt.Sprintf("/mc/sfe.%s.heartbeats", siteId)
	return address, nil
}

package eventsource

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"

	"github.com/skupperproject/skupper/pkg/vanflow"
	"github.com/skupperproject/skupper/pkg/vanflow/session"
)

const (
	sourceSuffixFlows      = ".flows"
	sourceSuffixLogs       = ".logs"
	sourceSuffixHeartbeats = ".heartbeats"
)

// Client is responsible for interacting with Event Sources
//
// Allows the caller to register message handler callbacks to react to received
// messages and to control exactly what sources the client listens to.
type Client struct {
	container   session.Container
	eventSource Info

	lock              sync.Mutex
	cleanup           []func()
	heartbeatHandlers []HeartbeatMessageHandler
	recordHandlers    []RecordMessageHandler

	wg     sync.WaitGroup
	logger *slog.Logger
}

type ClientOptions struct {
	// Source of vanflow events
	Source Info
}

func NewClient(container session.Container, cfg ClientOptions) *Client {
	c := &Client{
		container:   container,
		eventSource: cfg.Source,
		logger: slog.New(slog.Default().Handler()).With(
			slog.String("component", "vanflow.eventsource.client"),
			slog.String("instance", cfg.Source.ID),
		),
	}
	return c
}

type HeartbeatMessageHandler func(vanflow.HeartbeatMessage)
type RecordMessageHandler func(vanflow.RecordMessage)

// OnHeartbeat registers a callback handler for HeartbeatMessages.
func (c *Client) OnHeartbeat(handler HeartbeatMessageHandler) {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.heartbeatHandlers = append(c.heartbeatHandlers, handler)
}

// OnRecord registers a callback handler for RecordMessages.
func (c *Client) OnRecord(handler RecordMessageHandler) {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.recordHandlers = append(c.recordHandlers, handler)
}

func (c *Client) getHandlers() ([]RecordMessageHandler, []HeartbeatMessageHandler) {
	c.lock.Lock()
	defer c.lock.Unlock()
	records := make([]RecordMessageHandler, len(c.recordHandlers))
	heartbeats := make([]HeartbeatMessageHandler, len(c.heartbeatHandlers))
	copy(records, c.recordHandlers)
	copy(heartbeats, c.heartbeatHandlers)
	return records, heartbeats
}

// Listen instructs the Client to listen to an event source using the specified
// listener configuration until the context is cancelled or client.Close() is
// called.
func (c *Client) Listen(ctx context.Context, attributes ListenerConfigProvider) error {
	c.wg.Add(1)
	listenerCtx, listenerCancel := context.WithCancel(ctx)

	c.lock.Lock()
	defer c.lock.Unlock()
	c.cleanup = append(c.cleanup, listenerCancel)

	go func(ctx context.Context) {
		defer c.wg.Done()
		cfg := attributes.Get(c.eventSource)
		receiver := c.container.NewReceiver(cfg.Address, session.ReceiverOptions{
			Credit: cfg.Credit,
		})
		defer receiver.Close(ctx)
		for {
			amqpMsg, err := receiver.Next(ctx)
			if err != nil {
				if errors.Is(err, ctx.Err()) {
					return
				}
				c.logger.Error("client error receiving message", slog.Any("error", err), slog.String("address", cfg.Address))
				continue
			}
			if err := receiver.Accept(ctx, amqpMsg); err != nil {
				if errors.Is(err, ctx.Err()) {
					return
				}
				c.logger.Error("client error accepting message", slog.Any("error", err), slog.String("address", cfg.Address))
				continue
			}
			decoded, err := vanflow.Decode(amqpMsg)
			if err != nil {
				c.logger.Error("skipping message that could not be decoded", slog.Any("error", err))
				continue
			}
			recordHandlers, heartbeatHandlers := c.getHandlers()
			switch message := decoded.(type) {
			case vanflow.RecordMessage:
				for _, handler := range recordHandlers {
					handler(message)
				}
			case vanflow.HeartbeatMessage:
				for _, handler := range heartbeatHandlers {
					handler(message)
				}
			}
		}
	}(listenerCtx)
	return nil
}

// Close stops all listeners
func (c *Client) Close() {
	c.lock.Lock()
	defer c.lock.Unlock()
	for _, cancel := range c.cleanup {
		cancel()
	}
	c.wg.Wait()
}

// SendFlush sends a FlushMessage to the Event Source
func (c *Client) SendFlush(ctx context.Context) error {
	var flush vanflow.FlushMessage
	flush.To = c.eventSource.Direct
	msg := flush.Encode()

	sender := c.container.NewSender(c.eventSource.Direct, session.SenderOptions{})
	defer sender.Close(ctx)
	if err := sender.Send(ctx, msg); err != nil {
		return fmt.Errorf("client flush error: %w", err)
	}
	return nil
}

type ListenerConfig struct {
	Address string
	Credit  int
}

type ListenerConfigProvider interface {
	Get(Info) ListenerConfig
}

type addresser func(i Info) string

func (fn addresser) Get(info Info) ListenerConfig {
	return ListenerConfig{Address: fn(info), Credit: 256}
}

func FromSourceAddress() ListenerConfigProvider {
	return addresser(func(i Info) string { return i.Address })
}

func FromSourceAddressLogs() ListenerConfigProvider {
	return addresser(func(i Info) string { return i.Address + sourceSuffixLogs })
}

func FromSourceAddressFlows() ListenerConfigProvider {
	return addresser(func(i Info) string { return i.Address + sourceSuffixFlows })
}

func FromSourceAddressHeartbeats() ListenerConfigProvider {
	return addresser(func(i Info) string { return i.Address + sourceSuffixHeartbeats })
}

package main

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/c-kruse/vanflow/eventsource"
	"github.com/c-kruse/vanflow/session"
	"github.com/c-kruse/vanflow/store"
)

func newFlowIngest(factory session.ContainerFactory, dispatchRegistry *store.DispatchRegistry) *flowIngest {
	return &flowIngest{
		factory:          factory,
		dispatchRegistry: dispatchRegistry,
		sources:          make(map[string]context.CancelCauseFunc),
	}
}

type flowIngest struct {
	ctx       context.Context
	factory   session.ContainerFactory
	discovery *eventsource.Discovery

	dispatchRegistry *store.DispatchRegistry

	mu      sync.Mutex
	sources map[string]context.CancelCauseFunc
}

func (f *flowIngest) run(ctx context.Context) {
	f.ctx = ctx
	discoveryCtr := f.factory.Create()
	discoveryCtr.OnSessionError(func(err error) {
		slog.Info("discovery session container error", slog.Any("error", err))
	})

	discoveryCtr.Start(f.ctx)

	f.discovery = eventsource.NewDiscovery(discoveryCtr, eventsource.DiscoveryOptions{})
	err := f.discovery.Run(f.ctx, eventsource.DiscoveryHandlers{
		Discovered: f.onDiscoverSource,
		Forgotten:  f.onForgetSource,
	})
	if err != nil {
		slog.Error("discovery error", slog.Any("error", err))
	}
}

func (f *flowIngest) onDiscoverSource(source eventsource.Info) {
	clientCtx, cancel := context.WithCancelCause(f.ctx)
	// new container for client
	ctr := f.factory.Create()
	ctr.Start(clientCtx)
	client := eventsource.NewClient(ctr, eventsource.ClientOptions{Source: source})
	// register client with discovery to update lastseen, and monitor for staleness
	err := f.discovery.NewWatchClient(clientCtx, eventsource.WatchConfig{
		Client:      client,
		ID:          source.ID,
		Timeout:     time.Second * 30,
		GracePeriod: time.Second * 30,
	})
	if err != nil {
		slog.Error("status ingress error adding watch client for discovered source", slog.Any("error", err))
		f.discovery.Forget(source.ID)
		return
	}
	sourceRef := store.SourceRef{
		APIVersion: fmt.Sprint(source.Version),
		Type:       source.Type,
		Name:       source.ID,
	}
	// dispatch records to ingress store(s)
	dispatcher := f.dispatchRegistry.NewDispatcher(sourceRef)
	client.OnRecord(dispatcher.Dispatch)
	client.Listen(clientCtx, eventsource.FromSourceAddress())
	switch source.Type {
	case "CONTROLLER":
		client.Listen(f.ctx, eventsource.FromSourceAddressHeartbeats())
	case "ROUTER":
		client.Listen(f.ctx, eventsource.FromSourceAddressFlows())
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sources[source.ID] = cancel
	go func() {
		err := eventsource.FlushOnFirstMessage(clientCtx, client)
		if err != nil {
			slog.Error("error flushing on first message", slog.Any("error", err))
		}
		slog.Info("flush sent for new event source", slog.String("source", source.ID))
	}()
}

func (f *flowIngest) onForgetSource(source eventsource.Info) {
	f.mu.Lock()
	defer f.mu.Unlock()
	cancelCause, ok := f.sources[source.ID]
	if !ok {
		slog.Info("status ignoring discovery forget event for unknown client", slog.String("client", source.ID))
		return
	}
	cancelCause(fmt.Errorf("event source inactive: %w", context.DeadlineExceeded))
	delete(f.sources, source.ID)
}

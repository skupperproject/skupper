package collector

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/skupperproject/skupper/pkg/vanflow"
	"github.com/skupperproject/skupper/pkg/vanflow/eventsource"
	"github.com/skupperproject/skupper/pkg/vanflow/session"
	"github.com/skupperproject/skupper/pkg/vanflow/store"
	"golang.org/x/sync/errgroup"
)

func New(logger *slog.Logger, factory session.ContainerFactory, reg *prometheus.Registry) *Collector {
	sessionCtr := factory.Create()

	collector := &Collector{
		logger:        logger,
		session:       sessionCtr,
		discovery:     eventsource.NewDiscovery(sessionCtr, eventsource.DiscoveryOptions{}),
		clients:       make(map[string]*eventsource.Client),
		events:        make(chan changeEvent, 64),
		purgeQueue:    make(chan store.SourceRef, 8),
		recordRouting: make(eventsource.RecordStoreMap),
	}

	collector.Records = store.NewSyncMapStore(store.SyncMapStoreConfig{
		Handlers: store.EventHandlerFuncs{
			OnAdd:    collector.handleStoreAdd,
			OnChange: collector.handleStoreChange,
			OnDelete: collector.handleStoreDelete,
		},
		Indexers: map[string]store.Indexer{
			store.SourceIndex:      store.SourceIndexer,
			store.TypeIndex:        store.TypeIndexer,
			IndexByTypeParent:      indexByTypeParent,
			IndexByAddress:         indexByAddress,
			IndexByParentHost:      indexByParentHost,
			IndexByLifecycleStatus: indexByLifecycleStatus,
			IndexByTypeName:        indexByTypeName,
		},
	})
	routerCfg := collector.recordRouting
	for _, typ := range standardRecordTypes {
		routerCfg[typ.String()] = collector.Records
	}
	return collector
}

type Collector struct {
	logger *slog.Logger

	session   session.Container
	discovery *eventsource.Discovery

	mu            sync.Mutex
	clients       map[string]*eventsource.Client
	Records       store.Interface
	recordRouting eventsource.RecordStoreMap

	events     chan changeEvent
	purgeQueue chan store.SourceRef
}

func (c *Collector) Run(ctx context.Context) error {
	c.session.Start(ctx)
	g, ctx := errgroup.WithContext(ctx)
	g.Go(c.runSession(ctx))
	g.Go(c.runWorkQueue(ctx))
	g.Go(c.runDiscovery(ctx))
	g.Go(c.runRecordCleanup(ctx))
	return g.Wait()
}

func (c *Collector) runWorkQueue(ctx context.Context) func() error {

	return func() error {
		defer func() {
			c.logger.Info("queue worker shutdown complete")
		}()
		for {
			select {
			case <-ctx.Done():
				return nil
			case event := <-c.events:
				c.logger.Info("FLOW EVENT", slog.Any("event", event))
			}
		}
	}
}

func (c *Collector) runSession(ctx context.Context) func() error {
	return func() error {
		defer func() {
			c.logger.Info("session shutdown complete")
		}()
		sessionErrors := make(chan error, 1)
		c.session.OnSessionError(func(err error) {
			sessionErrors <- err
		})
		c.session.Start(ctx)
		for {
			select {
			case <-ctx.Done():
				return nil
			case err := <-sessionErrors:
				retryable, ok := err.(session.RetryableError)
				if !ok {
					return fmt.Errorf("unrecoverable session error: %w", err)
				}
				c.logger.Error("session error on collector container",
					slog.Any("error", retryable),
					slog.Duration("delay", retryable.Retry()),
				)

			}
		}
	}
}

func (c *Collector) runDiscovery(ctx context.Context) func() error {
	return func() error {
		defer func() {
			c.logger.Info("discovery shutdown complete")
		}()
		return c.discovery.Run(ctx, eventsource.DiscoveryHandlers{
			Discovered: c.discoveryHandler(ctx),
			Forgotten:  c.handleForgotten,
		})
	}
}

func (c *Collector) runRecordCleanup(ctx context.Context) func() error {
	return func() error {
		defer func() {
			c.logger.Info("record cleanup worker shutdown complete")
		}()
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		terminatedExemplar := store.Entry{
			Record: vanflow.SiteRecord{BaseRecord: vanflow.NewBase("", time.Unix(1, 0), time.Unix(2, 0))},
		}
		for {
			select {
			case <-ctx.Done():
				return nil
			case <-ticker.C:
				terminated := c.Records.Index(IndexByLifecycleStatus, terminatedExemplar)
				for _, e := range terminated {
					c.Records.Delete(e.Record.Identity())
				}
				if ct := len(terminated); ct > 0 {
					c.logger.Info("purged terminated records",
						slog.Int("count", ct),
					)
				}
			case source := <-c.purgeQueue:
				ct := c.purge(source)
				c.logger.Info("purged records from forgotten source",
					slog.String("source", source.ID),
					slog.Int("count", ct),
				)
			}
		}
	}
}

func (c *Collector) handleStoreAdd(e store.Entry) {
	select {
	case c.events <- addEvent{Record: e.Record}:
	default:
		c.logger.Error("Store event queue full")
	}
}

func (c *Collector) handleStoreChange(p, e store.Entry) {

	select {
	case c.events <- updateEvent{Prev: p.Record, Curr: e.Record}:
	default:
		c.logger.Error("Store event queue full")
	}
}
func (c *Collector) handleStoreDelete(e store.Entry) {

	select {
	case c.events <- deleteEvent{Record: e.Record}:
	default:
		c.logger.Error("Store event queue full")
	}
}

func (c *Collector) purge(source store.SourceRef) int {
	matching := c.Records.Index(store.SourceIndex, store.Entry{Metadata: store.Metadata{Source: source}})
	for _, record := range matching {
		c.Records.Delete(record.Record.Identity())
	}
	return len(matching)
}

func (c *Collector) discoveryHandler(ctx context.Context) func(eventsource.Info) {
	return func(source eventsource.Info) {
		c.logger.Info("starting client for new source", slog.String("id", source.ID), slog.String("type", source.Type))
		client := eventsource.NewClient(c.session, eventsource.ClientOptions{
			Source: source,
		})

		// register client with discovery to update lastseen, and monitor for staleness
		err := c.discovery.NewWatchClient(ctx, eventsource.WatchConfig{
			Client:      client,
			ID:          source.ID,
			Timeout:     time.Second * 30,
			GracePeriod: time.Second * 30,
		})

		if err != nil {
			c.logger.Error("error creating watcher for discoverd source", slog.Any("error", err))
			c.discovery.Forget(source.ID)
			return
		}

		router := eventsource.RecordStoreRouter{
			Stores: c.recordRouting,
			Source: sourceRef(source),
		}
		client.OnRecord(router.Route)
		client.Listen(ctx, eventsource.FromSourceAddress())
		if source.Type == "CONTROLLER" {
			client.Listen(ctx, eventsource.FromSourceAddressHeartbeats())
		}
		if source.Type == "ROUTER" {
			client.Listen(ctx, eventsource.FromSourceAddressFlows())
		}

		c.mu.Lock()
		defer c.mu.Unlock()
		c.clients[source.ID] = client

		go func() {
			ctx, cancel := context.WithTimeout(ctx, time.Second*5)
			defer cancel()
			if err := eventsource.FlushOnFirstMessage(ctx, client); err != nil {
				if errors.Is(err, ctx.Err()) {
					c.logger.Info("timed out waiting for first message. sending flush anyways")
					err = client.SendFlush(ctx)
				}
				if err != nil {
					c.logger.Error("error sending flush", slog.Any("error", err))
				}
			}
		}()
	}
}

func (c *Collector) handleForgotten(source eventsource.Info) {
	c.logger.Info("handling forgotten source", slog.String("id", source.ID))
	c.mu.Lock()
	defer c.mu.Unlock()
	client, ok := c.clients[source.ID]
	if ok {
		client.Close()
		delete(c.clients, source.ID)
	}
	c.purgeQueue <- sourceRef(source)
}

func sourceRef(source eventsource.Info) store.SourceRef {
	return store.SourceRef{
		Version: fmt.Sprint(source.Version),
		ID:      source.ID,
	}
}

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
		events:        make(chan changeEvent, 128),
		flows:         make(chan changeEvent, 1024),
		purgeQueue:    make(chan store.SourceRef, 8),
		recordRouting: make(eventsource.RecordStoreMap),
		metrics:       register(reg),
	}

	collector.Records = store.NewSyncMapStore(store.SyncMapStoreConfig{
		Handlers: store.EventHandlerFuncs{
			OnAdd:    collector.handleStoreAdd,
			OnChange: collector.handleStoreChange,
			OnDelete: collector.handleStoreDelete,
		},
		Indexers: RecordIndexers(),
	})
	collector.FlowRecords = store.NewSyncMapStore(store.SyncMapStoreConfig{
		Handlers: store.EventHandlerFuncs{
			OnAdd:    collector.handleFlowAdd,
			OnChange: collector.handleFlowChange,
			OnDelete: collector.handleFlowDelete,
		},
		Indexers: map[string]store.Indexer{
			store.SourceIndex: store.SourceIndexer,
			store.TypeIndex:   store.TypeIndexer,
			IndexByTypeParent: indexByTypeParent,
		},
	})
	collector.graph = NewGraph(collector.Records).(*graph)
	collector.processManager = newProcessManager(logger, collector.Records, collector.graph, newStableIdentityProvider(), collector.metrics)
	collector.addressManager = newAddressManager(collector.logger, collector.Records)
	collector.flowManager = newFlowManager(collector.logger, collector.graph, collector.FlowRecords, collector.Records, collector.metrics)
	routerCfg := collector.recordRouting
	for _, typ := range standardRecordTypes {
		routerCfg[typ.String()] = collector.Records
	}
	routerCfg[vanflow.TransportBiflowRecord{}.GetTypeMeta().String()] = collector.FlowRecords
	routerCfg[vanflow.AppBiflowRecord{}.GetTypeMeta().String()] = collector.FlowRecords
	return collector
}

type Collector struct {
	logger *slog.Logger

	session   session.Container
	discovery *eventsource.Discovery

	mu            sync.Mutex
	clients       map[string]*eventsource.Client
	Records       store.Interface
	FlowRecords   store.Interface
	graph         *graph
	recordRouting eventsource.RecordStoreMap

	processManager *processManager
	addressManager *addressManager
	flowManager    *flowManager

	events     chan changeEvent
	flows      chan changeEvent
	purgeQueue chan store.SourceRef

	metrics metrics
}

func (c *Collector) GetGraph() Graph {
	return c.graph
}

type FlowStateAccess interface {
	Get(id string) (FlowState, bool)
}

func (c *Collector) FlowStates() FlowStateAccess {
	return c.flowManager.state
}

func (c *Collector) Run(ctx context.Context) error {
	c.session.Start(ctx)
	g, ctx := errgroup.WithContext(ctx)
	g.Go(c.runSession(ctx))
	g.Go(c.runWorkQueue(ctx))
	g.Go(c.monitoring(ctx))
	g.Go(c.runDiscovery(ctx))
	g.Go(c.runRecordCleanup(ctx))
	g.Go(c.processManager.run(ctx))
	g.Go(c.addressManager.run(ctx))
	g.Go(c.flowManager.run(ctx))
	return g.Wait()
}

func (c *Collector) updateGraph(event changeEvent, stor readonly) {
	if dEvent, ok := event.(deleteEvent); ok {
		c.graph.Unindex(dEvent.Record)
		return
	}
	entry, ok := stor.Get(event.ID())
	if !ok {
		return
	}
	c.graph.Reindex(entry.Record)
}

func (c *Collector) monitoring(ctx context.Context) func() error {
	eventQueueSpace := c.metrics.internal.queueUtilization.WithLabelValues("records")
	flowQueueSpace := c.metrics.internal.queueUtilization.WithLabelValues("flows")
	return func() error {
		defer func() {
			c.logger.Info("collector monitoring shutdown complete")
		}()
		recordsCapacity := float64(cap(c.events))
		flowCapacity := float64(cap(c.flows))

		utilization := func(queue chan changeEvent, capacity float64) float64 {
			events := float64(len(queue))
			return events / capacity
		}
		for {
			select {
			case <-ctx.Done():
				return nil
			case <-time.After(time.Second * 5):
				recordsUtil := utilization(c.events, recordsCapacity)
				flowUtil := utilization(c.flows, flowCapacity)
				eventQueueSpace.Set(recordsUtil)
				flowQueueSpace.Set(flowUtil)
			}
		}
	}
}

func (c *Collector) runWorkQueue(ctx context.Context) func() error {
	// reactors respond to record changes. Should be quick, and perform minimal
	// read-only store ops. Handle any changes out of band.
	reactors := map[vanflow.TypeMeta][]func(event changeEvent, stor readonly){}
	for _, r := range standardRecordTypes {
		reactors[r] = append(reactors[r], c.updateGraph)
	}

	reactors[AddressRecord{}.GetTypeMeta()] = append(reactors[AddressRecord{}.GetTypeMeta()], c.updateGraph)
	reactors[FlowSourceRecord{}.GetTypeMeta()] = append(reactors[FlowSourceRecord{}.GetTypeMeta()], c.processManager.handleChangeEvent)
	reactors[vanflow.ListenerRecord{}.GetTypeMeta()] = append(reactors[vanflow.ListenerRecord{}.GetTypeMeta()], c.addressManager.handleChangeEvent)
	reactors[vanflow.ConnectorRecord{}.GetTypeMeta()] = append(reactors[vanflow.ConnectorRecord{}.GetTypeMeta()],
		c.addressManager.handleChangeEvent,
		c.processManager.handleChangeEvent,
		c.flowManager.handleCacheInvalidatingEvent,
	)
	reactors[vanflow.ProcessRecord{}.GetTypeMeta()] = append(reactors[vanflow.ProcessRecord{}.GetTypeMeta()],
		c.processManager.handleChangeEvent,
		c.flowManager.handleCacheInvalidatingEvent,
	)

	return func() error {
		defer func() {
			c.logger.Info("queue worker shutdown complete")
		}()
		for {
			select {
			case <-ctx.Done():
				return nil
			case event := <-c.flows:
				start := time.Now()
				typ := event.GetTypeMeta()
				c.flowManager.processEvent(event)
				c.metrics.internal.flowProcessingTime.WithLabelValues(typ.String()).Observe(time.Since(start).Seconds())
			case event := <-c.events:
				start := time.Now()
				typ := event.GetTypeMeta()
				for _, reactor := range reactors[typ] {
					reactor(event, c.Records)
				}
				c.metrics.internal.flowProcessingTime.WithLabelValues(typ.String()).Observe(time.Since(start).Seconds())
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

func (c *Collector) handleFlowAdd(e store.Entry) {
	select {
	case c.flows <- addEvent{Record: e.Record}:
	default:
		c.logger.Error("Flow event queue full")
	}
}

func (c *Collector) handleFlowChange(p, e store.Entry) {

	select {
	case c.flows <- updateEvent{Prev: p.Record, Curr: e.Record}:
	default:
		c.logger.Error("Flow event queue full")
	}
}
func (c *Collector) handleFlowDelete(e store.Entry) {

	select {
	case c.flows <- deleteEvent{Record: e.Record}:
	default:
		c.logger.Error("Flow event queue full")
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

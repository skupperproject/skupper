// Package collector implements the vanflow event listener that backs the
// network console collector. It contains a Collector responsible for
// orchestrating collection of vanflow records from remote sources into a
// vanflow Store as well as several controllers responsible for reacting to
// records to add inferred records and context into the Store.
package collector

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	opmetrics "github.com/skupperproject/skupper/cmd/network-observer/internal/collector/metrics"
	"github.com/skupperproject/skupper/pkg/vanflow"
	"github.com/skupperproject/skupper/pkg/vanflow/eventsource"
	"github.com/skupperproject/skupper/pkg/vanflow/session"
	"github.com/skupperproject/skupper/pkg/vanflow/store"
	"golang.org/x/sync/errgroup"
)

func New(logger *slog.Logger, factory session.ContainerFactory, reg *prometheus.Registry, flowRecordTTL time.Duration, flowLogger func(vanflow.RecordMessage)) *Collector {
	sessionCtr := factory.Create()

	collector := &Collector{
		logger:         logger,
		flowRecordTTL:  flowRecordTTL,
		session:        sessionCtr,
		discovery:      eventsource.NewDiscovery(sessionCtr, eventsource.DiscoveryOptions{}),
		sources:        make(map[string]eventSource),
		events:         make(chan changeEvent, 1024),
		purgeQueue:     make(chan store.SourceRef, 8),
		recordRouting:  make(eventsource.RecordStoreMap),
		metrics:        register(reg),
		metricsAdaptor: opmetrics.New(reg),
		flowLogging:    flowLogger,
	}

	collector.Records = store.NewSyncMapStore(store.SyncMapStoreConfig{
		Handlers: store.EventHandlerFuncs{
			OnAdd:    collector.handleStoreAdd,
			OnChange: collector.handleStoreChange,
			OnDelete: collector.handleStoreDelete,
		},
		Indexers: RecordIndexers(),
	})
	collector.graph = NewGraph(collector.Records).(*graph)
	collector.processManager = newProcessManager(logger, collector.Records, collector.graph, newStableIdentityProvider(), collector.metrics)
	collector.addressManager = newAddressManager(collector.logger, collector.Records)
	collector.pairManager = newPairManager(logger, collector.Records, collector.graph, collector.metrics)
	routerCfg := collector.recordRouting
	for _, typ := range standardRecordTypes {
		routerCfg[typ.String()] = collector.Records
	}
	return collector
}

type Collector struct {
	logger        *slog.Logger
	flowRecordTTL time.Duration
	flowLogging   func(vanflow.RecordMessage)

	session   session.Container
	discovery *eventsource.Discovery

	mu      sync.Mutex
	sources map[string]eventSource

	Records       store.Interface
	graph         *graph
	recordRouting eventsource.RecordStoreMap

	processManager *processManager
	addressManager *addressManager
	pairManager    *pairManager
	metricsAdaptor *opmetrics.Adaptor

	events     chan changeEvent
	purgeQueue chan store.SourceRef

	metrics metrics
}

type eventSource struct {
	client  *eventsource.Client
	manager *connectionManager
}

func (c *Collector) GetGraph() Graph {
	return c.graph
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
	g.Go(c.pairManager.run(ctx))
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
	if _, ok := event.(addEvent); ok {
		c.graph.Index(entry.Record)
	} else {
		c.graph.Reindex(entry.Record)
	}
}

func (c *Collector) monitoring(ctx context.Context) func() error {
	eventQueueSpace := c.metrics.internal.queueUtilization.WithLabelValues("records")
	return func() error {
		defer func() {
			c.logger.Info("collector monitoring shutdown complete")
		}()
		recordsCapacity := float64(cap(c.events))

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
				eventQueueSpace.Set(recordsUtil)
			}
		}
	}
}

func (c *Collector) dispatchMetricsEvents(event changeEvent, _ readonly) {
	switch event := event.(type) {
	case addEvent:
		c.metricsAdaptor.Add(event.Record)
	case updateEvent:
		c.metricsAdaptor.Update(event.Prev, event.Curr)
	case deleteEvent:
		c.metricsAdaptor.Remove(event.Record)
	}
}

func (c *Collector) runWorkQueue(ctx context.Context) func() error {
	// reactors respond to record changes. Should be quick, and perform minimal
	// read-only store ops. Handle any changes out of band.
	reactors := map[vanflow.TypeMeta][]func(event changeEvent, stor readonly){}
	for _, r := range standardRecordTypes {
		reactors[r] = append(reactors[r], c.updateGraph, c.dispatchMetricsEvents)
	}

	reactors[AddressRecord{}.GetTypeMeta()] = append(reactors[AddressRecord{}.GetTypeMeta()], c.updateGraph)
	reactors[FlowSourceRecord{}.GetTypeMeta()] = append(reactors[FlowSourceRecord{}.GetTypeMeta()], c.processManager.handleChangeEvent)
	reactors[vanflow.ListenerRecord{}.GetTypeMeta()] = append(reactors[vanflow.ListenerRecord{}.GetTypeMeta()], c.addressManager.handleChangeEvent)
	reactors[vanflow.ConnectorRecord{}.GetTypeMeta()] = append(reactors[vanflow.ConnectorRecord{}.GetTypeMeta()],
		c.addressManager.handleChangeEvent,
		c.processManager.handleChangeEvent,
	)
	reactors[vanflow.ProcessRecord{}.GetTypeMeta()] = append(reactors[vanflow.ProcessRecord{}.GetTypeMeta()],
		c.processManager.handleChangeEvent,
	)
	reactors[ProcPairRecord{}.GetTypeMeta()] = append(reactors[ProcPairRecord{}.GetTypeMeta()], c.pairManager.handleChangeEvent)

	return func() error {
		defer func() {
			c.logger.Info("queue worker shutdown complete")
		}()
		for {
			select {
			case <-ctx.Done():
				return nil
			case event := <-c.events:
				start := time.Now()
				typ := event.GetTypeMeta()
				event.ID()
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

func (c *Collector) handleStoreAdd(e store.Entry) {
	switch e.Record.(type) {
	case RequestRecord:
		return
	case ConnectionRecord:
		return
	}
	select {
	case c.events <- addEvent{Record: e.Record}:
	default:
		c.logger.Error("Store event queue full")
	}
}

func (c *Collector) handleStoreChange(p, e store.Entry) {
	switch e.Record.(type) {
	case RequestRecord:
		return
	case ConnectionRecord:
		return
	}
	select {
	case c.events <- updateEvent{Prev: p.Record, Curr: e.Record}:
	default:
		c.logger.Error("Store event queue full")
	}
}
func (c *Collector) handleStoreDelete(e store.Entry) {
	switch e.Record.(type) {
	case RequestRecord:
		return
	case ConnectionRecord:
		return
	}
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
			c.logger.Error("error creating watcher for discovered source", slog.Any("error", err))
			c.discovery.Forget(source.ID)
			return
		}

		sourceCtr := eventSource{
			client: client,
		}

		addresses := []eventsource.ListenerConfigProvider{
			eventsource.FromSourceAddress(),
		}

		router := eventsource.RecordStoreRouter{
			Stores: c.recordRouting,
			Source: sourceRef(source),
		}

		switch source.Type {
		case "CONTROLLER":
			addresses = append(addresses, eventsource.FromSourceAddressHeartbeats()) // listen to .heartbeats
		case "ROUTER":
			addresses = append(addresses, eventsource.FromSourceAddressFlows()) // listen to .flows
			sourceCtr.manager = newConnectionmanager(
				ctx,
				c.logger.With(slog.String("eventsource", fmt.Sprintf("%d/%s", source.Version, source.ID))),
				sourceRef(source),
				c.Records,
				c.graph,
				c.metrics,
				c.flowRecordTTL,
			)

			// route flow records to source-specific stores
			router.Stores = maps.Clone(router.Stores)
			for _, typ := range flowRecordTypes {
				router.Stores[typ.String()] = sourceCtr.manager.flows
			}
		}

		if c.flowLogging != nil {
			client.OnRecord(c.flowLogging)
		}
		client.OnRecord(router.Route)

		for _, address := range addresses {
			client.Listen(ctx, address)
		}

		c.mu.Lock()
		defer c.mu.Unlock()
		c.sources[source.ID] = sourceCtr

		go func() {
			ctx, cancel := context.WithTimeout(ctx, time.Second*5)
			defer cancel()
			if err := eventsource.FlushOnFirstMessage(ctx, client); err != nil {
				if errors.Is(err, ctx.Err()) {
					sendCtx, sendCancel := context.WithTimeout(ctx, time.Second*5)
					defer sendCancel()
					c.logger.Info("timed out waiting for first message. sending flush anyways")
					err = client.SendFlush(sendCtx)
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
	s, ok := c.sources[source.ID]
	if ok {
		s.client.Close()
		if s.manager != nil {
			s.manager.Stop()
		}
		delete(c.sources, source.ID)
	}
	c.purgeQueue <- sourceRef(source)
}

func sourceRef(source eventsource.Info) store.SourceRef {
	return store.SourceRef{
		Version: fmt.Sprint(source.Version),
		ID:      source.ID,
	}
}

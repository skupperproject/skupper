package collector

import (
	"container/list"
	"context"
	"log/slog"
	"maps"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/skupperproject/skupper/pkg/vanflow"
	"github.com/skupperproject/skupper/pkg/vanflow/store"
)

// connectionManager handles flow records for a specific event source.
type connectionManager struct {
	logger                *slog.Logger
	flows                 store.Interface
	records               store.Interface
	source                store.SourceRef
	graph                 *graph
	idp                   idProvider
	metrics               metrics
	mcMu                  sync.Mutex
	requestMetricsCache   map[labelSet]appMetrics
	transportMetricsCache map[labelSet]transportMetrics

	ttl time.Duration

	transportProcessingTime prometheus.Observer
	appProcessingTime       prometheus.Observer

	transportFlows *keyedLRUCache[transportState, *transportState]
	appFlows       *keyedLRUCache[appState, *appState]

	pairMu       sync.Mutex
	processPairs map[pair]bool

	attrMu          sync.Mutex
	processesCache  map[string]processAttributes
	connectorsCache map[string]connectorAttrs
	routerCache     map[string]routerAttrs
}

func newConnectionmanager(ctx context.Context, log *slog.Logger, source store.SourceRef, records store.Interface, graph *graph, metrics metrics, ttl time.Duration) *connectionManager {
	m := &connectionManager{
		logger:                  log,
		records:                 records,
		graph:                   graph,
		source:                  source,
		idp:                     newStableIdentityProvider(),
		metrics:                 metrics,
		ttl:                     ttl,
		transportProcessingTime: metrics.internal.flowProcessingTime.WithLabelValues(vanflow.TransportBiflowRecord{}.GetTypeMeta().String()),
		appProcessingTime:       metrics.internal.flowProcessingTime.WithLabelValues(vanflow.AppBiflowRecord{}.GetTypeMeta().String()),
		transportFlows: &keyedLRUCache[transportState, *transportState]{
			byID: make(map[string]*list.Element),
			lru:  list.New(),
		},
		appFlows: &keyedLRUCache[appState, *appState]{
			byID: make(map[string]*list.Element),
			lru:  list.New(),
		},
		processPairs:    make(map[pair]bool),
		processesCache:  make(map[string]processAttributes),
		connectorsCache: make(map[string]connectorAttrs),
		routerCache:     make(map[string]routerAttrs),
	}

	m.flows = store.NewSyncMapStore(store.SyncMapStoreConfig{
		Handlers: store.EventHandlerFuncs{
			OnAdd:    m.handleAdd,
			OnChange: m.handleChange,
			OnDelete: m.handleDelete,
		},
		Indexers: map[string]store.Indexer{
			store.TypeIndex: store.TypeIndexer,
		},
	})

	go m.run(ctx)
	return m
}

func (c *connectionManager) handleTransportFlow(record vanflow.TransportBiflowRecord) {
	state, ok := c.transportFlows.Get(record.ID)
	state.LastSeen = time.Now()
	if !ok {
		state.ID = record.ID
		state.FirstSeen = state.LastSeen
	}
	metrics := state.metrics
	if metrics == nil {
		state.Dirty = true
		c.transportFlows.Push(record.ID, state)
		return
	}
	if !state.Opened {
		metrics.opened.Inc()
		metrics.closed.Add(0)
		state.Opened = true
	}
	if !state.Terminated && record.EndTime != nil {
		terminated := record.EndTime.Compare(dref(record.StartTime).Time) >= 0
		if terminated {
			state.Terminated = true
			metrics.closed.Inc()
		}
	}
	if !state.LatencySet && record.Latency != nil && record.LatencyReverse != nil {
		delta := time.Microsecond * time.Duration(*record.Latency-*record.LatencyReverse)
		state.LatencySet = true
		metrics.latency.Observe(delta.Seconds())
		metrics.latencyLegacy.Observe(float64(*record.Latency))
		metrics.latencyLegacyReverse.Observe(float64(*record.LatencyReverse))
	}
	bs, br := dref(record.Octets), dref(record.OctetsReverse)
	sentInc := float64(bs - state.BytesSent)
	receivedInc := float64(br - state.BytesReceived)
	if receivedInc > 0 {
		metrics.received.Add(receivedInc)
		state.BytesReceived = br
	}
	if sentInc > 0 {
		metrics.sent.Add(sentInc)
		state.BytesSent = bs
	}
	c.transportFlows.Push(record.ID, state)
}

func (c *connectionManager) handleAppFlow(record vanflow.AppBiflowRecord) {
	state, ok := c.appFlows.Get(record.ID)
	if !ok {
		state.ID = record.ID
		state.TransportID = dref(record.Parent)
		state.FirstSeen = time.Now()
	}
	state.LastSeen = time.Now()
	metrics := state.metrics
	if metrics == nil {
		state.Dirty = true
		c.appFlows.Push(record.ID, state)
		return
	}
	if !state.Terminated && record.EndTime != nil {
		terminated := record.EndTime.Compare(dref(record.StartTime).Time) >= 0
		if terminated {
			state.Terminated = true
			metrics.requests.With(prometheus.Labels{
				"method": normalizeHTTPMethod(record.Method),
				"code":   normalizeHTTPResponseClass(record.Result),
			}).Inc()
		}
	}
	c.appFlows.Push(record.ID, state)
}

func (c *connectionManager) handleAdd(e store.Entry) {
	c.handleChange(e, e)
}

func (c *connectionManager) handleChange(p, e store.Entry) {
	start := time.Now()
	switch record := e.Record.(type) {
	case vanflow.TransportBiflowRecord:
		c.handleTransportFlow(record)
		c.transportProcessingTime.Observe(time.Since(start).Seconds())
	case vanflow.AppBiflowRecord:
		c.handleAppFlow(record)
		c.appProcessingTime.Observe(time.Since(start).Seconds())
	default:
		// ignore
	}
}

func normalizeHTTPMethod(method *string) string {
	m := dref(method)
	switch {
	case strings.EqualFold(m, http.MethodGet):
		return http.MethodGet
	case strings.EqualFold(m, http.MethodHead):
		return http.MethodHead
	case strings.EqualFold(m, http.MethodPost):
		return http.MethodPost
	case strings.EqualFold(m, http.MethodPut):
		return http.MethodPut
	case strings.EqualFold(m, http.MethodPatch):
		return http.MethodPatch
	case strings.EqualFold(m, http.MethodDelete):
		return http.MethodDelete
	case strings.EqualFold(m, http.MethodConnect):
		return http.MethodConnect
	case strings.EqualFold(m, http.MethodOptions):
		return http.MethodOptions
	case strings.EqualFold(m, http.MethodTrace):
		return http.MethodTrace
	default:
		return "unknown"
	}
}

func (c *connectionManager) handleDelete(e store.Entry) {
	switch record := e.Record.(type) {
	case vanflow.TransportBiflowRecord:
		c.transportFlows.Pop(record.ID)
		c.records.Delete(record.ID)
	case vanflow.AppBiflowRecord:
		c.appFlows.Pop(record.ID)
		c.records.Delete(record.ID)
	default:
		// ignore
	}
}

type reconcileReason int

const (
	success reconcileReason = iota
	missingRecord
	missingConnector
	missingSource
	missingDest
	missingTransport
	missingProtocol
	unreconciledTransport
)

func (c *connectionManager) reconcileRequest(state appState) (RequestRecord, reconcileReason) {
	var r RequestRecord
	entry, ok := c.flows.Get(state.ID)
	if !ok {
		return r, missingRecord
	}
	record, ok := entry.Record.(vanflow.AppBiflowRecord)
	if !ok {
		return r, missingRecord
	}
	transState, ok := c.transportFlows.Get(state.TransportID)
	if !ok {
		return r, missingTransport
	}
	if transState.metrics == nil {
		return r, unreconciledTransport
	}

	entry, ok = c.records.Get(transState.ID)
	if !ok {
		return r, missingRecord
	}
	connRecord, ok := entry.Record.(ConnectionRecord)
	if !ok {
		return r, missingRecord
	}

	protocol, ok := normalizeApplicationProtocol(record.Protocol)
	if !ok {
		return r, missingProtocol
	}

	rr := RequestRecord{
		ID:           record.ID,
		TransportID:  transState.ID,
		StartTime:    dref(record.StartTime).Time,
		EndTime:      dref(record.EndTime).Time,
		RoutingKey:   connRecord.RoutingKey,
		Protocol:     protocol,
		Connector:    connRecord.Connector,
		Listener:     connRecord.Listener,
		Source:       connRecord.Source,
		SourceSite:   connRecord.SourceSite,
		SourceRouter: connRecord.SourceRouter,
		Dest:         connRecord.Dest,
		DestSite:     connRecord.DestSite,
		DestRouter:   connRecord.DestRouter,
		SourceGroup:  connRecord.SourceGroup,
		DestGroup:    connRecord.DestGroup,

		stor: c.flows,
	}
	rr.metrics = c.getAppMetricSet(rr.toLabelSet())
	return rr, success
}

func (c *connectionManager) getAppMetricSet(l labelSet) appMetrics {
	c.mcMu.Lock()
	defer c.mcMu.Unlock()
	if c.requestMetricsCache == nil {
		c.requestMetricsCache = make(map[labelSet]appMetrics)
	}
	if m, ok := c.requestMetricsCache[l]; ok {
		return m
	}
	labels := l.asLabels()
	m := appMetrics{
		requests: c.metrics.requestsCounter.MustCurryWith(labels),
	}
	c.requestMetricsCache[l] = m
	return m
}

func (c *connectionManager) reconcile(state transportState) (ConnectionRecord, reconcileReason) {
	var cr ConnectionRecord
	entry, ok := c.flows.Get(state.ID)
	if !ok {
		return cr, missingRecord
	}
	record, ok := entry.Record.(vanflow.TransportBiflowRecord)
	if !ok {
		return cr, missingRecord
	}
	listenerID, connectorID := dref(record.Parent), dref(record.ConnectorID)
	cnctr, ok := c.connectorAttrs(connectorID)
	if !ok {
		return cr, missingConnector
	}
	var sourceNode Process
	listener := c.graph.Listener(listenerID)
	lRouterNode := listener.Parent()
	sourceSiteID := lRouterNode.Parent().ID()
	sourceSiteHost := dref(record.SourceHost)
	if sourceSiteID != "" && sourceSiteHost != "" {
		sourceNode = c.graph.SiteHost(SiteHostID(sourceSiteID, sourceSiteHost)).Process()
	}
	sourceproc, ok := c.processAttrs(sourceNode.ID())
	if !ok {
		return cr, missingSource
	}
	connector := c.graph.Connector(connectorID)
	cRouterNode := connector.Parent()
	dest := connector.Target()
	destproc, ok := c.processAttrs(dest.ID())
	if !ok {
		return cr, missingDest
	}

	sourceRattrs, ok := c.routerAttrs(lRouterNode.ID())
	if !ok {
		return cr, missingSource
	}

	destRattrs, ok := c.routerAttrs(cRouterNode.ID())
	if !ok {
		return cr, missingDest
	}

	cr = ConnectionRecord{
		ID:            record.ID,
		StartTime:     dref(record.StartTime).Time,
		EndTime:       dref(record.EndTime).Time,
		RoutingKey:    cnctr.Address,
		Protocol:      cnctr.Protocol,
		ConnectorHost: cnctr.Host,
		ConnectorPort: cnctr.Port,
		Source: NamedReference{
			ID:   sourceproc.ID,
			Name: sourceproc.Name,
		},
		SourceSite: NamedReference{
			ID:   sourceproc.SiteID,
			Name: sourceproc.SiteName,
		},
		SourceRouter: NamedReference(sourceRattrs),
		Dest: NamedReference{
			ID:   destproc.ID,
			Name: destproc.Name,
		},
		DestSite: NamedReference{
			ID:   destproc.SiteID,
			Name: destproc.SiteName,
		},
		DestRouter: NamedReference(destRattrs),
		Connector: NamedReference{
			ID: connectorID,
		},
		Listener: NamedReference{
			ID: listenerID,
		},
		SourceGroup: NamedReference{
			ID:   sourceproc.GroupID,
			Name: sourceproc.GroupName,
		},
		DestGroup: NamedReference{
			ID:   destproc.GroupID,
			Name: destproc.GroupName,
		},

		FlowStore: c.flows,
	}
	cr.metrics = c.getTransportMetricSet(cr.toLabelSet())
	return cr, success
}

func (c *connectionManager) getTransportMetricSet(l labelSet) transportMetrics {
	c.mcMu.Lock()
	defer c.mcMu.Unlock()
	if c.transportMetricsCache == nil {
		c.transportMetricsCache = make(map[labelSet]transportMetrics)
	}
	if m, ok := c.transportMetricsCache[l]; ok {
		return m
	}
	lRev := l
	lRev.SourceProcess, lRev.DestProcess = lRev.DestProcess, lRev.SourceProcess
	lRev.SourceSiteName, lRev.DestSiteName = lRev.DestSiteName, lRev.SourceSiteName
	lRev.SourceSiteID, lRev.DestSiteID = lRev.DestSiteID, lRev.SourceSiteID

	labels := l.asLabels()
	legacyLabels := maps.Clone(labels)
	legacyLabels["direction"] = "incoming"

	legacyLabelsReverse := lRev.asLabels()
	legacyLabelsReverse["direction"] = "outgoing"
	m := transportMetrics{
		opened:               c.metrics.flowOpenedCounter.With(labels),
		closed:               c.metrics.flowClosedCounter.With(labels),
		sent:                 c.metrics.flowBytesSentCounter.With(labels),
		received:             c.metrics.flowBytesReceivedCounter.With(labels),
		latency:              c.metrics.internal.flowLatency.With(labels),
		latencyLegacy:        c.metrics.internal.legancyLatency.With(legacyLabels),
		latencyLegacyReverse: c.metrics.internal.legancyLatency.With(legacyLabelsReverse),
	}
	c.transportMetricsCache[l] = m
	return m
}

func (c *connectionManager) run(ctx context.Context) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	go c.scheduleReconcile(ctx)
	go c.scheduleAppReconcile(ctx)
	invalidateCache := time.NewTicker(time.Second * 30)
	defer invalidateCache.Stop()
	purgeFlows := time.NewTicker(time.Second * 10)
	defer purgeFlows.Stop()
	rebuildPairs := time.NewTicker(time.Second * 3)
	defer rebuildPairs.Stop()
	reconcileFlowSource := time.NewTicker(time.Second * 5)
	defer reconcileFlowSource.Stop()
	reconcileSources := c.metrics.internal.reconcileTime.WithLabelValues(c.source.ID, "flow_sources")
	reconcilePairs := c.metrics.internal.reconcileTime.WithLabelValues(c.source.ID, "flow_pairs")
	reconcileEvictions := c.metrics.internal.reconcileTime.WithLabelValues(c.source.ID, "flow_evictions")
	flowSources := make(map[string]struct{})
	for {
		select {
		case <-ctx.Done():
			return
		case <-reconcileFlowSource.C:
			func() {
				start := time.Now()
				defer func() {
					reconcileSources.Observe(time.Since(start).Seconds())
				}()
				c.transportFlows.Matching(func(state *transportState) bool {
					// only reconcile flow sources for flows that are not yet
					// reconcilled, but have been waiting at least 15s (a delay
					// for any related records to propagate before perminately
					// associating a flow with an inferred source)
					return state.metrics == nil &&
						time.Since(state.FirstSeen) > 15*time.Second
				})(func(state transportState) bool {
					ent, ok := c.flows.Get(state.ID)
					if !ok {
						return true
					}
					flow, ok := ent.Record.(vanflow.TransportBiflowRecord)
					if !ok {
						return true
					}
					listener := c.graph.Listener(dref(flow.Parent))
					sourceSiteID := listener.Parent().Parent().ID()
					sourceSiteHost := dref(flow.SourceHost)

					if sourceSiteID == "" || sourceSiteHost == "" {
						return true
					}

					flowSourceID := c.idp.ID("flowsource", sourceSiteID, sourceSiteHost)
					if _, ok := flowSources[flowSourceID]; ok {
						return true
					}
					c.logger.Info("registering flow source", slog.String("site", sourceSiteID), slog.String("host", sourceSiteHost))
					c.records.Add(FlowSourceRecord{
						ID:    flowSourceID,
						Site:  sourceSiteID,
						Host:  sourceSiteHost,
						Start: time.Now(),
					}, c.source)
					flowSources[flowSourceID] = struct{}{}
					return true
				})
			}()
		case <-invalidateCache.C:
			func() {
				c.attrMu.Lock()
				defer c.attrMu.Unlock()
				c.processesCache = make(map[string]processAttributes)
				c.connectorsCache = make(map[string]connectorAttrs)
				c.routerCache = make(map[string]routerAttrs)
			}()
		case <-purgeFlows.C:
			func() {
				start := time.Now()
				defer func() {
					reconcileEvictions.Observe(time.Since(start).Seconds())
				}()
				{
					terminated := map[string]struct{}{}
					stale := map[string]struct{}{}
					cutoff := time.Now().Add(-1 * c.ttl)
					c.transportFlows.All()(func(state transportState) bool {
						if !state.LastSeen.Before(cutoff) {
							return false
						}
						if state.Terminated {
							terminated[state.ID] = struct{}{}
						} else {
							stale[state.ID] = struct{}{}
						}
						return true
					})

					if ct := len(terminated); ct > 0 {
						c.logger.Debug("purging terminated transport flows", slog.Int("count", ct))
						for id := range terminated {
							c.flows.Delete(id)
							c.records.Delete(id)
						}
					}
					if ct := len(stale); ct > 0 {
						c.logger.Info("purging stale transport flows", slog.Int("count", ct))
						for id := range stale {
							c.flows.Delete(id)
							c.records.Delete(id)
						}
					}
				}
				{
					terminated := map[string]struct{}{}
					stale := map[string]struct{}{}
					cutoff := time.Now().Add(-1 * c.ttl)
					c.appFlows.All()(func(state appState) bool {
						if !state.LastSeen.Before(cutoff) {
							return false
						}
						if state.Terminated {
							terminated[state.ID] = struct{}{}
						} else {
							stale[state.ID] = struct{}{}
						}
						return true
					})
					if ct := len(terminated); ct > 0 {
						c.logger.Debug("purging terminated app flows", slog.Int("count", ct))
						for id := range terminated {
							c.flows.Delete(id)
						}
					}
					if ct := len(stale); ct > 0 {
						c.logger.Info("purging stale app flows", slog.Int("count", ct))
						for id := range stale {
							c.flows.Delete(id)
						}
					}
				}
			}()
		case <-rebuildPairs.C:
			func() {
				start := time.Now()
				defer func() {
					reconcilePairs.Observe(time.Since(start).Seconds())
				}()
				c.pairMu.Lock()
				defer c.pairMu.Unlock()
				for pair, dirty := range c.processPairs {
					if !dirty {
						continue
					}

					id := c.idp.ID("processpair", pair.Source, pair.Dest, pair.Protocol)
					if _, ok := c.records.Get(id); !ok {
						record := ProcPairRecord{
							ID:       id,
							Source:   pair.Source,
							Dest:     pair.Dest,
							Protocol: pair.Protocol,
							Start:    time.Now(),
						}
						c.logger.Info("Adding process pairs", slog.Any("id", id))
						c.records.Add(record, store.SourceRef{ID: "self"})
					}
					c.processPairs[pair] = false
				}
			}()
		}
	}
}

type appReconcileResult struct {
	Dirty                          int
	PendingTransportCount          int
	PendingTransportReconcileCount int
	PendingProtocolCount           int
	PendingUnknownCount            int
	Reconciled                     []RequestRecord
}

func (c *connectionManager) runAppReconcile() appReconcileResult {
	var result appReconcileResult
	c.appFlows.Matching(func(state *appState) bool {
		return state.metrics == nil
	})(func(state appState) bool {
		if state.metrics != nil {
			return true
		}
		var push bool
		if state.Dirty {
			push = true
			state.Dirty = false
			result.Dirty++
		}

		request, reason := c.reconcileRequest(state)
		switch reason {
		case missingTransport:
			result.PendingTransportCount++
		case missingProtocol:
			result.PendingProtocolCount++
		case unreconciledTransport:
			result.PendingTransportReconcileCount++
		case success:
			c.records.Add(request, c.source)
			push = true
			metrics := request.metrics
			state.metrics = &metrics
			result.Reconciled = append(result.Reconciled, request)

			c.pairMu.Lock()
			p := pair{
				Protocol: request.Protocol,
				Source:   request.Source.ID,
				Dest:     request.Dest.ID,
			}
			if _, ok := c.processPairs[p]; !ok {
				c.processPairs[p] = true
			}
			c.pairMu.Unlock()
		default:
			result.PendingUnknownCount++
		}
		if push {
			c.appFlows.Push(state.ID, state)
		}
		return true
	})
	return result
}

func (c *connectionManager) scheduleAppReconcile(ctx context.Context) {
	reconcileProcesses := c.metrics.internal.reconcileTime.WithLabelValues(c.source.ID, "appflow_processes")
	b := backoff.WithContext(backoff.NewExponentialBackOff(
		backoff.WithMaxElapsedTime(0),
		backoff.WithMaxInterval(time.Second*5),
		backoff.WithInitialInterval(time.Second),
	), ctx)
	pending := c.metrics.internal.pendingFlows.MustCurryWith(prometheus.Labels{"eventsource": c.source.ID})
	ttyp := vanflow.AppBiflowRecord{}.GetTypeMeta().String()
	pendingTransport := pending.With(prometheus.Labels{
		"type":   ttyp,
		"reason": "transport",
	})
	pendingReconcile := pending.With(prometheus.Labels{
		"type":   ttyp,
		"reason": "transport_reconcile",
	})
	pendingProtocol := pending.With(prometheus.Labels{
		"type":   ttyp,
		"reason": "transport_protocol",
	})
	pendingUnknown := pending.With(prometheus.Labels{
		"type":   ttyp,
		"reason": "unknown",
	})
	pendingTransport.Set(0)
	pendingTransport.Set(0)
	pendingUnknown.Set(0)

	for {
		delay := b.NextBackOff()
		if delay == backoff.Stop {
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(delay):
		}
		func() {
			start := time.Now()
			defer func() {
				reconcileProcesses.Observe(time.Since(start).Seconds())
			}()
			result := c.runAppReconcile()
			pendingTransport.Set(float64(result.PendingTransportCount))
			pendingReconcile.Set(float64(result.PendingTransportReconcileCount))
			pendingProtocol.Set(float64(result.PendingProtocolCount))
			pendingUnknown.Set(float64(result.PendingUnknownCount))

			for _, r := range result.Reconciled {
				if flow, ok := r.GetFlow(); ok {
					c.handleAppFlow(flow)
				}
			}
			if result.Dirty > 0 {
				b.Reset()
			}
		}()
	}
}

type reconcileResult struct {
	PendingConnectorCount int
	PendingSourceCount    int
	PendingDestCount      int
	Dirty                 int
	Reconciled            []ConnectionRecord
}

func (c *connectionManager) runReconcile() reconcileResult {
	var result reconcileResult
	c.transportFlows.Matching(func(state *transportState) bool {
		return state.metrics == nil
	})(func(state transportState) bool {
		var push bool
		if state.Dirty {
			result.Dirty++
			push = true
			state.Dirty = false
		}
		connection, reason := c.reconcile(state)
		switch reason {
		case missingConnector:
			result.PendingConnectorCount++
		case missingSource:
			result.PendingSourceCount++
		case missingDest:
			result.PendingDestCount++
		case success:
			push = true
			c.records.Add(connection, c.source)
			metrics := connection.metrics
			state.metrics = &metrics
			result.Reconciled = append(result.Reconciled, connection)

			c.pairMu.Lock()
			p := pair{
				Protocol: connection.Protocol,
				Source:   connection.Source.ID,
				Dest:     connection.Dest.ID,
			}
			if _, ok := c.processPairs[p]; !ok {
				c.processPairs[p] = true
			}
			c.pairMu.Unlock()
		}
		if push {
			c.transportFlows.Push(state.ID, state)
		}
		return true
	})
	return result
}

func (c *connectionManager) scheduleReconcile(ctx context.Context) {
	reconcileProcesses := c.metrics.internal.reconcileTime.WithLabelValues(c.source.ID, "flow_processes")
	b := backoff.WithContext(backoff.NewExponentialBackOff(backoff.WithMaxElapsedTime(0), backoff.WithMaxInterval(time.Second*5)), ctx)
	pending := c.metrics.internal.pendingFlows.MustCurryWith(prometheus.Labels{"eventsource": c.source.ID})
	ttyp := vanflow.TransportBiflowRecord{}.GetTypeMeta().String()
	pendingConnector := pending.With(prometheus.Labels{
		"type":   ttyp,
		"reason": "connector",
	})
	pendingSource := pending.With(prometheus.Labels{
		"type":   ttyp,
		"reason": "source",
	})
	pendingDest := pending.With(prometheus.Labels{
		"type":   ttyp,
		"reason": "destination",
	})
	pendingConnector.Set(0)
	pendingSource.Set(0)
	pendingDest.Set(0)

	for {
		delay := b.NextBackOff()
		if delay == backoff.Stop {
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(delay):
		}
		func() {
			start := time.Now()
			defer func() {
				reconcileProcesses.Observe(time.Since(start).Seconds())
			}()
			result := c.runReconcile()
			pendingConnector.Set(float64(result.PendingConnectorCount))
			pendingSource.Set(float64(result.PendingSourceCount))
			pendingDest.Set(float64(result.PendingDestCount))

			for _, r := range result.Reconciled {
				if flow, ok := r.GetFlow(); ok {
					c.handleTransportFlow(flow)
				}
			}
			if result.Dirty > 0 {
				b.Reset()
			}
		}()
	}
}

func (c *connectionManager) Stop() {
	for _, e := range c.flows.List() {
		c.flows.Delete(e.Record.Identity())
	}
}

type transportMetrics struct {
	opened               prometheus.Counter
	closed               prometheus.Counter
	sent                 prometheus.Counter
	received             prometheus.Counter
	latency              prometheus.Observer
	latencyLegacy        prometheus.Observer
	latencyLegacyReverse prometheus.Observer
}
type appMetrics struct {
	requests *prometheus.CounterVec
}

type appState struct {
	ID          string
	TransportID string
	Dirty       bool
	Terminated  bool

	metrics *appMetrics

	FirstSeen time.Time
	LastSeen  time.Time
}

func (s *appState) Identity() string {
	return s.ID
}

type transportState struct {
	ID    string
	Dirty bool

	Opened        bool
	Terminated    bool
	BytesSent     uint64
	BytesReceived uint64
	LatencySet    bool

	metrics *transportMetrics

	FirstSeen time.Time
	LastSeen  time.Time
}

func (s *transportState) Identity() string {
	return s.ID
}

type ider interface {
	Identity() string
}

type keyedLRUCache[T any, PT interface {
	*T
	ider
}] struct {
	mu   sync.Mutex
	byID map[string]*list.Element
	lru  *list.List
}

func (q *keyedLRUCache[T, PT]) Get(id string) (T, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	elt, ok := q.byID[id]
	if !ok {
		var t T
		return t, false
	}
	return *elt.Value.(PT), true
}

func (q *keyedLRUCache[T, PT]) Push(id string, state T) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if item, ok := q.byID[id]; ok {
		item.Value = &state
		q.lru.MoveToBack(item)
		return
	}
	item := q.lru.PushBack(&state)
	q.byID[id] = item
}

func (q *keyedLRUCache[T, PT]) Pop(id string) (T, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	elt, ok := q.byID[id]
	if !ok {
		var t T
		return t, false
	}
	q.lru.Remove(elt)
	delete(q.byID, id)
	return *elt.Value.(PT), true
}

type sequence[T any] func(yield func(T) bool)

func (q *keyedLRUCache[T, PT]) Matching(matches func(PT) bool) sequence[T] {
	q.mu.Lock()
	defer q.mu.Unlock()
	var ids []string
	head := q.lru.Back()
	for head != nil {
		item := head.Value.(PT)
		if matches(item) {
			ids = append(ids, item.Identity())
		}
		head = head.Prev()
	}
	return q.iter(ids)
}

func (q *keyedLRUCache[T, PT]) All() sequence[T] {
	q.mu.Lock()
	defer q.mu.Unlock()
	ids := make([]string, q.lru.Len())
	head := q.lru.Front()
	idx := 0
	for head != nil {
		ids[idx] = head.Value.(PT).Identity()
		head = head.Next()
		idx++
	}
	return q.iter(ids)
}

func (q *keyedLRUCache[T, PT]) iter(ids []string) sequence[T] {
	return func(yield func(T) bool) {
		for _, id := range ids {
			v, ok := q.Get(id)
			if !ok {
				continue
			}
			if !yield(v) {
				return
			}
		}
	}
}

func (c *connectionManager) connectorAttrs(id string) (connectorAttrs, bool) {
	var attrs connectorAttrs
	c.attrMu.Lock()
	defer c.attrMu.Unlock()
	if cattr, ok := c.connectorsCache[id]; ok {
		return cattr, true
	}

	entry, ok := c.records.Get(id)
	if !ok {
		return attrs, false
	}
	cnctr, ok := entry.Record.(vanflow.ConnectorRecord)
	if !ok {
		return attrs, false
	}

	var complete bool
	if cnctr.Address != nil && cnctr.Protocol != nil {
		complete = true
		attrs.Address = *cnctr.Address
		attrs.Protocol = *cnctr.Protocol
		attrs.Host = dref(cnctr.DestHost)
		attrs.Port = dref(cnctr.DestPort)
		c.connectorsCache[id] = attrs
	}

	return attrs, complete
}

func (c *connectionManager) processAttrs(id string) (processAttributes, bool) {
	var attrs processAttributes
	c.attrMu.Lock()
	defer c.attrMu.Unlock()
	if pattr, ok := c.processesCache[id]; ok {
		return pattr, true
	}

	entry, ok := c.records.Get(id)
	if !ok {
		return attrs, false
	}
	proc, ok := entry.Record.(vanflow.ProcessRecord)
	if !ok || proc.Parent == nil || proc.Group == nil {
		return attrs, false
	}

	entry, ok = c.records.Get(*proc.Parent)
	if !ok {
		return attrs, false
	}
	site, ok := entry.Record.(vanflow.SiteRecord)
	if !ok {
		return attrs, false
	}
	groups := c.records.Index(IndexByTypeName, store.Entry{Record: ProcessGroupRecord{Name: *proc.Group}})
	if len(groups) == 0 {
		return attrs, false
	}
	group, ok := groups[0].Record.(ProcessGroupRecord)
	if !ok {
		return attrs, false
	}

	var complete bool
	if proc.Name != nil && site.Name != nil {
		complete = true
		attrs.ID = id
		attrs.Name = *proc.Name
		attrs.SiteID = site.ID
		attrs.SiteName = *site.Name
		attrs.GroupID = group.ID
		attrs.GroupName = group.Name
		c.processesCache[id] = attrs
	}
	return attrs, complete
}

func (c *connectionManager) routerAttrs(id string) (routerAttrs, bool) {
	var attrs routerAttrs
	c.attrMu.Lock()
	defer c.attrMu.Unlock()
	if rattr, ok := c.routerCache[id]; ok {
		return rattr, true
	}

	entry, ok := c.records.Get(id)
	if !ok {
		return attrs, false
	}
	rtr, ok := entry.Record.(vanflow.RouterRecord)
	if !ok {
		return attrs, false
	}

	var complete bool
	if rtr.Name != nil {
		complete = true
		attrs.ID = rtr.ID
		attrs.Name = *rtr.Name
		c.routerCache[id] = attrs
	}

	return attrs, complete
}

func normalizeHTTPResponseClass(result *string) string {
	class := "unknown"
	if result == nil {
		return class
	}
	code, err := strconv.Atoi(*result)
	if err != nil {
		return class
	}
	switch {
	case code < 200:
		return "1xx"
	case code < 300:
		return "2xx"
	case code < 400:
		return "3xx"
	case code < 500:
		return "4xx"
	case code < 600:
		return "5xx"
	default:
		return class
	}
}

func normalizeApplicationProtocol(result *string) (string, bool) {
	if result == nil {
		return "", false
	}
	switch *result {
	case "HTTP/1.0":
		fallthrough
	case "HTTP/1.1":
		return "http1", true
	case "HTTP/2":
		return "http2", true
	default:
		return *result, true
	}
}

func dref[T any](p *T) T {
	var t T
	if p != nil {
		return *p
	}
	return t
}

type pair struct {
	Source   string
	Dest     string
	Protocol string
}
type processAttributes struct {
	ID        string
	Name      string
	SiteID    string
	SiteName  string
	GroupID   string
	GroupName string
}

type connectorAttrs struct {
	Protocol string
	Address  string
	Host     string
	Port     string
}
type routerAttrs struct {
	ID   string
	Name string
}

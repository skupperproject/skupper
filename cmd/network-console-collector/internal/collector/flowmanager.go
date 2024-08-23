package collector

import (
	"container/list"
	"context"
	"log/slog"
	"strconv"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/skupperproject/skupper/pkg/vanflow"
	"github.com/skupperproject/skupper/pkg/vanflow/store"
)

type flowManager struct {
	logger  *slog.Logger
	flows   store.Interface
	records store.Interface
	graph   *graph
	idp     idProvider

	state   *flowStateQueue
	pending *flowStateQueue

	pairMu       sync.Mutex
	processPairs map[pair]bool

	attrMu          sync.Mutex
	processesCache  map[string]processAttributes
	connectorsCache map[string]connectorAttrs

	metrics metrics
}

type pair struct {
	Source   string
	Dest     string
	Protocol string
}
type processAttributes struct {
	ID       string
	Name     string
	SiteID   string
	SiteName string
}

type connectorAttrs struct {
	Protocol string
	Address  string
}

func newFlowManager(log *slog.Logger, graph *graph, flows, records store.Interface, m metrics) *flowManager {
	manager := &flowManager{
		logger:  log,
		graph:   graph,
		flows:   flows,
		records: records,
		idp:     newStableIdentityProvider(),
		state: &flowStateQueue{
			byID: make(map[string]*list.Element),
			lru:  list.New(),
		},
		pending: &flowStateQueue{
			byID: make(map[string]*list.Element),
			lru:  list.New(),
		},
		processPairs:    make(map[pair]bool),
		processesCache:  make(map[string]processAttributes),
		connectorsCache: make(map[string]connectorAttrs),
		metrics:         m,
	}

	return manager
}

func (m *flowManager) run(ctx context.Context) func() error {
	return func() error {
		defer func() {
			m.logger.Info("flow manager shutdown complete")
		}()

		purgeFlows := time.NewTicker(time.Second * 10)
		defer purgeFlows.Stop()
		rebuildPairs := time.NewTicker(time.Second * 3)
		defer rebuildPairs.Stop()
		reconcileFlowSource := time.NewTicker(time.Second * 5)
		defer reconcileFlowSource.Stop()

		reconcileSources := m.metrics.internal.reconcileTime.WithLabelValues("flow_sources")
		reconcilePairs := m.metrics.internal.reconcileTime.WithLabelValues("flow_pairs")
		reconcileEvictions := m.metrics.internal.reconcileTime.WithLabelValues("flow_evictions")

		flowSources := make(map[string]struct{})
		for {
			select {
			case <-ctx.Done():
				return nil
			case <-reconcileFlowSource.C:
				func() {
					start := time.Now()
					defer func() {
						reconcileSources.Observe(time.Since(start).Seconds())
					}()
					for _, state := range m.pending.Items() {
						if state.Conditions.HasSourceMatch {
							continue
						}
						entry, ok := m.flows.Get(state.ID)
						if !ok {
							continue
						}
						switch flow := entry.Record.(type) {
						case vanflow.TransportBiflowRecord:
							var sourceNode Process
							listener := m.graph.Listener(state.ListenerID)
							sourceSiteID := listener.Parent().Parent().ID()
							sourceSiteHost := dref(flow.SourceHost)
							if sourceSiteID != "" && sourceSiteHost != "" {
								sourceNode = m.graph.ConnectorTarget(ConnectorTargetID(sourceSiteID, sourceSiteHost)).Process()
							}
							if sourceNode.IsKnown() {
								m.processEvent(addEvent{Record: flow})
								continue
							}
							if sourceSiteID == "" ||
								sourceSiteHost == "" ||
								time.Since(state.FirstSeen) < 10*time.Second {
								continue
							}

							flowSourceID := m.idp.ID("flowsource", sourceSiteID, sourceSiteHost)
							if _, ok := flowSources[flowSourceID]; ok {
								continue
							}
							m.logger.Info("registering flow source", slog.String("site", sourceSiteID), slog.String("host", sourceSiteHost))
							m.records.Add(FlowSourceRecord{
								ID:    flowSourceID,
								Site:  sourceSiteID,
								Host:  sourceSiteHost,
								Start: time.Now(),
							}, store.SourceRef{ID: "self"})
							flowSources[flowSourceID] = struct{}{}
						case vanflow.AppBiflowRecord:
							if flow.Parent == nil {
								continue
							}
							parent := *flow.Parent
							transportState, ok := m.state.Get(parent)
							if !ok {
								continue
							}
							if transportState.Conditions.FullyQualified() {
								m.processEvent(addEvent{Record: flow})
							}
						default:
							continue
						}

					}
				}()
			case <-rebuildPairs.C:
				func() {
					start := time.Now()
					defer func() {
						reconcilePairs.Observe(time.Since(start).Seconds())
					}()
					m.pairMu.Lock()
					defer m.pairMu.Unlock()
					for pair, dirty := range m.processPairs {
						if !dirty {
							continue
						}

						sn := m.graph.Process(pair.Source)
						sSiteID := sn.Parent().ID()
						dn := m.graph.Process(pair.Dest)
						dSiteID := dn.Parent().ID()

						sourceProc, ok := sn.GetRecord()
						if !ok {
							return
						}
						destProc, ok := dn.GetRecord()
						if !ok {
							return
						}

						var (
							sourceGroupID string
							destGroupID   string
						)
						sGroup, dGroup := dref(sourceProc.Group), dref(destProc.Group)
						sGroupEntries := m.records.Index(IndexByTypeName, store.Entry{Record: ProcessGroupRecord{
							Name: sGroup,
						}})
						if len(sGroupEntries) > 0 {
							sourceGroupID = sGroupEntries[0].Record.Identity()
						}
						dGroupEntries := m.records.Index(IndexByTypeName, store.Entry{Record: ProcessGroupRecord{
							Name: dGroup,
						}})
						if len(dGroupEntries) > 0 {
							destGroupID = dGroupEntries[0].Record.Identity()
						}

						id := m.idp.ID("processpair", pair.Source, pair.Dest, pair.Protocol)
						if _, ok := m.records.Get(id); !ok {
							record := ProcPairRecord{
								ID:       id,
								Source:   pair.Source,
								Dest:     pair.Dest,
								Protocol: pair.Protocol,
								Start:    time.Now(),
							}
							m.logger.Info("Adding process pairs", slog.Any("id", id))
							m.records.Add(record, store.SourceRef{ID: "self"})
						}

						if sSiteID != "" && dSiteID != "" {
							id := m.idp.ID("sitepair", sSiteID, dSiteID, pair.Protocol)
							if _, ok := m.records.Get(id); !ok {
								record := SitePairRecord{
									ID:       id,
									Source:   sSiteID,
									Dest:     dSiteID,
									Protocol: pair.Protocol,
									Start:    time.Now(),
								}
								m.logger.Info("Adding site pairs", slog.Any("id", id))
								m.records.Add(record, store.SourceRef{ID: "self"})
							}
						}

						if sourceGroupID != "" && destGroupID != "" {
							id := m.idp.ID("processgrouppair", sourceGroupID, destGroupID, pair.Protocol)
							if _, ok := m.records.Get(id); !ok {
								record := ProcGroupPairRecord{
									ID:       id,
									Source:   sourceGroupID,
									Dest:     destGroupID,
									Protocol: pair.Protocol,
									Start:    time.Now(),
								}
								m.logger.Info("Adding process group pairs", slog.Any("id", id))
								m.records.Add(record, store.SourceRef{ID: "self"})
							}
						}

						m.processPairs[pair] = false
					}
				}()
			case <-purgeFlows.C:
				func() {
					start := time.Now()
					defer func() {
						reconcileEvictions.Observe(time.Since(start).Seconds())
					}()
					terminated := map[string]struct{}{}
					stale := map[string]struct{}{}
					pendingStale := map[string]struct{}{}
					m.state.Purge(func(state FlowState) bool {
						threshold := -15 * time.Minute
						if !state.LastSeen.Before(time.Now().Add(threshold)) {
							return false
						}
						if state.Conditions.Terminated {
							terminated[state.ID] = struct{}{}
						} else {
							stale[state.ID] = struct{}{}
						}
						return true
					})
					m.pending.Purge(func(state FlowState) bool {
						threshold := -1 * time.Minute
						if !state.LastSeen.Before(time.Now().Add(threshold)) {
							return false
						}
						pendingStale[state.ID] = struct{}{}
						return true
					})
					if ct := len(terminated); ct > 0 {
						m.logger.Debug("purging terminated flows", slog.Int("count", ct))
						for id := range terminated {
							m.flows.Delete(id)
						}
					}
					if ct := len(stale); ct > 0 {
						m.logger.Info("purging stale flows", slog.Int("count", ct))
						for id := range stale {
							m.flows.Delete(id)
						}
					}
					if ct := len(pendingStale); ct > 0 {
						m.logger.Info("purging stale flows with incomplete information", slog.Int("count", ct))
						for id := range pendingStale {
							m.flows.Delete(id)
						}
					}
					var (
						tPendingConnector int
						tPendingSource    int
						tPendingDest      int
						tPendingUnknown   int
						aPendingTransport int
						aPendingConnector int
						aPendingSource    int
						aPendingDest      int
						aPendingUnknown   int
					)
					for _, pending := range m.pending.Items() {
						if pending.IsAppFlow {
							if !pending.Conditions.HasTransportFlow {
								aPendingTransport++
								continue
							}
							if !pending.Conditions.HasConnectorMatch {
								aPendingConnector++
								continue
							}
							if !pending.Conditions.HasSourceMatch {
								aPendingSource++
								continue
							}
							if !pending.Conditions.HasDestMatch {
								aPendingDest++
								continue
							}
							aPendingUnknown++
						} else {
							if !pending.Conditions.HasConnectorMatch {
								tPendingConnector++
								continue
							}
							if !pending.Conditions.HasSourceMatch {
								tPendingSource++
								continue
							}
							if !pending.Conditions.HasDestMatch {
								tPendingDest++
								continue
							}
							tPendingUnknown++
						}
					}
					m.metrics.internal.pendingFlows.WithLabelValues("app", "transport").Set(float64(aPendingTransport))
					m.metrics.internal.pendingFlows.WithLabelValues("app", "connector").Set(float64(aPendingConnector))
					m.metrics.internal.pendingFlows.WithLabelValues("app", "source").Set(float64(aPendingSource))
					m.metrics.internal.pendingFlows.WithLabelValues("app", "dest").Set(float64(aPendingDest))
					m.metrics.internal.pendingFlows.WithLabelValues("app", "unknown").Set(float64(aPendingUnknown))
					m.metrics.internal.pendingFlows.WithLabelValues("transport", "connector").Set(float64(tPendingConnector))
					m.metrics.internal.pendingFlows.WithLabelValues("transport", "source").Set(float64(tPendingSource))
					m.metrics.internal.pendingFlows.WithLabelValues("transport", "dest").Set(float64(tPendingDest))
					m.metrics.internal.pendingFlows.WithLabelValues("transport", "unknown").Set(float64(tPendingUnknown))
				}()
			}
		}
	}
}

func (m *flowManager) handleCacheInvalidatingEvent(event changeEvent, _ readonly) {
	m.attrMu.Lock()
	defer m.attrMu.Unlock()
	delete(m.connectorsCache, event.ID())
	delete(m.processesCache, event.ID())
}

func (m *flowManager) processEvent(event changeEvent) {
	if _, ok := event.(deleteEvent); ok {
		m.state.Pop(event.ID())
		m.pending.Pop(event.ID())
		//todo(ck) decrement aggregate counts?
		return
	}

	entry, ok := m.flows.Get(event.ID())
	if !ok {
		return
	}
	state, _ := m.state.Get(event.ID())

	switch record := entry.Record.(type) {
	case vanflow.TransportBiflowRecord:
		prev := state
		previouslyActive := !prev.Conditions.Terminated
		m.updateTransportFlowState(record, &state)
		m.state.Push(state)
		log := m.logger.With(state.LogFields()...)
		if !state.Conditions.FullyQualified() {
			m.pending.Push(state)
			log.Debug("PENDING FLOW", slog.String("source_host", dref(record.SourceHost)))
			return
		}
		m.pending.Pop(state.ID)

		m.pairMu.Lock()
		p := pair{
			Protocol: state.Connector.Protocol,
			Source:   state.Source.ID,
			Dest:     state.Dest.ID,
		}
		if _, ok := m.processPairs[p]; !ok {
			m.processPairs[p] = true
		}
		m.pairMu.Unlock()

		var (
			prevOctets    uint64
			prevOctetsRev uint64
		)
		if update, ok := event.(updateEvent); ok {
			if prev, ok := update.Prev.(vanflow.TransportBiflowRecord); ok {
				prevOctets = dref(prev.Octets)
				prevOctetsRev = dref(prev.OctetsReverse)
			}
		}

		labels := state.labels()
		if !prev.Conditions.FullyQualified() {
			prevOctets, prevOctetsRev = 0, 0
			previouslyActive = true
			m.metrics.flowOpenedCounter.With(labels).Inc()
			m.metrics.flowClosedCounter.With(labels).Add(0)
		}

		if state.Conditions.Terminated && previouslyActive {
			m.metrics.flowClosedCounter.With(labels).Inc()
		}

		sentInc := float64(dref(record.Octets) - prevOctets)
		receivedInc := float64(dref(record.OctetsReverse) - prevOctetsRev)
		m.metrics.flowBytesSentCounter.With(labels).Add(sentInc)
		m.metrics.flowBytesReceivedCounter.With(labels).Add(receivedInc)

		log.Debug("FLOW",
			slog.Float64("bytes_out", sentInc),
			slog.Float64("bytes_in", receivedInc),
		)
	case vanflow.AppBiflowRecord:
		prev := state
		previouslyActive := !prev.Conditions.Terminated
		m.updateApplicationFlowState(record, &state)
		m.state.Push(state)
		log := m.logger.With(state.AppLogFields()...)
		if !state.Conditions.FullyQualified() {
			m.pending.Push(state)
			log.Debug("PENDING APP FLOW")
			return
		}
		m.pending.Pop(state.ID)

		if !prev.Conditions.FullyQualified() {
			previouslyActive = true
		}

		if state.Conditions.Terminated && previouslyActive {
			baseLabels := state.labels()
			baseLabels["protocol"] = dref(record.Protocol)
			baseLabels["method"] = dref(record.Method)
			baseLabels["code"] = httpResponseClass(record.Result)
			m.metrics.requestsCounter.With(baseLabels).Inc()
		}
		log.Debug("APP FLOW")
	}
}

func (m *flowManager) updateApplicationFlowState(flow vanflow.AppBiflowRecord, state *FlowState) {
	state.ID = flow.ID
	state.IsAppFlow = true
	state.LastSeen = time.Now()
	if flow.EndTime != nil {
		state.Conditions.Terminated = flow.EndTime.Compare(dref(flow.StartTime).Time) >= 0
	}

	state.ApplicationProtocol = dref(flow.Protocol)
	if flow.Parent == nil {
		return
	}
	transportID := *flow.Parent
	transportState, ok := m.state.Get(transportID)
	if !ok {
		return
	}
	state.TransportFlowID = transportID
	state.Conditions.HasConnectorMatch = transportState.Conditions.HasConnectorMatch
	state.Conditions.HasSourceMatch = transportState.Conditions.HasSourceMatch
	state.Conditions.HasDestMatch = transportState.Conditions.HasDestMatch
	state.Conditions.HasTransportFlow = true
	state.Connector = transportState.Connector
	state.Source = transportState.Source
	state.Dest = transportState.Dest
}

func (m *flowManager) updateTransportFlowState(flow vanflow.TransportBiflowRecord, state *FlowState) {
	state.ID = flow.ID
	state.LastSeen = time.Now()
	if state.FirstSeen.IsZero() {
		state.FirstSeen = state.LastSeen
	}
	if flow.EndTime != nil {
		state.Conditions.Terminated = flow.EndTime.Compare(dref(flow.StartTime).Time) >= 0
	}

	state.ListenerID, state.ConnectorID = dref(flow.Parent), dref(flow.ConnectorID)
	if !state.Conditions.HasSourceMatch {
		var sourceNode Process
		listener := m.graph.Listener(state.ListenerID)
		sourceSiteID := listener.Parent().Parent().ID()
		sourceSiteHost := dref(flow.SourceHost)
		if sourceSiteID != "" && sourceSiteHost != "" {
			sourceNode = m.graph.ConnectorTarget(ConnectorTargetID(sourceSiteID, sourceSiteHost)).Process()
		}
		if attrs, ok := m.processAttrs(sourceNode.ID()); ok {
			state.Source.ID = attrs.ID
			state.Source.Name = attrs.Name
			state.Source.SiteID = attrs.SiteID
			state.Source.SiteName = attrs.SiteName
			state.Conditions.HasSourceMatch = true
		}
	}

	if !state.Conditions.HasConnectorMatch {
		if attrs, ok := m.connectorAttrs(state.ConnectorID); ok {
			state.Connector.Protocol = attrs.Protocol
			state.Connector.Address = attrs.Address
			state.Conditions.HasConnectorMatch = true
		}
	}
	if !state.Conditions.HasDestMatch {
		connector := m.graph.Connector(state.ConnectorID)
		dest := connector.Target()
		if attrs, ok := m.processAttrs(dest.ID()); ok {
			state.Dest.ID = attrs.ID
			state.Dest.Name = attrs.Name
			state.Dest.SiteID = attrs.SiteID
			state.Dest.SiteName = attrs.SiteName
			state.Conditions.HasDestMatch = true
		}
	}

}

type flowStateQueue struct {
	mu   sync.Mutex
	byID map[string]*list.Element
	lru  *list.List
}

func (q *flowStateQueue) Len() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.lru.Len()
}
func (q *flowStateQueue) Get(id string) (FlowState, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	elt, ok := q.byID[id]
	if !ok {
		return FlowState{}, false
	}
	q.lru.MoveToBack(elt)
	return elt.Value.(FlowState), true
}

func (q *flowStateQueue) Push(state FlowState) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if item, ok := q.byID[state.ID]; ok {
		item.Value = state
		q.lru.MoveToBack(item)
		return
	}
	item := q.lru.PushBack(state)
	q.byID[state.ID] = item
}

func (q *flowStateQueue) Pop(id string) (FlowState, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	elt, ok := q.byID[id]
	if !ok {
		return FlowState{}, false
	}
	q.lru.Remove(elt)
	delete(q.byID, id)
	return elt.Value.(FlowState), true
}

func (q *flowStateQueue) Purge(purge func(FlowState) bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	for q.lru.Front() != nil {
		head := q.lru.Front()
		if remove := purge(head.Value.(FlowState)); !remove {
			return
		}
		q.lru.Remove(head)
	}
}
func (q *flowStateQueue) Items() []FlowState {
	q.mu.Lock()
	defer q.mu.Unlock()
	out := make([]FlowState, 0, len(q.byID))

	for item := q.lru.Front(); item != nil; item = item.Next() {
		out = append(out, item.Value.(FlowState))
	}
	return out
}

func (m *flowManager) connectorAttrs(id string) (connectorAttrs, bool) {
	var attrs connectorAttrs
	m.attrMu.Lock()
	defer m.attrMu.Unlock()
	if cattr, ok := m.connectorsCache[id]; ok {
		return cattr, true
	}

	entry, ok := m.records.Get(id)
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
		m.connectorsCache[id] = attrs
	}

	return attrs, complete
}

func (m *flowManager) processAttrs(id string) (processAttributes, bool) {
	var attrs processAttributes
	m.attrMu.Lock()
	defer m.attrMu.Unlock()
	if pattr, ok := m.processesCache[id]; ok {
		return pattr, true
	}

	entry, ok := m.records.Get(id)
	if !ok {
		return attrs, false
	}
	proc, ok := entry.Record.(vanflow.ProcessRecord)
	if !ok || proc.Parent == nil {
		return attrs, false
	}

	entry, ok = m.records.Get(*proc.Parent)
	if !ok {
		return attrs, false
	}
	site, ok := entry.Record.(vanflow.SiteRecord)
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
		m.processesCache[id] = attrs
	}

	return attrs, complete
}

type FlowState struct {
	ID        string
	IsAppFlow bool

	Conditions FlowStateConditions

	TransportFlowID     string
	ApplicationProtocol string

	ListenerID  string
	ConnectorID string
	Connector   struct {
		Protocol string
		Address  string
	}
	Source struct {
		ID       string
		Name     string
		SiteID   string
		SiteName string
	}
	Dest struct {
		ID       string
		Name     string
		SiteID   string
		SiteName string
	}

	FirstSeen time.Time
	LastSeen  time.Time
}

type FlowStateConditions struct {
	Terminated        bool
	HasConnectorMatch bool
	HasSourceMatch    bool
	HasDestMatch      bool
	HasTransportFlow  bool
}

func (c FlowStateConditions) FullyQualified() bool {
	return c.HasConnectorMatch && c.HasDestMatch && c.HasSourceMatch
}

func (s FlowState) LogFields() []any {
	return []any{
		slog.String("id", s.ID),
		slog.Bool("active", !s.Conditions.Terminated),
		slog.String("type", "transport"),
		slog.String("listener", s.ListenerID),
		slog.String("connector", s.ConnectorID),
		slog.String("source_site", s.Source.Name),
		slog.String("source_proc", s.Source.Name),
		slog.String("dest_site", s.Dest.Name),
		slog.String("dest_proc", s.Dest.Name),
		slog.String("routing_key", s.Connector.Address),
	}
}

func (s FlowState) AppLogFields() []any {
	return []any{
		slog.String("id", s.ID),
		slog.String("transport", s.TransportFlowID),
		slog.Bool("active", !s.Conditions.Terminated),
		slog.String("type", "application"),
		slog.String("listener", s.ListenerID),
		slog.String("connector", s.ConnectorID),
		slog.String("source_site", s.Source.Name),
		slog.String("source_proc", s.Source.Name),
		slog.String("dest_site", s.Dest.Name),
		slog.String("dest_proc", s.Dest.Name),
		slog.String("protocol", s.ApplicationProtocol),
	}
}
func (s FlowState) labels() prometheus.Labels {
	return map[string]string{
		"source_site_id":   s.Source.SiteID,
		"source_site_name": s.Source.SiteName,
		"dest_site_id":     s.Dest.SiteID,
		"dest_site_name":   s.Dest.SiteName,
		"routing_key":      s.Connector.Address,
		"protocol":         s.Connector.Protocol,
		"source_process":   s.Source.Name,
		"dest_process":     s.Dest.Name,
	}
}

func httpResponseClass(result *string) string {
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

func dref[T any](p *T) T {
	var t T
	if p != nil {
		return *p
	}
	return t
}

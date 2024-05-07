package main

import (
	"container/list"
	"context"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/c-kruse/vanflow"
	"github.com/c-kruse/vanflow/session"
	"github.com/c-kruse/vanflow/store"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	labels = []string{"extension", "reporter", "reporter_id", "security", "flags",
		"source_cluster", "source_namespace", "source_name", "source_is_root",
		"dest_cluster", "dest_namespace", "dest_name",
	}
	tcpSentTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "kiali_ext_tcp_sent_total",
		Help: "total bytes sent in a TCP connection",
	}, labels)
	tcpReceivedTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "kiali_ext_tcp_received_total",
		Help: "total bytes received in a TCP connection",
	}, labels)
	tcpOpenedTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "kiali_ext_tcp_connections_opened_total",
		Help: "total tcp connections opened",
	}, labels)
	tcpClosedTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "kiali_ext_tcp_connections_closed_total",
		Help: "total tcp connections closed",
	}, labels)

	collectorFlowsTotal = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "collector_flows_total",
		Help: "number of vanflow flow records stored",
	})
	collectorActiveFlowsEvictedTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "collector_active_flows_evicted_total",
		Help: "number of active flow pairs evicted for staleness",
	})
)

func newFlowCollector(factory session.ContainerFactory, name string) (*flowCollector, error) {

	reporterID, err := os.Hostname()
	if err != nil {
		return nil, err
	}
	staticLabels := prometheus.Labels{
		"extension":      name,
		"reporter_id":    reporterID,
		"reporter":       "combined",
		"security":       "plain",
		"flags":          "",
		"source_is_root": "true",
	}

	collector := &flowCollector{
		vSent:     tcpSentTotal.MustCurryWith(staticLabels),
		vReceived: tcpReceivedTotal.MustCurryWith(staticLabels),
		vOpened:   tcpOpenedTotal.MustCurryWith(staticLabels),
		vClosed:   tcpClosedTotal.MustCurryWith(staticLabels),

		activeFlows:       newFlowsQueue(),
		pendingEviction:   newFlowsQueue(),
		flowsPendingMatch: newUnmatchedQueue(),

		evictions: make(chan string, 32),
	}

	collector.flows = store.NewDefaultCachingStore(
		store.CacheConfig{
			Indexers: map[string]store.CacheIndexer{},
			EventHandlers: store.EventHandlerFuncs{
				OnAdd:    collector.addFlow,
				OnChange: collector.updateFlow,
			},
		},
	)

	collector.records = store.NewDefaultCachingStore(store.CacheConfig{
		Indexers: map[string]store.CacheIndexer{},
	})

	dispatch := &store.DispatchRegistry{}

	dispatch.RegisterStore(vanflow.FlowRecord{}, collector.flows)
	dispatch.RegisterStore(vanflow.RouterRecord{}, collector.records)
	dispatch.RegisterStore(vanflow.ListenerRecord{}, collector.records)
	dispatch.RegisterStore(vanflow.ConnectorRecord{}, collector.records)

	collector.ingest = newFlowIngest(factory, dispatch)

	return collector, nil
}

type flowCollector struct {
	vSent     *prometheus.CounterVec
	vReceived *prometheus.CounterVec
	vOpened   *prometheus.CounterVec
	vClosed   *prometheus.CounterVec

	ingest *flowIngest

	records store.Interface // store topology

	flows             store.Interface   // store the actual flow records
	activeFlows       *flowsQueue       // index containing active flow pairs
	pendingEviction   *flowsQueue       // index containing active flow pairs
	flowsPendingMatch *matchmakingQueue // index containing flows pending a matching pair

	evictions chan string
}

func (c *flowCollector) Run(ctx context.Context) error {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	go c.ingest.run(ctx)
	for {
		select {
		case <-ctx.Done():
			return nil
		case flowID := <-c.evictions:
			pair, ok := c.activeFlows.Pop(flowID)
			if !ok {
				slog.Error("evicting flow missing from queue", slog.String("flow", flowID))
				continue
			}
			c.pendingEviction.Push(pair)
		case now := <-ticker.C: // lazy cleanup routines

			// delete flows processed over 30s ago
			processedDeadline := now.Add(-30 * time.Second)
			var processed []store.Entry
			c.pendingEviction.Purge(func(pair flowPair) bool {
				sourceEntry, err := c.flows.Get(context.TODO(), store.Entry{Record: vanflow.FlowRecord{BaseRecord: vanflow.NewBase(pair.Source)}})
				if err != nil || !sourceEntry.Found {
					return true
				}
				destEntry, err := c.flows.Get(context.TODO(), store.Entry{Record: vanflow.FlowRecord{BaseRecord: vanflow.NewBase(pair.Dest)}})
				if err != nil || !destEntry.Found {
					return true
				}
				if sourceEntry.Entry.Meta.UpdatedAt.Before(processedDeadline) && destEntry.Entry.Meta.UpdatedAt.Before(processedDeadline) {
					processed = append(processed, sourceEntry.Entry, destEntry.Entry)
					return true
				}
				return false
			})
			for _, flow := range processed {
				err := c.flows.Delete(context.TODO(), flow)
				if err != nil {
					slog.Error("could not delete flow", slog.Any("error", err))
					continue
				}
				slog.Debug("removed expired flow", slog.Any("id", flow.Record.Identity()))
			}

			// delete flows without match when stale for 120s
			matchmakingDeadline := now.Add(-120 * time.Second)
			var unmatched []store.Entry
			c.flowsPendingMatch.Purge(func(flowID string) bool {
				flowEntry, err := c.flows.Get(context.TODO(), store.Entry{Record: vanflow.FlowRecord{BaseRecord: vanflow.NewBase(flowID)}})
				if err != nil || !flowEntry.Found {
					return true
				}
				if flowEntry.Meta.UpdatedAt.Before(matchmakingDeadline) {
					unmatched = append(unmatched, flowEntry.Entry)
					return true
				}
				return false
			})
			for _, flow := range unmatched {
				c.flows.Delete(context.TODO(), flow)
				slog.Debug("purging inactive flows", slog.String("flow", flow.Record.Identity()))
			}

			// delete stale active flow pairs older than 120s
			evictionDeadline := now.Add(-120 * time.Second)
			var evicted []store.Entry
			c.activeFlows.Purge(func(pair flowPair) bool {
				sourceEntry, err := c.flows.Get(context.TODO(), store.Entry{Record: vanflow.FlowRecord{BaseRecord: vanflow.NewBase(pair.Source)}})
				if err != nil || !sourceEntry.Found {
					return true
				}
				destEntry, err := c.flows.Get(context.TODO(), store.Entry{Record: vanflow.FlowRecord{BaseRecord: vanflow.NewBase(pair.Dest)}})
				if err != nil || !destEntry.Found {
					return true
				}
				if sourceEntry.Entry.Meta.UpdatedAt.Before(evictionDeadline) && destEntry.Entry.Meta.UpdatedAt.Before(evictionDeadline) {
					evicted = append(evicted, sourceEntry.Entry, destEntry.Entry)
					return true
				}
				return false
			})
			for _, flow := range evicted {
				err := c.flows.Delete(context.TODO(), flow)
				if err != nil {
					slog.Error("could not delete active flow", slog.Any("error", err))
					continue
				}
				slog.Debug("removed stale active flow", slog.Any("id", flow.Record.Identity()))
				collectorActiveFlowsEvictedTotal.Inc()
			}

			// update collector flows total metric
			resp, err := c.flows.List(context.TODO(), nil)
			if err != nil {
				slog.Debug("error listing flows", slog.Any("error", err))
				continue
			}
			collectorFlowsTotal.Set(float64(len(resp.Entries)))
		}
	}
}

func (c *flowCollector) addFlow(entry store.Entry) {
	c.updateFlow(store.Entry{}, entry)
}

func (c *flowCollector) updateFlow(prev, entry store.Entry) {
	flow := entry.Record.(*vanflow.FlowRecord)
	slog.Debug("flow update", slog.String("ID", flow.Identity()), slog.Any("meta", entry.Meta))

	pair, ok := c.activeFlows.Get(flow.ID)
	if !ok {
		pair, ok = c.flowsPendingMatch.MatchFlows(flow.ID, flow.Counterflow)
		if !ok {
			var cf string
			if flow.Counterflow != nil {
				cf = *flow.Counterflow
			}
			slog.Debug("adding pending flow pair match", slog.Any("counterflow", cf), slog.String("flow", flow.ID))
			c.flowsPendingMatch.Push(flow.ID, flow.Counterflow)
			return
		}

		source, dest, err := c.flowsFromPair(pair)
		if err != nil {
			slog.Error("couldn't get flows for pair", slog.Any("error", err))
		}
		// ignore pairs when labels cannot be pulled
		labelset, err := c.labelFlows(source, dest)
		if err != nil {
			slog.Debug("error resolving pair labels", slog.Any("counterflow", flow.Counterflow), slog.String("flow", flow.ID), slog.Any("error", err))
			return
		}
		pair.Labels = labelset
		c.activeFlows.Push(pair)
		if opened, err := c.vOpened.GetMetricWith(labelset.ToProm()); err == nil {
			opened.Inc()
		}
		if sent, err := c.vSent.GetMetricWith(labelset.ToProm()); err == nil {
			if source.Octets != nil {
				sent.Add(float64(*source.Octets))
			}
		}
		if received, err := c.vReceived.GetMetricWith(labelset.ToProm()); err == nil {
			if dest.Octets != nil {
				received.Add(float64(*dest.Octets))
			}
		}
	}

	// update sent/received
	if pf, ok := prev.Record.(*vanflow.FlowRecord); ok {
		var (
			octetsP uint64
			octetsC uint64
		)
		if pf.Octets != nil {
			octetsP = *pf.Octets
		}
		if flow.Octets != nil {
			octetsC = *flow.Octets
		}
		delta := octetsC - octetsP
		slog.Debug("active flow updated", slog.Any("bytes_delta", delta))
		if delta > 0 {
			switch flow.ID {
			case pair.Source:
				if sent, err := c.vSent.GetMetricWith(pair.Labels.ToProm()); err == nil {
					sent.Add(float64(delta))
				}
			case pair.Dest:
				if rcv, err := c.vReceived.GetMetricWith(pair.Labels.ToProm()); err == nil {
					rcv.Add(float64(delta))
				}
			}
		}
	}

	if c.flowPairClosed(pair) {
		slog.Debug("flow pair closed", slog.Any("pair", pair))
		if closed, err := c.vClosed.GetMetricWith(pair.Labels.ToProm()); err == nil {
			closed.Inc()
		}
		c.evictions <- pair.Source
	}
}

func (c *flowCollector) flowsFromPair(pair flowPair) (*vanflow.FlowRecord, *vanflow.FlowRecord, error) {
	sourceFlow, err := store.Get(context.TODO(), c.flows, &vanflow.FlowRecord{BaseRecord: vanflow.NewBase(pair.Source)})
	if err != nil {
		return nil, nil, err
	}
	destFlow, err := store.Get(context.TODO(), c.flows, &vanflow.FlowRecord{BaseRecord: vanflow.NewBase(pair.Dest)})
	return sourceFlow, destFlow, err
}

func (c *flowCollector) labelFlows(source, dest *vanflow.FlowRecord) (labelSet, error) {
	var labels labelSet
	if source.Parent == nil {
		return labels, fmt.Errorf("source flow missing parent referece")
	}
	listener, err := store.Get(context.TODO(), c.records, &vanflow.ListenerRecord{BaseRecord: vanflow.NewBase(*source.Parent)})
	if err != nil {
		return labels, err
	}
	if listener == nil {
		return labels, fmt.Errorf("could not find parent of listener side flow")
	}
	if dest.Parent == nil {
		return labels, fmt.Errorf("dest flow missing parent referece")
	}
	connector, err := store.Get(context.TODO(), c.records, &vanflow.ConnectorRecord{BaseRecord: vanflow.NewBase(*dest.Parent)})
	if err != nil {
		return labels, err
	}
	if connector == nil {
		return labels, fmt.Errorf("could not find parent of connector side flow")
	}

	listenerRouter, err := store.Get(context.TODO(), c.records, &vanflow.RouterRecord{BaseRecord: vanflow.NewBase(*listener.Parent)})
	if err != nil {
		return labels, err
	}
	if listenerRouter == nil {
		return labels, fmt.Errorf("could not find parent of listener")
	}

	connectorRouter, err := store.Get(context.TODO(), c.records, &vanflow.RouterRecord{BaseRecord: vanflow.NewBase(*connector.Parent)})
	if err != nil {
		return labels, err
	}
	if connectorRouter == nil {
		return labels, fmt.Errorf("could not find parent of connector")
	}

	labels.SourceCluster = "Kubernetes"
	if listenerRouter.Parent != nil {
		labels.SourceCluster = *listenerRouter.Parent
	}

	if listenerRouter.Hostname != nil {
		labels.Source = *listenerRouter.Hostname
	}
	if listenerRouter.Namespace != nil {
		labels.SourceNamespace = *listenerRouter.Namespace
	}

	labels.DestCluster = "Kuberentes"
	if connectorRouter.Parent != nil {
		labels.DestCluster = *connectorRouter.Parent
	}
	if connectorRouter.Namespace != nil {
		labels.DestNamespace = *connectorRouter.Namespace
	}
	if connector.Address != nil {
		labels.Dest = *connector.Address
	}
	return labels, nil
}

func (c *flowCollector) flowPairClosed(pair flowPair) bool {
	source, dest, err := c.flowsFromPair(pair)
	if err != nil {
		return false
	}
	sourceOpen := source.EndTime == nil || (source.StartTime != nil && source.StartTime.After(source.EndTime.Time))
	destOpen := dest.EndTime == nil || (dest.StartTime != nil && dest.StartTime.After(dest.EndTime.Time))
	if sourceOpen && destOpen {
		return false
	} else if sourceOpen {
		slog.Debug("half open flow", slog.String("source", pair.Source))
		return false
	} else if destOpen {
		slog.Debug("half open flow", slog.String("dest", pair.Dest))
		return false
	}
	return true
}

type flowPair struct {
	Source string
	Dest   string
	Labels labelSet
}

type labelSet struct {
	SourceCluster   string
	SourceNamespace string
	Source          string
	DestCluster     string
	DestNamespace   string
	Dest            string
}

func (l labelSet) ToProm() prometheus.Labels {
	return prometheus.Labels{
		"source_cluster":   l.SourceCluster,
		"source_namespace": l.SourceNamespace,
		"source_name":      l.Source,
		"dest_cluster":     l.DestCluster,
		"dest_namespace":   l.DestNamespace,
		"dest_name":        l.Dest,
	}
}

// matchmakingQueue a special queue for finding flow pairs. It stores flow ids
// in least recently accessed order so that stale flows can be quickly evicted
type matchmakingQueue struct {
	mu    sync.Mutex
	byID  map[string]*list.Element
	byCID map[string]*list.Element
	queue *list.List
}

func newUnmatchedQueue() *matchmakingQueue {
	return &matchmakingQueue{
		byID:  make(map[string]*list.Element),
		byCID: make(map[string]*list.Element),
		queue: list.New(),
	}
}

func (q *matchmakingQueue) Purge(purge func(string) bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	for q.queue.Front() != nil {
		head := q.queue.Front()
		if !purge(head.Value.(string)) {
			return
		}
		q.queue.Remove(head)
		v := head.Value.(string)
		delete(q.byID, v)
		delete(q.byCID, v)
	}
}

func (q *matchmakingQueue) Push(flowID string, counterID *string) {
	q.mu.Lock()
	defer q.mu.Unlock()
	item, ok := q.byID[flowID]
	if !ok {
		item = q.queue.PushBack(flowID)
	} else {
		q.queue.MoveToBack(item)
	}
	q.byID[flowID] = item
	if counterID != nil {
		q.byCID[*counterID] = item
	}
}

func (q *matchmakingQueue) MatchFlows(flowID string, counterID *string) (flowPair, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if counterID == nil || *counterID == "" {
		if item, ok := q.byCID[flowID]; ok {
			destFlowID := item.Value.(string)
			if flowItem, ok := q.byID[flowID]; ok {
				q.queue.Remove(flowItem)
				delete(q.byID, flowID)
				delete(q.byCID, flowID)
			}
			delete(q.byID, destFlowID)
			delete(q.byCID, destFlowID)
			q.queue.Remove(item)

			return flowPair{Source: flowID, Dest: destFlowID}, true
		}
	} else {
		if item, ok := q.byID[*counterID]; ok {
			sourceFlowID := item.Value.(string)
			q.queue.Remove(item)
			if flowItem, ok := q.byID[flowID]; ok {
				q.queue.Remove(flowItem)
				delete(q.byID, flowID)
				delete(q.byCID, flowID)
			}
			delete(q.byID, sourceFlowID)
			delete(q.byCID, sourceFlowID)
			return flowPair{Source: sourceFlowID, Dest: flowID}, true
		}
	}
	return flowPair{}, false
}

// flowsQueue is a special queue for storing flowPairs in least recently
// accessed order so that stale flow pairs can be quickly evicted
type flowsQueue struct {
	mu    sync.Mutex
	byID  map[string]*list.Element
	queue *list.List
}

func newFlowsQueue() *flowsQueue {
	return &flowsQueue{
		byID:  make(map[string]*list.Element),
		queue: list.New(),
	}
}

func (q *flowsQueue) Get(id string) (flowPair, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if item, ok := q.byID[id]; ok {
		q.queue.MoveToBack(item)
		return item.Value.(flowPair), true
	}
	return flowPair{}, false
}

func (q *flowsQueue) Pop(id string) (flowPair, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	item, ok := q.byID[id]
	if !ok {
		return flowPair{}, false
	}
	tup := q.queue.Remove(item).(flowPair)
	delete(q.byID, tup.Source)
	delete(q.byID, tup.Dest)
	return tup, true
}

func (q *flowsQueue) Push(tup flowPair) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if item, ok := q.byID[tup.Source]; ok {
		q.queue.MoveToBack(item)
		return
	}
	item := q.queue.PushBack(tup)
	q.byID[tup.Source] = item
	q.byID[tup.Dest] = item
}

func (q *flowsQueue) Purge(purge func(flowPair) bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	for q.queue.Front() != nil {
		head := q.queue.Front()
		if !purge(head.Value.(flowPair)) {
			return
		}
		q.queue.Remove(head)
	}
}

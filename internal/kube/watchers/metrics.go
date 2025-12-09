package watchers

import (
	"sync"
	"time"
)

type MetricsProvider interface {
	NewAddedMetric(kind string) CounterMetric
	NewDepthMetric(kind string) GaugeMetric
	NewDelayedMetric(kind string) ObservableMetric
	NewWorkDurationMetric(kind string) ObservableMetric
	NewRetriesMetric(kind string) CounterMetric
}

type CounterMetric interface {
	Inc()
}
type GaugeMetric interface {
	CounterMetric
	Dec()
}
type ObservableMetric interface {
	Observe(float64)
}

type noopMetric struct{}

func (noopMetric) Inc()            {}
func (noopMetric) Dec()            {}
func (noopMetric) Observe(float64) {}

type noopMetricsProvider struct{}

func (noopMetricsProvider) NewAddedMetric(kind string) CounterMetric           { return noopMetric{} }
func (noopMetricsProvider) NewDepthMetric(kind string) GaugeMetric             { return noopMetric{} }
func (noopMetricsProvider) NewDelayedMetric(kind string) ObservableMetric      { return noopMetric{} }
func (noopMetricsProvider) NewWorkDurationMetric(kind string) ObservableMetric { return noopMetric{} }
func (noopMetricsProvider) NewRetriesMetric(kind string) CounterMetric         { return noopMetric{} }

type metricsSet struct {
	Added        CounterMetric
	Depth        GaugeMetric
	Delayed      ObservableMetric
	WorkDuration ObservableMetric
	Retries      CounterMetric
}

type metricsQueue struct {
	provider MetricsProvider

	metricsMu sync.Mutex
	metrics   map[string]metricsSet
	pendingMu sync.Mutex
	pending   map[ResourceChange]time.Time
}

func (q *metricsQueue) add(evt ResourceChange) {
	q.pendingMu.Lock()
	defer q.pendingMu.Unlock()
	if _, ok := q.pending[evt]; ok {
		return
	}
	q.pending[evt] = time.Now()
	ms := q.metricsFor(evt.Handler.Kind())
	ms.Added.Inc()
	ms.Depth.Inc()
}

type metricsClose func(evt ResourceChange, retry bool)

func (q *metricsQueue) get(evt ResourceChange) metricsClose {
	q.pendingMu.Lock()
	defer q.pendingMu.Unlock()
	queuedAt, ok := q.pending[evt]
	if !ok {
		return q.done(time.Now())
	}
	q.metricsFor(evt.Handler.Kind()).Delayed.Observe(time.Since(queuedAt).Seconds())
	delete(q.pending, evt)
	return q.done(time.Now())
}

func (q *metricsQueue) done(startTime time.Time) metricsClose {
	return func(evt ResourceChange, retry bool) {
		ms := q.metricsFor(evt.Handler.Kind())
		ms.WorkDuration.Observe(time.Since(startTime).Seconds())
		ms.Depth.Dec()
		if retry {
			ms.Retries.Inc()
			q.add(evt)
		}
	}
}

func (q *metricsQueue) metricsFor(kind string) metricsSet {
	q.metricsMu.Lock()
	defer q.metricsMu.Unlock()
	if q.metrics == nil {
		q.metrics = map[string]metricsSet{}
	}
	if m, ok := q.metrics[kind]; ok {
		return m
	}
	m := metricsSet{
		Added:        q.provider.NewAddedMetric(kind),
		Depth:        q.provider.NewDepthMetric(kind),
		Delayed:      q.provider.NewDelayedMetric(kind),
		WorkDuration: q.provider.NewWorkDurationMetric(kind),
		Retries:      q.provider.NewRetriesMetric(kind),
	}
	q.metrics[kind] = m
	return m
}

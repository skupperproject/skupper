package flowlog

import (
	"context"
	"encoding/json"
	"hash/fnv"
	"log/slog"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"github.com/skupperproject/skupper/pkg/vanflow"
	"golang.org/x/time/rate"
)

const (
	recordTypeMatchesAll string = "internal.flowlog.matchAll"
	doNotSample                 = staticSampler(false)
)

// Rule specifies how particular vanflow record types should be logged.
type Rule struct {
	// Priority of the rule. Lowest matching a record type wins.
	Priority int
	// Match is the set of record types the rule applies to
	Match RecordTypeSet
	// Strategy for sampling records
	Strategy SampleStrategy
}

type MessageHandler func(vanflow.RecordMessage)

// New creates a MessageHandler given a set of rules and a log output function
func New(ctx context.Context, logFn func(msg string, args ...any), rules []Rule) MessageHandler {
	handler := &handler{
		logFn: logFn,
	}

	for _, rule := range rules {
		if rule.Strategy == nil || rule.Match == nil {
			continue
		}
		handler.rules = append(handler.rules, rule)
	}
	slices.SortFunc(handler.rules, func(l, r Rule) int {
		return l.Priority - r.Priority
	})
	go handler.report(ctx)
	return handler.handle
}

type SampleStrategy interface {
	// Sample returns true when the record should be logged
	Sample(r vanflow.Record) bool
}

type staticSampler bool

func (s staticSampler) Sample(vanflow.Record) bool {
	return bool(s)
}

// Unlimited SampleStrategy always samples
func Unlimited() SampleStrategy {
	return staticSampler(true)
}

type rateLimited struct {
	limiter *rate.Limiter
}

// RateLimited SampleStrategy samples events up to a limit (in events per
// second). Events exceeding the limit are not logged.
func RateLimited(limit float64, burst int) SampleStrategy {
	return rateLimited{
		limiter: rate.NewLimiter(rate.Limit(limit), burst),
	}
}

func (r rateLimited) Sample(vanflow.Record) bool {
	return r.limiter.Allow()
}

// TransportFlowHash uses a deterministic hash based on a TransportBiflow ID.
// Uses the AppBiflow Parent field (Transport ID) so that ideally related flows
// are sampled together.
func TransportFlowHash(percent float64, parent SampleStrategy) SampleStrategy {
	if percent < 0 || percent >= 1.0 {
		panic("percent must be value in range [0, 1)")
	}
	if parent == nil {
		parent = Unlimited()
	}
	return hashBasedSampler{
		parent: parent,
		mod:    10_000,
		q:      uint32(percent * 10_000),
	}
}

type hashBasedSampler struct {
	parent SampleStrategy
	mod    uint32
	q      uint32
}

func (h hashBasedSampler) Sample(r vanflow.Record) bool {
	var transportID string
	switch flow := r.(type) {
	case vanflow.TransportBiflowRecord:
		transportID = flow.ID
	case vanflow.AppBiflowRecord:
		if flow.Parent == nil {
			return false
		}
		transportID = *flow.Parent
	default:
		return false
	}
	hash := fnv.New32a()
	hash.Write([]byte(transportID))
	m := hash.Sum32() % h.mod
	if h.q >= m {
		return h.parent.Sample(r)
	}
	return false
}

// RecordTypeSet specifies the Record Types that a Rule is applicable for.
type RecordTypeSet map[vanflow.TypeMeta]struct{}

func (r RecordTypeSet) matchesAll() bool {
	_, ok := r[vanflow.TypeMeta{APIVersion: recordTypeMatchesAll}]
	return ok
}

func NewRecordTypeSet(records ...vanflow.Record) RecordTypeSet {
	set := RecordTypeSet{}
	for _, r := range records {
		set[r.GetTypeMeta()] = struct{}{}
	}
	return set
}

// NewRecordTypeSetAll returns a special RecordTypeSet that matches records of
// all types
func NewRecordTypeSetAll() RecordTypeSet {
	set := RecordTypeSet{}
	set[vanflow.TypeMeta{APIVersion: recordTypeMatchesAll}] = struct{}{}
	return set
}

type handler struct {
	logFn func(msg string, args ...any)
	rules []Rule

	resolved sync.Map
	sampled  sync.Map
}

func (h *handler) report(ctx context.Context) {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			h.logReport()
		}
	}
}

func (h *handler) logReport() {
	sampleCounts := make(map[string]int)
	h.sampled.Range(func(k, v any) bool {
		h.sampled.Delete(k)
		typ, ct := k.(vanflow.TypeMeta), v.(*atomic.Int64)
		sampleCounts[typ.String()] = int(ct.Load())
		return true
	})

	if len(sampleCounts) == 0 {
		return
	}
	var counts []any
	for typ, count := range sampleCounts {
		counts = append(counts, slog.Int(typ, count))

	}
	h.logFn("some vanflow records were not logged", counts...)
}

func (h *handler) resolve(typ vanflow.TypeMeta) SampleStrategy {
	r, ok := h.resolved.Load(typ)
	if ok {
		return r.(SampleStrategy)
	}
	var strategy SampleStrategy = doNotSample
	for _, rule := range h.rules {
		if rule.Match.matchesAll() {
			strategy = rule.Strategy
			break
		}
		if _, ok := rule.Match[typ]; ok {
			strategy = rule.Strategy
			break
		}
	}
	h.resolved.Store(typ, strategy)
	return strategy
}

func (h *handler) handle(msg vanflow.RecordMessage) {
	attrs := slog.Group("message", slog.String("to", msg.To), slog.String("subject", msg.Subject))
	for _, record := range msg.Records {
		typ := record.GetTypeMeta()
		strategy := h.resolve(typ)
		if !strategy.Sample(record) {
			if strategy != doNotSample {
				prev, _ := h.sampled.LoadOrStore(typ, new(atomic.Int64))
				prev.(*atomic.Int64).Add(1)
			}
			continue
		}

		// TODO(ck) more efficient slog.LogValuer for vanflow records?
		raw, _ := json.Marshal(record)
		var out map[string]any
		json.Unmarshal(raw, &out)
		recordValues := make([]any, 0, len(out))
		for k, v := range out {
			if v == nil {
				continue
			}
			recordValues = append(recordValues, slog.Any(k, v))
		}
		h.logFn(record.GetTypeMeta().String(), slog.Group("record", recordValues...), attrs)
	}
}

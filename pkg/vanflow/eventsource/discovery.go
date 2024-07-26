package eventsource

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/skupperproject/skupper/pkg/vanflow"
	"github.com/skupperproject/skupper/pkg/vanflow/session"
)

const beaconAddress = "mc/sfe.all"

// Discovery manages a collection of known event sources
type Discovery struct {
	container     session.Container
	lock          sync.Mutex
	state         map[string]Info
	discovered    chan Info
	forgotten     chan Info
	beaconAddress string
	logger        *slog.Logger
}

type DiscoveryHandlers struct {
	Discovered func(source Info)
	Forgotten  func(source Info)
}

type DiscoveryOptions struct {
	BeaconAddress string
}

func NewDiscovery(container session.Container, opts DiscoveryOptions) *Discovery {
	if opts.BeaconAddress == "" {
		opts.BeaconAddress = beaconAddress
	}
	return &Discovery{
		container:     container,
		state:         make(map[string]Info),
		discovered:    make(chan Info, 32),
		forgotten:     make(chan Info, 32),
		beaconAddress: opts.BeaconAddress,
		logger: slog.New(slog.Default().Handler()).With(
			slog.String("component", "vanflow.eventsource.discovery"),
		),
	}
}

// Run event source discovery until the context is cancelled
func (d *Discovery) Run(ctx context.Context, handlers DiscoveryHandlers) error {
	receiver := d.container.NewReceiver(d.beaconAddress, session.ReceiverOptions{})
	defer receiver.Close(ctx)
	dCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	go d.handleDiscovery(dCtx, handlers)
	for {
		msg, err := receiver.Next(ctx)
		if err != nil {
			return fmt.Errorf("discovery error receiving beacon messages: %w", err)
		}
		if err := receiver.Accept(ctx, msg); err != nil {
			d.logger.Error("discovery got error accepting message", slog.Any("error", err))
		}
		if *msg.Properties.Subject != "BEACON" {
			d.logger.Info("received non-beacon from beacon source", slog.Any("subject", msg.Properties.Subject), slog.Any("source", msg.Properties.To))
			continue
		}
		d.observe(vanflow.DecodeBeacon(msg))
	}
}

func (d *Discovery) handleDiscovery(ctx context.Context, handlers DiscoveryHandlers) {
	for {
		select {
		case <-ctx.Done():
			return
		case info := <-d.discovered:
			if handlers.Discovered != nil {
				handlers.Discovered(info)
			}
		case info := <-d.forgotten:
			if handlers.Forgotten != nil {
				handlers.Forgotten(info)
			}
		}
	}
}

// Get an EventSource by ID
func (d *Discovery) Get(id string) (source Info, ok bool) {
	d.lock.Lock()
	defer d.lock.Unlock()
	state, ok := d.state[id]
	if !ok {
		return source, false
	}
	return state, true
}

// Add an EventSource
func (d *Discovery) Add(info Info) bool {
	d.lock.Lock()
	defer d.lock.Unlock()
	if _, exists := d.state[info.ID]; exists {
		return false
	}
	d.state[info.ID] = info
	d.discovered <- info
	return true
}

// Forget an EventSource
func (d *Discovery) Forget(id string) bool {
	var (
		state  Info
		forget bool
	)
	d.lock.Lock()
	defer d.lock.Unlock()
	state, forget = d.state[id]
	delete(d.state, id)
	if forget {
		d.forgotten <- state
	}
	return forget
}

// List all known EventSources
func (d *Discovery) List() []Info {
	d.lock.Lock()
	defer d.lock.Unlock()
	results := make([]Info, 0, len(d.state))
	for _, state := range d.state {
		results = append(results, state)
	}
	return results
}

type WatchConfig struct {
	// Client to watch
	Client *Client
	// ID of the event source to watch
	ID string
	// Timeout for client activity. When set, timeout is the duration that
	// discovery will wait after the most recent client activity before
	// forgetting the source.
	Timeout time.Duration

	// GracePeriod is the time to wait for client activity before enforcing
	// the Timeout.
	GracePeriod time.Duration

	// DiscoveryUpdateInterval is the period at which discovery gets updated
	// with the latest LastSeen timestamp from the watcher. Defaults to once
	// per second.
	DiscoveryUpdateInterval time.Duration
}

// NewWatchClient creates a client for a given event source and uses that client
// to keep the source LastHeard time up to date.
func (d *Discovery) NewWatchClient(ctx context.Context, cfg WatchConfig) error {
	_, ok := d.Get(cfg.ID)
	if !ok {
		return fmt.Errorf("unknown event source %s", cfg.ID)
	}

	w := newWatch(d, cfg.Client)
	go w.run(ctx, cfg)
	return nil
}

type watch struct {
	lastSeen  atomic.Pointer[time.Time]
	discovery *Discovery
	client    *Client
}

func newWatch(discovery *Discovery, client *Client) *watch {
	w := watch{
		discovery: discovery,
		client:    client,
	}
	client.OnHeartbeat(func(vanflow.HeartbeatMessage) {
		ts := time.Now()
		w.lastSeen.Store(&ts)
	})

	client.OnRecord(func(vanflow.RecordMessage) {
		ts := time.Now()
		w.lastSeen.Store(&ts)
	})
	return &w
}

func (w *watch) run(ctx context.Context, cfg WatchConfig) {
	var (
		watchTimer  *time.Timer
		watchTimerC <-chan time.Time
	)

	if cfg.Timeout > 0 {
		watchTimer = time.NewTimer(cfg.GracePeriod + cfg.Timeout)
		watchTimerC = watchTimer.C
		defer watchTimer.Stop()
	}

	// only update discovery data once per second to keep
	// lock contention reasonable.
	advanceInterval := cfg.DiscoveryUpdateInterval
	if advanceInterval <= 0 {
		advanceInterval = time.Second
	}
	advanceTicker := time.NewTicker(advanceInterval)
	defer advanceTicker.Stop()
	defer w.client.Close()

	var prevObserved time.Time
	prevObserved, _ = w.advanceLastSeen(prevObserved)
	for {
		select {
		case <-ctx.Done():
			return
		case <-watchTimerC:
			// Watch Timeout has been reached. Check one last time for watch
			// activity before forgetting the event source.
			next, ok := w.advanceLastSeen(prevObserved)
			if !ok {
				w.discovery.Forget(cfg.ID)
				return
			}
			w.discovery.lastSeen(cfg.ID, next)
			watchTimer.Reset(cfg.Timeout)
			prevObserved = next
		case <-advanceTicker.C:
			next, ok := w.advanceLastSeen(prevObserved)
			if !ok {
				continue
			}

			w.discovery.lastSeen(cfg.ID, next)
			if watchTimer != nil {
				if !watchTimer.Stop() {
					<-watchTimer.C
				}
				watchTimer.Reset(cfg.Timeout)
			}
			prevObserved = next
		}
	}
}

func (w *watch) advanceLastSeen(prev time.Time) (time.Time, bool) {
	currentObserved := w.lastSeen.Load()
	if currentObserved == nil || !currentObserved.After(prev) {
		return prev, false
	}
	return *currentObserved, true
}

func (d *Discovery) observe(beacon vanflow.BeaconMessage) {
	var (
		state      Info
		discovered bool
	)
	d.lock.Lock()
	defer d.lock.Unlock()
	tObserved := time.Now()
	state, ok := d.state[beacon.Identity]
	if !ok {
		state = Info{
			ID:       beacon.Identity,
			Version:  int(beacon.Version),
			Type:     beacon.SourceType,
			Address:  beacon.Address,
			Direct:   beacon.Direct,
			LastSeen: tObserved,
		}
		discovered = true
	}
	state.LastSeen = tObserved
	d.state[beacon.Identity] = state
	if discovered {
		d.discovered <- state
	}
}

func (d *Discovery) lastSeen(id string, latest time.Time) {
	d.lock.Lock()
	defer d.lock.Unlock()
	state, ok := d.state[id]
	if !ok {
		return
	}
	if latest.After(state.LastSeen) {
		state.LastSeen = latest
		d.state[id] = state
	}
}

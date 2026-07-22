package eventsource

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	amqp "github.com/Azure/go-amqp"
	"github.com/skupperproject/skupper/pkg/vanflow"
	"github.com/skupperproject/skupper/pkg/vanflow/encoding"
	"github.com/skupperproject/skupper/pkg/vanflow/session"
	"github.com/skupperproject/skupper/pkg/vanflow/store"
)

const (
	// recordSendTimeout bounds a single attempt to send a record message.
	recordSendTimeout = 5 * time.Second
	// recordSendRetryDelay is the pause before retrying a failed record
	// message send.
	recordSendRetryDelay = time.Second
)

type ManagerConfig struct {
	Source Info

	Stores []store.Interface

	// HeartbeatInterval defaults to 2 seconds
	HeartbeatInterval time.Duration
	// BeaconInterval defaults to 10 seconds
	BeaconInterval time.Duration

	// UseAlternateHeartbeatAddress indicates that the manager should send
	// heartbeat messages to the source address with the `.heartbeats` suffix.
	UseAlternateHeartbeatAddress bool

	// FlushDelay is the amount of time to wait after receiving an initial
	// flush message before beginning to send updates.
	FlushDelay time.Duration
	// FlushBatchSize is the maximum number of records that will be sent in a
	// single record message on flush.
	FlushBatchSize int

	// UpdateBufferTime is the amount of time to wait for a full batch of
	// record updates before sending a partial batch.
	UpdateBufferTime time.Duration
	// UpdateBatchSize is the maximum number of record updates that will be
	// sent in a single record message. Defaults to 1.
	UpdateBatchSize int
}

func (c ManagerConfig) updateBatchSize() int {
	if c.UpdateBatchSize < 1 {
		return 1
	}
	return c.UpdateBatchSize
}

type RecordUpdate struct {
	Prev vanflow.Record
	Curr vanflow.Record
}

type Manager struct {
	ManagerConfig
	container session.Container

	notify chan struct{}

	mu           sync.Mutex
	pending      map[string]RecordUpdate
	pendingSince time.Time
	flushAt      *time.Time

	logger *slog.Logger
}

func NewManager(container session.Container, cfg ManagerConfig) *Manager {
	return &Manager{
		container:     container,
		ManagerConfig: cfg,
		notify:        make(chan struct{}, 1),
		pending:       make(map[string]RecordUpdate),
		logger: slog.New(slog.Default().Handler()).With(
			slog.String("component", "vanflow.eventsource.manager"),
			slog.String("instance", cfg.Source.ID),
		),
	}
}

// PublishUpdate queues a record update for delivery.
func (m *Manager) PublishUpdate(update RecordUpdate) {
	if update.Curr == nil {
		return
	}
	id := update.Curr.Identity()

	m.mu.Lock()
	// squash existing updates
	if prior, ok := m.pending[id]; ok {
		update.Prev = prior.Prev
	}
	m.pending[id] = update
	if m.pendingSince.IsZero() {
		m.pendingSince = time.Now()
	}
	m.mu.Unlock()

	m.signalWorkAvailable()
}

func (m *Manager) signalWorkAvailable() {
	select {
	case m.notify <- struct{}{}:
	default:
	}
}

func (m *Manager) Run(ctx context.Context) {
	go m.listenFlushes(ctx)
	go m.sendKeepalives(ctx)
	m.sendRecords(ctx)
}

func (m *Manager) sendRecords(ctx context.Context) {
	sender := m.container.NewSender(m.Source.Address, session.SenderOptions{})
	defer sender.Close(ctx)

	var retryIn time.Duration
	for {
		if !m.awaitSendWork(ctx, retryIn) {
			return
		}
		retryIn = 0

		if m.flushDue(time.Now()) {
			if err := m.streamFlush(ctx, sender); err != nil {
				if ctx.Err() != nil {
					return
				}
				m.requestFlush()
				retryIn = recordSendRetryDelay
				m.logger.Error("error sending flush record message. retrying",
					slog.Any("error", err))
			}
			continue
		}

		batch := m.take(m.updateBatchSize())
		msg, included := m.encodeUpdates(batch)
		if msg == nil {
			continue
		}
		if err := sendWithTimeout(ctx, recordSendTimeout, sender, msg); err != nil {
			if ctx.Err() != nil {
				return
			}
			m.requeue(included)
			retryIn = recordSendRetryDelay
			m.logger.Error("error sending record message. retrying",
				slog.Any("error", err),
				slog.Int("record_count", len(included)))
			continue
		}
		m.logger.Info("record message sent", slog.Int("record_count", len(included)))
	}
}

// awaitSendWork blocks until the record sender has a flush or records due
// returns false when context is cancelled first.
func (m *Manager) awaitSendWork(ctx context.Context, retryIn time.Duration) bool {
	if retryIn > 0 {
		return sleep(ctx, retryIn)
	}
	for {
		wait, ready := m.workState(time.Now())
		switch {
		case ready:
			return true
		case wait > 0:
			// There is work, but it is not due yet. Try again after a delay or
			// notify signal.
			if !m.waitFor(ctx, wait) {
				return false
			}
		default:
			select {
			case <-ctx.Done():
				return false
			case <-m.notify:
			}
		}
	}
}

// waitFor waits up to d for notify signal. Returns false when the context was
// cancelled first.
func (m *Manager) waitFor(ctx context.Context, d time.Duration) bool {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
	case <-m.notify:
	}
	return true
}

// workState reports whether the record sender should run now, or how long to
// wait before asking again. A zero wait with ready false means there is
// nothing outstanding.
func (m *Manager) workState(now time.Time) (wait time.Duration, ready bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var flushWait time.Duration
	if m.flushAt != nil {
		if !now.Before(*m.flushAt) {
			return 0, true
		}
		flushWait = m.flushAt.Sub(now)
	}

	// Queued updates are still served while a flush waits out its delay
	updateWait, updateReady := m.updateWorkState(now)
	switch {
	case updateReady:
		return 0, true
	case updateWait > 0 && (flushWait == 0 || updateWait < flushWait):
		return updateWait, false
	default:
		return flushWait, false
	}
}

// updateWorkState reports the state of the pending updates. Callers must hold
// the manager lock.
func (m *Manager) updateWorkState(now time.Time) (wait time.Duration, ready bool) {
	if len(m.pending) == 0 {
		return 0, false
	}
	if m.UpdateBufferTime <= 0 || len(m.pending) >= m.updateBatchSize() {
		return 0, true
	}
	if elapsed := now.Sub(m.pendingSince); elapsed < m.UpdateBufferTime {
		return m.UpdateBufferTime - elapsed, false
	}
	return 0, true
}

// take removes up to limit updates from the pending set.
func (m *Manager) take(limit int) []RecordUpdate {
	m.mu.Lock()
	defer m.mu.Unlock()
	if limit > len(m.pending) {
		limit = len(m.pending)
	}
	if limit < 1 {
		return nil
	}
	batch := make([]RecordUpdate, 0, limit)
	for id, update := range m.pending {
		batch = append(batch, update)
		delete(m.pending, id)
		if len(batch) == limit {
			break
		}
	}
	if len(m.pending) == 0 {
		m.pendingSince = time.Time{}
	}
	return batch
}

// requeue returns a batch that failed to send to the pending set.
func (m *Manager) requeue(batch []RecordUpdate) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, failed := range batch {
		id := failed.Curr.Identity()
		if current, ok := m.pending[id]; ok {
			failed = current
		}
		// a failed send has an uncertain outcome. Do not presume prior state.
		failed.Prev = nil
		m.pending[id] = failed
	}
	if m.pendingSince.IsZero() {
		m.pendingSince = time.Now()
	}
}

// encodeUpdates encodes the updates containing a delta into a message. Returns
// a message and the included record updates
func (m *Manager) encodeUpdates(batch []RecordUpdate) (*amqp.Message, []RecordUpdate) {
	var (
		records  []vanflow.Record
		included []RecordUpdate
	)
	for _, update := range batch {
		delta, changed := m.diffRecord(update)
		if !changed {
			continue
		}
		records = append(records, delta)
		included = append(included, update)
	}
	if len(records) == 0 {
		m.logger.Debug("record updates buffered but none were changed", slog.Int("record_count", len(batch)))
		return nil, nil
	}
	msg, err := vanflow.RecordMessage{Records: records}.Encode()
	if err != nil {
		m.logger.Error("skipping record message after encoding error",
			slog.Any("error", err),
			slog.Int("record_count", len(records)))
		return nil, nil
	}
	return msg, included
}

func (m *Manager) diffRecord(d RecordUpdate) (vanflow.Record, bool) {
	if d.Prev == nil {
		return d.Curr, true
	}
	prev, err := encoding.Encode(d.Prev)
	if err != nil {
		m.logger.Error("record update diff error encoding prev", slog.Any("error", err))
		return nil, false
	}
	next, err := encoding.Encode(d.Curr)
	if err != nil {
		m.logger.Error("record update diff error encoding curr", slog.Any("error", err))
		return nil, false
	}
	var isDiff bool
	delta := make(map[any]any)
	for k := range next {
		pv, ok := prev[k]
		if !ok {
			isDiff = true
			delta[k] = next[k]
			continue
		}
		if next[k] != pv {
			isDiff = true
			delta[k] = next[k]
		}
		kCodePoint, ok := k.(uint32)
		if ok && kCodePoint < 2 { // hack to keep identifying info in the record
			delta[k] = next[k]
		}
	}
	if !isDiff {
		return nil, false
	}
	deltaRecord, err := encoding.Decode(delta)
	if err != nil {
		m.logger.Error("record update diff error decoding delta", slog.Any("error", err))
		return nil, false
	}
	return deltaRecord.(vanflow.Record), true
}

// requestFlush schedules a flush of all store contents.
func (m *Manager) requestFlush() {
	m.mu.Lock()
	if m.flushAt == nil {
		at := time.Now().Add(m.FlushDelay)
		m.flushAt = &at
	}
	m.mu.Unlock()
	m.signalWorkAvailable()
}

func (m *Manager) flushDue(now time.Time) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.flushAt != nil && !now.Before(*m.flushAt)
}

// streamFlush sends the contents of every store as a series of batched record
// messages.
func (m *Manager) streamFlush(ctx context.Context, sender session.Sender) error {
	m.mu.Lock()
	m.flushAt = nil
	m.mu.Unlock()

	m.logger.Info("servicing flush", slog.String("source", m.Source.ID))
	for _, stor := range m.Stores {
		entries := stor.List()
		for len(entries) > 0 {
			batch := entries
			if n := m.FlushBatchSize; n > 0 && len(batch) > n {
				batch = batch[:n]
			}
			entries = entries[len(batch):]

			records := make([]vanflow.Record, len(batch))
			for i, entry := range batch {
				records[i] = entry.Record
			}
			msg, err := vanflow.RecordMessage{Records: records}.Encode()
			if err != nil {
				m.logger.Error("skipping flush record message after encoding error",
					slog.Any("error", err),
					slog.Int("record_count", len(records)))
				continue
			}
			if err := sendWithTimeout(ctx, recordSendTimeout, sender, msg); err != nil {
				return err
			}
		}
	}
	return nil
}

func (m *Manager) sendKeepalives(ctx context.Context) {
	beaconInterval := m.BeaconInterval
	if beaconInterval <= 0 {
		beaconInterval = 10 * time.Second
	}
	heartbeatInterval := m.HeartbeatInterval
	if heartbeatInterval <= 0 {
		heartbeatInterval = 2 * time.Second
	}

	beaconMessage := vanflow.BeaconMessage{
		Version:    uint32(m.Source.Version),
		SourceType: m.Source.Type,
		Address:    m.Source.Address,
		Direct:     m.Source.Direct,
		Identity:   m.Source.ID,
	}
	beaconMessage.To = beaconAddress
	heartbeatMessage := vanflow.HeartbeatMessage{
		Version:  uint32(m.Source.Version),
		Identity: m.Source.ID,
	}

	heartbeatAddr := m.Source.Address
	if m.UseAlternateHeartbeatAddress {
		heartbeatAddr = heartbeatAddr + sourceSuffixHeartbeats
	}

	beaconSender := m.container.NewSender(beaconAddress, session.SenderOptions{})
	defer beaconSender.Close(ctx)
	heartbeatSender := m.container.NewSender(heartbeatAddr, session.SenderOptions{})
	defer heartbeatSender.Close(ctx)

	// start with sending beacon
	for {
		if err := sendWithTimeout(ctx, beaconInterval, beaconSender, beaconMessage.Encode()); err != nil {
			m.logger.Error("error sending initial beacon message", slog.Any("error", err))
			if ctx.Err() != nil {
				m.logger.Error("gave up on sending initial beacon message as context has been closed",
					slog.Any("error", err))
				return
			}
			continue
		}
		break
	}

	messageDeliveryTimeout := heartbeatInterval
	if messageDeliveryTimeout > beaconInterval {
		messageDeliveryTimeout = beaconInterval
	}

	beaconTimer := time.NewTicker(beaconInterval)
	heartbeatTimer := time.NewTicker(heartbeatInterval)
	heartbeatTimeouts := 0
	firstHeartbeatSent := false
	for {
		select {
		case <-ctx.Done():
			return
		case <-beaconTimer.C:
			msg := beaconMessage.Encode()
			if err := sendWithTimeout(ctx, messageDeliveryTimeout, beaconSender, msg); err != nil {
				m.logger.Error("error sending event source beacon", slog.Any("error", err))
			}
		case <-heartbeatTimer.C:
			heartbeatMessage.Now = uint64(time.Now().UnixMicro())
			msg := heartbeatMessage.Encode()
			// skupper router will block messages sent multicast without a
			// listener by default. The router needs to register a beacon
			// before heartbeats are unblocked. This doesn't seem entirely
			// reliable so I've opted to ignore the first few heartbeat
			// timeouts before aborting the connection entirely.
			if err := sendWithTimeout(ctx, messageDeliveryTimeout, heartbeatSender, msg); err != nil {
				if errors.Is(err, errSendTimeoutExceeded) && heartbeatTimeouts < 3 && !firstHeartbeatSent {
					heartbeatTimeouts++
					m.logger.Info("initial heartbeat message send timed out")
					continue
				}
				m.logger.Error("error sending event source heartbeat",
					slog.Any("error", err),
					slog.Bool("priorSuccess", firstHeartbeatSent),
					slog.Int("timeouts", heartbeatTimeouts))
			}
			firstHeartbeatSent = true
		}
	}
}

func (m *Manager) listenFlushes(ctx context.Context) {
	flushReceiver := m.container.NewReceiver(m.Source.Direct, session.ReceiverOptions{Credit: 256})
	defer flushReceiver.Close(ctx)
	for {
		msg, err := flushReceiver.Next(ctx)
		if err != nil {
			if errors.Is(err, ctx.Err()) {
				return
			}
			m.logger.Error("flush receive error", slog.Any("error", err))
			continue
		}
		flushReceiver.Accept(ctx, msg)
		m.requestFlush()
	}
}

var errSendTimeoutExceeded = errors.New("send timed out")

func sendWithTimeout(ctx context.Context, timeout time.Duration, sender session.Sender, msg *amqp.Message) error {
	requestCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	err := sender.Send(requestCtx, msg)
	if err != nil {
		if errors.Is(err, requestCtx.Err()) {
			return errSendTimeoutExceeded
		}
	}
	return err
}

func sleep(ctx context.Context, d time.Duration) bool {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

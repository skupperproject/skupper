package eventsource

import (
	"context"
	"errors"
	"log/slog"
	"time"

	amqp "github.com/Azure/go-amqp"
	"github.com/skupperproject/skupper/pkg/vanflow"
	"github.com/skupperproject/skupper/pkg/vanflow/encoding"
	"github.com/skupperproject/skupper/pkg/vanflow/session"
	"github.com/skupperproject/skupper/pkg/vanflow/store"
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

type RecordUpdate struct {
	Prev vanflow.Record
	Curr vanflow.Record
}

type Manager struct {
	ManagerConfig
	container session.Container

	flushQueue  chan struct{}
	changeQueue chan RecordUpdate
	sendQueue   chan vanflow.RecordMessage

	logger *slog.Logger
}

func NewManager(container session.Container, cfg ManagerConfig) *Manager {
	return &Manager{
		container:     container,
		ManagerConfig: cfg,
		flushQueue:    make(chan struct{}, 8),
		changeQueue:   make(chan RecordUpdate, 256),
		sendQueue:     make(chan vanflow.RecordMessage, 256),
		logger: slog.New(slog.Default().Handler()).With(
			slog.String("component", "vanflow.eventsource.manager"),
			slog.String("instance", cfg.Source.ID),
		),
	}
}

func (m *Manager) PublishUpdate(update RecordUpdate) {
	m.changeQueue <- update
}

func (m *Manager) Run(ctx context.Context) {
	go m.listenFlushes(ctx)
	go m.sendKeepalives(ctx)
	go m.sendRecords(ctx)
	m.serve(ctx)
}

func (m *Manager) sendRecords(ctx context.Context) {
	sender := m.container.NewSender(m.Source.Address, session.SenderOptions{})
	defer sender.Close(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case record := <-m.sendQueue:
			msg, err := record.Encode()
			if err != nil {
				m.logger.Error("skipping record message after encoding error:", slog.Any("error", err))
				continue
			}
			if err := sender.Send(ctx, msg); err != nil {
				m.logger.Error("error sending event source record", slog.Any("error", err))
				continue
			}
			m.logger.Info("record message sent", slog.Int("record_count", len(record.Records)))
		}
	}
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

func (m *Manager) serve(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case update := <-m.changeQueue:
			var buffer []RecordUpdate
			if m.UpdateBufferTime > 0 && m.UpdateBatchSize > 1 {
				bufferCtx, cancelBuffer := context.WithTimeout(ctx, m.UpdateBufferTime)
				buffer = nextN(bufferCtx, m.changeQueue, m.UpdateBatchSize-1)
				cancelBuffer()
				if ctx.Err() != nil {
					return
				}
			}
			buffer = append([]RecordUpdate{update}, buffer...)

			var record vanflow.RecordMessage
			record.Records = make([]vanflow.Record, 0, len(buffer))
			for _, update := range buffer {
				delta, changed := m.diffRecord(update)
				if !changed {
					continue
				}
				record.Records = append(record.Records, delta)
			}
			if len(record.Records) == 0 {
				m.logger.Debug("record changes buffered but none were changed", slog.Int("record_count", len(buffer)))
				continue
			}
			select {
			case <-ctx.Done():
				return
			case m.sendQueue <- record:
			}

		case <-m.flushQueue:
			// handle a flush
			if m.FlushDelay > 0 {
				drainCtx, cancelDrain := context.WithTimeout(ctx, m.FlushDelay)
				nextN(drainCtx, m.flushQueue, 2048)
				cancelDrain()
				if ctx.Err() != nil {
					return
				}
			}
			m.logger.Info("servicing flush", slog.String("source", m.Source.ID))
			for _, stor := range m.Stores {
				entries := stor.List()

				for len(entries) > 0 {
					var batch []store.Entry
					batch, entries = entries, entries[:0]
					if len(batch) > m.FlushBatchSize && m.FlushBatchSize > 0 {
						batch, entries = batch[:m.FlushBatchSize], batch[m.FlushBatchSize:]
					}

					var record vanflow.RecordMessage
					record.Records = make([]vanflow.Record, len(batch))
					for i, entry := range batch {
						record.Records[i] = entry.Record
					}
					select {
					case <-ctx.Done():
						return
					case m.sendQueue <- record:
					}
				}
			}
		}
	}
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
		}
		flushReceiver.Accept(ctx, msg)
		select {
		case m.flushQueue <- struct{}{}:
		default: // drop flush if queue is full
		}
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

// pulls next N items from a channel
func nextN[T any](ctx context.Context, c <-chan T, n int) []T {
	var out []T
	if n < 1 {
		return out
	}
	for {
		select {
		case <-ctx.Done():
			return out
		case t := <-c:
			out = append(out, t)
			if len(out) == n {
				return out
			}
		}
	}
}

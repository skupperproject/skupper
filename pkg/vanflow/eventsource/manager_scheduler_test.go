package eventsource

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	amqp "github.com/Azure/go-amqp"
	"github.com/google/go-cmp/cmp"
	"github.com/skupperproject/skupper/pkg/vanflow"
	"github.com/skupperproject/skupper/pkg/vanflow/session"
	"github.com/skupperproject/skupper/pkg/vanflow/store"
)

func TestManagerSquashesDuplicateUpdates(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		const updateDelay = 5 * time.Second
		manager, sender := newSchedulerTestManager(ManagerConfig{
			UpdateBufferTime: updateDelay,
			UpdateBatchSize:  2,
		})
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		go manager.sendRecords(ctx)
		synctest.Wait()

		originalText, latestText := "original", "latest"
		initialSeverity, latestSeverity := uint64(1), uint64(2)
		initial := vanflow.LogRecord{
			BaseRecord:  vanflow.NewBase("record"),
			LogSeverity: &initialSeverity,
			LogText:     &originalText,
		}
		middle := initial
		middle.LogSeverity = &latestSeverity
		manager.PublishUpdate(RecordUpdate{Prev: initial, Curr: middle})
		synctest.Wait()
		assertSendAttempts(t, sender, 0)

		// A second update for the same identity neither fills the batch nor
		// restarts the buffer delay.
		time.Sleep(updateDelay - time.Second)
		final := middle
		final.LogText = &latestText
		manager.PublishUpdate(RecordUpdate{Prev: middle, Curr: final})
		synctest.Wait()
		assertSendAttempts(t, sender, 0)

		time.Sleep(time.Second - time.Millisecond)
		synctest.Wait()
		assertSendAttempts(t, sender, 0)

		time.Sleep(time.Millisecond)
		synctest.Wait()
		messages := sender.recordMessages(t)
		if len(messages) != 1 || len(messages[0].Records) != 1 {
			t.Fatalf("expected one message containing one squashed update, got %+v", messages)
		}
		want := vanflow.LogRecord{
			BaseRecord:  vanflow.NewBase("record"),
			LogSeverity: &latestSeverity,
			LogText:     &latestText,
		}
		if diff := cmp.Diff(want, messages[0].Records[0]); diff != "" {
			t.Errorf("unexpected squashed update (-want +got):\n%s", diff)
		}

		down, up := "down", "up"
		linkDown := vanflow.LinkRecord{
			BaseRecord: vanflow.NewBase("link"),
			Status:     &down,
		}
		linkUp := linkDown
		linkUp.Status = &up
		manager.PublishUpdate(RecordUpdate{Prev: linkDown, Curr: linkUp})
		manager.PublishUpdate(RecordUpdate{Prev: linkUp, Curr: linkDown})
		synctest.Wait()
		assertSendAttempts(t, sender, 1)

		time.Sleep(updateDelay)
		synctest.Wait()
		assertSendAttempts(t, sender, 1)
		if got := managerPendingLen(manager); got != 0 {
			t.Errorf("expected cancelled-out link updates to be drained: got %d pending", got)
		}
	})
}

func TestManagerSchedulesBufferedUpdatesBeforeDelayedFlush(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		const (
			updateDelay = 5 * time.Second
			flushDelay  = 10 * time.Second
		)
		stor := store.NewSyncMapStore(store.SyncMapStoreConfig{})
		stor.Add(managerTestRecord("stored"), store.SourceRef{ID: "source"})

		manager, sender := newSchedulerTestManager(ManagerConfig{
			Stores:           []store.Interface{stor},
			UpdateBufferTime: updateDelay,
			UpdateBatchSize:  2,
			FlushDelay:       flushDelay,
		})
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		go manager.sendRecords(ctx)
		synctest.Wait()

		manager.requestFlush()
		manager.PublishUpdate(RecordUpdate{Curr: managerTestRecord("updated")})
		synctest.Wait()
		assertSendAttempts(t, sender, 0)

		time.Sleep(updateDelay - time.Millisecond)
		synctest.Wait()
		assertSendAttempts(t, sender, 0)

		time.Sleep(time.Millisecond)
		synctest.Wait()
		assertMessageRecordIDs(t, sender, []string{"updated"})

		time.Sleep(flushDelay - updateDelay - time.Millisecond)
		synctest.Wait()
		assertSendAttempts(t, sender, 1)

		time.Sleep(time.Millisecond)
		synctest.Wait()
		assertMessageRecordIDs(t, sender, []string{"updated"}, []string{"stored"})
	})
}

func TestManagerSendsFullUpdateBatchWithoutWaiting(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		manager, sender := newSchedulerTestManager(ManagerConfig{
			UpdateBufferTime: time.Hour,
			UpdateBatchSize:  2,
		})
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		go manager.sendRecords(ctx)
		synctest.Wait()

		manager.PublishUpdate(RecordUpdate{Curr: managerTestRecord("one")})
		synctest.Wait()
		assertSendAttempts(t, sender, 0)

		manager.PublishUpdate(RecordUpdate{Curr: managerTestRecord("two")})
		synctest.Wait()
		assertMessageRecordIDs(t, sender, []string{"one", "two"})
	})
}

func TestManagerFlushesInConfiguredBatchSizes(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		stor := store.NewSyncMapStore(store.SyncMapStoreConfig{})
		for i := range 5 {
			stor.Add(managerTestRecord(fmt.Sprintf("record-%d", i)), store.SourceRef{ID: "source"})
		}
		manager, sender := newSchedulerTestManager(ManagerConfig{
			Stores:         []store.Interface{stor},
			FlushBatchSize: 2,
		})
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		go manager.sendRecords(ctx)
		synctest.Wait()

		manager.requestFlush()
		synctest.Wait()

		messages := sender.recordMessages(t)
		got := make([]int, len(messages))
		for i, message := range messages {
			got[i] = len(message.Records)
		}
		if diff := cmp.Diff([]int{2, 2, 1}, got); diff != "" {
			t.Errorf("unexpected flush batch sizes (-want +got):\n%s", diff)
		}
	})
}

func TestManagerSchedulesKeepalives(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		const (
			heartbeatInterval = 5 * time.Second
			beaconInterval    = 10 * time.Second
		)
		container := newKeepaliveTestContainer()
		manager := NewManager(container, ManagerConfig{
			Source: Info{
				ID:      "source",
				Version: 2,
				Type:    "test-source",
				Address: "records",
				Direct:  "direct",
			},
			HeartbeatInterval:            heartbeatInterval,
			BeaconInterval:               beaconInterval,
			UseAlternateHeartbeatAddress: true,
		})
		beacons := container.sender(beaconAddress)
		heartbeats := container.sender("records" + sourceSuffixHeartbeats)
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		go manager.sendKeepalives(ctx)
		synctest.Wait()

		assertSendAttempts(t, beacons, 1)
		assertSendAttempts(t, heartbeats, 0)
		decodedBeacon := decodeManagerTestMessage(t, beacons.sentMessages()[0])
		beacon, ok := decodedBeacon.(vanflow.BeaconMessage)
		if !ok {
			t.Fatalf("initial keepalive has type %T, want vanflow.BeaconMessage", decodedBeacon)
		}
		wantBeacon := vanflow.BeaconMessage{
			MessageProps: vanflow.MessageProps{To: beaconAddress, Subject: "BEACON"},
			Version:      2,
			SourceType:   "test-source",
			Address:      "records",
			Direct:       "direct",
			Identity:     "source",
		}
		if diff := cmp.Diff(wantBeacon, beacon); diff != "" {
			t.Errorf("unexpected initial beacon (-want +got):\n%s", diff)
		}

		time.Sleep(heartbeatInterval - time.Nanosecond)
		synctest.Wait()
		assertSendAttempts(t, heartbeats, 0)

		time.Sleep(time.Nanosecond)
		synctest.Wait()
		assertSendAttempts(t, heartbeats, 1)
		decodedHeartbeat := decodeManagerTestMessage(t, heartbeats.sentMessages()[0])
		heartbeat, ok := decodedHeartbeat.(vanflow.HeartbeatMessage)
		if !ok {
			t.Fatalf("keepalive has type %T, want vanflow.HeartbeatMessage", decodedHeartbeat)
		}
		if heartbeat.Identity != "source" || heartbeat.Version != 2 || heartbeat.Now == 0 {
			t.Errorf("unexpected heartbeat: %+v", heartbeat)
		}

		time.Sleep(beaconInterval - heartbeatInterval - time.Nanosecond)
		synctest.Wait()
		assertSendAttempts(t, beacons, 1)

		time.Sleep(time.Nanosecond)
		synctest.Wait()
		assertSendAttempts(t, beacons, 2)
		assertSendAttempts(t, heartbeats, 2)
	})
}

func TestManagerRetriesAfterSendTimeout(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		manager, sender := newSchedulerTestManager(ManagerConfig{})
		sender.block()
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		go manager.sendRecords(ctx)
		synctest.Wait()

		manager.PublishUpdate(RecordUpdate{Curr: managerTestRecord("record")})
		synctest.Wait()
		assertSendAttempts(t, sender, 1)

		time.Sleep(recordSendTimeout - time.Nanosecond)
		synctest.Wait()
		assertSendAttempts(t, sender, 1)

		time.Sleep(time.Nanosecond)
		synctest.Wait()
		assertSendAttempts(t, sender, 1)
		if got := managerPendingLen(manager); got != 1 {
			t.Fatalf("expected timed out record to be requeued: got %d pending records", got)
		}

		sender.unblock()
		time.Sleep(recordSendRetryDelay - time.Nanosecond)
		synctest.Wait()
		assertSendAttempts(t, sender, 1)

		time.Sleep(time.Nanosecond)
		synctest.Wait()
		assertSendAttempts(t, sender, 2)
		assertMessageRecordIDs(t, sender, []string{"record"})
		if got := managerPendingLen(manager); got != 0 {
			t.Errorf("expected retry to drain pending records: got %d", got)
		}
	})
}

func TestManagerRetriesLatestFullRecordAfterUncertainSend(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		manager, sender := newSchedulerTestManager(ManagerConfig{})
		sender.block()
		sender.failNext(errors.New("send outcome uncertain"))
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		go manager.sendRecords(ctx)
		synctest.Wait()

		stableText := "unchanged"
		initialSeverity, updatedSeverity := uint64(1), uint64(2)
		initial := vanflow.LogRecord{
			BaseRecord:  vanflow.NewBase("record"),
			LogSeverity: &initialSeverity,
			LogText:     &stableText,
		}
		updated := initial
		updated.LogSeverity = &updatedSeverity

		manager.PublishUpdate(RecordUpdate{Prev: initial, Curr: updated})
		synctest.Wait()
		assertSendAttempts(t, sender, 1)

		// Queue a reversion while the first send is in flight. The sender will
		// report an error even though it records the message as delivered, so
		// the manager cannot presume which state the receiver has.
		manager.PublishUpdate(RecordUpdate{Prev: updated, Curr: initial})
		synctest.Wait()
		sender.unblock()
		synctest.Wait()

		if got := managerPendingLen(manager); got != 1 {
			t.Fatalf("expected latest record to be requeued: got %d pending records", got)
		}

		time.Sleep(recordSendRetryDelay)
		synctest.Wait()
		assertSendAttempts(t, sender, 2)

		messages := sender.recordMessages(t)
		if len(messages) != 2 || len(messages[1].Records) != 1 {
			t.Fatalf("expected an uncertain send and one full-record retry, got %+v", messages)
		}
		if diff := cmp.Diff(initial, messages[1].Records[0]); diff != "" {
			t.Errorf("retry did not contain the complete latest record (-want +got):\n%s", diff)
		}
		if got := managerPendingLen(manager); got != 0 {
			t.Errorf("expected retry to drain pending records: got %d", got)
		}
	})
}

func newSchedulerTestManager(config ManagerConfig) (*Manager, *schedulerTestSender) {
	sender := &schedulerTestSender{}
	container := &schedulerTestContainer{sender: sender}
	config.Source = Info{ID: "source", Address: "records"}
	return NewManager(container, config), sender
}

func managerTestRecord(id string) vanflow.LogRecord {
	text := id
	return vanflow.LogRecord{BaseRecord: vanflow.NewBase(id), LogText: &text}
}

func managerPendingLen(manager *Manager) int {
	manager.mu.Lock()
	defer manager.mu.Unlock()
	return len(manager.pending)
}

func assertSendAttempts(t *testing.T, sender *schedulerTestSender, want int) {
	t.Helper()
	if got := sender.attempts(); got != want {
		t.Errorf("unexpected send attempts: want %d, got %d", want, got)
	}
}

func assertMessageRecordIDs(t *testing.T, sender *schedulerTestSender, want ...[]string) {
	t.Helper()
	messages := sender.recordMessages(t)
	got := make([][]string, len(messages))
	for i, message := range messages {
		for _, record := range message.Records {
			got[i] = append(got[i], record.Identity())
		}
		sort.Strings(got[i])
	}
	for i := range want {
		sort.Strings(want[i])
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("unexpected sent record identities (-want +got):\n%s", diff)
	}
}

func decodeManagerTestMessage(t *testing.T, message *amqp.Message) interface{} {
	t.Helper()
	decoded, err := vanflow.Decode(message)
	if err != nil {
		t.Fatalf("decode sent message: %v", err)
	}
	return decoded
}

type schedulerTestContainer struct {
	sender *schedulerTestSender
}

func (*schedulerTestContainer) Start(context.Context)      {}
func (*schedulerTestContainer) OnSessionError(func(error)) {}
func (*schedulerTestContainer) NewReceiver(string, session.ReceiverOptions) session.Receiver {
	panic("unexpected receiver")
}
func (c *schedulerTestContainer) NewSender(string, session.SenderOptions) session.Sender {
	return c.sender
}

type schedulerTestSender struct {
	mu       sync.Mutex
	blocked  chan struct{}
	nextErr  error
	tries    int
	received []*amqp.Message
}

func (s *schedulerTestSender) block() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.blocked = make(chan struct{})
}

func (s *schedulerTestSender) unblock() {
	s.mu.Lock()
	defer s.mu.Unlock()
	close(s.blocked)
}

func (s *schedulerTestSender) failNext(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nextErr = err
}

func (s *schedulerTestSender) attempts() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.tries
}

func (s *schedulerTestSender) sentMessages() []*amqp.Message {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]*amqp.Message(nil), s.received...)
}

func (s *schedulerTestSender) recordMessages(t *testing.T) []vanflow.RecordMessage {
	t.Helper()
	sent := s.sentMessages()
	messages := make([]vanflow.RecordMessage, 0, len(sent))
	for _, message := range sent {
		decoded := decodeManagerTestMessage(t, message)
		recordMessage, ok := decoded.(vanflow.RecordMessage)
		if !ok {
			t.Fatalf("sent message has type %T, want vanflow.RecordMessage", decoded)
		}
		messages = append(messages, recordMessage)
	}
	return messages
}

func (s *schedulerTestSender) Send(ctx context.Context, message *amqp.Message) error {
	s.mu.Lock()
	s.tries++
	blocked := s.blocked
	nextErr := s.nextErr
	s.nextErr = nil
	s.mu.Unlock()

	if blocked != nil {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-blocked:
		}
	}

	s.mu.Lock()
	s.received = append(s.received, message)
	s.mu.Unlock()
	return nextErr
}

func (*schedulerTestSender) Close(context.Context) error { return nil }

type keepaliveTestContainer struct {
	mu      sync.Mutex
	senders map[string]*schedulerTestSender
}

func newKeepaliveTestContainer() *keepaliveTestContainer {
	return &keepaliveTestContainer{senders: make(map[string]*schedulerTestSender)}
}

func (*keepaliveTestContainer) Start(context.Context)      {}
func (*keepaliveTestContainer) OnSessionError(func(error)) {}
func (*keepaliveTestContainer) NewReceiver(string, session.ReceiverOptions) session.Receiver {
	panic("unexpected receiver")
}
func (c *keepaliveTestContainer) NewSender(address string, _ session.SenderOptions) session.Sender {
	return c.sender(address)
}

func (c *keepaliveTestContainer) sender(address string) *schedulerTestSender {
	c.mu.Lock()
	defer c.mu.Unlock()
	sender, ok := c.senders[address]
	if !ok {
		sender = &schedulerTestSender{}
		c.senders[address] = sender
	}
	return sender
}

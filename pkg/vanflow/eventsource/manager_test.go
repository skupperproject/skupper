package eventsource

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/skupperproject/skupper/pkg/vanflow"
	"github.com/skupperproject/skupper/pkg/vanflow/store"
	"gotest.tools/v3/poll"
)

func TestManagerClient(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping flaky test: #1738")
	}
	tstCtx, tstCancel := context.WithCancel(context.Background())
	defer tstCancel()

	sourceID := uniqueSuffix("test-event-source-manager")
	factory, rtt := requireContainers(t)
	ctr := factory.Create()
	ctr.Start(tstCtx)
	ctrClient := factory.Create()
	ctrClient.Start(tstCtx)

	discovery := NewDiscovery(ctrClient, DiscoveryOptions{})
	go discovery.Run(tstCtx, DiscoveryHandlers{})

	sourceRef := store.SourceRef{ID: sourceID}
	source := Info{ID: sourceID, Type: "test-event-source", Address: mcsfe(sourceID), Direct: sfe(sourceID)}
	logStor := store.NewSyncMapStore(store.SyncMapStoreConfig{})
	listenerStor := store.NewSyncMapStore(store.SyncMapStoreConfig{})
	for i := 0; i < 64; i++ {
		logStor.Add(vanflow.LogRecord{BaseRecord: vanflow.NewBase(fmt.Sprintf("log-%d", i))}, sourceRef)
		listenerStor.Add(vanflow.ListenerRecord{BaseRecord: vanflow.NewBase(fmt.Sprintf("listener-%d", i))}, sourceRef)
	}
	manager := NewManager(ctr, ManagerConfig{
		Source:            source,
		Stores:            []store.Interface{logStor, listenerStor},
		HeartbeatInterval: rtt * 10,
		BeaconInterval:    rtt * 50,
	})
	go manager.Run(tstCtx)

	clientStor := store.NewSyncMapStore(store.SyncMapStoreConfig{})
	logsOnly := RecordStoreRouter{
		Source: sourceRef,
		Stores: RecordStoreMap{vanflow.LogRecord{}.GetTypeMeta().String(): clientStor},
	}

	client := NewClient(ctrClient, ClientOptions{Source: source})
	client.OnRecord(logsOnly.Route)
	client.Listen(tstCtx, FromSourceAddress())
	defer client.Close()
	t.Run("wait to flush", func(t *testing.T) {
		flushCtx, cancel := context.WithTimeout(tstCtx, rtt*100)
		defer cancel()
		if err := FlushOnFirstMessage(flushCtx, client); err != nil {
			t.Errorf("expected manager to be discovered by the client: %s", err)
		}
	})

	t.Run("will synchronize records", func(t *testing.T) {
		poll.WaitOn(t, func(t poll.LogT) poll.Result {
			actual := clientStor.List()
			expected := logStor.List()
			if !cmp.Equal(actual, expected, ignoreLastUpdateAndOrder...) {
				return poll.Continue("waiting for all records to be synced: %s", cmp.Diff(actual, expected, ignoreLastUpdateAndOrder...))
			}
			return poll.Success()
		}, poll.WithDelay(rtt), poll.WithTimeout(300*rtt))
	})

	t.Run("can publish updates", func(t *testing.T) {
		prevEntry, _ := logStor.Get("log-1")
		updated := prevEntry.Record.(vanflow.LogRecord)
		txt := "updated txt"
		updated.LogText = &txt
		manager.PublishUpdate(RecordUpdate{Prev: prevEntry.Record, Curr: updated})
		poll.WaitOn(t, func(t poll.LogT) poll.Result {
			actual, _ := clientStor.Get("log-1")
			expected := updated
			if !cmp.Equal(actual.Record, expected) {
				return poll.Continue("waiting for record '1' to be updated: %s", cmp.Diff(actual.Record, expected))
			}
			return poll.Success()
		}, poll.WithDelay(rtt), poll.WithTimeout(100*rtt))
	})

}

var ignoreLastUpdateAndOrder = []cmp.Option{
	cmpopts.IgnoreFields(store.Metadata{}, "LastUpdate"),
	cmpopts.SortSlices(func(a, b store.Entry) bool {
		return strings.Compare(a.Record.Identity(), b.Record.Identity()) < 0
	}),
}

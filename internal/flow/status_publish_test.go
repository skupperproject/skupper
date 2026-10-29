package flow

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync/atomic"
	"testing"
	"time"

	"github.com/skupperproject/skupper/internal/network"
	"github.com/skupperproject/skupper/pkg/vanflow"
	"github.com/skupperproject/skupper/pkg/vanflow/session"
	"github.com/skupperproject/skupper/pkg/vanflow/store"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/poll"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// flakyStatusClient fails the first N Update calls, then succeeds.
// Mimics a transient apiserver error that RetryOnConflict does not retry.
type flakyStatusClient struct {
	cm            *corev1.ConfigMap
	failRemaining atomic.Int32
	updateCalls   atomic.Int32
}

func (f *flakyStatusClient) Logger() *slog.Logger {
	return slog.Default().With(slog.String("component", "kube.flow.statusSync"))
}

func (f *flakyStatusClient) Get(ctx context.Context) (*corev1.ConfigMap, error) {
	return f.cm.DeepCopy(), nil
}

func (f *flakyStatusClient) Update(ctx context.Context, latest *corev1.ConfigMap) error {
	n := f.updateCalls.Add(1)
	if f.failRemaining.Add(-1) >= 0 {
		return fmt.Errorf("simulated apiserver timeout on update #%d", n)
	}
	f.cm = latest.DeepCopy()
	return nil
}

func newFlakyStatusSync(t *testing.T, failFirst int32) (*StatusSync, *flakyStatusClient) {
	t.Helper()
	client := &flakyStatusClient{
		cm: &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Name: "skupper-network-status", Namespace: "test"},
			Data:       map[string]string{},
		},
	}
	client.failRemaining.Store(failFirst)

	ss := NewStatusSync(session.NewMockContainerFactory(), nil, client, "skupper-network-status")
	ss.ctx = context.Background()
	ss.records.Add(
		vanflow.SiteRecord{BaseRecord: vanflow.NewBase("site-a")},
		store.SourceRef{ID: "local"},
	)
	return ss, client
}

// TestPublishFailureKeepsPrev ensures a failed publish does not advance prev,
// so a later call with an unchanged network retries the write (#2608).
func TestPublishFailureKeepsPrev(t *testing.T) {
	ss, client := newFlakyStatusSync(t, 1)

	prev := network.NetworkStatusInfo{}
	returned, err := ss.buildAndPublish(prev)
	assert.ErrorContains(t, err, "simulated apiserver timeout")
	assert.Assert(t, client.cm.Data["NetworkStatus"] == "")
	assert.DeepEqual(t, returned, prev) // must not advance prev on error

	next, err := ss.buildAndPublish(returned)
	assert.NilError(t, err)
	assert.Equal(t, client.updateCalls.Load(), int32(2))
	assert.Assert(t, client.cm.Data["NetworkStatus"] != "")

	var written network.NetworkStatusInfo
	assert.NilError(t, json.Unmarshal([]byte(client.cm.Data["NetworkStatus"]), &written))
	assert.Equal(t, len(written.SiteStatus), 1)
	assert.Equal(t, written.SiteStatus[0].Site.Identity, "site-a")
	assert.Equal(t, len(next.SiteStatus), 1)
}

// TestPublishFailureIsRetriedByRun checks that Run schedules a retry after a
// failed publish even when network records do not change again.
func TestPublishFailureIsRetriedByRun(t *testing.T) {
	ss, client := newFlakyStatusSync(t, 1)
	ss.limit = 0
	ss.publishRetry = time.Millisecond

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go ss.Run(ctx)

	// Kick the loop once; the first Update fails, then publishRetry should
	// fire and succeed without another record change.
	ss.notify()

	poll.WaitOn(t, func(log poll.LogT) poll.Result {
		if client.cm.Data["NetworkStatus"] == "" {
			return poll.Continue("waiting for retry to write NetworkStatus (updates=%d)", client.updateCalls.Load())
		}
		return poll.Success()
	}, poll.WithDelay(time.Millisecond*5), poll.WithTimeout(time.Second*2))

	assert.Assert(t, client.updateCalls.Load() >= 2)
}

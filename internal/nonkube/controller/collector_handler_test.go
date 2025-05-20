package controller

import (
	"testing"
	"time"

	"gotest.tools/v3/assert"
)

func TestCollectorHandler(t *testing.T) {
	started := make(chan struct{})
	done := make(chan struct{})
	stopCh := make(chan struct{})

	ch := NewCollectorLifecycleHandler("sample-namespace")
	fakeStart := func() {
		close(started)
		go func() {
			<-ch.ctx.Done()
			close(done)
		}()
	}
	ch.startMethod = fakeStart

	t.Run("start-collector", func(t *testing.T) {
		ch.Start(stopCh)
		timeout := time.NewTicker(time.Second)
		select {
		case <-timeout.C:
			assert.Assert(t, false, "Timeout waiting for collector to start")
		case <-started:
			t.Logf("fake collector started")
		}
	})

	t.Run("stop-collector", func(t *testing.T) {
		ch.Stop()
		timeout := time.NewTicker(time.Second)
		select {
		case <-timeout.C:
			assert.Assert(t, false, "Timeout waiting for collector to stop")
		case <-done:
			t.Logf("fake collector stopped")
		}
	})

	t.Run("restart-collector", func(t *testing.T) {
		started = make(chan struct{})
		done = make(chan struct{})
		ch.Start(stopCh)
		timeout := time.NewTicker(time.Second)
		select {
		case <-timeout.C:
			assert.Assert(t, false, "Timeout waiting for collector to restart")
		case <-started:
			t.Logf("fake collector restarted")
		}
	})

	t.Run("parent-stopped", func(t *testing.T) {
		close(stopCh)
		timeout := time.NewTicker(time.Second)
		select {
		case <-timeout.C:
			assert.Assert(t, false, "Timeout waiting for collector to stop")
		case <-done:
			t.Logf("fake collector stopped")
		}
	})
}

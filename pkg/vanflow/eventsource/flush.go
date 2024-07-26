package eventsource

import (
	"context"
	"sync"

	"github.com/skupperproject/skupper/pkg/vanflow"
)

// FlushOnFirstMessage adds handlers to a client to listen for the first
// heartbeat or record message received from the source and then calls
// client.SendFlush().
func FlushOnFirstMessage(ctx context.Context, client *Client) error {
	done := make(chan struct{})
	awaiter := messageAwaiter{done: done}
	client.OnRecord(awaiter.Record)
	client.OnHeartbeat(awaiter.Heartbeat)
	select {
	case <-done:
		return client.SendFlush(ctx)
	case <-ctx.Done():
		return ctx.Err()
	}
}

type messageAwaiter struct {
	done chan struct{}
	once sync.Once
}

func (f *messageAwaiter) Record(vanflow.RecordMessage) {
	f.once.Do(func() { close(f.done) })
}

func (f *messageAwaiter) Heartbeat(vanflow.HeartbeatMessage) {
	f.once.Do(func() { close(f.done) })
}

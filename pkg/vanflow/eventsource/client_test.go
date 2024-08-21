package eventsource

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"testing"
	"time"

	amqp "github.com/Azure/go-amqp"
	"github.com/skupperproject/skupper/pkg/vanflow"
	"github.com/skupperproject/skupper/pkg/vanflow/session"
	"gotest.tools/assert"
)

func TestClient(t *testing.T) {
	t.Parallel()
	tstCtx, tstCancel := context.WithCancel(context.Background())
	defer tstCancel()
	factory, rtt := requireContainers(t)
	ctr, tstCtr := factory.Create(), factory.Create()
	ctr.Start(tstCtx)
	tstCtr.Start(tstCtx)

	clientID := uniqueSuffix("test")
	client := NewClient(ctr, ClientOptions{Source: Info{
		ID:      clientID,
		Address: mcsfe(clientID),
	}})
	heartbeats := make(chan vanflow.HeartbeatMessage, 8)
	records := make(chan vanflow.RecordMessage, 8)
	client.OnHeartbeat(func(m vanflow.HeartbeatMessage) { heartbeats <- m })
	client.OnRecord(func(m vanflow.RecordMessage) { records <- m })

	sender := tstCtr.NewSender(mcsfe(clientID), session.SenderOptions{})
	assert.Check(t, client.Listen(tstCtx, FromSourceAddress()))

	heartbeat := vanflow.HeartbeatMessage{
		Identity: clientID, Version: 1, Now: 22,
		MessageProps: vanflow.MessageProps{
			To:      mcsfe(clientID),
			Subject: "HEARTBEAT",
		},
	}
	initRetryTimer := time.After(250 * rtt)
	for i := 0; i < 10; i++ {
		sender.Send(tstCtx, heartbeat.Encode())
		select {
		case actual := <-heartbeats:
			initRetryTimer = nil
			assert.DeepEqual(t, actual, heartbeat)
		case <-initRetryTimer:
			t.Log("retrying heartbeat")
			initRetryTimer = nil
		}
		heartbeat.Now++
	}
	record := vanflow.RecordMessage{
		MessageProps: vanflow.MessageProps{
			To:      mcsfe(clientID),
			Subject: "RECORD",
		},
	}
	for i := 0; i < 10; i++ {
		msg, err := record.Encode()
		assert.Check(t, err)
		sender.Send(tstCtx, msg)
		actual := <-records
		assert.DeepEqual(t, actual, record)
		name := fmt.Sprintf("router-%d", i)
		record.Records = append(record.Records, vanflow.RouterRecord{BaseRecord: vanflow.BaseRecord{ID: name}})
	}

	closed := make(chan struct{})
	go func() {
		defer close(closed)
		client.Close()
	}()
	select {
	case <-closed: //okay
	case <-time.After(500 * rtt):
		t.Error("expected client.Close() to promptly return")
	}

	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("client.Close() should be safe to call multiple times but paniced: %v", r)
			}
		}()
		client.Close()
	}()

	msg, err := record.Encode()
	assert.Check(t, err)
	sender.Send(tstCtx, msg)
	select {
	case <-time.After(100 * time.Millisecond): //okay
	case <-records:
		t.Error("expected client to stop handling records after close called")
	}

}

func TestClientFlush(t *testing.T) {
	factory, rtt := requireContainers(t)
	ctr, tstCtr := factory.Create(), factory.Create()
	ctr.Start(context.Background())
	tstCtr.Start(context.Background())

	testSuffix := uniqueSuffix("")
	testCases := []struct {
		ClientName string
		When       func(t *testing.T, ctx context.Context, client *Client)
		Expect     func(t *testing.T, ctx context.Context, flushMsg <-chan *amqp.Message)
	}{
		{
			ClientName: "flush" + testSuffix,
			When: func(t *testing.T, ctx context.Context, client *Client) {
				assert.Check(t, client.SendFlush(ctx))
			},
			Expect: func(t *testing.T, ctx context.Context, flushMsg <-chan *amqp.Message) {
				select {
				case <-time.After(rtt * 5):
					t.Errorf("expected flush message")
				case msg := <-flushMsg:
					assert.Equal(t, *msg.Properties.Subject, "FLUSH")
				}
			},
		}, {
			ClientName: "noop",
			When: func(t *testing.T, ctx context.Context, client *Client) {
			},
			Expect: func(t *testing.T, ctx context.Context, flushMsg <-chan *amqp.Message) {
				select {
				case <-time.After(rtt * 2):
				case msg := <-flushMsg:
					t.Errorf("unexpected flush message: %v", msg)
				}
			},
		}, {
			ClientName: "flush-on-first-message" + testSuffix,
			When: func(t *testing.T, ctx context.Context, client *Client) {
				sender := tstCtr.NewSender(mcsfe("flush-on-first-message")+testSuffix, session.SenderOptions{})
				go sendHeartbeatMessagesTo(t, ctx, sender)
				assert.Check(t, client.Listen(ctx, FromSourceAddress()))
				assert.Check(t, FlushOnFirstMessage(ctx, client))
			},
			Expect: func(t *testing.T, ctx context.Context, flushMsg <-chan *amqp.Message) {
				select {
				case <-time.After(rtt * 5):
					t.Errorf("expected flush message")
				case msg := <-flushMsg:
					assert.Equal(t, *msg.Properties.Subject, "FLUSH")
				}
			},
		}, {
			ClientName: "flush-on-first-message-timeout" + testSuffix,
			When: func(t *testing.T, ctx context.Context, client *Client) {
				flushCtx, cancel := context.WithTimeout(ctx, rtt*10)
				defer cancel()
				err := FlushOnFirstMessage(flushCtx, client)
				assert.Assert(t, err != nil)
			},
			Expect: func(t *testing.T, ctx context.Context, flushMsg <-chan *amqp.Message) {
				select {
				case <-time.After(rtt * 2):
				case msg := <-flushMsg:
					t.Errorf("unexpected flush message: %v", msg)
				}
			},
		},
	}
	for _, _tc := range testCases {
		tc := _tc
		t.Run(tc.ClientName, func(t *testing.T) {
			t.Parallel()
			tstCtx, tstCancel := context.WithCancel(context.Background())
			defer tstCancel()
			client := NewClient(ctr, ClientOptions{Source: Info{
				ID:      tc.ClientName,
				Address: mcsfe(tc.ClientName),
				Direct:  sfe(tc.ClientName),
			}})
			receiver := tstCtr.NewReceiver(sfe(tc.ClientName), session.ReceiverOptions{})

			flush := make(chan *amqp.Message)
			go func() {
				for {
					msg, err := receiver.Next(tstCtx)
					if tstCtx.Err() != nil {
						return
					}
					assert.Check(t, err)
					receiver.Accept(tstCtx, msg)
					flush <- msg
				}
			}()
			tc.When(t, tstCtx, client)
			tc.Expect(t, tstCtx, flush)
		})
	}

}

func sendHeartbeatMessagesTo(t *testing.T, ctx context.Context, sender session.Sender) {
	t.Helper()
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Millisecond):
			err := sender.Send(ctx, vanflow.HeartbeatMessage{}.Encode())
			if ctx.Err() != nil {
				return
			}
			assert.Check(t, err)
		}
	}
}

func requireContainers(t *testing.T) (session.ContainerFactory, time.Duration) {
	t.Helper()
	errNotSet := errors.New("SKUPPER_ROUTER_AMQP_ADDRESS environment variable not present")
	factory, err := func() (session.ContainerFactory, error) {
		routerAddress := os.Getenv("SKUPPER_ROUTER_AMQP_ADDRESS")
		if testing.Short() || routerAddress == "" {
			return nil, errNotSet
		}
		setupCtx, cancel := context.WithTimeout(context.Background(), time.Second*3)
		defer cancel()
		setupCtr := session.NewContainer(routerAddress, session.ContainerConfig{})
		setupCtr.Start(setupCtx)
		pingAddr := uniqueSuffix("ping")
		ping := setupCtr.NewSender(pingAddr, session.SenderOptions{})
		pong := setupCtr.NewReceiver(pingAddr, session.ReceiverOptions{})

		sendDone := make(chan error)
		go func() {
			defer close(sendDone)
			sendDone <- ping.Send(setupCtx, amqp.NewMessage([]byte("PING")))
		}()
		msg, err := pong.Next(setupCtx)
		if err != nil {
			return nil, fmt.Errorf("qdr receive failed: %s: %s", routerAddress, err)
		}
		pong.Accept(setupCtx, msg)
		if err := <-sendDone; err != nil {
			return nil, fmt.Errorf("qdr send failed: %s: %s", routerAddress, err)
		}
		ping.Close(setupCtx)
		pong.Close(setupCtx)

		factory := session.NewContainerFactory(routerAddress,
			session.ContainerConfig{
				ContainerID: uniqueSuffix("eventsourcetest"),
			})
		return factory, nil
	}()
	if err != nil {
		if testing.Short() || err == errNotSet {
			return session.NewMockContainerFactory(), time.Millisecond * 2
		}
		t.Fatalf("failed to setup tests: %v", err)
	}

	return factory, time.Millisecond * 100
}

func uniqueSuffix(prefix string) string {
	var salt [8]byte
	io.ReadFull(rand.Reader, salt[:])
	out := bytes.NewBuffer([]byte(prefix))
	out.WriteByte('-')
	hex.NewEncoder(out).Write(salt[:])
	return out.String()
}

func mcsfe(id string) string {
	return fmt.Sprintf("mc/sfe.%s", id)
}
func sfe(id string) string {
	return fmt.Sprintf("sfe.%s", id)
}

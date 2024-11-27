package vanflow

import (
	"testing"
	"time"

	amqp "github.com/Azure/go-amqp"
	"gotest.tools/v3/assert"
)

func TestDecode(t *testing.T) {
	testCases := []struct {
		Name  string
		In    *amqp.Message
		Error string
		Out   any
	}{
		{
			Name: "beacon",
			In:   BeaconMessage{}.Encode(),
			Out: BeaconMessage{MessageProps: MessageProps{
				Subject: "BEACON",
				To:      "mc/sfe.all",
			}},
		}, {
			Name: "heartbeat",
			In:   HeartbeatMessage{Identity: "a"}.Encode(),
			Out: HeartbeatMessage{Identity: "a", MessageProps: MessageProps{
				Subject: "HEARTBEAT",
				To:      "mc/sfe.a",
			}},
		}, {
			Name: "flush",
			In:   FlushMessage{}.Encode(),
			Out:  FlushMessage{MessageProps: MessageProps{Subject: "FLUSH"}},
		}, {
			Name:  "nil",
			In:    nil,
			Error: "cannot decode message",
		}, {
			Name:  "nil props",
			In:    new(amqp.Message),
			Error: "cannot decode message",
		}, {
			Name:  "no subject",
			In:    &amqp.Message{Properties: &amqp.MessageProperties{}},
			Error: "cannot decode message",
		}, {
			Name:  "unknown subject",
			In:    &amqp.Message{Properties: &amqp.MessageProperties{Subject: ptrTo("A")}},
			Error: "cannot decode message",
		}, {
			Name: "emtpy record",
			In:   func() *amqp.Message { m, _ := RecordMessage{}.Encode(); return m }(),
			Out:  RecordMessage{MessageProps: MessageProps{Subject: "RECORD"}},
		}, {
			Name: "record",
			In: func() *amqp.Message {
				r := RecordMessage{
					Records: []Record{
						SiteRecord{
							BaseRecord: NewBase("1", time.UnixMicro(222)),
						},
						RouterRecord{BaseRecord: BaseRecord{ID: "1"}},
						LinkRecord{
							BaseRecord: BaseRecord{ID: "1"},
							LinkCost:   ptrTo[uint64](1),
						},
					},
				}
				m, err := r.Encode()
				if err != nil {
					t.Fatal(err)
				}
				return m
			}(),
			Out: RecordMessage{
				MessageProps: MessageProps{Subject: "RECORD"},
				Records: []Record{
					SiteRecord{
						BaseRecord: NewBase("1", time.UnixMicro(222)),
					},
					RouterRecord{BaseRecord: BaseRecord{ID: "1"}},
					LinkRecord{
						BaseRecord: BaseRecord{ID: "1"},
						LinkCost:   ptrTo[uint64](1),
					},
				},
			},
		}, {
			Name: "bad record val",
			In: func() *amqp.Message {
				m, _ := RecordMessage{}.Encode()
				m.Value = []byte("testing")
				return m
			}(),
			Error: "unexpected type for message Value",
		},
	}
	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			actual, err := Decode(tc.In)
			if tc.Error != "" {
				assert.ErrorContains(t, err, tc.Error)
				return
			}
			assert.DeepEqual(t, actual, tc.Out)
		})
	}
}
func TestBeaconMessage(t *testing.T) {
	original := BeaconMessage{
		Version:    99,
		SourceType: "source",
		Address:    "address",
		Direct:     "direct",
		Identity:   "id",
	}
	msg := original.Encode()
	assert.Equal(t, *msg.Properties.Subject, "BEACON")
	assert.Equal(t, *msg.Properties.To, "mc/sfe.all")
	dup := DecodeBeacon(msg)
	dup.MessageProps = MessageProps{}
	assert.DeepEqual(t, original, dup)
}

func TestHeartbeatMessage(t *testing.T) {
	original := HeartbeatMessage{
		MessageProps: MessageProps{
			To:      "ignored",
			Subject: "ignored",
			ReplyTo: "ignored",
		},
		Identity: "myid",
		Version:  1,
		Now:      9999,
	}
	msgPropsExpected := MessageProps{
		To:      "mc/sfe.myid",
		Subject: "HEARTBEAT",
		ReplyTo: "",
	}
	msg := original.Encode()
	dup := DecodeHeartbeat(msg)
	original.MessageProps = msgPropsExpected
	assert.DeepEqual(t, original, dup)
}

func TestFlushMessage(t *testing.T) {
	original := FlushMessage{
		MessageProps{
			To:      "flushaddr",
			Subject: "ignored",
			ReplyTo: "flushreply",
		},
	}
	msgPropsExpected := MessageProps{
		To:      "flushaddr",
		Subject: "FLUSH",
		ReplyTo: "flushreply",
	}
	msg := original.Encode()
	dupe := DecodeFlush(msg)
	original.MessageProps = msgPropsExpected
	assert.DeepEqual(t, original, dupe)
}

func TestRecordMessage(t *testing.T) {
	original := RecordMessage{
		MessageProps: MessageProps{
			To:      "recordaddr",
			Subject: "ignored",
			ReplyTo: "ignored",
		},
	}
	msgPropsExpected := MessageProps{
		Subject: "RECORD",
		To:      "recordaddr",
	}
	msg, err := original.Encode()
	assert.Check(t, err)
	dupe, err := DecodeRecord(msg)
	assert.Check(t, err)
	original.MessageProps = msgPropsExpected
	assert.DeepEqual(t, original, dupe)
}

func ptrTo[T any](obj T) *T { return &obj }

package flow

import (
	"log"
	"sync"

	amqp "github.com/interconnectedcloud/go-amqp"
	"github.com/skupperproject/skupper/pkg/event"
	"github.com/skupperproject/skupper/pkg/messaging"
)

const (
	BeaconAddress string = "mc/sfe.all"
	RecordPrefix  string = "mc/sfe."
	DirectPrefix  string = "sfe."
)

type base struct {
	stopOnce          sync.Once
	done              chan struct{}
	connectionFactory messaging.ConnectionFactory
	incoming          chan []interface{}
	outgoing          chan interface{}
	address           string
}

func (c *base) stop() {
	c.stopOnce.Do(func() { close(c.done) })
}

type sender struct {
	base
	sendSettled bool
}

func (c *sender) start() {
	go c.send()
}

func newSender(connectionFactory messaging.ConnectionFactory, address string, sendSettled bool, update chan interface{}) *sender {
	return &sender{
		base: base{
			done:              make(chan struct{}),
			connectionFactory: connectionFactory,
			outgoing:          update,
			address:           address,
		},
		sendSettled: sendSettled,
	}
}

func (c *sender) send() {
	for {
		select {
		case <-c.done:
			log.Println("COLLECTOR: Flow process stopped sending")
			return
		default:
			if err := c._send(); err != nil {
				log.Printf("COLLECTOR: Error sending out updates %s", err.Error())
			}
		}

	}
}

func (c *sender) _send() error {
	client, err := c.connectionFactory.Connect()
	if err != nil {
		return err
	}
	log.Printf("COLLECTOR: Connection for sender %s to %s established\n", c.address, c.connectionFactory.Url())
	defer client.Close()

	sender, err := client.Sender(c.address)
	if err != nil {
		return err
	}

	defer sender.Close()

	var request *amqp.Message
	for {
		select {
		case <-c.done:
			return nil
		case update := <-c.outgoing:
			if beacon, ok := update.(*BeaconRecord); ok {
				msg, err := encodeBeacon(beacon)
				if err != nil {
					event.Recordf(FlowControllerEvent, "Failed to encode message for flow controller: %s", err.Error())
				} else {
					request = msg
				}
			}
			if heartbeat, ok := update.(*HeartbeatRecord); ok {
				msg, err := encodeHeartbeat(heartbeat)
				if err != nil {
					event.Recordf(FlowControllerEvent, "Failed to encode message for flow controller: %s", err.Error())
				} else {
					request = msg
				}
			}
			if fr, ok := update.(*FlushRecord); ok {
				msg, err := encodeFlush(fr)
				if err != nil {
					event.Recordf(FlowControllerEvent, "Failed to encode message for flow controller: %s", err.Error())
				} else {
					request = msg
				}
			}
			if site, ok := update.(*SiteRecord); ok {
				msg, err := encodeSite(site)
				if err != nil {
					event.Recordf(FlowControllerEvent, "Failed to encode message for flow controller: %s", err.Error())
				} else {
					request = msg
				}
			}
			if process, ok := update.(*ProcessRecord); ok {
				msg, err := encodeProcess(process)
				if err != nil {
					event.Recordf(FlowControllerEvent, "Failed to encode message for flow controller: %s", err.Error())
				} else {
					request = msg
				}
			}
			if host, ok := update.(*HostRecord); ok {
				msg, err := encodeHost(host)
				if err != nil {
					event.Recordf(FlowControllerEvent, "Failed to encode message for flow controller: %s", err.Error())
				} else {
					request = msg
				}
			}
		}
		if request != nil {
			request.SendSettled = c.sendSettled
			err = sender.Send(request)
			if err != nil {
				return err
			}
			request = nil
		}
	}
}

type receiver struct {
	base
}

func newReceiver(connectionFactory messaging.ConnectionFactory, address string, updates chan []interface{}) *receiver {
	return &receiver{
		base: base{
			done:              make(chan struct{}),
			connectionFactory: connectionFactory,
			incoming:          updates,
			address:           address,
		},
	}
}

func (r *receiver) start() {
	go r.receive()
}

func (r *receiver) receive() {
	for {
		select {
		case <-r.done:
			return
		default:
			if err := r._receive(); err != nil {
				log.Printf("COLLECTOR: Receiver %s %s\n", r.address, err.Error())
			}
		}
	}
}

func (r *receiver) _receive() error {
	client, err := r.connectionFactory.Connect()
	if err != nil {
		return err
	}
	log.Printf("COLLECTOR: Connection for receiver %s to %s established\n", r.address, r.connectionFactory.Url())
	defer client.Close()

	receiver, err := client.Receiver(r.address, 250)
	if err != nil {
		return err
	}
	defer receiver.Close()

	for {
		select {
		case <-r.done:
			return nil
		default:
		}
		msg, err := receiver.Receive()
		if err != nil {
			return err
		}
		receiver.Accept(msg)
		results := decode(msg)
		r.incoming <- results
	}
}

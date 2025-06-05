package flow

import (
	"log"
	"time"

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
	connectionFactory messaging.ConnectionFactory
	closed            bool
	client            messaging.Connection
	incoming          chan []interface{}
	outgoing          chan interface{}
	address           string
}

func (c *base) stop() {
	c.closed = true
	if c.client != nil {
		c.client.Close()
	}
}

type sender struct {
	base
	ticker  *time.Ticker
	request *amqp.Message
}

func (c *sender) start() {
	c.closed = false
	go c.send()
}

func newSender(connectionFactory messaging.ConnectionFactory, address string, update chan interface{}) *sender {
	return &sender{
		base: base{
			connectionFactory: connectionFactory,
			outgoing:          update,
			address:           address,
		},
	}
}

func (c *sender) send() {
	c.ticker = time.NewTicker(5 * time.Second)
	defer c.ticker.Stop()
	for !c.closed {
		err := c._send()
		if err != nil {
			log.Printf("COLLECTOR: Error sending out updates %s", err.Error())
		}
	}
	log.Println("COLLECTOR: Flow process stopped sending")
}

func (c *sender) _send() error {
	client, err := c.connectionFactory.Connect()
	if err != nil {
		return err
	}
	c.client = client
	defer client.Close()

	sender, err := client.Sender(c.address)
	if err != nil {
		return err
	}

	defer sender.Close()

	for {
		select {
		case update := <-c.outgoing:
			if beacon, ok := update.(*BeaconRecord); ok {
				msg, err := encodeBeacon(beacon)
				if err != nil {
					event.Recordf(FlowControllerEvent, "Failed to encode message for flow controller: %s", err.Error())
				} else {
					c.request = msg
				}
			}
			if heartbeat, ok := update.(*HeartbeatRecord); ok {
				msg, err := encodeHeartbeat(heartbeat)
				if err != nil {
					event.Recordf(FlowControllerEvent, "Failed to encode message for flow controller: %s", err.Error())
				} else {
					c.request = msg
				}
			}
			if fr, ok := update.(*FlushRecord); ok {
				msg, err := encodeFlush(fr)
				if err != nil {
					event.Recordf(FlowControllerEvent, "Failed to encode message for flow controller: %s", err.Error())
				} else {
					c.request = msg
				}
			}
			if site, ok := update.(*SiteRecord); ok {
				msg, err := encodeSite(site)
				if err != nil {
					event.Recordf(FlowControllerEvent, "Failed to encode message for flow controller: %s", err.Error())
				} else {
					c.request = msg
				}
			}
			if process, ok := update.(*ProcessRecord); ok {
				msg, err := encodeProcess(process)
				if err != nil {
					event.Recordf(FlowControllerEvent, "Failed to encode message for flow controller: %s", err.Error())
				} else {
					c.request = msg
				}
			}
			if host, ok := update.(*HostRecord); ok {
				msg, err := encodeHost(host)
				if err != nil {
					event.Recordf(FlowControllerEvent, "Failed to encode message for flow controller: %s", err.Error())
				} else {
					c.request = msg
				}
			}
		case <-c.ticker.C:
		}
		if c.request != nil {
			err = sender.Send(c.request)
			if err != nil {
				return err
			} else {
				c.request = nil
			}
		}
	}
}

type receiver struct {
	base
}

func newReceiver(connectionFactory messaging.ConnectionFactory, address string, updates chan []interface{}) *receiver {
	return &receiver{
		base: base{
			connectionFactory: connectionFactory,
			incoming:          updates,
			address:           address,
		},
	}
}

func (r *receiver) start() {
	r.closed = false
	go r.receive()
}

func (r *receiver) receive() {
	for !r.closed {
		err := r._receive()
		if err != nil {
			log.Println("COLLECTOR: Error receiving message ", err.Error())
		}
	}
}

func (r *receiver) _receive() error {
	client, err := r.connectionFactory.Connect()
	if err != nil {
		return err
	}
	r.client = client
	defer client.Close()

	receiver, err := client.Receiver(r.address, 10)
	if err != nil {
		return err
	}
	defer receiver.Close()

	for {
		msg, err := receiver.Receive()
		if err != nil {
			log.Println("COLLECTOR: Receiver error ", err.Error())
			return err
		}
		receiver.Accept(msg)
		results := decode(msg)
		r.incoming <- results
	}
}

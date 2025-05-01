package service_sync

import (
	"fmt"
	"time"

	amqp "github.com/interconnectedcloud/go-amqp"

	"github.com/skupperproject/skupper/pkg/event"
	"github.com/skupperproject/skupper/pkg/messaging"
)

const (
	ServiceSyncAddress string = "mc/$skupper-service-sync"
)

type base struct {
	connectionFactory messaging.ConnectionFactory
	updates           chan ServiceUpdate
	closed            bool
	client            messaging.Connection
	eventHandler      event.EventHandlerInterface
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

func newSender(connectionFactory messaging.ConnectionFactory, updates chan ServiceUpdate, eventHandler event.EventHandlerInterface) *sender {
	return &sender{
		base: base{
			connectionFactory: connectionFactory,
			updates:           updates,
			eventHandler:      eventHandler,
		},
	}
}

func (c *sender) send() {
	c.ticker = time.NewTicker(5 * time.Second)
	defer c.ticker.Stop()
	for !c.closed {
		err := c._send()
		if err != nil {
			event.Recordf(ServiceSyncEvent, "Error sending out updates: %s", err.Error())
		}
	}
	event.Record(ServiceSyncEvent, "Service sync stopped sending")
}

func (c *sender) _send() error {
	client, err := c.connectionFactory.Connect()
	if err != nil {
		return err
	}
	c.client = client
	message := fmt.Sprintf("Service sync sender connection to %s established", c.connectionFactory.Url())
	c.eventHandler.RecordNormalEvent(ServiceSyncEvent, message)
	defer client.Close()

	sender, err := client.Sender(ServiceSyncAddress)
	if err != nil {
		return err
	}

	defer sender.Close()

	for {
		select {
		case update := <-c.updates:
			msg, err := encode(&update)
			if err != nil {
				event.Recordf(ServiceSyncEvent, "Failed to encode message for service sync: %s", err.Error())
			} else {
				c.request = msg
			}

		case <-c.ticker.C:
		}
		if c.request != nil {
			err = sender.Send(c.request)
			if err != nil {
				return err
			}
		}
	}
}

type receiver struct {
	base
}

func newReceiver(connectionFactory messaging.ConnectionFactory, updates chan ServiceUpdate, eventHandler event.EventHandlerInterface) *receiver {
	return &receiver{
		base: base{
			connectionFactory: connectionFactory,
			updates:           updates,
			eventHandler:      eventHandler,
		},
	}
}

func (c *receiver) start() {
	c.closed = false
	go c.receive()
}

func (c *receiver) receive() {
	for !c.closed {
		err := c._receive()
		if err != nil {
			event.Recordf(ServiceSyncEvent, "Error receiving updates: %s", err.Error())
		}
	}
	event.Record(ServiceSyncEvent, "Service sync stopped receiving")
}

func (c *receiver) _receive() error {
	client, err := c.connectionFactory.Connect()
	if err != nil {
		return err
	}
	c.client = client
	message := fmt.Sprintf("Service sync receiver connection to %s established", c.connectionFactory.Url())
	c.eventHandler.RecordNormalEvent(ServiceSyncEvent, message)
	defer client.Close()

	receiver, err := client.Receiver(ServiceSyncAddress, 10)
	if err != nil {
		return err
	}

	defer receiver.Close()

	for {
		msg, err := receiver.Receive()
		if err != nil {
			return err
		}
		receiver.Accept(msg)
		update, err := decode(msg)
		if err != nil {
			event.Record(ServiceSyncEvent, err.Error())
		} else {
			c.updates <- update
		}
	}
}

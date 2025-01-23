package messaging

import (
	amqp "github.com/interconnectedcloud/go-amqp"
)

type ConnectionFactory interface {
	Connect() (Connection, error)
	Url() string
}

type Connection interface {
	Sender(address string) (Sender, error)
	Receiver(address string, credit uint32) (Receiver, error)
	Close()
}

type Sender interface {
	Send(msg *amqp.Message) error
	Close() error
}

type Receiver interface {
	Receive() (*amqp.Message, error)
	Accept(*amqp.Message) error
	Close() error
}

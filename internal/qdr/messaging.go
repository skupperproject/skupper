package qdr

import (
	"context"
	"crypto/tls"
	"time"

	amqp "github.com/interconnectedcloud/go-amqp"

	"github.com/skupperproject/skupper/internal/messaging"
)

type TlsConfigRetriever interface {
	GetTlsConfig() (*tls.Config, error)
}

type ConnectionFactory struct {
	url            string
	config         TlsConfigRetriever
	connectTimeout time.Duration
}

func (f *ConnectionFactory) Connect() (messaging.Connection, error) {
	if f.config == nil {
		return dial(f.url, amqp.ConnMaxFrameSize(4294967295), amqp.ConnConnectTimeout(f.connectTimeout))
	} else {
		tlsConfig, err := f.config.GetTlsConfig()
		if err != nil {
			return nil, err
		}
		return dial(f.url, amqp.ConnSASLExternal(), amqp.ConnMaxFrameSize(4294967295), amqp.ConnConnectTimeout(f.connectTimeout), amqp.ConnTLSConfig(tlsConfig))
	}
}

func dial(addr string, opts ...amqp.ConnOption) (*AmqpConnection, error) {
	client, err := amqp.Dial(addr, opts...)
	if err != nil {
		return nil, err
	}
	session, err := client.NewSession()
	if err != nil {
		client.Close()
		return nil, err
	}
	return &AmqpConnection{client: client, session: session}, nil
}

func (f *ConnectionFactory) Url() string {
	return f.url
}

func NewConnectionFactory(url string, config TlsConfigRetriever) *ConnectionFactory {
	return &ConnectionFactory{
		url:    url,
		config: config,
	}
}

type AmqpConnection struct {
	client  *amqp.Client
	session *amqp.Session
}

type AmqpSender struct {
	connection *AmqpConnection
	sender     *amqp.Sender
}

type AmqpReceiver struct {
	connection *AmqpConnection
	receiver   *amqp.Receiver
}

func (c *AmqpConnection) Close() {
	c.client.Close()
}

func (c *AmqpConnection) Sender(address string) (messaging.Sender, error) {
	sender, err := c.session.NewSender(amqp.LinkTargetAddress(address))
	if err != nil {
		return nil, err
	}
	return &AmqpSender{connection: c, sender: sender}, nil
}

func (c *AmqpConnection) Receiver(address string, credit uint32) (messaging.Receiver, error) {
	receiver, err := c.session.NewReceiver(
		amqp.LinkSourceAddress(address),
		amqp.LinkCredit(credit),
	)
	if err != nil {
		return nil, err
	}
	return &AmqpReceiver{connection: c, receiver: receiver}, nil
}

func (s *AmqpSender) Send(msg *amqp.Message) error {
	return s.sender.Send(context.Background(), msg)
}

func (s *AmqpSender) Close() error {
	return s.sender.Close(context.Background())
}

func (s *AmqpReceiver) Receive() (*amqp.Message, error) {
	return s.receiver.Receive(context.Background())
}

func (s *AmqpReceiver) Accept(msg *amqp.Message) error {
	return msg.Accept()
}

func (s *AmqpReceiver) Close() error {
	return s.receiver.Close(context.Background())
}

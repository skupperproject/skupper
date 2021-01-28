package qdr

import (
	"context"
	"fmt"

	amqp "github.com/interconnectedcloud/go-amqp"
)

const (
	VersionProperty string = "version"
)

type Request struct {
	Address    string
	Type       string
	Version    string
	Properties map[string]interface{}
}

type Response struct {
	Type       string
	Version    string
	Properties map[string]interface{}
	Body       string
}

type RequestResponse interface {
	Request(request *Request) (*Response, error)
}

type RequestServer struct {
	pool    *AgentPool
	address string
	handler RequestResponse
}

func NewRequestServer(address string, handler RequestResponse, pool *AgentPool) *RequestServer {
	return &RequestServer{
		pool, address, handler,
	}
}

func (s *RequestServer) Run(ctx context.Context) error {
	agent, err := s.pool.Get()
	if err != nil {
		return fmt.Errorf("Could not get management agent: %s", err)
	}
	defer agent.Close()

	receiver, err := agent.newReceiver(s.address)
	if err != nil {
		return fmt.Errorf("Could not open receiver for %s: %s", s.address, err)
	}
	for {
		err = s.serve(ctx, receiver, agent.anonymous)
		if err != nil {
			return fmt.Errorf("Error handling request for %s: %s", s.address, err)
		}
	}
}

func (s *RequestServer) serve(ctx context.Context, receiver *amqp.Receiver, sender *amqp.Sender) error {
	for {
		requestMsg, err := receiver.Receive(ctx)
		if err != nil {
			return fmt.Errorf("Failed reading request from %s: %s", s.address, err.Error())
		}
		requestMsg.Accept()

		request := Request{
			Address:    requestMsg.Properties.To,
			Type:       requestMsg.Properties.Subject,
			Properties: map[string]interface{}{},
		}
		for k, v := range requestMsg.ApplicationProperties {
			if k == VersionProperty {
				if version, ok := v.(string); ok {
					request.Version = version
				}
			} else {
				request.Properties[k] = v
			}
		}

		response, err := s.handler.Request(&request)
		if err != nil {
			//TODO: send back error response to avoid
			//requesting client having to time out
			return err
		}

		responseMsg := amqp.Message{
			Properties: &amqp.MessageProperties{
				To:      requestMsg.Properties.ReplyTo,
				Subject: response.Type,
			},
			ApplicationProperties: map[string]interface{}{},
			Value:                 response.Body,
		}
		correlationId, ok := AsUint64(requestMsg.Properties.CorrelationID)
		if !ok {
			responseMsg.Properties.CorrelationID = correlationId
		}
		for k, v := range response.Properties {
			responseMsg.ApplicationProperties[k] = v
		}
		responseMsg.ApplicationProperties[VersionProperty] = response.Version

		err = sender.Send(ctx, &responseMsg)
		if err != nil {
			return fmt.Errorf("Could not send response: %s", err)
		}
	}
}

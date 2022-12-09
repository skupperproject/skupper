package main

import (
	"crypto/tls"
	"log"

	utilruntime "k8s.io/apimachinery/pkg/util/runtime"

	"github.com/interconnectedcloud/go-amqp"
	"github.com/skupperproject/skupper/pkg/flow"
	"github.com/skupperproject/skupper/pkg/qdr"
)

type Controller struct {
	origin        string
	amqpClient    *amqp.Client
	amqpSession   *amqp.Session
	flowCollector *flow.FlowCollector
}

func NewController(origin string, scheme string, host string, port string, tlsConfig *tls.Config) (*Controller, error) {

	controller := &Controller{
		origin:        origin,
		flowCollector: flow.NewFlowCollector(origin, qdr.NewConnectionFactory(scheme+"://"+host+":"+port, tlsConfig)),
	}

	return controller, nil
}

func (c *Controller) Run(stopCh <-chan struct{}) error {
	defer utilruntime.HandleCrash()

	log.Println("Starting the Skupper flow collector")

	c.flowCollector.Start(stopCh)

	<-stopCh
	log.Println("Shutting down the Skupper flow collector")

	return nil
}

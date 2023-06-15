package main

import (
	"crypto/tls"
	"log"
	"time"

	utilruntime "k8s.io/apimachinery/pkg/util/runtime"

	"github.com/interconnectedcloud/go-amqp"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/skupperproject/skupper/pkg/flow"
	"github.com/skupperproject/skupper/pkg/qdr"
)

type Controller struct {
	origin        string
	amqpClient    *amqp.Client
	amqpSession   *amqp.Session
	FlowCollector *flow.FlowCollector
}

func NewController(origin string, reg prometheus.Registerer, scheme string, host string, port string, tlsConfig *tls.Config, recordTtl time.Duration) (*Controller, error) {

	controller := &Controller{
		origin:        origin,
		FlowCollector: flow.NewFlowCollector(origin, reg, qdr.NewConnectionFactory(scheme+"://"+host+":"+port, tlsConfig), recordTtl),
	}

	return controller, nil
}

func (c *Controller) Run(stopCh <-chan struct{}) error {
	defer utilruntime.HandleCrash()

	log.Println("COLLECTOR: Starting the Skupper flow collector")

	c.FlowCollector.Start(stopCh)

	<-stopCh
	log.Println("COLLECTOR: Shutting down the Skupper flow collector")

	return nil
}

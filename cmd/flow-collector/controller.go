package main

import (
	"log"
	"time"

	utilruntime "k8s.io/apimachinery/pkg/util/runtime"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/skupperproject/skupper/pkg/certs"
	"github.com/skupperproject/skupper/pkg/flow"
	"github.com/skupperproject/skupper/pkg/qdr"
)

type Controller struct {
	FlowCollector *flow.FlowCollector
}

func NewController(origin string, reg prometheus.Registerer, scheme string, host string, port string, tlsConfig *certs.TlsConfigRetriever, recordTtl time.Duration) (*Controller, error) {

	controller := &Controller{
		FlowCollector: flow.NewFlowCollector(flow.FlowCollectorSpec{
			Mode:              flow.RecordMetrics,
			Origin:            origin,
			PromReg:           reg,
			ConnectionFactory: qdr.NewConnectionFactory(scheme+"://"+host+":"+port, tlsConfig),
			FlowRecordTtl:     recordTtl,
		}),
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

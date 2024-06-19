package main

import (
	"log/slog"
	"time"

	utilruntime "k8s.io/apimachinery/pkg/util/runtime"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/skupperproject/skupper/pkg/flow"
	"github.com/skupperproject/skupper/pkg/qdr"
)

type Controller struct {
	FlowCollector *flow.FlowCollector
}

func NewController(origin string, reg prometheus.Registerer, scheme string, host string, port string, tlsConfig qdr.TlsConfigRetriever, recordTtl time.Duration) (*Controller, error) {

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

	slog.Info("Starting Network Console flow collector")

	c.FlowCollector.Start(stopCh)

	<-stopCh
	slog.Info("Network Console flow collector shutting down")

	return nil
}

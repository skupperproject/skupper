package flow

import (
	"log"
	"os"
	"strings"
	"time"

	"github.com/skupperproject/skupper/pkg/config"
	"github.com/skupperproject/skupper/pkg/messaging"
)

const (
	FlowControllerEvent string = "FlowControllerEvent"
)

type FlowController struct {
	origin            string
	creationTime      uint64
	connectionFactory messaging.ConnectionFactory
	beaconOutgoing    chan interface{}
	recordOutgoing    chan interface{}
	flushIncoming     chan []interface{}
	processOutgoing   chan *ProcessRecord
	processRecords    map[string]*ProcessRecord
	hostRecords       map[string]*HostRecord
	hostOutgoing      chan *HostRecord
	startTime         int64
}

func NewFlowController(origin string, creationTime uint64, connectionFactory messaging.ConnectionFactory) *FlowController {
	fc := &FlowController{
		origin:            origin,
		creationTime:      creationTime,
		connectionFactory: connectionFactory,
		beaconOutgoing:    make(chan interface{}),
		recordOutgoing:    make(chan interface{}),
		flushIncoming:     make(chan []interface{}),
		processOutgoing:   make(chan *ProcessRecord),
		processRecords:    make(map[string]*ProcessRecord),
		hostRecords:       make(map[string]*HostRecord),
		hostOutgoing:      make(chan *HostRecord),
		startTime:         time.Now().Unix(),
	}
	return fc
}

func UpdateProcess(c *FlowController, deleted bool, name string, process *ProcessRecord) error {

	// wait for update where host is assigned
	if !deleted && process != nil && process.SourceHost != nil {
		process.RecType = recordNames[Process]
		if _, ok := c.processRecords[process.Identity]; !ok {
			c.processRecords[process.Identity] = process
		}
		c.processOutgoing <- process
	} else {
		// deleted key may have ns prefix, check parts len
		match := name
		parts := strings.Split(name, "/")
		if len(parts) > 1 {
			match = parts[1]
		}
		t := time.Now()
		for id, record := range c.processRecords {
			if *record.Name == match {
				process = record
				process.EndTime = uint64(t.UnixNano()) / uint64(time.Microsecond)
				delete(c.processRecords, id)
				c.processOutgoing <- process
				break
			}
		}
	}

	return nil
}

func UpdateHost(c *FlowController, deleted bool, name string, host *HostRecord) error {
	if !deleted && host != nil {
		host.RecType = recordNames[Host]
		if _, ok := c.hostRecords[host.Identity]; !ok {
			c.hostRecords[host.Identity] = host
		}
	}
	if host != nil {
		c.hostOutgoing <- host
	}

	return nil
}

func (c *FlowController) updates(stopCh <-chan struct{}) {
	tickerAge := time.NewTicker(10 * time.Minute)
	defer tickerAge.Stop()

	beaconTimer := time.NewTicker(10 * time.Second)
	defer beaconTimer.Stop()

	heartbeatTimer := time.NewTicker(2 * time.Second)
	defer heartbeatTimer.Stop()

	identity := c.origin

	beacon := &BeaconRecord{
		Version:    1,
		SourceType: "CONTROLLER",
		Address:    RecordPrefix + c.origin,
		Direct:     "sfe." + c.origin,
		Identity:   identity,
	}

	heartbeat := &HeartbeatRecord{
		Version:  1,
		Identity: os.Getenv("SKUPPER_SITE_ID"),
		Source:   "sfe." + c.origin,
	}

	name := os.Getenv("SKUPPER_SITE_NAME")
	nameSpace := os.Getenv("SKUPPER_NAMESPACE")
	platform := string(config.GetPlatform())
	site := &SiteRecord{
		Base: Base{
			RecType:   recordNames[Site],
			Identity:  os.Getenv("SKUPPER_SITE_ID"),
			StartTime: c.creationTime,
		},
		Name:      &name,
		NameSpace: &nameSpace,
		Platform:  &platform,
	}

	c.beaconOutgoing <- beacon
	heartbeat.Now = uint64(time.Now().UnixNano()) / uint64(time.Microsecond)
	c.recordOutgoing <- heartbeat
	c.recordOutgoing <- site

	for {
		select {
		case <-beaconTimer.C:
			c.beaconOutgoing <- beacon
		case <-heartbeatTimer.C:
			heartbeat.Now = uint64(time.Now().UnixNano()) / uint64(time.Microsecond)
			c.recordOutgoing <- heartbeat
		case process := <-c.processOutgoing:
			c.recordOutgoing <- process
		case host := <-c.hostOutgoing:
			c.recordOutgoing <- host
		case flushUpdates := <-c.flushIncoming:
			for _, flushUpdate := range flushUpdates {
				_, ok := flushUpdate.(FlushRecord)
				if !ok {
					log.Println("Unable to convert interface to flush")
				}
			}
			c.recordOutgoing <- site
			for _, process := range c.processRecords {
				c.recordOutgoing <- process
			}
			for _, host := range c.hostRecords {
				c.recordOutgoing <- host
			}
		case <-tickerAge.C:
		case <-stopCh:
			return

		}
	}
}

func (c *FlowController) Start(stopCh <-chan struct{}) {
	go c.run(stopCh)
}

func (c *FlowController) run(stopCh <-chan struct{}) {
	beaconSender := newSender(c.connectionFactory, BeaconAddress, c.beaconOutgoing)
	recordSender := newSender(c.connectionFactory, RecordPrefix+c.origin, c.recordOutgoing)
	flushReceiver := newReceiver(c.connectionFactory, DirectPrefix+c.origin, c.flushIncoming)

	beaconSender.start()
	recordSender.start()
	flushReceiver.start()

	go c.updates(stopCh)
	<-stopCh

	beaconSender.stop()
	recordSender.stop()
	flushReceiver.stop()
}

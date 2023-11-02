package flow

import (
	"log"
	"os"
	"strings"
	"time"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/client"
	"github.com/skupperproject/skupper/pkg/config"
	"github.com/skupperproject/skupper/pkg/messaging"
)

const (
	FlowControllerEvent string = "FlowControllerEvent"
)

type FlowController struct {
	origin            string
	creationTime      uint64
	version           string
	connectionFactory messaging.ConnectionFactory
	beaconOutgoing    chan interface{}
	heartbeatOutgoing chan interface{}
	recordOutgoing    chan interface{}
	flushIncoming     chan []interface{}
	processOutgoing   chan *ProcessRecord
	processRecords    map[string]*ProcessRecord
	hostRecords       map[string]*HostRecord
	hostOutgoing      chan *HostRecord
	startTime         int64
}

func NewFlowController(origin string, version string, creationTime uint64, connectionFactory messaging.ConnectionFactory) *FlowController {
	fc := &FlowController{
		origin:            origin,
		creationTime:      creationTime,
		version:           version,
		connectionFactory: connectionFactory,
		beaconOutgoing:    make(chan interface{}, 10),
		heartbeatOutgoing: make(chan interface{}, 10),
		recordOutgoing:    make(chan interface{}, 10),
		flushIncoming:     make(chan []interface{}, 10),
		processOutgoing:   make(chan *ProcessRecord, 10),
		processRecords:    make(map[string]*ProcessRecord),
		hostRecords:       make(map[string]*HostRecord),
		hostOutgoing:      make(chan *HostRecord, 10),
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
		c.hostOutgoing <- host
	} else {
		if existing, ok := c.hostRecords[host.Identity]; ok {
			existing.EndTime = uint64(time.Now().UnixNano()) / uint64(time.Microsecond)
		}
		delete(c.hostRecords, host.Identity)
	}

	return nil
}

func (c *FlowController) updateBeaconAndHeartbeats(stopCh <-chan struct{}) {
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

	c.beaconOutgoing <- beacon
	heartbeat.Now = uint64(time.Now().UnixNano()) / uint64(time.Microsecond)
	c.heartbeatOutgoing <- heartbeat

	for {
		select {
		case <-beaconTimer.C:
			c.beaconOutgoing <- beacon
		case <-heartbeatTimer.C:
			heartbeat.Now = uint64(time.Now().UnixNano()) / uint64(time.Microsecond)
			c.heartbeatOutgoing <- heartbeat
		case <-tickerAge.C:
		case <-stopCh:
			return

		}
	}
}

func (c *FlowController) updateRecords(stopCh <-chan struct{}) {
	tickerAge := time.NewTicker(10 * time.Minute)
	defer tickerAge.Stop()

	name := os.Getenv("SKUPPER_SITE_NAME")
	nameSpace := os.Getenv("SKUPPER_NAMESPACE")
	policy := Disabled
	var platformStr string
	platform := config.GetPlatform()
	if platform == "" || platform == types.PlatformKubernetes {
		platformStr = string(types.PlatformKubernetes)
		cli, err := client.NewClient(nameSpace, "", "")
		if err == nil {
			cpv := client.NewClusterPolicyValidator(cli)
			if cpv.Enabled() {
				policy = Enabled
			}
		}
	} else if platform == types.PlatformPodman {
		platformStr = string(types.PlatformPodman)
	}
	site := &SiteRecord{
		Base: Base{
			RecType:   recordNames[Site],
			Identity:  os.Getenv("SKUPPER_SITE_ID"),
			StartTime: c.creationTime,
		},
		Name:      &name,
		NameSpace: &nameSpace,
		Platform:  &platformStr,
		Version:   &c.version,
		Policy:    &policy,
	}

	c.recordOutgoing <- site

	for {
		select {
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
	beaconSender := newSender(c.connectionFactory, BeaconAddress, true, c.beaconOutgoing)
	heartbeatSender := newSender(c.connectionFactory, RecordPrefix+c.origin+".heartbeats", true, c.heartbeatOutgoing)
	recordSender := newSender(c.connectionFactory, RecordPrefix+c.origin, false, c.recordOutgoing)
	flushReceiver := newReceiver(c.connectionFactory, DirectPrefix+c.origin, c.flushIncoming)

	beaconSender.start()
	heartbeatSender.start()
	recordSender.start()
	flushReceiver.start()

	go c.updateBeaconAndHeartbeats(stopCh)
	go c.updateRecords(stopCh)
	<-stopCh

	beaconSender.stop()
	heartbeatSender.start()
	recordSender.stop()
	flushReceiver.stop()
}

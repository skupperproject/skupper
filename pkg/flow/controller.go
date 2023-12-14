package flow

import (
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/pkg/config"
	"github.com/skupperproject/skupper/pkg/messaging"
)

const (
	FlowControllerEvent string = "FlowControllerEvent"
)

type FlowController struct {
	origin               string
	connectionFactory    messaging.ConnectionFactory
	beaconOutgoing       chan interface{}
	heartbeatOutgoing    chan interface{}
	recordOutgoing       chan interface{}
	flushIncoming        chan []interface{}
	processOutgoing      chan *ProcessRecord
	processRecords       map[string]*ProcessRecord
	hostRecords          map[string]*HostRecord
	hostOutgoing         chan *HostRecord
	siteRecordController siteRecordController
	startTime            int64
}

type PolicyEvaluator interface {
	Enabled() bool
}

const WithPolicyDisabled = policyEnabledConst(false)

func NewFlowController(origin string, version string, creationTime uint64, connectionFactory messaging.ConnectionFactory, policyEvaluator PolicyEvaluator) *FlowController {
	fc := &FlowController{
		origin:               origin,
		connectionFactory:    connectionFactory,
		beaconOutgoing:       make(chan interface{}, 10),
		heartbeatOutgoing:    make(chan interface{}, 10),
		recordOutgoing:       make(chan interface{}, 10),
		flushIncoming:        make(chan []interface{}, 10),
		processOutgoing:      make(chan *ProcessRecord, 10),
		processRecords:       make(map[string]*ProcessRecord),
		hostRecords:          make(map[string]*HostRecord),
		hostOutgoing:         make(chan *HostRecord, 10),
		siteRecordController: newSiteRecordController(creationTime, version, policyEvaluator),
		startTime:            time.Now().Unix(),
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

func (c *FlowController) updateBeacon(stopCh <-chan struct{}) {
	tickerAge := time.NewTicker(10 * time.Minute)
	defer tickerAge.Stop()

	beaconTimer := time.NewTicker(10 * time.Second)
	defer beaconTimer.Stop()

	identity := c.origin

	beacon := &BeaconRecord{
		Version:    1,
		SourceType: "CONTROLLER",
		Address:    RecordPrefix + c.origin,
		Direct:     "sfe." + c.origin,
		Identity:   identity,
	}

	c.beaconOutgoing <- beacon

	for {
		select {
		case <-beaconTimer.C:
			c.beaconOutgoing <- beacon
		case <-tickerAge.C:
		case <-stopCh:
			return

		}
	}
}

func (c *FlowController) updateHeartbeats(stopCh <-chan struct{}) {
	tickerAge := time.NewTicker(10 * time.Minute)
	defer tickerAge.Stop()

	heartbeatTimer := time.NewTicker(2 * time.Second)
	defer heartbeatTimer.Stop()

	heartbeat := &HeartbeatRecord{
		Version:  1,
		Identity: os.Getenv("SKUPPER_SITE_ID"),
		Source:   "sfe." + c.origin,
	}

	heartbeat.Now = uint64(time.Now().UnixNano()) / uint64(time.Microsecond)
	c.heartbeatOutgoing <- heartbeat

	for {
		select {
		case <-heartbeatTimer.C:
			heartbeat.Now = uint64(time.Now().UnixNano()) / uint64(time.Microsecond)
			c.heartbeatOutgoing <- heartbeat
		case <-tickerAge.C:
		case <-stopCh:
			return

		}
	}
}

func (c *FlowController) updateRecords(stopCh <-chan struct{}, siteRecordsIncoming <-chan *SiteRecord) {
	tickerAge := time.NewTicker(10 * time.Minute)
	defer tickerAge.Stop()
	for {
		select {
		case process := <-c.processOutgoing:
			c.recordOutgoing <- process
		case site, ok := <-siteRecordsIncoming:
			if !ok {
				continue
			}
			c.recordOutgoing <- site
		case host := <-c.hostOutgoing:
			c.recordOutgoing <- host
		case flushUpdates := <-c.flushIncoming:
			for _, flushUpdate := range flushUpdates {
				_, ok := flushUpdate.(FlushRecord)
				if !ok {
					log.Println("Unable to convert interface to flush")
				}
			}
			c.recordOutgoing <- c.siteRecordController.Record()
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

	go c.updateBeacon(stopCh)
	go c.updateHeartbeats(stopCh)
	go c.updateRecords(stopCh, c.siteRecordController.Start(stopCh))
	<-stopCh

	beaconSender.stop()
	heartbeatSender.stop()
	recordSender.stop()
	flushReceiver.stop()
}

type siteRecordController struct {
	mu            sync.Mutex
	Identity      string
	CreatedAt     uint64
	Version       string
	Name          string
	Namespace     string
	Platform      string
	PolicyEnabled bool

	policyEvaluator PolicyEvaluator
	pollInterval    time.Duration
}

func newSiteRecordController(createdAt uint64, version string, policyEvaluator PolicyEvaluator) siteRecordController {
	var policy bool
	var platformStr string
	platform := config.GetPlatform()
	if platform == "" || platform == types.PlatformKubernetes {
		platformStr = string(types.PlatformKubernetes)
		policy = policyEvaluator.Enabled()
	} else if platform == types.PlatformPodman {
		platformStr = string(types.PlatformPodman)
	}
	return siteRecordController{
		Identity:        os.Getenv("SKUPPER_SITE_ID"),
		CreatedAt:       createdAt,
		Version:         version,
		Name:            os.Getenv("SKUPPER_SITE_NAME"),
		Namespace:       os.Getenv("SKUPPER_NAMESPACE"),
		Platform:        platformStr,
		PolicyEnabled:   policy,
		policyEvaluator: policyEvaluator,
	}
}

func (c *siteRecordController) Start(stopCh <-chan struct{}) <-chan *SiteRecord {
	updates := make(chan *SiteRecord, 1)

	go func() {
		updates <- c.Record()

		if c.Platform != string(types.PlatformKubernetes) {
			return
		}
		// watch for changes to  policy enabled
		pollInterval := c.pollInterval
		if pollInterval <= 0 {
			pollInterval = time.Second * 30
		}
		ticker := time.NewTicker(pollInterval)
		defer ticker.Stop()
		for {
			select {
			case <-stopCh:
				return
			case <-ticker.C:
				if update := c.updatePolicy(); update != nil {
					updates <- update
				}
			}
		}
	}()
	return updates
}

func (c *siteRecordController) updatePolicy() *SiteRecord {
	c.mu.Lock()
	defer c.mu.Unlock()
	enabled := c.policyEvaluator.Enabled()
	if enabled == c.PolicyEnabled {
		return nil
	}
	c.PolicyEnabled = enabled
	policy := Disabled
	if enabled {
		policy = Enabled
	}
	return &SiteRecord{
		Base: Base{
			RecType:  recordNames[Site],
			Identity: c.Identity,
		},
		Policy: &policy,
	}

}

func (c *siteRecordController) Record() *SiteRecord {
	c.mu.Lock()
	defer c.mu.Unlock()
	policy := Disabled
	if c.PolicyEnabled {
		policy = Enabled
	}
	return &SiteRecord{
		Base: Base{
			RecType:   recordNames[Site],
			Identity:  c.Identity,
			StartTime: c.CreatedAt,
		},
		Name:      &c.Name,
		NameSpace: &c.Namespace,
		Platform:  &c.Platform,
		Version:   &c.Version,
		Policy:    &policy,
	}
}

type policyEnabledConst bool

func (c policyEnabledConst) Enabled() bool { return bool(c) }

package flow

import (
	"fmt"
	"reflect"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/skupperproject/skupper/pkg/messaging"
	"gotest.tools/assert"
)

func TestUpdates(t *testing.T) {
	scenarios := []struct {
		name       string
		subject    string
		to         string
		properties map[string]string
		stype      reflect.Type
		fields     map[int]interface{}
	}{
		{
			name:    "router-beacon",
			subject: "BEACON",
			to:      "mc/sfe.all",
			stype:   reflect.TypeOf(BeaconRecord{}),
			properties: map[string]string{
				"Version":    "1",
				"SourceType": "ROUTER",
				"Address":    "sfe.gnjdr:0",
			},
		},
		{
			name:    "controller-beacon",
			subject: "BEACON",
			to:      "mc/sfe.all",
			stype:   reflect.TypeOf(BeaconRecord{}),
			properties: map[string]string{
				"Version":    "1",
				"SourceType": "CONTROLLER",
				"Address":    "sfe.gnjdr:0",
			},
		},
	}
	for _, s := range scenarios {
		switch s.stype {
		case reflect.TypeOf(BeaconRecord{}):
			scenarioBeacon := &BeaconRecord{}
			version, _ := strconv.Atoi(s.properties["Version"])
			scenarioBeacon.Version = uint32(version)
			scenarioBeacon.SourceType = s.properties["SourceType"]
			scenarioBeacon.Address = s.properties["Address"]

			msg, err := encodeBeacon(scenarioBeacon)
			assert.Assert(t, err)
			assert.Equal(t, msg.Properties.Subject, s.subject)
		}
	}
}

func TestRecordUpdates(t *testing.T) {
	factory := messaging.NewMockConnectionFactory(t, "mockamqp://local")
	fc := NewFlowCollector(FlowCollectorSpec{
		Mode:              RecordMetrics,
		Origin:            "origin",
		PromReg:           prometheus.NewRegistry(),
		ConnectionFactory: factory,
		FlowRecordTtl:     time.Minute * 5,
	})

	stopCh := make(chan struct{})
	fc.Start(stopCh)
	factory.Broker.AwaitReceivers(BeaconAddress, 1)

	conn, err := factory.Connect()
	assert.Assert(t, err)
	beaconSender, err := conn.Sender(BeaconAddress)
	assert.Assert(t, err)
	flushReceiver, err := conn.Receiver("sfe.1234", 32)
	assert.Assert(t, err)

	beacon := BeaconRecord{
		Version:    1,
		SourceType: "ROUTER",
		Address:    RecordPrefix + "1234",
		Direct:     "sfe.1234",
		Identity:   "1234",
	}
	msgBeacon, err := encodeBeacon(&beacon)
	assert.Assert(t, err)
	assert.Assert(t, beaconSender.Send(msgBeacon))
	assert.Assert(t, beaconSender.Send(msgBeacon))

	heartbeat := HeartbeatRecord{
		Version:  1,
		Identity: "1234",
		Source:   "sfe.1234",
	}
	msgHB, err := encodeHeartbeat(&heartbeat)
	assert.Assert(t, err)

	factory.Broker.AwaitReceivers("mc/sfe.1234", 1)
	heartBeatSender, err := conn.Sender("mc/sfe.1234")
	heartBeatSender.Send(msgHB)

	// we don't have a safe way to access the internal state of the collector
	// without stopping it (or standing up a stub http mux). Instead, we'll
	// wait for a flush record so we know that the internal state is settled
	// before stopping it and cracking things open.
	msg, err := flushReceiver.Receive()
	assert.Assert(t, err)
	assert.Equal(t, msg.Properties.Subject, "FLUSH")

	close(stopCh)
	<-fc.beaconReceiver.done // only way to tell the flow collector is done

	eventSource, ok := fc.eventSources["1234"]
	t.Log(fc.eventSources)
	assert.Assert(t, ok)
	assert.Equal(t, eventSource.Beacons, 2)
	assert.Equal(t, eventSource.Heartbeats, 1)
}

func TestStartupShutdown(t *testing.T) {
	scenarios := []struct {
		Name        string
		Spec        FlowCollectorSpec
		Controllers int
	}{
		{
			Name:        "RecordMetrics basic",
			Controllers: 0,
			Spec: FlowCollectorSpec{
				Mode:              RecordMetrics,
				Origin:            "origin",
				PromReg:           prometheus.NewRegistry(),
				ConnectionFactory: messaging.NewMockConnectionFactory(t, "local://test"),
				FlowRecordTtl:     5 * time.Minute,
			},
		}, {
			Name:        "RecordMetrics four controllers",
			Controllers: 4,
			Spec: FlowCollectorSpec{
				Mode:              RecordMetrics,
				Origin:            "origin",
				PromReg:           prometheus.NewRegistry(),
				ConnectionFactory: messaging.NewMockConnectionFactory(t, "local://test"),
				FlowRecordTtl:     5 * time.Minute,
			},
		}, {
			Name:        "RecordStatus basic",
			Controllers: 0,
			Spec: FlowCollectorSpec{
				Mode:              RecordStatus,
				Origin:            "origin",
				PromReg:           prometheus.NewRegistry(),
				ConnectionFactory: messaging.NewMockConnectionFactory(t, "local://test"),
				FlowRecordTtl:     5 * time.Minute,
			},
		}, {
			Name:        "RecordStatus many controllers",
			Controllers: 128,
			Spec: FlowCollectorSpec{
				Mode:              RecordStatus,
				Origin:            "origin",
				PromReg:           prometheus.NewRegistry(),
				ConnectionFactory: messaging.NewMockConnectionFactory(t, "local://test"),
				FlowRecordTtl:     5 * time.Minute,
			},
		},
	}
	for _, _testspec := range scenarios {
		s := _testspec
		t.Run(s.Name, func(t *testing.T) {
			t.Parallel()
			fc := NewFlowCollector(s.Spec)

			done := make(chan struct{})

			conn, err := s.Spec.ConnectionFactory.Connect()
			assert.Assert(t, err)

			flushReceivers := []messaging.Receiver{}
			for site := 1; site < s.Controllers; site++ {

				r, err := conn.Receiver(fmt.Sprintf("sfe.%d", site), 128)
				assert.Assert(t, err)
				flushReceivers = append(flushReceivers, r)

				// simulated controller - beacons+heartbeats
				go runMockCollector(t, strconv.Itoa(site), conn, done)
			}
			// start flow collector
			fc.Start(done)
			mockFactory := s.Spec.ConnectionFactory.(*messaging.MockConnectionFactory)
			mockFactory.Broker.AwaitReceivers(BeaconAddress, 1)

			// wait for flushes
			var allFlushed sync.WaitGroup
			for _, r := range flushReceivers {
				allFlushed.Add(1)
				go func(receiver messaging.Receiver) {
					defer allFlushed.Done()
					msg, err := receiver.Receive()
					assert.Assert(t, err)
					assert.Equal(t, msg.Properties.Subject, "FLUSH")
				}(r)
			}
			allFlushed.Wait()
			close(done)

			// last thing FlowCollector does on stop is close the beaconReceiver
			// make sure that is done
			<-fc.beaconReceiver.done

		})
	}
}
func runMockCollector(t *testing.T, siteID string, conn messaging.Connection, done chan struct{}) {
	ticker := time.NewTicker(time.Second / 15)
	defer ticker.Stop()
	mcAddr := fmt.Sprintf("mc/sfe.%s", siteID)
	directAddr := fmt.Sprintf("sfe.%s", siteID)
	heartSender, err := conn.Sender(mcAddr + ".heartbeats")
	assert.Assert(t, err)
	beaconSender, err := conn.Sender(BeaconAddress)
	assert.Assert(t, err)
	beacon := BeaconRecord{
		Version:    1,
		SourceType: "CONTROLLER",
		Address:    mcAddr,
		Direct:     directAddr,
		Identity:   siteID,
	}
	msgBeacon, err := encodeBeacon(&beacon)
	assert.Assert(t, err)
	heartbeat := HeartbeatRecord{
		Version:  1,
		Identity: siteID,
		Source:   directAddr,
	}
	assert.Assert(t, err)
	for {
		select {
		case <-done:
			return
		case <-ticker.C:
			assert.Assert(t, beaconSender.Send(msgBeacon))
			heartbeat.Now = uint64(time.Now().UnixNano()) / uint64(time.Microsecond)
			msgHeart, err := encodeHeartbeat(&heartbeat)
			assert.Assert(t, err)
			assert.Assert(t, heartSender.Send(msgHeart))
		}
	}
}

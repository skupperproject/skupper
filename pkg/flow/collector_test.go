package flow

import (
	"reflect"
	"strconv"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
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
	reg := prometheus.NewRegistry()
	u, _ := time.ParseDuration("5m")
	fc := NewFlowCollector(FlowCollectorSpec{
		Mode:              RecordMetrics,
		Origin:            "origin",
		PromReg:           reg,
		ConnectionFactory: nil,
		FlowRecordTtl:     u,
	})
	fc.metrics = fc.NewMetrics(fc.prometheusReg)

	assert.Assert(t, fc != nil)
	stopCh := make(chan struct{})
	go fc.beaconUpdates(stopCh)
	go fc.recordUpdates(stopCh)

	beacon := BeaconRecord{
		Version:    1,
		SourceType: "CONTROLLER",
		Address:    RecordPrefix + "1234",
		Direct:     "sfe.1234",
		Identity:   "1234",
	}

	var beacons []interface{}
	beacons = append(beacons, beacon)
	fc.beaconsIncoming <- beacons
	time.Sleep(1 * time.Second)
	eventSource, ok := fc.eventSources["1234"]
	assert.Assert(t, ok)
	assert.Equal(t, eventSource.Beacons, 1)
	fc.beaconsIncoming <- beacons
	time.Sleep(1 * time.Second)
	assert.Equal(t, eventSource.Beacons, 2)

	heartbeat := HeartbeatRecord{
		Version:  1,
		Identity: "1234",
		Source:   "sfe.1234",
	}

	var heartbeats []interface{}
	heartbeats = append(heartbeats, heartbeat)
	fc.heartbeatsIncoming <- heartbeats
	time.Sleep(1 * time.Second)
	assert.Equal(t, eventSource.Heartbeats, 1)

	time.Sleep(1 * time.Second)

	close(stopCh)
}

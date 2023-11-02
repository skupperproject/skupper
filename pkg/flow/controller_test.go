package flow

import (
	"os"
	"testing"
	"time"

	"gotest.tools/assert"
)

func TestUpdateProcess(t *testing.T) {
	fc := NewFlowController("mysite", "X.Y.Z", uint64(time.Now().UnixNano())/uint64(time.Microsecond), nil)
	assert.Assert(t, fc != nil)
	procName := "tcp-go-echo"
	sourceIP := "10.0.0.1"
	process := &ProcessRecord{
		Base: Base{
			Identity:  "myprocess",
			Parent:    "mysite",
			StartTime: uint64(time.Now().UnixNano()) / uint64(time.Microsecond),
		},
		Name:       &procName,
		SourceHost: &sourceIP,
	}
	err := UpdateProcess(fc, false, "tcp-go-echo", process)
	assert.Assert(t, err)
	assert.Equal(t, len(fc.processRecords), 1)
	processUpdate := <-fc.processOutgoing
	assert.Assert(t, processUpdate != nil)
	assert.Equal(t, *processUpdate.SourceHost, "10.0.0.1")
	err = UpdateProcess(fc, true, "ns/tcp-go-echo", process)
	assert.Assert(t, err)
	assert.Equal(t, len(fc.processRecords), 0)
	processUpdate = <-fc.processOutgoing
	assert.Assert(t, processUpdate != nil)
	assert.Assert(t, processUpdate.EndTime != 0)
}

func TestUpdateHost(t *testing.T) {
	fc := NewFlowController("mysite", "X.Y.Z", uint64(time.Now().UnixNano())/uint64(time.Microsecond), nil)
	assert.Assert(t, fc != nil)
	hostName := "bastion-server"
	host := &HostRecord{
		Base: Base{
			Identity:  "myhost",
			Parent:    "mysite",
			StartTime: uint64(time.Now().UnixNano()) / uint64(time.Microsecond),
		},
		Name: &hostName,
	}
	err := UpdateHost(fc, false, "bastion-server", host)
	assert.Assert(t, err)
	assert.Equal(t, len(fc.hostRecords), 1)
	hostUpdate := <-fc.hostOutgoing
	assert.Assert(t, hostUpdate != nil)
	assert.Equal(t, hostUpdate.Parent, "mysite")
	err = UpdateHost(fc, true, "bastion-server", host)
	assert.Assert(t, err)
	assert.Equal(t, len(fc.hostRecords), 0)
}

func TestUpdateBeaconAndHeartbeats(t *testing.T) {
	_ = os.Setenv("SKUPPER_SITE_ID", "mySite")
	fc := NewFlowController("mySite", "X.Y.Z", uint64(time.Now().UnixNano())/uint64(time.Microsecond), nil)
	assert.Assert(t, fc != nil)
	stopCh := make(chan struct{})
	go fc.updateBeaconAndHeartbeats(stopCh)
	beaconUpdate := <-fc.beaconOutgoing
	assert.Assert(t, beaconUpdate != nil)
	beacon, ok := beaconUpdate.(*BeaconRecord)
	assert.Assert(t, ok)
	assert.Equal(t, beacon.Version, uint32(1))
	assert.Equal(t, beacon.SourceType, "CONTROLLER")
	heartbeatUpdate := <-fc.heartbeatOutgoing
	assert.Assert(t, heartbeatUpdate != nil)
	heartbeat, ok := heartbeatUpdate.(*HeartbeatRecord)
	assert.Assert(t, ok)
	assert.Equal(t, heartbeat.Source, "sfe.mySite")
	time.Sleep(1 * time.Second)
	beaconUpdate = <-fc.beaconOutgoing
	assert.Assert(t, beaconUpdate != nil)
	heartbeatUpdate = <-fc.heartbeatOutgoing
	assert.Assert(t, heartbeatUpdate != nil)
	heartbeat, ok = heartbeatUpdate.(*HeartbeatRecord)
	assert.Assert(t, ok)
	close(stopCh)
}

func TestUpdateRecords(t *testing.T) {
	_ = os.Setenv("SKUPPER_SITE_ID", "mySite")
	_ = os.Setenv("SKUPPER_NAMESPACE", "myNamespace")
	fc := NewFlowController("mySite", "X.Y.Z", uint64(time.Now().UnixNano())/uint64(time.Microsecond), nil)
	assert.Assert(t, fc != nil)
	stopCh := make(chan struct{})
	go fc.updateRecords(stopCh)

	recordUpdate := <-fc.recordOutgoing
	assert.Assert(t, recordUpdate != nil)
	siteUpdate, ok := recordUpdate.(*SiteRecord)
	assert.Assert(t, ok)
	assert.Equal(t, siteUpdate.Identity, "mySite")

	procName := "tcp-go-echo"
	sourceIP := "10.0.0.1"
	process := &ProcessRecord{
		Base: Base{
			Identity:  "myprocess",
			Parent:    "mysite",
			StartTime: uint64(time.Now().UnixNano()) / uint64(time.Microsecond),
		},
		Name:       &procName,
		SourceHost: &sourceIP,
	}
	fc.processOutgoing <- process

	recordUpdate = <-fc.recordOutgoing
	assert.Assert(t, recordUpdate != nil)
	processUpdate, ok := recordUpdate.(*ProcessRecord)
	assert.Assert(t, ok)
	assert.Equal(t, *processUpdate.Name, "tcp-go-echo")

	hostName := "bastion-server"
	host := &HostRecord{
		Base: Base{
			Identity:  "myhost",
			Parent:    "mysite",
			StartTime: uint64(time.Now().UnixNano()) / uint64(time.Microsecond),
		},
		Name: &hostName,
	}
	fc.hostOutgoing <- host

	recordUpdate = <-fc.recordOutgoing
	assert.Assert(t, recordUpdate != nil)
	hostUpdate, ok := recordUpdate.(*HostRecord)
	assert.Assert(t, ok)
	assert.Equal(t, *hostUpdate.Name, hostName)

	flush := FlushRecord{
		Address: "sfe.1234",
	}
	var flushes []interface{}
	flushes = append(flushes, flush)
	fc.flushIncoming <- flushes

	time.Sleep(5 * time.Second)
	close(stopCh)

}

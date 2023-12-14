package flow

import (
	"os"
	"testing"
	"time"

	"github.com/skupperproject/skupper/api/types"
	"gotest.tools/assert"
)

func TestUpdateProcess(t *testing.T) {
	fc := NewFlowController("mysite", "X.Y.Z", uint64(time.Now().UnixNano())/uint64(time.Microsecond), nil, WithPolicyDisabled)
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
	fc := NewFlowController("mysite", "X.Y.Z", uint64(time.Now().UnixNano())/uint64(time.Microsecond), nil, WithPolicyDisabled)
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
	fc := NewFlowController("mySite", "X.Y.Z", uint64(time.Now().UnixNano())/uint64(time.Microsecond), nil, WithPolicyDisabled)
	assert.Assert(t, fc != nil)
	stopCh := make(chan struct{})
	go fc.updateBeacon(stopCh)
	go fc.updateHeartbeats(stopCh)
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
	fc := NewFlowController("mySite", "X.Y.Z", uint64(time.Now().UnixNano())/uint64(time.Microsecond), nil, WithPolicyDisabled)
	assert.Assert(t, fc != nil)
	stopCh := make(chan struct{})

	go fc.updateRecords(stopCh, fc.siteRecordController.Start(stopCh))

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

func TestSiteController(t *testing.T) {
	alwaysOnEval := policyEnabledConst(true)
	flipFlopEval := stubPolicyEvaluator{
		Next: make(chan bool, 8),
	}
	testCases := []struct {
		controller        *siteRecordController
		expectedOut       []func(t *testing.T, record *SiteRecord, ok bool)
		waitTimeout       time.Duration
		expectWaitTimeout bool
	}{
		{
			controller: &siteRecordController{
				Identity:        "basic-podman",
				Name:            "basic",
				Platform:        string(types.PlatformPodman),
				CreatedAt:       1_111_111,
				policyEvaluator: WithPolicyDisabled,
			},
			expectedOut: []func(*testing.T, *SiteRecord, bool){
				func(t *testing.T, rec *SiteRecord, ok bool) {
					assert.Equal(t, rec.Identity, "basic-podman")
					assert.Equal(t, *rec.Name, "basic")
					assert.Equal(t, *rec.Platform, string(types.PlatformPodman))
					assert.Equal(t, rec.StartTime, uint64(1_111_111))
					assert.Equal(t, *rec.Policy, Disabled)
				},
			},
			waitTimeout: time.Millisecond * 500,
		},
		{
			controller: &siteRecordController{
				Identity:        "basic-kube",
				Name:            "basic",
				Platform:        string(types.PlatformKubernetes),
				CreatedAt:       1_111_111,
				PolicyEnabled:   true,
				policyEvaluator: alwaysOnEval,
				pollInterval:    time.Millisecond * 5,
			},
			expectedOut: []func(*testing.T, *SiteRecord, bool){
				func(t *testing.T, rec *SiteRecord, ok bool) {
					assert.Equal(t, rec.Identity, "basic-kube")
					assert.Equal(t, *rec.Name, "basic")
					assert.Equal(t, *rec.Platform, string(types.PlatformKubernetes))
					assert.Equal(t, rec.StartTime, uint64(1_111_111))
					assert.Equal(t, *rec.Policy, Enabled)
				},
			},
			waitTimeout:       time.Millisecond * 250,
			expectWaitTimeout: true,
		},
		{
			controller: &siteRecordController{
				Identity:        "policy-updates-kube",
				Name:            "basic",
				Platform:        string(types.PlatformKubernetes),
				CreatedAt:       1_111_111,
				policyEvaluator: flipFlopEval,
				pollInterval:    time.Millisecond * 5,
			},
			expectedOut: []func(*testing.T, *SiteRecord, bool){
				func(t *testing.T, rec *SiteRecord, ok bool) {
					assert.Equal(t, rec.Identity, "policy-updates-kube")
					assert.Equal(t, *rec.Name, "basic")
					assert.Equal(t, *rec.Platform, string(types.PlatformKubernetes))
					assert.Equal(t, rec.StartTime, uint64(1_111_111))
					assert.Equal(t, *rec.Policy, Disabled)
					// queue two updates to policy enabled
					flipFlopEval.Next <- false
					flipFlopEval.Next <- true
					flipFlopEval.Next <- false
				},
				func(t *testing.T, rec *SiteRecord, ok bool) { // closed
					assert.DeepEqual(t, *rec, SiteRecord{
						Base: Base{
							RecType:  recordNames[Site],
							Identity: "policy-updates-kube",
						},
						Policy: &Enabled,
					})
				},
				func(t *testing.T, rec *SiteRecord, ok bool) { // closed
					assert.DeepEqual(t, rec, &SiteRecord{
						Base: Base{
							RecType:  recordNames[Site],
							Identity: "policy-updates-kube",
						},
						Policy: &Disabled,
					})
				},
			},
			waitTimeout: time.Millisecond * 500,
		},
	}

	for _, testCase := range testCases {
		tc := testCase
		t.Run(tc.controller.Identity, func(t *testing.T) {
			t.Parallel()
			end := make(chan struct{})
			defer close(end)

			out := tc.controller.Start(end)
			for i, expect := range tc.expectedOut {
				select {
				case actual, ok := <-out:
					expect(t, actual, ok)
				case <-time.After(tc.waitTimeout):
					t.Fatalf("test case timed out waiting for output %d", i)
				}
			}
			if tc.expectWaitTimeout {
				select {
				case actual := <-out:
					t.Errorf("unexpected update at end of test: %v", actual)
				case <-time.After(tc.waitTimeout):
					// okay
				}
			}
		})
	}
}

type stubPolicyEvaluator struct {
	IsEnabled bool
	Next      chan bool
}

func (s stubPolicyEvaluator) Enabled() bool {
	select {
	case next, ok := <-s.Next:
		if ok {
			s.IsEnabled = next
		}
	default:
	}
	return s.IsEnabled
}

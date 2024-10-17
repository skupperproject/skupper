package eventsource

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/skupperproject/skupper/pkg/vanflow"
	"github.com/skupperproject/skupper/pkg/vanflow/session"
	"gotest.tools/assert"
	"gotest.tools/poll"
)

func TestDiscoveryBasic(t *testing.T) {
	t.Parallel()
	tstCtx, tstCancel := context.WithCancel(context.Background())
	defer tstCancel()
	factory, rtt := requireContainers(t)
	ctr, tstCtr := factory.Create(), factory.Create()
	ctr.Start(tstCtx)
	tstCtr.Start(tstCtx)

	beaconAddress := mcsfe(uniqueSuffix("all"))
	discovery := NewDiscovery(ctr, DiscoveryOptions{BeaconAddress: beaconAddress})

	discoveredOut := make(chan Info, 8)
	forgottenOut := make(chan Info, 8)
	done := make(chan struct{})
	go func() {
		defer close(done)
		discovery.Run(tstCtx, DiscoveryHandlers{
			Discovered: func(info Info) {
				discoveredOut <- info
			},
			Forgotten: func(info Info) {
				forgottenOut <- info
			},
		})
	}()

	tstSender := tstCtr.NewSender(beaconAddress, session.SenderOptions{})

	testSuffix := uniqueSuffix("")
	sourceAID, sourceBID := "a"+testSuffix, "b"+testSuffix
	beaconA := fixtureBeaconFor(sourceAID, "ROUTER")
	beaconB := fixtureBeaconFor(sourceBID, "CONTROLLER")

	tstSender.Send(tstCtx, beaconA.Encode())
	tstSender.Send(tstCtx, beaconB.Encode())

	// a "real" skupper router can drop beacon messages when there is not yet a
	// consumer ready. retry sending initial beacons once when the
	// implemenation is not a mock
	retryOnceAfter := time.Now().Add(time.Hour * 48)
	if _, isMock := tstSender.(interface{ IsMock() }); !isMock {
		retryOnceAfter = time.Now().Add(500 * rtt)
	}
	// wait for discovery.List to return two sources
	poll.WaitOn(t,
		func(t poll.LogT) poll.Result {
			actual, desired := len(discovery.List()), 2
			if actual == desired {
				return poll.Success()
			}
			if _, isMock := tstSender.(interface{ IsMock() }); !isMock && time.Now().After(retryOnceAfter) {
				tstSender.Send(tstCtx, beaconA.Encode())
				tstSender.Send(tstCtx, beaconB.Encode())
				retryOnceAfter = retryOnceAfter.Add(time.Hour)
			}
			return poll.Continue("number of event sources is %d, not %d", actual, desired)
		}, poll.WithTimeout(3000*rtt),
	)

	// expect two events
	eventA := <-discoveredOut
	eventB := <-discoveredOut
	if eventA.ID == sourceBID {
		eventA, eventB = eventB, eventA
	}

	sourceA, ok := discovery.Get(sourceAID)
	assert.Check(t, ok)
	sourceB, ok := discovery.Get(sourceBID)
	assert.Check(t, ok)

	assert.DeepEqual(t, sourceA, eventA, cmpopts.IgnoreFields(Info{}, "LastSeen"))
	assert.DeepEqual(t, sourceB, eventB, cmpopts.IgnoreFields(Info{}, "LastSeen"))

	tstSender.Send(tstCtx, beaconA.Encode())
	tstSender.Send(tstCtx, beaconB.Encode())

	// wait for LastSeen to update
	poll.WaitOn(t,
		func(t poll.LogT) poll.Result {
			presentA, ok := discovery.Get(sourceAID)
			if !ok {
				return poll.Error(fmt.Errorf("error getting source 'a'"))
			}
			presentB, ok := discovery.Get(sourceBID)
			if !ok {
				return poll.Error(fmt.Errorf("error getting source 'b'"))
			}
			prevA, currentA := eventA.LastSeen, presentA.LastSeen
			prevB, currentB := eventB.LastSeen, presentB.LastSeen
			if currentA.After(prevA) && currentB.After(prevB) {
				return poll.Success()
			}
			return poll.Continue("waiting for lastseen to advance")
		}, poll.WithTimeout(300*rtt),
	)
	assert.Check(t, len(discoveredOut) == 0, "expected no new discovery events after subsequent beacons")
	assert.Check(t, len(forgottenOut) == 0, "expected no new forgotten events after subsequent beacons")

	assert.Check(t, !discovery.Forget("c"), "expected to ignore call to Forget for unknown id")
	assert.Check(t, len(forgottenOut) == 0, "expected no new events after invalid call to Forget")

	assert.Check(t, discovery.Forget(sourceAID), "expected ok to forget event source 'a'")
	// wait for discovery.List to return only one source
	poll.WaitOn(t,
		func(t poll.LogT) poll.Result {
			actual, desired := len(discovery.List()), 1
			if actual == desired {
				return poll.Success()
			}
			return poll.Continue("number of event sources is %d, not %d: got %v", actual, desired, discovery.List())
		}, poll.WithTimeout(1000*rtt),
	)

	// expect one event
	eventDelete := <-forgottenOut
	assert.Check(t, eventDelete.ID == sourceAID)
	assert.Equal(t, len(discovery.List()), 1)
	_, ok = discovery.Get(sourceAID)
	assert.Check(t, !ok, "expected Get on forgotten ID to return not ok")

	tstCancel()
	select {
	case <-time.After(rtt * 500):
		t.Error("expected discovery.Run to finish after cancelling context")
	case <-done: // okay
	}
}

func TestDiscoveryWatch(t *testing.T) {
	t.Parallel()
	tstCtx, tstCancel := context.WithCancel(context.Background())
	defer tstCancel()
	factory, rtt := requireContainers(t)
	ctr, tstCtr := factory.Create(), factory.Create()
	ctr.Start(tstCtx)
	tstCtr.Start(tstCtx)

	beaconAddress := mcsfe(uniqueSuffix("all"))
	discovery := NewDiscovery(ctr, DiscoveryOptions{BeaconAddress: beaconAddress})

	discoveredOut := make(chan Info, 8)
	forgottenOut := make(chan Info, 8)
	done := make(chan struct{})
	go func() {
		defer close(done)
		discovery.Run(tstCtx, DiscoveryHandlers{
			Discovered: func(info Info) {
				discoveredOut <- info
			},
			Forgotten: func(info Info) {
				forgottenOut <- info
			},
		})
	}()

	sourceAID := uniqueSuffix("a")

	beaconSender := tstCtr.NewSender(beaconAddress, session.SenderOptions{})
	heartbeatSender := tstCtr.NewSender(mcsfe(sourceAID), session.SenderOptions{})
	// continually send heartbeats for source a
	go func() {
		heartbeat := vanflow.HeartbeatMessage{
			Version:      1,
			Now:          1000,
			Identity:     sourceAID,
			MessageProps: vanflow.MessageProps{To: mcsfe(sourceAID)},
		}
		for {
			select {
			case <-time.After(rtt):
				heartbeatSender.Send(tstCtx, heartbeat.Encode())
				heartbeat.Now++
			case <-tstCtx.Done():
				return
			}
		}
	}()

	// send a beacon for router a and await the discovery event
	beaconA := fixtureBeaconFor(sourceAID, "ROUTER")

	assert.Check(t, beaconSender.Send(tstCtx, beaconA.Encode()))
	var event Info
	select {
	case out := <-discoveredOut:
		event = out
	case <-time.After(10 * rtt):
		t.Log("retrying beacon") //
		assert.Check(t, beaconSender.Send(tstCtx, beaconA.Encode()))
		event = <-discoveredOut
	}

	client := NewClient(ctr, ClientOptions{Source: event})
	// start a new watched client and begin listening
	err := discovery.NewWatchClient(tstCtx, WatchConfig{
		Client:                  client,
		ID:                      event.ID,
		Timeout:                 rtt * 10,
		GracePeriod:             time.Second,
		DiscoveryUpdateInterval: rtt * 2,
	})
	assert.Check(t, err)

	listenCtx, listenCancel := context.WithCancel(tstCtx)
	client.Listen(listenCtx, FromSourceAddress())

	poll.WaitOn(t,
		func(t poll.LogT) poll.Result {
			present, ok := discovery.Get(sourceAID)
			if !ok {
				return poll.Error(fmt.Errorf("event source 'a' forgotten"))
			}
			prev, current := event.LastSeen, present.LastSeen
			if current.After(prev) {
				return poll.Success()
			}
			return poll.Continue("waiting for lastseen to advance")
		}, poll.WithDelay(rtt), poll.WithTimeout(300*rtt),
	)

	listenCancel()

	select {
	case event := <-forgottenOut:
		assert.Check(t, event.ID == sourceAID)
	case <-time.After(200 * rtt):
		t.Error("expected source to be forgotten after starting watch client with no activity")
	}
}

func fixtureBeaconFor(id string, source string) vanflow.BeaconMessage {
	return vanflow.BeaconMessage{
		Version:    1,
		SourceType: source,
		Address:    fmt.Sprintf("mc/sfe.%s", id),
		Direct:     fmt.Sprintf("sfe.%s", id),
		Identity:   id,
	}
}

package cluster

import (
	"fmt"
	"testing"
	"time"

	"gotest.tools/assert"
	apiv1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
)

type ClusterContextMock struct {
	calledWithService string
	calledWithRetryFn func() (*apiv1.Service, error)
}

const (
	expectedError string = "some error"
)

var (
	mockSucceeds bool
	mockService  *apiv1.Service = &apiv1.Service{
		ObjectMeta: v1.ObjectMeta{
			//Name: "mockName",
		},
	}
	defaultDuration = 10 * time.Millisecond
)

func (cc *ClusterContextMock) waitForSkupperServiceToBeCreated(name string, retryFn func() (*apiv1.Service, error), backoff wait.Backoff) (*apiv1.Service, error) {
	cc.calledWithService = name
	cc.calledWithRetryFn = retryFn

	mockService.Name = name
	if mockSucceeds {
		return mockService, nil
	}
	return nil, fmt.Errorf(expectedError)

}

func TestWaitForSkupperServiceToBeCreatedAndReadyToUse(t *testing.T) {
	cc := &ClusterContextMock{}

	endTimeIfDelays := time.Now().Add(defaultDuration)
	mockSucceeds = true
	service, err := waitForSkupperServiceToBeCreatedAndReadyToUse(cc, "serviceA", defaultDuration)
	assert.Assert(t, time.Now().Sub(endTimeIfDelays) >= 0)
	assert.Assert(t, err)
	assert.Equal(t, service.Name, "serviceA")
	assert.Equal(t, cc.calledWithService, "serviceA")

	assert.Assert(t, cc.calledWithRetryFn == nil)

	endTimeIfDelays = time.Now().Add(defaultDuration)
	mockSucceeds = false

	service, err = waitForSkupperServiceToBeCreatedAndReadyToUse(cc, "serviceB", defaultDuration)
	assert.Assert(t, time.Now().Sub(endTimeIfDelays) < 0)
	assert.Error(t, err, expectedError)
	assert.Equal(t, cc.calledWithService, "serviceB")
	assert.Assert(t, service == nil)
	assert.Assert(t, cc.calledWithRetryFn == nil)
}

func TestWaitForSkupperServiceToBeCreated(t *testing.T) {

	cc := &ClusterContext{}
	var count int
	retryFnSuccedsInTheThirdCall := func() (*apiv1.Service, error) {
		count = count + 1
		if count == 3 {
			return mockService, nil
		}
		return nil, fmt.Errorf("some error")
	}

	testRetry := wait.Backoff{
		Steps:    1,
		Duration: defaultDuration,
	}
	count = 0

	endTimeIfDelays := time.Now().Add(defaultDuration)
	service, err := cc.waitForSkupperServiceToBeCreated("SomeService", retryFnSuccedsInTheThirdCall, testRetry)
	assert.Assert(t, time.Now().Sub(endTimeIfDelays) < 0) //1 retry means no delay!
	assert.Error(t, err, "some error")
	assert.Equal(t, count, 1)
	assert.Assert(t, service == nil)

	testRetry.Steps = 3

	//3 retries means 2 sleeps: try,sleep,try,sleep,try
	endTimeIfDelays = time.Now().Add(time.Duration(testRetry.Steps-1) * defaultDuration)

	count = 0
	service, err = cc.waitForSkupperServiceToBeCreated("SomeService", retryFnSuccedsInTheThirdCall, testRetry)
	assert.Assert(t, time.Now().Sub(endTimeIfDelays) >= 0) //1 retry means no delay!
	assert.Assert(t, err)
	assert.Equal(t, count, 3)
	assert.Assert(t, service == mockService)

	//adding a call with 0 steps to show what we get if we call with Steps=0
	//result is, no errors, function not called. which is "logical?)...
	testRetry.Steps = 0
	count = 0
	service, err = cc.waitForSkupperServiceToBeCreated("SomeService", retryFnSuccedsInTheThirdCall, testRetry)
	assert.Assert(t, err)
	assert.Assert(t, service == nil)

}

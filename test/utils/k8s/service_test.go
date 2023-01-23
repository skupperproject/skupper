package k8s

import (
	"context"
	"fmt"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"

	"gotest.tools/assert"
	apiv1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type ClusterContextMock struct {
	calledWithService string
	calledWithRetryFn func() (*apiv1.Service, error)
}

const (
	expectedError string = "some error"
	ns            string = "default"
)

var (
	kubeClient   = fake.NewSimpleClientset()
	defaultRetry = wait.Backoff{
		Steps:    5,
		Duration: time.Second,
	}
)

func prepareMockService(name string, mockSucceeds bool) {
	kubeClient.Fake.ReactionChain = []k8stesting.Reactor{}
	svc := &apiv1.Service{
		ObjectMeta: v1.ObjectMeta{
			Name: name,
		},
	}
	if !mockSucceeds {
		// return error when trying to create a service
		kubeClient.Fake.PrependReactor("*", "services", func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
			return true, nil, fmt.Errorf(expectedError)
		})
		return
	}
	kubeClient.Fake.PrependReactor("*", "services", func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, svc, nil
	})
}

func TestWaitForServiceToBeCreated(t *testing.T) {

	tcs := []struct {
		name    string
		service string
		succeed bool
		retryFn bool
	}{
		{
			name:    "valid-service-no-retryfn",
			service: "serviceA",
			succeed: true,
			retryFn: false,
		},
		{
			name:    "invalid-service-no-retryfn",
			service: "serviceB",
			succeed: false,
			retryFn: false,
		},
		{
			name:    "valid-service-retryfn",
			service: "serviceC",
			succeed: true,
			retryFn: true,
		},
		{
			name:    "invalid-service-retryfn",
			service: "serviceD",
			succeed: false,
			retryFn: true,
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			prepareMockService(tc.service, tc.succeed)
			var retryFn func() (*apiv1.Service, error) = nil
			if tc.retryFn {
				retryFn = func() (*apiv1.Service, error) {
					return kubeClient.CoreV1().Services(ns).Get(context.TODO(), "serviceC", v1.GetOptions{})
				}
			}
			service, err := WaitForServiceToBeCreated(ns, kubeClient, tc.service, retryFn, defaultRetry)
			assert.Equal(t, err == nil, tc.succeed)
			if tc.succeed {
				assert.Equal(t, service.Name, tc.service)
			}
		})
	}

}

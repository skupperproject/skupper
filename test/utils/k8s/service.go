package k8s

import (
	"github.com/skupperproject/skupper/test/utils/constants"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/retry"
	"time"
)

func WaitForServiceToBeCreated(ns string, kubeClient kubernetes.Interface, name string, retryFn func() (*apiv1.Service, error), backoff wait.Backoff) (*apiv1.Service, error) {
	var service *apiv1.Service = nil
	var err error
	isError := func(err error) bool {
		return err != nil
	}

	_retryFn := func() (*apiv1.Service, error) {
		return kubeClient.CoreV1().Services(ns).Get(name, metav1.GetOptions{})
	}

	if retryFn == nil {
		retryFn = _retryFn
	}

	return service, retry.OnError(backoff, isError, func() error {
		service, err = retryFn()
		return err
	})
}

func WaitForServiceToBeCreatedAndReadyToUse(ns string, kubeClient kubernetes.Interface, serviceName string, serviceReadyPeriod time.Duration) (*apiv1.Service, error) {
	time.Sleep(serviceReadyPeriod)
	service, err := WaitForServiceToBeCreated(ns, kubeClient, serviceName, nil, constants.DefaultRetry)
	if err != nil {
		return nil, err
	}
	return service, nil
}

func WaitForSkupperServiceToBeCreatedAndReadyToUse(ns string, kubeClient kubernetes.Interface, serviceName string) (*apiv1.Service, error) {
	return WaitForServiceToBeCreatedAndReadyToUse(ns, kubeClient, serviceName, constants.SkupperServiceReadyPeriod)
}

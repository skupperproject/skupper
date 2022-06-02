package k8s

import (
	"fmt"
	"time"

	"github.com/skupperproject/skupper/test/utils/constants"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/retry"
)

func WaitForServiceToBeAvailableDefaultTimeout(ns string, kubeClient kubernetes.Interface, name string) (service *apiv1.Service, err error) {
	return WaitForServiceToBeAvailable(ns, kubeClient, name, constants.SkupperServiceReadyPeriod)
}

func WaitForServiceToBeAvailable(ns string, kubeClient kubernetes.Interface, name string, timeout time.Duration) (service *apiv1.Service, err error) {

	// Create a service informer
	done := make(chan struct{})
	factory := informers.NewSharedInformerFactory(kubeClient, 0)
	svcInformer := factory.Core().V1().Services().Informer()
	svcInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			svc, _ := cache.MetaNamespaceKeyFunc(obj)
			// if service has been found, close the done channel
			if svc == fmt.Sprintf("%s/%s", ns, name) {
				service = obj.(*apiv1.Service)
				close(done)
			}
		},
	})
	stop := make(chan struct{})
	go svcInformer.Run(stop)

	// Wait for informer to be done or a timeoutCh
	timeoutCh := time.After(timeout)
	select {
	case <-timeoutCh:
		err = fmt.Errorf("timed out waiting on service to be available: %s", name)
	case <-done:
		break
	}

	// stop informer
	close(stop)

	return
}

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
	service, err := WaitForServiceToBeCreated(ns, kubeClient, serviceName, nil, constants.DefaultRetry)
	if err != nil {
		return nil, err
	}
	time.Sleep(serviceReadyPeriod)
	return service, nil
}

func WaitForSkupperServiceToBeCreatedAndReadyToUse(ns string, kubeClient kubernetes.Interface, serviceName string) (*apiv1.Service, error) {
	fmt.Printf("Waiting for skupper service: %s\n", serviceName)
	return WaitForServiceToBeCreatedAndReadyToUse(ns, kubeClient, serviceName, time.Second*10)
}

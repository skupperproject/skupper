package k8s

import (
	"fmt"
	"time"

	"github.com/skupperproject/skupper/pkg/kube"
	"github.com/skupperproject/skupper/test/utils/constants"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

func DeleteNamespaceAndWait(kubeClient kubernetes.Interface, name string) error {
	// Create a namespace informer
	done := make(chan struct{})
	factory := informers.NewSharedInformerFactory(kubeClient, 0)
	nsInformer := factory.Core().V1().Namespaces().Informer()
	nsInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		DeleteFunc: func(obj interface{}) {
			ns, _ := cache.MetaNamespaceKeyFunc(obj)
			// when requested namespace has been deleted, close the done channel
			if ns == name {
				close(done)
			}
		},
	})
	stop := make(chan struct{})
	go nsInformer.Run(stop)

	// Delete the ns
	if err := kube.DeleteNamespace(name, kubeClient); err != nil {
		return err
	}

	// Wait for informer to be done or a timeout
	timeout := time.After(constants.NamespaceDeleteTimeout)
	var err error = nil
	select {
	case <-timeout:
		err = fmt.Errorf("timed out waiting on namespace to be deleted: %s", name)
	case <-done:
		break
	}

	// stop informer
	close(stop)

	return err
}

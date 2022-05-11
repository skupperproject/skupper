package client

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/pkg/kube"
	"github.com/skupperproject/skupper/pkg/utils"
	"gotest.tools/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/tools/cache"
)

func TestServiceInterfaceDeleteMulti(t *testing.T) {
	namespace := "vsidmulti"
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	svcsFound := []string{}
	svcsRemoved := []string{}

	var cli *VanClient
	var err error

	isCluster := *clusterRun
	if !isCluster {
		t.SkipNow()
	}

	cli, err = NewClient(namespace, "", "")
	assert.Check(t, err, namespace)
	_, err = kube.NewNamespace(namespace, cli.KubeClient)
	assert.Check(t, err, namespace)
	defer kube.DeleteNamespace(namespace, cli.KubeClient)

	// Service Informer -------------------------
	informerFactory := informers.NewSharedInformerFactoryWithOptions(cli.KubeClient, 0, informers.WithNamespace(namespace))
	svcInformer := informerFactory.Core().V1().Services().Informer()
	svcInformer.AddEventHandler(&cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			svc := obj.(*corev1.Service)
			svcsFound = append(svcsFound, svc.Name)
		},
		DeleteFunc: func(obj interface{}) {
			svc := obj.(*corev1.Service)
			svcsRemoved = append(svcsRemoved, svc.Name)
		},
	})

	// ------------------------------------------------------------
	// Start all the informers and wait until each one is ready.
	// ------------------------------------------------------------
	informerFactory.Start(ctx.Done())
	result := cache.WaitForCacheSync(ctx.Done(), svcInformer.HasSynced)
	assert.Check(t, result, "Unable to sync an informer.")

	// Create a router.
	err = cli.RouterCreate(ctx, types.SiteConfig{
		Spec: types.SiteConfigSpec{
			SkupperName:       namespace,
			RouterMode:        string(types.TransportModeInterior),
			EnableController:  true,
			EnableServiceSync: true,
			EnableConsole:     false,
			AuthMode:          "",
			User:              "",
			Password:          "",
			Ingress:           types.IngressNoneString,
		},
	})
	assert.Check(t, err, "Unable to create VAN router")

	// Creating multiple services
	const serviceCount = 10
	t.Logf("creating services (sequentially)")
	for i := 1; i <= serviceCount; i++ {
		address := fmt.Sprintf("vsidmulti-%d", i)
		// Create the VAN Service Interface.
		service := types.ServiceInterface{
			Address:  address,
			Protocol: "tcp",
			Ports:    []int{8080},
		}
		assert.Assert(t, cli.ServiceInterfaceCreate(ctx, &service))
	}

	// wait till all services show up
	ctxSvc, cn := context.WithTimeout(ctx, time.Minute)
	defer cn()
	totalServices := serviceCount + 3

	err = utils.RetryWithContext(ctxSvc, time.Second, func() (bool, error) {
		return len(svcsFound) == totalServices, nil
	})
	assert.Assert(t, err)

	// deleting services concurrently
	t.Logf("deleting services (concurrently)")
	wg := &sync.WaitGroup{}
	wg.Add(serviceCount)
	errList := []error{}
	for i := 1; i <= serviceCount; i++ {
		go func(i int) {
			defer wg.Done()
			address := fmt.Sprintf("vsidmulti-%d", i)
			observedError := cli.ServiceInterfaceRemove(ctx, address)
			if observedError != nil {
				errList = append(errList, observedError)
			}
		}(i)
	}
	timeout := time.After(time.Minute)
	ch := make(chan struct{})
	go func() {
		wg.Wait()
		close(ch)
	}()
	select {
	case <-ch:
		break
	case <-timeout:
		break
	}

	// wait till all services show up
	ctxSvc, cn = context.WithTimeout(ctx, time.Minute)
	defer cn()

	err = utils.RetryWithContext(ctxSvc, time.Second, func() (bool, error) {
		return len(svcsRemoved) == serviceCount, nil
	})

	// no error expected during service delete
	assert.Equal(t, len(errList), 0, "No errors expected, but found: %v", errList)
	assert.Equal(t, len(svcsRemoved), serviceCount)
	assert.Assert(t, err)

	sil, err := cli.ServiceInterfaceList(context.Background())
	assert.Assert(t, err)
	assert.Equal(t, len(sil), 0)
}

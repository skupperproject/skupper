package client

import (
	"context"
	"fmt"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/pkg/kube"
	"github.com/skupperproject/skupper/pkg/utils"
	"gotest.tools/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
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

func TestServiceInterfaceRemoveAnnotated(t *testing.T) {
	type testCase struct {
		name               string
		doc                string
		service            *corev1.Service
		expectModified     bool
		skupperServiceName string
	}

	namespace := "vsidannotated"
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var cli *VanClient
	var err error

	isCluster := *clusterRun
	if !isCluster {
		t.SkipNow()
	}

	cli, err = NewClient(namespace, "", "")
	assert.Assert(t, err, namespace)
	_, err = kube.NewNamespace(namespace, cli.KubeClient)
	assert.Assert(t, err, namespace)
	defer kube.DeleteNamespace(namespace, cli.KubeClient)

	waitServiceUpdated := func(ctx context.Context, service string, skupperAnnotations bool) error {
		err = utils.RetryWithContext(ctx, time.Second, func() (bool, error) {
			return skupperAnnotations == kube.IsOriginalServiceModified(service, cli.Namespace, cli.KubeClient), nil
		})
		return err
	}

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
	assert.Assert(t, err, "Unable to create VAN router")

	// Deploying an http service
	_, err = cli.KubeClient.AppsV1().Deployments(cli.Namespace).Create(httpDeployment)
	assert.Assert(t, err, "Unable to create deployment")

	_, err = kube.WaitDeploymentReady(httpDeployment.Name, cli.Namespace, cli.KubeClient, time.Minute, time.Second)
	assert.Assert(t, err, "Timed out waiting on deployment to be ready")

	// Create a regular k8s service (not touched by skupper)
	// This service can be used as a target for the test cases
	httpManual := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: "http-manual",
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{Name: "web", Protocol: corev1.ProtocolTCP, Port: 8080, TargetPort: intstr.FromInt(8080)},
			},
			Selector: httpDeployment.Spec.Template.ObjectMeta.Labels,
			Type:     corev1.ServiceTypeClusterIP,
		},
	}
	_, err = cli.KubeClient.CoreV1().Services(cli.Namespace).Create(httpManual)
	assert.Assert(t, err, "Error creating http-manual service")

	tcs := []testCase{
		{
			name: "self-modified",
			doc:  "An annotated service is defined with just the skupper.io/proxy annotation",
			service: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name: "http",
					Annotations: map[string]string{
						types.ProxyQualifier: "http",
					},
				},
				Spec: corev1.ServiceSpec{
					Ports: []corev1.ServicePort{
						{Name: "web", Protocol: corev1.ProtocolTCP, Port: 8080, TargetPort: intstr.FromInt(8080)},
					},
					Selector: httpDeployment.Spec.Template.ObjectMeta.Labels,
					Type:     corev1.ServiceTypeClusterIP,
				},
			},
			expectModified:     true,
			skupperServiceName: "http",
		},
		{
			name: "service-with-address",
			doc:  "An annotated service is defined with skupper.io/proxy and skupper.io/address annotations",
			service: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name: "http",
					Annotations: map[string]string{
						types.ProxyQualifier:   "http",
						types.AddressQualifier: "http-skupper",
					},
				},
				Spec: corev1.ServiceSpec{
					Ports: []corev1.ServicePort{
						{Name: "web", Protocol: corev1.ProtocolTCP, Port: 8080, TargetPort: intstr.FromInt(8080)},
					},
					Selector: httpDeployment.Spec.Template.ObjectMeta.Labels,
					Type:     corev1.ServiceTypeClusterIP,
				},
			},
			expectModified:     false,
			skupperServiceName: "http-skupper",
		},
		{
			name: "service-with-target",
			doc:  "An annotated service is defined with skupper.io/proxy and skupper.io/target annotations",
			service: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name: "http-target",
					Annotations: map[string]string{
						types.ProxyQualifier:         "http",
						types.TargetServiceQualifier: "http-manual",
					},
				},
				Spec: corev1.ServiceSpec{
					Ports: []corev1.ServicePort{
						{Name: "web", Protocol: corev1.ProtocolTCP, Port: 8080, TargetPort: intstr.FromInt(8080)},
					},
					Selector: httpDeployment.Spec.Template.ObjectMeta.Labels,
					Type:     corev1.ServiceTypeClusterIP,
				},
			},
			expectModified:     true,
			skupperServiceName: "http-target",
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			t.Logf("Test summary: %s", tc.doc)
			ctx, cn := context.WithTimeout(context.Background(), time.Minute)
			defer cn()

			t.Logf("Creating annotated service")
			_, err = cli.KubeClient.CoreV1().Services(cli.Namespace).Create(tc.service)
			assert.Assert(t, err, "Error creating service %s - %v", tc.service.Name, err)

			t.Logf("Waiting for skupper service named %s to exist", tc.skupperServiceName)
			var skupperSvc *types.ServiceInterface
			err = utils.RetryWithContext(ctx, time.Second, func() (bool, error) {
				skupperSvc, err = cli.ServiceInterfaceInspect(ctx, tc.skupperServiceName)
				if err != nil {
					return true, err
				}
				return skupperSvc != nil, nil
			})
			assert.Assert(t, err, "Skupper service was not created")
			assert.Assert(t, skupperSvc.IsAnnotated(), "Expected skupper service to be originated by annotation")

			if tc.expectModified {
				t.Logf("Validating original service has been modified")
				err = waitServiceUpdated(ctx, tc.skupperServiceName, true)
				assert.Assert(t, err, "Kubernetes service has not been modified")

				svc, err := kube.GetService(tc.service.Name, cli.Namespace, cli.KubeClient)
				assert.Assert(t, err, "Kubernetes service not found")
				assert.Assert(t, kube.IsOriginalServiceModified(tc.service.Name, cli.Namespace, cli.KubeClient), "Original annotations not found")

				// validating ports changed
				initPorts := kube.GetServicePortMap(tc.service)
				currPorts := kube.GetServicePortMap(svc)
				assert.Assert(t, !reflect.DeepEqual(initPorts, currPorts), "Service ports did not change")

				// validating selectors changed
				origSelector := utils.StringifySelector(tc.service.Spec.Selector)
				currSelector := utils.StringifySelector(svc.Spec.Selector)
				assert.Assert(t, origSelector != currSelector, "Service selector did not change")
			}

			t.Logf("Removing service definition for %s", tc.skupperServiceName)
			assert.Assert(t, cli.ServiceInterfaceRemove(ctx, tc.skupperServiceName), "Error removing service definition")

			err = utils.RetryWithContext(ctx, time.Second, func() (bool, error) {
				skupperSvc, err = cli.ServiceInterfaceInspect(ctx, tc.skupperServiceName)
				if err != nil {
					return true, err
				}
				return skupperSvc == nil, nil
			})
			assert.Assert(t, err, "Timed out waiting on skupper service to be removed")
			assert.Assert(t, skupperSvc == nil, "Skupper service not removed")

			if tc.expectModified {
				t.Logf("Validating original service has been restored")
				err = waitServiceUpdated(ctx, tc.skupperServiceName, false)
				assert.Assert(t, err, "Kubernetes service has not been restored")
				svc, err := kube.GetService(tc.service.Name, cli.Namespace, cli.KubeClient)
				assert.Assert(t, err, "Kubernetes service not found")
				assert.Assert(t, !kube.IsOriginalServiceModified(tc.service.Name, cli.Namespace, cli.KubeClient), "Original annotations still present")

				// validating ports changed
				initPorts := kube.GetServicePortMap(tc.service)
				currPorts := kube.GetServicePortMap(svc)
				assert.Assert(t, reflect.DeepEqual(initPorts, currPorts), "Service ports not restored")

				// validating selectors changed
				origSelector := utils.StringifySelector(tc.service.Spec.Selector)
				currSelector := utils.StringifySelector(svc.Spec.Selector)
				assert.Assert(t, origSelector == currSelector, "Service selector not restored")
			}

			t.Logf("Removing the original service")
			assert.Assert(t, kube.DeleteService(tc.service.Name, cli.Namespace, cli.KubeClient), "Error removing %s service", tc.service.Name)
		})
	}
}

package client

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/skupperproject/skupper/pkg/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"

	"github.com/google/go-cmp/cmp"
	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/pkg/kube"
	"gotest.tools/assert"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/tools/cache"
)

// If this function detects a difference between the expected and observed
// results, it asserts a test failure.
func check_result(t *testing.T, name string, timeoutSeconds float64, resultType string, expected []string, found *[]string, doc string) {
	if len(expected) <= 0 {
		return
	}
	// Sometimes it requires a little time for the requested entities to be
	// created and for the informers to tell us about them.
	// So -- count down by tenths of a second until the alotted timeout expires,
	// or until we have at least the correct number of results.
	for {
		if timeoutSeconds <= 0 {
			break
		}
		if len(*found) >= len(expected) {
			break
		}
		time.Sleep(time.Second)
		timeoutSeconds -= 1.0
	}
	if diff := cmp.Diff(expected, *found, trans); diff != "" {
		t.Errorf("TestServiceInterfaceCreate %s : %s mismatch (-want +got):\n%s", name, resultType, diff)
	}
}

func containsResult(t *testing.T, name string, timeoutSeconds float64, resultType string, expected []string, found *[]string) {
	if len(expected) <= 0 {
		return
	}

	for {
		if timeoutSeconds <= 0 {
			break
		}
		if len(*found) >= len(expected) {
			break
		}
		time.Sleep(time.Second)
		timeoutSeconds -= 1.0
	}
	for _, element := range expected {
		if !utils.StringSliceContains(*found, element) {
			t.Errorf("TestServiceInterfaceCreate %s : expected %s not found:%s", name, resultType, element)
		}
	}
}

func TestServiceInterfaceCreate(t *testing.T) {
	testcases := []struct {
		namespace        string
		doc              string
		init             bool
		expectedErr      string
		addr             string
		proto            string
		ports            []int
		user             string
		depsExpected     []string
		cmsExpected      []string
		rolesExpected    []string
		svcsExpected     []string
		realSvcsExpected []string
		secretsExpected  []string
		timeout          float64 // seconds
		tlsCredentials   string
	}{
		// The first four tests look at error returns (or the lack thereof)
		// caused by bad (or not bad) arguments. An expected error of "" means
		// that no error should be returned. An expected error of "error"
		// means that some kind of error should be returned. Anything else
		// means that the exact given error should be returned.
		{
			namespace:   "vsic-1",
			doc:         "Uninitialized.",
			init:        false,
			addr:        "",
			proto:       "",
			ports:       []int{0},
			expectedErr: "Skupper is not enabled",
		},
		{
			namespace:   "vsic-2",
			doc:         "Normal initialization.",
			init:        true,
			addr:        "vsic-2-addr",
			proto:       "tcp",
			ports:       []int{5672},
			expectedErr: "",
		},
		{
			namespace:   "vsic-3",
			doc:         "Bad protocol.",
			init:        true,
			addr:        "vsic-3-addr",
			proto:       "BISYNC",
			ports:       []int{64000},
			expectedErr: "BISYNC is not a valid mapping",
		},
		{
			namespace:   "vsic-4",
			doc:         "Bad port.",
			init:        true,
			addr:        "vsic-4-addr",
			proto:       "tcp",
			ports:       []int{314159},
			expectedErr: "outside valid range",
		},

		// The remaining tests verify the expected side-effects
		// of VAN service interface creation.
		// I.e., certain deployments and services should be created.
		{
			namespace:     "vsic-5",
			doc:           "Check basic deployments.",
			init:          true,
			addr:          "vsic-5-addr",
			proto:         "tcp",
			ports:         []int{1999, 2000},
			expectedErr:   "",
			depsExpected:  []string{"skupper-router", "skupper-service-controller"},
			cmsExpected:   []string{types.TransportConfigMapName, types.ServiceInterfaceConfigMap},
			rolesExpected: []string{types.ControllerRoleName, types.TransportRoleName},
			// The list of expected services is slightly different in
			// the mock environment vs. a real cluster.
			// It usually takes 10 or 12 seconds for the address service to
			// show up, but I am giving it a large timeout here. The result
			// checker will cut out as soon as it sees a result list of the
			// right size.
			svcsExpected:     []string{types.LocalTransportServiceName, types.TransportServiceName, types.ControllerServiceName},
			realSvcsExpected: []string{types.LocalTransportServiceName, types.TransportServiceName, types.ControllerServiceName, "vsic-5-addr"},
			timeout:          60.0,
		},
		{
			namespace:        "vsic-6",
			doc:              "Check basic deployments with TLS support",
			init:             true,
			addr:             "vsic-6-addr",
			proto:            "http2",
			ports:            []int{3000},
			expectedErr:      "",
			depsExpected:     []string{"skupper-router", "skupper-service-controller"},
			cmsExpected:      []string{types.TransportConfigMapName, types.ServiceInterfaceConfigMap},
			rolesExpected:    []string{types.ControllerRoleName, types.TransportRoleName},
			svcsExpected:     []string{types.LocalTransportServiceName, types.TransportServiceName, types.ControllerServiceName},
			realSvcsExpected: []string{types.LocalTransportServiceName, types.TransportServiceName, types.ControllerServiceName, "vsic-6-addr"},
			secretsExpected:  []string{types.ServiceClientSecret, types.SiteCaSecret, "skupper-vsic-6-addr"},
			timeout:          60.0,
			tlsCredentials:   "skupper-vsic-6-addr",
		},
	}

	for _, testcase := range testcases {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		depsFound := []string{}
		cmsFound := []string{}
		rolesFound := []string{}
		svcsFound := []string{}
		secretsFound := []string{}

		var cli *VanClient
		var err error

		isCluster := *clusterRun
		if isCluster {
			cli, err = NewClient(testcase.namespace, "", "")
		} else {
			cli, err = newMockClient(testcase.namespace, "", "")
		}
		assert.Check(t, err, testcase.namespace)
		_, err = kube.NewNamespace(testcase.namespace, cli.KubeClient)
		assert.Check(t, err, testcase.namespace)
		defer kube.DeleteNamespace(testcase.namespace, cli.KubeClient)

		// ------------------------------------------------------------
		// Create all informers that will let us check the expected
		// side effects of starting the VAN Service Interface..
		// ------------------------------------------------------------
		var informerList []cache.SharedIndexInformer

		// Deployment Informer -------------------------
		informerFactory := informers.NewSharedInformerFactoryWithOptions(cli.KubeClient, 0, informers.WithNamespace(testcase.namespace))

		depInformer := informerFactory.Apps().V1().Deployments().Informer()
		depInformer.AddEventHandler(&cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				dep := obj.(*appsv1.Deployment)
				depsFound = append(depsFound, dep.Name)
			},
		})
		informerList = append(informerList, depInformer)

		// Config Map Informer -------------------------
		cmInformer := informerFactory.Core().V1().ConfigMaps().Informer()
		cmInformer.AddEventHandler(&cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				cm := obj.(*corev1.ConfigMap)
				if cm.Name != "kube-root-ca.crt" { // seems to be something added in more recent kubernetes?
					cmsFound = append(cmsFound, cm.Name)
				}
			},
		})
		informerList = append(informerList, cmInformer)

		// Role Informer -------------------------
		roleInformer := informerFactory.Rbac().V1().Roles().Informer()
		roleInformer.AddEventHandler(&cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				role := obj.(*rbacv1.Role)
				rolesFound = append(rolesFound, role.Name)
			},
		})
		informerList = append(informerList, roleInformer)

		// Service Informer -------------------------
		svcInformer := informerFactory.Core().V1().Services().Informer()
		svcInformer.AddEventHandler(&cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				svc := obj.(*corev1.Service)
				svcsFound = append(svcsFound, svc.Name)
			},
		})
		informerList = append(informerList, svcInformer)

		// Secret Informer -------------------------
		secretInformer := informerFactory.Core().V1().Secrets().Informer()
		secretInformer.AddEventHandler(&cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				secret := obj.(*corev1.Secret)
				secretsFound = append(secretsFound, secret.Name)
			},
		})

		informerList = append(informerList, secretInformer)
		// ------------------------------------------------------------
		// Start all the informers and wait until each one is ready.
		// ------------------------------------------------------------
		informerFactory.Start(ctx.Done())
		for _, i := range informerList {
			result := cache.WaitForCacheSync(ctx.Done(), i.HasSynced)
			assert.Check(t, result, "Unable to sync an informer.")
		}

		// Create a router.
		if testcase.init {
			err = cli.RouterCreate(ctx, types.SiteConfig{
				Spec: types.SiteConfigSpec{
					SkupperName:       testcase.namespace,
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
		}

		if !isCluster && testcase.tlsCredentials != "" {
			setUpRouterDeploymentMock(cli)
		}

		// Create the VAN Service Interface.
		service := types.ServiceInterface{
			Address:        testcase.addr,
			Protocol:       testcase.proto,
			Ports:          testcase.ports,
			TlsCredentials: testcase.tlsCredentials,
		}
		observedError := cli.ServiceInterfaceCreate(ctx, &service)

		// Check error returns against what was expected.
		switch testcase.expectedErr {
		case "":
			assert.Check(t, observedError == nil || strings.Contains(observedError.Error(), "already defined"), "Test %s failure: An error was reported where none was expected. The error was |%s|.\n", testcase.namespace, observedError)
		default:
			if observedError == nil {
				assert.Check(t, observedError != nil, "Test %s failure: The expected error |%s| was not reported.\n", testcase.namespace, testcase.expectedErr)
			} else {
				assert.Check(t, strings.Contains(observedError.Error(), testcase.expectedErr), "Test %s failure: The reported error |%s| did not have the expected prefix |%s|.\n", testcase.namespace, observedError.Error(), testcase.expectedErr)
			}
		}

		// Check all the lists of expected entities.
		check_result(t, testcase.namespace, testcase.timeout, "dependencies", testcase.depsExpected, &depsFound, testcase.doc)
		check_result(t, testcase.namespace, testcase.timeout, "config maps", testcase.cmsExpected, &cmsFound, testcase.doc)
		check_result(t, testcase.namespace, testcase.timeout, "roles", testcase.rolesExpected, &rolesFound, testcase.doc)
		containsResult(t, testcase.namespace, testcase.timeout, "secret", testcase.secretsExpected, &secretsFound)

		if isCluster {
			check_result(t, testcase.namespace, testcase.timeout, "services", testcase.realSvcsExpected, &svcsFound, testcase.doc)
		} else {
			check_result(t, testcase.namespace, testcase.timeout, "services", testcase.svcsExpected, &svcsFound, testcase.doc)
		}
	}
}

func setUpRouterDeploymentMock(cli *VanClient) {
	var rep int32 = 1
	var routerDeployment = &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "skupper-router",
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &rep,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"application": "tcp-go-echo",
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name: "router",
						},
					},
				},
			},
		},
		Status: appsv1.DeploymentStatus{
			Replicas:      rep,
			ReadyReplicas: rep,
		},
	}
	if cli != nil {
		cli.KubeClient.(*fake.Clientset).Fake.PrependReactor("get", "deployments", func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
			name := action.(k8stesting.GetAction).GetName()
			if name == "skupper-router" {
				return true, routerDeployment, nil
			}
			return false, nil, nil
		})
	}
}

func TestServiceInterfaceCreateMulti(t *testing.T) {
	namespace := "vsicmulti"
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	svcsFound := []string{}

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
	t.Logf("creating services (concurrently)")
	wg := &sync.WaitGroup{}
	wg.Add(serviceCount)
	errList := []error{}
	for i := 1; i <= serviceCount; i++ {
		go func(i int) {
			defer wg.Done()
			address := fmt.Sprintf("vsicmulti-%d", i)
			// Create the VAN Service Interface.
			service := types.ServiceInterface{
				Address:  address,
				Protocol: "tcp",
				Ports:    []int{8080},
			}
			observedError := cli.ServiceInterfaceCreate(ctx, &service)
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
	ctxSvc, cn := context.WithTimeout(ctx, time.Minute)
	defer cn()
	totalServices := serviceCount + 3

	err = utils.RetryWithContext(ctxSvc, time.Second, func() (bool, error) {
		return len(svcsFound) == totalServices, nil
	})

	// no error expected during service create
	assert.Equal(t, len(errList), 0, "No errors expected, but found: %v", errList)
	assert.Equal(t, len(svcsFound), totalServices)
	assert.Assert(t, err)

	sil, err := cli.ServiceInterfaceList(context.Background())
	assert.Assert(t, err)
	assert.Equal(t, len(sil), serviceCount)
}

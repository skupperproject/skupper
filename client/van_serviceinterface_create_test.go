package client

import (
	"context"
	"fmt"
	"sort"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/skupperproject/skupper/api/types"
	"gotest.tools/assert"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/tools/cache"
	rbacv1 "k8s.io/api/rbac/v1"
)

var fp = fmt.Fprintf

// If this function detects a difference between the expected and observed
// results, it asserts a test failure.
func check_result(t *testing.T, timeout_seconds float64, expected []string, found *[]string, doc string) {
	if len(expected) <= 0 {
		return
	}
	for {
                // Sometimes it requires a little time for the requested entities to be
                // created and for the informers to tell us about them. 
                // So -- count down by tenths of a second until the alotted timeout expires, 
                // or until we have the correct number of results. 
                // If the latter -- check to make sure that they are as expected.
		if timeout_seconds <= 0 {
			assert.Assert(t, false, fmt.Sprintf("%s: timed out waiting for expected list size %d.\n", doc, len(expected)))
			break
		}
		if len(*found) >= len(expected) {
			break
		}
		time.Sleep(100 * time.Millisecond)
		timeout_seconds -= 0.1
	}
	sort.Strings(expected)
	sort.Strings(*found)
	report_string := fmt.Sprintf("%s: expected |%#v| found |%#v|\n", doc, expected, *found)
	assert.Assert(t, cmp.Equal(expected, *found, nil), report_string)
}

func TestVanServiceInterfaceCreate(t *testing.T) {
	testcases := []struct {
		name          string
		doc           string
		init          bool
		err           string
		addr          string
		proto         string
		port          int
		user          string
		depsExpected  []string
		cmsExpected   []string
		rolesExpected []string
		timeout       float64 // seconds
	}{
		// The first four tests look at error returns (or the lack thereof)
		// caused by bad (or not bad) arguments. An expected error of "" means
		// that no error should be returned. An expected error of "error"
		// means that some kind of error should be returned. Anything else
		// means that the exact given error should be returned.
		{
			name:  "vsic_1",
			doc:   "Uninitialized.",
			init:  false,
			addr:  "",
			proto: "",
			port:  0,
			err:   "Skupper not initialised in skupper",
		},
		{
			name:  "vsic_2",
			doc:   "Normal initialization.",
			init:  true,
			addr:  "half-addr",
			proto: "tcp",
			port:  5672,
			err:   "",
		},
		{
			name:  "vsic_3",
			doc:   "Bad protocol.",
			init:  true,
			addr:  "half-addr",
			proto: "BISYNC",
			port:  64000,
			err:   "BISYNC is not a valid mapping. Choose 'tcp', 'http' or 'http2'.",
		},
		{
			name:  "vsic_4",
			doc:   "Bad port.",
			init:  true,
			addr:  "half-addr",
			proto: "tcp",
			port:  314159,
			err:   "error", 
		},

		// The remaining tests verify the expected side-effects
		// of VAN service interface creation.
		// I.e., certain deployments and services should be created.
		{
			name:          "vsic_5",
			doc:           "Check basic deployments.",
			init:          true,
			addr:          "half-addr",
			proto:         "tcp",
			port:          1776,
			err:           "",
			depsExpected:  []string{"skupper-router", "skupper-service-controller"},
			cmsExpected:   []string{"skupper-services"},
			rolesExpected: []string{"skupper-edit", "skupper-view"},
			timeout:       5.0,
		},
	}

	for _, testcase := range testcases {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		depsFound := []string{}
		cmsFound := []string{}
		rolesFound := []string{}

		cli, err := newMockClient("skupper", "", "")
		assert.Check(t, err, testcase.name)

		//------------------------------------------------------------
		// Create all informers that will let us check the expected
		// side effects of starting the VAN Service Interface..
		//------------------------------------------------------------
		var informer_list []cache.SharedIndexInformer

		// Deployment Informer -------------------------
		informer_factory := informers.NewSharedInformerFactory(cli.KubeClient, 0)
		depInformer := informer_factory.Apps().V1().Deployments().Informer()
		depInformer.AddEventHandler(&cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				dep := obj.(*appsv1.Deployment)
				depsFound = append(depsFound, dep.Name)
			},
		})
		informer_list = append(informer_list, depInformer)

		// Config Map Informer -------------------------
		cmInformer := informer_factory.Core().V1().ConfigMaps().Informer()
		cmInformer.AddEventHandler(&cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				cm := obj.(*corev1.ConfigMap)
				cmsFound = append(cmsFound, cm.Name)
			},
		})
		informer_list = append(informer_list, cmInformer)

		// Role Informer -------------------------
		roleInformer := informer_factory.Rbac().V1().Roles().Informer()
		roleInformer.AddEventHandler(&cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				role := obj.(*rbacv1.Role)
				rolesFound = append(rolesFound, role.Name)
			},
		})
		informer_list = append(informer_list, roleInformer)

		//------------------------------------------------------------
		// Start all the informers and wait until each one is ready.
		//------------------------------------------------------------
		informer_factory.Start(ctx.Done())
		for _, i := range informer_list {
			result := cache.WaitForCacheSync(ctx.Done(), i.HasSynced)
			assert.Check(t, result, "Unable to sync an informer.")
		}

		// Create a router.
		err = cli.VanRouterCreate(ctx, types.VanSiteConfig{
			Spec: types.VanSiteConfigSpec{
				SkupperName:       "skupper",
				IsEdge:            false,
				EnableController:  true,
				EnableServiceSync: true,
				EnableConsole:     false,
				AuthMode:          "",
				User:              "",
				Password:          "",
				ClusterLocal:      true,
			},
		})
		assert.Check(t, err, "Unable to create VAN router")

		// Create the VAN Service Interface.
		service := types.ServiceInterface{
			Address:  testcase.addr,
			Protocol: testcase.proto,
			Port:     testcase.port,
		}
		err = cli.VanServiceInterfaceCreate(ctx, &service)

		// Check error returns against what was expected.
		switch testcase.err {
		case "":
			assert.Check(t, err == nil, "Test %s failure: %s  An error was reported where none was expected.\n", testcase.name, testcase.doc)
		case "error":
			assert.Check(t, err != nil, "Test %s failure: %s No error was reported, but one should have been.\n", testcase.name, testcase.doc)
		default:
			assert.Check(t, err == nil || testcase.err == err.Error(), "Test %s failure: %s The reported error was different from the expected error.\n", testcase.name, testcase.doc)
		}

		// Check all the lists of expected entities.
		check_result(t, testcase.timeout, testcase.depsExpected, &depsFound, testcase.doc)
		check_result(t, testcase.timeout, testcase.cmsExpected, &cmsFound, testcase.doc)
		check_result(t, testcase.timeout, testcase.rolesExpected, &rolesFound, testcase.doc)
	}
}

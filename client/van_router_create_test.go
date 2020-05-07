package client

import (
	"context"
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
)

func transform(in []string) []string {
	out := append([]string(nil), in...)
	sort.Strings(out)
	return out
}

func TestVanRouterCreateDefaults(t *testing.T) {
	testcases := []struct {
		doc              string
		namespace        string
		expectedError    string
		skupperName      string
		isEdge           bool
		enableController bool
		clusterLocal     bool
		depsExpected     []string
		cmsExpected      []string
		svcsExpected     []string
	}{
		{
			namespace:        "skupper",
			expectedError:    "",
			doc:              "test one",
			skupperName:      "skupper1",
			isEdge:           false,
			enableController: true,
			clusterLocal:     true,
			depsExpected:     []string{"skupper-proxy-controller", "skupper-router"},
			cmsExpected:      []string{"skupper-services"},
			svcsExpected:     []string{"skupper-messaging", "skupper-internal"},
		},
		{
			namespace:        "skupper",
			expectedError:    "Failed to get LoadBalancer IP or Hostname for service skupper-internal",
			doc:              "test two",
			skupperName:      "skupper2",
			isEdge:           false,
			enableController: true,
			clusterLocal:     false,
			depsExpected:     []string{"skupper-router"},
			cmsExpected:      []string{"skupper-services"},
			svcsExpected:     []string{"skupper-messaging", "skupper-internal"},
		},
		{
			namespace:        "skupper",
			expectedError:    "",
			doc:              "test three",
			skupperName:      "skupper3",
			isEdge:           true,
			enableController: true,
			clusterLocal:     true,
			depsExpected:     []string{"skupper-router", "skupper-proxy-controller"},
			cmsExpected:      []string{"skupper-services"},
			svcsExpected:     []string{"skupper-messaging"},
		},
		{
			namespace:        "skupper",
			expectedError:    "",
			doc:              "test four",
			skupperName:      "skupper4",
			isEdge:           false,
			enableController: false,
			clusterLocal:     true,
			depsExpected:     []string{"skupper-router"},
			cmsExpected:      []string{"skupper-services"},
			svcsExpected:     []string{"skupper-messaging", "skupper-internal"},
		},
	}

	trans := cmp.Transformer("Sort", func(in []string) []string {
		out := append([]string(nil), in...)
		sort.Strings(out)
		return out
	})

	for _, c := range testcases {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		depsFound := []string{}
		cmsFound := []string{}
		svcsFound := []string{}

		// Create the client
		cli, err := newMockClient(c.namespace, "", "")
		assert.Check(t, err, c.doc)

		informers := informers.NewSharedInformerFactory(cli.KubeClient, 0)
		depInformer := informers.Apps().V1().Deployments().Informer()
		depInformer.AddEventHandler(&cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				dep := obj.(*appsv1.Deployment)
				depsFound = append(depsFound, dep.Name)
			},
		})
		cmInformer := informers.Core().V1().ConfigMaps().Informer()
		cmInformer.AddEventHandler(&cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				cm := obj.(*corev1.ConfigMap)
				cmsFound = append(cmsFound, cm.Name)
			},
		})
		svcInformer := informers.Core().V1().Services().Informer()
		svcInformer.AddEventHandler(&cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				svc := obj.(*corev1.Service)
				svcsFound = append(svcsFound, svc.Name)
			},
		})
		informers.Start(ctx.Done())
		cache.WaitForCacheSync(ctx.Done(), depInformer.HasSynced)
		cache.WaitForCacheSync(ctx.Done(), cmInformer.HasSynced)
		cache.WaitForCacheSync(ctx.Done(), svcInformer.HasSynced)

		err = cli.VanRouterCreate(ctx, types.VanRouterCreateOptions{
			SkupperName:       c.skupperName,
			IsEdge:            c.isEdge,
			EnableController:  c.enableController,
			EnableServiceSync: true,
			EnableConsole:     false,
			AuthMode:          "",
			User:              "",
			Password:          "",
			ClusterLocal:      c.clusterLocal,
		})

		// TODO: make more deterministic
		time.Sleep(time.Second * 1)
		if c.clusterLocal {
			assert.Check(t, err, c.doc)
		} else {
			assert.Error(t, err, c.expectedError, c.doc)
		}
		assert.Assert(t, cmp.Equal(c.depsExpected, depsFound, trans), c.doc)
		assert.Assert(t, cmp.Equal(c.cmsExpected, cmsFound, trans), c.doc)
		assert.Assert(t, cmp.Equal(c.svcsExpected, svcsFound, trans), c.doc)
	}
}

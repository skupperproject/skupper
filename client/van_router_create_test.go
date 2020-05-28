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
	rbacv1 "k8s.io/api/rbac/v1"
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
		doc                  string
		namespace            string
		expectedError        string
		skupperName          string
		isEdge               bool
		enableController     bool
		enableRouterConsole  bool
		authMode             string
		clusterLocal         bool
		depsExpected         []string
		cmsExpected          []string
		rolesExpected        []string
		roleBindingsExpected []string
		secretsExpected      []string
		svcsExpected         []string
		svcAccountsExpected  []string
	}{
		{
			namespace:            "skupper",
			expectedError:        "",
			doc:                  "test one",
			skupperName:          "skupper1",
			isEdge:               false,
			enableController:     true,
			enableRouterConsole:  false,
			authMode:             "",
			clusterLocal:         true,
			depsExpected:         []string{"skupper-proxy-controller", "skupper-router"},
			cmsExpected:          []string{"skupper-site", "skupper-services"},
			rolesExpected:        []string{"skupper-view", "skupper-edit"},
			roleBindingsExpected: []string{"skupper-skupper-view", "skupper-proxy-controller-skupper-edit"},
			secretsExpected: []string{"skupper-ca",
				"skupper-internal-ca",
				"skupper-amqps",
				"skupper",
				"skupper-internal"},
			svcsExpected:        []string{"skupper-messaging", "skupper-internal", "skupper-controller"},
			svcAccountsExpected: []string{"skupper", "skupper-proxy-controller"},
		},
		{
			namespace:            "skupper",
			expectedError:        "Failed to get LoadBalancer IP or Hostname for service skupper-internal",
			doc:                  "test two",
			skupperName:          "skupper2",
			isEdge:               false,
			enableController:     true,
			enableRouterConsole:  false,
			authMode:             "",
			clusterLocal:         false,
			depsExpected:         []string{"skupper-router"},
			cmsExpected:          []string{"skupper-site", "skupper-services"},
			rolesExpected:        []string{"skupper-view"},
			roleBindingsExpected: []string{"skupper-skupper-view"},
			secretsExpected: []string{"skupper-ca",
				"skupper-internal-ca",
				"skupper-amqps",
				"skupper"},
			svcsExpected:        []string{"skupper-messaging", "skupper-internal"},
			svcAccountsExpected: []string{"skupper"},
		},
		{
			namespace:            "skupper",
			expectedError:        "",
			doc:                  "test three",
			skupperName:          "skupper3",
			isEdge:               true,
			enableController:     true,
			enableRouterConsole:  false,
			authMode:             "",
			clusterLocal:         true,
			depsExpected:         []string{"skupper-router", "skupper-proxy-controller"},
			cmsExpected:          []string{"skupper-site", "skupper-services"},
			rolesExpected:        []string{"skupper-view", "skupper-edit"},
			roleBindingsExpected: []string{"skupper-skupper-view", "skupper-proxy-controller-skupper-edit"},
			secretsExpected: []string{"skupper-ca",
				"skupper-amqps",
				"skupper"},
			svcsExpected:        []string{"skupper-messaging", "skupper-controller"},
			svcAccountsExpected: []string{"skupper", "skupper-proxy-controller"},
		},
		{
			namespace:            "skupper",
			expectedError:        "",
			doc:                  "test four",
			skupperName:          "skupper4",
			isEdge:               false,
			enableController:     false,
			enableRouterConsole:  false,
			authMode:             "",
			clusterLocal:         true,
			depsExpected:         []string{"skupper-router"},
			cmsExpected:          []string{"skupper-site", "skupper-services"},
			rolesExpected:        []string{"skupper-view"},
			roleBindingsExpected: []string{"skupper-skupper-view"},
			secretsExpected: []string{"skupper-ca",
				"skupper-internal-ca",
				"skupper-amqps",
				"skupper",
				"skupper-internal"},
			svcsExpected:        []string{"skupper-messaging", "skupper-internal"},
			svcAccountsExpected: []string{"skupper"},
		},
		{
			namespace:            "gilligan",
			expectedError:        "",
			doc:                  "test five",
			skupperName:          "skupper5",
			isEdge:               false,
			enableController:     true,
			enableRouterConsole:  true,
			authMode:             "internal",
			clusterLocal:         true,
			depsExpected:         []string{"skupper-proxy-controller", "skupper-router"},
			cmsExpected:          []string{"skupper-site", "skupper-services", "skupper-sasl-config"},
			rolesExpected:        []string{"skupper-view", "skupper-edit"},
			roleBindingsExpected: []string{"skupper-skupper-view", "skupper-proxy-controller-skupper-edit"},
			secretsExpected: []string{"skupper-ca",
				"skupper-internal-ca",
				"skupper-amqps",
				"skupper",
				"skupper-internal",
				"skupper-console-users"},
			svcsExpected:        []string{"skupper-messaging", "skupper-internal", "skupper-controller", "skupper-router-console"},
			svcAccountsExpected: []string{"skupper", "skupper-proxy-controller"},
		},
		{
			namespace:            "ginger",
			expectedError:        "",
			doc:                  "test six",
			skupperName:          "skupper6",
			isEdge:               false,
			enableController:     true,
			enableRouterConsole:  true,
			authMode:             "openshift",
			clusterLocal:         true,
			depsExpected:         []string{"skupper-proxy-controller", "skupper-router"},
			cmsExpected:          []string{"skupper-site", "skupper-services"},
			rolesExpected:        []string{"skupper-view", "skupper-edit"},
			roleBindingsExpected: []string{"skupper-skupper-view", "skupper-proxy-controller-skupper-edit"},
			secretsExpected: []string{"skupper-ca",
				"skupper-internal-ca",
				"skupper-amqps",
				"skupper",
				"skupper-internal"},
			svcsExpected:        []string{"skupper-messaging", "skupper-internal", "skupper-controller", "skupper-router-console"},
			svcAccountsExpected: []string{"skupper", "skupper-proxy-controller"},
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
		rolesFound := []string{}
		roleBindingsFound := []string{}
		secretsFound := []string{}
		svcsFound := []string{}
		svcAccountsFound := []string{}

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
		roleInformer := informers.Rbac().V1().Roles().Informer()
		roleInformer.AddEventHandler(&cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				role := obj.(*rbacv1.Role)
				rolesFound = append(rolesFound, role.Name)
			},
		})
		roleBindingInformer := informers.Rbac().V1().RoleBindings().Informer()
		roleBindingInformer.AddEventHandler(&cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				roleBinding := obj.(*rbacv1.RoleBinding)
				roleBindingsFound = append(roleBindingsFound, roleBinding.Name)
			},
		})
		secretInformer := informers.Core().V1().Secrets().Informer()
		secretInformer.AddEventHandler(&cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				secret := obj.(*corev1.Secret)
				secretsFound = append(secretsFound, secret.Name)
			},
		})
		svcInformer := informers.Core().V1().Services().Informer()
		svcInformer.AddEventHandler(&cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				svc := obj.(*corev1.Service)
				svcsFound = append(svcsFound, svc.Name)
			},
		})
		svcAccountInformer := informers.Core().V1().ServiceAccounts().Informer()
		svcAccountInformer.AddEventHandler(&cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				svcAccount := obj.(*corev1.ServiceAccount)
				svcAccountsFound = append(svcAccountsFound, svcAccount.Name)
			},
		})
		informers.Start(ctx.Done())
		cache.WaitForCacheSync(ctx.Done(), depInformer.HasSynced)
		cache.WaitForCacheSync(ctx.Done(), cmInformer.HasSynced)
		cache.WaitForCacheSync(ctx.Done(), roleInformer.HasSynced)
		cache.WaitForCacheSync(ctx.Done(), roleBindingInformer.HasSynced)
		cache.WaitForCacheSync(ctx.Done(), secretInformer.HasSynced)
		cache.WaitForCacheSync(ctx.Done(), svcInformer.HasSynced)
		cache.WaitForCacheSync(ctx.Done(), svcAccountInformer.HasSynced)

		err = cli.VanRouterCreate(ctx, types.VanRouterCreateOptions{
			SkupperName:         c.skupperName,
			IsEdge:              c.isEdge,
			EnableController:    c.enableController,
			EnableServiceSync:   true,
			EnableRouterConsole: c.enableRouterConsole,
			AuthMode:            c.authMode,
			EnableConsole:       false,
			User:                "",
			Password:            "",
			ClusterLocal:        c.clusterLocal,
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
		assert.Assert(t, cmp.Equal(c.rolesExpected, rolesFound, trans), c.doc)
		assert.Assert(t, cmp.Equal(c.roleBindingsExpected, roleBindingsFound, trans), c.doc)
		assert.Assert(t, cmp.Equal(c.secretsExpected, secretsFound, trans), c.doc)
		assert.Assert(t, cmp.Equal(c.svcsExpected, svcsFound, trans), c.doc)
		assert.Assert(t, cmp.Equal(c.svcAccountsExpected, svcAccountsFound, trans), c.doc)
	}
}

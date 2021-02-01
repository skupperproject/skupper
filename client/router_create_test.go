package client

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/pkg/kube"
	"gotest.tools/assert"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/tools/cache"
)

func TestRouterCreateDefaults(t *testing.T) {
	testcases := []struct {
		doc                  string
		namespace            string
		expectedError        string
		skupperName          string
		isEdge               bool
		enableController     bool
		enableRouterConsole  bool
		authMode             string
		user                 string
		password             string
		clusterLocal         bool
		depsExpected         []string
		cmsExpected          []string
		rolesExpected        []string
		roleBindingsExpected []string
		secretsExpected      []string
		svcsExpected         []string
		svcAccountsExpected  []string
		opts                 []cmp.Option
	}{
		{
			namespace:            "van-router-create1",
			expectedError:        "",
			doc:                  "test one",
			skupperName:          "skupper1",
			isEdge:               false,
			enableController:     true,
			enableRouterConsole:  false,
			authMode:             "",
			user:                 "",
			password:             "",
			clusterLocal:         true,
			depsExpected:         []string{"skupper-service-controller", "skupper-router"},
			cmsExpected:          []string{"skupper-services", "skupper-internal"},
			rolesExpected:        []string{"skupper-view", "skupper-edit"},
			roleBindingsExpected: []string{"skupper-skupper-view", "skupper-proxy-controller-skupper-edit"},
			secretsExpected: []string{"skupper-ca",
				"skupper-internal-ca",
				"skupper-amqps",
				"skupper",
				"skupper-internal"},
			svcsExpected:        []string{"skupper-messaging", "skupper-internal", "skupper-controller"},
			svcAccountsExpected: []string{"skupper", "skupper-proxy-controller"},
			opts: []cmp.Option{
				trans,
				cmpopts.IgnoreSliceElements(func(v string) bool { return !strings.HasPrefix(v, "skupper") }),
				cmpopts.IgnoreSliceElements(func(v string) bool { return strings.Contains(v, "dockercfg") }),
				cmpopts.IgnoreSliceElements(func(v string) bool { return strings.Contains(v, "token") }),
			},
		},
		{
			namespace:            "van-router-create2",
			expectedError:        "",
			doc:                  "test two",
			skupperName:          "skupper2",
			isEdge:               false,
			enableController:     true,
			enableRouterConsole:  true,
			authMode:             "unsecured",
			user:                 "",
			password:             "",
			clusterLocal:         false,
			depsExpected:         []string{"skupper-service-controller", "skupper-router"},
			cmsExpected:          []string{"skupper-services", "skupper-internal"},
			rolesExpected:        []string{"skupper-view", "skupper-edit"},
			roleBindingsExpected: []string{"skupper-skupper-view", "skupper-proxy-controller-skupper-edit"},
			secretsExpected: []string{"skupper-ca",
				"skupper-internal-ca",
				"skupper-amqps",
				"skupper",
				"skupper-internal"},
			svcsExpected:        []string{"skupper-messaging", "skupper-internal", "skupper-controller", "skupper-router-console"},
			svcAccountsExpected: []string{"skupper", "skupper-proxy-controller"},
			opts: []cmp.Option{
				trans,
				cmpopts.IgnoreSliceElements(func(v string) bool { return !strings.HasPrefix(v, "skupper") }),
				cmpopts.IgnoreSliceElements(func(v string) bool { return strings.Contains(v, "dockercfg") }),
				cmpopts.IgnoreSliceElements(func(v string) bool { return strings.Contains(v, "token") }),
			},
		},
		{
			namespace:            "van-router-create3",
			expectedError:        "",
			doc:                  "test three",
			skupperName:          "skupper3",
			isEdge:               false,
			enableController:     true,
			enableRouterConsole:  true,
			authMode:             "internal",
			user:                 "",
			password:             "",
			clusterLocal:         false,
			depsExpected:         []string{"skupper-service-controller", "skupper-router"},
			cmsExpected:          []string{"skupper-services", "skupper-internal", "skupper-sasl-config"},
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
			opts: []cmp.Option{
				trans,
				cmpopts.IgnoreSliceElements(func(v string) bool { return !strings.HasPrefix(v, "skupper") }),
				cmpopts.IgnoreSliceElements(func(v string) bool { return strings.Contains(v, "dockercfg") }),
				cmpopts.IgnoreSliceElements(func(v string) bool { return strings.Contains(v, "token") }),
			},
		},
		{
			namespace:            "van-router-create4",
			expectedError:        "",
			doc:                  "test four",
			skupperName:          "skupper4",
			isEdge:               false,
			enableController:     true,
			enableRouterConsole:  true,
			authMode:             "openshift",
			user:                 "",
			password:             "",
			clusterLocal:         false,
			depsExpected:         []string{"skupper-service-controller", "skupper-router"},
			cmsExpected:          []string{"skupper-services", "skupper-internal"},
			rolesExpected:        []string{"skupper-view", "skupper-edit"},
			roleBindingsExpected: []string{"skupper-skupper-view", "skupper-proxy-controller-skupper-edit"},
			secretsExpected: []string{"skupper-ca",
				"skupper-internal-ca",
				"skupper-amqps",
				"skupper",
				"skupper-internal",
				"skupper-controller-certs",
				"skupper-proxy-certs"},
			svcsExpected:        []string{"skupper-messaging", "skupper-internal", "skupper-controller", "skupper-router-console"},
			svcAccountsExpected: []string{"skupper", "skupper-proxy-controller"},
			opts: []cmp.Option{
				trans,
				cmpopts.IgnoreSliceElements(func(v string) bool { return !strings.HasPrefix(v, "skupper") }),
				cmpopts.IgnoreSliceElements(func(v string) bool { return strings.Contains(v, "dockercfg") }),
				cmpopts.IgnoreSliceElements(func(v string) bool { return strings.Contains(v, "token") }),
			},
		},
		{
			namespace:            "van-router-create5",
			expectedError:        "",
			doc:                  "test five",
			skupperName:          "skupper5",
			isEdge:               true,
			enableController:     true,
			enableRouterConsole:  true,
			authMode:             "unsecured",
			user:                 "Barney",
			password:             "Rubble",
			clusterLocal:         false,
			depsExpected:         []string{"skupper-service-controller", "skupper-router"},
			cmsExpected:          []string{"skupper-services", "skupper-internal"},
			rolesExpected:        []string{"skupper-view", "skupper-edit"},
			roleBindingsExpected: []string{"skupper-skupper-view", "skupper-proxy-controller-skupper-edit"},
			secretsExpected: []string{"skupper-ca",
				"skupper-amqps",
				"skupper"},
			svcsExpected:        []string{"skupper-messaging", "skupper-controller", "skupper-router-console"},
			svcAccountsExpected: []string{"skupper", "skupper-proxy-controller"},
			opts: []cmp.Option{
				trans,
				cmpopts.IgnoreSliceElements(func(v string) bool { return !strings.HasPrefix(v, "skupper") }),
				cmpopts.IgnoreSliceElements(func(v string) bool { return strings.Contains(v, "dockercfg") }),
				cmpopts.IgnoreSliceElements(func(v string) bool { return strings.Contains(v, "token") }),
			},
		},
	}

	isCluster := *clusterRun

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
		var cli *VanClient
		var err error
		if !isCluster {
			cli, err = newMockClient(c.namespace, "", "")
		} else {
			cli, err = NewClient(c.namespace, "", "")
		}
		assert.Check(t, err, c.doc)

		_, err = kube.NewNamespace(c.namespace, cli.KubeClient)
		assert.Check(t, err, c.doc)
		defer kube.DeleteNamespace(c.namespace, cli.KubeClient)

		informers := informers.NewSharedInformerFactoryWithOptions(cli.KubeClient, 0, informers.WithNamespace(c.namespace))
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

		getIngress := func() string {
			//true is none
			//false is loadBalanced?
			//need to know if this is ocp or kubernetes
			if c.clusterLocal || !isCluster {
				return types.IngressNoneString
			}
			return types.IngressLoadBalancerString
		}

		err = cli.RouterCreate(ctx, types.SiteConfig{
			Spec: types.SiteConfigSpec{
				SkupperName:         c.skupperName,
				IsEdge:              c.isEdge,
				EnableController:    c.enableController,
				EnableServiceSync:   true,
				EnableRouterConsole: c.enableRouterConsole,
				AuthMode:            c.authMode,
				EnableConsole:       false,
				User:                c.user,
				Password:            c.password,
				Ingress:             getIngress(),
			},
		})

		// TODO: make more deterministic
		time.Sleep(time.Second * 1)
		assert.Check(t, err, c.doc)
		if diff := cmp.Diff(c.depsExpected, depsFound, c.opts...); diff != "" {
			t.Errorf("TestRouterCreateDefaults "+c.doc+" deployments mismatch (-want +got):\n%s", diff)
		}
		if diff := cmp.Diff(c.cmsExpected, cmsFound, c.opts...); diff != "" {
			t.Errorf("TestRouterCreateDefaults "+c.doc+" config maps mismatch (-want +got):\n%s", diff)
		}
		if diff := cmp.Diff(c.rolesExpected, rolesFound, c.opts...); diff != "" {
			t.Errorf("TestRouterCreateDefaults "+c.doc+" roles mismatch (-want +got):\n%s", diff)
		}
		if diff := cmp.Diff(c.roleBindingsExpected, roleBindingsFound, c.opts...); diff != "" {
			t.Errorf("TestRouterCreateDefaults "+c.doc+" role bindings mismatch (-want +got):\n%s", diff)
		}
		if diff := cmp.Diff(c.svcsExpected, svcsFound, c.opts...); diff != "" {
			t.Errorf("TestRouterCreateDefaults "+c.doc+" services mismatch (-want +got):\n%s", diff)
		}
		if diff := cmp.Diff(c.svcAccountsExpected, svcAccountsFound, c.opts...); diff != "" {
			t.Errorf("TestRouterCreateDefaults "+c.doc+" service accounts mismatch (-want +got):\n%s", diff)
		}
		//TODO: consider set up short specific opts
		if !isCluster || (cli.RouteClient == nil && c.authMode == "openshift") {
			c.opts = append(c.opts, cmpopts.IgnoreSliceElements(func(v string) bool { return strings.Contains(v, "proxy-certs") }))
			c.opts = append(c.opts, cmpopts.IgnoreSliceElements(func(v string) bool { return strings.Contains(v, "controller-certs") }))
		}
		if diff := cmp.Diff(c.secretsExpected, secretsFound, c.opts...); diff != "" {
			t.Errorf("TestRouterCreateDefaults "+c.doc+" secrets mismatch (-want +got):\n%s", diff)
		}
	}
}

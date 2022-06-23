package client

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/pkg/kube"
	"gotest.tools/assert"
	is "gotest.tools/assert/cmp"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/tools/cache"
)

func TestRouterCreateDefaults(t *testing.T) {
	testcases := []struct {
		doc                  string
		siteId               string
		namespace            string
		expectedError        string
		skupperName          string
		routerMode           string
		enableController     bool
		enableRouterConsole  bool
		enableConsole        bool
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
			siteId:               "11111",
			expectedError:        "",
			doc:                  "test one",
			skupperName:          "skupper1",
			routerMode:           string(types.TransportModeInterior),
			enableController:     true,
			enableRouterConsole:  false,
			enableConsole:        false,
			authMode:             "",
			user:                 "",
			password:             "",
			clusterLocal:         true,
			depsExpected:         []string{"skupper-service-controller", "skupper-router"},
			cmsExpected:          []string{types.ServiceInterfaceConfigMap, types.TransportConfigMapName},
			rolesExpected:        []string{types.TransportRoleName, types.ControllerRoleName},
			roleBindingsExpected: []string{types.TransportRoleBindingName, types.ControllerRoleBindingName},
			secretsExpected: []string{types.LocalCaSecret,
				types.SiteCaSecret,
				types.LocalServerSecret,
				types.LocalClientSecret,
				types.ClaimsServerSecret,
				types.SiteServerSecret,
				types.ServiceCaSecret,
				types.ServiceClientSecret},
			svcsExpected:        []string{types.LocalTransportServiceName, types.ControllerServiceName, types.TransportServiceName},
			svcAccountsExpected: []string{types.TransportServiceAccountName, types.ControllerServiceAccountName},
			opts: []cmp.Option{
				trans,
				cmpopts.IgnoreSliceElements(func(v string) bool { return !strings.HasPrefix(v, "skupper") }),
				cmpopts.IgnoreSliceElements(func(v string) bool { return strings.Contains(v, "dockercfg") }),
				cmpopts.IgnoreSliceElements(func(v string) bool { return strings.Contains(v, "token") }),
			},
		},
		{
			namespace:            "van-router-create2",
			siteId:               "22222",
			expectedError:        "",
			doc:                  "test two",
			skupperName:          "skupper2",
			routerMode:           string(types.TransportModeInterior),
			enableController:     true,
			enableRouterConsole:  true,
			enableConsole:        true,
			authMode:             "unsecured",
			user:                 "",
			password:             "",
			clusterLocal:         false,
			depsExpected:         []string{"skupper-service-controller", "skupper-router"},
			cmsExpected:          []string{types.ServiceInterfaceConfigMap, types.TransportConfigMapName},
			rolesExpected:        []string{types.TransportRoleName, types.ControllerRoleName},
			roleBindingsExpected: []string{types.TransportRoleBindingName, types.ControllerRoleBindingName},
			secretsExpected: []string{types.LocalCaSecret,
				types.SiteCaSecret,
				types.LocalServerSecret,
				types.LocalClientSecret,
				types.ClaimsServerSecret,
				types.ConsoleServerSecret,
				types.SiteServerSecret,
				types.ServiceCaSecret,
				types.ServiceClientSecret},
			svcsExpected:        []string{types.LocalTransportServiceName, types.TransportServiceName, types.ControllerServiceName, "skupper-router-console"},
			svcAccountsExpected: []string{types.TransportServiceAccountName, types.ControllerServiceAccountName},
			opts: []cmp.Option{
				trans,
				cmpopts.IgnoreSliceElements(func(v string) bool { return !strings.HasPrefix(v, "skupper") }),
				cmpopts.IgnoreSliceElements(func(v string) bool { return strings.Contains(v, "dockercfg") }),
				cmpopts.IgnoreSliceElements(func(v string) bool { return strings.Contains(v, "token") }),
			},
		},
		{
			namespace:            "van-router-create3",
			siteId:               "33333",
			expectedError:        "",
			doc:                  "test three",
			skupperName:          "skupper3",
			routerMode:           string(types.TransportModeInterior),
			enableController:     true,
			enableRouterConsole:  true,
			enableConsole:        true,
			authMode:             "internal",
			user:                 "",
			password:             "",
			clusterLocal:         false,
			depsExpected:         []string{"skupper-service-controller", "skupper-router"},
			cmsExpected:          []string{types.ServiceInterfaceConfigMap, types.TransportConfigMapName, "skupper-sasl-config"},
			rolesExpected:        []string{types.TransportRoleName, types.ControllerRoleName},
			roleBindingsExpected: []string{types.TransportRoleBindingName, types.ControllerRoleBindingName},
			secretsExpected: []string{types.LocalCaSecret,
				types.SiteCaSecret,
				types.LocalServerSecret,
				types.LocalClientSecret,
				types.ClaimsServerSecret,
				types.SiteServerSecret,
				"skupper-console-users",
				types.ConsoleServerSecret,
				types.ServiceCaSecret,
				types.ServiceClientSecret},
			svcsExpected:        []string{types.LocalTransportServiceName, types.TransportServiceName, types.ControllerServiceName, "skupper-router-console"},
			svcAccountsExpected: []string{types.TransportServiceAccountName, types.ControllerServiceAccountName},
			opts: []cmp.Option{
				trans,
				cmpopts.IgnoreSliceElements(func(v string) bool { return !strings.HasPrefix(v, "skupper") }),
				cmpopts.IgnoreSliceElements(func(v string) bool { return strings.Contains(v, "dockercfg") }),
				cmpopts.IgnoreSliceElements(func(v string) bool { return strings.Contains(v, "token") }),
			},
		},
		{
			namespace:            "van-router-create4",
			siteId:               "44444",
			expectedError:        "",
			doc:                  "test four",
			skupperName:          "skupper4",
			routerMode:           string(types.TransportModeInterior),
			enableController:     true,
			enableRouterConsole:  true,
			enableConsole:        true,
			authMode:             "openshift",
			user:                 "",
			password:             "",
			clusterLocal:         false,
			depsExpected:         []string{"skupper-service-controller", "skupper-router"},
			cmsExpected:          []string{types.ServiceInterfaceConfigMap, types.TransportConfigMapName},
			rolesExpected:        []string{types.TransportRoleName, types.ControllerRoleName},
			roleBindingsExpected: []string{types.TransportRoleBindingName, types.ControllerRoleBindingName},
			secretsExpected: []string{types.LocalCaSecret,
				types.SiteCaSecret,
				types.LocalServerSecret,
				types.LocalClientSecret,
				types.ClaimsServerSecret,
				types.SiteServerSecret,
				types.OauthRouterConsoleSecret,
				types.ConsoleServerSecret,
				types.ServiceCaSecret,
				types.ServiceClientSecret},
			svcsExpected:        []string{types.LocalTransportServiceName, types.TransportServiceName, types.ControllerServiceName, "skupper-router-console"},
			svcAccountsExpected: []string{types.TransportServiceAccountName, types.ControllerServiceAccountName},
			opts: []cmp.Option{
				trans,
				cmpopts.IgnoreSliceElements(func(v string) bool { return !strings.HasPrefix(v, "skupper") }),
				cmpopts.IgnoreSliceElements(func(v string) bool { return strings.Contains(v, "dockercfg") }),
				cmpopts.IgnoreSliceElements(func(v string) bool { return strings.Contains(v, "token") }),
			},
		},
		{
			namespace:            "van-router-create5",
			siteId:               "55555",
			expectedError:        "",
			doc:                  "test five",
			skupperName:          "skupper5",
			routerMode:           string(types.TransportModeEdge),
			enableController:     true,
			enableRouterConsole:  true,
			enableConsole:        true,
			authMode:             "unsecured",
			user:                 "Barney",
			password:             "Rubble",
			clusterLocal:         false,
			depsExpected:         []string{"skupper-service-controller", "skupper-router"},
			cmsExpected:          []string{types.ServiceInterfaceConfigMap, types.TransportConfigMapName},
			rolesExpected:        []string{types.TransportRoleName, types.ControllerRoleName},
			roleBindingsExpected: []string{types.TransportRoleBindingName, types.ControllerRoleBindingName},
			secretsExpected: []string{types.LocalCaSecret,
				types.ConsoleServerSecret,
				types.LocalServerSecret,
				types.LocalClientSecret,
				types.ServiceCaSecret,
				types.ServiceClientSecret},
			svcsExpected:        []string{types.LocalTransportServiceName, types.ControllerServiceName, "skupper-router-console"},
			svcAccountsExpected: []string{types.TransportServiceAccountName, types.ControllerServiceAccountName},
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
			if c.clusterLocal || !isCluster {
				return types.IngressNoneString
			}
			return cli.GetIngressDefault()
		}

		err = cli.RouterCreate(ctx, types.SiteConfig{
			Spec: types.SiteConfigSpec{
				SkupperName:         c.skupperName,
				RouterMode:          c.routerMode,
				EnableController:    c.enableController,
				EnableServiceSync:   true,
				EnableRouterConsole: c.enableRouterConsole,
				AuthMode:            c.authMode,
				EnableConsole:       c.enableConsole,
				User:                c.user,
				Password:            c.password,
				Ingress:             getIngress(),
			},
			Reference: types.SiteConfigReference{
				UID: c.siteId,
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
		// TODO: consider set up short specific opts
		if !isCluster || (cli.RouteClient == nil && c.authMode == "openshift") {
			c.opts = append(c.opts, cmpopts.IgnoreSliceElements(func(v string) bool { return strings.Contains(v, types.OauthRouterConsoleSecret) }))
			c.opts = append(c.opts, cmpopts.IgnoreSliceElements(func(v string) bool { return strings.Contains(v, types.ConsoleServerSecret) }))
		}
		if diff := cmp.Diff(c.secretsExpected, secretsFound, c.opts...); diff != "" {
			t.Errorf("TestRouterCreateDefaults "+c.doc+" secrets mismatch (-want +got):\n%s", diff)
		}
	}
}

func TestRouterResourcesOptions(t *testing.T) {
	testcases := []struct {
		cpuOption                     string
		memoryOption                  string
		expectedCpu                   string
		expectedMemory                string
		configSyncCpuOption           string
		configSyncMemoryOption        string
		configSyncCpuLimitOption      string
		configSyncMemoryLimitOption   string
		configSyncNodeSelectorOption  string
		configSyncAffinityOption      string
		configSyncAntiAffinityOption  string
		configSyncExpectedCpu         string
		configSyncExpectedMemory      string
		configSyncExpectedCpuLimit    string
		configSyncExpectedMemoryLimit string
	}{
		{
			cpuOption:                     "2",
			memoryOption:                  "1G",
			expectedCpu:                   "2",
			expectedMemory:                "1G",
			configSyncCpuOption:           "1",
			configSyncMemoryOption:        "2G",
			configSyncCpuLimitOption:      "2",
			configSyncMemoryLimitOption:   "3G",
			configSyncExpectedCpu:         "1",
			configSyncExpectedMemory:      "2G",
			configSyncExpectedCpuLimit:    "2",
			configSyncExpectedMemoryLimit: "3G",
		},
		{
			cpuOption:   "0.8",
			expectedCpu: "800m",
		},
		{
			cpuOption:   "650m",
			expectedCpu: "650m",
		},
		{
			memoryOption:   "500M",
			expectedMemory: "500M",
		},
		{},
	}

	isCluster := *clusterRun

	for i, c := range testcases {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		namespace := fmt.Sprintf("router-resources-%d", i+1)
		var cli *VanClient
		var err error
		if !isCluster {
			cli, err = newMockClient(namespace, "", "")
		} else {
			cli, err = NewClient(namespace, "", "")
		}
		assert.Check(t, err, namespace)

		_, err = kube.NewNamespace(namespace, cli.KubeClient)
		assert.Check(t, err, namespace)
		defer kube.DeleteNamespace(namespace, cli.KubeClient)

		opts := types.SiteConfigSpec{}
		opts.Ingress = "none"
		opts.Router.Tuning.Cpu = c.cpuOption
		opts.Router.Tuning.Memory = c.memoryOption
		opts.ConfigSync.Tuning.Cpu = c.configSyncCpuOption
		opts.ConfigSync.Tuning.Memory = c.configSyncMemoryOption
		opts.ConfigSync.Tuning.CpuLimit = c.configSyncCpuLimitOption
		opts.ConfigSync.Tuning.MemoryLimit = c.configSyncMemoryLimitOption
		siteConfig, err := cli.SiteConfigCreate(ctx, opts)
		assert.Check(t, err, namespace)

		err = cli.RouterCreate(ctx, *siteConfig)
		assert.Check(t, err, namespace)

		deployment, err := cli.KubeClient.AppsV1().Deployments(namespace).Get("skupper-router", metav1.GetOptions{})
		assert.Check(t, err, namespace)

		container := deployment.Spec.Template.Spec.Containers[0]
		if c.expectedMemory != "" {
			quantity := container.Resources.Requests[corev1.ResourceMemory]
			assert.Equal(t, c.expectedMemory, quantity.String())
		} else {
			_, ok := container.Resources.Requests[corev1.ResourceMemory]
			assert.Assert(t, !ok, namespace)
		}
		if c.expectedCpu != "" {
			quantity := container.Resources.Requests[corev1.ResourceCPU]
			assert.Equal(t, c.expectedCpu, quantity.String())
		} else {
			_, ok := container.Resources.Requests[corev1.ResourceCPU]
			assert.Assert(t, !ok, namespace)
		}

		sideCarContainer := deployment.Spec.Template.Spec.Containers[1]
		if c.configSyncExpectedMemory != "" {
			quantity := sideCarContainer.Resources.Requests[corev1.ResourceMemory]
			assert.Equal(t, c.configSyncExpectedMemory, quantity.String())
		} else {
			_, ok := sideCarContainer.Resources.Requests[corev1.ResourceMemory]
			assert.Assert(t, !ok, namespace)
		}
		if c.configSyncExpectedCpu != "" {
			quantity := sideCarContainer.Resources.Requests[corev1.ResourceCPU]
			assert.Equal(t, c.configSyncExpectedCpu, quantity.String())
		} else {
			_, ok := sideCarContainer.Resources.Requests[corev1.ResourceCPU]
			assert.Assert(t, !ok, namespace)
		}
		if c.configSyncExpectedMemoryLimit != "" {
			quantity := sideCarContainer.Resources.Limits[corev1.ResourceMemory]
			assert.Equal(t, c.configSyncExpectedMemoryLimit, quantity.String())
		} else {
			_, ok := sideCarContainer.Resources.Limits[corev1.ResourceMemory]
			assert.Assert(t, !ok, namespace)
		}
		if c.configSyncExpectedCpuLimit != "" {
			quantity := sideCarContainer.Resources.Limits[corev1.ResourceCPU]
			assert.Equal(t, c.configSyncExpectedCpuLimit, quantity.String())
		} else {
			_, ok := sideCarContainer.Resources.Limits[corev1.ResourceCPU]
			assert.Assert(t, !ok, namespace)
		}

		existSharedVolume := false
		for _, volume := range deployment.Spec.Template.Spec.Volumes {
			if volume.Name == "skupper-router-certs" {
				existSharedVolume = true
			}
		}
		assert.Check(t, existSharedVolume == true)

		existSharedVolumePathInRouterContainer := false
		for _, path := range deployment.Spec.Template.Spec.Containers[0].VolumeMounts {
			if path.MountPath == "/etc/skupper-router-certs" {
				existSharedVolumePathInRouterContainer = true
			}
		}
		assert.Check(t, existSharedVolumePathInRouterContainer == true)

		existSharedVolumePathInSidecar := false
		for _, path := range deployment.Spec.Template.Spec.Containers[1].VolumeMounts {
			if path.MountPath == "/etc/skupper-router-certs" {
				existSharedVolumePathInSidecar = true
			}
		}
		assert.Check(t, existSharedVolumePathInSidecar == true)
	}
}

func TestRouterAffinityOptions(t *testing.T) {
	testcases := []struct {
		affinityOption     string
		antiAffinityOption string
		affinityLabels     map[string]string
		antiAffinityLabels map[string]string
	}{
		{
			affinityOption:     "app.kubernetes.io/name=foo",
			antiAffinityOption: "app.kubernetes.io/name=bar",
			affinityLabels: map[string]string{
				"app.kubernetes.io/name": "foo",
			},
			antiAffinityLabels: map[string]string{
				"app.kubernetes.io/name": "bar",
			},
		},
		{
			affinityOption: "app.kubernetes.io/name=foo",
			affinityLabels: map[string]string{
				"app.kubernetes.io/name": "foo",
			},
		},
		{
			antiAffinityOption: "app.kubernetes.io/name=bar",
			antiAffinityLabels: map[string]string{
				"app.kubernetes.io/name": "bar",
			},
		},
		{
			affinityOption:     "app.kubernetes.io/name=foo,flavour=strawberry",
			antiAffinityOption: "app.kubernetes.io/name=bar,flavour=vanilla",
			affinityLabels: map[string]string{
				"app.kubernetes.io/name": "foo",
				"flavour":                "strawberry",
			},
			antiAffinityLabels: map[string]string{
				"app.kubernetes.io/name": "bar",
				"flavour":                "vanilla",
			},
		},
		{},
	}

	isCluster := *clusterRun

	for i, c := range testcases {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		namespace := fmt.Sprintf("router-affinity-%d", i+1)
		var cli *VanClient
		var err error
		if !isCluster {
			cli, err = newMockClient(namespace, "", "")
		} else {
			cli, err = NewClient(namespace, "", "")
		}
		assert.Check(t, err, namespace)

		_, err = kube.NewNamespace(namespace, cli.KubeClient)
		assert.Check(t, err, namespace)
		defer kube.DeleteNamespace(namespace, cli.KubeClient)

		opts := types.SiteConfigSpec{}
		opts.Ingress = "none"
		opts.Router.Tuning.Affinity = c.affinityOption
		opts.Router.Tuning.AntiAffinity = c.antiAffinityOption
		siteConfig, err := cli.SiteConfigCreate(ctx, opts)
		assert.Check(t, err, namespace)

		err = cli.RouterCreate(ctx, *siteConfig)
		assert.Check(t, err, namespace)

		deployment, err := cli.KubeClient.AppsV1().Deployments(namespace).Get("skupper-router", metav1.GetOptions{})
		assert.Check(t, err, namespace)

		spec := deployment.Spec.Template.Spec
		if len(c.affinityLabels) > 0 {
			rules := spec.Affinity.PodAffinity.PreferredDuringSchedulingIgnoredDuringExecution
			assert.Equal(t, 1, len(rules), namespace)
			assert.Equal(t, int32(100), rules[0].Weight, namespace)
			assert.Equal(t, "kubernetes.io/hostname", rules[0].PodAffinityTerm.TopologyKey, namespace)
			actualLabels := rules[0].PodAffinityTerm.LabelSelector.MatchLabels
			for key, value := range c.affinityLabels {
				assert.Equal(t, value, actualLabels[key], namespace)
			}
			for key, value := range actualLabels {
				assert.Equal(t, c.affinityLabels[key], value, namespace)
			}
		} else if len(c.antiAffinityLabels) > 0 {
			assert.Assert(t, is.Nil(spec.Affinity.PodAffinity), namespace)
		} else {
			assert.Assert(t, is.Nil(spec.Affinity), namespace)
		}
		if len(c.antiAffinityLabels) > 0 {
			rules := spec.Affinity.PodAntiAffinity.PreferredDuringSchedulingIgnoredDuringExecution
			assert.Equal(t, 1, len(rules), namespace)
			assert.Equal(t, int32(100), rules[0].Weight, namespace)
			assert.Equal(t, "kubernetes.io/hostname", rules[0].PodAffinityTerm.TopologyKey, namespace)
			actualLabels := rules[0].PodAffinityTerm.LabelSelector.MatchLabels
			for key, value := range c.antiAffinityLabels {
				assert.Equal(t, value, actualLabels[key], namespace)
			}
			for key, value := range actualLabels {
				assert.Equal(t, c.antiAffinityLabels[key], value, namespace)
			}
		} else if len(c.affinityLabels) > 0 {
			assert.Assert(t, is.Nil(spec.Affinity.PodAntiAffinity), namespace)
		} else {
			assert.Assert(t, is.Nil(spec.Affinity), namespace)
		}
	}
}

func TestRouterNodeSelectorOption(t *testing.T) {
	testcases := []struct {
		option   string
		expected map[string]string
	}{
		{
			option: "kubernetes.io/hostname=foo",
			expected: map[string]string{
				"kubernetes.io/hostname": "foo",
			},
		},
		{
			option: "kubernetes.io/hostname=foo,bar=baz",
			expected: map[string]string{
				"kubernetes.io/hostname": "foo",
				"bar":                    "baz",
			},
		},
		{},
	}

	isCluster := *clusterRun

	for i, c := range testcases {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		namespace := fmt.Sprintf("router-node-selector-%d", i+1)
		var cli *VanClient
		var err error
		if !isCluster {
			cli, err = newMockClient(namespace, "", "")
		} else {
			cli, err = NewClient(namespace, "", "")
		}
		assert.Check(t, err, namespace)

		_, err = kube.NewNamespace(namespace, cli.KubeClient)
		assert.Check(t, err, namespace)
		defer kube.DeleteNamespace(namespace, cli.KubeClient)

		opts := types.SiteConfigSpec{}
		opts.Ingress = "none"
		opts.Router.Tuning.NodeSelector = c.option
		siteConfig, err := cli.SiteConfigCreate(ctx, opts)
		assert.Check(t, err, namespace)

		err = cli.RouterCreate(ctx, *siteConfig)
		assert.Check(t, err, namespace)

		deployment, err := cli.KubeClient.AppsV1().Deployments(namespace).Get("skupper-router", metav1.GetOptions{})
		assert.Check(t, err, namespace)

		spec := deployment.Spec.Template.Spec
		if len(c.expected) > 0 {
			for key, value := range c.expected {
				assert.Equal(t, value, spec.NodeSelector[key], namespace)
			}
			for key, value := range spec.NodeSelector {
				assert.Equal(t, c.expected[key], value, namespace)
			}
		}
	}
}

func TestControllerAffinityOptions(t *testing.T) {
	testcases := []struct {
		affinityOption     string
		antiAffinityOption string
		affinityLabels     map[string]string
		antiAffinityLabels map[string]string
	}{
		{
			affinityOption:     "app.kubernetes.io/name=foo",
			antiAffinityOption: "app.kubernetes.io/name=bar",
			affinityLabels: map[string]string{
				"app.kubernetes.io/name": "foo",
			},
			antiAffinityLabels: map[string]string{
				"app.kubernetes.io/name": "bar",
			},
		},
		{
			affinityOption: "app.kubernetes.io/name=foo",
			affinityLabels: map[string]string{
				"app.kubernetes.io/name": "foo",
			},
		},
		{
			antiAffinityOption: "app.kubernetes.io/name=bar",
			antiAffinityLabels: map[string]string{
				"app.kubernetes.io/name": "bar",
			},
		},
		{
			affinityOption:     "app.kubernetes.io/name=foo,flavour=strawberry",
			antiAffinityOption: "app.kubernetes.io/name=bar,flavour=vanilla",
			affinityLabels: map[string]string{
				"app.kubernetes.io/name": "foo",
				"flavour":                "strawberry",
			},
			antiAffinityLabels: map[string]string{
				"app.kubernetes.io/name": "bar",
				"flavour":                "vanilla",
			},
		},
		{},
	}

	isCluster := *clusterRun

	for i, c := range testcases {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		namespace := fmt.Sprintf("controller-affinity-%d", i+1)
		var cli *VanClient
		var err error
		if !isCluster {
			cli, err = newMockClient(namespace, "", "")
		} else {
			cli, err = NewClient(namespace, "", "")
		}
		assert.Check(t, err, namespace)

		_, err = kube.NewNamespace(namespace, cli.KubeClient)
		assert.Check(t, err, namespace)
		defer kube.DeleteNamespace(namespace, cli.KubeClient)

		opts := types.SiteConfigSpec{}
		opts.Ingress = "none"
		opts.EnableController = true
		opts.Controller.Affinity = c.affinityOption
		opts.Controller.AntiAffinity = c.antiAffinityOption
		siteConfig, err := cli.SiteConfigCreate(ctx, opts)
		assert.Check(t, err, namespace)

		err = cli.RouterCreate(ctx, *siteConfig)
		assert.Check(t, err, namespace)

		deployment, err := cli.KubeClient.AppsV1().Deployments(namespace).Get("skupper-service-controller", metav1.GetOptions{})
		assert.Check(t, err, namespace)

		spec := deployment.Spec.Template.Spec
		if len(c.affinityLabels) > 0 {
			assert.Assert(t, spec.Affinity != nil, namespace)
			assert.Assert(t, spec.Affinity.PodAffinity != nil, namespace)
			rules := spec.Affinity.PodAffinity.PreferredDuringSchedulingIgnoredDuringExecution
			assert.Equal(t, 1, len(rules), namespace)
			assert.Equal(t, int32(100), rules[0].Weight, namespace)
			assert.Equal(t, "kubernetes.io/hostname", rules[0].PodAffinityTerm.TopologyKey, namespace)
			actualLabels := rules[0].PodAffinityTerm.LabelSelector.MatchLabels
			for key, value := range c.affinityLabels {
				assert.Equal(t, value, actualLabels[key], namespace)
			}
			for key, value := range actualLabels {
				assert.Equal(t, c.affinityLabels[key], value, namespace)
			}
		} else if len(c.antiAffinityLabels) > 0 {
			assert.Assert(t, is.Nil(spec.Affinity.PodAffinity), namespace)
		} else {
			assert.Assert(t, is.Nil(spec.Affinity), namespace)
		}
		if len(c.antiAffinityLabels) > 0 {
			assert.Assert(t, spec.Affinity != nil, namespace)
			assert.Assert(t, spec.Affinity.PodAntiAffinity != nil, namespace)
			rules := spec.Affinity.PodAntiAffinity.PreferredDuringSchedulingIgnoredDuringExecution
			assert.Equal(t, 1, len(rules), namespace)
			assert.Equal(t, int32(100), rules[0].Weight, namespace)
			assert.Equal(t, "kubernetes.io/hostname", rules[0].PodAffinityTerm.TopologyKey, namespace)
			actualLabels := rules[0].PodAffinityTerm.LabelSelector.MatchLabels
			for key, value := range c.antiAffinityLabels {
				assert.Equal(t, value, actualLabels[key], namespace)
			}
			for key, value := range actualLabels {
				assert.Equal(t, c.antiAffinityLabels[key], value, namespace)
			}
		} else if len(c.affinityLabels) > 0 {
			assert.Assert(t, is.Nil(spec.Affinity.PodAntiAffinity), namespace)
		} else {
			assert.Assert(t, is.Nil(spec.Affinity), namespace)
		}
	}
}

func TestControllerNodeSelectorOption(t *testing.T) {
	testcases := []struct {
		option   string
		expected map[string]string
	}{
		{
			option: "kubernetes.io/hostname=foo",
			expected: map[string]string{
				"kubernetes.io/hostname": "foo",
			},
		},
		{
			option: "kubernetes.io/hostname=foo,bar=baz",
			expected: map[string]string{
				"kubernetes.io/hostname": "foo",
				"bar":                    "baz",
			},
		},
		{},
	}

	isCluster := *clusterRun

	for i, c := range testcases {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		namespace := fmt.Sprintf("controller-node-selector-%d", i+1)
		var cli *VanClient
		var err error
		if !isCluster {
			cli, err = newMockClient(namespace, "", "")
		} else {
			cli, err = NewClient(namespace, "", "")
		}
		assert.Check(t, err, namespace)

		_, err = kube.NewNamespace(namespace, cli.KubeClient)
		assert.Check(t, err, namespace)
		defer kube.DeleteNamespace(namespace, cli.KubeClient)

		opts := types.SiteConfigSpec{}
		opts.Ingress = "none"
		opts.EnableController = true
		opts.Controller.NodeSelector = c.option
		siteConfig, err := cli.SiteConfigCreate(ctx, opts)
		assert.Check(t, err, namespace)

		err = cli.RouterCreate(ctx, *siteConfig)
		assert.Check(t, err, namespace)

		deployment, err := cli.KubeClient.AppsV1().Deployments(namespace).Get("skupper-service-controller", metav1.GetOptions{})
		assert.Check(t, err, namespace)

		spec := deployment.Spec.Template.Spec
		if len(c.expected) > 0 {
			for key, value := range c.expected {
				assert.Equal(t, value, spec.NodeSelector[key], namespace)
			}
			for key, value := range spec.NodeSelector {
				assert.Equal(t, c.expected[key], value, namespace)
			}
		}
	}
}

func TestControllerResourcesOptions(t *testing.T) {
	testcases := []struct {
		cpuOption      string
		memoryOption   string
		expectedCpu    string
		expectedMemory string
	}{
		{
			cpuOption:      "2",
			memoryOption:   "1G",
			expectedCpu:    "2",
			expectedMemory: "1G",
		},
		{
			cpuOption:   "0.8",
			expectedCpu: "800m",
		},
		{
			cpuOption:   "650m",
			expectedCpu: "650m",
		},
		{
			memoryOption:   "500M",
			expectedMemory: "500M",
		},
		{},
	}

	isCluster := *clusterRun

	for i, c := range testcases {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		namespace := fmt.Sprintf("controller-resources-%d", i+1)
		var cli *VanClient
		var err error
		if !isCluster {
			cli, err = newMockClient(namespace, "", "")
		} else {
			cli, err = NewClient(namespace, "", "")
		}
		assert.Check(t, err, namespace)

		_, err = kube.NewNamespace(namespace, cli.KubeClient)
		assert.Check(t, err, namespace)
		defer kube.DeleteNamespace(namespace, cli.KubeClient)

		opts := types.SiteConfigSpec{}
		opts.Ingress = "none"
		opts.EnableController = true
		opts.Controller.Cpu = c.cpuOption
		opts.Controller.Memory = c.memoryOption
		siteConfig, err := cli.SiteConfigCreate(ctx, opts)
		assert.Check(t, err, namespace)

		err = cli.RouterCreate(ctx, *siteConfig)
		assert.Check(t, err, namespace)

		deployment, err := cli.KubeClient.AppsV1().Deployments(namespace).Get("skupper-service-controller", metav1.GetOptions{})
		assert.Check(t, err, namespace)
		assert.Assert(t, len(deployment.Spec.Template.Spec.Containers) > 0, namespace)

		container := deployment.Spec.Template.Spec.Containers[0]
		if c.expectedMemory != "" {
			quantity := container.Resources.Requests[corev1.ResourceMemory]
			assert.Equal(t, c.expectedMemory, quantity.String())
		} else {
			_, ok := container.Resources.Requests[corev1.ResourceMemory]
			assert.Assert(t, !ok, namespace)
		}
		if c.expectedCpu != "" {
			quantity := container.Resources.Requests[corev1.ResourceCPU]
			assert.Equal(t, c.expectedCpu, quantity.String())
		} else {
			_, ok := container.Resources.Requests[corev1.ResourceCPU]
			assert.Assert(t, !ok, namespace)
		}
	}
}

func TestLabelandAnnotationOptions(t *testing.T) {
	deployments := []string{"skupper-router", "skupper-service-controller"}
	testcases := []struct {
		requestedLabels      map[string]string
		requestedAnnotations map[string]string
		expectedLabels       map[string]string
		expectedAnnotations  map[string]string
	}{
		{
			requestedLabels: map[string]string{
				"foo": "bar",
			},
			expectedLabels: map[string]string{
				"foo": "bar",
			},
		},
		{
			requestedAnnotations: map[string]string{
				"foo": "bar",
			},
			expectedAnnotations: map[string]string{
				"foo": "bar",
			},
		},
		{
			requestedLabels: map[string]string{
				"a": "b",
			},
			expectedLabels: map[string]string{
				"a": "b",
			},
			requestedAnnotations: map[string]string{
				"c": "d",
			},
			expectedAnnotations: map[string]string{
				"c": "d",
			},
		},
	}

	isCluster := *clusterRun

	for i, c := range testcases {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		namespace := fmt.Sprintf("router-labelling-%d", i+1)
		var cli *VanClient
		var err error
		if !isCluster {
			cli, err = newMockClient(namespace, "", "")
		} else {
			cli, err = NewClient(namespace, "", "")
		}
		assert.Check(t, err, namespace)

		_, err = kube.NewNamespace(namespace, cli.KubeClient)
		assert.Check(t, err, namespace)
		defer kube.DeleteNamespace(namespace, cli.KubeClient)

		opts := types.SiteConfigSpec{
			Labels:           c.requestedLabels,
			Annotations:      c.requestedAnnotations,
			Ingress:          "none",
			EnableController: true,
		}
		siteConfig, err := cli.SiteConfigCreate(ctx, opts)
		assert.Check(t, err, namespace)

		err = cli.RouterCreate(ctx, *siteConfig)
		assert.Check(t, err, namespace)

		for _, name := range deployments {
			deployment, err := cli.KubeClient.AppsV1().Deployments(namespace).Get(name, metav1.GetOptions{})
			assert.Check(t, err, namespace)

			for key, value := range c.expectedLabels {
				assert.Equal(t, deployment.Spec.Template.Labels[key], value, namespace)
			}
			for key, value := range c.expectedAnnotations {
				assert.Equal(t, deployment.Spec.Template.Annotations[key], value, namespace)
			}
		}
	}
}

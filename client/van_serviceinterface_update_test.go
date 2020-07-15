package client

import (
	"context"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/pkg/kube"
	"gotest.tools/assert"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/tools/cache"
)

var depReplicas int32 = 1
var tcpDeployment *appsv1.Deployment = &appsv1.Deployment{
	TypeMeta: metav1.TypeMeta{
		APIVersion: "apps/v1",
		Kind:       "Deployment",
	},
	ObjectMeta: metav1.ObjectMeta{
		Name: "tcp-go-echo",
	},
	Spec: appsv1.DeploymentSpec{
		Replicas: &depReplicas,
		Selector: &metav1.LabelSelector{
			MatchLabels: map[string]string{"application": "tcp-go-echo"},
		},
		Template: corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					"application": "tcp-go-echo",
				},
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name:            "tcp-go-echo",
						Image:           "quay.io/skupper/tcp-go-echo",
						ImagePullPolicy: corev1.PullIfNotPresent,
						Ports: []corev1.ContainerPort{
							{
								Name:          "http",
								Protocol:      corev1.ProtocolTCP,
								ContainerPort: 80,
							},
						},
					},
				},
			},
		},
	},
}

var ssReplicas int32 = 2
var tcpStatefulSet *appsv1.StatefulSet = &appsv1.StatefulSet{
	TypeMeta: metav1.TypeMeta{
		APIVersion: "apps/v1",
		Kind:       "StatefulSet",
	},
	ObjectMeta: metav1.ObjectMeta{
		Name: "tcp-go-echo-ss",
	},
	Spec: appsv1.StatefulSetSpec{
		Replicas: &ssReplicas,
		Selector: &metav1.LabelSelector{
			MatchLabels: map[string]string{"application": "tcp-go-echo-ss"},
		},
		Template: corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					"application": "tcp-go-echo-ss",
				},
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name:            "tcp-go-echo",
						Image:           "quay.io/skupper/tcp-go-echo",
						ImagePullPolicy: corev1.PullIfNotPresent,
						Ports: []corev1.ContainerPort{
							{
								Name:          "http",
								Protocol:      corev1.ProtocolTCP,
								ContainerPort: 80,
							},
						},
					},
				},
			},
		},
	},
}

var httpDeployment *appsv1.Deployment = &appsv1.Deployment{
	TypeMeta: metav1.TypeMeta{
		APIVersion: "apps/v1",
		Kind:       "Deployment",
	},
	ObjectMeta: metav1.ObjectMeta{
		Name: "nginx",
	},
	Spec: appsv1.DeploymentSpec{
		Replicas: &depReplicas,
		Selector: &metav1.LabelSelector{
			MatchLabels: map[string]string{"application": "nginx"},
		},
		Template: corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					"application": "nginx",
				},
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name:            "nginx",
						Image:           "nginxinc/nginx-unprivileged",
						ImagePullPolicy: corev1.PullIfNotPresent,
						Ports: []corev1.ContainerPort{
							{
								Name:          "http",
								Protocol:      corev1.ProtocolTCP,
								ContainerPort: 8080,
							},
						},
					},
				},
			},
		},
	},
}

func TestVanServiceInteraceUpdate(t *testing.T) {
	testcases := []struct {
		doc             string
		expectedError   string
		name            string
		port            int
		eventChannel    bool
		aggregate       string
		secretsExpected []string
		opts            []cmp.Option
	}{
		{
			doc:           "test one",
			expectedError: "",
			name:          "tcp-go-echo",
			port:          9091,
			eventChannel:  false,
			aggregate:     "",
			opts: []cmp.Option{
				trans,
			},
		},
		{
			doc:           "test two",
			expectedError: "",
			name:          "nginx",
			port:          0,
			eventChannel:  false,
			aggregate:     "json",
			opts: []cmp.Option{
				trans,
			},
		},
		{
			doc:           "test three",
			expectedError: "",
			name:          "nginx",
			port:          0,
			eventChannel:  false,
			aggregate:     "multipart",
			opts: []cmp.Option{
				trans,
			},
		},
		{
			doc:           "test three",
			expectedError: "",
			name:          "nginx",
			port:          0,
			eventChannel:  true,
			aggregate:     "",
			opts: []cmp.Option{
				trans,
			},
		},
		{
			doc:           "test four",
			expectedError: "Only one of aggregate and event-channel can be specified for a given service.",
			name:          "nginx",
			port:          0,
			eventChannel:  true,
			aggregate:     "json",
			opts: []cmp.Option{
				trans,
			},
		},
		{
			doc:           "test five",
			expectedError: "invalidstrategy is not a valid aggregation strategy. Choose 'json' or 'multipart'.",
			name:          "nginx",
			port:          0,
			eventChannel:  false,
			aggregate:     "invalidstrategy",
			opts: []cmp.Option{
				trans,
			},
		},
	}

	var namespace string = "van-serviceinterface-update"
	var cli *VanClient
	var err error
	if *clusterRun {
		cli, err = NewClient(namespace, "", "")
	} else {
		cli, err = newMockClient(namespace, "", "")
	}
	assert.Assert(t, err)

	_, err = kube.NewNamespace(namespace, cli.KubeClient)
	defer kube.DeleteNamespace(namespace, cli.KubeClient)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	svcsFound := []string{}
	svcsExpected := []string{"skupper-messaging", "skupper-internal", "skupper-controller", "nginx", "tcp-go-echo", "tcp-go-echo-ss"}

	informers := informers.NewSharedInformerFactoryWithOptions(cli.KubeClient, 0, informers.WithNamespace(namespace))
	svcInformer := informers.Core().V1().Services().Informer()
	svcInformer.AddEventHandler(&cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			svc := obj.(*corev1.Service)
			svcsFound = append(svcsFound, svc.Name)
		},
	})

	informers.Start(ctx.Done())
	cache.WaitForCacheSync(ctx.Done(), svcInformer.HasSynced)

	// create three service targets
	deployments := cli.KubeClient.AppsV1().Deployments(namespace)
	statefulSets := cli.KubeClient.AppsV1().StatefulSets(namespace)

	_, err = deployments.Create(tcpDeployment)
	assert.Assert(t, err)
	_, err = statefulSets.Create(tcpStatefulSet)
	assert.Assert(t, err)
	_, err = deployments.Create(httpDeployment)
	assert.Assert(t, err)

	err = cli.VanServiceInterfaceCreate(ctx, &types.ServiceInterface{
		Address:      "noaddress",
		Protocol:     "tcp",
		Port:         12345,
		EventChannel: false,
		Aggregate:    "",
	})
	assert.Error(t, err, "Skupper not initialised in van-serviceinterface-update")

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
	assert.Assert(t, err)

	// create three service definitions
	time.Sleep(time.Second * 1)
	err = cli.VanServiceInterfaceCreate(ctx, &types.ServiceInterface{
		Address:      "tcp-go-echo",
		Protocol:     "tcp",
		Port:         9090,
		EventChannel: false,
		Aggregate:    "",
	})
	assert.Assert(t, err)

	err = cli.VanServiceInterfaceCreate(ctx, &types.ServiceInterface{
		Address:      "nginx",
		Protocol:     "http",
		Port:         8080,
		EventChannel: false,
		Aggregate:    "",
	})
	assert.Assert(t, err)

	err = cli.VanServiceInterfaceCreate(ctx, &types.ServiceInterface{
		Address:      "tcp-go-echo-ss",
		Protocol:     "tcp",
		Port:         9090,
		EventChannel: false,
		Aggregate:    "",
	})
	assert.Assert(t, err)

	time.Sleep(time.Second * 1)

	// bind services to targets
	// TODO: could range on list if target type was not needed for bind
	si, err := cli.VanServiceInterfaceInspect(ctx, "tcp-go-echo")
	assert.Assert(t, err)
	err = cli.VanServiceInterfaceBind(ctx, si, "deployment", "tcp-go-echo", "tcp", 9090)
	assert.Assert(t, err)

	si, err = cli.VanServiceInterfaceInspect(ctx, "tcp-go-echo-ss")
	assert.Assert(t, err)
	err = cli.VanServiceInterfaceBind(ctx, si, "statefulset", "tcp-go-echo-ss", "tcp", 9090)
	assert.Assert(t, err)

	si, err = cli.VanServiceInterfaceInspect(ctx, "nginx")
	assert.Assert(t, err)
	// bad bind
	err = cli.VanServiceInterfaceBind(ctx, si, "deployment", "nginx2", "http", 8080)
	assert.Error(t, err, "Could not read deployment nginx2: deployments.apps \"nginx2\" not found")
	// good bind
	err = cli.VanServiceInterfaceBind(ctx, si, "deployment", "nginx", "http", 8080)
	assert.Assert(t, err)

	items, err := cli.VanServiceInterfaceList(ctx)
	assert.Assert(t, err)
	assert.Equal(t, len(items), 3)

	if *clusterRun {
		// this delay is for service-controller to update
		time.Sleep(time.Second * 5)
		if diff := cmp.Diff(svcsExpected, svcsFound, trans); diff != "" {
			t.Errorf("TestVanServiceInterfaceUpdate services mismatch (-want +got):\n%s", diff)
		}
	}

	for _, c := range testcases {
		ctx, cancel := context.WithCancel(ctx)
		defer cancel()

		si, err := cli.VanServiceInterfaceInspect(ctx, c.name)
		assert.Check(t, err, c.doc)

		if c.port != 0 {
			si.Port = c.port
		}
		if c.eventChannel != si.EventChannel {
			si.EventChannel = c.eventChannel
		}
		if c.aggregate != si.Aggregate {
			si.Aggregate = c.aggregate
		}
		err = cli.VanServiceInterfaceUpdate(ctx, si)
		if c.expectedError == "" {
			assert.Check(t, err, c.doc)
		} else {
			assert.Error(t, err, c.expectedError)
		}
	}

	// unbind targets
	err = cli.VanServiceInterfaceUnbind(ctx, "deployment", "tcp-go-echo", "tcp-go-echo", false)
	assert.Assert(t, err)

	err = cli.VanServiceInterfaceUnbind(ctx, "statefulset", "tcp-go-echo-ss", "tcp-go-echo-ss", false)
	assert.Assert(t, err)

	err = cli.VanServiceInterfaceUnbind(ctx, "deployment", "nginx", "nginx", false)
	assert.Assert(t, err)

	// and remove all defined services
	items, err = cli.VanServiceInterfaceList(ctx)
	assert.Assert(t, err)
	assert.Equal(t, len(items), 3)
	for _, si := range items {
		err = cli.VanServiceInterfaceRemove(ctx, si.Address)
		assert.Assert(t, err)
	}

	items, err = cli.VanServiceInterfaceList(ctx)
	assert.Assert(t, err)
	assert.Equal(t, len(items), 0)

}

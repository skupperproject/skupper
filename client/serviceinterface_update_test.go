package client

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/skupperproject/skupper/pkg/certs"

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
		Labels: map[string]string{
			"app": "tcp-go-echo",
		},
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
								Name:          "tcp-go-echo",
								Protocol:      corev1.ProtocolTCP,
								ContainerPort: 9090,
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
		Labels: map[string]string{
			"app": "tcp-go-echo-ss",
		},
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
								Name:          "tcp-go-echo",
								Protocol:      corev1.ProtocolTCP,
								ContainerPort: 9090,
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
		ports           []int
		eventChannel    bool
		aggregate       string
		newLabels       map[string]string
		secretsExpected []string
		opts            []cmp.Option
	}{
		{
			doc:           "tcp-go-echo - change port",
			expectedError: "",
			name:          "tcp-go-echo",
			ports:         []int{9091},
			eventChannel:  false,
			aggregate:     "",
			opts: []cmp.Option{
				trans,
			},
		},
		{
			doc:           "tcp-go-echo-ss - change port and add labels",
			expectedError: "",
			name:          "tcp-go-echo-ss",
			ports:         []int{9091},
			newLabels: map[string]string{
				"app": "tcp-go-echo-ss-modified",
			},
			eventChannel: false,
			aggregate:    "",
			opts: []cmp.Option{
				trans,
			},
		},
		{
			doc:           "nginx - json aggregation strategy",
			expectedError: "",
			name:          "nginx",
			ports:         []int{},
			eventChannel:  false,
			aggregate:     "json",
			newLabels: map[string]string{
				"app": "nginx",
			},
			opts: []cmp.Option{
				trans,
			},
		},
		{
			doc:           "nginx - multipart aggregation strategy",
			expectedError: "",
			name:          "nginx",
			ports:         []int{},
			eventChannel:  false,
			aggregate:     "multipart",
			opts: []cmp.Option{
				trans,
			},
		},
		{
			doc:           "nginx - no aggregation strategy",
			expectedError: "",
			name:          "nginx",
			ports:         []int{},
			eventChannel:  true,
			aggregate:     "",
			opts: []cmp.Option{
				trans,
			},
		},
		{
			doc:           "nginx - error eventChannel with aggregation",
			expectedError: "Only one of aggregate and event-channel can be specified for a given service.",
			name:          "nginx",
			ports:         []int{},
			eventChannel:  true,
			aggregate:     "json",
			opts: []cmp.Option{
				trans,
			},
		},
		{
			doc:           "nginx - invalid aggregation strategy",
			expectedError: "invalidstrategy is not a valid aggregation strategy. Choose 'json' or 'multipart'.",
			name:          "nginx",
			ports:         []int{},
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
	svcsExpected := []string{types.LocalTransportServiceName, types.TransportServiceName, types.ControllerServiceName, "nginx", "tcp-go-echo", "tcp-go-echo-ss"}

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

	err = cli.ServiceInterfaceCreate(ctx, &types.ServiceInterface{
		Address:      "noaddress",
		Protocol:     "tcp",
		Ports:        []int{12345},
		EventChannel: false,
		Aggregate:    "",
	})
	assert.ErrorContains(t, err, "Skupper is not enabled in namespace 'van-serviceinterface-update'")

	err = cli.RouterCreate(ctx, types.SiteConfig{
		Spec: types.SiteConfigSpec{
			SkupperName:       "skupper",
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
	assert.Assert(t, err)

	// wait for skupper router component to be running
	pods, err := kube.GetPods("skupper.io/component=router", namespace, cli.KubeClient)
	assert.Assert(t, err)
	for _, pod := range pods {
		_, err := kube.WaitForPodStatus(namespace, cli.KubeClient, pod.Name, corev1.PodRunning, time.Second*180, time.Second*5)
		assert.Assert(t, err)
	}

	// create three service definitions
	siteCA, err := cli.KubeClient.CoreV1().Secrets(cli.Namespace).Get(types.ServiceCaSecret, metav1.GetOptions{})
	assert.Assert(t, err)

	err = cli.ServiceInterfaceCreate(ctx, &types.ServiceInterface{
		Address:      "tcp-go-echo",
		Protocol:     "tcp",
		Ports:        []int{9090},
		EventChannel: false,
		Aggregate:    "",
	})
	assert.Assert(t, err)
	serviceCert := certs.GenerateSecret("skupper-tcp-go-echo", "tcp-go-echo", "tcp-go-echo", siteCA)
	_, err = cli.KubeClient.CoreV1().Secrets(cli.Namespace).Create(&serviceCert)
	assert.Assert(t, err)

	err = cli.ServiceInterfaceCreate(ctx, &types.ServiceInterface{
		Address:      "nginx",
		Protocol:     "http",
		Ports:        []int{8080, 9090},
		EventChannel: false,
		Aggregate:    "",
	})
	assert.Assert(t, err)

	err = cli.ServiceInterfaceCreate(ctx, &types.ServiceInterface{
		Address:      "tcp-go-echo-ss",
		Protocol:     "tcp",
		Ports:        []int{9090},
		EventChannel: false,
		Aggregate:    "",
		Labels: map[string]string{
			"service": "tcp-go-echo-ss",
		},
	})
	assert.Assert(t, err)
	serviceCert = certs.GenerateSecret("skupper-tcp-go-echo-ss", "tcp-go-echo-ss", "tcp-go-echo-ss", siteCA)
	_, err = cli.KubeClient.CoreV1().Secrets(cli.Namespace).Create(&serviceCert)
	assert.Assert(t, err)

	// bind services to targets
	// TODO: could range on list if target type was not needed for bind
	si, err := cli.ServiceInterfaceInspect(ctx, "tcp-go-echo")
	assert.Assert(t, err)
	err = cli.ServiceInterfaceBind(ctx, si, "deployment", "tcp-go-echo", "tcp", map[int]int{9090: 9090})
	assert.Assert(t, err)

	si, err = cli.ServiceInterfaceInspect(ctx, "tcp-go-echo-ss")
	assert.Assert(t, err)
	err = cli.ServiceInterfaceBind(ctx, si, "statefulset", "tcp-go-echo-ss", "tcp", map[int]int{9090: 9090})
	assert.Assert(t, err)

	si, err = cli.ServiceInterfaceInspect(ctx, "nginx")
	assert.Assert(t, err)
	// bad bind
	err = cli.ServiceInterfaceBind(ctx, si, "deployment", "nginx2", "http", map[int]int{8080: 8080, 9090: 8080})
	assert.Error(t, err, "Could not read deployment nginx2: deployments.apps \"nginx2\" not found")
	// good bind
	err = cli.ServiceInterfaceBind(ctx, si, "deployment", "nginx", "http", map[int]int{8080: 8080, 9090: 8080})
	assert.Assert(t, err)

	items, err := cli.ServiceInterfaceList(ctx)
	assert.Assert(t, err)
	assert.Equal(t, len(items), 3)

	if *clusterRun {
		// this delay is for service-controller to update
		time.Sleep(time.Second * 30)
		if diff := cmp.Diff(svcsExpected, svcsFound, trans); diff != "" {
			t.Errorf("TestServiceInterfaceUpdate services mismatch (-want +got):\n%s", diff)
		}
	}

	for _, c := range testcases {
		ctx, cancel := context.WithCancel(ctx)
		defer cancel()

		si, err := cli.ServiceInterfaceInspect(ctx, c.name)
		assert.Check(t, err, c.doc)

		if len(c.ports) != 0 {
			si.Ports = c.ports
		}
		if c.eventChannel != si.EventChannel {
			si.EventChannel = c.eventChannel
		}
		if c.aggregate != si.Aggregate {
			si.Aggregate = c.aggregate
		}
		if len(c.newLabels) > 0 {
			si.Labels = c.newLabels
		}
		err = cli.ServiceInterfaceUpdate(ctx, si)
		if c.expectedError == "" {
			assert.Check(t, err, c.doc)
		} else {
			assert.Error(t, err, c.expectedError)
		}
	}

	// now check updates as expected
	si, err = cli.ServiceInterfaceInspect(ctx, "tcp-go-echo")
	assert.Assert(t, err)
	assert.Equal(t, si.Protocol, "tcp")
	assert.Assert(t, reflect.DeepEqual(si.Ports, []int{9091}))

	si, err = cli.ServiceInterfaceInspect(ctx, "nginx")
	assert.Assert(t, err)
	assert.Equal(t, si.Protocol, "http")
	assert.Equal(t, si.EventChannel, true)

	// unbind targets
	err = cli.ServiceInterfaceUnbind(ctx, "deployment", "tcp-go-echo", "tcp-go-echo", false)
	assert.Assert(t, err)

	err = cli.ServiceInterfaceUnbind(ctx, "statefulset", "tcp-go-echo-ss", "tcp-go-echo-ss", false)
	assert.Assert(t, err)

	err = cli.ServiceInterfaceUnbind(ctx, "deployment", "nginx", "nginx", false)
	assert.Assert(t, err)

	// and remove all defined services
	items, err = cli.ServiceInterfaceList(ctx)
	assert.Assert(t, err)
	assert.Equal(t, len(items), 3)
	for _, si := range items {
		err = cli.ServiceInterfaceRemove(ctx, si.Address)
		assert.Assert(t, err)
	}

	items, err = cli.ServiceInterfaceList(ctx)
	assert.Assert(t, err)
	assert.Equal(t, len(items), 0)

	_, err = cli.KubeClient.CoreV1().Secrets(cli.Namespace).Get("skupper-nginx", metav1.GetOptions{})
	if err != nil {
		assert.Equal(t, err.Error(), "secrets \"skupper-nginx\" not found")
	}

	_, err = cli.KubeClient.CoreV1().Secrets(cli.Namespace).Get("skupper-tcp-go-echo", metav1.GetOptions{})
	if err != nil {
		assert.Equal(t, err.Error(), "secrets \"skupper-tcp-go-echo\" not found")
	}

	_, err = cli.KubeClient.CoreV1().Secrets(cli.Namespace).Get("skupper-tcp-go-echo-ss", metav1.GetOptions{})
	if err != nil {
		assert.Equal(t, err.Error(), "secrets \"skupper-tcp-go-echo-ss\" not found")
	}

}

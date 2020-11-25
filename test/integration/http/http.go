package http

import (
	"context"
	"fmt"
	"testing"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/test/utils/base"
	"github.com/skupperproject/skupper/test/utils/constants"
	"github.com/skupperproject/skupper/test/utils/k8s"
	"gotest.tools/assert"

	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type HttpClusterTestRunner struct {
	base.ClusterTestRunnerBase
}

func int32Ptr(i int32) *int32 { return &i }

var httpbinDep *appsv1.Deployment = &appsv1.Deployment{
	TypeMeta: metav1.TypeMeta{
		APIVersion: "apps/v1",
		Kind:       "Deployment",
	},
	ObjectMeta: metav1.ObjectMeta{
		Name: "httpbin",
	},
	Spec: appsv1.DeploymentSpec{
		Replicas: int32Ptr(1),
		Selector: &metav1.LabelSelector{
			MatchLabels: map[string]string{"application": "httpbin"},
		},
		Template: apiv1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					"application": "httpbin",
				},
			},
			Spec: apiv1.PodSpec{
				Containers: []apiv1.Container{
					{
						Name:            "httpbin",
						Image:           "docker.io/kennethreitz/httpbin",
						ImagePullPolicy: apiv1.PullIfNotPresent,
						Ports: []apiv1.ContainerPort{
							{
								Name:          "http",
								Protocol:      apiv1.ProtocolTCP,
								ContainerPort: 8080,
							},
						},
						Command: []string{
							"gunicorn",
							"-b",
							"0.0.0.0:8080",
							"httpbin:app",
							"-k",
							"gevent",
						},
					},
				},
			},
		},
	},
}

var nghttp2Dep *appsv1.Deployment = &appsv1.Deployment{
	TypeMeta: metav1.TypeMeta{
		APIVersion: "apps/v1",
		Kind:       "Deployment",
	},
	ObjectMeta: metav1.ObjectMeta{
		Name: "nghttp2",
	},
	Spec: appsv1.DeploymentSpec{
		Replicas: int32Ptr(1),
		Selector: &metav1.LabelSelector{
			MatchLabels: map[string]string{"application": "nghttp2"},
		},
		Template: apiv1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					"application": "nghttp2",
				},
			},
			Spec: apiv1.PodSpec{
				Containers: []apiv1.Container{
					{
						Name:            "nghttp2",
						Image:           "docker.io/svagi/nghttp2",
						ImagePullPolicy: apiv1.PullIfNotPresent,
						Ports: []apiv1.ContainerPort{
							{
								Name:          "nghttp2",
								Protocol:      apiv1.ProtocolTCP,
								ContainerPort: 8443,
							},
						},
						//docker run -p 8443:8443 --network my-bridge-network -it svagi/nghttp2 nghttpx  -f"0.0.0.0,8443;no-tls" -b172.18.0.2,80 -L INFO
						Command: []string{
							"nghttpx",
							"-f0.0.0.0,8443;no-tls",
							"-bhttpbin,8080",
							"-L",
							"INFO",
						},
					},
				},
			},
		},
	},
}

func (r *HttpClusterTestRunner) RunTests(ctx context.Context, t *testing.T) {
	pubCluster1, err := r.GetPublicContext(1)
	assert.Assert(t, err)

	_, err = k8s.WaitForSkupperServiceToBeCreatedAndReadyToUse(pubCluster1.Namespace, pubCluster1.VanClient.KubeClient, "httpbin")
	assert.Assert(t, err)

	_, err = k8s.WaitForSkupperServiceToBeCreatedAndReadyToUse(pubCluster1.Namespace, pubCluster1.VanClient.KubeClient, "nghttp2")
	assert.Assert(t, err)

	runJob := func(cc *base.ClusterContext, jobName, testName string) {
		t.Helper()
		jobCmd := []string{"/app/http_test", "-test.run", testName}

		_, err = k8s.CreateTestJob(cc.Namespace, cc.VanClient.KubeClient, jobName, jobCmd)
		assert.Assert(t, err)
	}

	waitJob := func(cc *base.ClusterContext, jobName string) {
		t.Helper()
		job, err := k8s.WaitForJob(cc.Namespace, cc.VanClient.KubeClient, jobName, constants.ImagePullingAndResourceCreationTimeout)
		assert.Assert(t, err)
		cc.KubectlExec("logs job/" + jobName)
		k8s.AssertJob(t, job)
	}

	runJob(pubCluster1, "http1", "TestHttpJob")
	waitJob(pubCluster1, "http1")

	runJob(pubCluster1, "http2", "TestHttp2Job")
	waitJob(pubCluster1, "http2")
}

func (r *HttpClusterTestRunner) Setup(ctx context.Context, t *testing.T) {
	prv1Cluster, err := r.GetPrivateContext(1)
	assert.Assert(t, err)

	err = base.SetupSimplePublicPrivateAndConnect(ctx, &r.ClusterTestRunnerBase, "http")
	assert.Assert(t, err)

	privateDeploymentsClient := prv1Cluster.VanClient.KubeClient.AppsV1().Deployments(prv1Cluster.Namespace)

	createDeploymentInPrivateSite := func(dep *appsv1.Deployment) {
		t.Helper()
		fmt.Println("Creating httpbin deployment...")
		result, err := privateDeploymentsClient.Create(dep)
		assert.Assert(t, err)

		fmt.Printf("Created deployment %q.\n", result.GetObjectMeta().GetName())
	}

	createDeploymentInPrivateSite(httpbinDep)
	createDeploymentInPrivateSite(nghttp2Dep)

	service := types.ServiceInterface{
		Address:  "httpbin",
		Protocol: "http",
		Port:     8080,
	}

	err = prv1Cluster.VanClient.ServiceInterfaceCreate(ctx, &service)
	assert.Assert(t, err)

	err = prv1Cluster.VanClient.ServiceInterfaceBind(ctx, &service, "deployment", "httpbin", "http", 0)
	assert.Assert(t, err)

	http2service := types.ServiceInterface{
		Address:  "nghttp2",
		Protocol: "http2",
		Port:     8443,
	}

	err = prv1Cluster.VanClient.ServiceInterfaceCreate(ctx, &http2service)
	assert.Assert(t, err)

	err = prv1Cluster.VanClient.ServiceInterfaceBind(ctx, &http2service, "deployment", "nghttp2", "http2", 0)
	assert.Assert(t, err)

	http21service := types.ServiceInterface{
		Address:  "nghttp1",
		Protocol: "http",
		Port:     8443,
	}

	err = prv1Cluster.VanClient.ServiceInterfaceCreate(ctx, &http21service)
	assert.Assert(t, err)

	err = prv1Cluster.VanClient.ServiceInterfaceBind(ctx, &http21service, "deployment", "nghttp2", "http", 0)
	assert.Assert(t, err)

}

func (r *HttpClusterTestRunner) Run(ctx context.Context, t *testing.T) {
	defer base.TearDownSimplePublicAndPrivate(&r.ClusterTestRunnerBase)
	r.Setup(ctx, t)
	r.RunTests(ctx, t)
}

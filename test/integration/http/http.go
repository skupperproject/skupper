package http

import (
	"context"
	"fmt"
	"testing"

	"github.com/prometheus/common/log"
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

func sendReceive(servAddr string) {
	return
}

func (r *HttpClusterTestRunner) RunTests(ctx context.Context, t *testing.T) {
	//TODO https://github.com/skupperproject/skupper/issues/95
	//all this hardcoded sleeps must be fixed, probably along with #95
	//for now I am just keeping them in the same values that we are using
	//for tcp_echo test, since in case of reducing test may fail
	//intermitently
	pubCluster1, err := r.GetPublicContext(1)
	assert.Assert(t, err)

	_, err = k8s.WaitForSkupperServiceToBeCreatedAndReadyToUse(pubCluster1.Namespace, pubCluster1.VanClient.KubeClient, "httpbin")
	assert.Assert(t, err)

	jobName := "http"
	jobCmd := []string{"/app/http_test", "-test.run", "Job"}

	_, err = k8s.CreateTestJob(pubCluster1.Namespace, pubCluster1.VanClient.KubeClient, jobName, jobCmd)
	assert.Assert(t, err)

	job, err := k8s.WaitForJob(pubCluster1.Namespace, pubCluster1.VanClient.KubeClient, jobName, constants.ImagePullingAndResourceCreationTimeout)
	assert.Assert(t, err)
	k8s.AssertJob(t, job)
}

func (r *HttpClusterTestRunner) Setup(ctx context.Context, t *testing.T) {
	prv1Cluster, err := r.GetPrivateContext(1)
	assert.Assert(t, err)

	err = base.SetupSimplePublicPrivateAndConnect(ctx, &r.ClusterTestRunnerBase, "http")
	assert.Assert(t, err)

	privateDeploymentsClient := prv1Cluster.VanClient.KubeClient.AppsV1().Deployments(prv1Cluster.Namespace)

	fmt.Println("Creating deployment...")
	result, err := privateDeploymentsClient.Create(httpbinDep)
	assert.Assert(t, err)

	fmt.Printf("Created deployment %q.\n", result.GetObjectMeta().GetName())

	service := types.ServiceInterface{
		Address:  "httpbin",
		Protocol: "http",
		Port:     8080,
	}

	err = prv1Cluster.VanClient.ServiceInterfaceCreate(ctx, &service)
	assert.Assert(t, err)

	err = prv1Cluster.VanClient.ServiceInterfaceBind(ctx, &service, "deployment", "httpbin", "http", 0)
	assert.Assert(t, err)
}

func (r *HttpClusterTestRunner) Run(ctx context.Context, t *testing.T) {
	defer base.TearDownSimplePublicAndPrivate(&r.ClusterTestRunnerBase)
	r.Setup(ctx, t)
	r.RunTests(ctx, t)
}

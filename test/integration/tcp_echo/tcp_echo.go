package tcp_echo

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/skupperproject/skupper/test/utils/base"
	"github.com/skupperproject/skupper/test/utils/constants"
	"github.com/skupperproject/skupper/test/utils/k8s"

	"github.com/skupperproject/skupper/api/types"
	"gotest.tools/assert"

	appsv1 "k8s.io/api/apps/v1"

	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type TcpEchoClusterTestRunner struct {
	base.ClusterTestRunnerBase
}

func int32Ptr(i int32) *int32 { return &i }

var deployment *appsv1.Deployment = &appsv1.Deployment{
	TypeMeta: metav1.TypeMeta{
		APIVersion: "apps/v1",
		Kind:       "Deployment",
	},
	ObjectMeta: metav1.ObjectMeta{
		Name: "tcp-go-echo",
	},
	Spec: appsv1.DeploymentSpec{
		Replicas: int32Ptr(1),
		Selector: &metav1.LabelSelector{
			MatchLabels: map[string]string{"application": "tcp-go-echo"},
		},
		Template: apiv1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					"application": "tcp-go-echo",
				},
			},
			Spec: apiv1.PodSpec{
				Containers: []apiv1.Container{
					{
						Name:            "tcp-go-echo",
						Image:           "quay.io/skupper/tcp-go-echo",
						ImagePullPolicy: apiv1.PullIfNotPresent,
						Ports: []apiv1.ContainerPort{
							{
								Name:          "http",
								Protocol:      apiv1.ProtocolTCP,
								ContainerPort: 80,
							},
						},
					},
				},
			},
		},
	},
}

func (r *TcpEchoClusterTestRunner) RunTests(ctx context.Context, t *testing.T) {

	//XXX
	endTime := time.Now().Add(constants.ImagePullingAndResourceCreationTimeout)

	pub1Cluster, err := r.GetPublicContext(1)
	assert.Assert(t, err)

	prv1Cluster, err := r.GetPrivateContext(1)
	assert.Assert(t, err)

	_, err = k8s.WaitForSkupperServiceToBeCreatedAndReadyToUse(pub1Cluster.Namespace, pub1Cluster.VanClient.KubeClient, "tcp-go-echo")
	assert.Assert(t, err)

	_, err = k8s.WaitForSkupperServiceToBeCreatedAndReadyToUse(prv1Cluster.Namespace, prv1Cluster.VanClient.KubeClient, "tcp-go-echo")
	assert.Assert(t, err)

	jobName := "tcp-echo"
	jobCmd := []string{"/app/tcp_echo_test", "-test.run", "Job"}

	//Note here we are executing the same test but, in two different
	//namespaces (or clusters), the same service must exist in both clusters
	//because of the skupper connections and the "skupper exposed"
	//deployment.
	_, err = k8s.CreateTestJob(pub1Cluster.Namespace, pub1Cluster.VanClient.KubeClient, jobName, jobCmd)
	assert.Assert(t, err)

	_, err = k8s.CreateTestJob(prv1Cluster.Namespace, prv1Cluster.VanClient.KubeClient, jobName, jobCmd)
	assert.Assert(t, err)

	endTime = time.Now().Add(constants.ImagePullingAndResourceCreationTimeout)

	job, err := k8s.WaitForJob(pub1Cluster.Namespace, pub1Cluster.VanClient.KubeClient, jobName, endTime.Sub(time.Now()))
	assert.Assert(t, err)
	pub1Cluster.KubectlExec("logs job/" + jobName)
	k8s.AssertJob(t, job)

	job, err = k8s.WaitForJob(prv1Cluster.Namespace, prv1Cluster.VanClient.KubeClient, jobName, endTime.Sub(time.Now()))
	assert.Assert(t, err)
	prv1Cluster.KubectlExec("logs job/" + jobName)
	k8s.AssertJob(t, job)
}

func (r *TcpEchoClusterTestRunner) Setup(ctx context.Context, t *testing.T) {
	pub1Cluster, err := r.GetPublicContext(1)
	assert.Assert(t, err)

	err = base.SetupSimplePublicPrivateAndConnect(ctx, &r.ClusterTestRunnerBase, "tcp_echo")
	assert.Assert(t, err)

	publicDeploymentsClient := pub1Cluster.VanClient.KubeClient.AppsV1().Deployments(pub1Cluster.Namespace)

	fmt.Println("Creating deployment...")
	result, err := publicDeploymentsClient.Create(deployment)
	assert.Assert(t, err)

	fmt.Printf("Created deployment %q.\n", result.GetObjectMeta().GetName())

	fmt.Printf("Listing deployments in namespace %q:\n", pub1Cluster.Namespace)
	list, err := publicDeploymentsClient.List(metav1.ListOptions{})
	assert.Assert(t, err)

	for _, d := range list.Items {
		fmt.Printf(" * %s (%d replicas)\n", d.Name, *d.Spec.Replicas)
	}

	service := types.ServiceInterface{
		Address:  "tcp-go-echo",
		Protocol: "tcp",
		Port:     9090,
	}
	err = pub1Cluster.VanClient.ServiceInterfaceCreate(ctx, &service)
	assert.Assert(t, err)

	err = pub1Cluster.VanClient.ServiceInterfaceBind(ctx, &service, "deployment", "tcp-go-echo", "tcp", 0)
	assert.Assert(t, err)
}

func (r *TcpEchoClusterTestRunner) Run(ctx context.Context, t *testing.T) {
	defer base.TearDownSimplePublicAndPrivate(&r.ClusterTestRunnerBase)
	r.Setup(ctx, t)
	r.RunTests(ctx, t)
}

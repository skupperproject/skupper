package tcp_echo

import (
	"context"
	"fmt"
	"time"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/test/cluster"
	"gotest.tools/assert"

	appsv1 "k8s.io/api/apps/v1"

	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type TcpEchoClusterTestRunner struct {
	cluster.ClusterTestRunnerBase
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

func (r *TcpEchoClusterTestRunner) RunTests(ctx context.Context) {

	//XXX
	endTime := time.Now().Add(cluster.ImagePullingAndResourceCreationTimeout)

	_, err := cluster.WaitForSkupperServiceToBeCreatedAndReadyToUse(r.Pub1Cluster, "tcp-go-echo")
	assert.Assert(r.T, err)

	_, err = cluster.WaitForSkupperServiceToBeCreatedAndReadyToUse(r.Priv1Cluster, "tcp-go-echo")
	assert.Assert(r.T, err)

	jobName := "tcp-echo"
	jobCmd := []string{"/app/tcp_echo_test", "-test.run", "Job"}

	//Note here we are executing the same test but, in two different
	//namespaces (or clusters), the same service must exist in both clusters
	//because of the skupper connections and the "skupper exposed"
	//deployment.
	_, err = r.Pub1Cluster.CreateTestJob(jobName, jobCmd)
	assert.Assert(r.T, err)

	_, err = r.Priv1Cluster.CreateTestJob(jobName, jobCmd)
	assert.Assert(r.T, err)

	endTime = time.Now().Add(cluster.ImagePullingAndResourceCreationTimeout)

	job, err := r.Pub1Cluster.WaitForJob(jobName, endTime.Sub(time.Now()))
	assert.Assert(r.T, err)
	cluster.AssertJob(r.T, job)

	job, err = r.Priv1Cluster.WaitForJob(jobName, endTime.Sub(time.Now()))
	assert.Assert(r.T, err)
	cluster.AssertJob(r.T, job)
}

func (r *TcpEchoClusterTestRunner) Setup(ctx context.Context) {
	var err error
	err = r.Pub1Cluster.CreateNamespace()
	assert.Assert(r.T, err)

	err = r.Priv1Cluster.CreateNamespace()
	assert.Assert(r.T, err)

	publicDeploymentsClient := r.Pub1Cluster.VanClient.KubeClient.AppsV1().Deployments(r.Pub1Cluster.CurrentNamespace)

	fmt.Println("Creating deployment...")
	result, err := publicDeploymentsClient.Create(deployment)
	assert.Assert(r.T, err)

	fmt.Printf("Created deployment %q.\n", result.GetObjectMeta().GetName())

	fmt.Printf("Listing deployments in namespace %q:\n", r.Pub1Cluster.CurrentNamespace)
	list, err := publicDeploymentsClient.List(metav1.ListOptions{})
	assert.Assert(r.T, err)

	for _, d := range list.Items {
		fmt.Printf(" * %s (%d replicas)\n", d.Name, *d.Spec.Replicas)
	}

	routerCreateOpts := types.SiteConfig{
		Spec: types.SiteConfigSpec{
			SkupperName:       "",
			IsEdge:            false,
			EnableController:  true,
			EnableServiceSync: true,
			EnableConsole:     false,
			AuthMode:          types.ConsoleAuthModeUnsecured,
			User:              "nicob?",
			Password:          "nopasswordd",
			ClusterLocal:      false,
			Replicas:          1,
		},
	}

	routerCreateOpts.Spec.SkupperNamespace = r.Pub1Cluster.CurrentNamespace
	r.Pub1Cluster.VanClient.RouterCreate(ctx, routerCreateOpts)

	service := types.ServiceInterface{
		Address:  "tcp-go-echo",
		Protocol: "tcp",
		Port:     9090,
	}
	err = r.Pub1Cluster.VanClient.ServiceInterfaceCreate(ctx, &service)
	assert.Assert(r.T, err)

	err = r.Pub1Cluster.VanClient.ServiceInterfaceBind(ctx, &service, "deployment", "tcp-go-echo", "tcp", 0)
	assert.Assert(r.T, err)

	err = r.Pub1Cluster.VanClient.ConnectorTokenCreateFile(ctx, types.DefaultVanName, "/tmp/public_secret.yaml")
	assert.Assert(r.T, err)

	routerCreateOpts.Spec.SkupperNamespace = r.Priv1Cluster.CurrentNamespace
	err = r.Priv1Cluster.VanClient.RouterCreate(ctx, routerCreateOpts)

	var connectorCreateOpts types.ConnectorCreateOptions = types.ConnectorCreateOptions{
		SkupperNamespace: r.Priv1Cluster.CurrentNamespace,
		Name:             "",
		Cost:             0,
	}
	r.Priv1Cluster.VanClient.ConnectorCreateFromFile(ctx, "/tmp/public_secret.yaml", connectorCreateOpts)
}

func (r *TcpEchoClusterTestRunner) TearDown(ctx context.Context) {
	r.Pub1Cluster.DeleteNamespace()
	r.Priv1Cluster.DeleteNamespace()
}

func (r *TcpEchoClusterTestRunner) Run(ctx context.Context) {
	defer r.TearDown(ctx)
	r.Setup(ctx)
	r.RunTests(ctx)
}

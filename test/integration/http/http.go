package http

import (
	"context"
	"fmt"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/test/cluster"
	"gotest.tools/assert"

	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type HttpClusterTestRunner struct {
	cluster.ClusterTestRunnerBase
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
								ContainerPort: 80,
							},
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

func (r *HttpClusterTestRunner) RunTests(ctx context.Context) {
	//TODO https://github.com/skupperproject/skupper/issues/95
	//all this hardcoded sleeps must be fixed, probably along with #95
	//for now I am just keeping them in the same values that we are using
	//for tcp_echo test, since in case of reducing test may fail
	//intermitently

	_, err := cluster.WaitForSkupperServiceToBeCreatedAndReadyToUse(r.Pub1Cluster, "httpbin")
	assert.Assert(r.T, err)

	jobName := "http"
	jobCmd := []string{"/app/http_test", "-test.run", "Job"}

	_, err = r.Pub1Cluster.CreateTestJob(jobName, jobCmd)
	assert.Assert(r.T, err)

	job, err := r.Pub1Cluster.WaitForJob(jobName, cluster.ImagePullingAndResourceCreationTimeout)
	assert.Assert(r.T, err)
	cluster.AssertJob(r.T, job)
}

func (r *HttpClusterTestRunner) Setup(ctx context.Context) {
	var err error
	err = r.Pub1Cluster.CreateNamespace()
	assert.Assert(r.T, err)

	err = r.Priv1Cluster.CreateNamespace()
	assert.Assert(r.T, err)

	privateDeploymentsClient := r.Priv1Cluster.VanClient.KubeClient.AppsV1().Deployments(r.Priv1Cluster.CurrentNamespace)

	fmt.Println("Creating deployment...")
	result, err := privateDeploymentsClient.Create(httpbinDep)
	assert.Assert(r.T, err)

	fmt.Printf("Created deployment %q.\n", result.GetObjectMeta().GetName())

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

	routerCreateOpts.Spec.SkupperNamespace = r.Priv1Cluster.CurrentNamespace
	r.Priv1Cluster.VanClient.RouterCreate(ctx, routerCreateOpts)

	service := types.ServiceInterface{
		Address:  "httpbin",
		Protocol: "http",
		Port:     80,
	}

	err = r.Priv1Cluster.VanClient.ServiceInterfaceCreate(ctx, &service)
	assert.Assert(r.T, err)

	err = r.Priv1Cluster.VanClient.ServiceInterfaceBind(ctx, &service, "deployment", "httpbin", "http", 0)
	assert.Assert(r.T, err)

	routerCreateOpts.Spec.SkupperNamespace = r.Pub1Cluster.CurrentNamespace
	err = r.Pub1Cluster.VanClient.RouterCreate(ctx, routerCreateOpts)

	err = r.Pub1Cluster.VanClient.ConnectorTokenCreateFile(ctx, types.DefaultVanName, "/tmp/public_secret.yaml")
	assert.Assert(r.T, err)

	var connectorCreateOpts types.ConnectorCreateOptions = types.ConnectorCreateOptions{
		SkupperNamespace: r.Priv1Cluster.CurrentNamespace,
		Name:             "",
		Cost:             0,
	}
	r.Priv1Cluster.VanClient.ConnectorCreateFromFile(ctx, "/tmp/public_secret.yaml", connectorCreateOpts)
}

func (r *HttpClusterTestRunner) TearDown(ctx context.Context) {
	r.Pub1Cluster.DeleteNamespace()
	r.Priv1Cluster.DeleteNamespace()
}

func (r *HttpClusterTestRunner) Run(ctx context.Context) {
	defer r.TearDown(ctx)
	r.Setup(ctx)
	r.RunTests(ctx)
}

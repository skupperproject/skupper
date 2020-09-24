package http

import (
	"context"
	"fmt"

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

func (r *HttpClusterTestRunner) RunTests(ctx context.Context) {
	//TODO https://github.com/skupperproject/skupper/issues/95
	//all this hardcoded sleeps must be fixed, probably along with #95
	//for now I am just keeping them in the same values that we are using
	//for tcp_echo test, since in case of reducing test may fail
	//intermitently
	pubCluster1 := r.GetPublicContext(1)
	_, err := k8s.WaitForSkupperServiceToBeCreatedAndReadyToUse(pubCluster1.Namespace, pubCluster1.VanClient.KubeClient, "httpbin")
	assert.Assert(r.T, err)

	jobName := "http"
	jobCmd := []string{"/app/http_test", "-test.run", "Job"}

	_, err = k8s.CreateTestJob(pubCluster1.Namespace, pubCluster1.VanClient.KubeClient, jobName, jobCmd)
	assert.Assert(r.T, err)

	job, err := k8s.WaitForJob(pubCluster1.Namespace, pubCluster1.VanClient.KubeClient, jobName, constants.ImagePullingAndResourceCreationTimeout)
	assert.Assert(r.T, err)
	k8s.AssertJob(r.T, job)
}

func (r *HttpClusterTestRunner) Setup(ctx context.Context) {
	var err error
	pub1Cluster := r.GetPublicContext(1)
	prv1Cluster := r.GetPrivateContext(1)

	err = pub1Cluster.CreateNamespace()
	assert.Assert(r.T, err)

	err = prv1Cluster.CreateNamespace()
	assert.Assert(r.T, err)

	privateDeploymentsClient := prv1Cluster.VanClient.KubeClient.AppsV1().Deployments(prv1Cluster.Namespace)

	fmt.Println("Creating deployment...")
	result, err := privateDeploymentsClient.Create(httpbinDep)
	assert.Assert(r.T, err)

	fmt.Printf("Created deployment %q.\n", result.GetObjectMeta().GetName())

	routerCreateSpec := types.SiteConfigSpec{
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
	}
	// Configure the public namespace.
	routerCreateSpec.SkupperNamespace = prv1Cluster.Namespace
	privateSiteConfig, err := prv1Cluster.VanClient.SiteConfigCreate(context.Background(), routerCreateSpec)
	prv1Cluster.VanClient.RouterCreate(ctx, *privateSiteConfig)

	service := types.ServiceInterface{
		Address:  "httpbin",
		Protocol: "http",
		Port:     8080,
	}

	err = prv1Cluster.VanClient.ServiceInterfaceCreate(ctx, &service)
	assert.Assert(r.T, err)

	err = prv1Cluster.VanClient.ServiceInterfaceBind(ctx, &service, "deployment", "httpbin", "http", 0)
	assert.Assert(r.T, err)

	// Configure the public namespace.
	routerCreateSpec.SkupperNamespace = pub1Cluster.Namespace
	publicSiteConfig, err := prv1Cluster.VanClient.SiteConfigCreate(context.Background(), routerCreateSpec)
	err = pub1Cluster.VanClient.RouterCreate(ctx, *publicSiteConfig)

	const secretFile = "/tmp/public_http_1_secret.yaml"
	err = pub1Cluster.VanClient.ConnectorTokenCreateFile(ctx, types.DefaultVanName, secretFile)
	assert.Assert(r.T, err)

	var connectorCreateOpts types.ConnectorCreateOptions = types.ConnectorCreateOptions{
		SkupperNamespace: prv1Cluster.Namespace,
		Name:             "",
		Cost:             0,
	}
	_, err = prv1Cluster.VanClient.ConnectorCreateFromFile(ctx, secretFile, connectorCreateOpts)
	assert.Assert(r.T, err)
}

func (r *HttpClusterTestRunner) TearDown(ctx context.Context) {
	r.GetPublicContext(1).DeleteNamespace()
	r.GetPrivateContext(1).DeleteNamespace()
}

func (r *HttpClusterTestRunner) Run(ctx context.Context) {
	defer r.TearDown(ctx)
	r.Setup(ctx)
	r.RunTests(ctx)
}

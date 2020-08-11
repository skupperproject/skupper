package http

import (
	"context"
	"fmt"
	"os/exec"
	"time"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/test/cluster"
	"gotest.tools/assert"

	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/davecgh/go-spew/spew"
	vegeta "github.com/tsenart/vegeta/v12/lib"
)

type HttpClusterTestRunner struct {
	cluster.ClusterTestRunnerBase
}

func int32Ptr(i int32) *int32 { return &i }

const minute time.Duration = 60

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

	r.Pub1Cluster.GetService("httpbin", 3*minute)
	time.Sleep(20 * time.Second) //TODO XXX What is the right condition to wait for?

	r.Pub1Cluster.KubectlExecAsync(fmt.Sprintf("port-forward service/httpbin 8080:80"))
	defer exec.Command("pkill", "kubectl").Run()
	time.Sleep(60 * time.Second) //give time to port forwarding to start

	// The test we are doing here is the most basic one, TODO add more
	// testing, asserts, etc.
	rate := vegeta.Rate{Freq: 100, Per: time.Second}
	duration := 4 * time.Second
	targeter := vegeta.NewStaticTargeter(vegeta.Target{
		Method: "GET",
		URL:    "http://localhost:8080/",
	})
	attacker := vegeta.NewAttacker()

	var metrics vegeta.Metrics
	for res := range attacker.Attack(targeter, rate, duration, "Big Bang!") {
		metrics.Add(res)
	}
	metrics.Close()

	spew.Dump(metrics)

	// Success is the percentage of non-error responses.
	assert.Assert(r.T, metrics.Success > 0.95, "too many errors! see the log for details.")
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

	vanRouterCreateOpts := types.VanSiteConfig{
		Spec: types.VanSiteConfigSpec{
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

	vanRouterCreateOpts.Spec.SkupperNamespace = r.Priv1Cluster.CurrentNamespace
	r.Priv1Cluster.VanClient.VanRouterCreate(ctx, vanRouterCreateOpts)

	service := types.ServiceInterface{
		Address:  "httpbin",
		Protocol: "http",
		Port:     80,
	}

	err = r.Priv1Cluster.VanClient.VanServiceInterfaceCreate(ctx, &service)
	assert.Assert(r.T, err)

	err = r.Priv1Cluster.VanClient.VanServiceInterfaceBind(ctx, &service, "deployment", "httpbin", "http", 0)
	assert.Assert(r.T, err)

	vanRouterCreateOpts.Spec.SkupperNamespace = r.Pub1Cluster.CurrentNamespace
	err = r.Pub1Cluster.VanClient.VanRouterCreate(ctx, vanRouterCreateOpts)

	err = r.Pub1Cluster.VanClient.VanConnectorTokenCreateFile(ctx, types.DefaultVanName, "/tmp/public_secret.yaml")
	assert.Assert(r.T, err)

	var vanConnectorCreateOpts types.VanConnectorCreateOptions = types.VanConnectorCreateOptions{
		SkupperNamespace: r.Priv1Cluster.CurrentNamespace,
		Name:             "",
		Cost:             0,
	}
	r.Priv1Cluster.VanClient.VanConnectorCreateFromFile(ctx, "/tmp/public_secret.yaml", vanConnectorCreateOpts)
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

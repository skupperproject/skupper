package tcp_echo

import (
	"context"
	"fmt"
	"log"
	"net"
	"os/exec"
	"strings"
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

const minute time.Duration = 60

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

func sendReceive(servAddr string) {
	strEcho := "Halo"
	tcpAddr, err := net.ResolveTCPAddr("tcp", servAddr)
	if err != nil {
		log.Panicln("ResolveTCPAddr failed:", err.Error())
	}
	conn, err := net.DialTCP("tcp", nil, tcpAddr)
	if err != nil {
		log.Panicln("Dial failed:", err.Error())
	}
	_, err = conn.Write([]byte(strEcho))
	if err != nil {
		log.Panicln("Write to server failed:", err.Error())
	}

	reply := make([]byte, 1024)

	_, err = conn.Read(reply)
	if err != nil {
		log.Panicln("Write to server failed:", err.Error())
	}
	conn.Close()

	log.Println("Sent to server = ", strEcho)
	log.Println("Reply from server = ", string(reply))

	if !strings.Contains(string(reply), strings.ToUpper(strEcho)) {
		log.Panicf("Response from server different that expected: %s", string(reply))
	}
}

func forwardSendReceive(cc *cluster.ClusterContext, port string) {
	cc.KubectlExecAsync(fmt.Sprintf("port-forward service/tcp-go-echo %s:9090", port))

	defer exec.Command("pkill", "kubectl").Run() //XXX the forwarding needs to be redesigned, this is an ugly patch
	//there is an issue with killing with the cmd.Kill() method...
	//circleci 27472  0.0  0.0   4460   684 pts/3    S+   01:21   0:00 sh -c KUBECONFIG=/home/circleci/.kube/config kubectl -n  public1 port-forward service/tcp-go-echo 9090:9090
	//circleci 27473  0.6  0.5 146188 41992 pts/3    Sl+  01:21   0:00 kubectl -n public1 port-forward service/tcp-go-echo 9090:9090
	//using the Kill only kills the first process

	//TODO find a better solution for this
	time.Sleep(60 * time.Second) //give time to port forwarding to start

	sendReceive("127.0.0.1:" + port)
}

func (r *TcpEchoClusterTestRunner) RunTests(ctx context.Context) {
	var publicService *apiv1.Service
	var privateService *apiv1.Service

	r.Pub1Cluster.KubectlExec("get svc")
	r.Priv1Cluster.KubectlExec("get svc")

	publicService = r.Pub1Cluster.GetService("tcp-go-echo", 3*minute)
	privateService = r.Priv1Cluster.GetService("tcp-go-echo", 3*minute)

	time.Sleep(20 * time.Second) //TODO XXX What is the right condition to wait for?
	//error: unable to forward port because pod is not running. Current status=Pending

	fmt.Printf("Public service ClusterIp = %q\n", publicService.Spec.ClusterIP)
	fmt.Printf("Private service ClusterIp = %q\n", privateService.Spec.ClusterIP)

	forwardSendReceive(r.Pub1Cluster, "9090")
	forwardSendReceive(r.Priv1Cluster, "9091")
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

	//copiar de aca para listar losnamespaces!!
	fmt.Printf("Listing deployments in namespace %q:\n", r.Pub1Cluster.CurrentNamespace)
	list, err := publicDeploymentsClient.List(metav1.ListOptions{})
	assert.Assert(r.T, err)

	for _, d := range list.Items {
		fmt.Printf(" * %s (%d replicas)\n", d.Name, *d.Spec.Replicas)
	}

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

	vanRouterCreateOpts.Spec.SkupperNamespace = r.Pub1Cluster.CurrentNamespace
	r.Pub1Cluster.VanClient.VanRouterCreate(ctx, vanRouterCreateOpts)

	service := types.ServiceInterface{
		Address:  "tcp-go-echo",
		Protocol: "tcp",
		Port:     9090,
	}
	err = r.Pub1Cluster.VanClient.VanServiceInterfaceCreate(ctx, &service)
	assert.Assert(r.T, err)

	err = r.Pub1Cluster.VanClient.VanServiceInterfaceBind(ctx, &service, "deployment", "tcp-go-echo", "tcp", 0)
	assert.Assert(r.T, err)

	err = r.Pub1Cluster.VanClient.VanConnectorTokenCreateFile(ctx, types.DefaultVanName, "/tmp/public_secret.yaml")
	assert.Assert(r.T, err)

	vanRouterCreateOpts.Spec.SkupperNamespace = r.Priv1Cluster.CurrentNamespace
	err = r.Priv1Cluster.VanClient.VanRouterCreate(ctx, vanRouterCreateOpts)

	var vanConnectorCreateOpts types.VanConnectorCreateOptions = types.VanConnectorCreateOptions{
		SkupperNamespace: r.Priv1Cluster.CurrentNamespace,
		Name:             "",
		Cost:             0,
	}
	r.Priv1Cluster.VanClient.VanConnectorCreateFromFile(ctx, "/tmp/public_secret.yaml", vanConnectorCreateOpts)
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

// +build integration cli

package gateway

import (
	"context"
	"log"
	"os"
	"os/exec"
	"strconv"
	"testing"

	"github.com/skupperproject/skupper-example-tcp-echo/pkg/server"
	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/client"
	"github.com/skupperproject/skupper/pkg/kube"
	"github.com/skupperproject/skupper/test/integration/tcp_echo"
	"github.com/skupperproject/skupper/test/utils/base"
	"github.com/skupperproject/skupper/test/utils/constants"
)

var (
	testRunner         = &base.ClusterTestRunnerBase{}
	localTcpEchoPort   int
	forwardTcpEchoPort int
)

// TestMain initializes flag parsing
func TestMain(m *testing.M) {
	base.ParseFlags()

	// If skupper, systemctl or qdrouterd binaries are not available, skip
	binaries := []string{"skupper", "systemctl", "qdrouterd"}
	missingBinaries := []string{}
	for _, binary := range binaries {
		if err := exec.Command(binary, "--help").Run(); err != nil {
			missingBinaries = append(missingBinaries, binary)
		}
	}
	if len(missingBinaries) > 0 {
		log.Printf("skipping - required binaries not available: %s", missingBinaries)
		os.Exit(0)
	}

	// Basic 2 namespaces setup only
	clusterNeeds := base.ClusterNeeds{
		NamespaceId:     "gateway",
		PublicClusters:  1,
		PrivateClusters: 1,
	}
	if err := testRunner.Validate(clusterNeeds); err != nil {
		log.Printf("gateway tests cannot be executed: %s", err)
		return
	}
	if _, err := testRunner.Build(clusterNeeds, nil); err != nil {
		log.Printf("error preparing cluster contexts: %s", err)
		return
	}

	// Setting up teardown
	defer tearDownFn()
	base.HandleInterruptSignal(tearDownFn)

	// Creating namespaces
	ctx, cancelCtx := context.WithCancel(context.Background())
	defer cancelCtx()
	if err := base.SetupSimplePublicPrivateAndConnect(ctx, testRunner); err == nil {
		// Wait for sites to be connected
		pub, _ := testRunner.GetPublicContext(1)
		if err = base.WaitForSkupperConnectedSites(ctx, pub, 1); err != nil {
			exit("timed out waiting for skupper sites to be connected: %s", err)
		}

		// Setting up cluster version of tcp echo
		dep, err := pub.VanClient.KubeClient.AppsV1().Deployments(pub.Namespace).Create(tcp_echo.Deployment)
		if err != nil {
			exit("error deploying tcp-echo server into %s: %s", pub.Namespace, err)
		}

		// Waiting on TCP echo to be running
		_, err = kube.WaitDeploymentReady(dep.Name, dep.Namespace, pub.VanClient.KubeClient, constants.ImagePullingAndResourceCreationTimeout, constants.DefaultTick)
		if err != nil {
			exit("deployment not ready: %s", err)
		}

		// Exposing the tcp-echo-cluster service
		tcpEchoClusterSvc := &types.ServiceInterface{
			Address:  "tcp-echo-cluster",
			Protocol: "tcp",
			Port:     9090,
		}
		if err := pub.VanClient.ServiceInterfaceCreate(ctx, tcpEchoClusterSvc); err != nil {
			exit("error creating skupper serivce %s: %s", tcpEchoClusterSvc.Address, err)
		}
		if err := pub.VanClient.ServiceInterfaceBind(ctx, tcpEchoClusterSvc, "deployment", dep.Name, "tcp", 9090); err != nil {
			exit("error binding skupper service %s with deployment %s: %s", tcpEchoClusterSvc.Address, dep.Name, err)
		}

		// Setting up local tcp echo server
		localTcpEchoPort, err = client.GetFreePort()
		if err != nil {
			exit("unable to get a free tcp port for tcp-go-echo server: %s", err)
		}
		stopCh := make(chan interface{})
		go server.Run(strconv.Itoa(localTcpEchoPort), stopCh)
		defer close(stopCh)
		log.Printf("local tcp-go-echo server listening on %d", localTcpEchoPort)

		// Getting forward port for tcp-echo-cluster service
		forwardTcpEchoPort, err = client.GetFreePort()
		if err != nil {
			exit("unable to get a free tcp port to forward requests to tcp-go-cluster service: %s", err)
		}

		// Running gateway tests
		m.Run()
	} else {
		log.Printf("error setting up public and private namespaces: %s", err)
	}
}

func exit(format string, v ...interface{}) {
	log.Printf(format, v...)
	tearDownFn()
	os.Exit(1)
}

func tearDownFn() {
	if len(testRunner.ClusterContexts) > 0 {
		base.TearDownSimplePublicAndPrivate(testRunner)
	}
}

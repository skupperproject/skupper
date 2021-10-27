package gateway

import (
	"context"
	"fmt"
	"log"
	"os/exec"
	"strconv"
	"testing"

	"github.com/skupperproject/skupper-example-tcp-echo/pkg/server"
	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/client"
	"github.com/skupperproject/skupper/pkg/kube"
	"github.com/skupperproject/skupper/pkg/utils"
	"github.com/skupperproject/skupper/test/integration/examples/tcp_echo"
	"github.com/skupperproject/skupper/test/utils/base"
	"github.com/skupperproject/skupper/test/utils/constants"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	LocalTcpEchoPort   int
	ForwardTcpEchoPort int
	// Exposing the tcp-echo-cluster service
	tcpEchoClusterSvc = &types.ServiceInterface{
		Address:  "tcp-echo-cluster",
		Protocol: "tcp",
		Ports:    []int{9090},
	}
)

// ValidateSkip validates whether gateway tests should be skipped
func ValidateSkip(t *testing.T, gatewayType string) {
	if utils.StringSliceContains([]string{"", "service"}, gatewayType) {
		// If skupper, systemctl or qdrouterd binaries are not available, skip
		binaries := []string{"skupper", "systemctl", "qdrouterd"}
		missingBinaries := []string{}
		for _, binary := range binaries {
			if err := exec.Command(binary, "--help").Run(); err != nil {
				missingBinaries = append(missingBinaries, binary)
			}
		}
		if len(missingBinaries) > 0 {
			t.Skipf("skipping - required binaries not available: %s", missingBinaries)
		}
	} else if gatewayType == "docker" {
		// If docker binary is not available, skip
		if err := exec.Command("docker", "--help").Run(); err != nil {
			t.Skipf("skipping - required binary not available: docker")
		}
	} else if gatewayType == "podman" {
		// If podman binary is not available, skip
		if err := exec.Command("podman", "--help").Run(); err != nil {
			t.Skipf("skipping - required binary not available: podman")
		}
	}
}

// Setup prepares the expected services on the localhost and in the cluster
func Setup(stopCh chan interface{}, testRunner base.ClusterTestRunner) error {
	ctx, cancelCtx := context.WithCancel(context.Background())
	defer cancelCtx()
	pub, _ := testRunner.GetPublicContext(1)

	// Setting up cluster version of tcp echo
	dep, err := pub.VanClient.KubeClient.AppsV1().Deployments(pub.Namespace).Create(tcp_echo.Deployment)
	if err != nil {
		return fmt.Errorf("error deploying tcp-echo server into %s: %s", pub.Namespace, err)
	}

	// Waiting on TCP echo to be running
	_, err = kube.WaitDeploymentReady(dep.Name, dep.Namespace, pub.VanClient.KubeClient, constants.ImagePullingAndResourceCreationTimeout, constants.DefaultTick)
	if err != nil {
		return fmt.Errorf("deployment not ready: %s", err)
	}

	if err := pub.VanClient.ServiceInterfaceCreate(ctx, tcpEchoClusterSvc); err != nil {
		return fmt.Errorf("error creating skupper service %s: %s", tcpEchoClusterSvc.Address, err)
	}
	if err := pub.VanClient.ServiceInterfaceBind(ctx, tcpEchoClusterSvc, "deployment", dep.Name, "tcp", map[int]int{9090: 9090}); err != nil {
		return fmt.Errorf("error binding skupper service %s with deployment %s: %s", tcpEchoClusterSvc.Address, dep.Name, err)
	}

	// Setting up local tcp echo server
	LocalTcpEchoPort, err = client.GetFreePort()
	if err != nil {
		return fmt.Errorf("unable to get a free tcp port for tcp-go-echo server: %s", err)
	}
	go server.Run(strconv.Itoa(LocalTcpEchoPort), stopCh)
	log.Printf("local tcp-go-echo server listening on %d", LocalTcpEchoPort)

	// Getting forward port for tcp-echo-cluster service
	ForwardTcpEchoPort, err = client.GetFreePort()
	if err != nil {
		return fmt.Errorf("unable to get a free tcp port to forward requests to tcp-go-cluster service: %s", err)
	}

	return nil
}

func TearDown(testRunner base.ClusterTestRunner) error {
	ctx, cancelCtx := context.WithCancel(context.Background())
	defer cancelCtx()
	pub, _ := testRunner.GetPublicContext(1)

	// Deleting the skupper service
	if err := pub.VanClient.ServiceInterfaceRemove(ctx, tcpEchoClusterSvc.Address); err != nil {

	}
	// Deleting the deployment
	if err := pub.VanClient.KubeClient.AppsV1().Deployments(pub.Namespace).Delete(tcp_echo.Deployment.Name, &v1.DeleteOptions{}); err != nil {
		return err
	}

	return nil
}

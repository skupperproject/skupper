// +build integration

package tcp_echo

import (
	"context"
	"fmt"
	"github.com/skupperproject/skupper/test/utils/base"
	"github.com/skupperproject/skupper/test/utils/k8s"
	"log"
	"net"
	"strings"
	"testing"

	"gotest.tools/assert"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
)

var (
	testRunner = &TcpEchoClusterTestRunner{}
)

func TestMain(m *testing.M) {
	testRunner.Initialize(m)
}

func TestTcpEcho(t *testing.T) {

	needs := base.ClusterNeeds{
		NamespaceId:     "tcp-echo",
		PublicClusters:  1,
		PrivateClusters: 1,
	}
	testRunner.Build(t, needs, nil)
	ctx, cancel := context.WithCancel(context.Background())
	base.HandleInterruptSignal(testRunner.T, func(t *testing.T) {
		testRunner.TearDown(ctx)
		cancel()
	})
	testRunner.Run(ctx)
}

func sendReceive() error {
	servAddr := "tcp-go-echo:9090"

	strEcho := "Halo"
	tcpAddr, err := net.ResolveTCPAddr("tcp", servAddr)
	if err != nil {
		return fmt.Errorf("ResolveTCPAddr failed: %s\n", err.Error())
	}

	conn, err := net.DialTCP("tcp", nil, tcpAddr)
	if err != nil {
		return fmt.Errorf("Dial failed: %s\n", err.Error())
	}
	defer conn.Close()

	_, err = conn.Write([]byte(strEcho))
	if err != nil {
		return fmt.Errorf("Write to server failed: %s\n", err.Error())
	}

	reply := make([]byte, 1024)

	_, err = conn.Read(reply)
	if err != nil {
		return fmt.Errorf("Read from server failed: %s\n", err.Error())
	}

	log.Println("Sent to server = ", strEcho)
	log.Println("Reply from server = ", string(reply))

	if !strings.Contains(string(reply), strings.ToUpper(strEcho)) {
		return fmt.Errorf("Response from server different that expected: %s\n", string(reply))
	}

	return nil
}

func TestTcpEchoJob(t *testing.T) {
	k8s.SkipTestJobIfMustBeSkipped(t)
	assert.Assert(t, sendReceive())
}

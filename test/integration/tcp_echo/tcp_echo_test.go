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
	"time"

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
	doneCh := make(chan error)
	go func(doneCh chan error) {
		servAddr := "tcp-go-echo:9090"

		strEcho := "Halo"
		log.Println("Resolving TCP Address")
		tcpAddr, err := net.ResolveTCPAddr("tcp", servAddr)
		if err != nil {
			doneCh <- fmt.Errorf("ResolveTCPAddr failed: %s\n", err.Error())
			return
		}

		log.Println("Opening TCP connection")
		conn, err := net.DialTCP("tcp", nil, tcpAddr)
		if err != nil {
			doneCh <- fmt.Errorf("Dial failed: %s\n", err.Error())
			return
		}
		defer conn.Close()

		log.Println("Sending data")
		_, err = conn.Write([]byte(strEcho))
		if err != nil {
			doneCh <- fmt.Errorf("Write to server failed: %s\n", err.Error())
			return
		}

		log.Println("Receiving reply")
		reply := make([]byte, 1024)

		_, err = conn.Read(reply)
		if err != nil {
			doneCh <- fmt.Errorf("Read from server failed: %s\n", err.Error())
			return
		}

		log.Println("Sent to server = ", strEcho)
		log.Println("Reply from server = ", string(reply))

		if !strings.Contains(string(reply), strings.ToUpper(strEcho)) {
			doneCh <- fmt.Errorf("Response from server different that expected: %s\n", string(reply))
			return
		}

		doneCh <- nil
	}(doneCh)
	timeoutCh := time.After(time.Minute)

	// TCP Echo Client job sometimes hangs waiting for response
	// This will cause job to fail and a retry to occur
	var err error
	select {
	case err = <-doneCh:
	case <-timeoutCh:
		err = fmt.Errorf("timed out waiting for tcp-echo job to finish")
	}

	return err
}

func TestTcpEchoJob(t *testing.T) {
	k8s.SkipTestJobIfMustBeSkipped(t)
	assert.Assert(t, sendReceive())
}

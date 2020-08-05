// +build integration

package tcp_echo

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"strings"
	"testing"

	"gotest.tools/assert"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
)

func TestTcpEcho(t *testing.T) {
	testRunner := &TcpEchoClusterTestRunner{}

	testRunner.Build(t, "tcp-echo")
	ctx := context.Background()
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
	job := os.Getenv("JOB")
	if job == "" {
		t.Skip("JOB environment variable not defined")
		return
	}
	assert.Assert(t, sendReceive())
}

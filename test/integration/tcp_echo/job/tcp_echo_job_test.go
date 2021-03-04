//+build job

package job

import (
	"fmt"
	"log"
	"net"
	"strings"
	"testing"
	"time"

	"gotest.tools/assert"
)

func TestTcpEchoJob(t *testing.T) {
	assert.Assert(t, SendReceive("tcp-go-echo:9090"))
}

func SendReceive(addr string) error {
	doneCh := make(chan error)
	go func(doneCh chan error) {

		strEcho := "Halo"
		log.Println("Resolving TCP Address")
		tcpAddr, err := net.ResolveTCPAddr("tcp", addr)
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

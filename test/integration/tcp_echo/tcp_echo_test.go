// +build integration

package tcp_echo

import (
	"context"
	"testing"
)

func TestTcpEcho(t *testing.T) {
	testRunner := &TcpEchoClusterTestRunner{}

	testRunner.Build(t, "tcp-echo")
	ctx := context.Background()
	testRunner.Run(ctx)
}

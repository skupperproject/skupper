// +build integration

package tcp_echo

import (
	"context"
	"testing"
)

func TestTcpEcho(t *testing.T) {
	testRunner := &TcpEchoClusterTestRunner{}

	testRunner.Build(t)
	ctx := context.Background()
	testRunner.Run(ctx)
}

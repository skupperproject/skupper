// +build integration smoke

package tcp_echo

import (
	"context"
	"os"
	"testing"

	"github.com/skupperproject/skupper/test/utils/base"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
)

func TestMain(m *testing.M) {
	base.ParseFlags()
	os.Exit(m.Run())
}

func TestTcpEcho(t *testing.T) {

	needs := base.ClusterNeeds{
		NamespaceId:     "tcp-echo",
		PublicClusters:  1,
		PrivateClusters: 1,
	}
	testRunner := &TcpEchoClusterTestRunner{}
	testRunner.BuildOrSkip(t, needs, nil)
	ctx, cancel := context.WithCancel(context.Background())
	base.HandleInterruptSignal(t, func(t *testing.T) {
		base.TearDownSimplePublicAndPrivate(&testRunner.ClusterTestRunnerBase)
		cancel()
	})
	testRunner.Run(ctx, t)
}

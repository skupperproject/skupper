// +build integration

package http

import (
	"context"
	"github.com/skupperproject/skupper/test/utils/base"
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	base.ParseFlags()
	os.Exit(m.Run())
}

func TestHttp(t *testing.T) {
	needs := base.ClusterNeeds{
		NamespaceId:     "http",
		PublicClusters:  1,
		PrivateClusters: 1,
	}
	testRunner := &HttpClusterTestRunner{}
	testRunner.BuildOrSkip(t, needs, nil)
	ctx, cancel := context.WithCancel(context.Background())
	base.HandleInterruptSignal(t, func(t *testing.T) {
		base.TearDownSimplePublicAndPrivate(&testRunner.ClusterTestRunnerBase)
		cancel()
	})
	testRunner.Run(ctx, t)
}

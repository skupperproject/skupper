// +build integration

package basic

import (
	"context"
	"github.com/skupperproject/skupper/test/utils/base"
	"testing"
)

var (
	testRunner BasicTestRunner
)

func TestMain(m *testing.M) {
	testRunner.Initialize(m)
}

func TestBasic(t *testing.T) {
	needs := base.ClusterNeeds{
		NamespaceId:     "basic",
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

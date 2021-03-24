// +build integration smoke

package basic

import (
	"context"
	"os"
	"testing"

	"github.com/skupperproject/skupper/test/utils/base"
)

func TestMain(m *testing.M) {
	base.ParseFlags()
	os.Exit(m.Run())
}

func TestBasic(t *testing.T) {
	needs := base.ClusterNeeds{
		NamespaceId:     "basic",
		PublicClusters:  1,
		PrivateClusters: 1,
	}
	testRunner := &BasicTestRunner{}
	testRunner.BuildOrSkip(t, needs, nil)
	ctx, cancel := context.WithCancel(context.Background())
	base.HandleInterruptSignal(t, func(t *testing.T) {
		testRunner.TearDown(ctx)
		cancel()
	})
	testRunner.Run(ctx, t)
}

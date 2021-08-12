// +build integration smoke acceptance

package basic

import (
	"context"
	"os"
	"testing"

	"github.com/skupperproject/skupper/test/utils/base"
	"gotest.tools/assert"
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
	if err := testRunner.Validate(needs); err != nil {
		t.Skipf("%s", err)
	}
	_, err := testRunner.Build(needs, nil)
	assert.Assert(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	base.HandleInterruptSignal(func() {
		testRunner.TearDown(ctx)
		cancel()
	})
	testRunner.Run(ctx, t)
}

// +build integration

package http

import (
	"context"

	"github.com/skupperproject/skupper/test/utils/base"
	"gotest.tools/assert"

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
	if err := testRunner.Validate(needs); err != nil {
		t.Skipf("%s", err)
	}
	_, err := testRunner.Build(needs, nil)
	assert.Assert(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	base.HandleInterruptSignal(func() {
		base.TearDownSimplePublicAndPrivate(&testRunner.ClusterTestRunnerBase)
		cancel()
	})
	testRunner.Run(ctx, t)
}

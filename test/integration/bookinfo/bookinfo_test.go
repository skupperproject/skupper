// +build integration

package bookinfo

import (
	"context"
	"os"
	"testing"

	"github.com/skupperproject/skupper/test/utils/base"
	"gotest.tools/assert"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
)

func TestMain(m *testing.M) {
	base.ParseFlags()
	os.Exit(m.Run())
}

func TestBookinfo(t *testing.T) {
	needs := base.ClusterNeeds{
		NamespaceId:     "bookinfo",
		PublicClusters:  1,
		PrivateClusters: 1,
	}
	testRunner := &base.ClusterTestRunnerBase{}
	if err := testRunner.Validate(needs); err != nil {
		t.Skipf("%s", err)
	}
	_, err := testRunner.Build(needs, nil)
	assert.Assert(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	base.HandleInterruptSignal(func() {
		base.TearDownSimplePublicAndPrivate(testRunner)
		cancel()
	})
	Run(ctx, t, testRunner)
}

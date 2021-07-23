// +build integration

package mongodb

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

func TestMongo(t *testing.T) {
	needs := base.ClusterNeeds{
		NamespaceId:     "mongo",
		PublicClusters:  1,
		PrivateClusters: 1,
	}
	testRunner := &base.ClusterTestRunnerBase{}
	if err := testRunner.Validate(needs); err != nil {
		t.Skipf("%s", err)
	}
	_, err := testRunner.Build(needs, nil)
	assert.Assert(t, err)
	//ctx, cancel := context.WithCancel(context.Background())
	//base.HandleInterruptSignal(func() {
	//base.TearDownSimplePublicAndPrivate(&testRunner.ClusterTestRunnerBase)
	//cancel()
	//})
	Run(context.Background(), t, testRunner)
}

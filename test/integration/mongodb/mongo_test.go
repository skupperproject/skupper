// +build integration

package mongodb

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

func TestMongo(t *testing.T) {
	needs := base.ClusterNeeds{
		NamespaceId:     "mongo",
		PublicClusters:  1,
		PrivateClusters: 1,
	}
	testRunner := &base.ClusterTestRunnerBase{}
	testRunner.BuildOrSkip(t, needs, nil)
	//ctx, cancel := context.WithCancel(context.Background())
	//base.HandleInterruptSignal(t, func(t *testing.T) {
	//base.TearDownSimplePublicAndPrivate(&testRunner.ClusterTestRunnerBase)
	//cancel()
	//})
	Run(context.Background(), t, testRunner)
}

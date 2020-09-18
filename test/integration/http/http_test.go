// +build integration

package http

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/skupperproject/skupper/test/utils/base"
	"github.com/skupperproject/skupper/test/utils/k8s"
	"gotest.tools/assert"

	"github.com/davecgh/go-spew/spew"
	vegeta "github.com/tsenart/vegeta/v12/lib"
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
		testRunner.TearDown(ctx)
		cancel()
	})
	testRunner.Run(ctx, t)
}

func TestHttpJob(t *testing.T) {
	k8s.SkipTestJobIfMustBeSkipped(t)

	rate := vegeta.Rate{Freq: 100, Per: time.Second}
	duration := 4 * time.Second
	targeter := vegeta.NewStaticTargeter(vegeta.Target{
		Method: "GET",
		URL:    "http://httpbin:8080/",
	})
	attacker := vegeta.NewAttacker()

	var metrics vegeta.Metrics
	for res := range attacker.Attack(targeter, rate, duration, "Big Bang!") {
		metrics.Add(res)
	}
	metrics.Close()

	//this is too verbose, anyway mantaining for now until we add more
	//assertions
	spew.Dump(metrics)

	// Success is the percentage of non-error responses.
	assert.Assert(t, metrics.Success > 0.95, "too many errors! see the log for details.")
}

// +build integration

package http

import (
	"context"
	"testing"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/skupperproject/skupper/test/cluster"
	vegeta "github.com/tsenart/vegeta/v12/lib"
	"gotest.tools/assert"
)

func TestHttp(t *testing.T) {
	testRunner := &HttpClusterTestRunner{}

	testRunner.Build(t, "http")
	ctx := context.Background()
	testRunner.Run(ctx)
}

func TestHttpJob(t *testing.T) {
	cluster.SkipTestJobIfMustBeSkipped(t)

	rate := vegeta.Rate{Freq: 100, Per: time.Second}
	duration := 4 * time.Second
	targeter := vegeta.NewStaticTargeter(vegeta.Target{
		Method: "GET",
		URL:    "http://httpbin:80/",
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

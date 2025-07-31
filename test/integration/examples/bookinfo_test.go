//go:build integration || examples
// +build integration examples

package examples

import (
	"context"
	"testing"
	"runtime"

	"github.com/skupperproject/skupper/test/integration/examples/bookinfo"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
)

func TestBookinfo(t *testing.T) {
	if runtime.GOARCH == "s390x" {
		t.Skip("Skipping test on s390x architecture as bookinfo images support is unavailable for s390x")
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	bookinfo.Run(ctx, t, testRunner)
}

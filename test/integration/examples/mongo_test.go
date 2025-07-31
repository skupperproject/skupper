//go:build integration || examples
// +build integration examples

package examples

import (
	"context"
	"testing"
	"runtime"

	"github.com/skupperproject/skupper/test/integration/examples/mongodb"
)

func TestMongo(t *testing.T) {
	if runtime.GOARCH == "s390x" {
		t.Skip("Skipping test on s390x architecture as mongo image unsupported for s390x")
	}
	mongodb.Run(context.Background(), t, testRunner)
}

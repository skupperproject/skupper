//go:build integration || examples
// +build integration examples

package examples

import (
	"context"
	"testing"

	"github.com/skupperproject/skupper/test/integration/examples/mongodb"
	"github.com/skupperproject/skupper/test/utils/arch"
	"github.com/skupperproject/skupper/test/utils/base"
)

func TestMongo(t *testing.T) {
	// Skip test if the cluster architecture is s390x
	cluster := base.GetClusterContext()
	if err := arch.SkipOnlyS390x(t, cluster); err != nil {
		t.Fatal(err)
	}

	mongodb.Run(context.Background(), t, testRunner)
}

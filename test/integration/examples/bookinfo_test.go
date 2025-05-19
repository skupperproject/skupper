//go:build integration || examples
// +build integration examples

package examples

import (
	"context"
	"testing"

	"github.com/skupperproject/skupper/test/integration/examples/bookinfo"
	"github.com/skupperproject/skupper/test/utils/base"
	"github.com/skupperproject/skupper/test/utils/arch"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
)

func TestBookinfo(t *testing.T) {
	// Get the cluster context
	cluster := base.GetClusterContext()

	// Skip test if the cluster architecture is s390x
	if err := arch.SkipOnlyS390x(t, cluster); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	bookinfo.Run(ctx, t, testRunner)
}

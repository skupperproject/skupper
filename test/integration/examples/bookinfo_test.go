// +build integration examples

package examples

import (
	"context"
	"testing"

	"github.com/skupperproject/skupper/test/integration/examples/bookinfo"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
)

func TestBookinfo(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	bookinfo.Run(ctx, t, testRunner)
}

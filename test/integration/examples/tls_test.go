//go:build integration || smoke || examples
// +build integration smoke examples

package examples

import (
	"context"
	"testing"

	"github.com/skupperproject/skupper/test/integration/examples/tls_t"
	"github.com/skupperproject/skupper/test/utils/constants"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
)

func TestTls(t *testing.T) {
	ctx, cancelFn := context.WithTimeout(context.Background(), constants.ImagePullingAndResourceCreationTimeout)
	defer cancelFn()
	tls_t.Run(ctx, t, testRunner)
}

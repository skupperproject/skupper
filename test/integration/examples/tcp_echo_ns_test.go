//go:build integration || smoke || examples
// +build integration smoke examples

package examples

import (
	"context"
	"testing"

	"github.com/skupperproject/skupper/test/integration/examples/tcp_echo"
	"github.com/skupperproject/skupper/test/utils/constants"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
)

func TestTcpEchoNs(t *testing.T) {
	ctx, cancelFn := context.WithTimeout(context.Background(), constants.ImagePullingAndResourceCreationTimeout)
	defer cancelFn()
	tcp_echo.RunForNamespace(ctx, t, testRunner, "another-namespace")
}

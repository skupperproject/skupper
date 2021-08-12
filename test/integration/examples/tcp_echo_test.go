// +build integration smoke examples

package examples

import (
	"context"
	"testing"

	"github.com/skupperproject/skupper/test/integration/examples/tcp_echo"
	"github.com/skupperproject/skupper/test/utils/constants"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
)

func TestTcpEcho(t *testing.T) {
	ctx, cancelFn := context.WithTimeout(context.Background(), constants.ImagePullingAndResourceCreationTimeout)
	defer cancelFn()
	tcp_echo.Run(ctx, t, testRunner)
}

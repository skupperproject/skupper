//+build job

package job

import (
	"os"
	"testing"

	"github.com/skupperproject/skupper/test/integration/examples/tcp_echo"
	"github.com/skupperproject/skupper/test/utils"
	"gotest.tools/assert"
)

func TestTcpEchoJob(t *testing.T) {
	addr := utils.StrDefault("tcp-go-echo:9090", os.Getenv("ADDRESS"))
	assert.Assert(t, tcp_echo.SendReceive(addr))
}

//+build job

package job

import (
	"os"
	"testing"

	"github.com/skupperproject/skupper/test/integration/examples/tls_t"
	"github.com/skupperproject/skupper/test/utils"
	"gotest.tools/assert"
)

func TestTlsJob(t *testing.T) {

	// TODO: move string to package var?
	addr := utils.StrDefault("ssl-server:8443", os.Getenv("ADDRESS"))
	assert.Assert(t, tls_t.SendReceive(addr))
}

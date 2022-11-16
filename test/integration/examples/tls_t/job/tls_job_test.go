//+build job

package job

import (
	"os"
	"os/exec"
	"testing"

	"github.com/skupperproject/skupper/test/integration/examples/tls_t"
	"github.com/skupperproject/skupper/test/utils"
	"gotest.tools/assert"
)

func TestTlsJob(t *testing.T) {
	cmd := exec.Command("microdnf", "install", "openssl")
	err := cmd.Run()
	if err != nil {
		t.Fatalf("error instaslling openssl: %e", err)
	}
	addr := utils.StrDefault("ssl-server:8443", os.Getenv("ADDRESS"))
	assert.Assert(t, tls_t.SendReceive(addr))
}

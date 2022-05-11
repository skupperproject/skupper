//go:build (performance && !integration) || tcp
// +build performance,!integration tcp

package tcp

import (
	"testing"
)

func TestTcpPerf(t *testing.T) {
	runIperfTest(t, true)
}

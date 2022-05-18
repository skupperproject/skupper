//go:build (invalid && integration && !performance) || tcp
// +build invalid,integration,!performance tcp

package tcp

import (
	"testing"
)

func TestIperf(t *testing.T) {
	runIperfTest(t, false)
}

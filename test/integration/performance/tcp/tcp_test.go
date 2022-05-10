//go:build (integration && !performance) || tcp
// +build integration,!performance tcp

package tcp

import (
	"testing"
)

func TestIperf(t *testing.T) {
	runIperfTest(t, false)
}

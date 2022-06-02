//go:build integration && !performance
// +build integration,!performance

package performance

import (
	"testing"

	"github.com/skupperproject/skupper/test/integration/performance/common"
)

func TestMain(m *testing.M) {
	common.RunPerformanceTests(m, true)
}

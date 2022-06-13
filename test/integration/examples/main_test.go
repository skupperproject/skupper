//go:build integration || examples || cli || gateway || acceptance || smoke
// +build integration examples cli gateway acceptance smoke

package examples

import (
	"testing"

	"github.com/skupperproject/skupper/test/utils/base"
)

var (
	testRunner = &base.ClusterTestRunnerBase{}
)

// TestMain initializes flag parsing
func TestMain(m *testing.M) {
	base.RunBasicTopologyTests(m, base.BasicTopologySetup{
		TestRunner:  testRunner,
		NamespaceId: "examples",
	})
}

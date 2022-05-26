//go:build integration || acceptance || cli || smoke || annotation || gateway || console
// +build integration acceptance cli smoke annotation gateway console

package acceptance

import (
	"log"
	"testing"

	"github.com/skupperproject/skupper/test/integration/acceptance/annotation"
	"github.com/skupperproject/skupper/test/utils/base"
)

var (
	testRunner = &base.ClusterTestRunnerBase{}
)

// TestMain initializes flag parsing
func TestMain(m *testing.M) {
	base.RunBasicTopologyTests(m, base.BasicTopologySetup{
		TestRunner:  testRunner,
		NamespaceId: "acceptance",
		PreSkupperSetup: func(testRunner *base.ClusterTestRunnerBase) error {
			// Annotated resource test needs resources deployed before Skupper network is created
			if err := annotation.DeployResources(testRunner); err != nil {
				log.Printf("error deploying annotated resources before creating the skupper network")
				return err
			}
			return nil
		},
	})
}

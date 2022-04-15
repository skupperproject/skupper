package hello_policy

import (
	"testing"

	"github.com/skupperproject/skupper/pkg/apis/skupper/v1alpha1"
	"github.com/skupperproject/skupper/test/utils/base"
	"github.com/skupperproject/skupper/test/utils/skupper/cli"
)

func testServicePolicy(t *testing.T, pub, prv *base.ClusterContext) {

	testTable := []policyTestCase{
		{
			name: "initialize",
			steps: []policyTestStep{
				{
					name:     "skupper-init",
					parallel: true,
					commands: []cli.TestScenario{
						skupperInitInteriorTestScenario(pub, "", true),
						skupperInitEdgeTestScenario(prv, "", true),
					},
				}, {
					name: "connect",
					pubPolicy: []v1alpha1.SkupperClusterPolicySpec{
						{
							Namespaces:                    []string{"*"},
							AllowIncomingLinks:            true,
							AllowedOutgoingLinksHostnames: []string{"*"},
						},
					},
					commands: []cli.TestScenario{
						connectSites(pub, prv, "", "service"),
					},
				},
			},
		},
	}

	policyTestRunner{
		scenarios: testTable,
		// Add background policies; policies that are not removed across
		// runs
	}.run(t, pub, prv)

}

package hello_policy

import (
	"testing"

	"github.com/skupperproject/skupper/client"
	skupperv1 "github.com/skupperproject/skupper/pkg/apis/skupper/v1alpha1"
	"github.com/skupperproject/skupper/test/utils/base"
	"github.com/skupperproject/skupper/test/utils/skupper/cli"
	"github.com/skupperproject/skupper/test/utils/skupper/cli/link"
)

// Each policy piece has its own file.  On it, we define both the
// piece-specific tests _and_ the piece-specific infra.
//
// For example, the checking for link being (un)able to create or being
// destroyed is defined on functions on link_test.go
//
// These functions will take a cluster context and an optional name prefix.  It
// will return a slice of cli.TestScenario with the intended objective on the
// requested cluster, and the names of the individual scenarios will receive
// the prefix, if any given.  A use of that prefix would be, for example, to
// clarify that what's being checked is a 'side-effect' (eg when a link drops
// in a cluster because the policy was removed on the other cluster)

func createLink(t *testing.T, pub, prv *base.ClusterContext) (scenarios []cli.TestScenario) {

	scenarios = []cli.TestScenario{
		{
			Name: "connect-sites",
			Tasks: []cli.SkupperTask{
				{Ctx: pub, Commands: []cli.SkupperCommandTester{
					// skupper link create - connect to public and verify connection created
					&link.CreateTester{
						TokenFile: "./tmp/" + "works.token.yaml",
						Name:      "public",
						Cost:      1,
					},
				},
				},
			},
		},
	}

	return
}

type testLinkPolicyCase struct {
	policy skupperv1.SkupperClusterPolicySpec
}

func testLinkPolicy(t *testing.T, pub, prv *base.ClusterContext) {

	policyClient := client.NewPolicyValidatorAPI(pub.VanClient)

	initSteps := append(skupperInitInterior(pub), skupperInitEdge(prv)...)
	t.Run("init", func(t *testing.T) { cli.RunScenariosParallel(t, initSteps) })

	t.Run("empty-policy-fails-token-creation", func(t *testing.T) {
		createToken := createToken("fail", pub, "./tmp", false)
		cli.RunScenarios(t, []cli.TestScenario{createToken})
		res, err := policyClient.IncomingLink()
		if err != nil {
			t.Errorf("API check failed: %v", err)
		}
		if res.Allowed {
			t.Error("API reports incoming link creation as allowed")
		}
	})

	t.Run("allowing-policy-allows-creation", func(t *testing.T) {
		createToken := createToken("works", pub, "./tmp/", true)
		createLink := createLink(t, pub, prv)

		policySpec := skupperv1.SkupperClusterPolicySpec{
			Namespaces:                    []string{pub.Namespace},
			AllowIncomingLinks:            true,
			AllowedOutgoingLinksHostnames: []string{"*"},
		}
		err := applyPolicy(t, "generated-policy", policySpec, pub)
		if err != nil {
			t.Fatalf("Failed to apply policy: %v", err)
			return
		}

		cli.RunScenarios(t, append([]cli.TestScenario{createToken}, createLink...))
		res, err := policyClient.IncomingLink()
		if err != nil {
			t.Errorf("API check failed: %v", err)
		}
		if !res.Allowed {
			t.Error("API reports incoming link creation as not allowed")
		}
	})

	deleteSteps := append(deleteSkupper(pub), deleteSkupper(prv)...)
	t.Run("cleanup", func(t *testing.T) { cli.RunScenariosParallel(t, deleteSteps) })

}

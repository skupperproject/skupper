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

// Uses the named token to create a link from ctx1 to ctx2
//
// Returns a scenario with a single link.CreateTester
func createLinkTestScenario(ctx1, ctx2 *base.ClusterContext, prefix, name string) (scenario cli.TestScenario) {

	scenario = cli.TestScenario{
		Name: prefixName(prefix, "connect-sites"),
		Tasks: []cli.SkupperTask{
			{Ctx: ctx1, Commands: []cli.SkupperCommandTester{
				&link.CreateTester{
					TokenFile: "./tmp/" + name + ".token.yaml",
					Name:      "public",
					Cost:      1,
				},
			},
			},
		},
	}

	return
}

// The idea:
//
// - set policyStart
// - run prep steps
// - set policyChange
// - run scenario
// - deleteLink(s), if extant
//
// prep steps and policyChange may be empty, if unnecessary
//
// Uses:
//   - policyStart: allowed
//     prep: create token
//     policyChange: disallow
//     run: try to create link with pre-created token
//   - policyStart: allowed
//     pre: create token, link
//     policyChange: disallow
//     run: stuff came down
//   - policyStart: disallow
//     prep: try to create link, fail
//     policyChange: allow
//     run: creations now work

// This is one option
type testLinkPolicyCase struct {
	pubPolicyStart  skupperv1.SkupperClusterPolicySpec
	prvPolicyStart  skupperv1.SkupperClusterPolicySpec
	prep            []cli.TestScenario
	pubPolicyChange skupperv1.SkupperClusterPolicySpec
	prvPolicyChange skupperv1.SkupperClusterPolicySpec
	scenario        []cli.TestScenario
}

// This is another
type testLinkPolicyCase2 struct {
	tokenWorks bool
	policy     skupperv1.SkupperClusterPolicySpec
	linkWorks  bool
	linkFalls  bool
	scenario   []cli.TestScenario
}

func testLinkPolicy(t *testing.T, pub, prv *base.ClusterContext) {

	policyClient := client.NewPolicyValidatorAPI(pub.VanClient)

	initSteps := []cli.TestScenario{
		skupperInitInteriorTestScenario(pub, "", true),
		skupperInitEdgeTestScenario(prv, "", true),
	}

	t.Run("init", func(t *testing.T) { cli.RunScenariosParallel(t, initSteps) })

	t.Run("empty-policy-fails-token-creation", func(t *testing.T) {
		cli.RunScenarios(
			t,
			[]cli.TestScenario{
				createTokenPolicyScenario(pub, "", "./tmp", "fail", false),
			})

		res, err := policyClient.IncomingLink()
		if err != nil {
			t.Errorf("API check failed: %v", err)
		}
		if res.Allowed {
			t.Error("API reports incoming link creation as allowed")
		}
	})

	t.Run("allowing-policy-allows-creation", func(t *testing.T) {

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

		cli.RunScenarios(
			t,
			[]cli.TestScenario{
				createTokenPolicyScenario(pub, "", "./tmp", "works", true),
				createLinkTestScenario(pub, prv, "", "works"),
			})

		res, err := policyClient.IncomingLink()
		if err != nil {
			t.Errorf("API check failed: %v", err)
		}
		if !res.Allowed {
			t.Error("API reports incoming link creation as not allowed")
		}
	})

	deleteSteps := []cli.TestScenario{
		deleteSkupperTestScenario(pub, "pub"),
		deleteSkupperTestScenario(prv, "prv"),
	}
	t.Run("cleanup", func(t *testing.T) { cli.RunScenariosParallel(t, deleteSteps) })

}

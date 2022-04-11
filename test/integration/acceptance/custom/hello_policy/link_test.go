package hello_policy

import (
	"strconv"
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

// Uses the named token to create a link from ctx
//
// Returns a scenario with a single link.CreateTester
func createLinkTestScenario(ctx *base.ClusterContext, prefix, name string) (scenario cli.TestScenario) {

	scenario = cli.TestScenario{
		Name: prefixName(prefix, "connect-sites"),
		Tasks: []cli.SkupperTask{
			{Ctx: ctx, Commands: []cli.SkupperCommandTester{
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

func allowIncomingLinkPolicy(namespace string) (policySpec skupperv1.SkupperClusterPolicySpec) {
	policySpec = skupperv1.SkupperClusterPolicySpec{
		Namespaces:         []string{namespace},
		AllowIncomingLinks: true,
	}

	return
}

func allowedOutgoingLinksHostnamesPolicy(namespace string, hostnames []string) (policySpec skupperv1.SkupperClusterPolicySpec) {
	policySpec = skupperv1.SkupperClusterPolicySpec{
		Namespaces:                    []string{namespace},
		AllowedOutgoingLinksHostnames: hostnames,
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

type policyTestStep struct {
	name        string
	pubPolicy   []skupperv1.SkupperClusterPolicySpec
	prvPolicy   []skupperv1.SkupperClusterPolicySpec
	commands    []cli.TestScenario
	pubGetCheck policyGetCheck
	prvGetCheck policyGetCheck
	// Add GetCheck here
}

type policyTestCase struct {
	name  string
	steps []policyTestStep
}

func applyPolicies(
	t *testing.T,
	name string,
	pub *base.ClusterContext, pubPolicies []skupperv1.SkupperClusterPolicySpec,
	prv *base.ClusterContext, prvPolicies []skupperv1.SkupperClusterPolicySpec) {
	if len(pubPolicies)+len(prvPolicies) > 0 {
		t.Run(
			name,
			func(t *testing.T) {
				for i, policy := range pubPolicies {
					i := strconv.Itoa(i)
					err := applyPolicy(t, "pub-policy-"+i, policy, pub)
					if err != nil {
						t.Fatalf("Failed to apply policy: %v", err)
					}
				}
				for i, policy := range prvPolicies {
					i := strconv.Itoa(i)
					err := applyPolicy(t, "prv-policy-"+i, policy, prv)
					if err != nil {
						t.Fatalf("Failed to apply policy: %v", err)
					}
				}

			})
	}
}

func getChecks(t *testing.T, getCheck policyGetCheck, c *client.PolicyAPIClient) {
	ok, err := getCheck.check(c)
	if err != nil {
		t.Errorf("GET check failed with error: %v", err)
		return
	}

	if !ok {
		t.Errorf("GET check failed")
	}
}

func testLinkPolicy(t *testing.T, pub, prv *base.ClusterContext) {

	// these are final, do not change them.  They're used with
	// a boolean pointer to allow true/false/undefined
	_true := true
	_false := false

	testTable := []policyTestCase{
		{
			name: "empty-policy-fails-token-creation",
			steps: []policyTestStep{
				{
					name: "execute",
					commands: []cli.TestScenario{
						createTokenPolicyScenario(pub, "", "./tmp", "fail", false),
					},
					pubGetCheck: policyGetCheck{
						allowIncoming: &_false,
					},
				},
			},
		}, {
			name: "allowing-policy-allows-creation",
			steps: []policyTestStep{
				{
					name: "execute",
					pubPolicy: []skupperv1.SkupperClusterPolicySpec{
						allowIncomingLinkPolicy(pub.Namespace),
					},
					prvPolicy: []skupperv1.SkupperClusterPolicySpec{
						allowedOutgoingLinksHostnamesPolicy(prv.Namespace, []string{"*"}),
					},
					commands: []cli.TestScenario{
						createTokenPolicyScenario(pub, "", "./tmp", "works", true),
						createLinkTestScenario(prv, "", "works"),
						cli.TestScenario{
							Name: "check",
							Tasks: []cli.SkupperTask{
								{
									Ctx: prv,
									Commands: []cli.SkupperCommandTester{
										&link.StatusTester{
											Name: "public",
										},
									},
								},
							},
						},
					},
					pubGetCheck: policyGetCheck{
						allowIncoming: &_true,
					},
					prvGetCheck: policyGetCheck{
						allowIncoming: &_false,
					},
				}, {
					name: "delete",
					commands: []cli.TestScenario{
						{
							Name: "remove-link",
							Tasks: []cli.SkupperTask{
								{
									Ctx: prv,
									Commands: []cli.SkupperCommandTester{
										&link.DeleteTester{
											Name: "public",
										},
									},
								},
							},
						},
					},
				},
			},
		}, {
			name: "previously-created-token",
			steps: []policyTestStep{
				{
					name: "prepare",
					pubPolicy: []skupperv1.SkupperClusterPolicySpec{
						allowIncomingLinkPolicy(pub.Namespace),
					},
					prvPolicy: []skupperv1.SkupperClusterPolicySpec{
						allowedOutgoingLinksHostnamesPolicy(prv.Namespace, []string{"*"}),
					},
					commands: []cli.TestScenario{
						createTokenPolicyScenario(pub, "", "./tmp", "prev", true),
					},
				}, {
					name: "disallow-and-create-link",
					pubPolicy: []skupperv1.SkupperClusterPolicySpec{
						allowIncomingLinkPolicy("non-existing"),
					},
					commands: []cli.TestScenario{
						createLinkTestScenario(prv, "", "works"),
						cli.TestScenario{
							Name: "link-is-down",
							Tasks: []cli.SkupperTask{
								{
									Ctx: prv,
									Commands: []cli.SkupperCommandTester{
										&link.StatusTester{
											Name: "public"}},
								},
							},
						},
					},
					pubGetCheck: policyGetCheck{
						allowIncoming: &_false,
					},
				},
			},
		},
	}

	pubPolicyClient := client.NewPolicyValidatorAPI(pub.VanClient)
	prvPolicyClient := client.NewPolicyValidatorAPI(prv.VanClient)

	t.Run("init", func(t *testing.T) {
		cli.RunScenariosParallel(
			t,
			[]cli.TestScenario{
				skupperInitInteriorTestScenario(pub, "", true),
				skupperInitEdgeTestScenario(prv, "", true),
			})
	})

	for _, scenario := range testTable {
		removePolicies(t, pub)
		removePolicies(t, prv)
		base.StopIfInterrupted(t)
		t.Run(
			scenario.name,
			func(t *testing.T) {
				for _, step := range scenario.steps {
					applyPolicies(t, "policy-setup", pub, step.pubPolicy, prv, step.prvPolicy)
					cli.RunScenarios(t, step.commands)
					getChecks(t, step.pubGetCheck, pubPolicyClient)
					getChecks(t, step.prvGetCheck, prvPolicyClient)
				}
			})
	}

	t.Run(
		"cleanup",
		func(t *testing.T) {
			cli.RunScenariosParallel(
				t,
				[]cli.TestScenario{
					deleteSkupperTestScenario(pub, "pub"),
					deleteSkupperTestScenario(prv, "prv"),
				},
			)
		})

}

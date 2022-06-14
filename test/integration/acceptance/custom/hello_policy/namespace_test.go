//go:build policy
// +build policy

package hello_policy

import (
	"fmt"
	"testing"

	skupperv1 "github.com/skupperproject/skupper/pkg/apis/skupper/v1alpha1"
	"github.com/skupperproject/skupper/test/utils/base"
	"github.com/skupperproject/skupper/test/utils/skupper/cli"
)

// This test checks how the policy engine reacts to policies that are changing,
// as opposed to new policies or removal of policies.
//
// In this test, the creation and state of skupper links are used as a proxy to
// see how the policy engine reacts to different settings for the namespaces
// field.
//
// This test should be ok to run on both single and multi cluster environments.
func testNamespaceLinkTransitions(t *testing.T, pub, prv *base.ClusterContext) {

	testTable := []policyTestCase{
		{
			name: "init",
			steps: []policyTestStep{
				{
					name:     "init",
					parallel: true,
					pubPolicy: []skupperv1.SkupperClusterPolicySpec{
						allowIncomingLinkPolicy(pub.Namespace, true),
					},
					cliScenarios: []cli.TestScenario{
						skupperInitInteriorTestScenario(pub, "", true),
						skupperInitEdgeTestScenario(prv, "", true),
					},
				}, {
					name: "connect",
					// Skupper init does not need those policies, so we
					// only check them here, where the test will most probably
					// run only once, saving a few seconds
					getChecks: []policyGetCheck{
						{
							cluster:       pub,
							allowIncoming: cli.Boolp(true),
						}, {
							cluster:       prv,
							allowIncoming: cli.Boolp(false),
							allowedHosts:  []string{"any"},
						},
					},
					cliScenarios: []cli.TestScenario{
						createTokenPolicyScenario(pub, "", testPath, "transition", true),
						createLinkTestScenario(prv, "", "transition", false),
						sitesConnectedTestScenario(pub, prv, "", "transition"),
					},
				},
			},
		}, {
			name: "keep-policy--change-value--disconnects",
			steps: []policyTestStep{
				{
					name: "execute",
					pubPolicy: []skupperv1.SkupperClusterPolicySpec{
						allowIncomingLinkPolicy(pub.Namespace, false),
					},
					// We do not need these GET checks for the cliScenario, as
					// that's a Status scenario, but this could be an early
					// warning if the scenario fails.
					getChecks: []policyGetCheck{
						{
							cluster:       pub,
							allowIncoming: cli.Boolp(false),
						}, {
							cluster:       prv,
							allowIncoming: cli.Boolp(false),
						},
					},
					cliScenarios: []cli.TestScenario{
						linkStatusTestScenario(prv, "", "transition", false),
					},
				},
			},
		}, {
			name: "keep-policy--change-value--reconnects",
			steps: []policyTestStep{
				{
					name: "execute",
					pubPolicy: []skupperv1.SkupperClusterPolicySpec{
						allowIncomingLinkPolicy(pub.Namespace, true),
					},
					getChecks: []policyGetCheck{
						{
							cluster:       pub,
							allowIncoming: cli.Boolp(true),
						}, {
							cluster:       prv,
							allowIncoming: cli.Boolp(false),
						},
					},
					cliScenarios: []cli.TestScenario{
						linkStatusTestScenario(prv, "", "transition", true),
					},
				},
			},
		}, {
			// This whole test was created because of this specific test case.
			// More specifically the bug described on
			// https://github.com/skupperproject/skupper/issues/718
			name: "keep-policy--remove-namespace--disconnects",
			steps: []policyTestStep{
				{
					name: "execute",
					pubPolicy: []skupperv1.SkupperClusterPolicySpec{
						allowIncomingLinkPolicy("non-existent", true),
					},
					getChecks: []policyGetCheck{
						{
							cluster:       pub,
							allowIncoming: cli.Boolp(false),
						},
					},
					cliScenarios: []cli.TestScenario{
						linkStatusTestScenario(prv, "", "transition", false),
					},
				},
			},
		}, {
			name: "keep-policy--add-namespace--reconnects",
			steps: []policyTestStep{
				{
					name: "execute",
					pubPolicy: []skupperv1.SkupperClusterPolicySpec{
						allowIncomingLinkPolicy(pub.Namespace, true),
					},
					getChecks: []policyGetCheck{
						{
							cluster:       pub,
							allowIncoming: cli.Boolp(true),
						}, {
							cluster:       prv,
							allowIncoming: cli.Boolp(false),
						},
					},
					cliScenarios: []cli.TestScenario{
						linkStatusTestScenario(prv, "", "transition", true),
					},
				},
			},
		}, {
			name: "cleanup",
			steps: []policyTestStep{
				{
					name:     "execute",
					parallel: true,
					cliScenarios: []cli.TestScenario{
						deleteSkupperTestScenario(pub, "pub"),
						deleteSkupperTestScenario(prv, "prv"),
					},
				},
			},
		},
	}

	policyTestRunner{
		testCases:    testTable,
		keepPolicies: true,
		prvPolicies: []skupperv1.SkupperClusterPolicySpec{
			allowedOutgoingLinksHostnamesPolicy(prv.Namespace, []string{"*"}),
		},
	}.run(t, pub, prv)

}

// This is the struct used by testNamespaceIncomingLinks
type namespaceTest struct {
	// This will go straight to the policy's Namespaces field, so it will
	// be a definition of which namespaces are being affected by policy
	// change
	namespaces []string

	// Whether the policy change should affect the target namespace (pub1)
	// Notice the target is not what is on the field above; it's always
	// the same, static, namespace
	worksOnTarget bool

	// Whether the change should affect other namespaces (more specifically
	// pub2)
	worksElsewhere bool
}

// This test sets some policies with namespaces that use wide-matching regexes
// (such as "*"), and checks for collateral effects on multiple namespaces.
//
// For that reason, it should always run only on a single cluster (ie, pub1 and
// pub2 should be namespaces of the same cluster).
//
// In this test, the creation and state of skupper links are used as a proxy to
// see how the policy engine reacts to different settings for the namespaces
// field.
func testNamespaceIncomingLinks(t *testing.T, pub1, pub2 *base.ClusterContext) {

	// TODO: Change to use policyTestRunner

	var err error

	initSteps := []cli.TestScenario{
		skupperInitInteriorTestScenario(pub1, "pub", true),
		skupperInitInteriorTestScenario(pub2, "prv", true),
	}

	testTable := []namespaceTest{
		{
			namespaces:     []string{"*"},
			worksOnTarget:  true,
			worksElsewhere: true,
		}, {
			namespaces:     []string{pub1.Namespace},
			worksOnTarget:  true,
			worksElsewhere: false,
		}, {
			namespaces:     []string{pub2.Namespace},
			worksOnTarget:  false,
			worksElsewhere: true,
		}, {
			namespaces:     []string{fmt.Sprintf(".*%v$", pub1.Namespace[len(pub1.Namespace)-1:])},
			worksOnTarget:  true,
			worksElsewhere: false,
		}, {
			namespaces:     []string{fmt.Sprintf(".*%v$", pub2.Namespace[len(pub2.Namespace)-1:])},
			worksOnTarget:  false,
			worksElsewhere: true,
		}, {
			namespaces:     []string{"policy"},
			worksOnTarget:  true,
			worksElsewhere: true,
		}, {
			namespaces:     []string{`test.skupper.io/test-namespace=policy`},
			worksOnTarget:  true,
			worksElsewhere: true,
		}, {
			namespaces:     []string{"non-existing-label=true"},
			worksOnTarget:  false,
			worksElsewhere: false,
		}, {
			// AND-behavior for labels in a single entry
			namespaces:     []string{`test.skupper.io/test-namespace=policy,non-existing-label=true`},
			worksOnTarget:  false,
			worksElsewhere: false,
		}, {
			namespaces:     []string{`test.skupper.io/test-namespace=something_else`},
			worksOnTarget:  false,
			worksElsewhere: false,
		},
	}

	cli.RunScenariosParallel(t, initSteps)

	if t.Failed() {
		t.Fatalf("Initialization failed")
	}

	for index, item := range testTable {
		t.Run(fmt.Sprintf("case-%d", index), func(t *testing.T) {
			policySpec := skupperv1.SkupperClusterPolicySpec{
				Namespaces:         item.namespaces,
				AllowIncomingLinks: true,
			}
			err = applyPolicy("generated-policy", policySpec, pub1)
			if err != nil {
				t.Fatalf("Failed to apply policy: %v", err)
				return
			}
			base.PostPolicyChangeSleep()
			waitAllGetChecks([]policyGetCheck{
				{
					allowIncoming: &item.worksOnTarget,
					cluster:       pub1,
				}, {
					allowIncoming: &item.worksElsewhere,
					cluster:       pub2,
				},
			}, nil)
			cli.RunScenarios(
				t,
				[]cli.TestScenario{
					createTokenPolicyScenario(pub1, "target", testPath, fmt.Sprintf("%d", index), item.worksOnTarget),
					createTokenPolicyScenario(pub2, "elsewhere", testPath, fmt.Sprintf("%d-elsewhere", index), item.worksElsewhere),
				},
			)
		})

	}

	// TODO move this to tearDown?
	t.Run("skupper-delete", func(t *testing.T) {

		cli.RunScenariosParallel(
			t,
			[]cli.TestScenario{
				deleteSkupperTestScenario(pub1, "pub"),
				deleteSkupperTestScenario(pub2, "prv"),
			},
		)
	})

}

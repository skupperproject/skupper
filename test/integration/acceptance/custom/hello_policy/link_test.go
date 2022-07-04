//go:build policy
// +build policy

package hello_policy

import (
	"path/filepath"
	"testing"

	skupperv1 "github.com/skupperproject/skupper/pkg/apis/skupper/v1alpha1"
	"github.com/skupperproject/skupper/test/utils/base"
	"github.com/skupperproject/skupper/test/utils/skupper/cli"
	"github.com/skupperproject/skupper/test/utils/skupper/cli/link"
)

// Uses the named token to create a link on ctx
//
// Returns a scenario with a single link.CreateTester
//
// It runs no test task, as the link may have been created but as inactive
func createLinkTestScenario(ctx *base.ClusterContext, prefix, name string, disallowed bool) (scenario cli.TestScenario) {

	scenario = cli.TestScenario{
		Name: prefixName(prefix, "connect-sites"),
		Tasks: []cli.SkupperTask{
			{
				Ctx: ctx, Commands: []cli.SkupperCommandTester{
					&link.CreateTester{
						TokenFile:       filepath.Join(testPath, name+".token.yaml"),
						Name:            name,
						Cost:            1,
						PolicyProhibits: disallowed,
					},
				},
			},
		},
	}

	return
}

// Produces a TestScenario named link-is-up/down, and checks accordingly
func linkStatusTestScenario(ctx *base.ClusterContext, prefix, name string, up bool) (scenario cli.TestScenario) {
	var statusStr string

	if up {
		statusStr = "up"
	} else {
		statusStr = "down"
	}

	scenario = cli.TestScenario{
		Name: prefixName(prefix, "link-is-"+statusStr),
		Tasks: []cli.SkupperTask{
			{
				Ctx: ctx,
				Commands: []cli.SkupperCommandTester{
					&link.StatusTester{
						Name:   name,
						Active: up,
					},
				},
			},
		},
	}

	return
}

// Returns a TestScenario that calls skupper link delete on the named link.
//
// The scenario will be called remove-link
func linkDeleteTestScenario(ctx *base.ClusterContext, prefix, name string) (scenario cli.TestScenario) {
	scenario = cli.TestScenario{
		Name: prefixName(prefix, "remove-link"),
		Tasks: []cli.SkupperTask{
			{
				Ctx: ctx,
				Commands: []cli.SkupperCommandTester{
					&link.DeleteTester{
						Name: name,
					},
				},
			},
		},
	}
	return
}

// Returns a TestScenario that confirms two sites are connected.  The check is
// done on both sides.
//
// The scenario name is validate-sites-connected, and the configuration should
// match the main hello_world test
func sitesConnectedTestScenario(pub *base.ClusterContext, prv *base.ClusterContext, prefix, linkName string) (scenario cli.TestScenario) {

	scenario = cli.TestScenario{
		Name: prefixName(prefix, "validate-sites-connected"),
		Tasks: []cli.SkupperTask{
			{Ctx: pub, Commands: []cli.SkupperCommandTester{
				// skupper status - verify sites are connected
				&cli.StatusTester{
					RouterMode:          "interior",
					ConnectedSites:      1,
					ConsoleEnabled:      true,
					ConsoleAuthInternal: true,
					PolicyEnabled:       cli.Boolp(true),
				},
			}},
			{Ctx: prv, Commands: []cli.SkupperCommandTester{
				// skupper status - verify sites are connected
				&cli.StatusTester{
					RouterMode:     "edge",
					SiteName:       "private",
					ConnectedSites: 1,
					PolicyEnabled:  cli.Boolp(true),
				},
				// skupper link status - testing all links
				&link.StatusTester{
					Name:   linkName,
					Active: true,
				},
				// skupper link status - now using link name and a 10 secs wait
				&link.StatusTester{
					Name:   linkName,
					Active: true,
					Wait:   10,
				},
			}},
		},
	}
	return
}

// Uses other functions to create a token, link the two clusters and check all is good
//
// The commands cannot be run in parallel
func connectSitesTestScenario(pub, prv *base.ClusterContext, prefix, name string) (scenario cli.TestScenario) {

	scenario = createTokenPolicyScenario(pub, prefix, testPath, name, true)
	scenario.Name = prefixName(prefix, "connect-sites")

	scenario.AppendTasks(
		createLinkTestScenario(prv, prefix, name, false),
		linkStatusTestScenario(prv, prefix, name, true),
	)

	return scenario

}

// Return a SkupperClusterPolicySpec that (dis)allows incomingLinks on the
// given namespace.
func allowIncomingLinkPolicy(namespace string, allow bool) (policySpec skupperv1.SkupperClusterPolicySpec) {
	policySpec = skupperv1.SkupperClusterPolicySpec{
		Namespaces:         []string{namespace},
		AllowIncomingLinks: allow,
	}

	return
}

// This is the main test for incoming link policy (AllowIncomingLinks), and
// uses a plain policyTestCase.
//
// There are no background policies, and keepPolicies is set as false.  So,
// every individual test case will start with a blank list of policies
// (everything is denied).
//
// The policies in this test are all applied specifically to one of the named
// namespaces, so it should be safe to run on both single and multi-cluster
// configurations.
func testLinkPolicy(t *testing.T, pub, prv *base.ClusterContext) {

	testTable := []policyTestCase{
		{
			name: "init",
			steps: []policyTestStep{
				{
					name:     "execute",
					parallel: true,
					cliScenarios: []cli.TestScenario{
						skupperInitInteriorTestScenario(pub, "", true),
						skupperInitEdgeTestScenario(prv, "", true),
					},
				},
			},
		}, {
			name: "empty-policy-fails-token-creation",
			steps: []policyTestStep{
				{
					name: "execute",
					getChecks: []policyGetCheck{
						{
							cluster:       pub,
							allowIncoming: cli.Boolp(false),
						},
					},
					cliScenarios: []cli.TestScenario{
						createTokenPolicyScenario(pub, "", testPath, "fail", false),
					},
				},
			},
		}, {
			name: "allowing-policy-allows-creation",
			steps: []policyTestStep{
				{
					name: "execute",
					pubPolicy: []skupperv1.SkupperClusterPolicySpec{
						allowIncomingLinkPolicy(pub.Namespace, true),
					},
					prvPolicy: []skupperv1.SkupperClusterPolicySpec{
						allowedOutgoingLinksHostnamesPolicy(prv.Namespace, []string{"*"}),
					},
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
						createTokenPolicyScenario(pub, "", testPath, "works", true),
						createLinkTestScenario(prv, "", "works", false),
						linkStatusTestScenario(prv, "", "works", true),
						sitesConnectedTestScenario(pub, prv, "", "works"),
					},
				}, {
					name: "remove",
					pubPolicy: []skupperv1.SkupperClusterPolicySpec{
						allowIncomingLinkPolicy(pub.Namespace, false),
					},
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
						linkStatusTestScenario(prv, "", "works", false),
					},
				}, {
					name: "re-allow",
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
						linkStatusTestScenario(prv, "again", "works", true),
						sitesConnectedTestScenario(pub, prv, "", "works"),
						linkDeleteTestScenario(prv, "", "works"),
					},
				},
			},
		}, {
			name: "previously-created-token",
			steps: []policyTestStep{
				{
					name: "prepare",
					pubPolicy: []skupperv1.SkupperClusterPolicySpec{
						allowIncomingLinkPolicy(pub.Namespace, true),
					},
					prvPolicy: []skupperv1.SkupperClusterPolicySpec{
						allowedOutgoingLinksHostnamesPolicy(prv.Namespace, []string{"*"}),
					},
					getChecks: []policyGetCheck{
						{
							allowIncoming: cli.Boolp(true),
							cluster:       pub,
						}, {
							allowIncoming: cli.Boolp(false),
							allowedHosts:  []string{"any"},
							cluster:       prv,
						},
					},
					cliScenarios: []cli.TestScenario{
						createTokenPolicyScenario(pub, "", testPath, "previous", true),
					},
				}, {
					name: "disallow-and-create-link",
					pubPolicy: []skupperv1.SkupperClusterPolicySpec{
						allowIncomingLinkPolicy(pub.Namespace, false),
					},
					getChecks: []policyGetCheck{
						{
							cluster:       pub,
							allowIncoming: cli.Boolp(false),
						},
					},
					cliScenarios: []cli.TestScenario{
						createLinkTestScenario(prv, "", "previous", false),
						linkStatusTestScenario(prv, "", "previous", false),
					},
				}, {
					name: "re-allow-and-check-link",
					pubPolicy: []skupperv1.SkupperClusterPolicySpec{
						allowIncomingLinkPolicy(pub.Namespace, true),
					},
					getChecks: []policyGetCheck{
						{
							cluster:       pub,
							allowIncoming: cli.Boolp(true),
						},
					},
					cliScenarios: []cli.TestScenario{
						linkStatusTestScenario(prv, "now", "previous", true),
						sitesConnectedTestScenario(pub, prv, "", "previous"),
						linkDeleteTestScenario(prv, "", "previous"),
					},
				},
			},
		}, {
			name: "cleanup",
			steps: []policyTestStep{
				{
					name:     "delete",
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
		testCases: testTable,
	}.run(t, pub, prv)

}

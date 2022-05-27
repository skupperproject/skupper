//go:build policy
// +build policy

package hello_policy

import (
	"fmt"
	"log"
	"os"
	"testing"

	skupperv1 "github.com/skupperproject/skupper/pkg/apis/skupper/v1alpha1"
	"github.com/skupperproject/skupper/test/utils/base"
	"github.com/skupperproject/skupper/test/utils/skupper/cli"
)

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

func testNamespace(t *testing.T, pub1, pub2 *base.ClusterContext) {
	t.Run("testNamespaceLinkTransitions", func(t *testing.T) {
		testNamespaceLinkTransitions(t, pub1, pub2)
	})

	t.Run("testNamespaceIncomingLinks", func(t *testing.T) {
		testNamespaceIncomingLinks(t, pub1, pub2)
	})
}

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
					prvPolicy: []skupperv1.SkupperClusterPolicySpec{
						allowedOutgoingLinksHostnamesPolicy(prv.Namespace, []string{"*"}),
					},
					cliScenarios: []cli.TestScenario{
						skupperInitInteriorTestScenario(pub, "", true),
						skupperInitEdgeTestScenario(prv, "", true),
					},
				}, {
					name: "connect",
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
						createTokenPolicyScenario(pub, "", "./tmp", "transition", true),
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
					cliScenarios: []cli.TestScenario{
						linkStatusTestScenario(prv, "", "transition", false),
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
					cliScenarios: []cli.TestScenario{
						linkStatusTestScenario(prv, "", "transition", true),
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
					cliScenarios: []cli.TestScenario{
						linkStatusTestScenario(prv, "", "transition", false),
					},
					getChecks: []policyGetCheck{
						{
							cluster:       pub,
							allowIncoming: cli.Boolp(false),
						},
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
					cliScenarios: []cli.TestScenario{
						linkStatusTestScenario(prv, "", "transition", true),
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
	}.run(t, pub, prv)

}
func testNamespaceIncomingLinks(t *testing.T, pub1, pub2 *base.ClusterContext) {

	var err error

	// Creating a local directory for storing the token
	testPath := "./tmp/"
	_ = os.Mkdir(testPath, 0755)

	t.Run("apply-crd", func(t *testing.T) {
		if err = removePolicies(t, pub1); err != nil {
			t.Fatalf("Failed to remove policies")
		}
		if base.ShouldSkipPolicySetup() {
			log.Print("Skipping policy setup, per environment")
			return
		}
		if err = applyCrd(t, pub1); err != nil {
			t.Fatalf("Failed to add the CRD at the start: %v", err)
			return
		}
	})

	if t.Failed() {
		t.Fatalf("CRD setup failed")
	}

	initSteps := []cli.TestScenario{
		skupperInitInteriorTestScenario(pub1, "pub", true),
		skupperInitInteriorTestScenario(pub2, "prv", true),
	}

	testTable := []namespaceTest{
		{
			namespaces:     []string{"*"},
			worksOnTarget:  true,
			worksElsewhere: true,
		},
		{
			namespaces:     []string{"public-policy-namespaces-1"},
			worksOnTarget:  true,
			worksElsewhere: false,
		},
		{
			namespaces:     []string{"public-policy-namespaces-2"},
			worksOnTarget:  false,
			worksElsewhere: true,
		},
		{
			namespaces:     []string{".*1$"},
			worksOnTarget:  true,
			worksElsewhere: false,
		},
		{
			namespaces:     []string{".*2$"},
			worksOnTarget:  false,
			worksElsewhere: true,
		},
		{
			namespaces:     []string{"public"},
			worksOnTarget:  true,
			worksElsewhere: true,
		},
		{
			namespaces:     []string{`test.skupper.io/test-namespace=policy`},
			worksOnTarget:  true,
			worksElsewhere: true,
		},
		{
			namespaces:     []string{"non-existing-label=true"},
			worksOnTarget:  false,
			worksElsewhere: false,
		},
		{ // AND-behavior for labels in a single entry
			namespaces:     []string{`test.skupper.io/test-namespace=policy,non-existing-label=true`},
			worksOnTarget:  false,
			worksElsewhere: false,
		},
		{
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
			err = applyPolicy(t, "generated-policy", policySpec, pub1)
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

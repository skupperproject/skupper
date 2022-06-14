//go:build policy
// +build policy

package hello_policy

// TODO
// - cross testing (claim on router and vice versa)
// - full setup checking (create service and expose; check they appear/disappear; perhaps even curl the service).
// - Add also different removals and reinstates of policy (actual removal, changed namespace list)

import (
	"fmt"
	"log"
	"net"
	"net/url"
	"regexp"
	"strings"
	"testing"

	skupperv1 "github.com/skupperproject/skupper/pkg/apis/skupper/v1alpha1"
	"github.com/skupperproject/skupper/test/utils/base"
	"github.com/skupperproject/skupper/test/utils/skupper/cli"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	originalRouter = "originalRouter"
	originalClaim  = "originalClaim"
	originalEdge   = "originalEdge"
	router         = "router"
	claim          = "claim"
	edge           = "edge"
)

// Return a SkupperClusterPolicySpec that allows outgoing links to the given
// hostnames (a string list, following the policy's specs) on the given
// namespace.
func allowedOutgoingLinksHostnamesPolicy(namespace string, hostnames []string) (policySpec skupperv1.SkupperClusterPolicySpec) {
	policySpec = skupperv1.SkupperClusterPolicySpec{
		Namespaces:                    []string{namespace},
		AllowedOutgoingLinksHostnames: hostnames,
	}

	return
} // A function that transforms a string into another string // It is used in the test to perform various transformations to the
// actual hostnames discovered from skupper Secret entries
type transformFunction func(string) string

// This is the basic structure of a test: it has an identifier
// name, a function that will transform the host, and whether the
// link is expected to be allowed or not, given that transformation.
type hostnamesPolicyInstructions struct {
	name           string
	transformation transformFunction
	allowed        bool
}

// This is the main test for policy item AllowedOutgoingLinksHostnames
//
// It first creates a link, just so it can inspect its Secret entries and
// capture the hostname information from it:
//
// - skupper.io/url
// - inter-router-host
// - edge-host
//
// The non-background policies applied by this test are all on the
// private context, so it should be safe to run it on both single-cluster
// and multi-cluster environments.
//
// The test itself looks like a unit test: it keeps applying different
// transformations to the hostname on the policies it applies, and then
// checking whether links can be created (on the create tester), or whether
// they go up and down (on the status tester).
//
// The intention here is to:
//
// - Run that type of testing against a real hostname, coming from the
//   environment
// - Stress the policy engine a bit
func testHostnamesPolicy(t *testing.T, pub, prv *base.ClusterContext) {

	// Normal init, plus getting hostname information from temporary link
	init := []policyTestCase{
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
				}, {
					prvPolicy: []skupperv1.SkupperClusterPolicySpec{allowedOutgoingLinksHostnamesPolicy("*", []string{"*"})},
					name:      "create-token-link",
					cliScenarios: []cli.TestScenario{
						createTokenPolicyScenario(pub, "prefix", testPath, "hostnames", true),
						// This link is temporary; we only need it to get the hostnames for later steps
						createLinkTestScenario(prv, "", "hostnames", false),
						linkStatusTestScenario(prv, "", "hostnames", true),
					},
				}, {
					// We need to know the actual hosts we'll be connecting to, so we get them from the secret
					name: "register-hostnames",
					preHook: func(context map[string]string) error {
						secret, err := prv.VanClient.KubeClient.CoreV1().Secrets(prv.Namespace).Get("hostnames", v1.GetOptions{})
						if err != nil {
							return err
						}
						url, err := url.Parse(secret.ObjectMeta.Annotations["skupper.io/url"])
						if err != nil {
							return err
						}
						host, _, err := net.SplitHostPort(url.Host)
						if err != nil {
							return err
						}
						log.Printf("registering claim host = %v", host)
						context[originalClaim] = host

						interRouterHost, ok := secret.ObjectMeta.Annotations["inter-router-host"]
						if !ok {
							return fmt.Errorf("inter-router-host not available from secret")
						}
						log.Printf("registering router host = %v", interRouterHost)
						context[originalRouter] = interRouterHost

						edgeHost, ok := secret.ObjectMeta.Annotations["edge-host"]
						if !ok {
							return fmt.Errorf("edge-host not available from secret")
						}
						log.Printf("registering edge host = %v", edgeHost)
						context[originalEdge] = edgeHost

						return nil
					},
				}, {
					name:      "remove-tmp-policy-and-link",
					prvPolicy: []skupperv1.SkupperClusterPolicySpec{allowedOutgoingLinksHostnamesPolicy("REMOVE", []string{})},
					cliScenarios: []cli.TestScenario{
						linkStatusTestScenario(prv, "", "hostnames", false),
						linkDeleteTestScenario(prv, "", "hostnames"),
					},
				},
			},
		},
	}

	cleanup := []policyTestCase{
		{
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

	// a policyTestRunner table will be generated from these
	tests := []hostnamesPolicyInstructions{
		{
			name:           "same",
			transformation: func(input string) string { return input },
			allowed:        true,
		}, {
			name: "first-dot-anchored",
			transformation: func(input string) string {
				return fmt.Sprintf("^%v$", strings.Split(input, ".")[0])
			},
			allowed: false,
		}, {
			name: "first-dot-left-anchor",
			transformation: func(input string) string {
				return fmt.Sprintf("^%v.*", strings.Split(input, ".")[0])
			},
			allowed: true,
		}, {
			name: "last-dot-anchored",
			transformation: func(input string) string {
				split := strings.Split(input, ".")
				return fmt.Sprintf("^%v$", split[len(split)-1])
			},
			allowed: false,
		}, {
			name: "last-dot-right-anchor",
			transformation: func(input string) string {
				split := strings.Split(input, ".")
				return fmt.Sprintf("%v$", split[len(split)-1])
			},
			allowed: true,
		}, {
			name: "replaced-by-hard-dots",
			transformation: func(input string) string {
				if len(input) > 3 {
					return fmt.Sprintf("\\.%v\\.", input[1:len(input)-2])
				} else {
					return "\\.\\."
				}
			},
			allowed: false,
		}, {
			name: "replaced-by-soft-dots",
			transformation: func(input string) string {
				if len(input) > 3 {
					return fmt.Sprintf(".%v.", input[1:len(input)-2])
				} else {
					return "."
				}
			},
			allowed: true,
		}, {
			name: "lots-of-dashes",
			transformation: func(input string) string {
				re := regexp.MustCompile(".(.)")
				return re.ReplaceAllString(input, "-$1")
			},
			allowed: false,
		}, {
			name: "lots-of-dots",
			transformation: func(input string) string {
				re := regexp.MustCompile(".(.)")
				return re.ReplaceAllString(input, ".$1")
			},
			allowed: true,
		}, {
			name: "something-else",
			transformation: func(input string) string {
				return "^something-else-you-are-not-naming-your-host-like-this-right$"
			},
			allowed: false,
		}, {
			name: "anchored",
			transformation: func(input string) string {
				return fmt.Sprintf("^%v$", input)
			},
			allowed: true,
		}, {
			name: "unanchored-but-shorter-than-expected",
			transformation: func(input string) string {
				return fmt.Sprintf(".%v.", input)
			},
			allowed: false,
		}, {
			name: "hardify-dots",
			transformation: func(input string) string {
				// we're replacing any "." by "\."
				re := regexp.MustCompile(`\.`)
				return fmt.Sprintf("^%v$", re.ReplaceAllString(input, `\.`))
			},
			allowed: true,
		},
	}

	// This will be reused on the creation of individual test cases, and represent
	// the steps to run when a policy is expected to allow a link
	createTester := []cli.TestScenario{
		// No hostname given here: it comes from the token
		createLinkTestScenario(prv, "", "hostnames", false),
		linkStatusTestScenario(prv, "", "hostnames", true),
		linkDeleteTestScenario(prv, "", "hostnames"),
	}

	// This will be reused on the creation of individual test cases, and represent
	// the steps to run when a policy is expected to disallow a link
	failCreateTester := []cli.TestScenario{
		createLinkTestScenario(prv, "", "hostnames", true),
	}

	// Creation testing, tied to claim hostname
	createTestTable := []policyTestCase{}

	// Here we build the items that go to createTestTable, which will be the actual
	// input to the runner
	for _, t := range tests {
		var createTestCase policyTestCase
		var scenarios []cli.TestScenario

		var allowedHosts []string
		var disallowedHosts []string

		// Select actual scenarios to run based on the expectation of the test
		if t.allowed {
			scenarios = createTester
			allowedHosts = []string{"{{.originalClaim}}", "{{.originalRouter}}", "{{.originalEdge}}"}
			disallowedHosts = []string{}
		} else {
			scenarios = failCreateTester
			allowedHosts = []string{}
			disallowedHosts = []string{"{{.originalClaim}}", "{{.originalRouter}}", "{{.originalEdge}}"}
		}

		// capture for closure
		transformation := t.transformation

		createTestCase = policyTestCase{
			name: prefixName("create", t.name),
			steps: []policyTestStep{
				{
					name: "run",
					// At each step, we transform all hostnames using the transformation
					// function of the test
					preHook: func(c map[string]string) error {
						c[claim] = transformation(c[originalClaim])
						c[router] = transformation(c[originalRouter])
						c[edge] = transformation(c[originalEdge])
						return nil
					},
					// Check to confirm, and also to ensure that the createLink step runs
					// only once it is stable.
					getChecks: []policyGetCheck{
						{
							cluster:         prv,
							allowedHosts:    allowedHosts,
							disallowedHosts: disallowedHosts,
						},
					},
					// Then we apply the changed hostnames on the policy
					prvPolicy: []skupperv1.SkupperClusterPolicySpec{
						allowedOutgoingLinksHostnamesPolicy(
							prv.Namespace,
							[]string{"{{.claim}}", "{{.router}}", "{{.edge}}"},
						),
					},
					cliScenarios: scenarios,
				},
			},
		}
		createTestTable = append(createTestTable, createTestCase)
	}

	// The actual steps used on the test cases for policies where the
	// links are supposed to be allowed or not
	statusTester := []cli.TestScenario{
		linkStatusTestScenario(prv, "", "hostnames", true),
	}
	failStatusTester := []cli.TestScenario{
		linkStatusTestScenario(prv, "", "hostnames", false),
	}

	// status testing, tied to router hostname (link being establised)
	statusTestTable := []policyTestCase{}

	for _, t := range tests {
		var statusTestCase policyTestCase
		var scenarios []cli.TestScenario

		// Select steps based on test's expectation
		if t.allowed {
			scenarios = statusTester
		} else {
			scenarios = failStatusTester
		}

		// capture for closure
		transformation := t.transformation

		statusTestCase = policyTestCase{
			name: prefixName("status", t.name),
			steps: []policyTestStep{
				{
					name: "run",
					// Same as on the create test: transform hostnames then apply policy
					// using the new values.
					preHook: func(c map[string]string) error {
						c[claim] = transformation(c[originalClaim])
						c[router] = transformation(c[originalRouter])
						c[edge] = transformation(c[originalEdge])
						return nil
					},
					// Different from the createTester, we do not need GET checks here,
					// as we're only running status checks (and no create steps).  So, we can
					// just wait until the status is as expected.
					prvPolicy: []skupperv1.SkupperClusterPolicySpec{
						allowedOutgoingLinksHostnamesPolicy(
							prv.Namespace,
							[]string{"{{.claim}}", "{{.router}}", "{{.edge}}"},
						),
					},
					cliScenarios: scenarios,
				},
			},
		}
		statusTestTable = append(statusTestTable, statusTestCase)
	}

	// The create tester keeps creating and destroying links; the status tester will only watch
	// them going up and down.  This test case is actually only a preparation for the status
	// tester: it creates the link that the status tester will watch.
	linkForStatus := []policyTestCase{
		{
			name: "create-link-for-status-testing",
			steps: []policyTestStep{
				{
					name: "create",
					prvPolicy: []skupperv1.SkupperClusterPolicySpec{
						// We start allowing anything, so we can create the link
						allowedOutgoingLinksHostnamesPolicy(prv.Namespace, []string{"*"}),
					},
					// Check to confirm, and also to ensure that the createLink step runs
					// only once it is stable.
					getChecks: []policyGetCheck{
						{
							cluster:      prv,
							allowedHosts: []string{"any-should-be-allowed"},
						},
					},
					cliScenarios: []cli.TestScenario{
						createLinkTestScenario(prv, "", "hostnames", false),
						linkStatusTestScenario(prv, "", "hostnames", true),
					},
				},
			},
		},
	}

	testTable := []policyTestCase{}
	for _, t := range [][]policyTestCase{
		init,
		createTestTable,
		linkForStatus,
		statusTestTable,
		cleanup,
	} {
		testTable = append(testTable, t...)
	}

	// This is the context used by the preHook step on the Runner.
	// It is also used elsewhere on this function.
	var context = map[string]string{}

	policyTestRunner{
		testCases:    testTable,
		contextMap:   context,
		keepPolicies: true,
		// We allow everything on both clusters, except for hostnames
		pubPolicies: []skupperv1.SkupperClusterPolicySpec{
			{
				Namespaces:              []string{"*"},
				AllowIncomingLinks:      true,
				AllowedExposedResources: []string{"*"},
				AllowedServices:         []string{"*"},
			},
		},
		prvPolicies: []skupperv1.SkupperClusterPolicySpec{
			{
				Namespaces:              []string{"*"},
				AllowIncomingLinks:      true,
				AllowedExposedResources: []string{"*"},
				AllowedServices:         []string{"*"},
			},
		},
	}.run(t, pub, prv)
}

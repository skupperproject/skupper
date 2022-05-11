//go:build policy
// +build policy

package hello_policy

import (
	"fmt"
	"log"
	"net"
	"net/url"
	"regexp"
	"strings"
	"testing"

	"github.com/skupperproject/skupper/pkg/apis/skupper/v1alpha1"
	"github.com/skupperproject/skupper/test/utils/base"
	"github.com/skupperproject/skupper/test/utils/skupper/cli"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	originalRouter = "originalRouter"
	originalClaim  = "originalClaim"
	router         = "router"
	claim          = "claim"
)

type transformFunction func(string) string

type hostnamesPolicyInstructions struct {
	name           string
	transformation transformFunction
	allowed        bool
}

func testHostnamesPolicy(t *testing.T, pub, prv *base.ClusterContext) {

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
					prvPolicy: []v1alpha1.SkupperClusterPolicySpec{allowedOutgoingLinksHostnamesPolicy("*", []string{"*"})},
					name:      "create-token-link",
					cliScenarios: []cli.TestScenario{
						createTokenPolicyScenario(pub, "prefix", "./tmp", "hostnames", true),
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

						return nil
					},
				}, {
					name:      "remove-tmp-policy-and-link",
					prvPolicy: []v1alpha1.SkupperClusterPolicySpec{allowedOutgoingLinksHostnamesPolicy("REMOVE", []string{})},
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
				re := regexp.MustCompile("\\.")
				return fmt.Sprintf("^%v$", re.ReplaceAllString(input, "\\."))
			},
			allowed: true,
		},
	}

	createTester := []cli.TestScenario{
		createLinkTestScenario(prv, "", "hostnames", false),
		linkStatusTestScenario(prv, "", "hostnames", true),
		linkDeleteTestScenario(prv, "", "hostnames"),
	}

	failCreateTester := []cli.TestScenario{
		createLinkTestScenario(prv, "", "hostnames", true),
	}

	createTestTable := []policyTestCase{}

	for _, t := range tests {
		var createTestCase policyTestCase
		var name string
		var scenarios []cli.TestScenario

		if t.allowed {
			name = "succeed"
			scenarios = createTester
		} else {
			name = "fail"
			scenarios = failCreateTester
		}

		transformation := t.transformation

		createTestCase = policyTestCase{
			name: t.name,
			steps: []policyTestStep{
				{
					name:         name,
					prvPolicy:    []v1alpha1.SkupperClusterPolicySpec{allowedOutgoingLinksHostnamesPolicy(prv.Namespace, []string{"{{.claim}}", "{{.router}}"})},
					cliScenarios: scenarios,
					preHook: func(c map[string]string) error {
						c[claim] = transformation(c[originalClaim])
						c[router] = transformation(c[originalRouter])
						return nil
					},
				},
			},
		}
		createTestTable = append(createTestTable, createTestCase)
	}

	testTable := []policyTestCase{}
	for _, t := range [][]policyTestCase{init, createTestTable, cleanup} {
		testTable = append(testTable, t...)
	}

	var context = map[string]string{}

	policyTestRunner{
		testCases:  testTable,
		contextMap: context,
		// We allow everything on both clusters, except for hostnames
		pubPolicies: []v1alpha1.SkupperClusterPolicySpec{
			{
				Namespaces:              []string{"*"},
				AllowIncomingLinks:      true,
				AllowedExposedResources: []string{"*"},
				AllowedServices:         []string{"*"},
			},
		},
		prvPolicies: []v1alpha1.SkupperClusterPolicySpec{
			{
				Namespaces:              []string{"*"},
				AllowIncomingLinks:      true,
				AllowedExposedResources: []string{"*"},
				AllowedServices:         []string{"*"},
			},
		},
	}.run(t, pub, prv)
}

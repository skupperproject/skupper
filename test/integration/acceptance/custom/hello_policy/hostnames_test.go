//go:build policy
// +build policy

package hello_policy

import (
	"net"
	"net/url"
	"testing"

	"github.com/skupperproject/skupper/pkg/apis/skupper/v1alpha1"
	"github.com/skupperproject/skupper/test/utils/base"
	"github.com/skupperproject/skupper/test/utils/skupper/cli"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type transformTarget func()

type hostnamesPolicyInstructions struct {
	name           string
	transformation mapEntryFunction
	allowed        bool
}

func testHostnamesPolicy(t *testing.T, pub, prv *base.ClusterContext) {

	var context = map[string]string{}

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
						createLinkTestScenario(prv, "", "hostnames"),
						linkStatusTestScenario(prv, "", "hostnames", true),
					},
				}, {
					name: "register-claim-hostname",
					register: func() (string, string, error) {
						secret, err := prv.VanClient.KubeClient.CoreV1().Secrets(prv.Namespace).Get("hostnames", v1.GetOptions{})
						if err != nil {
							return "", "", err
						}

						url, err := url.Parse(secret.ObjectMeta.Annotations["skupper.io/url"])
						if err != nil {
							return "", "", err
						}
						host, _, err := net.SplitHostPort(url.Host)
						if err != nil {
							return "", "", err
						}

						return "target", host, nil

					},
				}, {
					name: "register-router-hostname",
					register: func() (string, string, error) {
						secret, err := prv.VanClient.KubeClient.CoreV1().Secrets(prv.Namespace).Get("hostnames", v1.GetOptions{})
						if err != nil {
							return "", "", err
						}

						host := secret.ObjectMeta.Annotations["inter-router-host"]
						return "router", host, nil
					},
				}, {
					name:      "remove-tmp-policy-and-link",
					prvPolicy: []v1alpha1.SkupperClusterPolicySpec{allowedOutgoingLinksHostnamesPolicy("REMOVE", []string{})},
					cliScenarios: []cli.TestScenario{
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
			transformation: func() (string, string, error) { return "actual", context["target"], nil },
			allowed:        true,
		},
	}

	createTester := []cli.TestScenario{
		createLinkTestScenario(prv, "", "hostnames"),
		linkStatusTestScenario(prv, "", "hostnames", true),
		linkDeleteTestScenario(prv, "", "hostnames"),
	}

	failCreateTester := []cli.TestScenario{
		createLinkTestScenario(prv, "", "hostnames"),
		linkStatusTestScenario(prv, "", "hostnames", false),
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

		createTestCase = policyTestCase{
			name: t.name,
			steps: []policyTestStep{
				{
					name:         name,
					prvPolicy:    []v1alpha1.SkupperClusterPolicySpec{allowedOutgoingLinksHostnamesPolicy(prv.Namespace, []string{"{{.actual}}", "{{.router}}"})},
					cliScenarios: scenarios,
					register:     t.transformation,
				},
			},
		}
		createTestTable = append(createTestTable, createTestCase)
	}

	testTable := init
	for _, t := range [][]policyTestCase{createTestTable, cleanup} {
		testTable = append(testTable, t...)
	}

	policyTestRunner{
		testCases:  testTable,
		contextMap: &context,
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

//go:build policy
// +build policy

package hello_policy

import (
	"fmt"
	"net"
	"net/url"
	"testing"

	"github.com/skupperproject/skupper/pkg/apis/skupper/v1alpha1"
	"github.com/skupperproject/skupper/test/utils/base"
	"github.com/skupperproject/skupper/test/utils/skupper/cli"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type hostnamesPolicyInstructions struct {
	name           string
	transformation hookFunction
	createAllowed  bool // Link creation works, based on policy to claim host
	connectAllowed bool // Link gets connected, based on policy to router host
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
						createLinkTestScenario(prv, "", "hostnames"),
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
						context["target"] = host

						interRouterHost, ok := secret.ObjectMeta.Annotations["inter-router-host"]
						if !ok {
							return fmt.Errorf("inter-router-host not available from secret")
						}
						context["router"] = interRouterHost

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
			transformation: func(context map[string]string) error { context["actual"] = context["target"]; return nil },
			createAllowed:  true,
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

		if t.createAllowed {
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
					preHook:      t.transformation,
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

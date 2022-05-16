//go:build policy
// +build policy

package hello_policy

import (
	"log"
	"testing"

	"github.com/skupperproject/skupper/pkg/apis/skupper/v1alpha1"
	skupperv1 "github.com/skupperproject/skupper/pkg/apis/skupper/v1alpha1"
	"github.com/skupperproject/skupper/test/utils/base"
	"github.com/skupperproject/skupper/test/utils/skupper/cli"
)

//
// Return a SkupperClusterPolicySpec that allows resources to be exposed
// on given namespace.  If the list of namespaces is empty, it will default
// to the cluster's namespace
func allowResourcesPolicy(namespaces, resources []string, cluster *base.ClusterContext) (policySpec skupperv1.SkupperClusterPolicySpec, ok bool) {
	if len(namespaces) == 0 {
		namespaces = []string{cluster.Namespace}
	}
	if len(resources) == 0 {
		// No policy if no resources
		return
	}
	policySpec = skupperv1.SkupperClusterPolicySpec{
		Namespaces:              namespaces,
		AllowedExposedResources: resources,
	}
	ok = true

	return
}

type resourceDetails struct {
	allowedExposedResources []string
	testAllowed             []string
	testDisallowed          []string
	survivors               []string // these should come from the previous test untouched
	zombies                 []string // they were gone somehow; they musn't come back
	namespaces              []string // if empty,  uses the context's namespace
}

type resourceTest struct {
	name string
	pub  resourceDetails
	prv  resourceDetails
}

func testResourcesPolicy(t *testing.T, pub, prv *base.ClusterContext) {

	resourceTests := []resourceTest{
		{
			name: "allow-all",
			pub: resourceDetails{
				allowedExposedResources: []string{"*"},
				testAllowed:             []string{"deployment/hello-world-frontend"},
			},
			prv: resourceDetails{
				testDisallowed: []string{"deployment/hello-world-backend"},
			},
		}, {
			name: "not-a-regex",
			pub: resourceDetails{
				allowedExposedResources: []string{".*"},
				testDisallowed:          []string{"deployment/hello-world-frontend"},
			},
		}, {
			name: "really--not-a-regex",
			pub: resourceDetails{
				allowedExposedResources: []string{".*/.*"},
				testDisallowed:          []string{"deployment/hello-world-frontend"},
			},
		}, {
			name: "specifically",
			pub: resourceDetails{
				allowedExposedResources: []string{"deployment/hello-world-frontend"},
				testAllowed:             []string{"deployment/hello-world-frontend"},
				zombies:                 []string{"deployment/hello-world-frontend"},
			},
			prv: resourceDetails{
				allowedExposedResources: []string{"deployment/hello-world-backend"},
				testAllowed:             []string{"deployment/hello-world-backend"},
				testDisallowed:          []string{"service/hello-world-backend"},
			},
		},
	}

	init := []policyTestCase{
		{
			name: "initialize",
			steps: []policyTestStep{
				{
					name:     "init",
					parallel: true,
					cliScenarios: []cli.TestScenario{
						skupperInitInteriorTestScenario(pub, "", true),
						skupperInitEdgeTestScenario(prv, "", true),
					},
				}, {
					name: "create-token-link",
					cliScenarios: []cli.TestScenario{
						createTokenPolicyScenario(pub, "prefix", "./tmp", "resources", true),
						createLinkTestScenario(prv, "", "resources", false),
						linkStatusTestScenario(prv, "", "resources", true),
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

	testTable := []policyTestCase{}
	testCases := []policyTestCase{}

	type clusterItem struct {
		cluster *base.ClusterContext
		details resourceDetails
	}

	for _, t := range resourceTests {
		// First, check that any resources that are expected to be down are so
		// Next, confirm that zombies did not come back to life
		// Then, check that the survivors are still around
		// Finally, try to create stuff
		clusterItems := []clusterItem{
			{
				cluster: pub,
				details: t.pub,
			}, {
				cluster: prv,
				details: t.prv,
			},
		}

		testSteps := []policyTestStep{}

		// Populate the policies...
		policyTestStep := policyTestStep{
			name: "install-policy-and-check-with-get",
		}
		if policy, ok := allowResourcesPolicy(t.pub.namespaces, t.pub.allowedExposedResources, pub); ok {
			policyTestStep.pubPolicy = []skupperv1.SkupperClusterPolicySpec{policy}
		}
		if policy, ok := allowResourcesPolicy(t.prv.namespaces, t.prv.allowedExposedResources, prv); ok {
			policyTestStep.prvPolicy = []skupperv1.SkupperClusterPolicySpec{policy}
		}

		// Here, we generate the GET checks.  We know that testAllowed, testDisallowed and survivors
		// must match.  We know nothing of zombies, though: allowed or not, we need to check for them
		// with status only, instead.
		getChecks := []policyGetCheck{}
		log.Println(t)
		for _, policyItem := range clusterItems {
			allowedResources := []string{}
			disallowedResources := []string{}
			allowedResources = append(allowedResources, policyItem.details.survivors...)
			allowedResources = append(allowedResources, policyItem.details.testAllowed...)
			disallowedResources = append(disallowedResources, policyItem.details.testDisallowed...)
			getChecks = append(getChecks, policyGetCheck{
				allowedResources:    allowedResources,
				disallowedResources: disallowedResources,
				cluster:             policyItem.cluster,
			})
		}
		log.Println(getChecks)
		policyTestStep.getChecks = getChecks

		testSteps = append(testSteps, policyTestStep)

		testCase := policyTestCase{
			name:  t.name,
			steps: testSteps,
		}
		log.Println(testCase)
		testCases = append(testCases, testCase)
	}

	for _, item := range [][]policyTestCase{init, testCases, cleanup} {
		testTable = append(testTable, item...)
	}

	policyTestRunner{
		testCases:    testTable,
		keepPolicies: true,
		// We allow everything on both clusters, except for resources
		pubPolicies: []v1alpha1.SkupperClusterPolicySpec{
			{
				Namespaces:                    []string{"*"},
				AllowIncomingLinks:            true,
				AllowedOutgoingLinksHostnames: []string{"*"},
				AllowedServices:               []string{"*"},
			},
		},
		prvPolicies: []v1alpha1.SkupperClusterPolicySpec{
			{
				Namespaces:                    []string{"*"},
				AllowIncomingLinks:            true,
				AllowedOutgoingLinksHostnames: []string{"*"},
				AllowedServices:               []string{"*"},
			},
		},
	}.run(t, pub, prv)
}

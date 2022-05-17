//go:build policy
// +build policy

package hello_policy

import (
	"fmt"
	"strings"
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

type clusterItem struct {
	cluster *base.ClusterContext
	details resourceDetails
}

func splitResource(s string) (kind, name string, err error) {
	split := strings.SplitN(s, "/", 2)
	if len(split) != 2 {
		err = fmt.Errorf("resource '%v' could not be parsed", s)
		return
	}
	kind = split[0]
	name = split[1]
	return
}

func resourcePolicyStep(r resourceTest, pub, prv *base.ClusterContext, clusterItems []clusterItem) policyTestStep {

	// Populate the policies...
	step := policyTestStep{
		name: "install-policy-and-check-with-get",
	}
	if policy, ok := allowResourcesPolicy(r.pub.namespaces, r.pub.allowedExposedResources, pub); ok {
		step.pubPolicy = []skupperv1.SkupperClusterPolicySpec{policy}
	}
	if policy, ok := allowResourcesPolicy(r.prv.namespaces, r.prv.allowedExposedResources, prv); ok {
		step.prvPolicy = []skupperv1.SkupperClusterPolicySpec{policy}
	}

	// Here, we generate the GET checks.  We know what testAllowed, testDisallowed and survivors
	// must match.  We know nothing of zombies, though: allowed or not, we need to check for them
	// with status only, instead.
	getChecks := []policyGetCheck{}
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
	step.getChecks = getChecks

	return step

}

func exposeTestScenario(ctx *base.ClusterContext, kind, target string, works bool) cli.TestScenario {

	scenario := cli.TestScenario{
		Name: "expose",
		Tasks: []cli.SkupperTask{
			{
				Ctx: ctx,
				Commands: []cli.SkupperCommandTester{
					// skupper expose - expose and ensure service is available
					&cli.ExposeTester{
						TargetType:      kind,
						TargetName:      target,
						Address:         target,
						Port:            8080,
						Protocol:        "http",
						TargetPort:      8080,
						PolicyProhibits: !works,
					},
				},
			},
		},
	}
	//				// skupper status - asserts that 1 service is exposed
	//				&cli.StatusTester{
	//					RouterMode:          "interior",
	//					ConnectedSites:      1,
	//					ExposedServices:     1,
	//					ConsoleEnabled:      true,
	////					ConsoleAuthInternal: true,
	//				},
	//			}},
	//			{Ctx: prv, Commands: []cli.SkupperCommandTester{
	//				// skupper expose - exposes backend and certify it is available
	//				&cli.ExposeTester{
	//					TargetType: "deployment",
	//					TargetName: "hello-world-backend",
	//					Address:    "hello-world-backend",
	//					Port:       8080,
	//					Protocol:   "http",
	//					TargetPort: 8080,
	//				},
	//				// skupper status - asserts that there are 2 exposed services
	//				&cli.StatusTester{
	//					RouterMode:      "edge",
	//					SiteName:        "private",
	//					ConnectedSites:  1,
	//					ExposedServices: 2,
	//				},
	//			}},
	//		},
	return scenario
}

func resourceExposeStep(clusterItems []clusterItem) ([]policyTestStep, error) {
	steps := []policyTestStep{}

	for _, ci := range clusterItems {
		for _, item := range ci.details.testAllowed {
			kind, name, err := splitResource(item)
			if err != nil {
				return nil, err
			}
			steps = append(steps, policyTestStep{
				name: prefixName("expose", kind),
				cliScenarios: []cli.TestScenario{
					exposeTestScenario(ci.cluster, kind, name, true),
				},
			})
		}
		for _, item := range ci.details.testDisallowed {
			kind, name, err := splitResource(item)
			if err != nil {
				return nil, err
			}
			steps = append(steps, policyTestStep{
				name: prefixName("expose-fail", kind),
				cliScenarios: []cli.TestScenario{
					exposeTestScenario(ci.cluster, kind, name, false),
				},
			})
		}
	}

	return steps, nil
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

	profiles := []struct {
		name string
		fn   func([]clusterItem) ([]policyTestStep, error)
	}{
		{
			name: "expose",
			fn:   resourceExposeStep,
		},
	}

	for _, p := range profiles {
		for _, rt := range resourceTests {
			// First, check that any resources that are expected to be down are so
			// Next, confirm that zombies did not come back to life
			// Then, check that the survivors are still around
			// Finally, try to create stuff

			clusterItems := []clusterItem{
				{
					cluster: pub,
					details: rt.pub,
				}, {
					cluster: prv,
					details: rt.prv,
				},
			}

			testSteps := []policyTestStep{}

			policyTestStep := resourcePolicyStep(rt, pub, prv, clusterItems)
			resourceCreateStep, err := p.fn(clusterItems)
			if err != nil {
				t.Fatalf("resource creation step failed: %v", err)
			}

			testSteps = append(testSteps, policyTestStep)
			testSteps = append(testSteps, resourceCreateStep...)

			testCase := policyTestCase{
				name:  prefixName(p.name, rt.name),
				steps: testSteps,
			}
			testCases = append(testCases, testCase)
		}
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

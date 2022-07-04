//go:build policy
// +build policy

package hello_policy

import (
	"fmt"
	"sort"
	"strings"
	"testing"

	skupperv1 "github.com/skupperproject/skupper/pkg/apis/skupper/v1alpha1"
	"github.com/skupperproject/skupper/test/utils/base"
	"github.com/skupperproject/skupper/test/utils/skupper/cli"
	"github.com/skupperproject/skupper/test/utils/skupper/cli/service"
)

// Return a SkupperClusterPolicySpec that allows resources to be exposed
// on given namespace.  If the list of namespaces is empty, it will default
// to the cluster's namespace
func allowResourcesPolicy(namespaces, resources []string, cluster *base.ClusterContext) (policySpec skupperv1.SkupperClusterPolicySpec, ok bool) {
	if len(namespaces) == 0 {
		namespaces = []string{cluster.Namespace}
	}
	policySpec = skupperv1.SkupperClusterPolicySpec{
		Namespaces:              namespaces,
		AllowedExposedResources: resources,
	}
	ok = true

	return
}

// All the tests that will be executed in a single cluster/namespace
type resourceDetails struct {
	allowedExposedResources []string
	testAllowed             []string
	testDisallowed          []string
	survivors               []string // these should come from the previous test untouched
	zombies                 []string // they were gone somehow; they musn't come back
	namespaces              []string // if empty,  uses the context's namespace
}

// A single test case for resource exposing, with tests to
// be run on two clusters/namespaces
type resourceTest struct {
	name string
	pub  resourceDetails
	prv  resourceDetails
}

// This struct is used to translate a []resourceTest (which makes it easy to
// describe a resource-related policy test) into an actual []policyTestCase
// testTable (which uses the PolicyTestRunner)
type clusterItem struct {
	cluster *base.ClusterContext
	details resourceDetails
	front   bool
}

// "deployment/hello-world-front" becomes "deployment" and "hello-world-front"
// If no slash on the resource, it will be an error.
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

// Installs the policies on the clusters, and runs the GET checks to confirm they've
// been applied as expected
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
		allowedResources = append(allowedResources, policyItem.details.survivors...)
		allowedResources = append(allowedResources, policyItem.details.testAllowed...)

		disallowedResources := []string{}
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

// Returns
//
// front - for true input
// back  - for false input
//
// This is used with the clusterItem.front boolean field, to properly name
// steps.
func prefixSide(front bool) string {
	if front {
		return "front"
	} else {
		return "back"
	}
}

// This is the first step that runs commands in the test: check for disallowed services,
// zombies and survivors
func resourcePreCheckSteps(clusterItems []clusterItem) ([]policyTestStep, error) {
	// First, we check for testDisallowed: if they were there in the past cycle, we should
	// wait for them to go away.  If they were not there, they should not be around anyway.
	// Doing this first also gives time for any zombies to appear, or survivors to die unexpectedly
	// and be properly reported

	// Next, we check for survivors.  Removing something should be faster than creating it,
	// that's why it comes before zombies.

	// Finally, we check for zombies

	steps := []policyTestStep{}

	// Disallowed
	disallowedScenarios := []cli.TestScenario{}
	for _, item := range clusterItems {
		if len(item.details.testDisallowed) > 0 {
			disallowedNames := []string{}
			for _, r := range item.details.testDisallowed {
				_, name, err := splitResource(r)
				if err != nil {
					return steps, fmt.Errorf("Failed parsing resource %w", err)
				}
				disallowedNames = append(disallowedNames, name)
			}
			disallowedScenarios = append(disallowedScenarios, cli.TestScenario{
				Name: prefixName(prefixSide(item.front), "check"),
				Tasks: []cli.SkupperTask{
					{
						Ctx:      item.cluster,
						Commands: []cli.SkupperCommandTester{serviceCheckTestCommand(disallowedNames, []string{}, []string{})},
					},
				},
			})
		}
	}
	if len(disallowedScenarios) > 0 {
		disallowedStep := policyTestStep{
			name:         "check-disallowed",
			parallel:     true,
			cliScenarios: disallowedScenarios,
		}
		steps = append(steps, disallowedStep)
	}

	// Survivors
	survivorScenarios := []cli.TestScenario{}
	for _, item := range clusterItems {
		if len(item.details.survivors) > 0 {
			survivorScenarios = append(survivorScenarios, cli.TestScenario{
				Name: prefixName(prefixSide(item.front), "check"),
				Tasks: []cli.SkupperTask{
					{
						Ctx: item.cluster,
						Commands: []cli.SkupperCommandTester{
							serviceCheckTestCommand([]string{}, []string{}, item.details.survivors),
						},
					},
				},
			})
		}
	}
	if len(survivorScenarios) > 0 {
		survivorStep := policyTestStep{
			name:         "survivor",
			parallel:     true,
			cliScenarios: survivorScenarios,
		}
		steps = append(steps, survivorStep)
	}

	// Zombies
	zombieScenarios := []cli.TestScenario{}
	for _, item := range clusterItems {
		if len(item.details.zombies) > 0 {
			zombieScenarios = append(zombieScenarios, cli.TestScenario{
				Name: prefixName(prefixSide(item.front), "check"),
				Tasks: []cli.SkupperTask{
					{
						Ctx: item.cluster,
						Commands: []cli.SkupperCommandTester{
							serviceCheckTestCommand(item.details.zombies, []string{}, []string{}),
						},
					},
				},
			})
		}
	}
	if len(zombieScenarios) > 0 {
		zombieStep := policyTestStep{
			name:         "zombie",
			parallel:     true,
			cliScenarios: zombieScenarios,
		}
		steps = append(steps, zombieStep)
	}

	return steps, nil
}

// Returns a TestScenario that uses service.BindTester to bind a resource identified by kind/target,
// and confirm whether that worked or not
func bindTestScenario(ctx *base.ClusterContext, kind, target string, works bool) cli.TestScenario {

	scenario := cli.TestScenario{
		Name: "bind",
		Tasks: []cli.SkupperTask{
			{
				Ctx: ctx,
				Commands: []cli.SkupperCommandTester{
					&service.BindTester{
						ServiceName:     target,
						TargetType:      kind,
						TargetName:      target,
						Protocol:        "http",
						TargetPort:      8080,
						PolicyProhibits: !works,
					},
				},
			},
		},
	}
	return scenario
}

// Returns a TestScenario that uses cli.ExposeTester to expose a resource identified by kind/target,
// and confirm whether that worked or not
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
	return scenario
}

// Return a sorted list of keys from the map, where the corresponding value
// is true.
func sortedMapKeys(m map[string]bool) []string {
	ret := make([]string, 0, len(m))
	for k, ok := range m {
		// False on the map means the key should not be considered
		if ok {
			ret = append(ret, k)
		}
	}

	// We sort so that individual tests are listed in a stable order
	sort.Strings(ret)

	return ret

}

// This is the last step in the test: GET, zombies and survivors are good, so
// we tried to bind/expose something, and now we're confirming its state
func resourcePostCheckSteps(clusterItems []clusterItem) ([]policyTestStep, error) {
	steps := []policyTestStep{}

	for _, ci := range clusterItems {
		// These maps are sets.  They have the calculated list of bound or
		// unbound services, from the the different lists in the test
		bound := map[string]bool{}
		unbound := map[string]bool{}
		for _, item := range ci.details.testDisallowed {
			_, name, err := splitResource(item)
			if err != nil {
				return nil, err
			}
			unbound[name] = true
		}
		for _, item := range ci.details.testAllowed {
			_, name, err := splitResource(item)
			if err != nil {
				return nil, err
			}
			bound[name] = true
			// service.statusTester does not care for resource type.  So, if someone
			// allowed services but not pods — since we're looking at skupper service
			// names only —, the end result for the name would be that it was allowed
			delete(unbound, name)
		}

		var prefix string
		if ci.front {
			prefix = "front"
		} else {
			prefix = "back"
		}
		scenario := serviceCheckTestScenario(
			ci.cluster,
			prefix,
			sortedMapKeys(unbound),
			[]string{},
			sortedMapKeys(bound),
		)

		cliScenarios := []cli.TestScenario{
			scenario,
		}

		steps = append(steps, policyTestStep{
			name:         "check-exposed",
			cliScenarios: cliScenarios,
		})
	}

	return steps, nil
}

// Returns the steps to be executed when the test is on running skupper service bind
func resourceBindSteps(clusterItems []clusterItem) ([]policyTestStep, error) {
	steps := []policyTestStep{}

	for _, ci := range clusterItems {
		for _, item := range ci.details.testAllowed {
			kind, name, err := splitResource(item)
			if err != nil {
				return nil, err
			}
			steps = append(steps, policyTestStep{
				name: prefixName("bind", kind),
				cliScenarios: []cli.TestScenario{
					bindTestScenario(ci.cluster, kind, name, true),
				},
			})
		}
		for _, item := range ci.details.testDisallowed {
			kind, name, err := splitResource(item)
			if err != nil {
				return nil, err
			}
			steps = append(steps, policyTestStep{
				name: prefixName("bind-fail", kind),
				cliScenarios: []cli.TestScenario{
					bindTestScenario(ci.cluster, kind, name, false),
				},
			})
		}
	}
	return steps, nil
}

// Returns the steps to be executed when the test is on running skupper expose
func resourceExposeSteps(clusterItems []clusterItem) ([]policyTestStep, error) {
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

// This test uses a resourceTest slice to define the source test table, defining which
// resources should be allowed to be exposed, per cluster, and which tests should be
// run at each step.
//
// Note there are two 'threads' of testing going on here: one on the front-end, one on
// the backend.  This is done to increase parallelism, but may make the test a bit
// harder to understand
//
// As each thread runs on its own namespace, with namespace-specific policies, the
// test should be safe to run on multicluster environments.
func testResourcesPolicy(t *testing.T, pub, prv *base.ClusterContext) {

	// The source test table
	resourceTests := []resourceTest{
		{
			name: "allow-all",
			pub: resourceDetails{
				allowedExposedResources: []string{"*"},
				testAllowed:             []string{"deployment/hello-world-frontend"},
			},
			prv: resourceDetails{
				allowedExposedResources: []string{"*"},
				testAllowed:             []string{"deployment/hello-world-backend"},
			},
		}, {
			name: "frontend-not-a-regex--backend-survives",
			pub: resourceDetails{
				zombies:                 []string{"hello-world-frontend"},
				allowedExposedResources: []string{".*"},
				testDisallowed:          []string{"deployment/hello-world-frontend"},
			},
			prv: resourceDetails{
				allowedExposedResources: []string{"*"},
				survivors:               []string{"hello-world-backend"},
			},
		}, {
			name: "front-end-really-not-a-regex--backend-star-not-special",
			pub: resourceDetails{
				zombies:                 []string{"hello-world-frontend"},
				allowedExposedResources: []string{".*/.*"},
				testDisallowed:          []string{"deployment/hello-world-frontend"},
			},
			prv: resourceDetails{
				allowedExposedResources: []string{"*/hello-world-backend"},
				testDisallowed:          []string{"deployment/hello-world-backend"},
			},
		}, {
			name: "specifically",
			pub: resourceDetails{
				allowedExposedResources: []string{"deployment/hello-world-frontend"},
				zombies:                 []string{"hello-world-frontend"},
				testAllowed:             []string{"deployment/hello-world-frontend"},
				testDisallowed:          []string{"service/hello-world-frontend"},
			},
			prv: resourceDetails{
				allowedExposedResources: []string{"service/hello-world-backend"},
				testAllowed:             []string{"service/hello-world-backend"},
				testDisallowed:          []string{"deployment/hello-world-backend"},
				zombies:                 []string{"hello-world-backend"},
			},
		}, {
			name: "dont-go",
			pub: resourceDetails{
				allowedExposedResources: []string{"*"},
				survivors:               []string{"hello-world-frontend"},
				testAllowed:             []string{"deployment/hello-world-frontend"},
			},
			prv: resourceDetails{
				allowedExposedResources: []string{"service/hello-world-backend"},
				testAllowed:             []string{"service/hello-world-backend"},
				survivors:               []string{"hello-world-backend"},
			},
		}, {
			name: "front-remove--back-change-namespace",
			pub: resourceDetails{
				allowedExposedResources: []string{"*"},
				testDisallowed:          []string{"deployment/hello-world-frontend"},
				namespaces:              []string{"REMOVE"},
			},
			prv: resourceDetails{
				allowedExposedResources: []string{"service/hello-world-backend"},
				testDisallowed:          []string{"service/hello-world-backend"},
				namespaces:              []string{"non-existing"},
			},
		}, {
			name: "allow-again",
			pub: resourceDetails{
				allowedExposedResources: []string{"service/hello-world-frontend"},
				testAllowed:             []string{"service/hello-world-frontend"},
			},
			prv: resourceDetails{
				allowedExposedResources: []string{"*"},
				testAllowed:             []string{"deployment/hello-world-backend"},
			},
		}, {
			name: "front-swap--back-clear",
			pub: resourceDetails{
				allowedExposedResources: []string{"deployment/hello-world-frontend"},
				// The service from the past cycle should come down
				testDisallowed: []string{"service/hello-world-frontend"},
				testAllowed:    []string{"deployment/hello-world-frontend"},
			},
			prv: resourceDetails{
				allowedExposedResources: []string{},
				testDisallowed:          []string{"deployment/hello-world-backend"},
			},
		}, {
			name: "allow-all-again--no-zombies",
			pub: resourceDetails{
				allowedExposedResources: []string{"*"},
				testAllowed: []string{
					"deployment/hello-world-frontend",
					"service/hello-world-frontend",
				},
			},
			prv: resourceDetails{
				allowedExposedResources: []string{"*"},
				zombies:                 []string{"hello-world-backend"},
				testAllowed: []string{
					"deployment/hello-world-backend",
					"service/hello-world-backend",
				},
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
						createTokenPolicyScenario(pub, "prefix", testPath, "resources", true),
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

	// The core tests. init + this + cleanup will make up testTable
	testCases := []policyTestCase{}

	// We run the same tests twice: once for service create + bind, the
	// other for expose directly.  We call these two steps 'profiles'
	profiles := []struct {
		name string
		fn   func([]clusterItem) ([]policyTestStep, error)
	}{
		{
			name: "expose",
			fn:   resourceExposeSteps,
		}, {
			name: "bind",
			fn:   resourceBindSteps,
		},
	}

	// Here, we transform []resourceTest into []policyTestCase.
	//
	// The first makes it easier to write and understand the test, the later is what
	// the runner expects
	for _, p := range profiles {
		for _, rt := range resourceTests {
			// The steps of the generated test:
			//
			// First, check that any resources that are expected to be down are so
			// Then, check that the survivors are still around
			// Next, confirm that zombies did not come back to life
			// Finally, try to create stuff
			// And test it

			clusterItems := []clusterItem{
				{
					cluster: pub,
					details: rt.pub,
					front:   true,
				}, {
					cluster: prv,
					details: rt.prv,
					front:   false,
				},
			}

			testSteps := []policyTestStep{}

			policySetupStep := resourcePolicyStep(rt, pub, prv, clusterItems)
			preCheckSteps, err := resourcePreCheckSteps(clusterItems)
			if err != nil {
				t.Fatalf("resource pre-check step definition failed: %v", err)
			}
			// Pick between resourceExposeSteps and resourceBindSteps, as appropriate for
			// the profile being tested
			resourceCreateStep, err := p.fn(clusterItems)
			if err != nil {
				t.Fatalf("resource creation step definition failed: %v", err)
			}
			postCheckSteps, err := resourcePostCheckSteps(clusterItems)
			if err != nil {
				t.Fatalf("resource post-check step definition failed: %v", err)
			}

			testSteps = append(testSteps, policySetupStep)
			testSteps = append(testSteps, preCheckSteps...)
			testSteps = append(testSteps, resourceCreateStep...)
			testSteps = append(testSteps, postCheckSteps...)

			testCase := policyTestCase{
				name:  prefixName(p.name, rt.name),
				steps: testSteps,
			}
			testCases = append(testCases, testCase)
		}
	}

	// The final test table
	testTable := []policyTestCase{}

	for _, item := range [][]policyTestCase{init, testCases, cleanup} {
		testTable = append(testTable, item...)
	}

	policyTestRunner{
		testCases:    testTable,
		keepPolicies: true,
		// We allow everything on both clusters, except for resources
		pubPolicies: []skupperv1.SkupperClusterPolicySpec{
			{
				Namespaces:                    []string{"*"},
				AllowIncomingLinks:            true,
				AllowedOutgoingLinksHostnames: []string{"*"},
				AllowedServices:               []string{"*"},
			},
		},
		prvPolicies: []skupperv1.SkupperClusterPolicySpec{
			{
				Namespaces:                    []string{"*"},
				AllowIncomingLinks:            true,
				AllowedOutgoingLinksHostnames: []string{"*"},
				AllowedServices:               []string{"*"},
			},
		},
	}.run(t, pub, prv)
}

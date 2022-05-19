//go:build policy
// +build policy

package hello_policy

import (
	"fmt"
	"log"
	"sort"
	"strings"
	"testing"

	"github.com/skupperproject/skupper/pkg/apis/skupper/v1alpha1"
	skupperv1 "github.com/skupperproject/skupper/pkg/apis/skupper/v1alpha1"
	"github.com/skupperproject/skupper/test/utils/base"
	"github.com/skupperproject/skupper/test/utils/skupper/cli"
)

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

// All the tests that will be executed in a single cluster
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

func resourcePolicyStepPreCheckTask(ctx *base.ClusterContext, d resourceDetails) (cli.SkupperTask, bool) {
	// First, we check for testDisallowed: if they were there in the past cycle, we should
	// wait for them to go away.  If they were not there, they should not be around anyway.
	// Doing this first also gives time for any zombies to appear, or survivors to die unexpectedly
	// and be properly reported

	// Next, we check for survivors.  Removing something should be faster than creating it,
	// that's why it comes before zombies.

	// Finally, we check for zombies

	disallowedNames := []string{}

	for _, r := range d.testDisallowed {
		_, name, err := splitResource(r)
		if err != nil {
			log.Printf("Failed parsing resource %v", r)
		}
		disallowedNames = append(disallowedNames, name)
	}

	commands := []cli.SkupperCommandTester{}
	commands = append(commands, serviceCheckTestCommand(disallowedNames, []string{}, []string{}))
	commands = append(commands, serviceCheckTestCommand([]string{}, []string{}, d.survivors))
	commands = append(commands, serviceCheckTestCommand(d.zombies, []string{}, []string{}))

	ret := cli.SkupperTask{
		Ctx:      ctx,
		Commands: commands,
	}

	var result bool
	if len(commands) > 0 {
		result = true
	}

	return ret, result
}

// Installs the policies on the clusters, and runs the GET checks to confirm they've
// been applied as expected
func resourcePolicyStep(r resourceTest, pub, prv *base.ClusterContext, clusterItems []clusterItem) policyTestStep {

	// Populate the policies...
	step := policyTestStep{
		name:     "install-policy-and-check-with-get",
		parallel: true,
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

	cliScenarios := []cli.TestScenario{}

	// TODO REFACTOR
	// Instead of two tasks with one command for each disallowed, zombie, survivor
	// Do disallowed in parallel on both clusters, then zombies and survivors in the same
	// manner.  This would better identify on the logs what's being done
	for _, item := range []struct {
		name    string
		ctx     *base.ClusterContext
		details resourceDetails
	}{
		{
			"pub",
			pub,
			r.pub,
		}, {
			"prv",
			prv,
			r.prv,
		},
	} {
		if cliTask, ok := resourcePolicyStepPreCheckTask(item.ctx, item.details); ok {
			scenario := cli.TestScenario{
				Name:  prefixName(item.name, "check-pre"),
				Tasks: []cli.SkupperTask{cliTask},
			}
			cliScenarios = append(cliScenarios, scenario)
		}
	}

	//	cliScenario := cli.TestScenario{
	//		Name:  "check-pre",
	//		Tasks: []cli.SkupperTask{},
	//	}
	//	tasks := []cli.SkupperTask{}
	//	if pubCli, ok := resourcePolicyStepPreCheckTask(pub, r.pub); ok {
	//		tasks = append(tasks, pubCli)
	//	}
	//	if prvCli, ok := resourcePolicyStepPreCheckTask(prv, r.prv); ok {
	//		tasks = append(tasks, prvCli)
	//	}
	if len(cliScenarios) > 0 {
		step.cliScenarios = cliScenarios
	}

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
			name: "front-end-really-not-a-regex--backend-survives",
			pub: resourceDetails{
				zombies:                 []string{"hello-world-frontend"},
				allowedExposedResources: []string{".*/.*"},
				testDisallowed:          []string{"deployment/hello-world-frontend"},
			},
			prv: resourceDetails{
				allowedExposedResources: []string{"deployment/hello-world-backend"},
				survivors:               []string{"hello-world-backend"},
			},
		}, {
			name: "specifically",
			pub: resourceDetails{
				allowedExposedResources: []string{"deployment/hello-world-frontend"},
				zombies:                 []string{"hello-world-frontend"},
				testAllowed:             []string{"deployment/hello-world-frontend"},
			},
			prv: resourceDetails{
				allowedExposedResources: []string{"service/hello-world-backend"},
				testAllowed:             []string{"service/hello-world-backend"},
				testDisallowed:          []string{"deployment/hello-world-backend"},
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

	// We run the same tests twice: once for service create + bind, the
	// other for expose directly.  We call these two steps 'profiles'
	profiles := []struct {
		name string
		fn   func([]clusterItem) ([]policyTestStep, error)
	}{
		{
			name: "expose",
			fn:   resourceExposeStep,
		}, // TOODO resourceBindStep
	}

	for _, p := range profiles {
		for _, rt := range resourceTests {
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

			policyTestStep := resourcePolicyStep(rt, pub, prv, clusterItems)
			resourceCreateStep, err := p.fn(clusterItems)
			if err != nil {
				t.Fatalf("resource creation step definition failed: %v", err)
			}
			checkSteps, err := resourcePostCheckSteps(clusterItems)
			if err != nil {
				t.Fatalf("resource check step definition failed: %v", err)
			}

			testSteps = append(testSteps, policyTestStep)
			testSteps = append(testSteps, resourceCreateStep...)
			testSteps = append(testSteps, checkSteps...)

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

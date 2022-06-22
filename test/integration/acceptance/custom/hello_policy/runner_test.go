//go:build policy
// +build policy

package hello_policy

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"text/template"
	"time"

	"github.com/skupperproject/skupper/client"
	skupperv1 "github.com/skupperproject/skupper/pkg/apis/skupper/v1alpha1"
	"github.com/skupperproject/skupper/pkg/utils"
	"github.com/skupperproject/skupper/test/utils/base"
	"github.com/skupperproject/skupper/test/utils/skupper/cli"
)

// TODO:
// - on cli.RunScenarios, environment option to bounce pods between each command

// Removes all policies from all given clusters
func wipePolicies(ctx ...*base.ClusterContext) error {

	for _, c := range ctx {
		err := removePolicies(c)
		if err != nil {
			return fmt.Errorf("failed removing policies from %v: %w", c.Namespace, err)
		}
	}

	return nil

}

// Runs each policyTestCase in turn
//
// By default, all policies are removed between the tests cases, but that can
// be controlled with keepPolicies.  Notice that between steps in a test case
// the policies are never removed by the runner (unless explicitly requested in
// the step).
type policyTestRunner struct {
	testCases    []policyTestCase
	keepPolicies bool
	pubPolicies  []skupperv1.SkupperClusterPolicySpec
	prvPolicies  []skupperv1.SkupperClusterPolicySpec
	contextMap   map[string]string // user needs to initialize if using
}

// Runs each test case in turn
func (r policyTestRunner) run(t *testing.T, pub, prv *base.ClusterContext) {

	err := wipePolicies(pub, prv)
	if err != nil {
		t.Fatalf("Unable to remove policies: %v", err)
	}
	if len(r.pubPolicies)+len(r.prvPolicies) > 0 {
		t.Run(
			"background-policy-setup",
			func(t *testing.T) {
				for i, policy := range r.pubPolicies {
					i := strconv.Itoa(i)
					err := applyPolicy("background-pub-policy-"+i, policy, pub)
					if err != nil {
						t.Fatalf("Failed to apply policy: %v", err)
					}
				}
				for i, policy := range r.prvPolicies {
					i := strconv.Itoa(i)
					err := applyPolicy("background-prv-policy-"+i, policy, prv)
					if err != nil {
						t.Fatalf("Failed to apply policy: %v", err)
					}
				}

			})
	}

	for _, testCase := range r.testCases {
		if !r.keepPolicies {
			err = removeAllPoliciesExcept(pub, []regexp.Regexp{*regexp.MustCompile("^background-.*")})
			if err != nil {
				t.Fatalf("Failed removing or preserving policies: %v", err)
			}
			err = removeAllPoliciesExcept(prv, []regexp.Regexp{*regexp.MustCompile("^background-.*")})
			if err != nil {
				t.Fatalf("Failed removing or preserving policies: %v", err)
			}
		}
		if base.IsTestInterrupted() {
			break
		}
		testCase.run(t, pub, prv, r.contextMap)
	}
	err = wipePolicies(pub, prv)
	if err != nil {
		t.Fatalf("Unable to remove policies: %v", err)
	}
}

// A function to programatically control the skipping of policyTestCase
// and policyTestStep.  An empty response means no skip; a non-empty
// response is used as the reason for skipping.
type skipFunction func() string

// A named slice, with methods to run each step
type policyTestCase struct {
	name  string
	steps []policyTestStep
	skip  skipFunction
	// TODO: Add a context, so that tests that are known to run for very
	// 	 long time when they fail can have their runtimes capped
}

// Runs the individual steps in a test case.  The test case is an individual
// Go test
func (c policyTestCase) run(t *testing.T, pub, prv *base.ClusterContext, contextMap map[string]string) {

	t.Run(
		c.name,
		func(t *testing.T) {
			if c.skip != nil {
				if skipReason := c.skip(); skipReason != "" {
					t.Skipf(skipReason)
				}
			}
			for _, step := range c.steps {

				step.run(t, pub, prv, contextMap)
				if base.IsTestInterrupted() {
					break
				}
			}
			base.StopIfInterrupted(t)
		})
}

// A hook function is executed right at the start of a policyTestStep.  It can
// be a closure that captures variables during test table definition, and that
// executes when the actual test executes, so it can perform things that can't
// be done during definition.
type hookFunction func(map[string]string) error

// Configures a step on the policy test runner, which allows for setting
// policies on the two clusters, check the policy status with `get` commands
// and run a set of cli command scenarios, along with some other helper steps.
//
// ATTENTION to how the policy lists (pubPolicy, prvPolicy) work:
// - Each item on the list will generate a policy named pub/prv-policy-i,
//   based on their position on the list (i is an index)
// - Every time a list is defined, each of its items will be either updated
//   or created
//
// So, if the previous step defined two public policies, and the current step...
//
// - defines none: nothing is changed; the two policies stay in place
// - defines only one: the first policy is updated; the second one is not touched
// - defines two policies: both are updated
// - defined three policies: the first two are updated; the third one created
//
// You may use this behavior on your tests, by placing changing policies at the
// start of the list, and never-changing at the end, so your updates will simply
// have the first one or two policies listed.  However, be careful, it is easy
// to overlook this behavior causing weird test errors.
//
// When you have more than one policy and you're not updating all, it may be
// good to document it on the struct.  Something like this:
//
//   pubPolicy: []skupperv1.SkupperClusterPolicySpec{
//     allowIncomingLinkPolicy(pub.Namespace, true),
//     // second policy is not being changed on this test
//  },
//
// To remove a policy, set it as having a sole namespace named REMOVE.  To keep
// a policy while updating or removing another one that follows it, set it with
// a sole namespace named KEEP.
//
// Right after the policy is set up, the getChecks verifications will run: those
// are run on the service-controller container, against the `get` command.  These
// checks work in a retry loop with a timeout, so they can be used to wait for the
// policy changes to stabilize before running the CLI commands.
//
// After all work for the step is done, it can optionally sleep for a configured
// duration of time, using time.Sleep().  Do not use the sleep for normal testing,
// as it may hide errors.  Use it only for specialized testing where the time
// between steps is paramount to the test itself.
//
// The very first step executed when a policyTestStep is run is the preHook, if
// configured.  That's a call to a function in the form func(map[string]string)error,
// that receives a context map.  One can use the preHook function to operate on
// the context map, or to do any other operations that cannot be done at the time
// the test table is defined (for example, if it depends on previous steps).
//
// The context map is also used when applying policies, so the key/values pairs
// there can be accessed as Go Templates on the policies' items.
//
// Note: When writing the tests, try to keep the fields in the same order as
// they appear on the struct definition.  That's the order in which they'll be run,
// and that helps the test reader understand when each step will execute.
type policyTestStep struct {
	name         string
	preHook      hookFunction
	pubPolicy    []skupperv1.SkupperClusterPolicySpec // ATTENTION to usage; see doc
	prvPolicy    []skupperv1.SkupperClusterPolicySpec
	getChecks    []policyGetCheck
	cliScenarios []cli.TestScenario

	// configuration
	parallel bool // This will run the cliScenarios parallel
	sleep    time.Duration

	// if provided, skipFunction will be run and its result checked.  If not empty,
	// the test will be skipped with the return string as the input to t.Skip().
	// This allows to programatically skip some of the steps, based on environmental
	// information.
	skip skipFunction
}

// Runs the TestStep as an individual Go Test
func (s policyTestStep) run(t *testing.T, pub, prv *base.ClusterContext, contextMap map[string]string) {
	t.Run(
		s.name,
		func(t *testing.T) {
			if s.skip != nil {
				var skipResult = s.skip()
				if skipResult != "" {
					t.Skip(skipResult)
				}
			}
			s.installCliErrorHook(t)
			s.runPreHook(t, contextMap)
			s.applyPolicies(t, pub, prv, contextMap)
			s.waitChecks(t, contextMap)
			s.runCommands(t)

			if s.sleep.Nanoseconds() > 0 {
				log.Printf("Sleeping for %v", s.sleep)
				time.Sleep(s.sleep)
			}
		})
}

// Update the configured cliScenarios to have an ErrorHook that runs
// testRunner.DumpTestInfo()
func (s policyTestStep) installCliErrorHook(t *testing.T) {
	for i, scenario := range s.cliScenarios {
		s.cliScenarios[i].ErrorHook = func() {
			path := filepath.Join(
				testPath,
				strings.ReplaceAll(t.Name()+"-"+scenario.Name, "/", "-"),
			)
			_ = os.Mkdir(path, 0755)
			testRunner.DumpTestInfo(path)
		}
	}

}

func (s policyTestStep) runPreHook(t *testing.T, contextMap map[string]string) {
	if s.preHook == nil {
		return
	}
	err := s.preHook(contextMap)
	if err != nil {
		t.Fatalf("preHook step failed: %v", err)
	}
}

// Apply all policies, on pub and prv
//
// See policyTestStep documentation for behavior
func (s policyTestStep) applyPolicies(t *testing.T, pub, prv *base.ClusterContext, contextMap map[string]string) {

	if len(s.pubPolicy)+len(s.prvPolicy) > 0 {
		t.Run(
			"policy-setup",
			func(t *testing.T) {
				apply := []struct {
					policyList []skupperv1.SkupperClusterPolicySpec
					cluster    *base.ClusterContext
					prefix     string
				}{
					{
						policyList: s.pubPolicy,
						cluster:    pub,
						prefix:     "pub",
					}, {
						policyList: s.prvPolicy,
						cluster:    prv,
						prefix:     "prv",
					},
				}

				for _, item := range apply {
					for i, policy := range item.policyList {
						i := strconv.Itoa(i)
						policyName := prefixName(item.prefix, "policy-"+i)

						var err error

						if len(policy.Namespaces) == 1 {
							// Check if the namespace is actually a sentinel
							switch policy.Namespaces[0] {
							case "REMOVE":
								err = removePolicies(item.cluster, policyName)
								if err != nil {
									t.Fatalf("Failed to remove policy: %v", err)
								}
								continue
							case "KEEP":
								// We're just not doing anything with this one
								continue
							}
						}

						templatedPolicySpec, err := templatePolicySpec(policy, contextMap)
						if err != nil {
							t.Fatalf("Failed to template policy %v: %v", policy, err)
						}

						err = applyPolicy(policyName, templatedPolicySpec, item.cluster)
						if err != nil {
							t.Fatalf("Failed to apply policy: %v", err)
						}
					}
				}

			})
		base.PostPolicyChangeSleep()
	}
}

// Templates each of the strings using the map c, and return the result
func templateStringList(c map[string]string, l ...string) ([]string, error) {
	if len(l) == 0 {
		return l, nil
	}

	var ret = make([]string, 0, len(l))

	for _, item := range l {
		buf := &bytes.Buffer{}
		tmpl, err := template.New("").Parse(item)
		if err != nil {
			return ret, err
		}
		err = tmpl.Execute(buf, c)
		if err != nil {
			return ret, err
		}
		ret = append(ret, buf.String())
	}
	return ret, nil
}

// Runs a template over each string item in a skupperv1.SkupperClusterPolicy spec
func templatePolicySpec(p skupperv1.SkupperClusterPolicySpec, c map[string]string) (skupperv1.SkupperClusterPolicySpec, error) {
	if len(c) == 0 {
		return p, nil
	}

	namespaces, err := templateStringList(c, p.Namespaces...)
	if err != nil {
		return p, err
	}
	allowedOutgoingLinksHostnames, err := templateStringList(c, p.AllowedOutgoingLinksHostnames...)
	if err != nil {
		return p, err
	}
	allowedExposedResources, err := templateStringList(c, p.AllowedExposedResources...)
	if err != nil {
		return p, err
	}
	allowedServices, err := templateStringList(c, p.AllowedServices...)
	if err != nil {
		return p, err
	}

	newPolicySpec := skupperv1.SkupperClusterPolicySpec{
		Namespaces:                    namespaces,
		AllowIncomingLinks:            p.AllowIncomingLinks,
		AllowedOutgoingLinksHostnames: allowedOutgoingLinksHostnames,
		AllowedExposedResources:       allowedExposedResources,
		AllowedServices:               allowedServices,
	}

	return newPolicySpec, err
}

// Wait for all checks to succeed, unless configured otherwise
func (s policyTestStep) waitChecks(t *testing.T, contextMap map[string]string) {
	if base.ShouldPolicyWaitOnGet() {
		err := waitAllGetChecks(s.getChecks, contextMap)
		if err != nil {
			t.Errorf("GET check wait failed: %v", err)
		}
	} else {
		if len(s.getChecks) > 0 {
			log.Printf("Running single GET checks, as configured on the environment")
			for _, check := range s.getChecks {
				ok, err := check.check(contextMap)
				if err != nil {
					errMsg := fmt.Sprintf("GET check %v failed: %v", check, err)
					log.Print(errMsg)
					t.Errorf(errMsg)
				}
				if !ok {
					errMsg := fmt.Sprintf("GET check %v returned incorrect response", check)
					log.Print(errMsg)
					t.Errorf(errMsg)
				}
			}
			log.Printf("All tests pass")
		}
	}
}

// Run the commands part of the policyTestStep
func (s policyTestStep) runCommands(t *testing.T) {
	if s.parallel {
		cli.RunScenariosParallel(t, s.cliScenarios)
	} else {
		cli.RunScenarios(t, s.cliScenarios)
	}
}

// This will run the configured checks using client.NewPolicyValidatorAPI
type policyGetCheck struct {
	allowIncoming       *bool
	allowedHosts        []string
	disallowedHosts     []string
	allowedServices     []string
	disallowedServices  []string
	allowedResources    []string
	disallowedResources []string
	cluster             *base.ClusterContext
}

// fmt.Stringer implementation, to make %v for policyGetCheck more consise
// and informative
func (c policyGetCheck) String() string {
	var ret []string

	if c.allowIncoming != nil {
		ret = append(ret, fmt.Sprintf("allowIncoming:%v", *c.allowIncoming))
	}

	if c.cluster != nil {
		ret = append(ret, fmt.Sprintf("namespace:%v", c.cluster.Namespace))
	}

	lists := []struct {
		name     string
		contents []string
	}{
		{"allowedHosts", c.allowedHosts},
		{"disallowedHosts", c.disallowedHosts},
		{"allowedServices", c.allowedServices},
		{"disallowedServices", c.disallowedServices},
		{"allowedResources", c.allowedResources},
		{"disallowedResources", c.disallowedResources},
	}

	for _, l := range lists {
		if len(l.contents) > 0 {
			ret = append(ret, fmt.Sprintf("%v:%v", l.name, l.contents))
		}
	}

	return fmt.Sprintf("policyGetCheck{%v}", strings.Join(ret, " "))
}

// This will stand for one of the functions in client.PolicyAPIClient, below
type checkString func(string) (*client.PolicyAPIResult, error)

// Run all configured checks.  Runs all checks, even if prior checks did not
// correspond to the expectation, unless an error is returned in any steps.
//
// If provided, the contextMap will be used to template the strings in the
// policyGetCheck structure.  It can be set to nil to disable templating.
func (p policyGetCheck) check(contextMap map[string]string) (ok bool, err error) {
	ok = true
	var c = client.NewPolicyValidatorAPI(p.cluster.VanClient)

	// allowIncoming
	if p.allowIncoming != nil {

		var res *client.PolicyAPIResult

		res, err = c.IncomingLink()

		if err != nil {
			ok = false
			log.Printf("IncomingLink check failed with error %v", err)
			return
		}

		if res.Allowed != *p.allowIncoming {
			log.Printf("Unexpected IncomingLink result (%v)", res.Allowed)
			ok = false
		}
	}

	// allowedHosts and allowedServices
	// All tests are very similar, so we run them with a table
	lists := []struct {
		name     string
		list     []string
		function checkString
		expect   bool
	}{
		{
			name:     "allowedHosts",
			list:     p.allowedHosts,
			function: c.OutgoingLink,
			expect:   true,
		}, {
			name:     "disallowedHosts",
			list:     p.disallowedHosts,
			function: c.OutgoingLink,
			expect:   false,
		}, {
			name:     "allowedServices",
			list:     p.allowedServices,
			function: c.Service,
			expect:   true,
		}, {
			name:     "disallowedServices",
			list:     p.disallowedServices,
			function: c.Service,
			expect:   false,
		},
	}

	for _, list := range lists {
		// If not configured, just move on
		if len(list.list) == 0 {
			continue
		}
		// Template the list, in case we want to get something from the context
		templatedList, err := templateStringList(contextMap, list.list...)
		if err != nil {
			log.Printf("Failed templating %v: %v", list.name, err)
			return false, err
		}
		// Run the configured function for each element on the list
		for _, element := range templatedList {
			var res *client.PolicyAPIResult
			res, err = list.function(element)
			if err != nil {
				log.Printf("%v check failed with error %v", list.name, err)
				return false, err
			}
			if res.Allowed != list.expect {
				log.Printf("Unexpected %v result for %v (%v)", list.name, element, res.Allowed)
				ok = false
			}
		}

	}

	// allowedResources is different from the others, as its function takes two
	// arguments.  Still, allowed and disallowed follow similar paths, so we
	// use a table
	resourceItems := []struct {
		allow bool
		list  []string
	}{
		{
			allow: true,
			list:  p.allowedResources,
		}, {
			allow: false,
			list:  p.disallowedResources,
		},
	}
	for _, resourceItem := range resourceItems {
		// Template the list, in case we want to get something from the context
		templatedList, err := templateStringList(contextMap, resourceItem.list...)
		if err != nil {
			log.Printf("Failed templating exposed resources (%v): %v", resourceItem.allow, err)
			return false, err
		}
		for _, element := range templatedList {
			var res *client.PolicyAPIResult
			splitted := strings.SplitN(element, "/", 2)
			if len(splitted) != 2 {
				// TODO: should we try to do something else, instead?  Fail, perhaps?
				log.Printf("Ignoring GET check for resource without '/': %v", element)
				continue
			}
			res, err = c.Expose(splitted[0], splitted[1])
			if err != nil {
				log.Printf("Resource check failed with error %v", err)
				return false, err
			}
			if res.Allowed != resourceItem.allow {
				log.Printf("Unexpected resource result: %v(%v)", element, res.Allowed)
				ok = false
			}
		}
	}

	return
}

// This will keep running all GetChecks in the slice, until all
// of them return true in the same cycle, or until timeout
//
// As policy changes are supposed to be very quick, the checks
// will run with a one second interval.
//
// If provided, the contextMap will be used to template the strings
// in the policyGetCheck structure.  It can be set to nil to disable
// templating.
func waitAllGetChecks(checks []policyGetCheck, contextMap map[string]string) error {
	if len(checks) == 0 {
		// nothing to check
		return nil
	}
	var attempts int
	ctx, cancelFn := context.WithTimeout(context.Background(), time.Minute*2)
	defer cancelFn()
	err := utils.RetryWithContext(ctx, time.Second, func() (bool, error) {
		attempts++
		log.Printf("Running GET checks -- attempt %v", attempts)
		if base.IsTestInterrupted() {
			return false, fmt.Errorf("test interrupted by user")
		}
		var allGood = true
		for _, check := range checks {
			ok, err := check.check(contextMap)
			if err != nil {
				log.Printf("Error on GET check: %v", err)
				allGood = false
			}
			if !ok {
				log.Printf("Check %+v failed validation", check)
				allGood = false
			}
		}
		if allGood {
			log.Printf("All checks pass")
			return true, nil
		}
		return false, nil
	})

	return err

}

//go:build policy
// +build policy

package hello_policy

import (
	"fmt"
	"log"
	"regexp"
	"strconv"
	"testing"
	"time"

	"github.com/skupperproject/skupper/client"
	"github.com/skupperproject/skupper/pkg/apis/skupper/v1alpha1"
	skupperv1 "github.com/skupperproject/skupper/pkg/apis/skupper/v1alpha1"
	"github.com/skupperproject/skupper/test/utils/base"
	"github.com/skupperproject/skupper/test/utils/skupper/cli"
)

// TODO:
// - If a scenario fails, show events and logs?
// - on cli.RunScenarios, environment option to bounce pods between each command

func wipePolicies(t *testing.T, ctx ...*base.ClusterContext) error {

	for _, c := range ctx {
		err := removePolicies(t, c)
		if err != nil {
			return fmt.Errorf("failed removing policies from %v: %w", c.Namespace, err)
		}
	}

	return nil

}

// Each policy piece has its own file.  On it, we define both the
// piece-specific tests _and_ the piece-specific infra.
//
// For example, the checking for link being (un)able to create or being
// destroyed is defined on functions on link_test.go
//
// These functions will take a cluster context and an optional name prefix.  They
// will return a cli.TestScenario with the intended objective on the
// requested cluster, and the name of the scenario will receive
// the prefix, if any given.  A use of that prefix would be, for example, to
// clarify that what's being checked is a 'side-effect' (eg when a link drops
// in a cluster because the policy was removed on the other cluster)
//
// policyTestRunner
//   []policyTestCase
//     []policyTestStep
//         policies
//         cli commands
//         GET checks
//         post-step sleep

// Runs each policyTestCase in turn
//
// By default, all policies are removed between the tests cases, but that can be
// controlled with keepPolicies
type policyTestRunner struct {
	testCases    []policyTestCase
	keepPolicies bool
	pubPolicies  []v1alpha1.SkupperClusterPolicySpec
	prvPolicies  []v1alpha1.SkupperClusterPolicySpec
}

// Runs each test case in turn
func (r policyTestRunner) run(t *testing.T, pub, prv *base.ClusterContext) {

	err := wipePolicies(t, pub, prv)
	if err != nil {
		t.Fatalf("Unable to remove policies: %w", err)
	}
	if len(r.pubPolicies)+len(r.prvPolicies) > 0 {
		t.Run(
			"background-policy-setup",
			func(t *testing.T) {
				for i, policy := range r.pubPolicies {
					i := strconv.Itoa(i)
					err := applyPolicy(t, "background-pub-policy-"+i, policy, pub)
					if err != nil {
						t.Fatalf("Failed to apply policy: %v", err)
					}
				}
				for i, policy := range r.prvPolicies {
					i := strconv.Itoa(i)
					err := applyPolicy(t, "background-prv-policy-"+i, policy, prv)
					if err != nil {
						t.Fatalf("Failed to apply policy: %v", err)
					}
				}

			})
	}

	for _, testCase := range r.testCases {
		if !r.keepPolicies {
			keepPolicies(t, pub, []regexp.Regexp{*regexp.MustCompile("^background-.*")})
			keepPolicies(t, prv, []regexp.Regexp{*regexp.MustCompile("^background-.*")})
		}
		if base.IsTestInterrupted() {
			break
		}
		testCase.run(t, pub, prv)
	}
	err = wipePolicies(t, pub, prv)
	if err != nil {
		t.Fatalf("Unable to remove policies: %v", err)
	}
}

// A named slice, with methods to run each step
type policyTestCase struct {
	name  string
	steps []policyTestStep
	// TODO: Add a context, so that tests that are known to run for very
	// 	 long time when they fail can have their runtimes capped
}

// Runs the individual steps in a test case.  The test case is an individual
// Go test
func (c policyTestCase) run(t *testing.T, pub, prv *base.ClusterContext) {

	t.Run(
		c.name,
		func(t *testing.T) {
			for _, step := range c.steps {

				step.run(t, pub, prv)
				if base.IsTestInterrupted() {
					break
				}
			}
			base.StopIfInterrupted(t)
		})
}

type skipFunction func() string

// Configures a step on the policy test runner, which allows for setting
// policies on the two clusters, run a set of cli command scenarios and then perform
// some checks using the `get` command.
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
// After all work for the step is done, it can optionally sleep for a configured
// duration of time, using time.Sleep().  Do not use the sleep for normal testing,
// as it may hide errors.  Use it only for specialized testing where the time
// between steps is paramount to the test itself.
type policyTestStep struct {
	name         string
	pubPolicy    []skupperv1.SkupperClusterPolicySpec // ATTENTION to usage; see doc
	prvPolicy    []skupperv1.SkupperClusterPolicySpec
	cliScenarios []cli.TestScenario
	parallel     bool           // This will run the cliScenarios parallel
	pubGetCheck  policyGetCheck // TODO remove this, refactor
	prvGetCheck  policyGetCheck // TODO remove this, refactor
	getChecks    []policyGetCheck
	sleep        time.Duration

	// if provided, skipFunction will be run and its result checked.  If not empty,
	// the test will be skipped with the return string as the input to t.Skip().
	// This allows to programatically skip some of the steps, based on environmental
	// information.
	skip skipFunction
}

// Runs the TestStep as an individual Go Test
func (s policyTestStep) run(t *testing.T, pub, prv *base.ClusterContext) {
	t.Run(
		s.name,
		func(t *testing.T) {
			if s.skip != nil {
				var skipResult = s.skip()
				if skipResult != "" {
					t.Skip(skipResult)
				}
			}
			s.applyPolicies(t, pub, prv)
			s.waitChecks(t, pub, prv)
			s.runCommands(t, pub, prv)
			s.runChecks(t, pub, prv)

			if s.sleep.Nanoseconds() > 0 {
				log.Printf("Sleeping for %v", s.sleep)
				time.Sleep(s.sleep)
			}
		})
}

// Apply all policies, on pub and prv
//
// See policyTestStep documentation for behavior
func (s policyTestStep) applyPolicies(t *testing.T, pub, prv *base.ClusterContext) {

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
								err = removePolicies(t, item.cluster, policyName)
								if err != nil {
									t.Fatalf("Failed to remove policy: %v", err)
								}
								continue
							case "KEEP":
								// We're just not doing anything with this one
								continue
							}
						}

						err = applyPolicy(t, policyName, policy, item.cluster)
						if err != nil {
							t.Fatalf("Failed to apply policy: %v", err)
						}
					}
				}

			})
		base.PostPolicyChangeSleep()
	}
}

// Runs a policy check using `get`.  It receives a *testing.T, so it does not return;
// it marks the test as failed if the check fails.
func getChecks(t *testing.T, getCheck policyGetCheck, c *client.PolicyAPIClient) {
	ok, err := getCheck.check(c)
	if err != nil {
		t.Errorf("GET check failed with error: %v", err)
		return
	}

	if !ok {
		t.Errorf("GET check failed (check: %v)", getCheck)
	}
}

// Runs the configured GET checks
func (s policyTestStep) runChecks(t *testing.T, pub, prv *base.ClusterContext) {

	// TODO: move these two into getChecks
	pubPolicyClient := client.NewPolicyValidatorAPI(pub.VanClient)
	prvPolicyClient := client.NewPolicyValidatorAPI(prv.VanClient)

	log.Printf("Running GET checks on %v", pub.Namespace)
	getChecks(t, s.pubGetCheck, pubPolicyClient)
	log.Printf("Running GET checks on %v", prv.Namespace)
	getChecks(t, s.prvGetCheck, prvPolicyClient)
}

func (s policyTestStep) waitChecks(t *testing.T, pub, prv *base.ClusterContext) {
	err := waitAllGetChecks(s.getChecks)
	if err != nil {
		t.Errorf("GET check wait failed: %v", err)
	}
}

// Run the commands part of the policyTestStep
func (s policyTestStep) runCommands(t *testing.T, pub, prv *base.ClusterContext) {
	if s.parallel {
		cli.RunScenariosParallel(t, s.cliScenarios)
	} else {
		cli.RunScenarios(t, s.cliScenarios)
	}
}

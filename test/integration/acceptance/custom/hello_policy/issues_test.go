//go:build policy
// +build policy

package hello_policy

import (
	"fmt"
	"log"
	"strings"
	"testing"
	"time"

	"github.com/skupperproject/skupper/client"
	"github.com/skupperproject/skupper/pkg/apis/skupper/v1alpha1"
	skupperv1 "github.com/skupperproject/skupper/pkg/apis/skupper/v1alpha1"
	"github.com/skupperproject/skupper/pkg/kube"
	"github.com/skupperproject/skupper/test/utils/base"
	"github.com/skupperproject/skupper/test/utils/skupper/cli"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Just as the name says.
//
// ctx.VanClient.KubeClient.CoreV1().Pods(ctx.Namespace).Delete()
//
// The pod name comes from kube.GetReadyPod for service-controller
func seekServiceControllerAndDelete(ctx *base.ClusterContext) {
	pod, err := kube.GetReadyPod(ctx.Namespace, ctx.VanClient.KubeClient, "service-controller")
	if err != nil {
		log.Printf("Ignoring pod listing error '%v'", err)
		return
	}

	ctx.VanClient.KubeClient.CoreV1().Pods(ctx.Namespace).Delete(
		pod.Name,
		&metav1.DeleteOptions{},
	)
}

// Test for https://github.com/skupperproject/skupper/issues/753
//
// This uses seekServiceControllerAndDelete to repeatedly remove the service
// controller and check what kind of error is received
func testLinkIssue753(t *testing.T, pub, prv *base.ClusterContext) {

	base.SkipIssueTests(t)

	done := make(chan int)
	var count int

	testTable := []policyTestCase{
		{
			name: "init",
			steps: []policyTestStep{
				{
					name:     "init",
					parallel: true,
					pubPolicy: []skupperv1.SkupperClusterPolicySpec{
						allowIncomingLinkPolicy(pub.Namespace, true),
					},
					cliScenarios: []cli.TestScenario{
						skupperInitInteriorTestScenario(pub, "", true),
						skupperInitEdgeTestScenario(prv, "", true),
					},
				},
			},
		}, {
			name: "main",
			steps: []policyTestStep{
				{
					// The main test needs to run on a preHook, since it is very specific
					// The runner is actually used only as a helper for policy setup, etc,
					// and to keep consistency on the reports
					name: "do-on-hook",
					preHook: func(_ map[string]string) error {
						// keep removing the pod; we want to force a situation where
						// we get a Unix response 137
						go func() {
						breakLabel:
							for {
								select {
								case <-done:
									break breakLabel
								default:
									seekServiceControllerAndDelete(pub)
									count++
								}
								time.Sleep(time.Second)
							}
							log.Print("Closing goroutine")
						}()

						// Keep running the GET check IncomingLink, seeing whether we get the error
						//
						// We cap the test number at 30 controller restarts.  This number is arbitrary,
						// but was more than enough to reproduce the issue during testing.
						p := client.NewPolicyValidatorAPI(pub.VanClient)
						for count < 30 {
							_, err := p.IncomingLink()

							if err != nil {
								log.Printf("Error: %v", err)
								// originally, we looked only for "command terminated with exit code".  However, the fix for
								// #753 on #782 just added a better explanation of what's going on ("skupper-service-controller not ready"),
								// while keeping the actual error with exit code for helping on investigations.  So, now we check that,
								// if a policy error ocurred with a command exit code, we don't report just that: we expect that
								// something else be given as an explanation to the error, like #782
								if strings.Contains(err.Error(), "Policy validation error: command terminated with exit code") {
									return fmt.Errorf("Error matched: '%w'", err)
								}
							}

						}

						close(done)
						return nil
					},
				},
			},
		}, {
			name: "cleanup",
			steps: []policyTestStep{
				{
					name:     "execute",
					parallel: true,
					cliScenarios: []cli.TestScenario{
						deleteSkupperTestScenario(pub, "pub"),
						deleteSkupperTestScenario(prv, "prv"),
					},
				},
			},
		},
	}

	policyTestRunner{
		testCases:    testTable,
		keepPolicies: true,
		prvPolicies: []skupperv1.SkupperClusterPolicySpec{
			allowedOutgoingLinksHostnamesPolicy(prv.Namespace, []string{"*"}),
		},
	}.run(t, pub, prv)

}

// Testing for #789
//
// https://github.com/skupperproject/skupper/issues/789
//
// It's basically the same previously-created-token test case from
// testLinkPolicy, but on a loop.
func testLinkIssue789(t *testing.T, pub, prv *base.ClusterContext) {

	base.SkipIssueTests(t)

	testTable := []policyTestCase{
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
				},
			},
		}, {
			name: "previously-created-token",
			steps: []policyTestStep{
				{
					name: "prepare",
					pubPolicy: []skupperv1.SkupperClusterPolicySpec{
						allowIncomingLinkPolicy(pub.Namespace, true),
					},
					getChecks: []policyGetCheck{
						{
							allowIncoming: cli.Boolp(true),
							cluster:       pub,
						}, {
							allowIncoming: cli.Boolp(false),
							allowedHosts:  []string{"any"},
							cluster:       prv,
						},
					},
					cliScenarios: []cli.TestScenario{
						createTokenPolicyScenario(pub, "", testPath, "previous", true),
					},
				},
			},
		},
	}

	for i := 0; i < 30; i++ {
		testTable = append(testTable, policyTestCase{
			name: "run",
			steps: []policyTestStep{
				{
					name: "disallow-and-create-link",
					pubPolicy: []skupperv1.SkupperClusterPolicySpec{
						allowIncomingLinkPolicy(pub.Namespace, false),
					},
					getChecks: []policyGetCheck{
						{
							cluster:       pub,
							allowIncoming: cli.Boolp(false),
						},
					},
					cliScenarios: []cli.TestScenario{
						createLinkTestScenario(prv, "", "previous", false),
						linkStatusTestScenario(prv, "", "previous", false),
					},
				}, {
					name: "re-allow-and-check-link",
					pubPolicy: []skupperv1.SkupperClusterPolicySpec{
						allowIncomingLinkPolicy(pub.Namespace, true),
					},
					getChecks: []policyGetCheck{
						{
							cluster:       pub,
							allowIncoming: cli.Boolp(true),
						},
					},
					cliScenarios: []cli.TestScenario{
						linkStatusTestScenario(prv, "now", "previous", true),
						sitesConnectedTestScenario(pub, prv, "", "previous"),
						linkDeleteTestScenario(prv, "", "previous"),
					},
				},
			},
		})
	}

	testTable = append(testTable, policyTestCase{
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
	})

	policyTestRunner{
		testCases:    testTable,
		keepPolicies: true,
		prvPolicies: []skupperv1.SkupperClusterPolicySpec{
			allowedOutgoingLinksHostnamesPolicy(prv.Namespace, []string{"*"}),
		},
	}.run(t, pub, prv)

}

// If a service is up and running, then we remove all policies and immediatelly reallow them: will
// the services be removed?
//
// The result may depend on cluster load and other factors, so we run it multiple times (runs+1)
//
// The test removes the policy in two different ways: one is actual removal, the other is reusing the
// policy, but removing only the target namespace from its list
//
// TODO:
// - test with changing the setting on AllowedServices, instead?
//
// This test expects the service to be removed.  See removeReallowKeep for the alternative.  Only
// one of them should be part of the test set, but which is unclear.
//
// This is testing for https://github.com/skupperproject/skupper/issues/727
func removeReallowRemove(pub, prv *base.ClusterContext, runs int) (allTestSteps []policyTestStep) {

	baseTestSteps := []policyTestStep{
		{
			name:     "add-policy--check-services-absent",
			parallel: true,
			pubPolicy: []v1alpha1.SkupperClusterPolicySpec{
				{
					Namespaces:      []string{pub.Namespace},
					AllowedServices: []string{"*"},
				}, {
					Namespaces:      []string{prv.Namespace},
					AllowedServices: []string{"*"},
				},
			},
			cliScenarios: []cli.TestScenario{
				serviceCheckAbsentTestScenario(pub, "frontend", []string{"hello-world-frontend", "hello-world-backend"}),
				serviceCheckAbsentTestScenario(prv, "backend", []string{"hello-world-frontend", "hello-world-backend"}),
			},
		}, {
			name:     "create-service",
			parallel: true,
			cliScenarios: []cli.TestScenario{
				serviceCreateFrontTestScenario(pub, "", true),
				serviceCreateBackTestScenario(prv, "", true),
			},
		}, {
			name:     "check-services-created",
			parallel: true,
			cliScenarios: []cli.TestScenario{
				serviceCheckFrontStatusTestScenario(pub, "", []string{"hello-world-frontend", "hello-world-backend"}, []string{}, []string{}),
				serviceCheckBackStatusTestScenario(prv, "", []string{"hello-world-frontend", "hello-world-backend"}, []string{}, []string{}),
			},
		}, {
			name: "remove-policy",
			pubPolicy: []v1alpha1.SkupperClusterPolicySpec{
				{
					Namespaces:      []string{"non-existing-namespace"},
					AllowedServices: []string{"*"},
				}, {
					Namespaces: []string{"REMOVE"},
				},
			},
			// We do not run anything on this policy, as we want to the next step (recreating the
			// policy) to be executed right away
		},
	}

	// The first execution is just preparation; we need to run at the very least two full cycles
	for i := 0; i < runs+1; i++ {
		allTestSteps = append(allTestSteps, baseTestSteps...)
	}

	return
}

// If a service is up and running, then we remove all policies and immediatelly reallow them: will
// the services be removed?
//
// The result may depend on cluster load and other factors, so we run it multiple times (runs+1)
//
// The test removes the policy in two different ways: one is actual removal, the other is reusing the
// policy, but removing only the target namespace from its list
//
// TODO:
// - test with changing the setting on AllowedServices, instead
// - test with switching the policy: remove existing policy but add another that allows
//
// This test expects the service to be kept.  See removeReallowRemove for the alternative.  Only
// one of them should be part of the test set, but which is unclear.
//
// This is testing for https://github.com/skupperproject/skupper/issues/727
func removeReallowKeep(pub, prv *base.ClusterContext, runs int) (allTestSteps []policyTestStep) {

	allTestSteps = append(allTestSteps, policyTestStep{
		name:     "initial-service-creation",
		parallel: true,
		pubPolicy: []v1alpha1.SkupperClusterPolicySpec{
			{
				Namespaces:      []string{pub.Namespace},
				AllowedServices: []string{"*"},
			}, {
				Namespaces:      []string{prv.Namespace},
				AllowedServices: []string{"*"},
			},
		},
		cliScenarios: []cli.TestScenario{
			serviceCreateFrontTestScenario(pub, "", true),
			serviceCreateBackTestScenario(prv, "", true),
		},
	})

	baseTestSteps := []policyTestStep{
		{
			name:     "add-policy--check-services-present",
			parallel: true,
			pubPolicy: []v1alpha1.SkupperClusterPolicySpec{
				{
					Namespaces:      []string{pub.Namespace},
					AllowedServices: []string{"*"},
				}, {
					Namespaces:      []string{prv.Namespace},
					AllowedServices: []string{"*"},
				},
			},
			cliScenarios: []cli.TestScenario{
				serviceCheckFrontStatusTestScenario(pub, "", []string{"hello-world-frontend", "hello-world-backend"}, []string{}, []string{}),
				serviceCheckBackStatusTestScenario(prv, "", []string{"hello-world-frontend", "hello-world-backend"}, []string{}, []string{}),
			},
		}, {
			name: "remove-policy",
			pubPolicy: []v1alpha1.SkupperClusterPolicySpec{
				{
					Namespaces:      []string{"non-existing-namespace"},
					AllowedServices: []string{"*"},
				}, {
					Namespaces: []string{"REMOVE"},
				},
			},
			// We do not run anything on this policy, as we want to the next step (recreating the
			// policy) to be executed right away
		},
	}

	// The first execution is just preparation; we need to run at the very least two full cycles
	for i := 0; i < runs+1; i++ {
		s := baseTestSteps
		s[1].sleep = time.Duration(5000/runs*i) * time.Millisecond
		allTestSteps = append(allTestSteps, s...)
	}

	return
}

// Can a service be created immediatelly after a policy allowing it?  If not,
// do we get a proper error message for that?
//
// This is testing for https://github.com/skupperproject/skupper/issues/728
//
// This test is valid for manual runs, where the results will be inspected for
// which type of errors it produces (see the issue logs for details), but it's
// not good for CI testing, given its inconstant nature: service creation may
// work or fail (both are proper outcomes), but the ensuing test needs to match
// the previous step.
//
// For that reason, the test is hard code skipped; anyone doing that manual
// test must manually change that code.
func allowAndCreate(pub, prv *base.ClusterContext, runs int) (allTestSteps []policyTestStep) {

	allTestSteps = append(allTestSteps, policyTestStep{
		// These policies are created just so the first cycle of the
		// baseTestSteps has something to remove
		name: "initial-dummy-policies",
		pubPolicy: []v1alpha1.SkupperClusterPolicySpec{
			{
				Namespaces:      []string{pub.Namespace},
				AllowedServices: []string{"*"},
			}, {
				Namespaces:      []string{prv.Namespace},
				AllowedServices: []string{"*"},
			},
		},
	})

	baseTestSteps := []policyTestStep{
		{
			name:     "remove-policy--check-services-absent",
			parallel: true,
			pubPolicy: []v1alpha1.SkupperClusterPolicySpec{
				{
					Namespaces:      []string{"non-existing"},
					AllowedServices: []string{"*"},
				}, {
					Namespaces: []string{"REMOVE"},
				},
			},
			cliScenarios: []cli.TestScenario{
				serviceCheckAbsentTestScenario(pub, "frontend", []string{"hello-world-frontend", "hello-world-backend"}),
				serviceCheckAbsentTestScenario(prv, "backend", []string{"hello-world-frontend", "hello-world-backend"}),
			},
		}, {
			name: "add-policy--create-service",
			pubPolicy: []v1alpha1.SkupperClusterPolicySpec{
				{
					Namespaces:      []string{pub.Namespace},
					AllowedServices: []string{"*"},
				}, {
					Namespaces:      []string{prv.Namespace},
					AllowedServices: []string{"*"},
				},
			},
			// We try to add services right after giving policy permission.  Should that work, should that fail?
			// If it fails, it should give a failure response.
			parallel: true,
			cliScenarios: []cli.TestScenario{
				serviceCreateFrontTestScenario(pub, "", true),
				serviceCreateBackTestScenario(prv, "", true),
			},
		}, {
			// This is the crux of this testing.  If we got to this point, the service should have been created
			name:     "check-services-created",
			parallel: true,
			cliScenarios: []cli.TestScenario{
				serviceCheckFrontStatusTestScenario(pub, "", []string{"hello-world-frontend", "hello-world-backend"}, []string{}, []string{}),
				serviceCheckBackStatusTestScenario(prv, "", []string{"hello-world-frontend", "hello-world-backend"}, []string{}, []string{}),
			},
		},
	}

	// The first execution is just preparation; we need to run at the very least two full cycles
	for i := 0; i < runs+1; i++ {
		s := baseTestSteps
		// s[1].sleep = time.Duration(5000/runs*i) * time.Millisecond
		allTestSteps = append(allTestSteps, s...)
	}

	return

}

// As the name says: if the last policy that allows a service to exist is removed, the
// service needs to be removed by the system
//
// No issue was opened for this test, but it is basically the same as
// https://github.com/skupperproject/skupper/issues/718
// with the exception that it is services, not links, and the expectation is
// for removal, not simply disabling.
func removePolicyRemoveServices(pub, prv *base.ClusterContext, runs int) (allTestSteps []policyTestStep) {

	baseTestSteps := []policyTestStep{
		{
			// We're just adding the policy, so there should be no services
			// around.  Neither at start of test, nor at start of cycle
			// (that's what we're testing, that policy removal got rid of
			// running services)
			name: "add-policy--check-services-absent",
			pubPolicy: []v1alpha1.SkupperClusterPolicySpec{
				{
					Namespaces:      []string{pub.Namespace},
					AllowedServices: []string{"*"},
				}, {
					Namespaces:      []string{prv.Namespace},
					AllowedServices: []string{"*"},
				},
			},
			// Avoid https://github.com/skupperproject/skupper/issues/728
			getChecks: []policyGetCheck{
				{
					cluster:         pub,
					allowedServices: []string{"any"},
				}, {
					cluster:         pub,
					allowedServices: []string{"any"},
				},
			},
		}, {
			name:     "create-services",
			parallel: true,
			cliScenarios: []cli.TestScenario{
				serviceCreateFrontTestScenario(pub, "", true),
				serviceCreateBackTestScenario(prv, "", true),
			},
		}, {
			name:     "check-services",
			parallel: true,
			cliScenarios: []cli.TestScenario{
				serviceCheckFrontStatusTestScenario(pub, "", []string{"hello-world-frontend", "hello-world-backend"}, []string{}, []string{}),
				serviceCheckBackStatusTestScenario(prv, "", []string{"hello-world-frontend", "hello-world-backend"}, []string{}, []string{}),
			},
		}, {
			// This is the crux of this testing.  If we remove policies, the services
			// must be removed
			name:     "remove-policy--check-services-removed",
			parallel: true,
			pubPolicy: []v1alpha1.SkupperClusterPolicySpec{
				{
					Namespaces:      []string{"non-existing"},
					AllowedServices: []string{"*"},
				}, {
					Namespaces: []string{"REMOVE"},
				},
			},
			cliScenarios: []cli.TestScenario{
				serviceCheckAbsentTestScenario(pub, "frontend", []string{"hello-world-frontend", "hello-world-backend"}),
				serviceCheckAbsentTestScenario(prv, "backend", []string{"hello-world-frontend", "hello-world-backend"}),
			},
		},
	}

	// The first execution is just preparation; we need to run at the very least two full cycles
	for i := 0; i < runs+1; i++ {
		s := baseTestSteps
		// s[1].sleep = time.Duration(5000/runs*i) * time.Millisecond
		allTestSteps = append(allTestSteps, s...)
	}

	return

}

// This test groups several issues that were opened for the services part of
// policies, using a single skupper initialization/finalization.  All of them
// run a core of repeating steps hundreds of times to try and reproduce an
// issue that happens intermitently, so they may take a while to run
//
// TODO: Refactor to allow for individual tests to be run specifically
func testServicePolicyIssues(t *testing.T, pub, prv *base.ClusterContext) {

	base.SkipIssueTests(t)

	testTable := []policyTestCase{
		{
			name: "initialize",
			steps: []policyTestStep{
				{
					name:     "skupper-init",
					parallel: true,
					cliScenarios: []cli.TestScenario{
						skupperInitInteriorTestScenario(pub, "", true),
						skupperInitEdgeTestScenario(prv, "", true),
					},
				}, {
					name: "connect",
					cliScenarios: []cli.TestScenario{
						connectSitesTestScenario(pub, prv, "", "service"),
					},
				},
			},
		}, {
			// This is testing for https://github.com/skupperproject/skupper/issues/728
			name:  "allow-policy--and--immediatelly-create-service",
			steps: allowAndCreate(pub, prv, 100),
			skip:  func() string { return "Manual test.  Hardcoded skip" },
		}, {
			// This is testing for https://github.com/skupperproject/skupper/issues/727
			name:  "remove-policy-reallow--check-service-kept",
			steps: removeReallowKeep(pub, prv, 500),
			skip:  func() string { return "#727 needs resolved before this test becomes valid.  Skipping" },
		}, {
			// This is testing for https://github.com/skupperproject/skupper/issues/727
			name:  "remove-policy-reallow--check-service-removed",
			steps: removeReallowRemove(pub, prv, 500),
			skip:  func() string { return "#727 needs resolved before this test becomes valid.  Skipping" },
		}, {
			name:  "remove-policy--remove-service",
			steps: removePolicyRemoveServices(pub, prv, 100),
		}, {
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

	policyTestRunner{
		testCases: testTable,
		pubPolicies: []v1alpha1.SkupperClusterPolicySpec{
			{
				Namespaces:                    []string{"*"},
				AllowIncomingLinks:            true,
				AllowedOutgoingLinksHostnames: []string{"*"},
			},
		},
	}.run(t, pub, prv)

}

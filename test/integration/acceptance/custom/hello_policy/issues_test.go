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
	skupperv1 "github.com/skupperproject/skupper/pkg/apis/skupper/v1alpha1"
	"github.com/skupperproject/skupper/pkg/kube"
	"github.com/skupperproject/skupper/test/utils/base"
	"github.com/skupperproject/skupper/test/utils/skupper/cli"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Test for https://github.com/skupperproject/skupper/issues/753
func test753(t *testing.T, pub, prv *base.ClusterContext) {

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

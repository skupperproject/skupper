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
					name: "do-on-hook",
					preHook: func(_ map[string]string) error {
						// keep removing the pod
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

						p := client.NewPolicyValidatorAPI(pub.VanClient)
						for count < 30 {
							_, err := p.IncomingLink()

							if err != nil {
								log.Printf("Error: %v", err)
								if strings.Contains(err.Error(), "command terminated with exit code") {
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

func seekServiceControllerAndDelete(ctx *base.ClusterContext) {
	/*
		pods, err := ctx.VanClient.KubeClient.CoreV1().Pods(ctx.Namespace).List(
			metav1.ListOptions{LabelSelector: "skupper.io/component=service-controller"},
		)
		if err != nil {
			log.Printf("Got pod listing error: %v", err)
			return
		}
		if len(pods.Items) == 0 {
			log.Printf("No pods found")
			return
		}
		if len(pods.Items) > 1 {
			log.Printf("Multiple pounds found; picking the one that shows as ready")
		}
	*/

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

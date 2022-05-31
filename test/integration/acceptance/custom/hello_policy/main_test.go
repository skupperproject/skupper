//go:build policy
// +build policy

package hello_policy

import (
	"log"
	"os"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/skupperproject/skupper/pkg/kube"
	"github.com/skupperproject/skupper/test/utils/base"
	"github.com/skupperproject/skupper/test/utils/constants"
	"github.com/skupperproject/skupper/test/utils/k8s"
	"github.com/skupperproject/skupper/test/utils/skupper/cli"
	"gotest.tools/assert"
	corev1 "k8s.io/api/core/v1"
)

var testPath = "./tmp/"

// Helper to deploy an application on Kubernetes and wait for it
type KubeDeploy struct {
	image  string
	labels map[string]string
	name   string
}

// Actual deployment, according to struct's specification, on the
// given context.  If a deployment with the same name already exists,
// it will just log and keep going.
func (kd KubeDeploy) deploy(ctx *base.ClusterContext) (err error) {
	deployment, _ := k8s.NewDeployment(kd.name, ctx.Namespace, k8s.DeploymentOpts{
		Image:         kd.image,
		Labels:        kd.labels,
		RestartPolicy: corev1.RestartPolicyAlways,
	})
	// Creating deployments
	if _, err = ctx.VanClient.KubeClient.AppsV1().Deployments(ctx.Namespace).Create(deployment); err != nil {
		if strings.Contains(err.Error(), `deployments.apps "`+kd.name+`" already exists`) {
			log.Printf("Ignoring application already deployed: %v", err)
			err = nil
		}
		return
	}
	// Waiting for deployments to be ready
	if _, err = kube.WaitDeploymentReady(kd.name, ctx.Namespace, ctx.VanClient.KubeClient, constants.ImagePullingAndResourceCreationTimeout, constants.DefaultTick); err != nil {
		return
	}
	return
}

// Deploys the front-end side of quay.io/skupper/hello-world-*
func deployFrontend(ctx *base.ClusterContext) (err error) {
	err = KubeDeploy{
		image:  "quay.io/skupper/hello-world-frontend",
		labels: map[string]string{"app": "hello-world-frontend"},
		name:   "hello-world-frontend",
	}.deploy(ctx)
	return
}

//
// Deploys the backend-end side of quay.io/skupper/hello-world-*
func deployBackend(ctx *base.ClusterContext) (err error) {
	err = KubeDeploy{
		image:  "quay.io/skupper/hello-world-backend",
		labels: map[string]string{"app": "hello-world-backend"},
		name:   "hello-world-backend",
	}.deploy(ctx)
	return
}

// deployResources Deploys the hello-world-frontend and hello-world-backend
// pods and validate they are available
func deployResources(pub *base.ClusterContext, prv *base.ClusterContext) error {
	err := deployFrontend(pub)
	if err != nil {
		return err
	}
	err = deployBackend(prv)
	if err != nil {
		return err
	}

	return nil
}

// If the prefix is empty, return name, unchanged
//
// Else, return "prefix-name"
func prefixName(prefix, name string) (newName string) {

	newName = name

	if prefix == "" {
		return
	}

	newName = prefix + "-" + name

	return
}

// TestMain initializes flag parsing
func TestMain(m *testing.M) {
	base.ParseFlags()
	os.Exit(m.Run())
}

// This will return up to four namespaces/contexts
//
// pub1 and pub2 represent the frontend and backend to be tied together by
// skupper, on the same cluster.
//
// pub3 and prv1 are the same, but they only exist for multi-cluster testing,
// where each is on a different cluster
//
// TODO: change to pub1, pub2, prv1; implement prv1 with Multi
func setup(t *testing.T) (pub1, pub2, pub3, prv1 *base.ClusterContext) {

	t.Run("Setup", func(t *testing.T) {
		// First, validate if skupper binary is in the PATH, or fail the test
		log.Printf("Running 'skupper version' to determine whether the skupper binary is available and record its version")
		_, _, err := cli.RunSkupperCli([]string{"version"})
		if err != nil {
			t.Fatalf("skupper binary is not available")
		}

		log.Printf("Creating namespaces")
		needs := base.ClusterNeeds{
			// TODO: Change this to just 'policy', as it will be reused
			NamespaceId:    "policy-namespaces",
			PublicClusters: 2,
		}
		runner := &base.ClusterTestRunnerBase{}
		if err := runner.Validate(needs); err != nil {
			t.Skipf("%s", err)
		}
		_, err = runner.Build(needs, nil)
		assert.Assert(t, err)

		// This is the target domain
		pub1, err = runner.GetPublicContext(1)
		assert.Assert(t, err)
		// This is the 'other' domain
		pub2, err = runner.GetPublicContext(2)
		assert.Assert(t, err)

		// TODO.  From here down, put it on a loop, as there may be four

		// creating namespaces
		assert.Assert(t, pub1.CreateNamespace())
		assert.Assert(t, pub2.CreateNamespace())

		// labelling the namespaces
		pub1.LabelNamespace("test.skupper.io/test-namespace", "policy")
		pub2.LabelNamespace("test.skupper.io/test-namespace", "policy")
	})

	return
}

// Returns the name of the function, without module or package information.
func getFuncName(function interface{}) string {
	name := runtime.FuncForPC(reflect.ValueOf(function).Pointer()).Name()
	split := strings.Split(name, ".")
	return split[len(split)-1]
}

// This should be the only test on the package; it sets the environment up and
// calls the actual tests.  Some items are listed as tests (t.Run), but are not
// really tests; instead they're setup and teardown functions.  They're set
// this way so that their times are reported on the end, and the full length of
// the test can be better understood.
//
// To run a single test, refer to it as '//testName'.  For example,
//
//   go test -tags policy -timeout 60 -run "//testNamespace"
//
// This will ensure that not only the selected tests, but also the setup and
// tear down are run.
//
// TBD control setup/teardown with environment variables.
//
// Because the policy changes are cluster-wise, we need to run all tests in
// serial.
//
// Individual tests should expect the environment they receive to have the CRD
// installed and no policies at test start.  Tests are responsible for running
// skupper init and skupper delete (?)
func TestPolicies(t *testing.T) {

	// Creating a local directory for storing the token
	_ = os.Mkdir(testPath, 0755)

	pub1, pub2, _, _ := setup(t)
	//	pub1, pub2, pub3, prv1 := setup(t)

	allContexts := []*base.ClusterContext{pub1, pub2}

	// teardown once test completes
	tearDownFn := func() {
		t.Run("teardown", func(t *testing.T) {
			var wg sync.WaitGroup
			log.Print("entering teardown")
			if base.ShouldSkipPolicyTeardown() {
				log.Print("Skipping policy tear down, per env variables")
			} else {
				for i, context := range allContexts {
					if context == nil {
						log.Printf("Context #%v is nil; skipping", i)
						break
					}
					wg.Add(1)
					context := context
					go func() {
						defer wg.Done()
						log.Printf("Removing Policy CRD from context %v", context.Namespace)
						_, err := removeCrd(context)
						if err != nil {
							log.Printf("Failed removing CRD: %v", err)
						}
						removeClusterRole(context)
					}()
				}
			}
			if base.ShouldSkipNamespaceTeardown() {
				log.Print("Skipping namespace tear down, per env variables")
				log.Print("Removing skupper from namespaces, instead")
				if pub1 == nil || pub2 == nil {
					log.Print("At least one of the namespaces was not initialized, which was not expected.  Skipping this step")
				} else {
					cli.RunScenariosParallel(t, []cli.TestScenario{
						{
							Name: "skupper-delete-pub1",
							Tasks: []cli.SkupperTask{
								{
									Ctx: pub1,
									Commands: []cli.SkupperCommandTester{
										&cli.DeleteTester{
											IgnoreNotInstalled: true,
										},
									},
								},
							},
						}, {
							Name: "skupper-delete-pub2",
							Tasks: []cli.SkupperTask{
								{
									Ctx: pub2,
									Commands: []cli.SkupperCommandTester{
										&cli.DeleteTester{
											IgnoreNotInstalled: true,
										},
									},
								},
							},
						},
					})
				}

			} else {
				for i, context := range allContexts {
					if context == nil {
						log.Printf("Context #%v is nil; skipping", i)
						break
					}
					wg.Add(1)
					context := context
					go func() {
						defer wg.Done()
						log.Printf("Removing namespace %v", context.Namespace)
						err := context.DeleteNamespace()
						if err != nil {
							log.Printf("Removal of namespace %v failed: %v", context.Namespace, err)
						}
					}()
				}
			}
			wg.Wait()
			log.Print("tearDown completed")
		})
	}
	defer tearDownFn()
	base.HandleInterruptSignal(func() {
		tearDownFn()
	})

	if t.Failed() {
		t.Fatalf("Setup failed")
	}

	t.Run("application-deployment", func(t *testing.T) {
		// deploying frontend and backend services
		assert.Assert(t, deployResources(pub1, pub2))
	})

	if t.Failed() {
		t.Fatalf("Application deployment failed")
	}

	type policyTestFunction func(*testing.T, *base.ClusterContext, *base.ClusterContext)
	type policyTestItem struct {
		function policyTestFunction
		single   bool
		multiple bool
		skipCrd  bool
	}

	testTable := []policyTestItem{
		{
			function: testHelloPolicy,
			skipCrd:  true,
		}, {
			function: testNamespace,
		}, {
			function: testLinkPolicy,
		}, {
			function: testServicePolicy,
		}, {
			function: testServicePolicyTransitions,
		}, {
			function: testHostnamesPolicy,
		}, {
			function: testResourcesPolicy,
		},
	}

	// This allows to select a specific testing with
	// "go test -run //testname"
	t.Run("tests", func(t *testing.T) {
		for _, item := range testTable {
			item := item
			name := getFuncName(item.function)
			t.Run(name, func(t *testing.T) {
				var err error
				for _, ctx := range []*base.ClusterContext{pub1, pub2} {
					if !item.skipCrd {
						err = applyCrd(ctx)
						if err != nil {
							t.Fatalf("failed to apply CRD on %v: %v", ctx, err)
						}
					}
					err = removePolicies(ctx)
					if err != nil {
						t.Fatalf("failed to remove policies on %v: %v", ctx, err)
					}
				}
				item.function(t, pub1, pub2)
			})
			base.StopIfInterrupted(t)
		}
	})

}

//go:build policy
// +build policy

package hello_policy

import (
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"
	"sync"
	"testing"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/client"
	skupperv1 "github.com/skupperproject/skupper/pkg/apis/skupper/v1alpha1"
	clientv1 "github.com/skupperproject/skupper/pkg/generated/client/clientset/versioned/typed/skupper/v1alpha1"
	"github.com/skupperproject/skupper/pkg/kube"
	"github.com/skupperproject/skupper/test/utils/base"
	"github.com/skupperproject/skupper/test/utils/constants"
	"github.com/skupperproject/skupper/test/utils/k8s"
	"github.com/skupperproject/skupper/test/utils/skupper/cli"
	"gotest.tools/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type policyGetCheck struct {
	allowIncoming    *bool
	checkUndefinedAs *bool
}

type checkItem func() (result *client.PolicyAPIResult, err error)

// This is a helper to check for either a defined policy item or a
// default, allowing for nil values representing a no check.
// The arguments cannot be both nil.
func checkValue(policyItem *bool, checkUndefinedAs *bool, checkFunc checkItem) (result bool, err error) {

	var effectiveValue *client.PolicyAPIResult
	var expectedValue bool

	switch {

	case policyItem != nil:
		expectedValue = *policyItem

	case checkUndefinedAs != nil:
		expectedValue = *checkUndefinedAs

	default:
		err = fmt.Errorf("checkValue() should not be called with two nils")
		return
	}

	effectiveValue, err = checkFunc()
	if err != nil {
		return
	}

	result = effectiveValue.Allowed == expectedValue

	return
}

func (p policyGetCheck) check(c *client.PolicyAPIClient) (ok bool, err error) {
	ok = true

	if p.allowIncoming != nil || p.checkUndefinedAs != nil {

		var item bool

		item, err = checkValue(
			p.allowIncoming,
			p.checkUndefinedAs,
			func() (*client.PolicyAPIResult, error) {
				return c.IncomingLink()
			})
		if err != nil {
			return
		}

		ok = ok && item
	}
	return
}

// Adds the CRD to the cluster
func applyCrd(t *testing.T, cluster *base.ClusterContext) (err error) {
	var out []byte
	log.Printf("Adding CRD into the %v cluster", cluster.KubeConfig)
	out, err = cluster.KubectlExec("apply -f ../../../../../api/types/crds/skupper_cluster_policy_crd.yaml")
	if err != nil {
		log.Printf("CRD applying failed: %v", err)
		log.Print("Output:\n", out)
		return
	}
	return
}

func isCrdInstalled(cluster *base.ClusterContext) (installed bool, err error) {
	var out []byte
	installed = true

	// TODO: replace this by some kube API
	out, err = cluster.KubectlExec("get crd skupperclusterpolicies.skupper.io")
	if err != nil {
		if strings.Contains(
			string(out),
			`Error from server (NotFound): customresourcedefinitions.apiextensions.k8s.io "`) {
			installed = false
			err = nil
		}
	}
	return
}

// Remove the CRD from the cluster
func removeCrd(t *testing.T, cluster *base.ClusterContext) (changed bool, err error) {
	changed = true

	log.Printf("Removing CRD from the cluster %v", cluster.KubeConfig)

	installed, err := isCrdInstalled(cluster)
	if err != nil {
		t.Fatalf("Failed checking for CRD")
		return
	}

	if !installed {
		changed = false
		log.Print("CRD was not present, so not changing anything")
		return
	}

	if _, err := cluster.KubectlExec("delete crd skupperclusterpolicies.skupper.io"); err != nil {
		t.Fatalf("Removal of CRD failed: %v", err)
	}
	return
}

// Remove the cluster role, but do not fail if it is not there
func removeClusterRole(t *testing.T, cluster *base.ClusterContext) (changed bool, err error) {
	changed = true
	log.Printf("Removing cluster role %v from the CRD definition", types.ControllerServiceAccountName)

	// Is it there?
	role, err := cluster.VanClient.KubeClient.RbacV1().ClusterRoles().Get(types.ControllerServiceAccountName, metav1.GetOptions{})
	if role == nil && err != nil {
		log.Print("The role did not exist on the cluster; skipping removal")
		changed = false
		err = nil
		return
	}
	cluster.VanClient.KubeClient.RbacV1().ClusterRoles().Delete(types.ControllerServiceAccountName, nil)
	return
}

// Removes all policies from the cluster.
//
// In the future, change the signature so the last item is ..policies, so specific
// policies can be given
func removePolicies(t *testing.T, cluster *base.ClusterContext, policies ...string) (err error) {

	log.Print("Removing policies")

	var list *skupperv1.SkupperClusterPolicyList

	skupperCli, err := clientv1.NewForConfig(cluster.VanClient.RestConfig)
	if err != nil {
		return
	}

	if len(policies) == 0 {
		policies = []string{}
		// We're listing and removing everything
		list, err = listPolicies(t, cluster)
		if err != nil {
			return
		}
		for _, item := range list.Items {
			policies = append(policies, item.Name)
		}
	}

	for _, item := range policies {
		log.Printf("- %v", item)
		item_err := skupperCli.SkupperClusterPolicies().Delete(item, &metav1.DeleteOptions{})
		if item_err != nil {
			log.Printf("  removal failed: %v", item_err)
			if err == nil {
				// We'll return the first error from the list, but keep trying the others
				err = item_err
			}
		}
	}

	return
}

func listPolicies(t *testing.T, cluster *base.ClusterContext) (list *skupperv1.SkupperClusterPolicyList, err error) {

	installed, err := isCrdInstalled(cluster)
	if err != nil {
		t.Fatalf("Failed to check for CRD on the cluster")
		return
	}

	if !installed {
		log.Print("The CRD is not installed, so considering the policy list as empty")
		list = &skupperv1.SkupperClusterPolicyList{}
		return
	}

	skupperCli, err := clientv1.NewForConfig(cluster.VanClient.RestConfig)
	if err != nil {
		return
	}

	list, err = skupperCli.SkupperClusterPolicies().List(metav1.ListOptions{})
	if err != nil {
		log.Print("Failed listing policies")
		return
	}

	return
}

func keepPolicies(t *testing.T, cluster *base.ClusterContext, patterns []regexp.Regexp) (err error) {

	policyList := []string{}

	list, err := listPolicies(t, cluster)
	if err != nil {
		return
	}

	for _, item := range list.Items {
		var matched bool
		for _, re := range patterns {
			if re.MatchString(item.Name) {
				matched = true
				break
			}
		}
		if !matched {
			policyList = append(policyList, item.Name)
		}
	}

	if len(policyList) == 0 {
		return
	}

	err = removePolicies(t, cluster, policyList...)

	return

}

// Apply a SkupperClusterPolicySpec with the given name on the
// requested cluster
func applyPolicy(t *testing.T, name string, spec skupperv1.SkupperClusterPolicySpec, cluster *base.ClusterContext) (err error) {

	log.Printf("Applying policy %v (%v)...", name, spec)
	skupperCli, err := clientv1.NewForConfig(cluster.VanClient.RestConfig)
	if err != nil {
		return
	}
	var policy = skupperv1.SkupperClusterPolicy{
		TypeMeta: metav1.TypeMeta{
			Kind:       "SkupperClusterPolicy",
			APIVersion: "skupper.io/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: spec,
	}

	existing, err := skupperCli.SkupperClusterPolicies().Get(name, metav1.GetOptions{})
	if err != nil {
		log.Printf("... as a new policy")
		_, err = skupperCli.SkupperClusterPolicies().Create(&policy)
		if err != nil {
			return err
		}
	} else {
		log.Printf("... as an update to an existing policy")
		policy.ObjectMeta.ResourceVersion = existing.ObjectMeta.ResourceVersion
		_, err := skupperCli.SkupperClusterPolicies().Update(&policy)
		if err != nil {
			return err
		}
	}

	return
}

// Helper to deploy an application on Kubernetes and wait for it
type KubeDeploy struct {
	image  string
	labels map[string]string
	name   string
}

// Actual deployment, according to struct's specification, on the
// given context
func (kd KubeDeploy) deploy(ctx *base.ClusterContext) (err error) {
	frontend, _ := k8s.NewDeployment(kd.name, ctx.Namespace, k8s.DeploymentOpts{
		Image:         kd.image,
		Labels:        kd.labels,
		RestartPolicy: corev1.RestartPolicyAlways,
	})
	// Creating deployments
	if _, err = ctx.VanClient.KubeClient.AppsV1().Deployments(ctx.Namespace).Create(frontend); err != nil {
		return
	}
	// Waiting for deployments to be ready
	if _, err := kube.WaitDeploymentReady(kd.name, ctx.Namespace, ctx.VanClient.KubeClient, constants.ImagePullingAndResourceCreationTimeout, constants.DefaultTick); err != nil {
		return err
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
	deployFrontend(pub)
	deployBackend(prv)

	return nil
}

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

// This should be the only test on the package; it sets the environment up and
// calls the actual tests.  Some items are listed as tests (t.Run), but are not
// really tests; instead they're setup and teardown functions.  They're set
// this way so that their times are reported on the end, and the full length of
// the test can be better understood.
//
// To run a single test, refer to it as '//testName'.  For example,
//   go test -run "//testNamespace"
// This will ensure that not only the tests, but also the setup and tear down
// are run.
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
						removeCrd(t, context)
						removeClusterRole(t, context)
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
							Name: "skupper-delete",
							Tasks: []cli.SkupperTask{
								{
									Ctx: pub1,
									Commands: []cli.SkupperCommandTester{
										&cli.DeleteTester{},
									},
								}, {
									Ctx: pub2,
									Commands: []cli.SkupperCommandTester{
										&cli.DeleteTester{},
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

	// This allows to select a specific testing with
	// "go test -run //testname"
	t.Run("tests", func(t *testing.T) {
		// Change all below for a loop with function references
		// and some introspection?
		t.Run("testHelloPolicy", func(t *testing.T) {
			testHelloPolicy(t, pub1, pub2)
		})

		base.StopIfInterrupted(t)

		t.Run("testNamespace", func(t *testing.T) {
			applyCrd(t, pub1)
			removePolicies(t, pub1)
			testNamespace(t, pub1, pub2)
		})

		base.StopIfInterrupted(t)

		t.Run("testLinkPolicy", func(t *testing.T) {
			applyCrd(t, pub1)
			removePolicies(t, pub1)
			removePolicies(t, pub2)
			testLinkPolicy(t, pub1, pub2)
		})

		base.StopIfInterrupted(t)

		t.Run("testServicePolicy", func(t *testing.T) {
			applyCrd(t, pub1)
			removePolicies(t, pub1)
			removePolicies(t, pub2)
			testServicePolicy(t, pub1, pub2)
		})

		base.StopIfInterrupted(t)

		t.Run("testServicePolicyTransitions", func(t *testing.T) {
			applyCrd(t, pub1)
			removePolicies(t, pub1)
			removePolicies(t, pub2)
			testServicePolicyTransitions(t, pub1, pub2)
		})

		base.StopIfInterrupted(t)
	})

}

// This will return up to four namespaces/contexts
//
// pub1 and pub2 represent the frontend and backend to be tied together by
// skupper, on the same cluster.
//
// pub3 and prv1 are the same, but they only exist for multi-cluster testing,
// where each is on a different cluster
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

package hello_policy

import (
	"fmt"
	"log"
	"os"
	"testing"

	skupperv1 "github.com/skupperproject/skupper/pkg/apis/skupper/v1alpha1"
	"github.com/skupperproject/skupper/test/utils/base"
	"github.com/skupperproject/skupper/test/utils/skupper/cli"
	"github.com/skupperproject/skupper/test/utils/skupper/cli/token"
	"gotest.tools/assert"
)

type namespaceTest struct {
	// This will go straight to the policy's Namespaces field, so it will
	// be a definition of which namespaces are being affected by policy
	// change
	namespaces []string

	// Whether the policy change should affect the target namespace (pub1)
	// Notice the target is not what is on the field above; it's always
	// the same, static, namespace
	worksOnTarget bool

	// Whether the change should affect other namespaces (more specifically
	// pub2)
	worksElsewhere bool
}

func createToken(name string, cluster *base.ClusterContext, testPath string, works bool) (createToken cli.TestScenario) {

	createToken = cli.TestScenario{
		Name: "create-token",
		Tasks: []cli.SkupperTask{
			{Ctx: cluster, Commands: []cli.SkupperCommandTester{
				// skupper token create - verify token has been created
				&token.CreateTester{
					Name:             name,
					FileName:         testPath + name + ".token.yaml",
					ExpectDisallowed: !works,
				},
			}},
		},
	}
	return
}

func testNamespace(t *testing.T) {

	var pub1, pub2 *base.ClusterContext
	var err error

	t.Run("Setup", func(t *testing.T) {
		// vvvvvvvvvvvv  Move this preamble to some shared file? vvvvvvvvvvvv
		//
		// First, validate if skupper binary is in the PATH, or fail the test
		log.Printf("Running 'skupper --help' to determine if skupper binary is available")
		_, _, err := cli.RunSkupperCli([]string{"--help"})
		if err != nil {
			t.Fatalf("skupper binary is not available")
		}

		// For this test, I'm not checking effects on communicating clusters,
		// so there is no multiCluster testing, and two namespaces on pub are
		// enough
		// TODO: However, having a 'private-' namespace would make the regexes
		// a bit more rich
		log.Printf("Creating namespaces")
		needs := base.ClusterNeeds{
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

		// creating namespaces
		assert.Assert(t, pub1.CreateNamespace())
		assert.Assert(t, pub2.CreateNamespace())

		// labelling the namespaces
		pub1.LabelNamespace("test.skupper.io/test-namespace", "policy")
		pub2.LabelNamespace("test.skupper.io/test-namespace", "policy")
	})

	// teardown once test completes
	tearDownFn := func() {
		t.Log("entering teardown")
		if base.ShouldSkipPolicyTeardown() {
			t.Log("Skipping policy tear down, per env variables")
		} else {
			t.Log("Removing Policy CRD")
			removeCrd(t, pub1)
			t.Log("Removing cluster role skupper-service-controller from the CRD definition")
			pub1.VanClient.KubeClient.RbacV1().ClusterRoles().Delete("skupper-service-controller", nil)
		}
		if base.ShouldSkipNamespaceTeardown() {
			t.Log("Skipping namespace tear down, per env variables")
		} else {
			t.Log("Removing pub1 namespace")
			_ = pub1.DeleteNamespace()
			t.Log("Removing pub2 namespace")
			_ = pub2.DeleteNamespace()
		}
		t.Log("tearDown completed")
	}
	defer tearDownFn()
	base.HandleInterruptSignal(func() {
		tearDownFn()
	})

	if t.Failed() {
		t.Fatalf("Setup failed")
	}

	// Creating a local directory for storing the token
	testPath := "./tmp/"
	_ = os.Mkdir(testPath, 0755)

	// deploying frontend and backend services
	assert.Assert(t, deployResources(pub1, pub2))

	// ^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

	t.Run("apply-crd", func(t *testing.T) {
		if base.ShouldSkipPolicySetup() {
			t.Log("Skipping policy setup, per environment")
			return
		}
		// We first remove the CRD to make sure we have a clean slate for
		// policies before we start, then we re-add it
		// TODO: make a removeAllPolicies(), instead; it should be faster
		if _, err = removeCrd(t, pub1); err != nil {
			t.Fatalf("Failed to remove the CRD at the start: %v", err)
			return
		}
		if err = applyCrd(t, pub1); err != nil {
			t.Fatalf("Failed to add the CRD at the start: %v", err)
			return
		}
	})

	if t.Failed() {
		t.Fatalf("CRD setup failed")
	}

	initSteps := []cli.TestScenario{
		{
			Name: "initialize",
			Tasks: []cli.SkupperTask{
				{Ctx: pub1, Commands: []cli.SkupperCommandTester{
					// skupper init - interior mode, enabling console and internal authentication
					&cli.InitTester{
						ConsoleAuth:         "internal",
						ConsoleUser:         "internal",
						ConsolePassword:     "internal",
						RouterMode:          "interior",
						EnableConsole:       false,
						EnableRouterConsole: true,
					},
					// skupper status - verify initialized as interior
					&cli.StatusTester{
						RouterMode:          "interior",
						ConsoleEnabled:      false,
						ConsoleAuthInternal: true,
						PolicyEnabled:       true,
					},
				}},
			},
		},
	}

	testTable := []namespaceTest{
		{
			namespaces:     []string{"*"},
			worksOnTarget:  true,
			worksElsewhere: true,
		},
		{
			namespaces:     []string{"public-policy-namespaces-1"},
			worksOnTarget:  true,
			worksElsewhere: false,
		},
		{
			namespaces:     []string{"public-policy-namespaces-2"},
			worksOnTarget:  false,
			worksElsewhere: true,
		},
		{
			namespaces:     []string{".*1$"},
			worksOnTarget:  true,
			worksElsewhere: false,
		},
		{
			namespaces:     []string{".*2$"},
			worksOnTarget:  false,
			worksElsewhere: true,
		},
		{
			namespaces:     []string{"public"},
			worksOnTarget:  true,
			worksElsewhere: true,
		},
		{
			namespaces:     []string{`test.skupper.io/test-namespace=policy`},
			worksOnTarget:  true,
			worksElsewhere: true,
		},
		{
			namespaces:     []string{"non-existing-label=true"},
			worksOnTarget:  false,
			worksElsewhere: false,
		},
		{ // AND-behavior for labels in a single entry
			namespaces:     []string{`test.skupper.io/test-namespace=policy,non-existing-label=true`},
			worksOnTarget:  false,
			worksElsewhere: false,
		},
		{
			namespaces:     []string{`test.skupper.io/test-namespace=something_else`},
			worksOnTarget:  false,
			worksElsewhere: false,
		},
	}

	cli.RunScenarios(t, initSteps)

	if t.Failed() {
		t.Fatalf("Initialization failed")
	}

	for index, item := range testTable {
		t.Run(fmt.Sprintf("case-%d", index), func(t *testing.T) {
			policySpec := skupperv1.SkupperClusterPolicySpec{
				Namespaces:         item.namespaces,
				AllowIncomingLinks: true,
			}
			err = applyPolicy(t, "generated-policy", policySpec, pub1)
			if err != nil {
				t.Fatalf("Failed to apply policy: %v", err)
				return
			}
			cli.RunScenarios(
				t,
				[]cli.TestScenario{
					createToken(fmt.Sprintf("%d", index), pub1, testPath, item.worksOnTarget),
				},
			)
		})

	}

	t.Run("skupper-delete", func(t *testing.T) {

		cli.RunScenarios(
			t,
			[]cli.TestScenario{
				{
					Name: "skupper-delete",
					Tasks: []cli.SkupperTask{
						{
							Ctx: pub1, Commands: []cli.SkupperCommandTester{
								&cli.DeleteTester{},
							},
						},
					},
				},
			},
		)
	})

}

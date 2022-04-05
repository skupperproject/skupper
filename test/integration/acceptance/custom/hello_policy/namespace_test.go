package hello_policy

import (
	"fmt"
	"os"
	"testing"

	skupperv1 "github.com/skupperproject/skupper/pkg/apis/skupper/v1alpha1"
	"github.com/skupperproject/skupper/test/utils/base"
	"github.com/skupperproject/skupper/test/utils/skupper/cli"
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

func testNamespace(t *testing.T, pub1, pub2 *base.ClusterContext) {

	var err error

	// Creating a local directory for storing the token
	testPath := "./tmp/"
	_ = os.Mkdir(testPath, 0755)

	t.Run("apply-crd", func(t *testing.T) {
		if base.ShouldSkipPolicySetup() {
			t.Log("Skipping policy setup, per environment")
			return
		}
		// Should this be affected by base.ShouldSkipPolicySetup?
		// Should that method be renamed to include only CRD setup?
		if err = removePolicies(t, pub1); err != nil {
			t.Fatalf("Failed to remove policies")
		}
		if err = applyCrd(t, pub1); err != nil {
			t.Fatalf("Failed to add the CRD at the start: %v", err)
			return
		}
	})

	if t.Failed() {
		t.Fatalf("CRD setup failed")
	}

	initSteps := skupperInitInterior(pub1, true)

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
					createTokenPolicyScenario(fmt.Sprintf("%d", index), pub1, testPath, item.worksOnTarget),
				},
			)
		})

	}

	// TODO move this to tearDown?
	t.Run("skupper-delete", func(t *testing.T) {

		cli.RunScenarios(
			t,
			deleteSkupper(pub1),
		)
	})

}

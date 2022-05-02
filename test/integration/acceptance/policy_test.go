//go:build policy
// +build policy

package acceptance

import (
	"bytes"
	"fmt"
	"os"
	"testing"
	"text/template"
)

// Check
//		PreSkupperSetup: func(testRunner *base.ClusterTestRunnerBase) error {
//
// Combinations
// ============

// Basic test table.
// - For all that have policy repeat: "*", this, other, regex
// - Tests with no modifications: always run all tests.  Otherwise, base on env var
// - These could be starting points for longer tests
// - For long individual tests, perhaps the profile could be merged at each step,
//   so that the previous changes would be taken into account (instead of using
//   the base, which would no longer be valid)
// - In case of two separate clusters, install same policies on both?  And CRDs?
//   Simultaneous, or possibly add another layer with one or two options here?
//
// {
// 	Background: clean
// 	Base: allowAll
// 	Modifications: nil
// 	Checks: All in allow all
// },{
// 	Background: CRD, no policy
// 	Base: denyAll
// 	Modifications: nil
// 	Checks: All in deny all
// },{
// 	Background: CRD, all-allowing policy
// 	Base: allowAll
// 	Modifications: nil
// 	Checks: All in allow all
// },{
// 	Background: CRD, no policy
// 	Base: denyAll
// 	Modifications: add
// 	Checks: All in deny all
// },{
// 	Background: CRD, all-allowing policy
// 	Base: allowAll
// 	Modifications: nil
// 	Checks: All in allow all
// },{
//
//  modifications, remove CRD, add CRD, remove policy, add policy, specific policies

// 	Background: Update, clean
// 	Base: allowAll
// 	Modifications: nil
// 	Checks: All in allow all
// },{
// 	Background: Update, CRD
// 	Base: denyAll
// 	Modifications: nil
// 	Checks: All in deny all
// },{
// 	Background: Update, clean
// 	Base: allowAll
// 	Modifications: nil
// 	Checks: All in allow all
// },{
// 	Background: Update, clean
// 	Base: allowAll
// 	Modifications: nil
// 	Checks: All in allow all
// },{
// 	Background: Update, clean
// 	Base: allowAll
// 	Modifications: nil
// 	Checks: All in allow all
// },{

// Environment variables
const (
	// By default, not all checks are run for each test profile.  Instead,
	// only those tests specifically listed on the profile are run.
	// Setting this environment variable to any value changes that behavior,
	// and ensures that the non-overridden base profile checks are run as well
	ENV_SKUPPER_TEST_PROFILE_ALL_CHECKS = "SKUPPER_TEST_PROFILE_ALL_CHECKS"
)

var (
	// ALL_ENV_VARS is a list of all environment variables used by this test.
	// That list is presented on the output at the start of the run
	ALL_ENV_VARS = [...]string{
		ENV_SKUPPER_TEST_PROFILE_ALL_CHECKS,
	}
)

const (
	CURRENT_NO_CRD_NO_POLICY = iota
)

var Backgrounds = map[int]func(){
	CURRENT_NO_CRD_NO_POLICY: func() {

	},
}

// Done
// - Initial Policy templating
// - Infra for individual checks, but not any actual checks
// - Test Profile infra, using the checks above, with merging
// - Basic AllowAll Profile, and some individual test profiles
// - Basic testing using that infra, with non-combinatorial testing

// Todo
//
// - Backgrounds
// - Actual command execution (prep and checks)
// - Combinatorial
//   - This possibly includes some refactoring on TestProfile, so that changes to
//     to individual items are not combined (ie, namespaces "*", "asdf.*", ".*.querty"
//     will generate three combinations, and not be part of a single combination
// - For the combinatorial, the list of tests to be run could be from a non-source
//   file.  That could be used for ad-hoc semi-automated testing, or for keeping a
//   list of tests to be run on releases
// - Add Backgrounds to combinatorial

var PolicyTemplate = template.Must(template.New("PolicyTemplate").Parse(`

Namespaces: {{ .Namespaces }}

### Network control attributes
AllowIncomingLinks: {{ .AllowIncomingLinks }}
AllowedOutgoingLinksHostnames: {{ .AllowedOutgoingLinksHostnames }}

### Service control attributes
AllowedExposedResources: {{ .AllowedExposedResources }}
AllowedServices: {{ .AllowedServices }}

### Gateway control attributes
AllowGateway {{ .AllowGateway }}

`))

// Idea
//
// - Policy item
//   - Policy item option -> verifier
//
// On policy item, make sure to include invalid options (like a string where a number
// is expected).  Is the prior policy maintained in that case?
//
// There will be a class (or function) named 'TestCombo'.  It may receive a parameter
//
// n == 0    - Run only the basic test.  Set each policy option, leave the others on their
//             defaults
// 0 < n <=1 - Run a percentage of tests.  1 means all of them
// n > 1     - n will be an int.  Run exactly this number of tests.  For n = 2, that means
//             run the basic test plus one random combination
//
// For the basic test, as mentioned, it's just one policy item changing per test, the
// rest remains unchanged.
//
// For the combination: the number of generated combinations will be
//
//   (a + 1) * (b + 1) * (c + 1)...
//
// Where a, b, c... are the number of policy options for each policy item.  The +1 is for the
// case where that item is not changed.
//
// Once all possibilities are generated, they're shuffled and the number of test cases requested
// reserved (n).
//
// There should be some kind of checks to make sure that the same test does not run twice
// (for example, in case the combinatorial selected a basic test profile)
//
// Make sure to print:
//
// - The total number of test cases generated
// - The actual number selected to run
// - The time to run each
// - The exact profile being executed, in a manner that allows it to be reproduced easily
//
// Note: namespaces should drive defaults(?).  For example, on a DenyAll background, a policy
//       that allows linkCreation on the target namespace should allow.  However, if the
//	 policy is referring to another namespace, it should continue to be denied
//
// TODO: Allow for test profiles.  Either set by env variable pointing to a file, or
//       hardcoded (for example, if we find a bug on a specific profile, and want
//       to make sure it does not resurface)
//
// TODO: Use generated random namespace names, and check for possibility of parallel execution

//
// Backgrounds should have default tests and test overrides
//
// Default tests are what's going to be checked when the test does not define a specific test for
// the resource.  For example, on a background of CRD and no policy, the default checks should
// all fail.  Then, if a specific test enables a specific resource, it should specify how to
// test it.
//
// Overrides are for the case where the CRD is not defined.  The tests should still be run,
// but they should always give the result of the override.  Overrides are optional.

// Checks

// Token creation, link site, create service, expose resource, expose resource using
// annotations, create gateway

// console: create token, link sites

type PolicyCheck interface {
	Check() bool
}

type TokenCheck struct {
	Works bool
}

func (t TokenCheck) Check() bool {
	fmt.Println("Do something to confirm that token creation works")
	return true == t.Works
}

type ServiceCheck struct {
	Works bool
}

func (s ServiceCheck) Check() bool {
	fmt.Println("Do something to confirm that service creation works")
	return true == s.Works
}

type IncomingLinkCheck struct {
	Works bool
}

func (il IncomingLinkCheck) Check() bool {
	fmt.Println("Do something to confirm that incoming link creation works")
	return true == il.Works
}

type TestProfile struct {
	Namespaces                    string
	AllowIncomingLinks            string
	AllowedOutgoingLinksHostnames string
	AllowedExposedResources       string
	AllowedServices               string
	AllowGateway                  string

	Checks map[string]PolicyCheck
}

// Merge two TestProfiles.  For most fields, anything from the other profile
// that does not have a default value ("") will overwrite the receiver's
// copy values.
//
// For Checks, any checks defined on the other overwrite those on the receiver.
// Note that new checks can be added as well.
//
// This function returns a copy of the receiver, with the changes made
func (tp TestProfile) merge(other TestProfile) TestProfile {
	// I could  use a merging helper, here...

	if other.Namespaces != "" {
		tp.Namespaces = other.Namespaces
	}

	if other.AllowIncomingLinks != "" {
		tp.AllowIncomingLinks = other.AllowIncomingLinks
	}

	if other.AllowedOutgoingLinksHostnames != "" {
		tp.AllowedOutgoingLinksHostnames = other.AllowedOutgoingLinksHostnames
	}

	if other.AllowedExposedResources != "" {
		tp.AllowedExposedResources = other.AllowedExposedResources
	}

	if other.AllowedServices != "" {
		tp.AllowedServices = other.AllowedServices
	}

	if other.AllowGateway != "" {
		tp.AllowGateway = other.AllowGateway
	}

	for k, v := range other.Checks {
		tp.Checks[k] = v
	}

	return tp
}

// A base TestProfile with all policies in the most permissive
// state, and checks to confirm all operations that are affected
// by the policies
var AllowAll = TestProfile{
	Namespaces:                    `["*"]`,
	AllowIncomingLinks:            "true",
	AllowedOutgoingLinksHostnames: `["*"]`,
	AllowedExposedResources:       `["*"]`,
	AllowedServices:               `["*"]`,
	AllowGateway:                  "true",

	// Pros and cons of maps vs struct fields
	// maps are easier to merge and work with, but the
	// compiler will not notice a typo within a key
	Checks: map[string]PolicyCheck{
		"TokenCheck":        TokenCheck{true},
		"ServiceCheck":      ServiceCheck{true},
		"IncomingLinkCheck": IncomingLinkCheck{false},
	},

	//	IncomingLinkCheck PolicyCheck
	//	OutgoingLinkCheck PolicyCheck
	//	ExposeCheck       PolicyCheck
	//	ServiceCheck      PolicyCheck
	//	GatewayCheck      PolicyCheck
}

var (
	DenyLink = TestProfile{
		AllowIncomingLinks: "false",

		Checks: map[string]PolicyCheck{
			"TokenCheck":        TokenCheck{false},
			"IncomingLinkCheck": IncomingLinkCheck{false},
		},
	}

	DenyService = TestProfile{
		AllowedServices: "[]",

		Checks: map[string]PolicyCheck{"ServiceCheck": ServiceCheck{false}},
	}

	AllowService = TestProfile{
		AllowedServices: `[ "this_service" ]`,

		Checks: map[string]PolicyCheck{"ServiceCheck": ServiceCheck{true}},
	}

	RestrictService = TestProfile{
		AllowedServices: `[ "that_service" ]`,

		Checks: map[string]PolicyCheck{"ServiceCheck": ServiceCheck{false}},
	}
)

var IndividualTests = []TestProfile{DenyLink, DenyService, AllowService, RestrictService}

// This will test individual testProfiles in an AllowAll basic profile
func TestIndividualPolicyFields(t *testing.T) {

	t.Skip("Retired test.  Review and remove code")
	return
	t.Log("Environment variables:")
	for _, envVar := range ALL_ENV_VARS {
		t.Logf(" - %v: %v\n", envVar, os.Getenv(envVar))
	}

	for _, testProfile := range IndividualTests {
		mergedProfile := AllowAll.merge(testProfile)

		testName := fmt.Sprintf("%+v", mergedProfile)

		t.Run(testName, func(t *testing.T) {

			// TODO: move this to TestProfile
			buf := &bytes.Buffer{}
			if err := PolicyTemplate.Execute(buf, mergedProfile); err != nil {
				t.Fatal("Template generation failed", err)
			}
			t.Log("Generated policy:", buf.String())

			// Check results

			// By default, check only what's been explicity requested.
			// If SKUPPER_TEST_PROFILE_ALL_CHECKS is set on the environment, then
			// run all checks, including those from the base profile.
			_, testAllChecks := os.LookupEnv(ENV_SKUPPER_TEST_PROFILE_ALL_CHECKS)
			var actualChecks map[string]PolicyCheck
			if testAllChecks {
				actualChecks = mergedProfile.Checks
			} else {
				actualChecks = testProfile.Checks
			}

			for checkName, check := range actualChecks {
				t.Log("Checking", checkName)
				if !check.Check() {
					t.Errorf("Check %v failed: %+v", checkName, check)
				}
			}
		})

	}
}

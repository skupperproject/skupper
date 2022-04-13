package hello_policy

import (
	skupperv1 "github.com/skupperproject/skupper/pkg/apis/skupper/v1alpha1"
	"github.com/skupperproject/skupper/test/utils/skupper/cli"
)

// Each policy piece has its own file.  On it, we define both the
// piece-specific tests _and_ the piece-specific infra.
//
// For example, the checking for link being (un)able to create or being
// destroyed is defined on functions on link_test.go
//
// These functions will take a cluster context and an optional name prefix.  It
// will return a slice of cli.TestScenario with the intended objective on the
// requested cluster, and the names of the individual scenarios will receive
// the prefix, if any given.  A use of that prefix would be, for example, to
// clarify that what's being checked is a 'side-effect' (eg when a link drops
// in a cluster because the policy was removed on the other cluster)

//
// The idea:
//
// - set policyStart
// - run prep steps
// - set policyChange
// - run scenario
// - deleteLink(s), if extant
//
// prep steps and policyChange may be empty, if unnecessary
//
// Uses:
//   - policyStart: allowed
//     prep: create token
//     policyChange: disallow
//     run: try to create link with pre-created token
//   - policyStart: allowed
//     pre: create token, link
//     policyChange: disallow
//     run: stuff came down
//   - policyStart: disallow
//     prep: try to create link, fail
//     policyChange: allow
//     run: creations now work

// Configures a step on the policy test runner, which allows for setting
// policies on the two clusters, run a set of cli commands and then perform
// some checks using the `get` command.
//
// ATTENTION to how the policies work:
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
type policyTestStep struct {
	name        string
	pubPolicy   []skupperv1.SkupperClusterPolicySpec // ATTENTION to usage; see doc
	prvPolicy   []skupperv1.SkupperClusterPolicySpec
	commands    []cli.TestScenario
	pubGetCheck policyGetCheck
	prvGetCheck policyGetCheck
	parallel    bool // This will run the cli commands in parallel
}

// A named slice, with methods to run each step
type policyTestCase struct {
	name  string
	steps []policyTestStep
}

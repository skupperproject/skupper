//go:build policy
// +build policy

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
//     policyTestRunner (keepPolicies bool, background policies, contextMap)
//       []policyTestCase (just a name and a set of steps)
//         []policyTestStep (a name and all the actual configuration of the test: what to execute and how)
//             preHook
//             policies
//             GET checks
//             cliScenarios  ([]cli.TestScenario)
//
//             parallel (decide between cli.RunScenarios and cli.RunScenariosParallel)
//             skipFunction
//             sleep (post execution)
//
//
//     cli.TestScenario (name and list of tasks)
//       []cli.SkupperTask (which cluster to run, and a list of commands)
//         []cli.SkupperCommandTester (an interface; each item represent an individual call to the skupper binary)
package hello_policy

// TODO
//
// Pre-merge (priority on top)
// - Document
//   - The whole (how do things fit together)
//   - The items
//   - Rationale for individual test cases/steps
// - Stop at start if CRD already present (avoid changing pre-existing policies); update CI
// - Review Fernando's PR
//   - Better CRD removal at the end; check that two contexts point to the same cluster
// - Review old code and remove it (first attempt at hello_world)
// - Check TODO across the code
// - Check 'ExpectAuthError': change name to 'Expect no service'?
// - Add GETs, make test overal less flaky
// - Reorganize test calling from main_test
// - Ensure it works with upstream CI (especially host checking)
// - Check on status for multicluster checking
// - Re-implement hello_world using runner, composing functions
// - Dump, capture debug info on errors
// - List points where I could get help for better solutions (../../../../.../crd)
//
// Post-merge (priority on top)
// - Confirm 'not-bound' checks are really checking services for not being bound
// - Check for tests that need better finish
//   - AllowedOutgoingLinksHostnames
//     - Cross testing (claim on router and vice versa)
//     - full setup checking (create service and expose; check they appear/disappear; perhaps even curl the service)
//     - different removals and reinstates of policy (actual removal, changed namespace list)
// - Define how specific-issue (reproducer) tests are going to be handled
// - Non-admin skupper init
// - Non-admin user (or: use admin only for CRD/CR, init)
// - Review test structure.  In special repeated items (test_name#01)
// - Check test coverage (specific image and all)
// - Additional tests: gateway, annotation, upgrade, console
// - Operator + config map
//

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

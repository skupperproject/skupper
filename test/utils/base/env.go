package base

import "os"

// This file contains the different environment variables that affect the
// running of tests.
//
// Note that individual tests may or may not implement each of the options.
//
// The constant names start with ENV_ and continue with a description of their
// use.  The actual variable names substitute ENV_ for SKUPPER_TEST_.
//
// For most variables, simply being set already activates their behavior, with
// any value.  If a variable works differently, that should be described on its
// comments.

const (
	// Skips the creation of namespaces.  Used during testing development,
	// to speed up test runs, by reusing a previously-set environment
	ENV_SKIP_NAMESPACE_SETUP = "SKUPPER_TEST_SKIP_NAMESPACE_SETUP"

	// Skips the teardown of namespaces.  Used during testing development,
	// to leave a test setup behind for semi-automated testing, or for
	// speeding up test runs
	ENV_SKIP_NAMESPACE_TEARDONW = "SKUPPER_TEST_SKIP_NAMESPACE_TEARDOWN"

	// Skips the initial setup of policies, for those tests where policies
	// are used.  Used for speeding up test execution and for semi-automated
	// testing.
	ENV_SKIP_POLICY_SETUP = "SKUPPER_TEST_SKIP_POLICY_SETUP"

	// Skips the teardown of policies, for those tests where policies
	// are used.  In practice, that means that the CRD will be left on
	// the environment, as well as any policy CRs.  Used for speeding up
	// test execution and for semi-automated testing
	ENV_SKIP_POLICY_TEARDOWN = "SKUPPER_TEST_SKIP_POLICY_TEARDOWN"

	// If defined, both stdout and stderr of all issued skupper commands
	// will be shown on the test output, even if they did not fail
	ENV_VERBOSE_COMMANDS = "SKUPPER_TEST_VERBOSE_COMMANDS"
)

func ShouldSkipNamespaceSetup() bool {
	_, found := os.LookupEnv(ENV_SKIP_NAMESPACE_SETUP)
	return found
}

func ShouldSkipNamespaceTeardown() bool {
	_, found := os.LookupEnv(ENV_SKIP_NAMESPACE_TEARDONW)
	return found
}

func ShouldSkipPolicyTeardown() bool {
	_, found := os.LookupEnv(ENV_SKIP_POLICY_TEARDOWN)
	return found
}

func ShouldSkipPolicySetup() bool {
	_, found := os.LookupEnv(ENV_SKIP_POLICY_SETUP)
	return found
}

func IsVerboseCommandOutput() bool {
	_, showVerbose := os.LookupEnv(ENV_VERBOSE_COMMANDS)
	return showVerbose
}

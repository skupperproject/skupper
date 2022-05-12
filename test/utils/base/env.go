package base

import (
	"log"
	"os"
	"strconv"
	"time"
)

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

// ** CLI **
const (

	// If defined, calls to cli.RunScenariosParallel will actually be run
	// in serial.  Use this, for example, when the output of the tests is
	// too difficult to read because of the parallelism
	ENV_CLI_NO_PARALLEL = "SKUPPER_TEST_CLI_NO_PARALLEL"

	// If defined, the status commands will try at most this number of
	// attempts (an int).  Otherwise, they'll fail only on the timeout.
	ENV_MAX_STATUS_ATTEMPTS = "SKUPPER_TEST_MAX_STATUS_ATTEMPTS"

	// If defined, both stdout and stderr of all issued skupper commands
	// will be shown on the test output, even if they did not fail
	ENV_VERBOSE_COMMANDS = "SKUPPER_TEST_VERBOSE_COMMANDS"
)

// ** TODO **
const (

	// Skips the creation of namespaces.  Used during testing development,
	// to speed up test runs, by reusing a previously-set environment
	ENV_SKIP_NAMESPACE_SETUP = "SKUPPER_TEST_SKIP_NAMESPACE_SETUP"

	// Skips the teardown of namespaces.  Used during testing development,
	// to leave a test setup behind for semi-automated testing, or for
	// speeding up test runs
	ENV_SKIP_NAMESPACE_TEARDONW = "SKUPPER_TEST_SKIP_NAMESPACE_TEARDOWN"
)

// ** POLICY **
const (

	// Skips the initial setup of policies, for those tests where policies
	// are used.  Used for speeding up test execution and for semi-automated
	// testing.
	ENV_SKIP_POLICY_SETUP = "SKUPPER_TEST_SKIP_POLICY_SETUP"

	// Skips the teardown of policies, for those tests where policies
	// are used.  In practice, that means that the CRD will be left on
	// the environment, as well as any policy CRs.  Used for speeding up
	// test execution and for semi-automated testing
	ENV_SKIP_POLICY_TEARDOWN = "SKUPPER_TEST_SKIP_POLICY_TEARDOWN"

	// this is used by policyTestStep at test/integration/acceptance/custom/hello_policy/runner.go
	// It's the number of seconds to wait after any policy changes take effect.  If the PolicyStep
	// defined several policy changes, they'll all run one after the other, then the sleep will
	// kick in.  If no policy changes, no sleep.
	ENV_POST_POLICY_CHANGE_SLEEP = "SKUPPER_TEST_POST_POLICY_CHANGE_SLEEP"

	// By default, after each policy change, the policy runner will wait for all of the
	// configured checks (if any) to return success before moving on.  If this variable is
	// set, the checks will run only once and the test step will fail on unexpected response.
	// The GET checks are used to wait for the policy changes to stabilize before moving to
	// the CLI tasks.  This variable removes that wait, so it can be used to see how the
	// system behaves when tests are run before the changes are finished.
	ENV_POLICY_NO_GET_WAIT = "SKUPPER_TEST_POLICY_NO_GET_WAIT"
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

func ShouldRunScenariosInParallel() bool {
	_, found := os.LookupEnv(ENV_CLI_NO_PARALLEL)
	return !found
}

// Checks for the environment variable configuration and
// executes the sleep
func PostPolicyChangeSleep() {
	envSleep := os.Getenv(ENV_POST_POLICY_CHANGE_SLEEP)

	sleep, err := strconv.Atoi(envSleep)

	if err == nil {
		log.Printf("Waiting %vs after policy change, per environment variable configuration", sleep)
		time.Sleep(time.Duration(sleep) * time.Second)
	}
}

// This checks whether the current attempt sent as an argument
// is greather than the environment variable ENV_MAX_STATUS_ATTEMPTS
//
// If the variable is not set or is malformed, this will always
// return false (meaning that the status commands will only fail once
// they reach their timeout)
func IsMaxStatusAttemptsReached(currentAttempt int) bool {

	envMax := os.Getenv(ENV_MAX_STATUS_ATTEMPTS)

	max, err := strconv.Atoi(envMax)

	if err != nil {
		// We do not error if someone put an invalid value on the
		// env variable
		return false
	}

	return max < currentAttempt

}

func ShouldPolicyWaitOnGet() bool {
	_, setting := os.LookupEnv(ENV_POLICY_NO_GET_WAIT)
	return !setting
}

package cli

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os/exec"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/skupperproject/skupper/test/utils/base"
	"gotest.tools/assert"
)

const (
	SkupperBinary  = "skupper"
	commandTimeout = 2 * time.Minute
)

// SkupperCommandTester defines an interface for all skupper (binary)
// commands. The idea is that each command implementation provides a
// set of Properties to help defining the command line execution and
// it must be able to run the command and validate the results.
type SkupperCommandTester interface {
	// Command returns a slice of strings representing the composed arguments
	Command(cluster *base.ClusterContext) []string
	// Run executed given command using the skupper binary and validates
	// if execution was successful, returning stdout, stderr and error
	Run(cluster *base.ClusterContext) (string, string, error)
}

// TestScenario represents a set of tasks performed using the skupper cli.
// It helps grouping a set of commands that can be performed against
// different clusters.
type TestScenario struct {
	Name  string
	Tasks []SkupperTask
}

// Appends the tasks from other TestScenarios to this one.  Use this for
// composing complex scenarios from simpler ones.
func (ts *TestScenario) AppendTasks(others ...TestScenario) {
	for _, scenario := range others {
		ts.Tasks = append(ts.Tasks, scenario.Tasks...)
	}
}

// SkupperTask defines a set of skupper commands (init, status, expose, ...) that will be
// executed in the given ClusterContext
type SkupperTask struct {
	Ctx      *base.ClusterContext
	Commands []SkupperCommandTester
}

// Helper function that runs all tasks for a given scenario against
// the specified cluster. If an error occurs, it stops processing
// the remaining tasks.
func RunScenario(scenario TestScenario) (string, string, error) {
	log.Printf("Running Skupper Command Tester scenario: %s\n", scenario.Name)
	var stdout, stderr string
	for _, task := range scenario.Tasks {
		for _, cmd := range task.Commands {
			stdout, stderr, err := cmd.Run(task.Ctx)
			if err != nil {
				return stdout, stderr, err
			}
		}
	}
	return stdout, stderr, nil
}

func RunScenarios(t *testing.T, scenarios []TestScenario) {
	var stdout, stderr string
	var err error

	if len(scenarios) > 1 {
		log.Print("Scenario set outline:")
		for i, scenario := range scenarios {
			log.Printf("%2d - %v", i, scenario.Name)
		}
	}

	// Running the scenarios
	for _, scenario := range scenarios {
		passed := t.Run(scenario.Name, func(t *testing.T) {
			stdout, stderr, err = RunScenario(scenario)
			assert.Assert(t, err)
		})
		if !passed {
			t.Fail()
			log.Printf("%s has failed, exiting", scenario.Name)
			log.Printf("STDOUT:\n%s", stdout)
			log.Printf("STDERR:\n%s", stderr)
			break
		}
	}
}

// Runs a list of []TestScenario in parallel.  Each scenario will run in
// parallel to the other.  However, the RunScenariosParallel call will only
// return when all of them finish.  This allows the caller function to do
// other in-test steps that depend on the RunScenariosParallel items finishing.
//
// To implement and signify that in the output, the steps are enclosed in a
// test called simply 'parallel'.
func RunScenariosParallel(t *testing.T, scenarios []TestScenario) {
	var stdout, stderr string
	var err error

	t.Run("parallel", func(t *testing.T) {
		log.Print("Parallel scenario set outline:")
		for i, scenario := range scenarios {
			log.Printf("%2d - %v", i, scenario.Name)
		}

		// Running the scenarios
		for _, scenario := range scenarios {
			// Make those local, so each run of the closure uses its own version
			scenario, stdout, stderr, err := scenario, stdout, stderr, err
			passed := t.Run(scenario.Name, func(t *testing.T) {
				if base.ShouldRunScenariosInParallel() {
					t.Parallel()
				}
				stdout, stderr, err = RunScenario(scenario)
				assert.Assert(t, err)
			})
			if !passed {
				t.Fail()
				log.Printf("%s has failed, exiting", scenario.Name)
				log.Printf("STDOUT:\n%s", stdout)
				log.Printf("STDERR:\n%s", stderr)
				break
			}
		}
	})
}

// RunSkupperCli executes the skupper binary (assuming it is available
// in the PATH), returning stdout, stderr and error.
func RunSkupperCli(args []string) (string, string, error) {

	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)

	// Defining a context with a timeout
	ctx, fn := context.WithTimeout(context.TODO(), commandTimeout)
	defer fn()

	// TODO:
	// - Dry run: SKUPPER_TEST_CLI_DRY_RUN: only print the commands
	// - call main(): (instead of exec) allow for checking coverage

	// Preparing the command to run
	cmd := exec.CommandContext(ctx, SkupperBinary, args...)
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	// Running the skupper command
	log.Printf("Running: skupper %s\n", strings.Join(args, " "))
	err := cmd.Run()
	if base.IsVerboseCommandOutput() {
		fmt.Printf("STDOUT:\n%v\n", stdout.String())
		fmt.Printf("STDERR:\n%v\n", stderr.String())
		fmt.Printf("Error: %v\n", err)
	}
	return stdout.String(), stderr.String(), err
}

// SkupperCommonOptions returns a list of all options that are common
// to all skupper commands
func SkupperCommonOptions(cluster *base.ClusterContext) []string {
	args := []string{}

	args = append(args, "--namespace", cluster.Namespace)
	args = append(args, "--kubeconfig", cluster.KubeConfig)

	return args
}

// A way to verify cli commands output
//
// StdOut and StdErr take a slice of plain strings.  It will expect that
// each string comes after the previous one.  In other words, the search
// for the second string from StdOut starts where the match for the first
// one finished.
//
// If you want to search for one static string instead, where there is
// nothing in between each segment, just use a single item with one big
// string
//
// StdOutRe and StdErrRe take a slice of regular expressions.  Those do
// not have the same restriction on one coming after the other.  If you
// want that behavior with regexes, create a single regex with the two
// expressions you're looking for.
//
// StdOutReNot and StdErrReNot behave like the previous ones, but ensure
// that the patters are not there in the checked string
type Expect struct {
	StdOut      []string
	StdErr      []string
	StdOutRe    []regexp.Regexp
	StdErrRe    []regexp.Regexp
	StdOutReNot []regexp.Regexp
	StdErrReNot []regexp.Regexp
}

// Looks for each bit (a substring), inside the string s, in order
//
// The 'name' is used for the error message.
func checkPlain(s string, bits []string, name string) (err error) {
	var startPos int
	missingPieces := []string{}

	for _, item := range bits {
		partial := s[startPos:]

		index := strings.Index(partial, item)
		if index >= 0 {
			// We found something, so the next check will start
			// where that match finished
			startPos += index + len(item)
		} else {
			missingPieces = append(missingPieces, item)
			// we continue even if an error, to report all missing pieces
		}
	}

	if len(missingPieces) > 0 {
		if len(bits) == 1 {
			err = fmt.Errorf(
				"Expected %v: \n%s\n",
				name,
				bits[0],
			)
		} else {
			msg := fmt.Sprintf(
				"Expected %v:\n%s\nmissing bits:\n",
				name,
				strings.Join(bits, "(...)"),
			)
			for _, mp := range missingPieces {
				msg = fmt.Sprintf("%v- %v\n", msg, mp)
			}
			err = fmt.Errorf(msg)
		}
	}
	return
}

// Looks for each bit (a regular expression), inside the string s.  Each bit
// is checked against the whole string, so they can be in a different order
// in the string.
//
// If expected is true; a bit that does not match will be an error; if it is
// false, a bit that matches will be an error.
//
// The 'name' is used for the error message.
func checkRe(s string, bits []regexp.Regexp, name string, expected bool) (err error) {

	var problems []string

	for _, b := range bits {
		match := b.MatchString(s)
		if match && !expected {
			problems = append(problems, fmt.Sprintf("Unexpected %s: regular expression %v matched", name, b))
		}

		if !match && expected {
			problems = append(problems, fmt.Sprintf("Expected %s not found: regular expression %v did not match", name, b))
		}
	}

	if len(problems) > 0 {
		message := fmt.Sprintf("Errors checking regular expressions on %v:\n", name)
		for _, p := range problems {
			message += fmt.Sprintf("- %s\n", p)
		}
		err = fmt.Errorf(message)
	}

	return err
}

// Groups and reports on a set of errors for the same input
func groupErrors(name, actual string, errors []error) (err error) {

	var hasErrors bool
	for _, e := range errors {
		if e != nil {
			hasErrors = true
			break
		}
	}
	if !hasErrors {
		return
	}
	message := "Incorrect output:\n"
	for _, e := range errors {
		if e != nil {
			message += fmt.Sprintf("%v\n", e)
		}
	}
	message += fmt.Sprintf("Actual %v:\n%v", name, actual)

	return fmt.Errorf(message)
}

// Checks all items from the specification.
func (e Expect) Check(stdout, stderr string) (err error) {

	stdOutErrors := groupErrors(
		"stdout",
		stdout,
		[]error{
			checkPlain(stdout, e.StdOut, "stdout"),
			checkRe(stdout, e.StdOutRe, "stdout", true),
			checkRe(stdout, e.StdOutReNot, "stdout", false),
		})
	stdErrErrors := groupErrors(
		"stderr",
		stderr,
		[]error{
			checkPlain(stderr, e.StdErr, "stderr"),
			checkRe(stderr, e.StdErrRe, "stderr", true),
			checkRe(stderr, e.StdErrReNot, "stderr", false),
		})

	var message string
	if stdOutErrors != nil {
		message += fmt.Sprint(stdOutErrors)
	}
	if stdErrErrors != nil {
		message += fmt.Sprint(stdErrErrors)
	}

	if message != "" {
		err = fmt.Errorf(message)
	}
	return

}

// Returns a pointer to a boolean value.
//
// Some structures use nil to mark undefined values, and they
// use this to return a boolean when the value is defined.
func Boolp(value bool) *bool {
	return &value
}

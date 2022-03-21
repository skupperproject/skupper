package cli

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
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

const (
	ENV_SKUPPER_TEST_VERBOSE_COMMANDS = "SKUPPER_TEST_VERBOSE_COMMANDS"
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

	t.Log("Scenario set outline:")
	for i, scenario := range scenarios {
		t.Logf("%2d - %v", i, scenario.Name)
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
	if _, showVerbose := os.LookupEnv(ENV_SKUPPER_TEST_VERBOSE_COMMANDS); showVerbose {
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

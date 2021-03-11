package token

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/skupperproject/skupper/test/utils/base"
	"github.com/skupperproject/skupper/test/utils/skupper/cli"
)

// CreateTester runs `skupper token create` command, validating
// the output as well as asserting token file has been created.
type CreateTester struct {
	Name     string
	FileName string
}

func (t *CreateTester) Command(cluster *base.ClusterContext) []string {
	args := cli.SkupperCommonOptions(cluster)
	args = append(args, "token", "create", t.FileName)

	if t.Name != "" {
		args = append(args, "--name", t.Name)
	}

	return args
}

func (t *CreateTester) Run(cluster *base.ClusterContext) (stdout string, stderr string, err error) {
	// Execute token create command
	stdout, stderr, err = cli.RunSkupperCli(t.Command(cluster))
	if err != nil {
		return
	}

	// Validating output
	log.Printf("Validating 'skupper token create'")

	log.Println("validating stdout")
	expectedOutput := fmt.Sprintf("Connection token written to %s", t.FileName)
	if !strings.Contains(stdout, expectedOutput) {
		err = fmt.Errorf("output did not match - expected: %s - found: %s", expectedOutput, stdout)
		return
	}

	// Validating that token file exists
	log.Println("validating token file")
	_, err = os.Stat(t.FileName)
	if err != nil {
		err = fmt.Errorf("token file was not created - %v", err)
		return
	}

	return
}

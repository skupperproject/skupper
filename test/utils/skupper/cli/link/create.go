package link

import (
	"fmt"
	"log"
	"regexp"
	"strconv"

	"github.com/skupperproject/skupper/test/utils/base"
	"github.com/skupperproject/skupper/test/utils/skupper/cli"
)

// CreateTester runs `skupper link create` and asserts output
// contains what is expected by the user.
type CreateTester struct {
	TokenFile string
	Name      string
	Cost      int
}

func (l *CreateTester) Command(cluster *base.ClusterContext) []string {
	args := cli.SkupperCommonOptions(cluster)
	args = append(args, "link", "create", l.TokenFile)

	if l.Name != "" {
		args = append(args, "--name", l.Name)
	}

	if l.Cost > 0 {
		args = append(args, "--cost", strconv.Itoa(l.Cost))
	}

	return args
}

func (l *CreateTester) Run(cluster *base.ClusterContext) (stdout string, stderr string, err error) {
	// Execute link create command
	stdout, stderr, err = cli.RunSkupperCli(l.Command(cluster))
	if err != nil {
		return
	}

	// Validating output
	log.Printf("Validating 'skupper link create'")

	// Preparing regex to validate output
	connectionNameRegex := "conn[0-9]+"
	if l.Name != "" {
		connectionNameRegex = l.Name
	}

	// Example: Skupper configured to connect to 10.0.0.1:45671 (name=conn1)
	outRegex := regexp.MustCompile(fmt.Sprintf(`Skupper configured to connect to .* \(name=%s\)`, connectionNameRegex))

	// Ensure stdout matches expected regexp
	if !outRegex.MatchString(stdout) {
		err = fmt.Errorf("expected output does not match - found: %s", stdout)
		return
	}

	return
}

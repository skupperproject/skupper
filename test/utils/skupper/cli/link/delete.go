package link

import (
	"fmt"
	"log"
	"regexp"

	"github.com/skupperproject/skupper/test/utils/base"
	"github.com/skupperproject/skupper/test/utils/skupper/cli"
)

// DeleteTester runs `skupper link delete` and asserts output
// contains what is expected by the user.
type DeleteTester struct {
	Name string
}

func (l *DeleteTester) Command(cluster *base.ClusterContext) []string {
	args := cli.SkupperCommonOptions(cluster)
	args = append(args, "link", "delete", l.Name)
	return args
}

func (l *DeleteTester) Run(cluster *base.ClusterContext) (stdout string, stderr string, err error) {
	// Execute link create command
	stdout, stderr, err = cli.RunSkupperCli(l.Command(cluster))
	if err != nil {
		return
	}

	// Validating output
	log.Printf("Validating 'skupper link delete'")

	// Example: Link 'conn1' has been removed
	linkName := l.Name
	if l.Name == "" {
		linkName = `conn[0-9]+`
	}
	outRegex := regexp.MustCompile(fmt.Sprintf(`Link '%s' has been removed`, linkName))

	// Ensure stdout matches expected regexp
	if !outRegex.MatchString(stdout) {
		err = fmt.Errorf("expected output does not match - found: %s", stdout)
		return
	}

	return
}

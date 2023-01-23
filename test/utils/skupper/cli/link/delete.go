package link

import (
	"fmt"
	"log"
	"regexp"

	"github.com/skupperproject/skupper/api/types"

	"github.com/skupperproject/skupper/test/utils/base"
	"github.com/skupperproject/skupper/test/utils/skupper/cli"
)

// DeleteTester runs `skupper link delete` and asserts output
// contains what is expected by the user.
type DeleteTester struct {
	Name string
}

func (l *DeleteTester) Command(platform types.Platform, cluster *base.ClusterContext) []string {
	args := cli.SkupperCommonOptions(platform, cluster)
	args = append(args, "link", "delete", l.Name)
	return args
}

func (l *DeleteTester) Run(platform types.Platform, cluster *base.ClusterContext) (stdout string, stderr string, err error) {
	// Execute link create command
	stdout, stderr, err = cli.RunSkupperCli(l.Command(platform, cluster))
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

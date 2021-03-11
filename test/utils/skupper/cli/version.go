package cli

import (
	"fmt"
	"log"
	"regexp"

	"github.com/skupperproject/skupper/test/utils/base"
)

// VersionTester runs `skupper version` and validates its output
type VersionTester struct{}

func (v *VersionTester) Command(cluster *base.ClusterContext) []string {
	args := SkupperCommonOptions(cluster)
	args = append(args, "version")
	return args
}

func (v *VersionTester) Run(cluster *base.ClusterContext) (stdout string, stderr string, err error) {
	// Execute version command
	stdout, stderr, err = RunSkupperCli(v.Command(cluster))
	if err != nil {
		return
	}

	// Validate the version for all the components is displayed
	log.Printf("Validating 'skupper version'")
	for _, component := range []string{"client", "transport", "controller"} {
		regex := regexp.MustCompile(fmt.Sprintf(`%s version .* \S`, component))
		if !regex.MatchString(stdout) {
			err = fmt.Errorf("missing expected content - regex: %s - stdout: %s", regex.String(), stdout)
		}
	}

	return
}

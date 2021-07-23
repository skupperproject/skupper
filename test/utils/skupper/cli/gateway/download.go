package gateway

import (
	"fmt"
	"os"
	"regexp"

	"github.com/skupperproject/skupper/test/utils/base"
	"github.com/skupperproject/skupper/test/utils/skupper/cli"
)

// DownloadTester runs `skupper gateway download` and asserts that
// a tar ball has been generated
type DownloadTester struct {
	OutputPath string
	Name       string
}

func (d *DownloadTester) Command(cluster *base.ClusterContext) []string {
	args := cli.SkupperCommonOptions(cluster)
	args = append(args, "gateway", "download", d.OutputPath)

	if d.Name != "" {
		args = append(args, "--name", d.Name)
	}

	return args
}

func (d *DownloadTester) Run(cluster *base.ClusterContext) (stdout string, stderr string, err error) {
	// Execute the gateway download command
	stdout, stderr, err = cli.RunSkupperCli(d.Command(cluster))
	if err != nil {
		return
	}

	// Basic validation of the stdout
	tarBall := fmt.Sprintf("%s/%s.tar.gz", d.OutputPath, d.Name)
	if matched, _ := regexp.MatchString(fmt.Sprintf(`Skupper gateway definition written to '%s'`, tarBall), stdout); !matched {
		err = fmt.Errorf("output does not contain expected content - found: %s", stdout)
		return
	}

	// Verifying that tar ball file exists
	_, err = os.Stat(tarBall)
	if err != nil {
		return
	}

	return
}

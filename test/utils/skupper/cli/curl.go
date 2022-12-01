package cli

import (
	"fmt"
	"strings"

	"github.com/skupperproject/skupper/test/utils/base"
)

// CurlTester is not a true SkupperCommandTester, as it does not call Skupper.
// Instead, it calls curl on the provided target url, from within the router
// deployment.  curl is executed with -f, so HTTP responses above 400 will
// cause it to report a non-zero
type CurlTester struct {
	Target string
	// Possible improvements: search output, use CA, cert, key files
}

func (c *CurlTester) Command(cluster *base.ClusterContext) []string {
	return []string{"exec -c router -ti deployment/skupper-router -- curl -fsSvk ", c.Target}
}

// As this calls cluster.KubectlExec, stdout and stderr will be together in the stdout
// return value.  stderr is always empty.
func (c *CurlTester) Run(cluster *base.ClusterContext) (stdout string, stderr string, err error) {
	// Execute expose command
	out, err := cluster.KubectlExec(fmt.Sprintf(strings.Join(c.Command(cluster), " ")))
	return string(out), "", err
}

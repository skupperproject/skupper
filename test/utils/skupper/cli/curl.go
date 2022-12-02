package cli

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/skupperproject/skupper/pkg/utils"
	"github.com/skupperproject/skupper/test/utils/base"
)

// CurlTester is not a true SkupperCommandTester, as it does not call Skupper.
// Instead, it calls curl on the provided target url, from within the router
// deployment.  curl is executed with -f, so HTTP responses above 400 will
// cause it to report a non-zero
type CurlTester struct {
	Target     string
	ExpectFail bool
	Interval   time.Duration
	MaxRetries int
	// Possible improvements: search output, use CA, cert, key files
}

func (c *CurlTester) Command(cluster *base.ClusterContext) []string {
	return []string{"exec -c router deployment/skupper-router -- curl -fsSvk ", c.Target}
}

// As this calls cluster.KubectlExec, stdout and stderr will be together in the stdout
// return value.  stderr is always empty.
func (c *CurlTester) Run(cluster *base.ClusterContext) (stdout string, stderr string, err error) {
	// TODO replace this by test/utils/tools/curl.go?
	var out []byte
	var count int
	var works string
	if c.ExpectFail {
		works = "does not work"
	} else {
		works = "works"
	}
	utils.RetryError(c.Interval, c.MaxRetries, func() error {
		count++
		log.Printf("Validating url %s %s - attempt %d", c.Target, works, count)
		out, err = cluster.KubectlExec(fmt.Sprintf(strings.Join(c.Command(cluster), " ")))
		if c.ExpectFail {
			if err != nil {
				err = nil
			} else {
				err = fmt.Errorf("expected error on curl operation, but it succeeded")
			}
		}
		return err
	})
	return string(out), "", err
}

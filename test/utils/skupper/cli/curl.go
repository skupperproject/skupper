package cli

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/skupperproject/skupper/pkg/utils"
	"github.com/skupperproject/skupper/test/utils/base"
	"github.com/skupperproject/skupper/test/utils/tools"
)

// CurlTester is not a true SkupperCommandTester, as it does not call Skupper.
// Instead, it calls curl on the provided target url, from within the router
// deployment.  curl is executed with -f, so HTTP responses above 400 will
// cause it to report a non-zero
type CurlTester struct {
	Target     string
	ExpectFail bool
	Silent     bool
	ShowError  bool
	Verbose    bool
	MaxRetries int
	// Retry interval.  Default is 1s
	Interval time.Duration
	// Individual curl invocation timeout in seconds.  If not given, the default is 10
	TimeOut int
	// Possible improvements: search output, use CA, cert, key files
}

func (c *CurlTester) Command(cluster *base.ClusterContext) []string {
	// The curl command is created on tools.Curl()
	return []string{}
}

// As this uses util.Curl, headers, body and other info will be together in the stdout
// return value.
func (c *CurlTester) Run(cluster *base.ClusterContext) (stdout string, stderr string, err error) {
	// TODO replace this by test/utils/tools/curl.go?
	var resp *tools.CurlResponse
	var count int
	var works string
	if c.ExpectFail {
		works = "does not work"
	} else {
		works = "works"
	}
	var interval = c.Interval
	if interval == 0 {
		interval = time.Second
	}
	var timeout = c.TimeOut
	if timeout == 0 {
		timeout = 10
	}
	var cOpts = tools.CurlOpts{
		Timeout:   timeout,
		ShowError: c.ShowError,
		Silent:    c.Silent,
		Verbose:   c.Verbose,
	}
	log.Printf("Running: curl %v", c)
	// utils.RetryError does not allow its MaxRetries == 0, and it is actually
	// the number of tries, not retries
	var tries = c.MaxRetries + 1
	utils.RetryError(interval, tries, func() error {
		count++
		log.Printf("Validating url %s %s - attempt %d", c.Target, works, count)
		resp, err = tools.Curl(cluster.VanClient.KubeClient, cluster.VanClient.RestConfig, cluster.VanClient.Namespace, "", c.Target, cOpts)
		log.Printf("curl returned HTTP response %d (%v)", resp.StatusCode, err)
		if c.ExpectFail {
			if err != nil || resp.StatusCode >= 400 {
				err = nil
			} else {
				err = fmt.Errorf("expected error on curl operation, but it succeeded")
			}
		} else {
			if resp.StatusCode >= 400 {
				err = fmt.Errorf("HTTP error: %d", resp.StatusCode)
			}
		}
		return err
	})
	out := fmt.Sprintf(
		"- HTTP Version: %s\n- Reason: %s\n- Headers:\n%s\n- Body\n%s",
		resp.HttpVersion,
		strings.TrimSpace(resp.ReasonPhrase),
		resp.Headers,
		resp.Body,
	)
	if base.IsVerboseCommandOutput() || err != nil {
		fmt.Printf("RESULT:\n%v\n", out)
		fmt.Printf("STDERR:\n%v\n", resp.Output)
		fmt.Printf("Error: %v\n", err)
	}
	return string(out), resp.Output, err
}

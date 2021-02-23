// +build integration

package http

import (
	"context"
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/skupperproject/skupper/test/utils/base"
	"github.com/skupperproject/skupper/test/utils/k8s"
	"golang.org/x/net/http2"
	"gotest.tools/assert"

	"github.com/davecgh/go-spew/spew"
	vegeta "github.com/tsenart/vegeta/v12/lib"
)

func TestMain(m *testing.M) {
	base.ParseFlags()
	os.Exit(m.Run())
}

func TestHttp(t *testing.T) {
	needs := base.ClusterNeeds{
		NamespaceId:     "http",
		PublicClusters:  1,
		PrivateClusters: 1,
	}
	testRunner := &HttpClusterTestRunner{}
	testRunner.BuildOrSkip(t, needs, nil)
	ctx, cancel := context.WithCancel(context.Background())
	base.HandleInterruptSignal(t, func(t *testing.T) {
		base.TearDownSimplePublicAndPrivate(&testRunner.ClusterTestRunnerBase)
		cancel()
	})
	testRunner.Run(ctx, t)
}

func TestHttpJob(t *testing.T) {
	testHttpJob(t, "http://httpbin:8080/")
}

func TestHttp2Job(t *testing.T) {
	//TODO: enable this if we add suport for "Upgrade" in skupper http2
	//testHttpJob(t, "http://nghttp2:8443/")

	k8s.SkipTestJobIfMustBeSkipped(t)

	//https://www.mailgun.com/blog/http-2-cleartext-h2c-client-example-go/
	//hack to support h2c
	client := http.Client{
		Transport: &http2.Transport{
			// So http2.Transport doesn't complain the URL scheme isn't 'https'
			AllowHTTP: true,
			// Pretend we are dialing a TLS endpoint.
			// Note, we ignore the passed tls.Config
			DialTLS: func(network, addr string, cfg *tls.Config) (net.Conn, error) {
				return net.Dial(network, addr)
			},
		},
	}

	resp, err := client.Get("http://nghttp2:8443/")
	assert.Assert(t, err)
	fmt.Printf("Client Proto: %d\n", resp.ProtoMajor)
	fmt.Println("Client Header:", resp.Header)

	defer resp.Body.Close()
	_body, err := ioutil.ReadAll(resp.Body)
	assert.Assert(t, err)

	body := string(_body)
	assert.Assert(t, strings.Contains(body, "A simple HTTP Request &amp; Response Service."), body)
	assert.Assert(t, resp.Status == "200 OK", resp.Status)
}

func testHttpJob(t *testing.T, url string) {
	k8s.SkipTestJobIfMustBeSkipped(t)
	fmt.Printf("Running job for url: %s\n", url)

	rate := vegeta.Rate{Freq: 100, Per: time.Second}
	duration := 4 * time.Second
	targeter := vegeta.NewStaticTargeter(vegeta.Target{
		Method: "GET",
		URL:    url,
	})
	attacker := vegeta.NewAttacker()

	var metrics vegeta.Metrics
	for res := range attacker.Attack(targeter, rate, duration, "Big Bang!") {
		metrics.Add(res)
	}
	metrics.Close()

	//this is too verbose, anyway mantaining for now until we add more
	//assertions
	spew.Dump(metrics)

	// Success is the percentage of non-error responses.
	assert.Assert(t, metrics.Success > 0.95, "too many errors! see the log for details.")

	fmt.Printf("Success!!\n")
}

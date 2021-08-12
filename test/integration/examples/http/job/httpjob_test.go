//+build job

package job

import (
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/tsenart/vegeta/v12/lib"
	"golang.org/x/net/http2"
	"gotest.tools/assert"
)

func TestHttpJob(t *testing.T) {
	testHttpJob(t, "http://nginx1:8080/")
}

func TestHttp2Job(t *testing.T) {
	//TODO: enable this if we add suport for "Upgrade" in skupper http2
	//testHttpJob(t, "http://nghttp2:8443/")

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
	assert.Assert(t, metrics.Success > 0.98, "too many errors! see the log for details.")

	fmt.Printf("Success!!\n")
}

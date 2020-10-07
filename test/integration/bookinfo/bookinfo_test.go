// +build integration

package bookinfo

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/skupperproject/skupper/test/utils/base"
	"github.com/skupperproject/skupper/test/utils/k8s"
	"gotest.tools/assert"

	_ "k8s.io/client-go/plugin/pkg/client/auth"
)

func TestMain(m *testing.M) {
	base.ParseFlags()
	os.Exit(m.Run())
}

func TestBookinfo(t *testing.T) {
	needs := base.ClusterNeeds{
		NamespaceId:     "bookinfo",
		PublicClusters:  1,
		PrivateClusters: 1,
	}
	testRunner := &base.ClusterTestRunnerBase{}
	testRunner.BuildOrSkip(t, needs, nil)
	ctx, cancel := context.WithCancel(context.Background())
	base.HandleInterruptSignal(t, func(t *testing.T) {
		base.TearDownSimplePublicAndPrivate(testRunner)
		cancel()
	})
	Run(ctx, t, testRunner)
}

func tryProductPage() ([]byte, error) {
	client := http.Client{
		Timeout: 30 * time.Second,
	}

	resp, err := client.Get("http://productpage:9080/productpage?u=test")
	if err != nil {
		return nil, err
	}

	if resp.Status != "200 OK" {
		return nil, fmt.Errorf("unexpedted http response status: %v", resp.Status)
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return body, nil
}

func TestBookinfoJob(t *testing.T) {
	k8s.SkipTestJobIfMustBeSkipped(t)
	_body, err := tryProductPage()
	assert.Assert(t, err)

	body := string(_body)
	fmt.Printf("body:\n%s\n", body)
	assert.Assert(t, strings.Contains(body, "Book Details"))
	assert.Assert(t, strings.Contains(body, "An extremely entertaining play by Shakespeare. The slapstick humour is refreshing!"))
	assert.Assert(t, !strings.Contains(body, "Ratings service is currently unavailable"))
}

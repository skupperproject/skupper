// +build job

package job

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"testing"
	"time"

	"gotest.tools/assert"
)

func TestBookinfoJob(t *testing.T) {
	_body, err := tryProductPage()
	assert.Assert(t, err)

	body := string(_body)
	fmt.Printf("body:\n%s\n", body)
	assert.Assert(t, strings.Contains(body, "Book Details"))
	assert.Assert(t, strings.Contains(body, "An extremely entertaining play by Shakespeare. The slapstick humour is refreshing!"))
	assert.Assert(t, !strings.Contains(body, "Ratings service is currently unavailable"))
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


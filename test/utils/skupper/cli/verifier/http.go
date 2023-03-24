package verifier

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/client"
	"github.com/skupperproject/skupper/pkg/utils"
	"github.com/skupperproject/skupper/test/utils/tools"
)

type CurlVerifier struct {
	Url          string
	Opts         tools.CurlOpts
	Interval     time.Duration
	MaxRetries   int
	LastResponse *HttpResp
}

func (c *CurlVerifier) GetRequest(platform types.Platform, cli *client.VanClient) error {
	var resp *tools.CurlResponse
	var err error
	interval := c.Interval
	maxRetries := c.MaxRetries
	if interval == 0 {
		interval = time.Second
	}
	if maxRetries == 0 {
		maxRetries = 10
	}
	err = utils.RetryError(interval, maxRetries, func() error {
		log.Printf("Validating URL: %s from namespace: %s", c.Url, cli.Namespace)
		resp, err = tools.Curl(cli.KubeClient, cli.RestConfig, cli.Namespace, "", c.Url, c.Opts)
		return err
	})
	if err != nil {
		log.Printf("error validating url: %s", c.Url)
		if resp != nil {
			log.Printf("response: %v", resp.Output)
		}
		return err
	}
	var httpError = new(HttpResp)
	c.LastResponse = httpError
	httpError.err = err
	httpError.body = resp.Body
	httpError.code = resp.StatusCode
	if resp.Headers != nil {
		httpError.header = map[string][]string{}
		for k, v := range resp.Headers {
			httpError.header[k] = []string{v}
		}
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return httpError
	}
	return nil
}

type HttpVerifier struct {
	Url          string
	Method       string
	Header       map[string][]string
	Data         string
	User         string
	Password     string
	Ctx          context.Context
	Interval     time.Duration
	MaxRetries   int
	LastResponse *HttpResp
	ExpectedCode int
}

func (h *HttpVerifier) Request(platform types.Platform, cli *client.VanClient) error {
	var httpResp = new(HttpResp)
	var err error
	var req *http.Request
	var resp *http.Response

	ctx := h.Ctx
	if ctx == nil {
		ctx = context.Background()
	}

	httpCli := &http.Client{}
	method := utils.DefaultStr(h.Method, "GET")
	var inData io.Reader
	if h.Data != "" {
		inData = strings.NewReader(h.Data)
	}
	req, err = http.NewRequestWithContext(ctx, method, h.Url, inData)
	if err != nil {
		httpResp.err = fmt.Errorf("error preparing http request - %v", err)
		return httpResp
	}
	if h.Header != nil {
		req.Header = h.Header
	}
	httpCli.Transport = &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
	}
	if h.User != "" && h.Password != "" {
		req.SetBasicAuth(h.User, h.Password)
	}
	interval := h.Interval
	if interval == 0 {
		interval = time.Second
	}
	maxRetries := h.MaxRetries
	if maxRetries == 0 {
		maxRetries = 10
	}
	err = utils.RetryError(interval, maxRetries, func() error {
		log.Printf("Validating URL: %s", h.Url)
		resp, err = httpCli.Do(req)
		if err == nil && h.ExpectedCode > 0 && h.ExpectedCode != resp.StatusCode {
			return fmt.Errorf("invalid status code - expecting: %d - found: %d", h.ExpectedCode, resp.StatusCode)
		}
		return err
	})
	if err != nil && resp == nil {
		httpResp.err = fmt.Errorf("error making http request - %v", err)
		return httpResp
	}

	defer resp.Body.Close()
	var bodyData []byte

	// preparing http error
	h.LastResponse = httpResp
	httpResp.err = err
	httpResp.code = resp.StatusCode
	httpResp.header = resp.Header

	var readErr error
	bodyData, readErr = io.ReadAll(resp.Body)
	httpResp.body = string(bodyData)
	if readErr != nil {
		httpResp.err = fmt.Errorf("error reading response data - %v", err)
		return httpResp
	}
	if err != nil {
		return httpResp
	}

	return nil
}

type HttpResp struct {
	err    error
	body   string
	header map[string][]string
	code   int
}

func (h *HttpResp) Error() string {
	if h.err != nil {
		return h.err.Error()
	}
	return ""
}

func (h *HttpResp) Body() string {
	return h.body
}

func (h *HttpResp) Code() int {
	return h.code
}

func (h *HttpResp) Header() map[string][]string {
	return h.header
}

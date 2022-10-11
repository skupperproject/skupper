package podman

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"

	runtime2 "github.com/go-openapi/runtime"
	runtime "github.com/go-openapi/runtime/client"
	"github.com/go-openapi/strfmt"
	"github.com/skupperproject/skupper/client/generated/libpod/models"
	"github.com/skupperproject/skupper/pkg/config"
)

const (
	DEFAULT_BASE_PATH    = "/v4.0.0"
	DefaultNetworkDriver = "bridge"
)

var (
	formats = strfmt.NewFormats()
)

type PodmanRestClient struct {
	RestClient *runtime.Runtime
}

func NewPodmanClient(endpoint, basePath string) (*PodmanRestClient, error) {
	var err error
	var sockFile bool

	if endpoint == "" {
		endpoint = fmt.Sprintf("%s/podman/podman.sock", config.GetRuntimeDir())
	}

	var u = &url.URL{
		Scheme: "http",
	}
	if strings.HasPrefix(endpoint, "/") {
		u.Host = "unix"
		sockFile = true
	} else {
		host := endpoint
		match, _ := regexp.Match(`http[s]*://`, []byte(host))
		if !match {
			host = "http://" + host
		}
		u, err = url.Parse(host)
		if err != nil {
			return nil, err
		}
	}

	hostPort := u.Hostname()
	if u.Port() != "" {
		hostPort = net.JoinHostPort(u.Hostname(), u.Port())
	}
	if basePath == "" {
		basePath = DEFAULT_BASE_PATH
	}
	c := runtime.New(hostPort, basePath, []string{u.Scheme})
	if u.Scheme == "https" {
		ct := c.Transport.(*http.Transport)
		if ct.TLSClientConfig != nil {
			ct.TLSClientConfig = &tls.Config{}
		}
		ct.TLSClientConfig.InsecureSkipVerify = true
	}
	if sockFile {
		_, err := os.Stat(endpoint)
		if err != nil {
			return nil, fmt.Errorf("invalid sock file: %v", err)
		}
		ct := c.Transport.(*http.Transport)
		ct.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
			return net.Dial("unix", endpoint)
		}
	}

	cli := &PodmanRestClient{
		RestClient: c,
	}
	return cli, nil
}

// boolTrue returns a true bool pointer (for false, just use new(bool))
func boolTrue() *bool {
	b := true
	return &b
}

func stringP(val string) *string {
	return &val
}

type responseReaderID struct {
}

func (r *responseReaderID) ReadResponse(response runtime2.ClientResponse, consumer runtime2.Consumer) (interface{}, error) {
	switch response.Code() {
	case 200, 201:
		resp := &models.IDResponse{}
		if err := consumer.Consume(response.Body(), resp); err != nil {
			return nil, err
		}
		return resp, nil
	case 404:
		return nil, fmt.Errorf("not found")
	case 409:
		return nil, fmt.Errorf("conflict")
	case 500:
		return nil, fmt.Errorf("server error")
	default:
		return nil, fmt.Errorf("unexpected error")
	}
}

type responseReaderBody struct {
}

func (r *responseReaderBody) Consume(reader io.Reader, i interface{}) error {
	bodyStr, ok := i.(*string)
	if !ok {
		return fmt.Errorf("error parsing body")
	}
	data, err := ioutil.ReadAll(reader)
	if err != nil {
		return fmt.Errorf("error reading response body: %v", err)
	}
	*bodyStr = string(data)
	return nil
}

func (r *responseReaderBody) ReadResponse(response runtime2.ClientResponse, consumer runtime2.Consumer) (interface{}, error) {
	switch response.Code() {
	case 200, 201:
		bodyStr := ""
		err := r.Consume(response.Body(), &bodyStr)
		if err != nil {
			return nil, err
		}
		return bodyStr, nil
	case 404:
		return nil, fmt.Errorf("not found")
	case 409:
		return nil, fmt.Errorf("conflict")
	case 500:
		return nil, fmt.Errorf("server error")
	default:
		return nil, fmt.Errorf("unexpected error")
	}
}

func (p *PodmanRestClient) ResponseIDReader(httpClient *runtime2.ClientOperation) {
	httpClient.Reader = &responseReaderID{}
}

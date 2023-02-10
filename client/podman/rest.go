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
	"github.com/skupperproject/skupper/pkg/utils"
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

	if endpoint == "" {
		endpoint = fmt.Sprintf("unix://%s/podman/podman.sock", config.GetRuntimeDir())
	}

	var u *url.URL
	isSockFile := strings.HasPrefix(endpoint, "/")
	if isSockFile || strings.HasPrefix(endpoint, "unix://") {
		if isSockFile {
			endpoint = "unix://" + endpoint
		}
		isSockFile = true
		u, err = url.Parse(endpoint)
		if err != nil {
			return nil, err
		}
		u.Scheme = "http"
		u.Host = "unix"
	} else {
		host := endpoint
		match, _ := regexp.Match(`(http[s]*|tcp)://`, []byte(host))
		if !match {
			if !strings.Contains(host, "://") {
				host = "http://" + host
			} else {
				return nil, fmt.Errorf("invalid endpoint: %s", host)
			}
		}
		u, err = url.Parse(host)
		if err != nil {
			return nil, err
		}
		if u.Scheme == "tcp" {
			u.Scheme = "http"
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
		if ct.TLSClientConfig == nil {
			ct.TLSClientConfig = &tls.Config{}
		}
		ct.TLSClientConfig.InsecureSkipVerify = true
	} else {
		ct := c.Transport.(*http.Transport)
		ct.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
			return net.Dial("tcp", hostPort)
		}
	}
	if isSockFile {
		_, err := os.Stat(u.RequestURI())
		if err != nil {
			return nil, fmt.Errorf("Podman service is not available on provided endpoint - %w", err)
		}
		ct := c.Transport.(*http.Transport)
		ct.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
			return net.Dial("unix", u.RequestURI())
		}
	}

	cli := &PodmanRestClient{
		RestClient: c,
	}
	if err = cli.Validate(); err != nil {
		return nil, err
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

	var dataClean []byte
	for idx, v := range data {
		// skip 8 first bytes every 8k chunk
		if idx%(8192+8) < 8 {
			continue
		}
		dataClean = append(dataClean, v)
	}

	for idx, v := range dataClean {
		// bad characters ascii 0 or 1 returned sometimes, stripping them off
		if v < 2 {
			dataClean = dataClean[:idx]
			break
		}
	}

	// fmt.Println(string(dataClean[len(dataClean)-8 : len(dataClean)]))
	// fmt.Println(dataClean[len(dataClean)-8 : len(dataClean)])
	*bodyStr = string(dataClean)
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

func (p *PodmanRestClient) Validate() error {
	version, err := p.Version()
	if err != nil {
		return fmt.Errorf("Podman service is not available on provided endpoint (unable to verify version) - %w", err)
	}
	apiVersion := utils.ParseVersion(version.Server.APIVersion)
	if apiVersion.Major < 4 {
		return fmt.Errorf("podman version must be 4.0.0 or greater, found: %s", version.Server.APIVersion)
	}
	return nil
}

package podman

import (
	"context"
	"crypto/tls"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/go-openapi/runtime"
	runtimeclient "github.com/go-openapi/runtime/client"
	"github.com/go-openapi/strfmt"
	"github.com/skupperproject/skupper-libpod/v4/models"
	"github.com/skupperproject/skupper/pkg/config"
	"github.com/skupperproject/skupper/pkg/utils"
)

const (
	ENV_PODMAN_ENDPOINT  = "PODMAN_ENDPOINT"
	DEFAULT_BASE_PATH    = "/v4.0.0"
	DefaultNetworkDriver = "bridge"
)

var (
	formats        = strfmt.NewFormats()
	localAddresses = []string{"127.0.0.1", "::1", "0.0.0.0", "::"}
)

type PodmanRestClient struct {
	RestClient runtime.ClientTransport
	endpoint   string
}

type RestClientFactory func(endpoint, basePath string) (*PodmanRestClient, error)

func NewPodmanClient(endpoint, basePath string) (*PodmanRestClient, error) {
	var err error

	if endpoint == "" {
		defaultEndpoint := GetDefaultPodmanEndpoint()
		endpoint = utils.DefaultStr(os.Getenv(ENV_PODMAN_ENDPOINT), defaultEndpoint)
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
		addresses, err := net.LookupHost(u.Hostname())
		if err != nil {
			return nil, fmt.Errorf("unable to resolve hostname: %s", u.Hostname())
		}
		for _, addr := range addresses {
			if utils.StringSliceContains(localAddresses, addr) {
				return nil, fmt.Errorf("local addresses cannot be used, got: %s", u.Hostname())
			}
		}
	}
	hostPort := u.Hostname()
	if u.Port() != "" {
		hostPort = net.JoinHostPort(u.Hostname(), u.Port())
	}
	if basePath == "" {
		basePath = DEFAULT_BASE_PATH
	}
	c := runtimeclient.New(hostPort, basePath, []string{u.Scheme})
	// Initializing transport like the http.DefaultTransport
	// to avoid modifying it directly, as Runtime.Transport is
	// set to http.DefaultTransport (variable)
	dialer := &net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
	}
	c.Transport = &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		DialContext:           dialer.DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}
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
		endpoint:   endpoint,
	}
	if err = cli.Validate(); err != nil {
		return nil, err
	}
	return cli, nil
}

func GetDefaultPodmanEndpoint() string {
	return fmt.Sprintf("unix://%s/podman/podman.sock", config.GetRuntimeDir())
}

func (p *PodmanRestClient) IsSockEndpoint() bool {
	return strings.HasPrefix(p.endpoint, "/") || strings.HasPrefix(p.endpoint, "unix://")
}

func (p *PodmanRestClient) GetEndpoint() string {
	return p.endpoint
}

func (p *PodmanRestClient) IsRunningInContainer() bool {
	// See: https://docs.podman.io/en/latest/markdown/podman-run.1.html
	_, err := os.Stat("/run/.containerenv")
	return err == nil
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

func (r *responseReaderID) ReadResponse(response runtime.ClientResponse, consumer runtime.Consumer) (interface{}, error) {
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

type multiplexedBodyReader struct {
	output map[byte][]byte
}

func (r *multiplexedBodyReader) Stdout() string {
	if r.output == nil {
		return ""
	}
	return string(r.output[1])
}

func (r *multiplexedBodyReader) Stderr() string {
	if r.output == nil {
		return ""
	}
	return string(r.output[2])
}

func (r *multiplexedBodyReader) ReadResponse(response runtime.ClientResponse, consumer runtime.Consumer) (interface{}, error) {
	return ReadResponse(response, r)
}

func (r *multiplexedBodyReader) readStreams(offset *uint64, data []byte) {
	if *offset == 0 {
		r.output = map[byte][]byte{0: []byte{}, 1: []byte{}, 2: []byte{}}
	}
	if *offset == uint64(len(data)) {
		return
	}
	header := data[*offset : *offset+8]
	// stream can be: 0 = stdin, 1 = stdout, 2 = stderr
	streamId := header[0]
	// frame size to read
	size := binary.BigEndian.Uint32(header[4:])
	// skip to start of frame
	*offset += 8
	// frame content
	frame := data[*offset : *offset+uint64(size)]
	r.output[streamId] = append(r.output[streamId], frame...)
	*offset += uint64(size)
	r.readStreams(offset, data)
}

func (r *multiplexedBodyReader) Consume(reader io.Reader, i interface{}) error {
	consumeErr := runtime.TextConsumer().Consume(reader, i)
	bodyStr, ok := i.(*string)
	if !ok {
		return fmt.Errorf("error parsing body")
	}
	body := *bodyStr

	//  Source: https://github.com/docker/engine/blob/8955d8da8951695a98eb7e15bead19d402c6eb27/api/swagger.yaml#L6818C4-L6818C4
	//	### Stream format
	//
	//	When the TTY setting is disabled in [`POST /containers/create`](#operation/ContainerCreate),
	//	the stream over the hijacked connected is multiplexed to separate out
	//	`stdout` and `stderr`. The stream consists of a series of frames, each
	//	containing a header and a payload.
	//
	//		The header contains the information which the stream writes (`stdout` or
	//	`stderr`). It also contains the size of the associated frame encoded in
	//	the last four bytes (`uint32`).
	//
	//		It is encoded on the first eight bytes like this:
	//
	//	```go
	//        header := [8]byte{STREAM_TYPE, 0, 0, 0, SIZE1, SIZE2, SIZE3, SIZE4}
	//        ```
	//
	//	`STREAM_TYPE` can be:
	//
	//	- 0: `stdin` (is written on `stdout`)
	//	- 1: `stdout`
	//	- 2: `stderr`
	//
	//	`SIZE1, SIZE2, SIZE3, SIZE4` are the four bytes of the `uint32` size
	//	encoded as big endian.
	//
	//		Following the header is the payload, which is the specified number of
	//	bytes of `STREAM_TYPE`.
	//
	//		The simplest way to implement this protocol is the following:
	//
	//	1. Read 8 bytes.
	//	2. Choose `stdout` or `stderr` depending on the first byte.
	//	3. Extract the frame size from the last four bytes.
	//	4. Read the extracted size and output it on the correct output.
	//	5. Goto 1.
	offset := uint64(0)
	r.readStreams(&offset, []byte(body))

	// remove next 8 bytes if control character found (0 or 1)
	*bodyStr = string(r.output[1]) + string(r.output[2])
	return consumeErr
}

type responseReaderOctetStreamBody struct {
}

func (r *responseReaderOctetStreamBody) ReadResponse(response runtime.ClientResponse, consumer runtime.Consumer) (interface{}, error) {
	return ReadResponse(response, r)
}

func (r *responseReaderOctetStreamBody) Consume(reader io.Reader, i interface{}) error {
	bodyStr, ok := i.(*string)
	if !ok {
		return fmt.Errorf("error parsing body")
	}
	data, err := io.ReadAll(reader)
	if err != nil {
		return fmt.Errorf("error reading response body: %v", err)
	}

	var dataClean []byte
	idx := 0
	for _, v := range data {
		// skip 8 first bytes on each line
		if idx < 8 {
			idx++
			continue
		} else if v == '\n' {
			idx = 0
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

	*bodyStr = string(dataClean)
	return nil
}

type responseReaderJSONErrorBody struct {
}

func (r *responseReaderJSONErrorBody) ReadResponse(response runtime.ClientResponse, consumer runtime.Consumer) (interface{}, error) {
	return ReadResponse(response, r)
}

func (r *responseReaderJSONErrorBody) Consume(reader io.Reader, i interface{}) error {
	bodyStr, ok := i.(*string)
	if !ok {
		return fmt.Errorf("error parsing body")
	}
	data, err := io.ReadAll(reader)
	if err != nil {
		return fmt.Errorf("error reading response body: %v", err)
	}

	// identify error messages in multiline response
	errorRegexp := regexp.MustCompile(`(?s).*({"error":\s*"[^"]+"}).*(?s)`)
	*bodyStr = errorRegexp.ReplaceAllString(string(data), `$1`)
	return nil
}

func ReadResponse(response runtime.ClientResponse, consumer runtime.Consumer) (interface{}, error) {
	switch response.Code() {
	case 200, 201:
		var err error
		bodyStr := ""
		err = consumer.Consume(response.Body(), &bodyStr)
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

func (p *PodmanRestClient) ResponseIDReader(httpClient *runtime.ClientOperation) {
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

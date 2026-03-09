package compat

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/go-openapi/runtime"
	runtimeclient "github.com/go-openapi/runtime/client"
	"github.com/go-openapi/strfmt"
	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/internal/utils"
	"github.com/skupperproject/skupper/pkg/nonkube/api"
)

const (
	EnvContainerEndpoint = "CONTAINER_ENDPOINT"
	DefaultBasePath      = ""
	DefaultNetworkDriver = "bridge"
)

var (
	formats        = strfmt.NewFormats()
	localAddresses = []string{"127.0.0.1", "::1", "0.0.0.0", "::"}
)

type ContainerEngine string
type IdGetter func() int

var getUid IdGetter = os.Getuid

var (
	ContainerEnginePodman ContainerEngine = "podman"
	ContainerEngineDocker ContainerEngine = "docker"
)

type CompatClient struct {
	RestClient runtime.ClientTransport
	engine     ContainerEngine
	endpoint   string
}

func NewCompatClient(endpoint, basePath string) (*CompatClient, error) {
	var err error

	if endpoint == "" {
		defaultEndpoint := GetDefaultContainerEndpoint()
		endpoint = utils.DefaultStr(os.Getenv(EnvContainerEndpoint), defaultEndpoint)
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
			if slices.Contains(localAddresses, addr) {
				return nil, fmt.Errorf("local addresses cannot be used, got: %s", u.Hostname())
			}
		}
	}
	hostPort := u.Hostname()
	if u.Port() != "" {
		hostPort = net.JoinHostPort(u.Hostname(), u.Port())
	}
	if basePath == "" {
		basePath = DefaultBasePath
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
			return nil, fmt.Errorf("Container engine is not available on provided endpoint - %w", err)
		}
		ct := c.Transport.(*http.Transport)
		ct.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
			return net.Dial("unix", u.RequestURI())
		}
	}

	cli := &CompatClient{
		RestClient: c,
		endpoint:   endpoint,
	}
	if version, err := cli.Validate(); err != nil {
		return nil, err
	} else {
		cli.engine = ContainerEngine(version.Engine)
	}
	return cli, nil
}

func GetDefaultContainerEndpoint() string {
	platform := os.Getenv(types.ENV_PLATFORM)
	// the container endpoint is mapped to the podman socket inside the container
	if api.IsRunningInContainer() {
		if platform == "docker" {
			return "unix:///var/run/docker.sock"
		}
		return "unix:///var/run/podman.sock"
	} else {
		if platform == "docker" {
			return "unix:///run/docker.sock"
		}

		return fmt.Sprintf("unix://%s/podman/podman.sock", GetRuntimeDir())
	}
}

func GetRuntimeDir() string {
	if getUid() == 0 {
		return "/run"
	}
	runtimeDir, ok := os.LookupEnv("XDG_RUNTIME_DIR")
	if !ok {
		runtimeDir = fmt.Sprintf("/run/user/%d", getUid())
	}
	return runtimeDir
}

func (c *CompatClient) IsSockEndpoint() bool {
	return strings.HasPrefix(c.endpoint, "/") || strings.HasPrefix(c.endpoint, "unix://")
}

func (c *CompatClient) GetEndpoint() string {
	return c.endpoint
}

func asStringInterfaceMap(i interface{}) map[string]interface{} {
	res := make(map[string]interface{})
	v := reflect.ValueOf(i)
	if v.Kind() == reflect.Map {
		for _, k := range v.MapKeys() {
			res[k.String()] = v.MapIndex(k).Interface()
		}
		return res
	}
	return res
}

func asInterfaceSlice(i interface{}) []interface{} {
	v := reflect.ValueOf(i)
	if v.Kind() == reflect.Slice {
		return v.Interface().([]interface{})
	}
	return make([]interface{}, 0)
}

func asSlice[T any](i interface{}) []T {
	v := reflect.ValueOf(i)
	s := make([]T, 0)
	if v.Kind() == reflect.Slice {
		for _, vv := range v.Interface().([]interface{}) {
			s = append(s, vv.(T))
		}
	}
	return s
}

func jsonNumberAsInt(i interface{}) int64 {
	if n, ok := i.(json.Number); ok {
		ni64, _ := n.Int64()
		return ni64
	}
	return 0
}

func asStringStringMap(i interface{}) map[string]string {
	m := asStringInterfaceMap(i)
	result := make(map[string]string)
	for k, v := range m {
		result[k] = v.(string)
	}
	return result
}

type APIError struct {
	// API root cause formatted for automated parsing
	// Example: API root cause
	Because string `json:"cause,omitempty"`

	// human error message, formatted for a human to read
	// Example: human error message
	Message string `json:"message,omitempty"`

	// http response code
	ResponseCode int64 `json:"response,omitempty"`
}

func (a *APIError) Error() string {
	return a.Message
}

func ToAPIError(err interface{}) *APIError {
	if err == nil {
		return nil
	}
	apiError := new(APIError)
	genericApiError := map[string]interface{}{}
	resultJson, _ := json.Marshal(err)
	_ = json.Unmarshal(resultJson, &genericApiError)
	payload, payloadOk := genericApiError["Payload"]
	if payloadOk {
		payloadJson, _ := json.Marshal(payload)
		if jsonErr := json.Unmarshal(payloadJson, apiError); jsonErr != nil {
			apiError.Message = fmt.Sprintf("unable to parse compat API error: %s", string(resultJson))
		}
		return apiError
	}
	if asError, ok := err.(error); ok {
		apiError.Message = asError.Error()
	} else {
		apiError.Message = fmt.Sprintf("unable to parse error: %v", err)
	}
	return apiError
}

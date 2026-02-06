package main

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/url"
	"os"
	"time"

	"github.com/skupperproject/skupper/internal/utils/tlscfg"
)

type Config struct {
	APIListenAddress    string
	APIEnableAccessLogs bool
	APITLS              TLSSpec

	EnableConsole   bool
	ConsoleLocation string
	PrometheusAPI   string

	RouterURL     string
	RouterTLS     TLSSpec
	FlowRecordTTL time.Duration

	VanflowLoggingProfile string

	EnableProfile bool
	CORSAllowAll  bool

	MetricsListenAddress string
}

type TLSSpec struct {
	CA         string
	Cert       string
	Key        string
	SkipVerify bool
}

func (t TLSSpec) hasCert() bool {
	return len(t.Cert) > 0
}

func (t TLSSpec) config() (*tls.Config, error) {
	config := tlscfg.Modern()

	config.InsecureSkipVerify = t.SkipVerify

	if len(t.CA) > 0 && !t.SkipVerify {
		certPool := x509.NewCertPool()
		file, err := os.ReadFile(t.CA)
		if err != nil {
			return nil, err
		}
		if ok := certPool.AppendCertsFromPEM(file); !ok {
			return nil, fmt.Errorf("failed to add CA to certificate pool")
		}
		config.RootCAs = certPool
	}

	if t.hasCert() {
		tlsCert, err := tls.LoadX509KeyPair(t.Cert, t.Key)
		if err != nil {
			return nil, err
		}
		config.Certificates = []tls.Certificate{tlsCert}
	}

	return config, nil
}

func parsePrometheusAPI(base string) (*url.URL, error) {
	targetPromAPI, err := url.Parse(base)
	if err != nil {
		return nil, err
	}
	if targetPromAPI.Path == "" {
		targetPromAPI.Path = "/"
	}
	targetPromAPI = targetPromAPI.JoinPath("/api/v1/")
	return targetPromAPI, nil
}

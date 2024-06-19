package main

import (
	"encoding/json"
	"os"
	"time"
)

type Config struct {
	APIListenAddress     string
	APIDisableAccessLogs bool
	TLSCert              string
	TLSKey               string

	EnableConsole   bool
	ConsoleLocation string
	PrometheusAPI   string

	FlowConnectionFile string
	FlowRecordTTL      time.Duration

	EnableProfile bool
	CORSAllowAll  bool
}

type TLSSpec struct {
	CA     string `json:"ca,omitempty"`
	Cert   string `json:"cert,omitempty"`
	Key    string `json:"key,omitempty"`
	Verify bool   `json:"verify,omitempty"`
}

type ConnectionSpec struct {
	Scheme string  `json:"scheme,omitempty"`
	Host   string  `json:"host,omitempty"`
	Port   string  `json:"port,omitempty"`
	TLS    TLSSpec `json:"tls,omitempty"`
}

func getConnectInfo(file string) (ConnectionSpec, error) {
	var spec ConnectionSpec
	f, err := os.Open(file)
	if err != nil {
		return spec, err
	}
	defer f.Close()
	err = json.NewDecoder(f).Decode(&spec)
	return spec, err
}

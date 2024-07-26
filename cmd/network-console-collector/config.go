package main

import (
	"time"
)

type Config struct {
	APIListenAddress     string
	APIDisableAccessLogs bool
	APITLS               TLSSpec

	EnableConsole   bool
	ConsoleLocation string
	PrometheusAPI   string

	RouterURL     string
	RouterTLS     TLSSpec
	FlowRecordTTL time.Duration

	EnableProfile bool
	CORSAllowAll  bool
}

type TLSSpec struct {
	CA     string
	Cert   string
	Key    string
	Verify bool
}

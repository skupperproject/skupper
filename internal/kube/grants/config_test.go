package grants

import (
	"flag"
	"os"
	"testing"

	"gotest.tools/v3/assert"
)

func Test_BoundGrantConfig(t *testing.T) {
	tests := []struct {
		name           string
		args           []string
		env            map[string]string
		expectedValue  *GrantConfig
		expectedErrors []string
	}{
		{
			name: "defaults",
			expectedValue: &GrantConfig{
				Port:                 9090,
				TlsCredentialsSecret: "skupper-grant-server",
				Hostname:             os.Getenv("HOSTNAME"),
			},
		},
		{
			name: "env vars",
			env: map[string]string{
				"SKUPPER_ENABLE_GRANTS":                "true",
				"SKUPPER_GRANT_SERVER_AUTOCONFIGURE":   "true",
				"SKUPPER_GRANT_SERVER_BASE_URL":        "https://acme.org:8888/grants",
				"SKUPPER_GRANT_SERVER_PORT":            "1234",
				"SKUPPER_GRANT_SERVER_TLS_CREDENTIALS": "my-secret",
				"HOSTNAME":                             "my-host",
			},
			expectedValue: &GrantConfig{
				Enabled:              true,
				AutoConfigure:        true,
				BaseUrl:              "https://acme.org:8888/grants",
				Port:                 1234,
				TlsCredentialsSecret: "my-secret",
				Hostname:             "my-host",
			},
		},
		{
			name: "args",
			env: map[string]string{
				"SKUPPER_ENABLE_GRANTS":                "true",
				"SKUPPER_GRANT_SERVER_AUTOCONFIGURE":   "true",
				"SKUPPER_GRANT_SERVER_BASE_URL":        "https://acme.org:8888/grants",
				"SKUPPER_GRANT_SERVER_PORT":            "1234",
				"SKUPPER_GRANT_SERVER_TLS_CREDENTIALS": "my-secret",
				"HOSTNAME":                             "my-host",
			},
			args: []string{
				"--enable-grants=false",
				"--grant-server-autoconfigure=false",
				"--grant-server-base-url=https://anotherhost:8080/blah",
				"--grant-server-port=9876",
				"--grant-server-tls-credentials=a-different-secret",
				"--grant-server-podname=a-different-host",
			},
			expectedValue: &GrantConfig{
				Enabled:              false,
				AutoConfigure:        false,
				BaseUrl:              "https://anotherhost:8080/blah",
				Port:                 9876,
				TlsCredentialsSecret: "a-different-secret",
				Hostname:             "a-different-host",
			},
		},
		{
			name: "args override env vars",
			args: []string{
				"--enable-grants",
				"--grant-server-autoconfigure",
				"--grant-server-base-url=https://acme.org:8888/grants",
				"--grant-server-port=1234",
				"--grant-server-tls-credentials=my-secret",
				"--grant-server-podname=my-host",
			},
			expectedValue: &GrantConfig{
				Enabled:              true,
				AutoConfigure:        true,
				BaseUrl:              "https://acme.org:8888/grants",
				Port:                 1234,
				TlsCredentialsSecret: "my-secret",
				Hostname:             "my-host",
			},
		},
		{
			name: "invalid env var for enabled",
			env: map[string]string{
				"SKUPPER_ENABLE_GRANTS":                "this is not a bool",
				"SKUPPER_GRANT_SERVER_AUTOCONFIGURE":   "true",
				"SKUPPER_GRANT_SERVER_BASE_URL":        "https://acme.org:8888/grants",
				"SKUPPER_GRANT_SERVER_PORT":            "1234",
				"SKUPPER_GRANT_SERVER_TLS_CREDENTIALS": "my-secret",
				"HOSTNAME":                             "my-host",
			},
			expectedErrors: []string{
				"Invalid environment variable(s)",
				"SKUPPER_ENABLE_GRANTS",
				"this is not a bool",
			},
			expectedValue: &GrantConfig{
				Enabled:              false,
				AutoConfigure:        true,
				BaseUrl:              "https://acme.org:8888/grants",
				Port:                 1234,
				TlsCredentialsSecret: "my-secret",
				Hostname:             "my-host",
			},
		},
		{
			name: "invalid env var for autoconfigure",
			env: map[string]string{
				"SKUPPER_ENABLE_GRANTS":                "true",
				"SKUPPER_GRANT_SERVER_AUTOCONFIGURE":   "this is not a bool",
				"SKUPPER_GRANT_SERVER_BASE_URL":        "https://acme.org:8888/grants",
				"SKUPPER_GRANT_SERVER_PORT":            "1234",
				"SKUPPER_GRANT_SERVER_TLS_CREDENTIALS": "my-secret",
				"HOSTNAME":                             "my-host",
			},
			expectedErrors: []string{
				"Invalid environment variable(s)",
				"SKUPPER_GRANT_SERVER_AUTOCONFIGURE",
				"this is not a bool",
			},
			expectedValue: &GrantConfig{
				Enabled:              true,
				AutoConfigure:        false,
				BaseUrl:              "https://acme.org:8888/grants",
				Port:                 1234,
				TlsCredentialsSecret: "my-secret",
				Hostname:             "my-host",
			},
		},
		{
			name: "invalid env var for port",
			env: map[string]string{
				"SKUPPER_ENABLE_GRANTS":                "true",
				"SKUPPER_GRANT_SERVER_AUTOCONFIGURE":   "true",
				"SKUPPER_GRANT_SERVER_BASE_URL":        "https://acme.org:8888/grants",
				"SKUPPER_GRANT_SERVER_PORT":            "foo/bar/baz",
				"SKUPPER_GRANT_SERVER_TLS_CREDENTIALS": "my-secret",
				"HOSTNAME":                             "my-host",
			},
			expectedErrors: []string{
				"Invalid environment variable(s)",
				"SKUPPER_GRANT_SERVER_PORT",
				"foo/bar/baz",
			},
			expectedValue: &GrantConfig{
				Enabled:              true,
				AutoConfigure:        true,
				BaseUrl:              "https://acme.org:8888/grants",
				Port:                 9090,
				TlsCredentialsSecret: "my-secret",
				Hostname:             "my-host",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for k, v := range tt.env {
				t.Setenv(k, v)
			}
			flags := &flag.FlagSet{}
			config, err := BoundGrantConfig(flags)
			flags.Parse(tt.args)
			if len(tt.expectedErrors) > 0 {
				for _, text := range tt.expectedErrors {
					assert.ErrorContains(t, err, text)
				}
			} else if err != nil {
				t.Error(err)
			}
			assert.DeepEqual(t, config, tt.expectedValue)
		})
	}
}

func Test_GrantConfigMethods(t *testing.T) {
	tests := []struct {
		name           string
		input          *GrantConfig
		expectedAddr   string
		expectedScheme string
	}{
		{
			name: "case 1",
			input: &GrantConfig{
				Port: 1234,
			},
			expectedAddr:   ":1234",
			expectedScheme: "http",
		},
		{
			name: "case 2",
			input: &GrantConfig{
				Port:                 9090,
				TlsCredentialsSecret: "foo",
			},
			expectedAddr:   ":9090",
			expectedScheme: "https",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.input.addr(), tt.expectedAddr)
			assert.Equal(t, tt.input.scheme(), tt.expectedScheme)
		})
	}
}

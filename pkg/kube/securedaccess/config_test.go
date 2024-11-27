package securedaccess

import (
	"flag"
	"testing"

	"gotest.tools/v3/assert"
)

func Test_BoundConfig(t *testing.T) {
	tests := []struct {
		name           string
		args           []string
		env            map[string]string
		expectedValue  *Config
		expectedErrors []string
	}{
		{
			name: "defaults",
			expectedValue: &Config{
				EnabledAccessTypes: []string{
					"local",
					"loadbalancer",
					"route",
				},
				GatewayPort: 8443,
			},
		},
		{
			name: "env vars",
			env: map[string]string{
				"SKUPPER_DEFAULT_ACCESS_TYPE":  "nodeport",
				"SKUPPER_CLUSTER_HOST":         "mycluster.org",
				"SKUPPER_ENABLED_ACCESS_TYPES": "nodeport,ingress-nginx",
				"SKUPPER_INGRESS_DOMAIN":       "gateway.ingress.com",
				"SKUPPER_HTTP_PROXY_DOMAIN":    "gateway.contour.com",
			},
			expectedValue: &Config{
				EnabledAccessTypes: []string{
					"nodeport",
					"ingress-nginx",
				},
				DefaultAccessType: "nodeport",
				ClusterHost:       "mycluster.org",
				IngressDomain:     "gateway.ingress.com",
				HttpProxyDomain:   "gateway.contour.com",
				GatewayPort:       8443,
			},
		},
		{
			name: "args",
			args: []string{
				"--enabled-access-types=ingress-nginx,nodeport",
				"--default-access-type=ingress-nginx",
				"--cluster-host=foo.bar.com",
				"--ingress-domain=baz.com",
				"--http-proxy-domain=bif.baf.bof.com",
			},
			expectedValue: &Config{
				EnabledAccessTypes: []string{
					"ingress-nginx",
					"nodeport",
				},
				DefaultAccessType: "ingress-nginx",
				ClusterHost:       "foo.bar.com",
				IngressDomain:     "baz.com",
				HttpProxyDomain:   "bif.baf.bof.com",
				GatewayPort:       8443,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for k, v := range tt.env {
				t.Setenv(k, v)
			}
			flags := &flag.FlagSet{}
			config, err := BoundConfig(flags)
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

func Test_Verify(t *testing.T) {
	tests := []struct {
		name          string
		config        *Config
		expectedError string
	}{
		{
			name: "defaults",
			config: &Config{
				EnabledAccessTypes: []string{
					"local",
					"loadbalancer",
					"route",
				},
			},
		},
		{
			name: "default not enabled",
			config: &Config{
				EnabledAccessTypes: []string{
					"local",
					"loadbalancer",
					"route",
				},
				DefaultAccessType: "ingress-nginx",
			},
			expectedError: "Default access type \"ingress-nginx\" is not in enabled list.",
		},
		{
			name: "nodeport not configured",
			config: &Config{
				EnabledAccessTypes: []string{
					"nodeport",
					"loadbalancer",
					"route",
				},
				DefaultAccessType: "nodeport",
			},
			expectedError: "Cluster host must be set to enable nodeport access type.",
		},
		{
			name: "nodeport is configured",
			config: &Config{
				EnabledAccessTypes: []string{
					"nodeport",
					"loadbalancer",
					"route",
				},
				DefaultAccessType: "nodeport",
				ClusterHost:       "myhost",
			},
		},
		{
			name: "gateway class not configured",
			config: &Config{
				EnabledAccessTypes: []string{
					"gateway",
				},
			},
			expectedError: "Gateway class must be set to enable gateway access type.",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Verify()
			if tt.expectedError != "" {
				assert.ErrorContains(t, err, tt.expectedError)
			} else if err != nil {
				t.Error(err)
			}
		})
	}
}

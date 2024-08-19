package grants

import (
	"flag"
	"fmt"
	"strings"

	iflag "github.com/skupperproject/skupper/internal/flag"
)

type GrantConfig struct {
	Enabled              bool
	AutoConfigure        bool
	BaseUrl              string
	Port                 int
	TlsCredentialsSecret string
	Hostname             string
}

func BoundGrantConfig(flags *flag.FlagSet) (*GrantConfig, error) {
	c := &GrantConfig{}
	var errors []string
	if err := iflag.BoolVar(flags, &c.Enabled, "enable-grants", "SKUPPER_ENABLE_GRANTS", false, "Enable use of AccessGrants."); err != nil {
		errors = append(errors, err.Error())
	}
	if err := iflag.BoolVar(flags, &c.AutoConfigure, "grant-server-autoconfigure", "SKUPPER_GRANT_SERVER_AUTOCONFIGURE", false, "Automatically configure the URL and TLS credentials for the AccessGrant Server."); err != nil {
		errors = append(errors, err.Error())
	}
	iflag.StringVar(flags, &c.BaseUrl, "grant-server-base-url", "SKUPPER_GRANT_SERVER_BASE_URL", "", "The base url through which the AccessGrant server can be reached.")
	if err := iflag.IntVar(flags, &c.Port, "grant-server-port", "SKUPPER_GRANT_SERVER_PORT", 9090, "The port on which the AccessGrant server should listen."); err != nil {
		errors = append(errors, err.Error())
	}
	iflag.StringVar(flags, &c.TlsCredentialsSecret, "grant-server-tls-credentials", "SKUPPER_GRANT_SERVER_TLS_CREDENTIALS", "skupper-grant-server", "The name of a secret in which TLS credentials for the AccessGrant server are found.")
	iflag.StringVar(flags, &c.Hostname, "grant-server-podname", "HOSTNAME", "", "The name of the pod in which the AccessGrant server is running (defaults to $HOSTNAME).")
	if len(errors) > 0 {
		return c, fmt.Errorf("Invalid environment variable(s): %s", strings.Join(errors, ", "))
	}
	return c, nil
}

func (c *GrantConfig) addr() string {
	return fmt.Sprintf(":%d", c.Port)
}

func (c *GrantConfig) scheme() string {
	if c.tlsEnabled() {
		return "https"
	} else {
		return "http"
	}
}

func (c *GrantConfig) tlsEnabled() bool {
	return c.TlsCredentialsSecret != ""
}

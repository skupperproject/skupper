package claims

import (
	"os"
	"path"
	"strconv"
)

type GrantConfig struct {
	Enabled              bool
	BaseUrl              string
	SecuredAccessKey     string
	Addr                 string
	TlsCredentialsPath   string
	TlsCredentialsSecret string
	CertPath             string
	KeyPath              string
	CaPath               string
}

func GrantConfigFromEnv() *GrantConfig {
	c := &GrantConfig{
		Enabled:              boolEnvVar("SKUPPER_ENABLE_GRANTS"),
		BaseUrl:              os.Getenv("SKUPPER_CLAIMS_BASE_URL"),
		SecuredAccessKey:     os.Getenv("SKUPPER_CLAIMS_GET_BASE_URL_FROM"),
		TlsCredentialsPath:   os.Getenv("SKUPPER_CLAIMS_TLS_CREDENTIALS_PATH"),
		TlsCredentialsSecret: os.Getenv("SKUPPER_CLAIMS_TLS_CREDENTIALS_SECRET"),
	}

	port := os.Getenv("SKUPPER_CLAIMS_PORT")
	if port == "" {
		port = "9090"
	}
	c.Addr = ":" + port

	if c.TlsCredentialsSecret != "" && c.TlsCredentialsPath == "" {
		c.TlsCredentialsPath = "/etc/controller/grant-server"
	}
	if c.TlsCredentialsPath != "" {
		c.CertPath = path.Join(c.TlsCredentialsPath, "tls.crt")
		c.KeyPath =  path.Join(c.TlsCredentialsPath, "tls.key")
		c.CaPath = path.Join(c.TlsCredentialsPath, "ca.crt")
	}

	return c
}

func (c *GrantConfig) scheme() string {
	if c.TlsCredentialsSecret != "" {
		return "https"
	} else {
		return "http"
	}
}

func boolEnvVar(name string) bool {
	value, _ := strconv.ParseBool(os.Getenv(name))
	return value
}


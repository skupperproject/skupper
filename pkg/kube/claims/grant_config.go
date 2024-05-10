package claims

import (
	"log"
	"os"
	"path"
	"strconv"
)

type GrantConfig struct {
	Enabled          bool
	BaseUrl          string
	SecuredAccessKey string
	Addr             string
	CertPath         string
	KeyPath          string
	CaCert           string
}

func GrantConfigFromEnv() *GrantConfig {
	c := &GrantConfig{
		Enabled:          boolEnvVar("SKUPPER_ENABLE_GRANTS"),
		BaseUrl:          os.Getenv("SKUPPER_CLAIMS_BASE_URL"),
		SecuredAccessKey: os.Getenv("SKUPPER_CLAIMS_GET_BASE_URL_FROM"),
	}

	port := os.Getenv("SKUPPER_CLAIMS_PORT")
	if port == "" {
		port = "9090"
	}
	c.Addr = ":" + port

	dir := os.Getenv("SKUPPER_CLAIMS_TLS_CREDENTIALS")
	if dir != "" {
		dir = "/etc/controller/claims"
		c.CertPath = path.Join(dir, "tls.crt")
		c.KeyPath =  path.Join(dir, "tls.key")

		ca := path.Join(dir, "ca.crt")
		content, err := os.ReadFile(ca)
		if err != nil {
			log.Printf("Could not load CA from %s: %s", err)
		} else {
			c.CaCert = string(content)
		}
	}

	return c
}

func boolEnvVar(name string) bool {
	value, _ := strconv.ParseBool(os.Getenv(name))
	return value
}


package config

import (
	"bytes"
	_ "embed"
	"text/template"

	"golang.org/x/crypto/bcrypt"
)

var (
	//go:embed prometheus.yml.template
	PrometheusConfig string

	//go:embed prometheus-web-config.yml.template
	WebConfigForPrometheus string
)

type PrometheusInfo struct {
	BasicAuth   bool
	TlsAuth     bool
	Scheme      string
	ServiceName string
	Namespace   string
	Port        string
	User        string
	Password    string
	Hash        string
}

func ScrapeConfigForPrometheus(info PrometheusInfo) string {
	var buf bytes.Buffer
	promConfig := template.Must(template.New("promConfig").Parse(PrometheusConfig))
	promConfig.Execute(&buf, info)

	return buf.String()
}

func ScrapeWebConfigForPrometheus(info PrometheusInfo) string {
	var buf bytes.Buffer
	promConfig := template.Must(template.New("prmConfig").Parse(WebConfigForPrometheus))
	promConfig.Execute(&buf, info)

	return buf.String()
}

func HashPrometheusPassword(password string) ([]byte, error) {
	return bcrypt.GenerateFromPassword([]byte(password), 14)
}

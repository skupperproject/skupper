package configs

import (
	"bytes"
	"text/template"

	"github.com/skupperproject/skupper/api/types"
)

func ConnectJson(host string) string {
	connect_json := `
{
    "scheme": "amqps",
    "host": "` + host + `",
    "port": "5671",
    "tls": {
        "ca": "/etc/messaging/ca.crt",
        "cert": "/etc/messaging/tls.crt",
        "key": "/etc/messaging/tls.key",
        "verify": true
    }
}
`
	return connect_json
}

func ConnectorConfig(connector *types.Connector) string {
	config := `

sslProfile {
    name: {{.Name}}-profile
    certFile: /etc/skupper-router-certs/{{.Name}}/tls.crt
    privateKeyFile: /etc/skupper-router-certs/{{.Name}}/tls.key
    caCertFile: /etc/skupper-router-certs/{{.Name}}/ca.crt
}

connector {
    name: {{.Name}}-connector
    host: {{.Host}}
    port: {{.Port}}
    role: {{.Role}}
    cost: {{.Cost}}
    sslProfile: {{.Name}}-profile
}

`
	var buff bytes.Buffer
	connectorconfig := template.Must(template.New("connectorconfig").Parse(config))
	connectorconfig.Execute(&buff, connector)
	return buff.String()
}

func QdrouterdConfig(assembly *types.AssemblySpec) string {
	config := `
router {
    mode: {{.Mode}}
    id: {{.Name}}-${HOSTNAME}
    metadata: ${SKUPPER_SITE_ID}
}
{{range .Listeners}}

listener {
    {{- if .Name}}
    name: {{.Name}}
    {{- end}}
    {{- if .Host}}
    host: {{.Host}}
    {{- end}}
    {{- if .Port}}
    port: {{.Port}}
    {{- end}}
    {{- if .RouteContainer}}
    role: route-container
    {{- else }}
    role: normal
    {{- end}}
    {{- if .Http}}
    http: true
    {{- end}}
    {{- if .AuthenticatePeer}}
    authenticatePeer: true
    {{- end}}
    {{- if .SaslMechanisms}}
    saslMechanisms: {{.SaslMechanisms}}
    {{- end}}
    {{- if .SslProfile}}
    sslProfile: {{.SslProfile}}
    {{- end}}
}

{{- end}}

listener {
    port: 9090
    role: normal
    http: true
    httpRootDir: disabled
    websockets: false
    healthz: true
    metrics: true
}
{{range .InterRouterListeners}}
listener {
    {{- if .Name}}
    name: {{.Name}}
    {{- end}}
    role: inter-router
    {{- if .Host}}
    host: {{.Host}}
    {{- end}}
    {{- if .Port}}
    port: {{.Port}}
    {{- end}}
    {{- if .Cost}}
    cost: {{.Cost}}
    {{- end}}
    {{- if .SaslMechanisms}}
    saslMechanisms: {{.SaslMechanisms}}
    {{- end}}
    {{- if .AuthenticatePeer}}
    authenticatePeer: true
    {{- end}}
    {{- if .SslProfile}}
    sslProfile: {{.SslProfile}}
    {{- end}}
}
{{- end}}
{{range .EdgeListeners}}
listener {
    {{- if .Name}}
    name: {{.Name}}
    {{- end}}
    role: edge
    {{- if .Host}}
    host: {{.Host}}
    {{- end}}
    {{- if .Port}}
    port: {{.Port}}
    {{- end}}
    {{- if .Cost}}
    cost: {{.Cost}}
    {{- end}}
    {{- if .SaslMechanisms}}
    saslMechanisms: {{.SaslMechanisms}}
    {{- end}}
    {{- if .AuthenticatePeer}}
    authenticatePeer: true
    {{- end}}
    {{- if .SslProfile}}
    sslProfile: {{.SslProfile}}
    {{- end}}
}
{{- end}}
{{range .SslProfiles}}

sslProfile {
   name: {{.Name}}
   certFile: /etc/skupper-router-certs/{{.Name}}/tls.crt
   privateKeyFile: /etc/skupper-router-certs/{{.Name}}/tls.key
   caCertFile: /etc/skupper-router-certs/{{.Name}}/ca.crt
}
{{- end}}

address {
    prefix: mc
    distribution: multicast
}

## Connectors: ##
`
	var buff bytes.Buffer
	qdrconfig := template.Must(template.New("qdrconfig").Parse(config))
	qdrconfig.Execute(&buff, assembly)
	return buff.String()
}

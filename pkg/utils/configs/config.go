package configs

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

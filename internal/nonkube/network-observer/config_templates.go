package networkobserver

import "fmt"

func RenderPrometheusConfig(netobsPort int) string {
	return fmt.Sprintf(`global:
  scrape_interval: 15s
  evaluation_interval: 15s
alerting:
  alertmanagers:
    - static_configs:
        - targets:
scrape_configs:
  - job_name: "network-observer-local"
    scheme: http
    follow_redirects: true
    enable_http2: true
    static_configs:
      - targets: ["localhost:%d"]
`, netobsPort)
}

func RenderNginxConfig(nginxPort, netobsPort int) string {
	return fmt.Sprintf(`ssl_session_cache   shared:SSL:10m;
ssl_session_timeout 10m;

server {
    listen              %d ssl;
    keepalive_timeout   70;

    ssl_certificate     /etc/certificates/tls.crt;
    ssl_certificate_key /etc/certificates/tls.key;
    ssl_protocols       TLSv1.3;
    add_header Strict-Transport-Security "max-age=63072000" always;

    auth_basic           "Skupper";
    auth_basic_user_file /etc/httpusers/htpasswd;

    location /api/ {
        proxy_pass http://127.0.0.1:%d;
    }
    location / {
        proxy_pass http://127.0.0.1:%d;
    }
}
`, nginxPort, netobsPort, netobsPort)
}

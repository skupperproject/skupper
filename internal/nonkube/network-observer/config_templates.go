package networkobserver

import "fmt"

func RenderPrometheusConfig(targetsDir string) string {
	return fmt.Sprintf(`global:
  scrape_interval: 15s
  evaluation_interval: 15s
alerting:
  alertmanagers:
    - static_configs:
        - targets:
scrape_configs:
  - job_name: "skupper-network-observers"
    scheme: http
    follow_redirects: true
    enable_http2: true
    file_sd_configs:
      - files:
          - "%s/*.json"
        refresh_interval: 15s
`, targetsDir)
}

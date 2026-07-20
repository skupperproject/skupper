package networkobserver

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/skupperproject/skupper/internal/images"
	"github.com/skupperproject/skupper/pkg/nonkube/api"
)

const SystemdServiceTemplate = `[Unit]
Description=Skupper Network Observer - %s
After=network.target
Wants=skupper-network-observer-prometheus-%s.service skupper-network-observer-app-%s.service skupper-network-observer-nginx-%s.service
After=skupper-network-observer-prometheus-%s.service skupper-network-observer-app-%s.service skupper-network-observer-nginx-%s.service

[Service]
Type=oneshot
RemainAfterExit=yes
ExecStart=/bin/true
ExecStop=/bin/true

[Install]
WantedBy=default.target
`

const SystemdPrometheusServiceTemplate = `[Unit]
Description=Skupper Network Observer Prometheus - %s
After=network.target
PartOf=skupper-network-observer-%s.service

[Service]
Type=simple
Restart=always
RestartSec=5
ExecStartPre=-{{.ContainerEngine}} stop %s-skupper-prometheus
ExecStartPre=-{{.ContainerEngine}} rm %s-skupper-prometheus
ExecStart={{.ContainerEngine}} run --name %s-skupper-prometheus \
    --label application=skupper-v2 \
    --label skupper.io/v2-component=prometheus \
    --user={{.RunAsUser}} \
{{.UsernsFlag}}    --network host \
    --restart always \
    -v %s/network-observer/prometheus:/etc/prometheus:z \
    -v %s/network-observer/prometheus/data:/prometheus:z \
    {{.PrometheusImage}} \
    --config.file=/etc/prometheus/prometheus.yml \
    --storage.tsdb.path=/prometheus/ \
    --web.listen-address=:{{.PrometheusPort}}
ExecStop={{.ContainerEngine}} stop %s-skupper-prometheus
ExecStopPost={{.ContainerEngine}} rm %s-skupper-prometheus

[Install]
WantedBy=skupper-network-observer-%s.service
`

const SystemdNetworkObserverServiceTemplate = `[Unit]
Description=Skupper Network Observer Application - %s
After=network.target skupper-controller.service skupper-network-observer-prometheus-%s.service
Wants=skupper-controller.service
PartOf=skupper-network-observer-%s.service

[Service]
Type=simple
Restart=always
RestartSec=5
ExecStartPre=-{{.ContainerEngine}} stop %s-skupper-network-observer
ExecStartPre=-{{.ContainerEngine}} rm %s-skupper-network-observer
ExecStart={{.ContainerEngine}} run --name %s-skupper-network-observer \
    --label application=skupper-v2 \
    --label skupper.io/v2-component=network-observer \
    --user={{.RunAsUser}} \
{{.UsernsFlag}}    --network host \
    --restart always \
    -v %s/runtime/certs/skupper-local-client:/etc/messaging:ro,z \
    {{.NetworkObserverImage}} \
    -listen=127.0.0.1:{{.NetobsPort}} \
    -prometheus-api=http://127.0.0.1:{{.PrometheusPort}} \
    -router-endpoint={{.RouterEndpoint}} \
    -router-tls-ca=/etc/messaging/ca.crt \
    -router-tls-cert=/etc/messaging/tls.crt \
    -router-tls-key=/etc/messaging/tls.key \
    -listen-metrics=:{{.MetricsPort}}
ExecStop={{.ContainerEngine}} stop %s-skupper-network-observer
ExecStopPost={{.ContainerEngine}} rm %s-skupper-network-observer

[Install]
WantedBy=skupper-network-observer-%s.service
`

const SystemdNginxServiceTemplate = `[Unit]
Description=Skupper Network Observer Nginx Proxy - %s
After=network.target skupper-network-observer-app-%s.service
PartOf=skupper-network-observer-%s.service

[Service]
Type=simple
Restart=always
RestartSec=5
ExecStartPre=-{{.ContainerEngine}} stop %s-skupper-nginx
ExecStartPre=-{{.ContainerEngine}} rm %s-skupper-nginx
ExecStart={{.ContainerEngine}} run --name %s-skupper-nginx \
    --label application=skupper-v2 \
    --label skupper.io/v2-component=nginx-proxy \
    --user={{.RunAsUser}} \
{{.UsernsFlag}}    --network host \
    --restart always \
    -v %s/network-observer/nginx/conf.d:/etc/nginx/conf.d:z \
    -v %s/network-observer/certs:/etc/certificates:z \
    -v %s/network-observer/htpasswd:/etc/httpusers:z \
    {{.NginxImage}}
ExecStop={{.ContainerEngine}} stop %s-skupper-nginx
ExecStopPost={{.ContainerEngine}} rm %s-skupper-nginx

[Install]
WantedBy=skupper-network-observer-%s.service
`

type SystemdServiceManager struct {
	Namespace       string
	ContainerEngine string
	ServiceDir      string
	RunAsUser       string
	ports           ports
}

func NewSystemdServiceManager(namespace, containerEngine string, p ports) *SystemdServiceManager {
	serviceDir := getSystemdServiceDir()
	return &SystemdServiceManager{
		Namespace:       namespace,
		ContainerEngine: containerEngine,
		ServiceDir:      serviceDir,
		RunAsUser:       fmt.Sprintf("%d:%d", os.Getuid(), os.Getgid()),
		ports:           p,
	}
}

func getSystemdServiceDir() string {
	if os.Getuid() == 0 {
		return "/etc/systemd/system"
	}
	home, err := os.UserHomeDir()
	if err != nil {
		home = fmt.Sprintf("/home/%s", os.Getenv("USER"))
	}
	return filepath.Join(home, ".config", "systemd", "user")
}

func (s *SystemdServiceManager) CreateServices() error {

	if err := os.MkdirAll(s.ServiceDir, 0755); err != nil {
		return fmt.Errorf("failed to create systemd service directory: %w", err)
	}

	namespacePath := api.GetHostNamespaceHome(s.Namespace)

	mainServiceName := fmt.Sprintf("skupper-network-observer-%s.service", s.Namespace)
	mainServicePath := filepath.Join(s.ServiceDir, mainServiceName)
	mainServiceContent := fmt.Sprintf(SystemdServiceTemplate,
		s.Namespace,
		s.Namespace, s.Namespace, s.Namespace,
		s.Namespace, s.Namespace, s.Namespace)
	if err := os.WriteFile(mainServicePath, []byte(mainServiceContent), 0644); err != nil {
		return fmt.Errorf("failed to write main service file: %w", err)
	}

	prometheusServiceName := fmt.Sprintf("skupper-network-observer-prometheus-%s.service", s.Namespace)
	prometheusServicePath := filepath.Join(s.ServiceDir, prometheusServiceName)
	prometheusServiceContent := s.renderPrometheusService(namespacePath)
	if err := os.WriteFile(prometheusServicePath, []byte(prometheusServiceContent), 0644); err != nil {
		return fmt.Errorf("failed to write prometheus service file: %w", err)
	}

	appServiceName := fmt.Sprintf("skupper-network-observer-app-%s.service", s.Namespace)
	appServicePath := filepath.Join(s.ServiceDir, appServiceName)
	appServiceContent := s.renderNetworkObserverService(namespacePath)
	if err := os.WriteFile(appServicePath, []byte(appServiceContent), 0644); err != nil {
		return fmt.Errorf("failed to write network observer service file: %w", err)
	}

	nginxServiceName := fmt.Sprintf("skupper-network-observer-nginx-%s.service", s.Namespace)
	nginxServicePath := filepath.Join(s.ServiceDir, nginxServiceName)
	nginxServiceContent := s.renderNginxService(namespacePath)
	if err := os.WriteFile(nginxServicePath, []byte(nginxServiceContent), 0644); err != nil {
		return fmt.Errorf("failed to write nginx service file: %w", err)
	}

	if err := s.reloadSystemd(); err != nil {
		return fmt.Errorf("failed to reload systemd: %w", err)
	}

	for _, svc := range []string{
		fmt.Sprintf("skupper-network-observer-prometheus-%s.service", s.Namespace),
		fmt.Sprintf("skupper-network-observer-app-%s.service", s.Namespace),
		fmt.Sprintf("skupper-network-observer-nginx-%s.service", s.Namespace),
		mainServiceName,
	} {
		if err := s.enableService(svc); err != nil {
			return fmt.Errorf("failed to enable service %s: %w", svc, err)
		}
	}

	if err := s.startService(mainServiceName); err != nil {
		return fmt.Errorf("failed to start service: %w", err)
	}

	return nil
}

func (s *SystemdServiceManager) userNsFlag() string {
	if s.ContainerEngine == "podman" {
		return "    --userns=keep-id \\\n"
	}
	return ""
}

func (s *SystemdServiceManager) renderPrometheusService(namespacePath string) string {
	content := fmt.Sprintf(SystemdPrometheusServiceTemplate,
		s.Namespace, s.Namespace,
		s.Namespace, s.Namespace, s.Namespace,
		namespacePath, namespacePath,
		s.Namespace, s.Namespace, s.Namespace)
	content = strings.ReplaceAll(content, "{{.ContainerEngine}}", s.ContainerEngine)
	content = strings.ReplaceAll(content, "{{.RunAsUser}}", s.RunAsUser)
	content = strings.ReplaceAll(content, "{{.UsernsFlag}}", s.userNsFlag())
	content = strings.ReplaceAll(content, "{{.PrometheusImage}}", images.GetPrometheusImageName())
	content = strings.ReplaceAll(content, "{{.PrometheusPort}}", fmt.Sprintf("%d", s.ports.prometheus))
	return content
}

func (s *SystemdServiceManager) renderNetworkObserverService(namespacePath string) string {
	content := fmt.Sprintf(SystemdNetworkObserverServiceTemplate,
		s.Namespace, s.Namespace, s.Namespace,
		s.Namespace, s.Namespace, s.Namespace,
		namespacePath,
		s.Namespace, s.Namespace, s.Namespace)
	content = strings.ReplaceAll(content, "{{.ContainerEngine}}", s.ContainerEngine)
	content = strings.ReplaceAll(content, "{{.RunAsUser}}", s.RunAsUser)
	content = strings.ReplaceAll(content, "{{.UsernsFlag}}", s.userNsFlag())
	content = strings.ReplaceAll(content, "{{.NetworkObserverImage}}", images.GetNetworkObserverImageName())
	content = strings.ReplaceAll(content, "{{.NetobsPort}}", fmt.Sprintf("%d", s.ports.netobs))
	content = strings.ReplaceAll(content, "{{.PrometheusPort}}", fmt.Sprintf("%d", s.ports.prometheus))
	content = strings.ReplaceAll(content, "{{.RouterEndpoint}}", s.ports.router)
	content = strings.ReplaceAll(content, "{{.MetricsPort}}", fmt.Sprintf("%d", s.ports.metrics))
	return content
}

func (s *SystemdServiceManager) renderNginxService(namespacePath string) string {
	content := fmt.Sprintf(SystemdNginxServiceTemplate,
		s.Namespace, s.Namespace, s.Namespace,
		s.Namespace, s.Namespace, s.Namespace,
		namespacePath, namespacePath, namespacePath,
		s.Namespace, s.Namespace, s.Namespace)
	content = strings.ReplaceAll(content, "{{.ContainerEngine}}", s.ContainerEngine)
	content = strings.ReplaceAll(content, "{{.RunAsUser}}", s.RunAsUser)
	content = strings.ReplaceAll(content, "{{.UsernsFlag}}", s.userNsFlag())
	content = strings.ReplaceAll(content, "{{.NginxImage}}", images.GetNginxImageName())
	return content
}

func (s *SystemdServiceManager) reloadSystemd() error {
	var cmd *exec.Cmd
	if os.Getuid() == 0 {
		cmd = exec.Command("systemctl", "daemon-reload")
	} else {
		cmd = exec.Command("systemctl", "--user", "daemon-reload")
	}
	return cmd.Run()
}

func (s *SystemdServiceManager) enableService(serviceName string) error {
	var cmd *exec.Cmd
	if os.Getuid() == 0 {
		cmd = exec.Command("systemctl", "enable", serviceName)
	} else {
		cmd = exec.Command("systemctl", "--user", "enable", serviceName)
	}
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to enable %s: %w", serviceName, err)
	}
	return nil
}

func (s *SystemdServiceManager) startService(serviceName string) error {
	var cmd *exec.Cmd
	if os.Getuid() == 0 {
		cmd = exec.Command("systemctl", "start", serviceName)
	} else {
		cmd = exec.Command("systemctl", "--user", "start", serviceName)
	}
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to start %s: %w", serviceName, err)
	}
	return nil
}

func (s *SystemdServiceManager) RemoveServices() error {
	mainServiceName := fmt.Sprintf("skupper-network-observer-%s.service", s.Namespace)

	// Stop and disable the main service
	if err := s.stopAndDisableService(mainServiceName); err != nil {
		// Log but don't fail if service doesn't exist
		fmt.Printf("Warning: failed to stop service: %v\n", err)
	}

	// Remove service files
	serviceNames := []string{
		mainServiceName,
		fmt.Sprintf("skupper-network-observer-prometheus-%s.service", s.Namespace),
		fmt.Sprintf("skupper-network-observer-app-%s.service", s.Namespace),
		fmt.Sprintf("skupper-network-observer-nginx-%s.service", s.Namespace),
	}

	for _, serviceName := range serviceNames {
		servicePath := filepath.Join(s.ServiceDir, serviceName)
		if err := os.Remove(servicePath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to remove service file %s: %w", serviceName, err)
		}
	}

	// Reload systemd
	if err := s.reloadSystemd(); err != nil {
		return fmt.Errorf("failed to reload systemd: %w", err)
	}

	return nil
}

func (s *SystemdServiceManager) stopAndDisableService(serviceName string) error {
	var stopCmd, disableCmd *exec.Cmd
	if os.Getuid() == 0 {
		stopCmd = exec.Command("systemctl", "stop", serviceName)
		disableCmd = exec.Command("systemctl", "disable", serviceName)
	} else {
		stopCmd = exec.Command("systemctl", "--user", "stop", serviceName)
		disableCmd = exec.Command("systemctl", "--user", "disable", serviceName)
	}

	err := stopCmd.Run()
	if err != nil {
		return err
	}

	err = disableCmd.Run()
	if err != nil {
		return err
	}

	return nil
}

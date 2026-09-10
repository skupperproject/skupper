package networkobserver

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

const SystemdPrometheusHostServiceTemplate = `[Unit]
Description=Skupper Prometheus (host-level)
After=network.target

[Service]
Type=simple
Restart=always
RestartSec=5
ExecStart=%s start --attach skupper-prometheus
ExecStop=%s stop skupper-prometheus

[Install]
WantedBy=default.target
`

const SystemdNetworkObserverServiceTemplate = `[Unit]
Description=Skupper Network Observer - %s
After=network.target skupper-controller.service skupper-prometheus.service
Wants=skupper-controller.service skupper-prometheus.service

[Service]
Type=simple
Restart=always
RestartSec=5
ExecStart=%s start --attach %s-skupper-network-observer
ExecStop=%s stop %s-skupper-network-observer

[Install]
WantedBy=default.target
`

type SystemdServiceManager struct {
	Namespace       string
	ContainerEngine string
	ServiceDir      string
}

func NewSystemdServiceManager(namespace, containerEngine string, _ ports) *SystemdServiceManager {
	serviceDir := getSystemdServiceDir()
	return &SystemdServiceManager{
		Namespace:       namespace,
		ContainerEngine: containerEngine,
		ServiceDir:      serviceDir,
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

func (s *SystemdServiceManager) CreateNetworkObserverService() error {

	if err := os.MkdirAll(s.ServiceDir, 0755); err != nil {
		return fmt.Errorf("failed to create systemd service directory: %w", err)
	}

	svcName := fmt.Sprintf("skupper-network-observer-%s.service", s.Namespace)
	svcPath := filepath.Join(s.ServiceDir, svcName)
	svcContent := fmt.Sprintf(SystemdNetworkObserverServiceTemplate,
		s.Namespace,
		s.ContainerEngine, s.Namespace,
		s.ContainerEngine, s.Namespace)
	if err := os.WriteFile(svcPath, []byte(svcContent), 0644); err != nil {
		return fmt.Errorf("failed to write network observer service file: %w", err)
	}

	if err := s.enableService(svcName); err != nil {
		return fmt.Errorf("failed to enable service %s: %w", svcName, err)
	}

	if err := s.startService(svcName); err != nil {
		return fmt.Errorf("failed to start service %s: %w", svcName, err)
	}

	return nil
}

func (s *SystemdServiceManager) RemoveNetworkObserverService() error {
	svcName := fmt.Sprintf("skupper-network-observer-%s.service", s.Namespace)

	if err := s.stopAndDisableService(svcName); err != nil {
		fmt.Printf("Warning: failed to stop/disable service %s: %v\n", svcName, err)
	}

	svcPath := filepath.Join(s.ServiceDir, svcName)
	if err := os.Remove(svcPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove service file %s: %w", svcName, err)
	}

	if err := s.reloadSystemd(); err != nil {
		return fmt.Errorf("failed to reload systemd: %w", err)
	}

	return nil
}

func (s *SystemdServiceManager) CreatePrometheusService() error {
	if err := os.MkdirAll(s.ServiceDir, 0755); err != nil {
		return fmt.Errorf("failed to create systemd service directory: %w", err)
	}

	svcName := "skupper-prometheus.service"
	svcPath := filepath.Join(s.ServiceDir, svcName)
	svcContent := fmt.Sprintf(SystemdPrometheusHostServiceTemplate,
		s.ContainerEngine,
		s.ContainerEngine)
	if err := os.WriteFile(svcPath, []byte(svcContent), 0644); err != nil {
		return fmt.Errorf("failed to write prometheus service file: %w", err)
	}

	if err := s.enableServiceByPath(svcPath); err != nil {
		return fmt.Errorf("failed to enable prometheus service: %w", err)
	}
	if err := s.startService(svcName); err != nil {
		return fmt.Errorf("failed to start prometheus service: %w", err)
	}
	return nil
}

func (s *SystemdServiceManager) RemovePrometheusService() error {
	svcName := "skupper-prometheus.service"
	if err := s.stopAndDisableService(svcName); err != nil {
		fmt.Printf("Warning: failed to stop prometheus service: %v\n", err)
	}
	svcPath := filepath.Join(s.ServiceDir, svcName)
	if err := os.Remove(svcPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove prometheus service file: %w", err)
	}
	if err := s.reloadSystemd(); err != nil {
		return fmt.Errorf("failed to reload systemd: %w", err)
	}
	return nil
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

func (s *SystemdServiceManager) enableServiceByPath(servicePath string) error {
	var cmd *exec.Cmd
	if os.Getuid() == 0 {
		cmd = exec.Command("systemctl", "enable", servicePath)
	} else {
		cmd = exec.Command("systemctl", "--user", "enable", servicePath)
	}
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to enable %s: %w", servicePath, err)
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

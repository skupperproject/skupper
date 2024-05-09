package config

import (
	"bytes"
	_ "embed"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"text/template"

	"github.com/skupperproject/skupper/api/types"
)

var (
	//go:embed systemd_service.template
	SystemdServiceTemplate string
)

type systemdServiceInfo struct {
	Platform    types.Platform
	RuntimeDir  string
	DataHomeDir string
}

func NewSystemdServiceInfo(platform types.Platform) *systemdServiceInfo {
	return &systemdServiceInfo{
		Platform:    platform,
		RuntimeDir:  GetRuntimeDir(),
		DataHomeDir: GetDataHome(),
	}
}

func (s *systemdServiceInfo) Create() error {
	if !IsSystemdUserEnabled() {
		return fmt.Errorf("SystemD is not enabled at user level")
	}

	var buf bytes.Buffer
	service := template.Must(template.New(fmt.Sprintf("skupper-%s", s.Platform)).Parse(SystemdServiceTemplate))
	err := service.Execute(&buf, s)
	if err != nil {
		return err
	}

	// Creating the base dir
	baseDir := filepath.Dir(s.getServiceFile())
	if _, err := os.Stat(baseDir); err != nil {
		if err = os.MkdirAll(baseDir, 0755); err != nil {
			return fmt.Errorf("unable to create base directory %s - %q", baseDir, err)
		}
	}

	// Saving systemd user service
	serviceName := s.getServiceName()
	err = os.WriteFile(s.getServiceFile(), buf.Bytes(), 0644)
	if err != nil {
		return fmt.Errorf("Unable to write user unit file: %w", err)
	}

	// Enabling systemd user service
	cmd := GetCmdEnableSystemdService(serviceName)
	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("Unable to enable user service: %w", err)
	}

	// Reloading systemd user daemon
	cmd = GetCmdReloadSystemdDaemon()
	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("Unable to user service daemon-reload: %w", err)
	}

	// Starting systemd user service
	cmd = GetCmdStartSystemdService(serviceName)
	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("Unable to start user service: %w", err)
	}

	return nil
}

func (s *systemdServiceInfo) getServiceFile() string {
	if os.Getuid() == 0 {
		return path.Join("/etc/systemd/system", s.getServiceName())
	}
	return path.Join(GetConfigHome(), "systemd/user", s.getServiceName())
}

func (s *systemdServiceInfo) getServiceName() string {
	return "skupper-" + string(s.Platform) + ".service"
}

func (s *systemdServiceInfo) Remove() error {
	if !IsSystemdUserEnabled() {
		return fmt.Errorf("SystemD is not enabled at user level")
	}

	// Stopping systemd user service
	serviceName := "skupper-" + string(s.Platform) + ".service"
	cmd := GetCmdStopSystemdService(serviceName)
	_ = cmd.Run()

	// Disabling systemd user service
	cmd = GetCmdDisableSystemdService(serviceName)
	_ = cmd.Run()

	// Removing the .service file
	_ = os.Remove(s.getServiceFile())

	// Reloading systemd user daemon
	cmd = GetCmdReloadSystemdDaemon()
	_ = cmd.Run()

	// Resetting failed status
	cmd = GetCmdResetFailedSystemService(serviceName)
	_ = cmd.Run()

	return nil
}

func GetCmdEnableSystemdService(serviceName string) *exec.Cmd {
	if os.Getuid() == 0 {
		return exec.Command("systemctl", "enable", serviceName)
	}
	return exec.Command("systemctl", "--user", "enable", serviceName)
}

func GetCmdDisableSystemdService(serviceName string) *exec.Cmd {
	if os.Getuid() == 0 {
		return exec.Command("systemctl", "disable", serviceName)
	}
	return exec.Command("systemctl", "--user", "disable", serviceName)
}

func GetCmdReloadSystemdDaemon() *exec.Cmd {
	if os.Getuid() == 0 {
		return exec.Command("systemctl", "daemon-reload")
	}
	return exec.Command("systemctl", "--user", "daemon-reload")
}

func GetCmdStartSystemdService(serviceName string) *exec.Cmd {
	if os.Getuid() == 0 {
		return exec.Command("systemctl", "start", serviceName)
	}
	return exec.Command("systemctl", "--user", "start", serviceName)
}

func GetCmdStopSystemdService(serviceName string) *exec.Cmd {
	if os.Getuid() == 0 {
		return exec.Command("systemctl", "stop", serviceName)
	}
	return exec.Command("systemctl", "--user", "stop", serviceName)
}

func GetCmdResetFailedSystemService(serviceName string) *exec.Cmd {
	if os.Getuid() == 0 {
		return exec.Command("systemctl", "reset-failed", serviceName)
	}
	return exec.Command("systemctl", "--user", "reset-failed", serviceName)
}

func GetCmdIsSystemdEnabled() *exec.Cmd {
	if os.Getuid() == 0 {
		return exec.Command("systemctl", []string{"list-units", "--no-pager"}...)
	}
	return exec.Command("systemctl", []string{"--user", "list-units", "--no-pager"}...)
}

func IsSystemdUserEnabled() bool {
	cmd := GetCmdIsSystemdEnabled()
	if err := cmd.Run(); err != nil {
		return false
	}
	return true
}

func IsLingeringEnabled(user string) bool {
	lingerFile := fmt.Sprintf("/var/lib/systemd/linger/%s", user)
	_, err := os.Stat(lingerFile)
	return err == nil
}

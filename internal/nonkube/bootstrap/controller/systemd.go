package controller

import (
	"bytes"
	_ "embed"
	"fmt"
	"github.com/skupperproject/skupper/internal/nonkube/common"
	"github.com/skupperproject/skupper/pkg/container"
	"log/slog"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"text/template"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/pkg/nonkube/api"
)

var (
	//go:embed systemd_container_service.template
	SystemdContainerServiceTemplate string
)

const (
	rootSystemdBasePath = "/etc/systemd/system"
)

type SystemdService interface {
	GetServiceName() string
	Create() error
	Remove() error
	GetServiceFile() string
}

type CommandExecutor func(name string, arg ...string) *exec.Cmd

type systemdServiceInfo struct {
	Name                string
	Image               string
	Env                 map[string](string)
	Mounts              []container.Volume
	Annotations         map[string]string
	Platform            string
	RuntimeDir          string
	GetUid              api.IdGetter
	command             CommandExecutor
	rootSystemdBasePath string
	ScriptPath          string
}

func NewSystemdServiceInfo(systemContainer container.Container, platform string) (SystemdService, error) {

	scriptPath := path.Join(api.GetSystemControllerPath(), "internal", "scripts")

	return &systemdServiceInfo{
		Name:                "skupper-controller",
		Image:               systemContainer.Image,
		Env:                 systemContainer.Env,
		Mounts:              systemContainer.Mounts,
		Annotations:         systemContainer.Annotations,
		RuntimeDir:          api.GetRuntimeDir(),
		GetUid:              os.Getuid,
		command:             exec.Command,
		rootSystemdBasePath: rootSystemdBasePath,
		Platform:            platform,
		ScriptPath:          scriptPath,
	}, nil
}

func (s *systemdServiceInfo) GetServiceName() string {
	return fmt.Sprintf("%s.service", s.Name)
}

func (s *systemdServiceInfo) Create() error {
	if !api.IsRunningInContainer() && !s.isSystemdEnabled() {
		msg := "SystemD is not enabled"
		if s.GetUid() != 0 {
			msg += " at user level"
		}
		return fmt.Errorf("%s", msg)
	}
	var logger = common.NewLogger()
	logger.Debug("creating systemd service")
	var buf = new(bytes.Buffer)
	var service *template.Template
	logger.Debug("using service template for:", slog.String("platform", s.Platform))
	if s.Platform == string(types.PlatformLinux) {
		return fmt.Errorf("the creation of the systemd service is not supported on Linux platform")
	} else {
		service = template.Must(template.New(s.GetServiceName()).Parse(SystemdContainerServiceTemplate))
	}
	err := service.Execute(buf, s)
	if err != nil {
		return err
	}

	// Creating the base dir
	baseDir := filepath.Dir(s.GetServiceFile())
	if _, err := os.Stat(baseDir); err != nil {
		if err = os.MkdirAll(baseDir, 0755); err != nil {
			return fmt.Errorf("unable to create base directory %s - %q", baseDir, err)
		}
	}

	// Saving systemd user service
	serviceName := s.GetServiceName()
	logger.Debug("writing service file", slog.String("path", s.GetServiceFile()))
	err = os.WriteFile(s.GetServiceFile(), buf.Bytes(), 0644)
	if err != nil {
		return fmt.Errorf("unable to write unit file (%s): %w", s.GetServiceFile(), err)
	}

	// Only enable when running locally
	if !api.IsRunningInContainer() {
		logger.Debug("enabling systemd service", slog.String("name", serviceName))
		return s.enableService(serviceName)
	}

	return nil
}

func (s *systemdServiceInfo) GetServiceFile() string {
	if api.IsRunningInContainer() {
		outputStat, err := os.Stat("/output")
		if err == nil && outputStat.IsDir() {
			return path.Join(path.Join("output", string(api.ScriptsPath)), s.GetServiceName())
		}
	}
	if s.GetUid() == 0 {
		return path.Join(s.rootSystemdBasePath, s.GetServiceName())
	}
	return path.Join(api.GetConfigHome(), "systemd/user", s.GetServiceName())
}

func (s *systemdServiceInfo) Remove() error {
	if !api.IsRunningInContainer() && !s.isSystemdEnabled() {
		return fmt.Errorf("SystemD is not enabled at user level")
	}

	logger := common.NewLogger()

	// Stopping systemd user service
	if !api.IsRunningInContainer() {
		logger.Debug("stopping service", slog.String("name", s.GetServiceName()))
		cmd := s.getCmdStopSystemdService(s.GetServiceName())
		_ = cmd.Run()

		// Disabling systemd user service
		logger.Debug("disabling service", slog.String("name", s.GetServiceName()))
		cmd = s.getCmdDisableSystemdService(s.GetServiceName())
		_ = cmd.Run()
	}

	// Removing the .service file
	logger.Debug("removing service", slog.String("path", s.GetServiceFile()))
	_ = os.Remove(s.GetServiceFile())

	// Reloading systemd user daemon
	if !api.IsRunningInContainer() {
		logger.Debug("reloading systemd daemon")
		cmd := s.getCmdReloadSystemdDaemon()
		_ = cmd.Run()

		// Resetting failed status
		logger.Debug("resetting failed systemd service", slog.String("name", s.GetServiceName()))
		cmd = s.getCmdResetFailedSystemService(s.GetServiceName())
		_ = cmd.Run()
	}

	return nil
}

func (s *systemdServiceInfo) enableService(serviceName string) error {
	// Enabling systemd user service
	cmd := s.getCmdEnableSystemdService(serviceName)
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("unable to enable service (%s): %w", s.GetServiceFile(), err)
	}

	// Reloading systemd user daemon
	cmd = s.getCmdReloadSystemdDaemon()
	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("Unable to user service daemon-reload: %w", err)
	}

	// Starting systemd user service
	cmd = s.getCmdStartSystemdService(serviceName)
	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("Unable to start user service: %w", err)
	}

	return nil
}

func (s *systemdServiceInfo) getCmdEnableSystemdService(serviceName string) *exec.Cmd {
	if s.GetUid() == 0 {
		return s.command("systemctl", "enable", serviceName)
	}
	return s.command("systemctl", "--user", "enable", serviceName)
}

func (s *systemdServiceInfo) getCmdDisableSystemdService(serviceName string) *exec.Cmd {
	if s.GetUid() == 0 {
		return s.command("systemctl", "disable", serviceName)
	}
	return s.command("systemctl", "--user", "disable", serviceName)
}

func (s *systemdServiceInfo) getCmdReloadSystemdDaemon() *exec.Cmd {
	if s.GetUid() == 0 {
		return s.command("systemctl", "daemon-reload")
	}
	return s.command("systemctl", "--user", "daemon-reload")
}

func (s *systemdServiceInfo) getCmdStartSystemdService(serviceName string) *exec.Cmd {
	if s.GetUid() == 0 {
		return s.command("systemctl", "start", serviceName)
	}
	return s.command("systemctl", "--user", "start", serviceName)
}

func (s *systemdServiceInfo) getCmdStopSystemdService(serviceName string) *exec.Cmd {
	if s.GetUid() == 0 {
		return s.command("systemctl", "stop", serviceName)
	}
	return s.command("systemctl", "--user", "stop", serviceName)
}

func (s *systemdServiceInfo) getCmdResetFailedSystemService(serviceName string) *exec.Cmd {
	if s.GetUid() == 0 {
		return s.command("systemctl", "reset-failed", serviceName)
	}
	return s.command("systemctl", "--user", "reset-failed", serviceName)
}

func (s *systemdServiceInfo) getCmdIsSystemdEnabled() *exec.Cmd {
	if s.GetUid() == 0 {
		return s.command("systemctl", []string{"list-units", "--no-pager"}...)
	}
	return s.command("systemctl", []string{"--user", "list-units", "--no-pager"}...)
}

func (s *systemdServiceInfo) isSystemdEnabled() bool {
	cmd := s.getCmdIsSystemdEnabled()
	if err := cmd.Run(); err != nil {
		return false
	}
	return true
}

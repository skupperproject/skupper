package common

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
	"github.com/skupperproject/skupper/pkg/apis/skupper/v1alpha1"
	"github.com/skupperproject/skupper/pkg/config"
	"github.com/skupperproject/skupper/pkg/nonkube/apis"
)

var (
	//go:embed systemd_container_service.template
	SystemdContainerServiceTemplate string
	//go:embed systemd_service.template
	SystemdServiceTemplate string
)

const (
	rootSystemdBasePath = "/etc/systemd/system"
)

type SystemdService interface {
	GetServiceName() string
	Create() error
	Remove() error
}

type CommandExecutor func(name string, arg ...string) *exec.Cmd

type systemdServiceInfo struct {
	Site                *v1alpha1.Site
	SiteScriptPath      string
	SiteConfigPath      string
	SiteHomePath        string
	RuntimeDir          string
	getUid              apis.IdGetter
	command             CommandExecutor
	rootSystemdBasePath string
}

func NewSystemdServiceInfo(site *v1alpha1.Site) (SystemdService, error) {
	siteHomePath, err := apis.GetHostSiteHome(site)
	if err != nil {
		return nil, err
	}
	siteScriptPath := path.Join(siteHomePath, RuntimeScriptsPath)
	siteConfigPath := path.Join(siteHomePath, ConfigRouterPath)
	return &systemdServiceInfo{
		Site:                site,
		SiteScriptPath:      siteScriptPath,
		SiteConfigPath:      siteConfigPath,
		RuntimeDir:          apis.GetRuntimeDir(),
		getUid:              os.Getuid,
		command:             exec.Command,
		rootSystemdBasePath: rootSystemdBasePath,
	}, nil
}

func (s *systemdServiceInfo) GetServiceName() string {
	return fmt.Sprintf("skupper-site-%s.service", s.Site.Name)
}

func (s *systemdServiceInfo) Create() error {
	if !apis.IsRunningInContainer() && !s.isSystemdEnabled() {
		msg := "SystemD is not enabled"
		if s.getUid() != 0 {
			msg += " at user level"
		}
		return fmt.Errorf(msg)
	}

	platform := config.GetPlatform()
	var buf = new(bytes.Buffer)
	var service *template.Template
	if platform == types.PlatformSystemd {
		service = template.Must(template.New(s.GetServiceName()).Parse(SystemdServiceTemplate))
	} else {
		service = template.Must(template.New(s.GetServiceName()).Parse(SystemdContainerServiceTemplate))
	}
	err := service.Execute(buf, s)
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
	serviceName := s.GetServiceName()
	err = os.WriteFile(s.getServiceFile(), buf.Bytes(), 0644)
	if err != nil {
		return fmt.Errorf("unable to write unit file (%s): %w", s.getServiceFile(), err)
	}

	// Only enable when running locally
	if !apis.IsRunningInContainer() {
		return s.enableService(serviceName)
	}

	return nil
}

func (s *systemdServiceInfo) getServiceFile() string {
	if apis.IsRunningInContainer() {
		return path.Join(GetDefaultOutputPath(s.Site.Name), RuntimeScriptsPath, s.GetServiceName())
	}
	if s.getUid() == 0 {
		return path.Join(s.rootSystemdBasePath, s.GetServiceName())
	}
	return path.Join(apis.GetConfigHome(), "systemd/user", s.GetServiceName())
}

func (s *systemdServiceInfo) Remove() error {
	if !apis.IsRunningInContainer() && !s.isSystemdEnabled() {
		return fmt.Errorf("SystemD is not enabled at user level")
	}

	// Stopping systemd user service
	if !apis.IsRunningInContainer() {
		cmd := s.getCmdStopSystemdService(s.GetServiceName())
		_ = cmd.Run()

		// Disabling systemd user service
		cmd = s.getCmdDisableSystemdService(s.GetServiceName())
		_ = cmd.Run()
	}

	// Removing the .service file
	_ = os.Remove(s.getServiceFile())

	// Reloading systemd user daemon
	if !apis.IsRunningInContainer() {
		cmd := s.getCmdReloadSystemdDaemon()
		_ = cmd.Run()

		// Resetting failed status
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
		return fmt.Errorf("unable to enable service (%s): %w", s.getServiceFile(), err)
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
	if s.getUid() == 0 {
		return s.command("systemctl", "enable", serviceName)
	}
	return s.command("systemctl", "--user", "enable", serviceName)
}

func (s *systemdServiceInfo) getCmdDisableSystemdService(serviceName string) *exec.Cmd {
	if s.getUid() == 0 {
		return s.command("systemctl", "disable", serviceName)
	}
	return s.command("systemctl", "--user", "disable", serviceName)
}

func (s *systemdServiceInfo) getCmdReloadSystemdDaemon() *exec.Cmd {
	if s.getUid() == 0 {
		return s.command("systemctl", "daemon-reload")
	}
	return s.command("systemctl", "--user", "daemon-reload")
}

func (s *systemdServiceInfo) getCmdStartSystemdService(serviceName string) *exec.Cmd {
	if s.getUid() == 0 {
		return s.command("systemctl", "start", serviceName)
	}
	return s.command("systemctl", "--user", "start", serviceName)
}

func (s *systemdServiceInfo) getCmdStopSystemdService(serviceName string) *exec.Cmd {
	if s.getUid() == 0 {
		return s.command("systemctl", "stop", serviceName)
	}
	return s.command("systemctl", "--user", "stop", serviceName)
}

func (s *systemdServiceInfo) getCmdResetFailedSystemService(serviceName string) *exec.Cmd {
	if s.getUid() == 0 {
		return s.command("systemctl", "reset-failed", serviceName)
	}
	return s.command("systemctl", "--user", "reset-failed", serviceName)
}

func (s *systemdServiceInfo) getCmdIsSystemdEnabled() *exec.Cmd {
	if s.getUid() == 0 {
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

func IsLingeringEnabled(user string) bool {
	lingerFile := fmt.Sprintf("/var/lib/systemd/linger/%s", user)
	_, err := os.Stat(lingerFile)
	return err == nil
}

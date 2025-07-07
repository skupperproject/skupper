package common

import (
	"bytes"
	_ "embed"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"text/template"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/pkg/apis/skupper/v2alpha1"
	"github.com/skupperproject/skupper/pkg/nonkube/api"
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
	GetServiceFile() string
}

type SystemdGlobal interface {
	Enable() error
	Disable() error
}

type CommandExecutor func(name string, arg ...string) *exec.Cmd

type systemdServiceInfo struct {
	Site                *v2alpha1.Site
	SiteId              string
	Namespace           string
	SiteScriptPath      string
	SiteConfigPath      string
	SiteHomePath        string
	RuntimeDir          string
	getUid              api.IdGetter
	command             CommandExecutor
	rootSystemdBasePath string
	platform            string
}

type systemdGlobal struct {
	getUid              api.IdGetter
	command             CommandExecutor
	rootSystemdBasePath string
	platform            string
}

func NewSystemdServiceInfo(siteState *api.SiteState, platform string) (SystemdService, error) {
	site := siteState.Site
	siteHomePath := api.GetHostSiteHome(site)
	siteScriptPath := path.Join(siteHomePath, string(api.ScriptsPath))
	siteConfigPath := path.Join(siteHomePath, string(api.RouterConfigPath))
	namespace := site.Namespace
	if namespace == "" {
		namespace = "default"
	}
	return &systemdServiceInfo{
		Site:                site,
		SiteId:              siteState.SiteId,
		Namespace:           namespace,
		SiteScriptPath:      siteScriptPath,
		SiteConfigPath:      siteConfigPath,
		RuntimeDir:          api.GetRuntimeDir(),
		getUid:              os.Getuid,
		command:             exec.Command,
		rootSystemdBasePath: rootSystemdBasePath,
		platform:            platform,
	}, nil
}

func (s *systemdServiceInfo) GetServiceName() string {
	return fmt.Sprintf("skupper-%s.service", s.Namespace)
}

func (s *systemdServiceInfo) Create() error {
	if !api.IsRunningInContainer() && !s.isSystemdEnabled() {
		msg := "SystemD is not enabled"
		if s.getUid() != 0 {
			msg += " at user level"
		}
		return fmt.Errorf("%s", msg)
	}
	var logger = NewLogger()
	logger.Debug("creating systemd service")
	var buf = new(bytes.Buffer)
	var service *template.Template
	logger.Debug("using service template for:", slog.String("platform", s.platform))
	if s.platform == string(types.PlatformLinux) {
		service = template.Must(template.New(s.GetServiceName()).Parse(SystemdServiceTemplate))
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
		return path.Join(api.GetInternalOutputPath(s.Site.Namespace, api.ScriptsPath), s.GetServiceName())
	}
	if s.getUid() == 0 {
		return path.Join(s.rootSystemdBasePath, s.GetServiceName())
	}
	return path.Join(api.GetConfigHome(), "systemd/user", s.GetServiceName())
}

func (s *systemdServiceInfo) Remove() error {
	if !api.IsRunningInContainer() && !s.isSystemdEnabled() {
		return fmt.Errorf("SystemD is not enabled at user level")
	}

	logger := NewLogger()

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

func NewSystemdGlobal(platform string) (SystemdGlobal, error) {

	return &systemdGlobal{
		getUid:              os.Getuid,
		command:             exec.Command,
		rootSystemdBasePath: rootSystemdBasePath,
		platform:            platform,
	}, nil
}

func (sg *systemdGlobal) getCmdEnableSocket() *exec.Cmd {
	if sg.getUid() == 0 {
		return sg.command("systemctl", "enable", sg.platform+".socket")
	} else if sg.platform == "podman" {
		return sg.command("systemctl", "--user", "enable", sg.platform+".socket")
	}
	return nil
}

func (sg *systemdGlobal) getCmdStartSocket() *exec.Cmd {
	if sg.getUid() == 0 {
		return sg.command("systemctl", "start", sg.platform+".socket")
	} else if sg.platform == "podman" {
		return sg.command("systemctl", "--user", "start", sg.platform+".socket")
	}
	return nil
}

func (sg *systemdGlobal) getCmdStopSocket() *exec.Cmd {

	if sg.getUid() == 0 {
		return sg.command("systemctl", "stop", sg.platform+".socket")
	} else if sg.platform == "podman" {
		return sg.command("systemctl", "--user", "stop", sg.platform+".socket")
	}

	return nil
}

func (sg *systemdGlobal) getCmdDisableSocket() *exec.Cmd {

	if sg.getUid() == 0 {
		return sg.command("systemctl", "disable", sg.platform+".socket")
	} else if sg.platform == "podman" {
		return sg.command("systemctl", "--user", "disable", sg.platform+".socket")
	}

	return nil
}

func (sg *systemdGlobal) Enable() error {

	if sg.platform == "docker" && sg.getUid() != 0 {
		return nil
	}

	err := sg.getCmdEnableSocket().Run()
	if err != nil {
		return err
	}

	err = sg.getCmdStartSocket().Run()
	if err != nil {
		return err
	}

	fmt.Printf("Enabled %s socket \n", sg.platform)

	return nil

}

func (sg *systemdGlobal) Disable() error {

	if sg.platform == "podman" {

		err := sg.getCmdDisableSocket().Run()
		if err != nil {
			return err
		}

		err = sg.getCmdStopSocket().Run()
		if err != nil {
			return err
		}
	}

	return nil

}

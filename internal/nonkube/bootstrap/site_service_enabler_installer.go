package bootstrap

import (
	"bytes"
	_ "embed"
	"fmt"
	"os"
	"os/exec"
	"path"
	"text/template"

	"github.com/skupperproject/skupper/pkg/nonkube/api"
)

//go:embed site_service_enabler_service.template
var siteServiceEnablerServiceTemplate string

//go:embed site_service_enabler_script.template
var siteServiceEnablerScriptTemplate string

const (
	siteServiceEnablerRootSystemdBasePath = "/etc/systemd/system"
	siteServiceEnablerName                = "skupper-site-service-enabler"
	siteServiceEnablerServiceFile         = siteServiceEnablerName + ".service"
	siteServiceEnablerWrapperScript       = siteServiceEnablerName
)

type siteServiceEnablerData struct {
	ScriptPath string
	WantedBy   string
}

type siteServiceEnablerScriptData struct {
	NamespacesDir  string
	SystemdUnitDir string
	SystemctlArgs  string
}

type SiteServiceEnablerInstaller struct {
	uid                 int
	rootSystemdBasePath string
	scriptDir           string
	command             func(string, ...string) *exec.Cmd
}

func newSiteServiceEnablerInstaller() *SiteServiceEnablerInstaller {
	return &SiteServiceEnablerInstaller{
		uid:                 os.Getuid(),
		rootSystemdBasePath: siteServiceEnablerRootSystemdBasePath,
		scriptDir:           path.Join(api.GetSystemControllerPath(), "bin"),
		command:             exec.Command,
	}
}

func (s *SiteServiceEnablerInstaller) Install() error {
	if s.isRunning() {
		return nil
	}
	if err := os.MkdirAll(s.scriptDir, 0755); err != nil {
		return fmt.Errorf("unable to create script directory %q: %w", s.scriptDir, err)
	}
	if err := os.MkdirAll(s.userSystemdDir(), 0755); err != nil {
		return fmt.Errorf("unable to create systemd unit directory %q: %w", s.userSystemdDir(), err)
	}

	scriptPath := path.Join(s.scriptDir, siteServiceEnablerWrapperScript)
	if err := s.renderFile(siteServiceEnablerScriptTemplate, s.scriptData(), scriptPath, 0755); err != nil {
		return fmt.Errorf("unable to write wrapper script %q: %w", scriptPath, err)
	}

	serviceFile := s.unitPath(siteServiceEnablerServiceFile)
	if err := s.renderFile(siteServiceEnablerServiceTemplate, s.templateData(scriptPath), serviceFile, 0644); err != nil {
		return fmt.Errorf("unable to write site enabler service unit: %w", err)
	}

	if err := s.systemctl("daemon-reload"); err != nil {
		return fmt.Errorf("daemon-reload failed: %w", err)
	}
	if err := s.systemctl("enable", siteServiceEnablerServiceFile); err != nil {
		return fmt.Errorf("unable to enable %s: %w", siteServiceEnablerServiceFile, err)
	}
	if err := s.systemctl("start", siteServiceEnablerServiceFile); err != nil {
		return fmt.Errorf("unable to start %s: %w", siteServiceEnablerServiceFile, err)
	}

	return nil
}

func (s *SiteServiceEnablerInstaller) Remove() error {
	unitFile := s.unitPath(siteServiceEnablerServiceFile)
	if _, err := os.Stat(unitFile); err == nil {
		if err := s.systemctl("stop", siteServiceEnablerServiceFile); err != nil {
			return fmt.Errorf("failed to stop %s: %w", siteServiceEnablerServiceFile, err)
		}
		if err := s.systemctl("disable", siteServiceEnablerServiceFile); err != nil {
			return fmt.Errorf("failed to disable %s: %w", siteServiceEnablerServiceFile, err)
		}
		if err := os.Remove(unitFile); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to remove unit file: %w", err)
		}
	}
	if err := os.Remove(path.Join(s.scriptDir, siteServiceEnablerWrapperScript)); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove wrapper script: %w", err)
	}
	if err := s.systemctl("daemon-reload"); err != nil {
		return fmt.Errorf("daemon-reload failed after remove: %w", err)
	}
	return nil
}

func (s *SiteServiceEnablerInstaller) scriptData() siteServiceEnablerScriptData {
	systemctlArgs := ""
	if s.uid != 0 {
		systemctlArgs = "--user "
	}
	return siteServiceEnablerScriptData{
		NamespacesDir:  api.GetDefaultOutputNamespacesPath(),
		SystemdUnitDir: s.userSystemdDir(),
		SystemctlArgs:  systemctlArgs,
	}
}

func (s *SiteServiceEnablerInstaller) templateData(scriptPath string) siteServiceEnablerData {
	wantedBy := "default.target"
	if s.uid == 0 {
		wantedBy = "multi-user.target"
	}
	return siteServiceEnablerData{
		ScriptPath: scriptPath,
		WantedBy:   wantedBy,
	}
}

func (s *SiteServiceEnablerInstaller) unitPath(unit string) string {
	return path.Join(s.userSystemdDir(), unit)
}

func (s *SiteServiceEnablerInstaller) userSystemdDir() string {
	if s.uid == 0 {
		return s.rootSystemdBasePath
	}
	return path.Join(api.GetConfigHome(), "systemd", "user")
}

func (s *SiteServiceEnablerInstaller) isRunning() bool {
	return s.systemctl("is-active", "--quiet", siteServiceEnablerServiceFile) == nil
}

func (s *SiteServiceEnablerInstaller) systemctl(args ...string) error {
	var fullArgs []string
	if s.uid != 0 {
		fullArgs = append(fullArgs, "--user")
	}
	fullArgs = append(fullArgs, args...)
	return s.command("systemctl", fullArgs...).Run()
}

func (s *SiteServiceEnablerInstaller) renderFile(tmplText string, data any, dst string, mode os.FileMode) error {
	tmpl, err := template.New("").Parse(tmplText)
	if err != nil {
		return err
	}
	var buf bytes.Buffer
	if err = tmpl.Execute(&buf, data); err != nil {
		return err
	}
	return os.WriteFile(dst, buf.Bytes(), mode)
}

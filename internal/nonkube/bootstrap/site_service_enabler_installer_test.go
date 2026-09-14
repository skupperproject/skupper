package bootstrap

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"gotest.tools/v3/assert"
)

func fakeCommand(calls *[][]string) func(string, ...string) *exec.Cmd {
	return func(name string, args ...string) *exec.Cmd {
		*calls = append(*calls, append([]string{name}, args...))
		return exec.CommandContext(context.Background(), "true")
	}
}

func fakeCommandNotRunning(calls *[][]string) func(string, ...string) *exec.Cmd {
	return func(name string, args ...string) *exec.Cmd {
		*calls = append(*calls, append([]string{name}, args...))
		for _, a := range args {
			if a == "is-active" {
				return exec.CommandContext(context.Background(), "false")
			}
		}
		return exec.CommandContext(context.Background(), "true")
	}
}

func newTestInstaller(t *testing.T, uid int, calls *[][]string) *SiteServiceEnablerInstaller {
	t.Helper()
	tmp := t.TempDir()

	t.Setenv("XDG_CONFIG_HOME", filepath.Join(tmp, "config"))
	return &SiteServiceEnablerInstaller{
		uid:                 uid,
		rootSystemdBasePath: filepath.Join(tmp, "etc", "systemd", "system"),
		scriptDir:           filepath.Join(tmp, "bin"),
		command:             fakeCommand(calls),
	}
}

func TestTemplateData_NonRoot(t *testing.T) {
	s := &SiteServiceEnablerInstaller{uid: 1000}
	d := s.templateData("/some/path/script")
	assert.Equal(t, d.ScriptPath, "/some/path/script")
	assert.Equal(t, d.WantedBy, "default.target")
}

func TestTemplateData_Root(t *testing.T) {
	s := &SiteServiceEnablerInstaller{uid: 0}
	d := s.templateData("/some/path/script")
	assert.Equal(t, d.WantedBy, "multi-user.target")
}

func TestUserSystemdDir_Root(t *testing.T) {
	s := &SiteServiceEnablerInstaller{uid: 0, rootSystemdBasePath: "/etc/systemd/system"}
	assert.Equal(t, s.userSystemdDir(), "/etc/systemd/system")
}

func TestUserSystemdDir_NonRoot(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "/fake/config")
	s := &SiteServiceEnablerInstaller{uid: 1000}
	got := s.userSystemdDir()
	assert.Assert(t, strings.HasPrefix(got, "/fake/config"), "expected XDG_CONFIG_HOME prefix, got %s", got)
}

func TestUnitPath(t *testing.T) {
	s := &SiteServiceEnablerInstaller{uid: 0, rootSystemdBasePath: "/etc/systemd/system"}
	got := s.unitPath("skupper-site-service-enabler.service")
	assert.Equal(t, got, "/etc/systemd/system/skupper-site-service-enabler.service")
}

func TestSystemctl_NonRoot_AddsUserFlag(t *testing.T) {
	var calls [][]string
	s := &SiteServiceEnablerInstaller{uid: 1000, command: fakeCommand(&calls)}
	err := s.systemctl("start", "foo.service")
	assert.NilError(t, err)
	assert.Equal(t, len(calls), 1)
	assert.DeepEqual(t, calls[0], []string{"systemctl", "--user", "start", "foo.service"})
}

func TestSystemctl_Root_NoUserFlag(t *testing.T) {
	var calls [][]string
	s := &SiteServiceEnablerInstaller{uid: 0, command: fakeCommand(&calls)}
	err := s.systemctl("start", "foo.service")
	assert.NilError(t, err)
	assert.Equal(t, len(calls), 1)
	assert.DeepEqual(t, calls[0], []string{"systemctl", "start", "foo.service"})
}

func TestRenderFile_ServiceTemplate(t *testing.T) {
	tmp := t.TempDir()
	dst := filepath.Join(tmp, "out.service")
	s := &SiteServiceEnablerInstaller{}
	err := s.renderFile(siteServiceEnablerServiceTemplate, siteServiceEnablerData{
		ScriptPath: "/usr/bin/skupper-site-service-enabler",
		WantedBy:   "multi-user.target",
	}, dst, 0644)
	assert.NilError(t, err)

	content, err := os.ReadFile(dst)
	assert.NilError(t, err)
	assert.Assert(t, strings.Contains(string(content), "ExecStart=/usr/bin/skupper-site-service-enabler"))
	assert.Assert(t, strings.Contains(string(content), "WantedBy=multi-user.target"))
}

func TestRenderFile_ScriptTemplate(t *testing.T) {
	tmp := t.TempDir()
	dst := filepath.Join(tmp, "out.sh")
	s := &SiteServiceEnablerInstaller{}
	err := s.renderFile(siteServiceEnablerScriptTemplate, siteServiceEnablerScriptData{
		NamespacesDir:  "/home/user/.local/share/skupper/namespaces",
		SystemdUnitDir: "/home/user/.config/systemd/user",
		SystemctlArgs:  "--user ",
	}, dst, 0755)
	assert.NilError(t, err)

	content, err := os.ReadFile(dst)
	assert.NilError(t, err)
	assert.Assert(t, strings.Contains(string(content), "NAMESPACES_DIR=\"/home/user/.local/share/skupper/namespaces\""))
	assert.Assert(t, strings.Contains(string(content), "UNIT_DIR=\"/home/user/.config/systemd/user\""))
	assert.Assert(t, strings.Contains(string(content), "SYSTEMCTL_ARGS=\"--user \""))
	assert.Assert(t, strings.Contains(string(content), "POLL_INTERVAL"))
}

func renderScript(t *testing.T) string {
	t.Helper()
	tmp := t.TempDir()
	dst := filepath.Join(tmp, "out.sh")
	s := &SiteServiceEnablerInstaller{}
	err := s.renderFile(siteServiceEnablerScriptTemplate, siteServiceEnablerScriptData{
		NamespacesDir:  "/ns",
		SystemdUnitDir: "/units",
		SystemctlArgs:  "",
	}, dst, 0755)
	assert.NilError(t, err)
	content, err := os.ReadFile(dst)
	assert.NilError(t, err)
	return string(content)
}

func TestScript_RestartsServiceOnUnitChange(t *testing.T) {
	body := renderScript(t)
	assert.Assert(t, strings.Contains(body, "changed=1"), "expected changed=1 inside change detection branch")
	assert.Assert(t, strings.Contains(body, `if [ "$changed" -eq 1 ]`), "expected conditional restart block")
	assert.Assert(t, strings.Contains(body, `systemctl_run restart "$svc"`), "expected restart via systemctl_run")
}

func TestScript_OwnershipMarkerAppendedToUnitCopy(t *testing.T) {
	body := renderScript(t)
	assert.Assert(t, strings.Contains(body, "OWNERSHIP_MARKER="), "expected OWNERSHIP_MARKER variable")
	assert.Assert(t, strings.Contains(body, "X-ManagedBy=skupper-site-service-enabler"), "expected X-ManagedBy marker text")
	assert.Assert(t, strings.Contains(body, `awk '/^\[Unit\]/`), "expected awk injection into [Unit] section")
	assert.Assert(t, strings.Contains(body, `printf '%s' "$owned" > "$dst"`), "expected owned content written to dst")
}

func TestScript_ListActiveUsesMarkerNotPrefix(t *testing.T) {
	body := renderScript(t)
	assert.Assert(t, strings.Contains(body, `grep -rl "${OWNERSHIP_MARKER}"`), "expected grep on ownership marker in list_active")
	assert.Assert(t, !strings.Contains(body, `find "${UNIT_DIR}" -maxdepth 1 -name "skupper-*.service"`), "must not use prefix-based find")
}

func TestScript_KeepsUnitFileWhenDisableFails(t *testing.T) {
	tmp := t.TempDir()
	dst := filepath.Join(tmp, "out.sh")
	s := &SiteServiceEnablerInstaller{}
	err := s.renderFile(siteServiceEnablerScriptTemplate, siteServiceEnablerScriptData{
		NamespacesDir:  "/ns",
		SystemdUnitDir: "/units",
		SystemctlArgs:  "",
	}, dst, 0755)
	assert.NilError(t, err)

	content, err := os.ReadFile(dst)
	assert.NilError(t, err)
	body := string(content)

	assert.Assert(t, strings.Contains(body, `if systemctl_run disable --now "$svc"`), "expected disable guarding rm")
	assert.Assert(t, strings.Contains(body, "rm -f"), "expected rm -f inside disable branch")
}

func TestRenderFile_InvalidTemplate(t *testing.T) {
	tmp := t.TempDir()
	s := &SiteServiceEnablerInstaller{}
	err := s.renderFile("{{.Invalid", siteServiceEnablerData{}, filepath.Join(tmp, "out.service"), 0644)
	assert.Assert(t, err != nil)
}

func TestInstall_CreatesWrapperScript(t *testing.T) {
	var calls [][]string
	s := newTestInstaller(t, 1000, &calls)
	s.command = fakeCommandNotRunning(&calls)

	err := s.Install()
	assert.NilError(t, err)

	scriptPath := filepath.Join(s.scriptDir, siteServiceEnablerWrapperScript)
	content, err := os.ReadFile(scriptPath)
	assert.NilError(t, err)

	assert.Assert(t, strings.HasPrefix(string(content), "#!/bin/sh"), "expected shell shebang")
	assert.Assert(t, strings.Contains(string(content), "POLL_INTERVAL"))

	info, err := os.Stat(scriptPath)
	assert.NilError(t, err)
	assert.Equal(t, info.Mode(), os.FileMode(0755))
}

func TestInstall_CreatesServiceFile(t *testing.T) {
	var calls [][]string
	s := newTestInstaller(t, 0, &calls)
	s.command = fakeCommandNotRunning(&calls)
	_ = os.MkdirAll(s.unitPath(""), 0755)

	err := s.Install()
	assert.NilError(t, err)

	svcPath := s.unitPath(siteServiceEnablerServiceFile)
	content, err := os.ReadFile(svcPath)
	assert.NilError(t, err)
	assert.Assert(t, strings.Contains(string(content), "ExecStart="))
	assert.Assert(t, strings.Contains(string(content), "WantedBy=multi-user.target"))
}

func TestInstall_SystemctlCallOrder(t *testing.T) {
	var calls [][]string
	s := newTestInstaller(t, 0, &calls)
	s.command = fakeCommandNotRunning(&calls)
	_ = os.MkdirAll(s.unitPath(""), 0755)

	err := s.Install()
	assert.NilError(t, err)

	assert.Equal(t, len(calls), 4)
	assert.DeepEqual(t, calls[0], []string{"systemctl", "is-active", "--quiet", siteServiceEnablerServiceFile})
	assert.DeepEqual(t, calls[1], []string{"systemctl", "daemon-reload"})
	assert.DeepEqual(t, calls[2], []string{"systemctl", "enable", siteServiceEnablerServiceFile})
	assert.DeepEqual(t, calls[3], []string{"systemctl", "start", siteServiceEnablerServiceFile})
}

func TestInstall_SkipsWhenAlreadyRunning(t *testing.T) {
	var calls [][]string
	s := newTestInstaller(t, 0, &calls)

	err := s.Install()
	assert.NilError(t, err)

	assert.Equal(t, len(calls), 1)
	assert.DeepEqual(t, calls[0], []string{"systemctl", "is-active", "--quiet", siteServiceEnablerServiceFile})
}

func TestInstall_NonRoot_SystemctlUsesUserFlag(t *testing.T) {
	var calls [][]string
	s := newTestInstaller(t, 1000, &calls)
	s.command = fakeCommandNotRunning(&calls)

	err := s.Install()
	assert.NilError(t, err)

	for _, c := range calls {
		assert.Equal(t, c[1], "--user", "expected --user flag in call %v", c)
	}
}

func TestInstall_ScriptDirCreationFailure(t *testing.T) {
	var calls [][]string
	s := newTestInstaller(t, 1000, &calls)
	s.command = fakeCommandNotRunning(&calls)
	blocker := s.scriptDir
	_ = os.MkdirAll(filepath.Dir(blocker), 0755)
	_ = os.WriteFile(blocker, []byte("block"), 0644)
	s.scriptDir = filepath.Join(blocker, "subdir")

	err := s.Install()
	assert.Assert(t, err != nil)
	assert.Assert(t, strings.Contains(err.Error(), "unable to create script directory"))
}

func TestRemove_SystemctlCallOrder(t *testing.T) {
	var calls [][]string
	s := newTestInstaller(t, 0, &calls)

	// Pre-create the unit file so stop/disable are attempted.
	svcPath := s.unitPath(siteServiceEnablerServiceFile)
	_ = os.MkdirAll(filepath.Dir(svcPath), 0755)
	_ = os.WriteFile(svcPath, []byte("[Unit]"), 0644)

	err := s.Remove()
	assert.NilError(t, err)

	assert.Equal(t, len(calls), 3)
	assert.DeepEqual(t, calls[0], []string{"systemctl", "stop", siteServiceEnablerServiceFile})
	assert.DeepEqual(t, calls[1], []string{"systemctl", "disable", siteServiceEnablerServiceFile})
	assert.DeepEqual(t, calls[2], []string{"systemctl", "daemon-reload"})
}

func TestRemove_DeletesFiles(t *testing.T) {
	var calls [][]string
	s := newTestInstaller(t, 0, &calls)

	svcPath := s.unitPath(siteServiceEnablerServiceFile)
	_ = os.MkdirAll(filepath.Dir(svcPath), 0755)
	_ = os.WriteFile(svcPath, []byte("[Unit]"), 0644)

	scriptPath := filepath.Join(s.scriptDir, siteServiceEnablerWrapperScript)
	_ = os.MkdirAll(s.scriptDir, 0755)
	_ = os.WriteFile(scriptPath, []byte("#!/bin/sh"), 0755)

	err := s.Remove()
	assert.NilError(t, err)

	_, err = os.Stat(svcPath)
	assert.Assert(t, os.IsNotExist(err), "service file should be removed")

	_, err = os.Stat(scriptPath)
	assert.Assert(t, os.IsNotExist(err), "wrapper script should be removed")
}

func TestRemove_NonRoot_SystemctlUsesUserFlag(t *testing.T) {
	var calls [][]string
	s := newTestInstaller(t, 1000, &calls)

	// Pre-create the unit file so stop/disable are attempted.
	svcPath := s.unitPath(siteServiceEnablerServiceFile)
	_ = os.MkdirAll(filepath.Dir(svcPath), 0755)
	_ = os.WriteFile(svcPath, []byte("[Unit]"), 0644)

	err := s.Remove()
	assert.NilError(t, err)

	for _, c := range calls {
		assert.Equal(t, c[1], "--user", "expected --user flag in call %v", c)
	}
}

func TestRemove_ToleratesMissingFiles(t *testing.T) {
	var calls [][]string
	s := newTestInstaller(t, 0, &calls)
	err := s.Remove()
	assert.NilError(t, err)
	// Unit absent: only daemon-reload is called.
	assert.Equal(t, len(calls), 1)
	assert.DeepEqual(t, calls[0], []string{"systemctl", "daemon-reload"})
}

func TestRemove_SkipsStopDisableWhenUnitAbsent(t *testing.T) {
	var calls [][]string
	s := newTestInstaller(t, 0, &calls)

	err := s.Remove()
	assert.NilError(t, err)

	// stop and disable must not have been called; only daemon-reload.
	for _, c := range calls {
		assert.Assert(t, c[1] != "stop", "stop must not be called when unit is absent")
		assert.Assert(t, c[1] != "disable", "disable must not be called when unit is absent")
	}
}

func TestRemove_FailsOnStopError(t *testing.T) {
	var calls [][]string
	s := newTestInstaller(t, 0, &calls)
	s.command = func(name string, args ...string) *exec.Cmd {
		calls = append(calls, append([]string{name}, args...))
		for _, a := range args {
			if a == "stop" {
				return exec.CommandContext(context.Background(), "false")
			}
		}
		return exec.CommandContext(context.Background(), "true")
	}

	// Pre-create the unit file so the stop branch is entered.
	svcPath := s.unitPath(siteServiceEnablerServiceFile)
	_ = os.MkdirAll(filepath.Dir(svcPath), 0755)
	_ = os.WriteFile(svcPath, []byte("[Unit]"), 0644)

	err := s.Remove()
	assert.Assert(t, err != nil)
	assert.Assert(t, strings.Contains(err.Error(), "failed to stop"))
	// disable and daemon-reload must not have been called
	assert.Equal(t, len(calls), 1)
}

func TestRemove_FailsOnDisableError(t *testing.T) {
	var calls [][]string
	s := newTestInstaller(t, 0, &calls)
	s.command = func(name string, args ...string) *exec.Cmd {
		calls = append(calls, append([]string{name}, args...))
		for _, a := range args {
			if a == "disable" {
				return exec.CommandContext(context.Background(), "false")
			}
		}
		return exec.CommandContext(context.Background(), "true")
	}

	// Pre-create the unit file so the disable branch is entered.
	svcPath := s.unitPath(siteServiceEnablerServiceFile)
	_ = os.MkdirAll(filepath.Dir(svcPath), 0755)
	_ = os.WriteFile(svcPath, []byte("[Unit]"), 0644)

	err := s.Remove()
	assert.Assert(t, err != nil)
	assert.Assert(t, strings.Contains(err.Error(), "failed to disable"))
	// daemon-reload must not have been called
	assert.Equal(t, len(calls), 2)
}

func TestRemove_FailsOnUnitFileRemoveError(t *testing.T) {
	var calls [][]string
	s := newTestInstaller(t, 0, &calls)
	s.command = fakeCommand(&calls)

	unitPath := s.unitPath(siteServiceEnablerServiceFile)
	_ = os.MkdirAll(unitPath, 0755)
	_ = os.WriteFile(filepath.Join(unitPath, "child"), []byte("x"), 0644)

	err := s.Remove()
	assert.Assert(t, err != nil)
	assert.Assert(t, strings.Contains(err.Error(), "failed to remove unit file"))
}

func TestRemove_FailsOnDaemonReloadError(t *testing.T) {
	var calls [][]string
	s := newTestInstaller(t, 0, &calls)
	s.command = func(name string, args ...string) *exec.Cmd {
		calls = append(calls, append([]string{name}, args...))
		for _, a := range args {
			if a == "daemon-reload" {
				return exec.CommandContext(context.Background(), "false")
			}
		}
		return exec.CommandContext(context.Background(), "true")
	}

	err := s.Remove()
	assert.Assert(t, err != nil)
	assert.Assert(t, strings.Contains(err.Error(), "daemon-reload failed after remove"))
}

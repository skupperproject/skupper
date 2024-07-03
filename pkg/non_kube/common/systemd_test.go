package common

import (
	"fmt"
	"os"
	"os/exec"
	"path"
	"strings"
	"testing"

	"github.com/skupperproject/skupper/pkg/utils"
	"gotest.tools/assert"
)

func TestSystemdService(t *testing.T) {
	outputPathOrig := os.Getenv("OUTPUT_PATH")
	xdgConfigHomeOrig := os.Getenv("XDG_CONFIG_HOME")
	platformOrig := os.Getenv("SKUPPER_PLATFORM")
	defer func() {
		_ = os.Setenv("OUTPUT_PATH", outputPathOrig)
		_ = os.Setenv("XDG_CONFIG_HOME", xdgConfigHomeOrig)
		_ = os.Setenv("SKUPPER_PLATFORM", platformOrig)
	}()
	siteState := fakeSiteState()

	outputPath := t.TempDir()
	assert.Assert(t, os.MkdirAll(outputPath, 0755))
	assert.Assert(t, os.Setenv("OUTPUT_PATH", outputPath))
	assert.Assert(t, os.Setenv("XDG_CONFIG_HOME", outputPath))

	for _, platform := range []string{"systemd", "podman", "docker"} {
		for _, uid := range []int{0, 1000} {
			assert.Assert(t, os.Setenv("SKUPPER_PLATFORM", platform))
			systemdService, err := NewSystemdServiceInfo(siteState.Site)
			assert.Assert(t, err)
			assert.Equal(t, systemdService.GetServiceName(), "skupper-site-site-name.service")
			systemdServiceImpl := systemdService.(*systemdServiceInfo)
			assert.Equal(t, systemdServiceImpl.SiteScriptPath, path.Join(outputPath, "sites/site-name/runtime/scripts"))
			assert.Equal(t, systemdServiceImpl.SiteConfigPath, path.Join(outputPath, "sites/site-name/config/router"))
			systemdServiceImpl.command = func(name string, arg ...string) *exec.Cmd {
				assert.Assert(t, utils.StringSliceContains(arg, "--user") == (uid != 0))
				t.Logf("mocking command: %s %s", name, strings.Join(arg, " "))
				return exec.Command("echo", "mock")
			}
			systemdServiceImpl.getUid = func() int {
				return uid
			}
			systemdServiceImpl.rootSystemdBasePath = outputPath
			t.Run(fmt.Sprintf("create-systemd-%s-as-uid-%d", platform, uid), func(t *testing.T) {
				assert.Assert(t, systemdService.Create())
				serviceFile, err := os.ReadFile(systemdServiceImpl.getServiceFile())
				assert.Assert(t, err)
				var startCmd string
				var stopCmd string
				switch platform {
				case "systemd":
					startCmd = fmt.Sprintf("ExecStart=skrouterd -c %s/skrouterd.json", systemdServiceImpl.SiteConfigPath)
					stopCmd = ""
				default:
					startCmd = fmt.Sprintf("ExecStart=/bin/bash %s/start.sh", systemdServiceImpl.SiteScriptPath)
					stopCmd = fmt.Sprintf("ExecStop=/bin/bash %s/stop.sh", systemdServiceImpl.SiteScriptPath)
				}
				assert.Assert(t, strings.Contains(string(serviceFile), startCmd))
				assert.Assert(t, strings.Contains(string(serviceFile), stopCmd))
			})
			assert.Assert(t, systemdService.Remove())
			_, err = os.ReadFile(systemdServiceImpl.getServiceFile())
			assert.Assert(t, err != nil)
		}
	}
}

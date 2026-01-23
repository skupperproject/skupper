package common

import (
	"fmt"
	"os"
	"os/exec"
	"path"
	"slices"
	"strings"
	"testing"

	"github.com/skupperproject/skupper/pkg/nonkube/api"
	"gotest.tools/v3/assert"
)

func TestSystemdService(t *testing.T) {
	outputPathOrig := os.Getenv("SKUPPER_OUTPUT_PATH")
	xdgConfigHomeOrig := os.Getenv("XDG_CONFIG_HOME")
	platformOrig := os.Getenv("SKUPPER_PLATFORM")
	defer func() {
		t.Setenv("SKUPPER_OUTPUT_PATH", outputPathOrig)
		t.Setenv("XDG_CONFIG_HOME", xdgConfigHomeOrig)
		t.Setenv("SKUPPER_PLATFORM", platformOrig)
	}()
	siteState := fakeSiteState()

	outputPath := t.TempDir()
	assert.Assert(t, os.MkdirAll(outputPath, 0755))
	t.Setenv("SKUPPER_OUTPUT_PATH", outputPath)
	t.Setenv("XDG_CONFIG_HOME", outputPath)

	for _, platform := range []string{"linux", "podman", "docker"} {
		for _, uid := range []int{0, 1000} {
			//assert.Assert(t, t.Setenv("SKUPPER_PLATFORM", platform))
			systemdService, err := NewSystemdServiceInfo(siteState, platform)
			assert.Assert(t, err)
			assert.Equal(t, systemdService.GetServiceName(), "skupper-default.service")
			systemdServiceImpl := systemdService.(*systemdServiceInfo)
			assert.Equal(t, systemdServiceImpl.SiteScriptPath, path.Join(outputPath, "namespaces/default", string(api.ScriptsPath)))
			assert.Equal(t, systemdServiceImpl.SiteConfigPath, path.Join(outputPath, "namespaces/default", string(api.RouterConfigPath)))
			systemdServiceImpl.command = func(name string, arg ...string) *exec.Cmd {
				assert.Assert(t, slices.Contains(arg, "--user") == (uid != 0))
				t.Logf("mocking command: %s %s", name, strings.Join(arg, " "))
				return exec.Command("echo", "mock")
			}
			systemdServiceImpl.getUid = func() int {
				return uid
			}
			systemdServiceImpl.rootSystemdBasePath = outputPath
			t.Run(fmt.Sprintf("create-systemd-%s-as-uid-%d", platform, uid), func(t *testing.T) {
				assert.Assert(t, systemdService.Create())
				serviceFile, err := os.ReadFile(systemdServiceImpl.GetServiceFile())
				assert.Assert(t, err)
				var startCmd string
				var stopCmd string
				switch platform {
				case "linux":
					startCmd = fmt.Sprintf("ExecStart=skrouterd -c %s/skrouterd.json", systemdServiceImpl.SiteConfigPath)
					stopCmd = ""
					assert.Assert(t, strings.Contains(string(serviceFile), `Environment="SKUPPER_SITE_ID=site-id"`), string(serviceFile))
				default:
					startCmd = fmt.Sprintf("ExecStart=/bin/bash %s/start.sh", systemdServiceImpl.SiteScriptPath)
					stopCmd = fmt.Sprintf("ExecStop=/bin/bash %s/stop.sh", systemdServiceImpl.SiteScriptPath)
				}
				assert.Assert(t, strings.Contains(string(serviceFile), startCmd))
				assert.Assert(t, strings.Contains(string(serviceFile), stopCmd))
			})
			assert.Assert(t, systemdService.Remove())
			_, err = os.ReadFile(systemdServiceImpl.GetServiceFile())
			assert.Assert(t, err != nil)
		}
	}
}

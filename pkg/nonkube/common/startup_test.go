package common

import (
	"fmt"
	"os"
	"path"
	"strings"
	"testing"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/pkg/nonkube/api"
	"gotest.tools/v3/assert"
)

func TestGetStartupScripts(t *testing.T) {
	xdgDataHomeOrig := os.Getenv("XDG_DATA_HOME")
	skupperPlatformOrig := os.Getenv("SKUPPER_PLATFORM")
	defer func() {
		t.Setenv("XDG_DATA_HOME", xdgDataHomeOrig)
		t.Setenv("SKUPPER_PLATFORM", skupperPlatformOrig)
	}()

	siteState := fakeSiteState()
	tempDir := t.TempDir()
	t.Setenv("XDG_DATA_HOME", tempDir)

	for _, platform := range []string{"podman", "docker"} {
		t.Run("platform-"+platform, func(t *testing.T) {
			t.Setenv("SKUPPER_PLATFORM", platform)
			startupArgs := StartupScriptsArgs{
				Namespace: siteState.GetNamespace(),
				SiteId:    siteState.SiteId,
				Platform:  types.Platform(platform),
				Bundle:    false,
			}
			scripts, err := GetStartupScripts(startupArgs, api.GetInternalOutputPath)
			assert.Assert(t, err)
			assert.Assert(t, scripts != nil)
			assert.Assert(t, os.MkdirAll(scripts.GetPath(), 0755))
			assert.Assert(t, scripts.Create())
			_, err = os.ReadDir(scripts.GetPath())
			assert.Assert(t, err)
			startSh, err := os.ReadFile(path.Join(scripts.GetPath(), "start.sh"))
			assert.Assert(t, err)
			assert.Assert(t, strings.Contains(string(startSh), fmt.Sprintf("%s container ls", platform)))
			assert.Assert(t, strings.Contains(string(startSh), fmt.Sprintf("%s start", platform)))
			assert.Assert(t, strings.Contains(string(startSh), fmt.Sprintf("skupper.io/site-id=%s", siteState.SiteId)))
			stopSh, err := os.ReadFile(path.Join(scripts.GetPath(), "stop.sh"))
			assert.Assert(t, strings.Contains(string(stopSh), fmt.Sprintf("%s container ls", platform)))
			assert.Assert(t, strings.Contains(string(stopSh), fmt.Sprintf("%s stop", platform)))
			assert.Assert(t, strings.Contains(string(stopSh), fmt.Sprintf("skupper.io/site-id=%s", siteState.SiteId)))
			assert.Assert(t, err)
			scripts.Remove()
		})
	}

	t.Run("invalid-platform", func(t *testing.T) {
		startupArgs := StartupScriptsArgs{
			Namespace: siteState.GetNamespace(),
			SiteId:    siteState.SiteId,
			Platform:  types.PlatformKubernetes,
		}
		scripts, err := GetStartupScripts(startupArgs, api.GetInternalOutputPath)
		assert.ErrorContains(t, err, "startup scripts can only be used with podman or docker platforms")
		assert.Assert(t, scripts == nil)
	})

}

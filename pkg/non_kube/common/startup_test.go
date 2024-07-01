package common

import (
	"fmt"
	"os"
	"path"
	"strings"
	"testing"

	"gotest.tools/assert"
)

func TestGetStartupScripts(t *testing.T) {
	xdgDataHomeOrig := os.Getenv("XDG_DATA_HOME")
	skupperPlatformOrig := os.Getenv("SKUPPER_PLATFORM")
	defer func() {
		_ = os.Setenv("XDG_DATA_HOME", xdgDataHomeOrig)
		_ = os.Setenv("SKUPPER_PLATFORM", skupperPlatformOrig)
	}()

	siteState := fakeSiteState()
	tempDir := t.TempDir()
	os.Setenv("XDG_DATA_HOME", tempDir)

	for _, platform := range []string{"podman", "docker"} {
		t.Run("platform-"+platform, func(t *testing.T) {
			os.Setenv("SKUPPER_PLATFORM", platform)
			scripts, err := GetStartupScripts(siteState.Site, siteState.SiteId)
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
		os.Setenv("SKUPPER_PLATFORM", "kubernetes")
		scripts, err := GetStartupScripts(siteState.Site, siteState.SiteId)
		assert.ErrorContains(t, err, "startup scripts can only be used with podman or docker platforms")
		assert.Assert(t, scripts == nil)
	})

}

package api

import (
	"os"
	"path"
	"testing"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/internal/config"
	"github.com/skupperproject/skupper/pkg/apis/skupper/v2alpha1"
	"gotest.tools/v3/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestGetConfigHome(t *testing.T) {
	const XDG_CONFIG_HOME = "XDG_CONFIG_HOME"
	const HOME = "HOME"

	tests := []struct {
		name          string
		want          string
		homeDir       string
		xdgConfigHome string
	}{
		{
			name:          "xdg-config-home-unset",
			want:          "/home/skupper/.config",
			homeDir:       "/home/skupper",
			xdgConfigHome: "",
		},
		{
			name:          "xdg-config-home-set",
			want:          "/home/skupper/.custom",
			homeDir:       "/home/skupper",
			xdgConfigHome: "/home/skupper/.custom",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var xdgConfigHomeOrig = os.Getenv(XDG_CONFIG_HOME)
			var homeOrig = os.Getenv(HOME)

			if tt.xdgConfigHome != "" {
				t.Setenv(XDG_CONFIG_HOME, tt.xdgConfigHome)
			}
			if tt.homeDir != "" {
				t.Setenv(HOME, tt.homeDir)
			}
			got := GetConfigHome()
			t.Setenv(XDG_CONFIG_HOME, xdgConfigHomeOrig)
			t.Setenv(HOME, homeOrig)
			if got != tt.want {
				t.Errorf("GetConfigHome() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetDataHome(t *testing.T) {
	const XDG_DATA_HOME = "XDG_DATA_HOME"
	const HOME = "HOME"

	tests := []struct {
		name        string
		want        string
		homeDir     string
		xdgDataHome string
	}{
		{
			name:        "xdg-data-home-unset",
			want:        "/home/skupper/.local/share/skupper",
			homeDir:     "/home/skupper",
			xdgDataHome: "",
		},
		{
			name:        "xdg-data-home-set",
			want:        "/home/skupper/.custom/skupper",
			homeDir:     "/home/skupper",
			xdgDataHome: "/home/skupper/.custom",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var xdgDataHomeOrig = os.Getenv(XDG_DATA_HOME)
			var homeOrig = os.Getenv(HOME)

			if tt.xdgDataHome != "" {
				t.Setenv(XDG_DATA_HOME, tt.xdgDataHome)
			}
			if tt.homeDir != "" {
				t.Setenv(HOME, tt.homeDir)
			}
			got := GetDataHome()
			t.Setenv(XDG_DATA_HOME, xdgDataHomeOrig)
			t.Setenv(HOME, homeOrig)
			if got != tt.want {
				t.Errorf("GetDataHome() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetPlatform(t *testing.T) {
	tests := []struct {
		name   string
		want   types.Platform
		cliVar string
		envVar string
	}{
		{
			name:   "default-as-nothing-set",
			want:   "kubernetes",
			cliVar: "",
			envVar: "",
		},
		{
			name:   "podman-as-cli-flag-set",
			want:   "podman",
			cliVar: "podman",
			envVar: "",
		},
		{
			name:   "podman-as-cli-flag-set-highest-precedence",
			want:   "podman",
			cliVar: "podman",
			envVar: "kubernetes",
		},
		{
			name:   "podman-as-envvar-set",
			want:   "podman",
			cliVar: "",
			envVar: "podman",
		},
		{
			name:   "podman-as-envvar-set-over-config",
			want:   "podman",
			cliVar: "",
			envVar: "podman",
		},
	}
	// Saving original values
	cliOrig := config.Platform
	envOrig := os.Getenv(types.ENV_PLATFORM)
	// Restore original values
	defer func() {
		config.Platform = cliOrig
		t.Setenv(types.ENV_PLATFORM, envOrig)
	}()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config.Platform = tt.cliVar
			t.Setenv(types.ENV_PLATFORM, tt.envVar)
			got := config.GetPlatform()
			if got != tt.want {
				t.Errorf("GetPlatform() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetHostSiteHome(t *testing.T) {
	fakeSite := &v2alpha1.Site{
		ObjectMeta: metav1.ObjectMeta{
			Name: "my-test-site",
		},
		Spec: v2alpha1.SiteSpec{},
	}
	homeDir, err := os.UserHomeDir()
	assert.Assert(t, err)
	defaultSiteHome := path.Join(homeDir, ".local/share/skupper/namespaces/default")
	const fakeXdgDataHome = "/fake/xdg/home"
	xdgSiteHome := path.Join(fakeXdgDataHome, "/skupper/namespaces/default")

	envXdgDataHome := "XDG_DATA_HOME"
	originalXdgDataHome := os.Getenv(envXdgDataHome)
	defer func() {
		t.Setenv(envXdgDataHome, originalXdgDataHome)
	}()
	for _, scenario := range []struct {
		expectedSiteHome string
		useXdgDataHome   bool
	}{
		{
			expectedSiteHome: defaultSiteHome,
			useXdgDataHome:   false,
		},
		{
			expectedSiteHome: xdgSiteHome,
			useXdgDataHome:   true,
		},
	} {
		if scenario.useXdgDataHome {
			t.Setenv(envXdgDataHome, fakeXdgDataHome)
		}
		siteHome := GetHostSiteHome(fakeSite)
		assert.Equal(t, siteHome, scenario.expectedSiteHome)
	}
}

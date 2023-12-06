package config

import (
	"bytes"
	"fmt"
	"os"
	"reflect"
	"testing"

	"github.com/skupperproject/skupper/api/types"
	"gopkg.in/yaml.v3"
	"gotest.tools/assert"
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
				_ = os.Setenv(XDG_CONFIG_HOME, tt.xdgConfigHome)
			}
			if tt.homeDir != "" {
				_ = os.Setenv(HOME, tt.homeDir)
			}
			got := GetConfigHome()
			_ = os.Setenv(XDG_CONFIG_HOME, xdgConfigHomeOrig)
			_ = os.Setenv(HOME, homeOrig)
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
				_ = os.Setenv(XDG_DATA_HOME, tt.xdgDataHome)
			}
			if tt.homeDir != "" {
				_ = os.Setenv(HOME, tt.homeDir)
			}
			got := GetDataHome()
			_ = os.Setenv(XDG_DATA_HOME, xdgDataHomeOrig)
			_ = os.Setenv(HOME, homeOrig)
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
		cfgVal types.Platform
	}{
		{
			name:   "default-as-nothing-set",
			want:   "kubernetes",
			cliVar: "",
			envVar: "",
			cfgVal: "",
		},
		{
			name:   "podman-as-cli-flag-set",
			want:   "podman",
			cliVar: "podman",
			envVar: "",
			cfgVal: "",
		},
		{
			name:   "podman-as-cli-flag-set-highest-precedence",
			want:   "podman",
			cliVar: "podman",
			envVar: "kubernetes",
			cfgVal: types.PlatformKubernetes,
		},
		{
			name:   "podman-as-envvar-set",
			want:   "podman",
			cliVar: "",
			envVar: "podman",
			cfgVal: "",
		},
		{
			name:   "podman-as-envvar-set-over-config",
			want:   "podman",
			cliVar: "",
			envVar: "podman",
			cfgVal: types.PlatformKubernetes,
		},
		{
			name:   "podman-as-config-set",
			want:   "podman",
			cliVar: "",
			envVar: "",
			cfgVal: types.PlatformPodman,
		},
	}
	// Saving original values
	cliOrig := Platform
	envOrig := os.Getenv(types.ENV_PLATFORM)
	cfgFileOrig := PlatformConfigFile
	// Restore original values
	defer func() {
		Platform = cliOrig
		_ = os.Setenv(types.ENV_PLATFORM, envOrig)
		PlatformConfigFile = cfgFileOrig
	}()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			Platform = tt.cliVar
			_ = os.Setenv(types.ENV_PLATFORM, tt.envVar)

			// creating a temporary platform file
			f, err := os.CreateTemp(os.TempDir(), "platform-*.yaml")
			_ = f.Close()
			assert.Assert(t, err, "unable to create temporary platform file")
			PlatformConfigFile = f.Name()
			if tt.cfgVal == "" {
				_ = os.Remove(f.Name())
			} else {
				info := &PlatformInfo{}
				assert.Assert(t, info.Update(tt.cfgVal), "error setting platform in %s", f.Name())
			}

			got := GetPlatform()

			// removing temporary file
			if tt.cfgVal != "" {
				_ = os.Remove(f.Name())
			}
			if got != tt.want {
				t.Errorf("GetPlatform() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPlatformInfo_Load(t *testing.T) {
	tests := []struct {
		name          string
		existing      *PlatformInfo
		want          *PlatformInfo
		wantReadErr   bool
		wantDecodeErr bool
	}{
		{
			name:     "default-no-config-file",
			existing: nil,
			want:     &PlatformInfo{},
		},
		{
			name:        "error-no-permission",
			existing:    nil,
			want:        &PlatformInfo{},
			wantReadErr: true,
		},
		{
			name:          "error-invalid-content",
			existing:      &PlatformInfo{},
			want:          &PlatformInfo{},
			wantDecodeErr: true,
		},
		{
			name: "valid-config-file",
			existing: &PlatformInfo{
				Current:  types.PlatformPodman,
				Previous: types.PlatformKubernetes,
			},
			want: &PlatformInfo{
				Current:  types.PlatformPodman,
				Previous: types.PlatformKubernetes,
			},
		},
	}
	// Saving original values
	cfgFileOrig := PlatformConfigFile
	// Restore original values
	defer func() {
		PlatformConfigFile = cfgFileOrig
	}()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f, err := os.CreateTemp(os.TempDir(), "platform-*.yaml")
			tempFile := f.Name()
			assert.Assert(t, err, "error creating temporary platform config file")
			tempFile = f.Name()
			defer func(name string) {
				_ = os.Remove(name)
			}(tempFile)
			PlatformConfigFile = tempFile
			if !tt.wantReadErr {
				_ = os.Remove(tempFile)
			} else {
				_ = os.Chmod(tempFile, 0)
			}
			if tt.existing == nil {
				tt.existing = &PlatformInfo{}
			} else if !tt.wantReadErr {
				// populating temp file
				info := tt.existing
				var data string
				if !tt.wantDecodeErr {
					data = fmt.Sprintf("current: %s\nprevious: %s\n", info.Current, info.Previous)
				} else {
					data = `invalid content`
				}
				assert.Assert(t, os.WriteFile(tempFile, []byte(data), 0644), "error writing to temporary file")
			}
			wantErr := tt.wantReadErr || tt.wantDecodeErr
			if err := tt.existing.Load(); (err != nil) != wantErr {
				t.Errorf("Load() error = %v, wantReadErr %v", err, tt.wantReadErr)
			}
			assert.Assert(t, reflect.DeepEqual(tt.existing, tt.want), "want = %v, got = %v", tt.want, tt.existing)
		})
	}
}

func TestPlatformInfo_Update(t *testing.T) {
	tests := []struct {
		name         string
		existing     *PlatformInfo
		platform     types.Platform
		want         *PlatformInfo
		wantWriteErr bool
		wantReadErr  bool
	}{
		{
			name:     "new-config-file",
			existing: nil,
			platform: types.PlatformPodman,
			want: &PlatformInfo{
				Current:  types.PlatformPodman,
				Previous: types.PlatformPodman,
			},
		},
		{
			name: "config-file-changed",
			existing: &PlatformInfo{
				Current:  types.PlatformKubernetes,
				Previous: types.PlatformKubernetes,
			},
			platform: types.PlatformPodman,
			want: &PlatformInfo{
				Current:  types.PlatformPodman,
				Previous: types.PlatformKubernetes,
			},
		},
		{
			name: "error-no-write-permission",
			existing: &PlatformInfo{
				Current:  types.PlatformKubernetes,
				Previous: types.PlatformKubernetes,
			},
			platform: types.PlatformPodman,
			want: &PlatformInfo{
				Current:  types.PlatformKubernetes,
				Previous: types.PlatformKubernetes,
			},
			wantWriteErr: true,
		},
		{
			name: "error-no-read-permission",
			existing: &PlatformInfo{
				Current:  types.PlatformKubernetes,
				Previous: types.PlatformKubernetes,
			},
			platform:    types.PlatformPodman,
			want:        &PlatformInfo{},
			wantReadErr: true,
		},
	}
	// Saving original values
	cfgFileOrig := PlatformConfigFile
	// Restore original values
	defer func() {
		PlatformConfigFile = cfgFileOrig
	}()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := os.TempDir()
			f, err := os.CreateTemp(tempDir, "platform-*.yaml")
			assert.Assert(t, err, "error creating temporary platform config file")
			_ = f.Close()
			tempFile := f.Name()
			defer func(name string) {
				_ = os.Remove(name)
			}(tempFile)
			PlatformConfigFile = tempFile
			if tt.existing == nil {
				_ = os.Remove(tempFile)
				tt.existing = &PlatformInfo{}
			} else {
				data := fmt.Sprintf("current: %s\nprevious: %s\n", tt.existing.Current, tt.existing.Previous)
				assert.Assert(t, os.WriteFile(tempFile, []byte(data), 0644), "error preparing platform config file")
			}

			if tt.wantWriteErr {
				_ = os.Chmod(tempFile, 0444)
			} else if tt.wantReadErr {
				_ = os.Chmod(tempFile, 0)
			}

			wantErr := tt.wantWriteErr || tt.wantReadErr
			if err := tt.existing.Update(tt.platform); (err != nil) != wantErr {
				t.Errorf("Update() error = %v, wantErr %v", err, wantErr)
			}

			// loading file
			if tt.want != nil {
				data, _ := os.ReadFile(tempFile)
				decoder := yaml.NewDecoder(bytes.NewReader(data))
				current := &PlatformInfo{}
				_ = decoder.Decode(current)
				assert.Assert(t, reflect.DeepEqual(tt.want, current), "want = %v, got = %v", tt.want, current)
			}
		})
	}
}

package apis

import (
	"fmt"
	"os"
	"path"

	"github.com/skupperproject/skupper/pkg/apis/skupper/v1alpha1"
)

type IdGetter func() int

var getUid IdGetter = os.Getuid

func GetDataHome() string {
	if getUid() == 0 {
		return "/usr/local/share/skupper"
	}
	dataHome, ok := os.LookupEnv("XDG_DATA_HOME")
	if !ok {
		homeDir, _ := os.UserHomeDir()
		dataHome = homeDir + "/.local/share"
	}
	return path.Join(dataHome, "skupper")
}

func GetConfigHome() string {
	configHome, ok := os.LookupEnv("XDG_CONFIG_HOME")
	if !ok {
		homeDir, _ := os.UserHomeDir()
		return homeDir + "/.config"
	} else {
		return configHome
	}
}

func GetRuntimeDir() string {
	if getUid() == 0 {
		return "/run"
	}
	runtimeDir, ok := os.LookupEnv("XDG_RUNTIME_DIR")
	if !ok {
		runtimeDir = fmt.Sprintf("/run/user/%d", getUid())
	}
	return runtimeDir
}

// GetHostDataHome returns the value of the SKUPPER_OUTPUT_PATH environment
// variable when running via container or the result of GetDataHome() otherwise.
// This is only useful during the bootstrap process.
func GetHostDataHome() (string, error) {
	// If container provides SKUPPER_OUTPUT_PATH use it
	if os.Getenv("SKUPPER_OUTPUT_PATH") != "" {
		return os.Getenv("SKUPPER_OUTPUT_PATH"), nil
	}
	return GetDataHome(), nil
}

func GetHostSiteHome(site *v1alpha1.Site) (string, error) {
	dataHome, err := GetHostDataHome()
	if err != nil {
		return "", err
	}
	return path.Join(dataHome, "sites", site.Name), nil
}

func IsRunningInContainer() bool {
	// See: https://docs.podman.io/en/latest/markdown/podman-run.1.html
	for _, file := range []string{"/run/.containerenv", "/.dockerenv"} {
		if _, err := os.Stat(file); err == nil {
			return true
		}
	}
	return false
}

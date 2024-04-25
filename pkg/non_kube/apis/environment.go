package apis

import (
	"fmt"
	"os"
	"path"

	"github.com/prometheus/procfs"
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

// GetHostDataHome returns the root of the /output mount point
// or the value of the OUTPUT_PATH environment variable when
// running via container or the result of GetDataHome() otherwise.
// This is only useful during the bootstrap process.
func GetHostDataHome() (string, error) {
	// If container provides OUTPUT_PATH use it
	if os.Getenv("OUTPUT_PATH") != "" {
		return os.Getenv("OUTPUT_PATH"), nil
	}
	if IsRunningInContainer() {
		mounts, err := procfs.GetProcMounts(1)
		if err != nil {
			return "", fmt.Errorf("error getting mount points: %v", err)
		}
		// TODO today the mountinfo does not return the proper root location
		//      when running as the root user, it shows (/root/root/...)
		for _, mount := range mounts {
			if mount.MountPoint == "/output" {
				return mount.Root, nil
			}
		}
		return "", fmt.Errorf("unable to determine host data home directory")
	} else {
		return GetDataHome(), nil
	}
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
	if _, err := os.Stat("/run/.containerenv"); err == nil {
		return true
	}
	if _, err := os.Stat("/.dockerenv"); err == nil {
		return true
	}
	return false
}

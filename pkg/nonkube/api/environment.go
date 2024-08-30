package api

import (
	"fmt"
	"os"
	"path"

	"github.com/skupperproject/skupper/pkg/apis/skupper/v1alpha1"
)

type InternalPath string
type InternalPathProvider func(namespace string, internalPath InternalPath) string

const (
	ConfigRouterPath       InternalPath = "config/router"
	CertificatesCaPath     InternalPath = "certificates/ca"
	CertificatesClientPath InternalPath = "certificates/client"
	CertificatesServerPath InternalPath = "certificates/server"
	CertificatesLinkPath   InternalPath = "certificates/link"
	LoadedSiteStatePath    InternalPath = "sources"
	RuntimeSiteStatePath   InternalPath = "runtime/state"
	RuntimeTokenPath       InternalPath = "runtime/link"
	RuntimeScriptsPath     InternalPath = "runtime/scripts"
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
	ns := site.Namespace
	if ns == "" {
		ns = "default"
	}
	return path.Join(dataHome, "namespaces", ns), nil
}

func GetHostNamespacesPath() (string, error) {
	return getHostPath("namespaces")
}

func GetHostBundlesPath() (string, error) {
	return getHostPath("bundles")
}

func getHostPath(basePath string) (string, error) {
	dataHome, err := GetHostDataHome()
	if err != nil {
		return "", err
	}
	return path.Join(dataHome, basePath), nil
}

func GetCustomSiteHome(site *v1alpha1.Site, customBaseDir string) string {
	return getCustomSiteHome(site, customBaseDir, "namespaces")
}

func GetCustomBundleHome(site *v1alpha1.Site, customBaseDir string) string {
	return getCustomSiteHome(site, customBaseDir, "bundles")
}

func getCustomSiteHome(site *v1alpha1.Site, customBaseDir string, basePath string) string {
	ns := site.Namespace
	if ns == "" {
		ns = "default"
	}
	return path.Join(customBaseDir, basePath, ns)
}

func GetHostSiteInternalPath(site *v1alpha1.Site, internalPath InternalPath) (string, error) {
	dataHome, err := GetHostSiteHome(site)
	if err != nil {
		return "", err
	}
	return path.Join(dataHome, string(internalPath)), nil
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

func GetDefaultOutputPath(namespace string) string {
	return getDefaultOutputPath(namespace, false)
}

func GetDefaultBundleOutputPath(namespace string) string {
	return getDefaultOutputPath(namespace, true)
}

func getDefaultOutputPath(namespace string, isBundle bool) string {
	basePath := "namespaces"
	if isBundle {
		basePath = "bundles"
	}
	if namespace == "" {
		namespace = "default"
	}
	if IsRunningInContainer() {
		outputStat, err := os.Stat("/output")
		if err == nil && outputStat.IsDir() {
			return path.Join("/output", basePath, namespace)
		}
	}
	return path.Join(GetDataHome(), basePath, namespace)
}

func GetDefaultOutputNamespacesPath() string {
	if IsRunningInContainer() {
		outputStat, err := os.Stat("/output")
		if err == nil && outputStat.IsDir() {
			return path.Join("/output", "namespaces")
		}
	}
	return path.Join(GetDataHome(), "namespaces")
}

func GetDefaultOutputBundlesPath() string {
	if IsRunningInContainer() {
		outputStat, err := os.Stat("/output")
		if err == nil && outputStat.IsDir() {
			return path.Join("/output", "bundles")
		}
	}
	return path.Join(GetDataHome(), "bundles")
}

func GetInternalOutputPath(namespace string, internalPath InternalPath) string {
	return path.Join(GetDefaultOutputPath(namespace), string(internalPath))
}

func GetInternalBundleOutputPath(namespace string, internalPath InternalPath) string {
	return path.Join(GetDefaultBundleOutputPath(namespace), string(internalPath))
}

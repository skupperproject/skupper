package api

import (
	"fmt"
	"os"
	"path"

	"github.com/skupperproject/skupper/pkg/apis/skupper/v2alpha1"
)

type InternalPath string
type InternalPathProvider func(namespace string, internalPath InternalPath) string

const (
	InputCertificatesBasePath   InternalPath = "input/certs"
	InputCertificatesCaPath     InternalPath = "input/certs/ca"
	InputCertificatesClientPath InternalPath = "input/certs/client"
	InputCertificatesServerPath InternalPath = "input/certs/server"
	InputSiteStatePath          InternalPath = "input/resources"
	RouterConfigPath            InternalPath = "runtime/router"
	CertificatesBasePath        InternalPath = "runtime/certs"
	CertificatesCaPath          InternalPath = "runtime/certs/ca"
	CertificatesClientPath      InternalPath = "runtime/certs/client"
	CertificatesServerPath      InternalPath = "runtime/certs/server"
	CertificatesLinkPath        InternalPath = "runtime/certs/link"
	RuntimePath                 InternalPath = "runtime"
	RuntimeSiteStatePath        InternalPath = "runtime/resources"
	RuntimeTokenPath            InternalPath = "runtime/links"
	LoadedSiteStatePath         InternalPath = "internal/snapshot"
	ScriptsPath                 InternalPath = "internal/scripts"
)

type IdGetter func() int

var getUid IdGetter = os.Getuid

func GetDataHome() string {
	if getUid() == 0 {
		return "/var/lib/skupper"
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
func GetHostDataHome() string {
	// If container provides SKUPPER_OUTPUT_PATH use it
	if os.Getenv("SKUPPER_OUTPUT_PATH") != "" {
		return os.Getenv("SKUPPER_OUTPUT_PATH")
	}
	return GetDataHome()
}

func GetHostSiteHome(site *v2alpha1.Site) string {
	dataHome := GetHostDataHome()
	ns := site.Namespace
	if ns == "" {
		ns = "default"
	}
	return path.Join(dataHome, "namespaces", ns)
}

func GetHostNamespaceHome(ns string) string {
	dataHome := GetHostDataHome()
	if ns == "" {
		ns = "default"
	}
	return path.Join(dataHome, "namespaces", ns)
}

func GetHostNamespacesPath() string {
	return getHostPath("namespaces")
}

func GetHostBundlesPath() string {
	return getHostPath("bundles")
}

func getHostPath(basePath string) string {
	dataHome := GetHostDataHome()
	return path.Join(dataHome, basePath)
}

func GetCustomSiteHome(site *v2alpha1.Site, customBaseDir string) string {
	return getCustomSiteHome(site, customBaseDir, "namespaces")
}

func GetCustomBundleHome(site *v2alpha1.Site, customBaseDir string) string {
	return getCustomSiteHome(site, customBaseDir, "bundles")
}

func getCustomSiteHome(site *v2alpha1.Site, customBaseDir string, basePath string) string {
	ns := site.Namespace
	if ns == "" {
		ns = "default"
	}
	return path.Join(customBaseDir, basePath, ns)
}

func GetHostSiteInternalPath(site *v2alpha1.Site, internalPath InternalPath) string {
	dataHome := GetHostSiteHome(site)
	return path.Join(dataHome, string(internalPath))
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

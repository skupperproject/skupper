package base

import (
	"os"
	"path"
)

func EdgeKubeConfigs() []string {
	return KubeConfigFiles(true, true)
}

func KubeConfigs() []string {
	return KubeConfigFiles(false, true)
}

// KubeConfigFiles return the available kubeconfig files
// based on provided --kubeconfig, --edgekubeconfig or the
// default KUBECONFIG environment variable (only if no flag
// has been provided)
func KubeConfigFiles(includeEdge, includePublic bool) []string {
	var kubeConfigFiles []string
	edgeConfigs := len(TestFlags.EdgeKubeConfigs)
	pubConfigs := len(TestFlags.KubeConfigs)

	// If should include edge and --edgekubeconfig flag has been provided
	// edge should be processed before public as public can be used as
	// edge but not otherwise
	if includeEdge && edgeConfigs > 0 {
		kubeConfigFiles = append(kubeConfigFiles, TestFlags.EdgeKubeConfigs...)
	}

	// If should include public and kubeconfig flag has been provided
	if includePublic && pubConfigs > 0 {
		kubeConfigFiles = append(kubeConfigFiles, TestFlags.KubeConfigs...)
	}

	// Only try to include the default config if no flag provided
	if pubConfigs == 0 && edgeConfigs == 0 {
		defaultConfig := KubeConfigDefault()
		if defaultConfig != "" {
			kubeConfigFiles = []string{defaultConfig}
		}
	}

	return kubeConfigFiles
}

// KubeConfigDefault returns the "default" KUBECONFIG
// filename if one exists or an empty string otherwise.
func KubeConfigDefault() string {
	// Otherwise use the default
	kubeconfig := os.Getenv("KUBECONFIG")
	if kubeconfig == "" {
		homedir, _ := os.UserHomeDir()
		kubeconfig = path.Join(homedir, ".kube/config")
	}

	// Validate that it exists
	_, err := os.Stat(kubeconfig)
	if err != nil {
		return ""
	}

	return kubeconfig
}

// KubeConfigFilesCount returns total amount of kubeconfig
// files provided (using --kubeconfig or --edgekubeconfig)
// or 1 if no flags provided and the default exists
func KubeConfigFilesCount(includeEdge, includePublic bool) int {
	return len(KubeConfigFiles(includeEdge, includePublic))
}

// MultipleClusters returns true if more than one --kubeconfig or
// --edgekubeconfig files have been provided
func MultipleClusters() bool {
	return KubeConfigFilesCount(true, true) > 1
}

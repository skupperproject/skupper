package base

import (
	"gotest.tools/assert"
	"os"
	"path"
	"testing"
)

func EdgeKubeConfigs(t *testing.T) []string {
	return KubeConfigFiles(t, true, true)
}

func KubeConfigs(t *testing.T) []string {
	return KubeConfigFiles(t, false, true)
}

// KubeConfigFiles return the available kubeconfig files
// based on provided --kubeconfig, --edgekubeconfig or the
// default KUBECONFIG environment variable (only if no flag
// has been provided)
func KubeConfigFiles(t *testing.T, includeEdge, includePublic bool) []string {
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
		kubeConfigFiles = []string{KubeConfigDefault(t)}
	}

	return kubeConfigFiles
}

// KubeConfigDefault returns the "default" KUBECONFIG
// filename if one exists
func KubeConfigDefault(t *testing.T) string {
	// Otherwise use the default
	kubeconfig := os.Getenv("KUBECONFIG")
	if kubeconfig == "" {
		homedir, err := os.UserHomeDir()
		assert.Assert(t, err)
		kubeconfig = path.Join(homedir, ".kube/config")
	}

	// Validate that it exists
	_, err := os.Stat(kubeconfig)
	if err != nil {
		t.Skipf("KUBECONFIG file could not be determined")
	}

	return kubeconfig
}

// KubeConfigFilesCount returns total amount of kubeconfig
// files provided (using --kubeconfig or --edgekubeconfig)
// or 1 if no flags provided and the default exists
func KubeConfigFilesCount(t *testing.T, includeEdge, includePublic bool) int {
	return len(KubeConfigFiles(t, includeEdge, includePublic))
}

// MultipleClusters returns true if more than one --kubeconfig or
// --edgekubeconfig files have been provided
func MultipleClusters(t *testing.T) bool {
	return KubeConfigFilesCount(t, true, true) > 1
}

package k8s

import (
	"github.com/skupperproject/skupper/test/utils/base"
	"gotest.tools/assert"
	"os"
	"path"
	"testing"
)

// KubeConfigFiles return the available kubeconfig files
// based on provided --kubeconfig, --edgekubeconfig or the
// default KUBECONFIG environment variable (only if no flag
// has been provided)
func KubeConfigFiles(t *testing.T, includeEdge, includePublic bool) []string {
	kubeConfigFiles := []string{}
	edgeConfigs := len(base.TestFlags.EdgeKubeConfigs)
	pubConfigs := len(base.TestFlags.KubeConfigs)

	// If should include edge and --edgekubeconfig flag has been provided
	// edge should be processed before public as public can be used as
	// edge but not otherwise
	if includeEdge && edgeConfigs > 0 {
		kubeConfigFiles = append(kubeConfigFiles, base.TestFlags.EdgeKubeConfigs...)
	}

	// If should include public and kubeconfig flag has been provided
	if includePublic && pubConfigs > 0 {
		kubeConfigFiles = append(kubeConfigFiles, base.TestFlags.KubeConfigs...)
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
	assert.Assert(t, err, "KUBECONFIG file could not be determined")

	return kubeconfig
}

// KubeConfigFilesCount returns total amount of kubeconfig
// files provided (using --kubeconfig or --edgekubeconfig)
// or 1 if no flags provided and the default exists
func KubeConfigFilesCount(t *testing.T, includeEdge, includePublic bool) int {
	return len(KubeConfigFiles(t, includeEdge, includePublic))
}

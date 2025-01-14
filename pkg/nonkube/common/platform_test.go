package common

import (
	"fmt"
	"os"
	"path"
	"testing"

	"github.com/skupperproject/skupper/pkg/nonkube/api"
	"gotest.tools/v3/assert"
)

func TestNamespacePlatformLoader_GetPathProvider(t *testing.T) {
	nsPlatformLoader := &NamespacePlatformLoader{}
	defaultPathProvider := nsPlatformLoader.GetPathProvider()
	assert.Equal(t, defaultPathProvider("", api.RuntimePath), api.GetInternalOutputPath("", api.RuntimePath))
	customProvider := createCustomPathProvider(t)
	nsPlatformLoader.PathProvider = customProvider
	assert.Equal(t, nsPlatformLoader.GetPathProvider()("", api.RuntimePath), customProvider("", api.RuntimePath))
}

func TestNamespacePlatformLoader_Load(t *testing.T) {
	for _, tc := range []struct {
		name                string
		namespace           string
		noPlatformFile      bool
		invalidPlatformFile bool
		errorPrefix         string
		expectedPlatform    string
	}{
		{
			name:             "empty-namespace-default-platform",
			expectedPlatform: "kubernetes",
		},
		{
			name:             "sample-namespace-podman-platform",
			namespace:        "sample",
			expectedPlatform: "podman",
		},
		{
			name:           "empty-namespace-no-platform-file",
			noPlatformFile: true,
			errorPrefix:    "failed to read platform config file for namespace default: ",
		},
		{
			name:                "empty-namespace-invalid-platform-file",
			invalidPlatformFile: true,
			errorPrefix:         "failed to unmarshal platform config file for namespace default: ",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pathProvider := createCustomPathProvider(t)
			basePath := pathProvider(tc.namespace, api.InternalBasePath)
			assert.Assert(t, os.MkdirAll(basePath, 0755))
			nsPlatformLoader := &NamespacePlatformLoader{
				PathProvider: pathProvider,
			}
			if !tc.noPlatformFile {
				data := fmt.Sprintf("platform: %s", tc.expectedPlatform)
				if tc.invalidPlatformFile {
					data = "bad-data"
				}
				assert.Assert(t, os.WriteFile(path.Join(basePath, "platform.yaml"), []byte(data), 0644), "error creating fake platform.yaml")
			}
			platform, err := nsPlatformLoader.Load(tc.namespace)
			if tc.noPlatformFile || tc.invalidPlatformFile {
				assert.ErrorContains(t, err, tc.errorPrefix)
			} else {
				assert.NilError(t, err)
				assert.Equal(t, platform, tc.expectedPlatform)
			}
		})
	}
}

func createCustomPathProvider(t *testing.T) api.InternalPathProvider {
	baseDir := t.TempDir()
	return func(namespace string, internalPath api.InternalPath) string {
		if namespace == "" {
			namespace = "default"
		}
		return path.Join(baseDir, namespace, string(internalPath))
	}
}

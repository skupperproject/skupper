package config

import (
	"os"
	"testing"

	"github.com/skupperproject/skupper/api/types"
	"gotest.tools/v3/assert"
)

func TestGetPlatform(t *testing.T) {
	// Save original args and env to restore after test
	originalArgs := os.Args
	originalPlatformEnvVar, platformEnvExists := os.LookupEnv(types.ENV_PLATFORM)
	originalPlatformVar := Platform

	// Deferred anonymous functions can access the surrounding function's state.
	defer func() {
		// Restore the original state
		os.Args = originalArgs
		if platformEnvExists {
			os.Setenv(types.ENV_PLATFORM, originalPlatformEnvVar)
		} else {
			os.Unsetenv(types.ENV_PLATFORM)
		}
		Platform = originalPlatformVar
		ClearPlatform()
	}()

	testCases := []struct {
		name             string
		platformVar      string
		platformEnvVar   string
		commandLineArgs  []string
		expectedPlatform types.Platform //expected outcome of the test
	}{
		{
			name:             "Default Kubernetes platform",
			platformVar:      "",
			platformEnvVar:   "",
			commandLineArgs:  []string{"skupper"},
			expectedPlatform: types.PlatformKubernetes,
		},
		{
			name:             "Platform variable set",
			platformVar:      string(types.PlatformPodman),
			platformEnvVar:   "",
			commandLineArgs:  []string{"skupper"},
			expectedPlatform: types.PlatformPodman,
		},
		{
			name:             "Platform env var set",
			platformVar:      "",
			platformEnvVar:   string(types.PlatformDocker),
			commandLineArgs:  []string{"skupper"},
			expectedPlatform: types.PlatformDocker,
		},
		{
			name:             "CLI - --platform arg with space",
			platformVar:      "",
			platformEnvVar:   "",
			commandLineArgs:  []string{"skupper", "--platform", "podman"},
			expectedPlatform: types.PlatformPodman,
		},
		{
			name:             "CLI - --platform arg with space",
			platformVar:      "",
			platformEnvVar:   "",
			commandLineArgs:  []string{"skupper", "--platform", "kubernetes"},
			expectedPlatform: types.PlatformKubernetes,
		},
		{
			name:             "CLI arg with short option and space",
			platformVar:      "",
			platformEnvVar:   "",
			commandLineArgs:  []string{"skupper", "-p", "docker"},
			expectedPlatform: types.PlatformDocker,
		},
		{
			name:             "CLI arg with equals",
			platformVar:      "",
			platformEnvVar:   "",
			commandLineArgs:  []string{"skupper", "--platform=linux"},
			expectedPlatform: types.PlatformLinux,
		},
		{
			name:             "CLI arg with equals",
			platformVar:      "",
			platformEnvVar:   "",
			commandLineArgs:  []string{"skupper", "--platform=kubernetes"},
			expectedPlatform: types.PlatformKubernetes,
		},
		{
			name:             "CLI arg with the short -p option with equals",
			platformVar:      "",
			platformEnvVar:   "",
			commandLineArgs:  []string{"skupper", "-p=podman"},
			expectedPlatform: types.PlatformPodman,
		},
		{
			name:             "Invalid platform defaults to kubernetes",
			platformVar:      "invalid-platform",
			platformEnvVar:   "",
			commandLineArgs:  []string{"skupper"},
			expectedPlatform: types.PlatformKubernetes,
		},
		{
			name:             "Priority: CLI arg over env var and var",
			platformVar:      string(types.PlatformKubernetes),
			platformEnvVar:   string(types.PlatformKubernetes),
			commandLineArgs:  []string{"skupper", "--platform", "podman"},
			expectedPlatform: types.PlatformPodman,
		},
		{
			name:             "Priority: platform var over env var over default var (types.PlatformKubernetes)",
			platformVar:      string(types.PlatformKubernetes),
			platformEnvVar:   string(types.PlatformDocker),
			commandLineArgs:  []string{"skupper"},
			expectedPlatform: types.PlatformKubernetes,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			// Clear the platform before each test run
			ClearPlatform()

			// Set the Platform var in platform.go for every test.
			Platform = testCase.platformVar
			if testCase.platformEnvVar == "" {
				os.Unsetenv(types.ENV_PLATFORM)
			} else {
				os.Setenv(types.ENV_PLATFORM, testCase.platformEnvVar)
			}
			os.Args = testCase.commandLineArgs

			platform := GetPlatform()
			assert.Equal(t, testCase.expectedPlatform, platform)

			// Test to see if the platform is cached in configuredPlatform.
			if testCase.name == "Default Kubernetes platform" {
				// Change environment to types.PlatformDocker and make sure that the platform
				// after calling GetPlatform() is still types.PlatformKubernetes
				os.Setenv(types.ENV_PLATFORM, string(types.PlatformDocker))
				platform = GetPlatform()
				assert.Equal(t, testCase.expectedPlatform, platform, "Platform should be cached and not change when env changes")
			}
		})
	}
}

func TestClearPlatform(t *testing.T) {
	// Save original state and apply it back before the test exits.
	originalPlatformVar := Platform
	originalPlatformEnv, platformEnvExists := os.LookupEnv(types.ENV_PLATFORM)

	// Deferred anonymous functions can access the surrounding function's state.
	defer func() {
		// Restore original state
		Platform = originalPlatformVar
		if platformEnvExists {
			os.Setenv(types.ENV_PLATFORM, originalPlatformEnv)
		} else {
			os.Unsetenv(types.ENV_PLATFORM)
		}
	}()

	// Initialize Platform
	Platform = string(types.PlatformPodman)
	os.Unsetenv(types.ENV_PLATFORM)

	// First call should set the cached platform to PlatformPodman
	platPodman := GetPlatform()
	assert.Equal(t, types.PlatformPodman, platPodman)

	// Change the environment
	Platform = string(types.PlatformDocker)

	// Second call should return cached value, PlatformPodman
	platPodman = GetPlatform()
	assert.Equal(t, types.PlatformPodman, platPodman)

	// Clear the cache
	ClearPlatform()

	// Third call should use the new environment PlatformLinux
	Platform = ""
	os.Setenv(types.ENV_PLATFORM, string(types.PlatformLinux))
	platLinux := GetPlatform()
	assert.Equal(t, types.PlatformLinux, platLinux)

}

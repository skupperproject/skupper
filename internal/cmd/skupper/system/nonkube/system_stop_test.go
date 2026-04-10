package nonkube

import (
	"fmt"
	"os"
	"path"
	"testing"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/internal/cmd/skupper/common"
	"github.com/skupperproject/skupper/internal/cmd/skupper/common/testutils"
	"github.com/skupperproject/skupper/internal/config"
	"github.com/skupperproject/skupper/pkg/nonkube/api"
	"gotest.tools/v3/assert"
)

func TestCmdSystemTearDown_ValidateInput(t *testing.T) {
	type test struct {
		name            string
		namespace       string
		args            []string
		envSystemReload string
		expectedError   string
		configPlatform  string // platform.yaml content
		currentPlatform string // SKUPPER_PLATFORM env var value
	}

	testTable := []test{
		{
			name:            "arg-not-accepted",
			args:            []string{"namespace"},
			configPlatform:  "podman",
			currentPlatform: "podman",
			expectedError:   "this command does not accept arguments",
		},
		{
			name:            "invalid-namespace",
			namespace:       "Invalid",
			configPlatform:  "docker",
			currentPlatform: "docker",
			expectedError:   "namespace is not valid: value does not match this regular expression: ^[a-z0-9]([-a-z0-9]*[a-z0-9])?$",
		},
		{
			name:            "platform-mismatch-podman-vs-docker",
			namespace:       "test-ns",
			configPlatform:  "podman",
			currentPlatform: "docker",
			expectedError:   `existing namespace uses "podman" platform and it cannot change to "docker"`,
		},
	}

	for _, test := range testTable {
		t.Run(test.name, func(t *testing.T) {

			// Create a temporary directory to simulate the namespace output path
			tmpDir := t.TempDir()

			namespace := test.namespace
			if namespace == "" {
				namespace = "default"
			}

			internalDir := path.Join(tmpDir, "skupper", "namespaces", namespace, string(api.InternalBasePath))
			err := os.MkdirAll(internalDir, 0755)
			assert.NilError(t, err)

			//create platform config file
			platformYaml := fmt.Sprintf("platform: %s\n", test.configPlatform)
			err = os.WriteFile(path.Join(internalDir, "platform.yaml"), []byte(platformYaml), 0644)
			assert.NilError(t, err)

			t.Setenv("XDG_DATA_HOME", tmpDir)

			//set the current platform
			config.ClearPlatform()
			t.Setenv(types.ENV_PLATFORM, test.currentPlatform)
			t.Cleanup(func() {
				config.ClearPlatform()
			})

			command := &CmdSystemStop{}
			command.Namespace = test.namespace
			command.CobraCmd = common.ConfigureCobraCommand(common.PlatformLinux, common.SkupperCmdDescription{}, command, nil)

			testutils.CheckValidateInput(t, command, test.expectedError, test.args)
		})
	}
}

func TestCmdSystemTearDown_InputToOptions(t *testing.T) {

	type test struct {
		name              string
		args              []string
		namespace         string
		expectedNamespace string
	}

	testTable := []test{
		{
			name:              "options-by-default",
			expectedNamespace: "default",
		},
		{
			name:              "namespace-provided",
			args:              []string{"east"},
			namespace:         "east",
			expectedNamespace: "east",
		},
	}

	for _, test := range testTable {
		t.Run(test.name, func(t *testing.T) {

			cmd := newCmdSystemTeardownWithMocks(false)
			cmd.Namespace = test.namespace
			cmd.InputToOptions()

			assert.Check(t, cmd.Namespace == test.expectedNamespace)

		})
	}
}

func TestCmdSystemTeardown_Run(t *testing.T) {
	type test struct {
		name          string
		teardownFails bool
		errorMessage  string
	}

	testTable := []test{
		{
			name:          "runs ok",
			teardownFails: false,
			errorMessage:  "",
		},
		{
			name:          "teardown fails",
			teardownFails: true,
			errorMessage:  "System teardown has failed: fail",
		},
	}

	for _, test := range testTable {
		command := newCmdSystemTeardownWithMocks(test.teardownFails)

		t.Run(test.name, func(t *testing.T) {

			err := command.Run()
			if err != nil {
				assert.Check(t, test.errorMessage == err.Error())
			} else {
				assert.Check(t, err == nil)
			}
		})
	}
}

// --- helper methods

func newCmdSystemTeardownWithMocks(systemTeardDownFails bool) *CmdSystemStop {

	cmdMock := &CmdSystemStop{
		TearDown: mockCmdSystemTeardown,
	}
	if systemTeardDownFails {
		cmdMock.TearDown = mockCmdSystemTeardownFails
	}

	return cmdMock
}

func mockCmdSystemTeardown(namespace string) error { return nil }
func mockCmdSystemTeardownFails(namespace string) error {
	return fmt.Errorf("fail")
}

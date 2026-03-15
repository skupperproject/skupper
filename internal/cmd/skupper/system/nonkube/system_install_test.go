package nonkube

import (
	"fmt"
	"os"
	"testing"

	cmd "github.com/skupperproject/skupper/internal/cmd/skupper/common"
	"github.com/skupperproject/skupper/internal/config"

	"github.com/skupperproject/skupper/internal/cmd/skupper/common/testutils"
	"gotest.tools/v3/assert"
)

func TestCmdSystemInstall_ValidateInput(t *testing.T) {
	type test struct {
		name          string
		args          []string
		platform      string
		reloadType    string
		expectedError string
	}

	testTable := []test{
		{
			name:          "args-are-not-accepted",
			args:          []string{"something"},
			platform:      "podman",
			expectedError: "this command does not accept arguments",
		},
		{
			name:          "platform not supported",
			platform:      "linux",
			expectedError: "the selected platform is not supported by this command. There is nothing to install",
		},
		{
			name:          "reload type not supported",
			reloadType:    "both",
			platform:      "podman",
			expectedError: "reload type is not valid: value both not allowed. It should be one of this options: [manual auto]",
		},
	}

	for _, test := range testTable {
		t.Run(test.name, func(t *testing.T) {

			config.ClearPlatform()
			err := os.Setenv("SKUPPER_PLATFORM", test.platform)
			assert.Check(t, err == nil)

			command := &CmdSystemInstall{Flags: &cmd.CommandSystemInstallFlags{}}
			command.Flags.ReloadType = test.reloadType

			testutils.CheckValidateInput(t, command, test.expectedError, test.args)
		})
	}
}

func TestCmdSystemInstall_Run(t *testing.T) {
	type test struct {
		name                  string
		socketEnablementFails bool
		errorMessage          string
	}

	testTable := []test{
		{
			name:                  "runs ok",
			socketEnablementFails: false,
			errorMessage:          "",
		},
		{
			name:                  "socket enablement fails",
			socketEnablementFails: true,
			errorMessage:          "failed to configure the environment : systemd failed to enable podman socket",
		},
	}

	for _, test := range testTable {
		command := newCmdSystemInstallWithMocks(test.socketEnablementFails)

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

func newCmdSystemInstallWithMocks(podmanSocketEnablementFails bool) *CmdSystemInstall {

	cmdMock := &CmdSystemInstall{
		SystemInstall: mockCmdSystemInstall,
	}

	if podmanSocketEnablementFails {
		cmdMock.SystemInstall = mockCmdSystemInstallSocketEnablementFails
	}

	return cmdMock
}

func mockCmdSystemInstall(platform string, reloadType string) error { return nil }
func mockCmdSystemInstallSocketEnablementFails(platform string, reloadType string) error {
	return fmt.Errorf("systemd failed to enable podman socket")
}

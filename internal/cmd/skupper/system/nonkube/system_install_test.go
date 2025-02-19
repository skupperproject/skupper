package nonkube

import (
	"fmt"
	"testing"

	"github.com/skupperproject/skupper/internal/cmd/skupper/common"
	"github.com/skupperproject/skupper/internal/cmd/skupper/common/testutils"
	"gotest.tools/v3/assert"
)

func TestCmdSystemInstall_ValidateInput(t *testing.T) {
	type test struct {
		name          string
		args          []string
		expectedError string
	}

	testTable := []test{
		{
			name:          "args-are-not-accepted",
			args:          []string{"something"},
			expectedError: "this command does not accept arguments",
		},
	}

	for _, test := range testTable {
		t.Run(test.name, func(t *testing.T) {

			command := &CmdSystemReload{}
			command.CobraCmd = common.ConfigureCobraCommand(common.PlatformLinux, common.SkupperCmdDescription{}, command, nil)

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

func mockCmdSystemInstall() error { return nil }
func mockCmdSystemInstallSocketEnablementFails() error {
	return fmt.Errorf("systemd failed to enable podman socket")
}

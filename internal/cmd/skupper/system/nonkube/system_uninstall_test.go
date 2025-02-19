package nonkube

import (
	"fmt"
	"testing"

	"github.com/skupperproject/skupper/internal/cmd/skupper/common"
	"github.com/skupperproject/skupper/internal/cmd/skupper/common/testutils"
	"gotest.tools/v3/assert"
)

func TestCmdSystemUnInstall_ValidateInput(t *testing.T) {
	type test struct {
		name          string
		args          []string
		flags         *common.CommandSystemUninstallFlags
		mock          func() (bool, error)
		expectedError string
	}

	testTable := []test{
		{
			name:          "args are not accepted",
			args:          []string{"something"},
			expectedError: "this command does not accept arguments",
		},
		{
			name:  "force flag is provided",
			flags: &common.CommandSystemUninstallFlags{Force: true},
		},
		{
			name:          "force flag is not provided and there are active sites",
			flags:         &common.CommandSystemUninstallFlags{Force: false},
			mock:          mockCmdSystemUninstallThereAreStillSites,
			expectedError: "Uninstallation halted: Active sites detected.",
		},
		{
			name:  "force flag is not provided but there are not any active site",
			flags: &common.CommandSystemUninstallFlags{Force: false},
			mock:  mockCmdSystemUninstallNoActiveSites,
		},
		{
			name:          "force flag is not provided but checking sites fails",
			flags:         &common.CommandSystemUninstallFlags{Force: false},
			mock:          mockCmdSystemUninstallCheckActiveSitesFails,
			expectedError: "error",
		},
	}

	for _, test := range testTable {
		t.Run(test.name, func(t *testing.T) {

			command := newCmdSystemUninstallWithMocks(false)
			command.CobraCmd = common.ConfigureCobraCommand(common.PlatformLinux, common.SkupperCmdDescription{}, command, nil)
			command.CheckActiveSites = test.mock
			command.Flags = test.flags

			testutils.CheckValidateInput(t, command, test.expectedError, test.args)
		})
	}

}

func TestCmdSystemUninstall_InputToOptions(t *testing.T) {

	type test struct {
		name          string
		flags         *common.CommandSystemUninstallFlags
		expectedForce bool
	}

	testTable := []test{
		{
			name:          "options-by-default",
			flags:         &common.CommandSystemUninstallFlags{Force: false},
			expectedForce: false,
		},
		{
			name:          "forced to uninstall",
			flags:         &common.CommandSystemUninstallFlags{Force: true},
			expectedForce: true,
		},
	}

	for _, test := range testTable {
		t.Run(test.name, func(t *testing.T) {

			cmd := newCmdSystemUninstallWithMocks(false)
			cmd.Flags = test.flags
			cmd.InputToOptions()

			assert.Check(t, cmd.forceUninstall == test.expectedForce)
		})
	}
}

func TestCmdSystemUninstall_Run(t *testing.T) {
	type test struct {
		name               string
		disableSocketFails bool
		errorMessage       string
	}

	testTable := []test{
		{
			name:               "runs ok",
			disableSocketFails: false,
			errorMessage:       "",
		},
		{
			name:               "disable socket fails",
			disableSocketFails: true,
			errorMessage:       "failed to uninstall : disable socket fails",
		},
	}

	for _, test := range testTable {
		command := newCmdSystemUninstallWithMocks(test.disableSocketFails)

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

func newCmdSystemUninstallWithMocks(disableSocketFails bool) *CmdSystemUninstall {

	cmdMock := &CmdSystemUninstall{
		SystemUninstall:  mockCmdSystemUninstall,
		CheckActiveSites: mockCmdSystemUninstallNoActiveSites,
	}

	if disableSocketFails {
		cmdMock.SystemUninstall = mockCmdSystemUninstallDisableSocketFails
	}

	return cmdMock
}

func mockCmdSystemUninstall() error { return nil }
func mockCmdSystemUninstallDisableSocketFails() error {
	return fmt.Errorf("disable socket fails")
}

func mockCmdSystemUninstallThereAreStillSites() (bool, error)    { return true, nil }
func mockCmdSystemUninstallCheckActiveSitesFails() (bool, error) { return false, fmt.Errorf("error") }
func mockCmdSystemUninstallNoActiveSites() (bool, error)         { return false, nil }

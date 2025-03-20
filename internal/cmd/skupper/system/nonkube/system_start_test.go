package nonkube

import (
	"fmt"
	"github.com/skupperproject/skupper/internal/utils"
	"os"
	"testing"

	"github.com/skupperproject/skupper/internal/cmd/skupper/common"
	"github.com/skupperproject/skupper/internal/cmd/skupper/common/testutils"
	"github.com/skupperproject/skupper/internal/config"
	"github.com/skupperproject/skupper/internal/nonkube/bootstrap"
	"github.com/skupperproject/skupper/pkg/nonkube/api"
	"gotest.tools/v3/assert"
)

func TestCmdSystemSetup_ValidateInput(t *testing.T) {
	type test struct {
		name          string
		args          []string
		namespace     string
		expectedError string
	}

	testTable := []test{
		{
			name:          "args-are-not-accepted",
			args:          []string{"something"},
			namespace:     utils.RandomId(4),
			expectedError: "this command does not accept arguments",
		},
	}

	for _, test := range testTable {
		t.Run(test.name, func(t *testing.T) {

			command := &CmdSystemStart{}
			command.CobraCmd = common.ConfigureCobraCommand(common.PlatformLinux, common.SkupperCmdDescription{}, command, nil)
			command.Namespace = test.namespace

			testutils.CheckValidateInput(t, command, test.expectedError, test.args)
		})
	}
}

func TestCmdSystemSetup_InputToOptions(t *testing.T) {

	type test struct {
		name                   string
		args                   []string
		platform               string
		namespace              string
		expectedBinary         string
		expectedNamespace      string
		expectedIsBundle       bool
		expectedBundleStrategy string
	}

	testTable := []test{
		{
			name:              "options-by-default",
			expectedBinary:    "podman",
			expectedNamespace: "default",
		},
		{
			name:              "linux",
			namespace:         "east",
			platform:          "linux",
			expectedBinary:    "skrouterd",
			expectedNamespace: "east",
		},
		{
			name:              "docker",
			namespace:         "east",
			platform:          "docker",
			expectedBinary:    "docker",
			expectedNamespace: "east",
		},
	}

	for _, test := range testTable {
		t.Run(test.name, func(t *testing.T) {
			os.Setenv(common.ENV_PLATFORM, test.platform)
			config.ClearPlatform()

			cmd := newCmdSystemSetupWithMocks(false, false)
			cmd.Namespace = test.namespace

			cmd.InputToOptions()

			assert.Check(t, cmd.ConfigBootstrap.Binary == test.expectedBinary)
			assert.Check(t, cmd.ConfigBootstrap.BundleStrategy == test.expectedBundleStrategy)
			assert.Check(t, cmd.ConfigBootstrap.Namespace == test.expectedNamespace)
			assert.Check(t, cmd.ConfigBootstrap.IsBundle == test.expectedIsBundle)
		})
	}
}

func TestCmdSystemSetup_Run(t *testing.T) {
	type test struct {
		name           string
		preCheckFails  bool
		bootstrapFails bool
		errorMessage   string
	}

	testTable := []test{
		{
			name:           "runs ok",
			preCheckFails:  false,
			bootstrapFails: false,
			errorMessage:   "",
		},
		{
			name:           "pre check fails",
			preCheckFails:  true,
			bootstrapFails: false,
			errorMessage:   "precheck fails",
		},
		{
			name:           "bootstrap fails",
			preCheckFails:  false,
			bootstrapFails: true,
			errorMessage:   "Failed to bootstrap: bootstrap fails",
		},
	}

	for _, test := range testTable {
		command := newCmdSystemSetupWithMocks(test.preCheckFails, test.bootstrapFails)

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

func newCmdSystemSetupWithMocks(precheckFails bool, bootstrapFails bool) *CmdSystemStart {

	cmdMock := &CmdSystemStart{
		PreCheck:  mockCmdSystemSetupPreCheck,
		Bootstrap: mockCmdSystemSetupBootStrap,
		PostExec:  mockCmdSystemSetupPostExec,
	}
	if precheckFails {
		cmdMock.PreCheck = mockCmdSystemSetupPreCheckFails
	}

	if bootstrapFails {
		cmdMock.Bootstrap = mockCmdSystemSetupBootStrapFails
	}

	return cmdMock
}

func mockCmdSystemSetupPreCheck(config *bootstrap.Config) error { return nil }
func mockCmdSystemSetupPreCheckFails(config *bootstrap.Config) error {
	return fmt.Errorf("precheck fails")
}
func mockCmdSystemSetupBootStrap(config *bootstrap.Config) (*api.SiteState, error) {
	return &api.SiteState{}, nil
}
func mockCmdSystemSetupBootStrapFails(config *bootstrap.Config) (*api.SiteState, error) {
	return nil, fmt.Errorf("bootstrap fails")
}
func mockCmdSystemSetupPostExec(config *bootstrap.Config, siteState *api.SiteState) {}

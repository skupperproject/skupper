package nonkube

import (
	"fmt"
	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/internal/cmd/skupper/common"
	"github.com/skupperproject/skupper/internal/cmd/skupper/common/utils"
	"github.com/skupperproject/skupper/pkg/nonkube/api"
	"github.com/skupperproject/skupper/pkg/nonkube/bootstrap"
	"gotest.tools/assert"
	"os"
	"strings"
	"testing"
)

func TestCmdSystemStart_ValidateInput(t *testing.T) {
	type test struct {
		name           string
		args           []string
		flags          *common.CommandSystemSetupFlags
		expectedErrors []string
	}

	testTable := []test{
		{
			name:           "args-are-not-accepted",
			args:           []string{"something"},
			expectedErrors: []string{"this command does not accept arguments"},
		},
		{
			name: "invalid-bundle-strategy",
			flags: &common.CommandSystemSetupFlags{
				Strategy: "not-valid",
			},
			expectedErrors: []string{"Invalid bundle strategy: not-valid"},
		},
	}

	for _, test := range testTable {
		t.Run(test.name, func(t *testing.T) {

			command := &CmdSystemSetup{}
			command.CobraCmd = common.ConfigureCobraCommand(types.PlatformSystemd, common.SkupperCmdDescription{}, command, nil)

			if test.flags != nil {
				command.Flags = test.flags
			}

			actualErrors := command.ValidateInput(test.args)
			actualErrorsMessages := utils.ErrorsToMessages(actualErrors)
			assert.DeepEqual(t, actualErrorsMessages, test.expectedErrors)

		})
	}
}

func TestCmdSystemStart_InputToOptions(t *testing.T) {

	type test struct {
		name                   string
		args                   []string
		flags                  common.CommandSystemSetupFlags
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
			flags:             common.CommandSystemSetupFlags{},
			expectedBinary:    "podman",
			expectedNamespace: "default",
		},
		{
			name: "systemd",
			flags: common.CommandSystemSetupFlags{
				Path: "input-path",
			},
			namespace:         "east",
			platform:          "systemd",
			expectedBinary:    "skrouterd",
			expectedNamespace: "east",
		},
		{
			name: "docker",
			flags: common.CommandSystemSetupFlags{
				Path: "input-path",
			},
			namespace:         "east",
			platform:          "docker",
			expectedBinary:    "docker",
			expectedNamespace: "east",
		},
		{
			name: "bundle-default",
			flags: common.CommandSystemSetupFlags{
				Path:     "input-path",
				Strategy: "bundle",
			},
			namespace:              "east",
			platform:               "podman",
			expectedBinary:         "",
			expectedNamespace:      "east",
			expectedIsBundle:       true,
			expectedBundleStrategy: "bundle",
		},
	}

	for _, test := range testTable {
		t.Run(test.name, func(t *testing.T) {
			os.Setenv(types.ENV_PLATFORM, test.platform)

			cmd := newCmdSystemStartWithMocks(false, false)
			cmd.Flags = &test.flags
			cmd.Namespace = test.namespace

			cmd.InputToOptions()

			assert.Check(t, cmd.ConfigBootstrap.Binary == test.expectedBinary)
			assert.Check(t, cmd.ConfigBootstrap.BundleStrategy == test.expectedBundleStrategy)
			assert.Check(t, cmd.ConfigBootstrap.Namespace == test.expectedNamespace)
			assert.Check(t, cmd.ConfigBootstrap.IsBundle == test.expectedIsBundle)
			assert.Check(t, strings.Contains(cmd.ConfigBootstrap.InputPath, cmd.Flags.Path))
		})
	}
}

func TestCmdSystemStart_Run(t *testing.T) {
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
		command := newCmdSystemStartWithMocks(test.preCheckFails, test.bootstrapFails)

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

func newCmdSystemStartWithMocks(precheckFails bool, bootstrapFails bool) *CmdSystemSetup {

	cmdMock := &CmdSystemSetup{
		PreCheck:  mockCmdSystemStartPreCheck,
		Bootstrap: mockCmdSystemStartBootStrap,
		PostExec:  mockCmdSystemStartPostExec,
	}
	if precheckFails {
		cmdMock.PreCheck = mockCmdSystemStartPreCheckFails
	}

	if bootstrapFails {
		cmdMock.Bootstrap = mockCmdSystemStartBootStrapFails
	}

	return cmdMock
}

func mockCmdSystemStartPreCheck(config *bootstrap.Config) error { return nil }
func mockCmdSystemStartPreCheckFails(config *bootstrap.Config) error {
	return fmt.Errorf("precheck fails")
}
func mockCmdSystemStartBootStrap(config *bootstrap.Config) (*api.SiteState, error) {
	return &api.SiteState{}, nil
}
func mockCmdSystemStartBootStrapFails(config *bootstrap.Config) (*api.SiteState, error) {
	return nil, fmt.Errorf("bootstrap fails")
}
func mockCmdSystemStartPostExec(config *bootstrap.Config, siteState *api.SiteState) {}

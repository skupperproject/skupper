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
	"testing"
)

func TestCmdSystemReload_ValidateInput(t *testing.T) {
	type test struct {
		name           string
		args           []string
		expectedErrors []string
	}

	testTable := []test{
		{
			name:           "args-are-not-accepted",
			args:           []string{"something"},
			expectedErrors: []string{"this command does not accept arguments"},
		},
	}

	for _, test := range testTable {
		t.Run(test.name, func(t *testing.T) {

			command := &CmdSystemReload{}
			command.CobraCmd = common.ConfigureCobraCommand(types.PlatformSystemd, common.SkupperCmdDescription{}, command, nil)

			actualErrors := command.ValidateInput(test.args)
			actualErrorsMessages := utils.ErrorsToMessages(actualErrors)
			assert.DeepEqual(t, actualErrorsMessages, test.expectedErrors)

		})
	}
}

func TestCmdSystemReload_InputToOptions(t *testing.T) {

	type test struct {
		name              string
		args              []string
		flags             common.CommandSystemStartFlags
		platform          string
		namespace         string
		expectedBinary    string
		expectedNamespace string
	}

	testTable := []test{
		{
			name:              "options-by-default",
			flags:             common.CommandSystemStartFlags{},
			expectedBinary:    "podman",
			expectedNamespace: "default",
		},
		{
			name:              "systemd",
			namespace:         "east",
			platform:          "systemd",
			expectedBinary:    "skrouterd",
			expectedNamespace: "east",
		},
	}

	for _, test := range testTable {
		t.Run(test.name, func(t *testing.T) {
			os.Setenv(types.ENV_PLATFORM, test.platform)

			cmd := newCmdSystemReloadWithMocks(false, false)
			cmd.Flags = &test.flags
			cmd.Namespace = test.namespace

			cmd.InputToOptions()

			assert.Check(t, cmd.ConfigBootstrap.Binary == test.expectedBinary)
			assert.Check(t, cmd.ConfigBootstrap.Namespace == test.expectedNamespace)
		})
	}
}

func TestCmdSystemReload_Run(t *testing.T) {
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
		command := newCmdSystemReloadWithMocks(test.preCheckFails, test.bootstrapFails)

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

func newCmdSystemReloadWithMocks(precheckFails bool, bootstrapFails bool) *CmdSystemStart {

	cmdMock := &CmdSystemStart{
		PreCheck:  mockCmdSystemReloadPreCheck,
		Bootstrap: mockCmdSystemReloadBootStrap,
		PostExec:  mockCmdSystemReloadPostExec,
	}
	if precheckFails {
		cmdMock.PreCheck = mockCmdSystemReloadPreCheckFails
	}

	if bootstrapFails {
		cmdMock.Bootstrap = mockCmdSystemReloadBootStrapFails
	}

	return cmdMock
}

func mockCmdSystemReloadPreCheck(config *bootstrap.Config) error { return nil }
func mockCmdSystemReloadPreCheckFails(config *bootstrap.Config) error {
	return fmt.Errorf("precheck fails")
}
func mockCmdSystemReloadBootStrap(config *bootstrap.Config) (*api.SiteState, error) {
	return &api.SiteState{}, nil
}
func mockCmdSystemReloadBootStrapFails(config *bootstrap.Config) (*api.SiteState, error) {
	return nil, fmt.Errorf("bootstrap fails")
}
func mockCmdSystemReloadPostExec(config *bootstrap.Config, siteState *api.SiteState) {}

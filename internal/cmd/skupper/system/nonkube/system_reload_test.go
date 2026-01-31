package nonkube

import (
	"fmt"
	"os"
	"testing"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/internal/cmd/skupper/common"
	"github.com/skupperproject/skupper/internal/cmd/skupper/common/testutils"
	"github.com/skupperproject/skupper/internal/config"
	"github.com/skupperproject/skupper/internal/nonkube/bootstrap"
	"github.com/skupperproject/skupper/pkg/nonkube/api"
	"gotest.tools/v3/assert"
)

func TestCmdSystemReload_ValidateInput(t *testing.T) {
	type test struct {
		name            string
		namespace       string
		envSystemReload string
		args            []string
		expectedError   string
	}

	testTable := []test{
		{
			name:          "args-are-not-accepted",
			args:          []string{"something"},
			expectedError: "this command does not accept arguments",
		},
		{
			name:          "invalid-namespace",
			namespace:     "Invalid",
			expectedError: "namespace is not valid: value does not match this regular expression: ^[a-z0-9]([-a-z0-9]*[a-z0-9])?$",
		},
		{
			name:            "command-is-disabled",
			args:            []string{},
			namespace:       "ns",
			envSystemReload: types.SystemReloadTypeAuto,
			expectedError:   "this command is disabled because automatic reloading is configured",
		},
	}

	for _, test := range testTable {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv(types.ENV_SYSTEM_AUTO_RELOAD, types.SystemReloadTypeManual)
			if test.envSystemReload != "" {
				t.Setenv(types.ENV_SYSTEM_AUTO_RELOAD, test.envSystemReload)
			}
			command := &CmdSystemReload{}
			command.Namespace = test.namespace
			command.CobraCmd = common.ConfigureCobraCommand(common.PlatformLinux, common.SkupperCmdDescription{}, command, nil)

			testutils.CheckValidateInput(t, command, test.expectedError, test.args)
		})
	}
}

func TestCmdSystemReload_InputToOptions(t *testing.T) {

	type test struct {
		name              string
		args              []string
		platform          string
		namespace         string
		expectedBinary    string
		expectedNamespace string
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
	}

	for _, test := range testTable {
		t.Run(test.name, func(t *testing.T) {
			os.Setenv(common.ENV_PLATFORM, test.platform)
			config.ClearPlatform()

			cmd := newCmdSystemReloadWithMocks(false, false)
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

func newCmdSystemReloadWithMocks(precheckFails bool, bootstrapFails bool) *CmdSystemReload {

	cmdMock := &CmdSystemReload{
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

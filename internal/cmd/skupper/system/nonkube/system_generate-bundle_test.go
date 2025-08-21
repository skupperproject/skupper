package nonkube

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/skupperproject/skupper/internal/cmd/skupper/common"
	"github.com/skupperproject/skupper/internal/cmd/skupper/common/testutils"
	"github.com/skupperproject/skupper/internal/config"
	"github.com/skupperproject/skupper/internal/nonkube/bootstrap"
	"github.com/skupperproject/skupper/pkg/nonkube/api"
	"gotest.tools/v3/assert"
)

func TestCmdSystemGenerateBundle_ValidateInput(t *testing.T) {
	type test struct {
		name          string
		namespace     string
		args          []string
		flags         *common.CommandSystemGenerateBundleFlags
		expectedError string
	}

	testTable := []test{
		{
			name:          "no-args",
			namespace:     "test",
			args:          []string{},
			expectedError: "You need to specify a name for the bundle file to generate.",
		},
		{
			name:          "many-args",
			args:          []string{"bundle", "name"},
			expectedError: "This command does not accept more than one argument.",
		},
		{
			name: "invalid-bundle-strategy",
			args: []string{"bundle-name"},
			flags: &common.CommandSystemGenerateBundleFlags{
				Type: "not-valid",
			},
			expectedError: "Invalid bundle type: value not-valid not allowed. It should be one of this options: [tarball shell-script]",
		},
		{
			name: "invalid-input-path",
			args: []string{"bundle-name"},
			flags: &common.CommandSystemGenerateBundleFlags{
				Input: "/example",
			},
			expectedError: "The input path does not exist",
		},
		{
			name: "valid-input-path",
			args: []string{"bundle-name"},
			flags: &common.CommandSystemGenerateBundleFlags{
				Input: "./",
			},
		},
		{
			name:          "invalid-namespace",
			namespace:     "Invalid",
			args:          []string{"bundle-name"},
			expectedError: "namespace is not valid: value does not match this regular expression: ^[a-z0-9]([-a-z0-9]*[a-z0-9])?$",
		},
	}

	for _, test := range testTable {
		t.Run(test.name, func(t *testing.T) {

			command := &CmdSystemGenerateBundle{}
			command.CobraCmd = common.ConfigureCobraCommand(common.PlatformLinux, common.SkupperCmdDescription{}, command, nil)
			command.Namespace = test.namespace
			if test.flags != nil {
				command.Flags = test.flags
			}

			testutils.CheckValidateInput(t, command, test.expectedError, test.args)
		})
	}
}

func TestCmdSystemGenerateBundle_InputToOptions(t *testing.T) {

	type test struct {
		name                   string
		args                   []string
		flags                  common.CommandSystemGenerateBundleFlags
		namespace              string
		platform               string
		expectedNamespace      string
		expectedIsBundle       bool
		expectedBundleStrategy string
		expectedBundleName     string
		expectedInputPath      string
	}

	testTable := []test{
		{
			name:              "options-by-default",
			flags:             common.CommandSystemGenerateBundleFlags{},
			expectedNamespace: "default",
			platform:          "linux",
		},
		{
			name: "shell-script",
			flags: common.CommandSystemGenerateBundleFlags{
				Input: "input-path",
				Type:  "shell-script",
			},
			namespace:              "east",
			platform:               "podman",
			expectedNamespace:      "east",
			expectedIsBundle:       true,
			expectedBundleStrategy: "bundle",
		},
		{
			name: "tarball",
			flags: common.CommandSystemGenerateBundleFlags{
				Input: "input-path",
				Type:  "tarball",
			},
			namespace:              "east",
			platform:               "docker",
			expectedNamespace:      "east",
			expectedIsBundle:       true,
			expectedBundleStrategy: "tarball",
		},
	}

	for _, test := range testTable {
		t.Run(test.name, func(t *testing.T) {

			os.Setenv(common.ENV_PLATFORM, test.platform)
			config.ClearPlatform()

			cmd := newCmdSystemGenerateBundleWithMocks(false, false)
			cmd.Flags = &test.flags
			cmd.Namespace = test.namespace
			cmd.BundleName = "bundle-name"

			cmd.InputToOptions()

			assert.Check(t, cmd.ConfigBootstrap.BundleStrategy == test.expectedBundleStrategy)
			assert.Check(t, cmd.ConfigBootstrap.Namespace == test.expectedNamespace)
			assert.Check(t, cmd.ConfigBootstrap.IsBundle == test.expectedIsBundle)
			assert.Check(t, strings.Contains(cmd.ConfigBootstrap.InputPath, cmd.Flags.Input))
			assert.Check(t, string(cmd.ConfigBootstrap.Platform) == test.platform)
		})
	}
}

func TestCmdSystemGenerateBundle_Run(t *testing.T) {
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
			errorMessage:   "Failed to generate bundle: bootstrap fails",
		},
	}

	for _, test := range testTable {
		command := newCmdSystemGenerateBundleWithMocks(test.preCheckFails, test.bootstrapFails)

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

func newCmdSystemGenerateBundleWithMocks(precheckFails bool, bootstrapFails bool) *CmdSystemGenerateBundle {

	cmdMock := &CmdSystemGenerateBundle{
		PreCheck:  mockCmdSystemGenerateBundlePreCheck,
		Bootstrap: mockCmdSystemGenerateBundleBootStrap,
		PostExec:  mockCmdSystemGenerateBundlePostExec,
	}
	if precheckFails {
		cmdMock.PreCheck = mockCmdSystemGenerateBundlePreCheckFails
	}

	if bootstrapFails {
		cmdMock.Bootstrap = mockCmdSystemGenerateBundleBootStrapFails
	}

	return cmdMock
}

func mockCmdSystemGenerateBundlePreCheck(config *bootstrap.Config) error { return nil }
func mockCmdSystemGenerateBundlePreCheckFails(config *bootstrap.Config) error {
	return fmt.Errorf("precheck fails")
}
func mockCmdSystemGenerateBundleBootStrap(config *bootstrap.Config) (*api.SiteState, error) {
	return &api.SiteState{}, nil
}
func mockCmdSystemGenerateBundleBootStrapFails(config *bootstrap.Config) (*api.SiteState, error) {
	return nil, fmt.Errorf("bootstrap fails")
}
func mockCmdSystemGenerateBundlePostExec(config *bootstrap.Config, siteState *api.SiteState) {}

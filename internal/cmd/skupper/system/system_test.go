package system

import (
	"testing"

	"github.com/skupperproject/skupper/internal/cmd/skupper/common"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"gotest.tools/v3/assert"
)

func TestCmdSystemFactory(t *testing.T) {

	type test struct {
		name                          string
		expectedFlagsWithDefaultValue map[string]interface{}
		command                       *cobra.Command
	}

	testTable := []test{
		{
			name: "CmdSystemSetupFactory",
			expectedFlagsWithDefaultValue: map[string]interface{}{
				common.FlagNamePath:     "",
				common.FlagNameStrategy: "",
				common.FlagNameForce:    "false",
			},
			command: CmdSystemSetupFactory(common.PlatformPodman),
		},
		{
			name:                          "CmdSystemReloadFactory",
			expectedFlagsWithDefaultValue: map[string]interface{}{},
			command:                       CmdSystemReloadFactory(common.PlatformPodman),
		},
		{
			name:                          "CmdSystemStopFactory",
			expectedFlagsWithDefaultValue: map[string]interface{}{},
			command:                       CmdSystemStopFactory(common.PlatformPodman),
		},
		{
			name:                          "CmdSystemStartFactory",
			expectedFlagsWithDefaultValue: map[string]interface{}{},
			command:                       CmdSystemStartFactory(common.PlatformPodman),
		},
		{
			name: "CmdSystemGenerateBundleFactory",
			expectedFlagsWithDefaultValue: map[string]interface{}{
				common.FlagNameInput: "",
				common.FlagNameType:  "tarball",
			},
			command: CmdSystemGenerateBundleFactory(common.PlatformPodman),
		},
		{
			name:                          "CmdSystemInstallFactory",
			expectedFlagsWithDefaultValue: map[string]interface{}{},
			command:                       CmdSystemInstallFactory(common.PlatformKubernetes),
		},
		{
			name: "CmdSystemUninstallFactory",
			expectedFlagsWithDefaultValue: map[string]interface{}{
				common.FlagNameForce: "false",
			},
			command: CmdSystemUnInstallFactory(common.PlatformKubernetes),
		},
	}

	for _, test := range testTable {

		var flagList []interface{}
		t.Run(test.name, func(t *testing.T) {

			test.command.Flags().VisitAll(func(flag *pflag.Flag) {
				flagList = append(flagList, flag.Name)

				// Check if the flag name exists in the expectedFlagsWithDefaultValue map
				expectedValue, exists := test.expectedFlagsWithDefaultValue[flag.Name]
				if !exists {
					t.Errorf("flag %q not expected", flag.Name)
					return
				}

				// Check if the default value matches the expected default value
				assert.Equal(t, expectedValue, flag.DefValue)
			})

			assert.Check(t, len(flagList) == len(test.expectedFlagsWithDefaultValue))

			assert.Assert(t, test.command.PreRunE != nil)
			assert.Assert(t, test.command.Run != nil)
			assert.Assert(t, test.command.PostRun != nil)
			assert.Assert(t, test.command.Use != "")
			assert.Assert(t, test.command.Short != "")
			assert.Assert(t, test.command.Long != "")
		})
	}
}

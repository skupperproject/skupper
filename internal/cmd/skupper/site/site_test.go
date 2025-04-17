package site

import (
	"testing"

	"github.com/skupperproject/skupper/internal/cmd/skupper/common"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"gotest.tools/v3/assert"
)

func TestCmdSiteFactory(t *testing.T) {

	type test struct {
		name                          string
		expectedFlagsWithDefaultValue map[string]interface{}
		command                       *cobra.Command
	}

	testTable := []test{
		{
			name: "CmdSiteCreateFactory",
			expectedFlagsWithDefaultValue: map[string]interface{}{
				common.FlagNameEnableLinkAccess: "false",
				common.FlagNameLinkAccessType:   "",
				common.FlagNameTimeout:          "3m0s",
				common.FlagNameWait:             "ready",
				common.FlagNameHA:               "false",
			},
			command: CmdSiteCreateFactory(common.PlatformKubernetes),
		},
		{
			name: "CmdSiteUpdateFactory",
			expectedFlagsWithDefaultValue: map[string]interface{}{
				common.FlagNameEnableLinkAccess: "false",
				common.FlagNameLinkAccessType:   "",
				common.FlagNameTimeout:          "30s",
				common.FlagNameWait:             "ready",
				common.FlagNameHA:               "false",
			},
			command: CmdSiteUpdateFactory(common.PlatformKubernetes),
		},
		{
			name: "CmdSiteDeleteFactory",
			expectedFlagsWithDefaultValue: map[string]interface{}{
				common.FlagNameTimeout: "1m0s",
				common.FlagNameWait:    "true",
				common.FlagNameAll:     "false",
			},
			command: CmdSiteDeleteFactory(common.PlatformKubernetes),
		},
		{
			name: "CmdSiteStatusFactory",
			expectedFlagsWithDefaultValue: map[string]interface{}{
				common.FlagNameOutput: "",
			},
			command: CmdSiteStatusFactory(common.PlatformKubernetes),
		},
		{
			name: "CmdSiteGenerateFactory",
			expectedFlagsWithDefaultValue: map[string]interface{}{
				common.FlagNameEnableLinkAccess: "false",
				common.FlagNameLinkAccessType:   "",
				common.FlagNameOutput:           "yaml",
				common.FlagNameHA:               "false",
			},
			command: CmdSiteGenerateFactory(common.PlatformKubernetes),
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

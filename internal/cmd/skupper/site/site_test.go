package site

import (
	"fmt"
	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/internal/cmd/skupper/common"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"gotest.tools/assert"
	"testing"
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
				common.FlagNameServiceAccount:   "",
				common.FlagNameOutput:           "",
				common.FlagNameHost:             "",
			},
			command: CmdSiteCreateFactory(types.PlatformKubernetes),
		},
		{
			name: "CmdSiteUpdateFactory",
			expectedFlagsWithDefaultValue: map[string]interface{}{
				common.FlagNameEnableLinkAccess: "false",
				common.FlagNameLinkAccessType:   "",
				common.FlagNameServiceAccount:   "",
				common.FlagNameOutput:           "",
				common.FlagNameHost:             "",
			},
			command: CmdSiteUpdateFactory(types.PlatformKubernetes),
		},
		{
			name:                          "CmdSiteDeleteFactory",
			expectedFlagsWithDefaultValue: map[string]interface{}{},
			command:                       CmdSiteDeleteFactory(types.PlatformKubernetes),
		},
		{
			name:                          "CmdSiteStatusFactory",
			expectedFlagsWithDefaultValue: map[string]interface{}{},
			command:                       CmdSiteStatusFactory(types.PlatformKubernetes),
		},
	}

	for _, test := range testTable {

		var flagList []string
		t.Run(test.name, func(t *testing.T) {

			test.command.Flags().VisitAll(func(flag *pflag.Flag) {
				flagList = append(flagList, flag.Name)
				assert.Check(t, test.expectedFlagsWithDefaultValue[flag.Name] != nil, fmt.Sprintf("flag %q not expected", flag.Name))
				assert.Check(t, test.expectedFlagsWithDefaultValue[flag.Name] == flag.DefValue)
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

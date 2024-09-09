package link

import (
	"fmt"
	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/internal/cmd/skupper/common"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"gotest.tools/assert"
	"testing"
)

func TestCmdLinkFactory(t *testing.T) {

	type test struct {
		name                          string
		expectedFlagsWithDefaultValue map[string]interface{}
		command                       *cobra.Command
	}

	testTable := []test{
		{
			name: "CmdLinkGenerateFactory",
			expectedFlagsWithDefaultValue: map[string]interface{}{
				common.FlagNameTlsSecret:          "",
				common.FlagNameCost:               "1",
				common.FlagNameOutput:             "yaml",
				common.FlagNameGenerateCredential: "true",
				common.FlagNameTimeout:            "60",
			},
			command: CmdLinkGenerateFactory(types.PlatformKubernetes),
		},
		{
			name: "CmdLinkUpdateFactory",
			expectedFlagsWithDefaultValue: map[string]interface{}{
				common.FlagNameTlsSecret: "",
				common.FlagNameCost:      "1",
				common.FlagNameOutput:    "",
				common.FlagNameTimeout:   "60",
			},
			command: CmdLinkUpdateFactory(types.PlatformKubernetes),
		},
		{
			name: "CmdLinkStatusFactory",
			expectedFlagsWithDefaultValue: map[string]interface{}{
				common.FlagNameOutput: "",
			},
			command: CmdLinkStatusFactory(types.PlatformKubernetes),
		},
		{
			name: "CmdLinkDeleteFactory",
			expectedFlagsWithDefaultValue: map[string]interface{}{
				common.FlagNameTimeout: "60",
			},
			command: CmdLinkDeleteFactory(types.PlatformKubernetes),
		},
	}

	for _, test := range testTable {

		var flagList []string
		t.Run(test.name, func(t *testing.T) {

			test.command.Flags().VisitAll(func(flag *pflag.Flag) {
				flagList = append(flagList, flag.Name)
				assert.Check(t, test.expectedFlagsWithDefaultValue[flag.Name] != nil, fmt.Sprintf("flag %q not expected", flag.Name))
				assert.Check(t, test.expectedFlagsWithDefaultValue[flag.Name] == flag.DefValue, fmt.Sprintf("default value %q for flag %q not expected", flag.DefValue, flag.Name))
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

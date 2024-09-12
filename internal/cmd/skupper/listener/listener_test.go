package listener

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
			name: "CmdListenerCreateFactory",
			expectedFlagsWithDefaultValue: map[string]interface{}{
				common.FlagNameRoutingKey:   "",
				common.FlagNameListenerHost: "",
				common.FlagNameTlsSecret:    "",
				common.FlagNameListenerType: "tcp",
				common.FlagNameOutput:       "",
				common.FlagNameTimeout:      "1m0s",
			},
			command: CmdListenerCreateFactory(types.PlatformKubernetes),
		},
		{
			name: "CmdListenerUpdateFactory",
			expectedFlagsWithDefaultValue: map[string]interface{}{
				common.FlagNameRoutingKey:   "",
				common.FlagNameListenerHost: "",
				common.FlagNameTlsSecret:    "",
				common.FlagNameListenerType: "tcp",
				common.FlagNameOutput:       "",
				common.FlagNameTimeout:      "1m0s",
				common.FlagNameListenerPort: "0",
			},
			command: CmdListenerUpdateFactory(types.PlatformKubernetes),
		},
		{
			name: "CmdListenerStatusFactory",
			expectedFlagsWithDefaultValue: map[string]interface{}{
				common.FlagNameOutput: "",
			},
			command: CmdListenerStatusFactory(types.PlatformKubernetes),
		},
		{
			name: "CmdListenerDeleteFactory",
			expectedFlagsWithDefaultValue: map[string]interface{}{
				common.FlagNameTimeout: "1m0s",
			},
			command: CmdListenerDeleteFactory(types.PlatformKubernetes),
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

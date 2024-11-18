package connector

import (
	"fmt"
	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/internal/cmd/skupper/common"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"gotest.tools/assert"
	"testing"
)

func TestCmdConnectorFactory(t *testing.T) {

	type test struct {
		name                          string
		expectedFlagsWithDefaultValue map[string]interface{}
		command                       *cobra.Command
	}

	testTable := []test{
		{
			name: "CmdConnectorCreateFactory",
			expectedFlagsWithDefaultValue: map[string]interface{}{
				common.FlagNameRoutingKey:      "",
				common.FlagNameHost:            "",
				common.FlagNameTlsCredentials:  "",
				common.FlagNameConnectorType:   "tcp",
				common.FlagNameIncludeNotReady: "false",
				common.FlagNameSelector:        "",
				common.FlagNameWorkload:        "",
				common.FlagNameOutput:          "",
				common.FlagNameTimeout:         "1m0s",
			},
			command: CmdConnectorCreateFactory(types.PlatformKubernetes),
		},
		{
			name: "CmdConnectorUpdateFactory",
			expectedFlagsWithDefaultValue: map[string]interface{}{
				common.FlagNameRoutingKey:      "",
				common.FlagNameHost:            "",
				common.FlagNameTlsCredentials:  "",
				common.FlagNameConnectorType:   "tcp",
				common.FlagNameIncludeNotReady: "false",
				common.FlagNameSelector:        "",
				common.FlagNameWorkload:        "",
				common.FlagNameOutput:          "",
				common.FlagNameTimeout:         "1m0s",
				common.FlagNameConnectorPort:   "0",
			},
			command: CmdConnectorUpdateFactory(types.PlatformKubernetes),
		},
		{
			name: "CmdConnectorStatusFactory",
			expectedFlagsWithDefaultValue: map[string]interface{}{
				common.FlagNameConnectorStatusOutput: "",
			},
			command: CmdConnectorStatusFactory(types.PlatformKubernetes),
		},
		{
			name: "CmdConnectorDeleteFactory",
			expectedFlagsWithDefaultValue: map[string]interface{}{
				common.FlagNameTimeout: "1m0s",
			},
			command: CmdConnectorDeleteFactory(types.PlatformKubernetes),
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

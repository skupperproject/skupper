package nonkube

import (
	"testing"

	"github.com/skupperproject/skupper/internal/cmd/skupper/common"
	"github.com/skupperproject/skupper/internal/cmd/skupper/common/utils"
	"github.com/skupperproject/skupper/internal/nonkube/client/fs"
	"github.com/spf13/cobra"

	"gotest.tools/v3/assert"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestCmdConnectorDelete_ValidateInput(t *testing.T) {
	type test struct {
		name              string
		args              []string
		flags             *common.CommandConnectorDeleteFlags
		cobraGenericFlags map[string]string
		k8sObjects        []runtime.Object
		skupperObjects    []runtime.Object
		expectedErrors    []string
	}

	testTable := []test{
		{
			name:           "connector name is not specified",
			args:           []string{},
			flags:          &common.CommandConnectorDeleteFlags{},
			expectedErrors: []string{"connector name must be configured"},
		},
		{
			name:           "connector name is nil",
			args:           []string{""},
			flags:          &common.CommandConnectorDeleteFlags{},
			expectedErrors: []string{"connector name must not be empty"},
		},
		{
			name:           "connector name is not valid",
			args:           []string{"my name"},
			flags:          &common.CommandConnectorDeleteFlags{},
			expectedErrors: []string{"connector name is not valid: value does not match this regular expression: ^[a-z0-9]([-a-z0-9]*[a-z0-9])*(\\.[a-z0-9]([-a-z0-9]*[a-z0-9])*)*$"},
		},
		{
			name:           "more than one argument is specified",
			args:           []string{"my", "connector"},
			flags:          &common.CommandConnectorDeleteFlags{},
			expectedErrors: []string{"only one argument is allowed for this command"},
		},
		{
			name:           "kubernetes flags are not valid on this platform",
			args:           []string{"my-connector"},
			flags:          &common.CommandConnectorDeleteFlags{},
			expectedErrors: []string{},
			cobraGenericFlags: map[string]string{
				common.FlagNameContext:    "test",
				common.FlagNameKubeconfig: "test",
			},
		},
	}

	for _, test := range testTable {
		t.Run(test.name, func(t *testing.T) {
			command := &CmdConnectorDelete{Flags: &common.CommandConnectorDeleteFlags{}}
			command.CobraCmd = &cobra.Command{Use: "test"}

			if test.flags != nil {
				command.Flags = test.flags
			}

			if test.cobraGenericFlags != nil && len(test.cobraGenericFlags) > 0 {
				for name, value := range test.cobraGenericFlags {
					command.CobraCmd.Flags().String(name, value, "")
				}
			}

			actualErrors := command.ValidateInput(test.args)

			actualErrorsMessages := utils.ErrorsToMessages(actualErrors)

			assert.DeepEqual(t, actualErrorsMessages, test.expectedErrors)

		})
	}
}

func TestCmdConnectorDelete_Run(t *testing.T) {
	type test struct {
		name                string
		namespace           string
		deleteName          string
		k8sObjects          []runtime.Object
		skupperObjects      []runtime.Object
		skupperErrorMessage string
		errorMessage        string
	}

	testTable := []test{
		{
			name:                "run fails default",
			deleteName:          "my-connector",
			skupperErrorMessage: "error",
			errorMessage:        "error",
		},
		{
			name:                "run fails",
			namespace:           "test",
			deleteName:          "my-connector",
			skupperErrorMessage: "error",
			errorMessage:        "error",
		},
	}

	for _, test := range testTable {
		cmd := &CmdConnectorDelete{}

		t.Run(test.name, func(t *testing.T) {

			cmd.connectorName = test.deleteName
			cmd.namespace = test.namespace
			cmd.connectorHandler = fs.NewConnectorHandler(cmd.namespace)
			cmd.InputToOptions()

			err := cmd.Run()
			if err != nil {
				assert.Check(t, test.errorMessage == err.Error())
			} else {
				assert.Check(t, err == nil)
			}
		})
	}
}

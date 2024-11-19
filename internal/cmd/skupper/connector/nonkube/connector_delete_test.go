package nonkube

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/skupperproject/skupper/internal/cmd/skupper/common"
	"github.com/skupperproject/skupper/internal/cmd/skupper/common/utils"
	"github.com/skupperproject/skupper/internal/nonkube/client/fs"
	"github.com/skupperproject/skupper/pkg/apis/skupper/v2alpha1"
	"github.com/skupperproject/skupper/pkg/nonkube/api"
	"github.com/spf13/cobra"
	"gotest.tools/v3/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

	homeDir, err := os.UserHomeDir()
	assert.Check(t, err == nil)
	path := filepath.Join(homeDir, "/.local/share/skupper/namespaces/test/", string(api.InputSiteStatePath))

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
			name:           "connector doesn't exist ",
			args:           []string{"no-connector"},
			flags:          &common.CommandConnectorDeleteFlags{},
			expectedErrors: []string{"connector no-connector does not exist"},
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

	//Add a temp file so connector exists for update tests will pass
	connectorResource := v2alpha1.Connector{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "skupper.io/v2alpha1",
			Kind:       "Connector",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-connector",
			Namespace: "test",
		},
	}

	command := &CmdConnectorDelete{Flags: &common.CommandConnectorDeleteFlags{}}
	command.CobraCmd = &cobra.Command{Use: "test"}
	command.namespace = "test"
	command.connectorHandler = fs.NewConnectorHandler(command.namespace)

	defer command.connectorHandler.Delete("my-connector")
	content, err := command.connectorHandler.EncodeToYaml(connectorResource)
	assert.Check(t, err == nil)
	err = command.connectorHandler.WriteFile(path, "my-connector.yaml", content, common.Connectors)
	assert.Check(t, err == nil)

	for _, test := range testTable {
		t.Run(test.name, func(t *testing.T) {
			command.connectorName = ""
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

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
	"gotest.tools/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestCmdConnectorStatus_ValidateInput(t *testing.T) {
	type test struct {
		name              string
		args              []string
		flags             *common.CommandConnectorStatusFlags
		cobraGenericFlags map[string]string
		k8sObjects        []runtime.Object
		skupperObjects    []runtime.Object
		expectedErrors    []string
	}

	homeDir, err := os.UserHomeDir()
	assert.Check(t, err == nil)
	path := filepath.Join(homeDir, "/.local/share/skupper/namespaces/test/", string(api.RuntimeSiteStatePath))

	testTable := []test{
		{
			name:           "connector is not shown because connector does not exist in the namespace",
			args:           []string{"no-connector"},
			flags:          &common.CommandConnectorStatusFlags{},
			expectedErrors: []string{"connector no-connector does not exist in namespace test"},
		},
		{
			name:           "connector name is nil",
			args:           []string{""},
			flags:          &common.CommandConnectorStatusFlags{},
			expectedErrors: []string{"connector name must not be empty"},
		},
		{
			name:           "more than one argument is specified",
			args:           []string{"my", "connector"},
			flags:          &common.CommandConnectorStatusFlags{},
			expectedErrors: []string{"only one argument is allowed for this command"},
		},
		{
			name:           "connector name is not valid.",
			args:           []string{"my new connector"},
			flags:          &common.CommandConnectorStatusFlags{},
			expectedErrors: []string{"connector name is not valid: value does not match this regular expression: ^[a-z0-9]([-a-z0-9]*[a-z0-9])*(\\.[a-z0-9]([-a-z0-9]*[a-z0-9])*)*$"},
		},
		{
			name:           "no args",
			flags:          &common.CommandConnectorStatusFlags{},
			expectedErrors: []string{},
		},
		{
			name:           "bad output status",
			args:           []string{"my-connector"},
			flags:          &common.CommandConnectorStatusFlags{Output: "not-supported"},
			expectedErrors: []string{"output type is not valid: value not-supported not allowed. It should be one of this options: [json yaml]"},
		},
		{
			name:           "good output status",
			args:           []string{"my-connector"},
			flags:          &common.CommandConnectorStatusFlags{Output: "json"},
			expectedErrors: []string{},
		},
	}

	//Add a temp file so connector exists for status tests
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

	command := &CmdConnectorStatus{}
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

func TestCmdConnectorStatus_Run(t *testing.T) {
	type test struct {
		name                string
		connectorName       string
		flags               common.CommandConnectorStatusFlags
		k8sObjects          []runtime.Object
		skupperObjects      []runtime.Object
		skupperErrorMessage string
		errorMessage        string
	}

	homeDir, err := os.UserHomeDir()
	assert.Check(t, err == nil)
	path := filepath.Join(homeDir, "/.local/share/skupper/namespaces/test/", string(api.InputSiteStatePath))

	testTable := []test{
		{
			name:          "run fails connector doesn't exist",
			connectorName: "no-connector",
			errorMessage:  "failed to read file: open " + path + "/connectors/no-connector.yaml: no such file or directory",
		},
		{
			name:          "runs ok, returns 1 connectors",
			connectorName: "my-connector",
		},
		{
			name:          "runs ok, returns 1 connectors yaml",
			connectorName: "my-connector",
			flags:         common.CommandConnectorStatusFlags{Output: "yaml"},
		},
		{
			name: "runs ok, returns all connectors",
		},
		{
			name:  "runs ok, returns all connectors json",
			flags: common.CommandConnectorStatusFlags{Output: "json"},
		},
		{
			name:         "runs ok, returns all connectors output bad",
			flags:        common.CommandConnectorStatusFlags{Output: "bad-value"},
			errorMessage: "format bad-value not supported",
		},
		{
			name:          "runs ok, returns 1 connectors bad output",
			connectorName: "my-connector",
			flags:         common.CommandConnectorStatusFlags{Output: "bad-value"},
			errorMessage:  "format bad-value not supported",
		},
	}

	//Add a temp file so connector exists for status tests
	connectorResource1 := v2alpha1.Connector{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "skupper.io/v2alpha1",
			Kind:       "Connector",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-connector",
			Namespace: "test",
		},
		Spec: v2alpha1.ConnectorSpec{
			Host:       "1.2.3.4",
			Port:       8080,
			RoutingKey: "backend-8080",
		},
		Status: v2alpha1.ConnectorStatus{
			Status: v2alpha1.Status{
				Conditions: []metav1.Condition{
					{
						Type:   "Configured",
						Status: "True",
					},
				},
			},
		},
	}
	connectorResource2 := v2alpha1.Connector{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "skupper.io/v2alpha1",
			Kind:       "Connector",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-connector2",
			Namespace: "test",
		},
		Spec: v2alpha1.ConnectorSpec{
			Host:       "1.1.1.1",
			Port:       9999,
			RoutingKey: "test-9999",
		},
		Status: v2alpha1.ConnectorStatus{
			Status: v2alpha1.Status{
				Conditions: []metav1.Condition{
					{
						Type:   "Configured",
						Status: "True",
					},
				},
			},
		},
	}

	// add two connectors in runtime directory
	command := &CmdConnectorStatus{}
	command.namespace = "test"
	command.connectorHandler = fs.NewConnectorHandler(command.namespace)

	defer command.connectorHandler.Delete("my-connector")
	defer command.connectorHandler.Delete("my-connector2")

	path = filepath.Join(homeDir, "/.local/share/skupper/namespaces/test/", string(api.RuntimeSiteStatePath))
	content, err := command.connectorHandler.EncodeToYaml(connectorResource1)
	assert.Check(t, err == nil)
	err = command.connectorHandler.WriteFile(path, "my-connector.yaml", content, common.Connectors)
	assert.Check(t, err == nil)

	content, err = command.connectorHandler.EncodeToYaml(connectorResource2)
	assert.Check(t, err == nil)
	err = command.connectorHandler.WriteFile(path, "my-connector2.yaml", content, common.Connectors)
	assert.Check(t, err == nil)

	for _, test := range testTable {
		command.connectorName = test.connectorName
		command.Flags = &test.flags
		command.output = command.Flags.Output

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

func TestCmdConnectorStatus_RunNoDirectory(t *testing.T) {
	type test struct {
		name                string
		flags               common.CommandConnectorStatusFlags
		k8sObjects          []runtime.Object
		skupperObjects      []runtime.Object
		skupperErrorMessage string
		errorMessage        string
	}

	homeDir, err := os.UserHomeDir()
	assert.Check(t, err == nil)
	path := filepath.Join(homeDir, "/.local/share/skupper/namespaces/test1/", string(api.InputSiteStatePath))

	testTable := []test{
		{
			name:         "runs fails no directory",
			errorMessage: "failed to read file: open " + path + "/connectors/my-connector.yaml: no such file or directory",
		},
	}

	for _, test := range testTable {
		command := &CmdConnectorStatus{}
		command.namespace = "test1"
		command.connectorHandler = fs.NewConnectorHandler(command.namespace)
		command.connectorName = "my-connector"
		command.Flags = &test.flags
		command.output = command.Flags.Output
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

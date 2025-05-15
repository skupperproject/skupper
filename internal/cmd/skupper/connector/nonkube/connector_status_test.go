package nonkube

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/skupperproject/skupper/internal/cmd/skupper/common"
	"github.com/skupperproject/skupper/internal/cmd/skupper/common/testutils"
	"github.com/skupperproject/skupper/internal/nonkube/client/fs"
	"github.com/skupperproject/skupper/pkg/apis/skupper/v2alpha1"
	"github.com/skupperproject/skupper/pkg/nonkube/api"
	"gotest.tools/v3/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestCmdConnectorStatus_ValidateInput(t *testing.T) {
	type test struct {
		name              string
		args              []string
		flags             *common.CommandConnectorStatusFlags
		cobraGenericFlags map[string]string
		expectedError     string
	}

	tmpDir := filepath.Join(t.TempDir(), "/skupper")
	err := os.Setenv("SKUPPER_OUTPUT_PATH", tmpDir)
	assert.Check(t, err == nil)
	path := filepath.Join(tmpDir, "/namespaces/test/", string(api.RuntimeSiteStatePath))

	testTable := []test{
		{
			name:          "connector is not shown because connector does not exist in the namespace",
			args:          []string{"no-connector"},
			flags:         &common.CommandConnectorStatusFlags{},
			expectedError: "connector no-connector does not exist in namespace test",
		},
		{
			name:          "connector name is nil",
			args:          []string{""},
			flags:         &common.CommandConnectorStatusFlags{},
			expectedError: "connector name must not be empty",
		},
		{
			name:          "more than one argument is specified",
			args:          []string{"my", "connector"},
			flags:         &common.CommandConnectorStatusFlags{},
			expectedError: "only one argument is allowed for this command",
		},
		{
			name:          "connector name is not valid.",
			args:          []string{"my new connector"},
			flags:         &common.CommandConnectorStatusFlags{},
			expectedError: "connector name is not valid: value does not match this regular expression: ^[a-z0-9]([-a-z0-9]*[a-z0-9])*(\\.[a-z0-9]([-a-z0-9]*[a-z0-9])*)*$",
		},
		{
			name:          "no args",
			flags:         &common.CommandConnectorStatusFlags{},
			expectedError: "",
		},
		{
			name:          "bad output status",
			args:          []string{"my-connector"},
			flags:         &common.CommandConnectorStatusFlags{Output: "not-supported"},
			expectedError: "output type is not valid: value not-supported not allowed. It should be one of this options: [json yaml]",
		},
		{
			name:          "good output status",
			args:          []string{"my-connector"},
			flags:         &common.CommandConnectorStatusFlags{Output: "json"},
			expectedError: "",
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

			testutils.CheckValidateInput(t, command, test.expectedError, test.args)
		})
	}
}

func TestCmdConnectorStatus_Run(t *testing.T) {
	type test struct {
		name          string
		connectorName string
		flags         common.CommandConnectorStatusFlags
		errorMessage  string
	}

	tmpDir := filepath.Join(t.TempDir(), "/skupper")
	err := os.Setenv("SKUPPER_OUTPUT_PATH", tmpDir)
	assert.Check(t, err == nil)
	path := filepath.Join(tmpDir, "/namespaces/test/", string(api.RuntimeSiteStatePath))

	testTable := []test{
		{
			name:          "run fails connector doesn't exist",
			connectorName: "no-connector",
			errorMessage:  "no such file or directory",
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
				assert.Check(t, strings.HasSuffix(err.Error(), test.errorMessage))
			} else {
				assert.NilError(t, err)
			}
		})
	}
}

func TestCmdConnectorStatus_RunNoDirectory(t *testing.T) {
	type test struct {
		name         string
		flags        common.CommandConnectorStatusFlags
		errorMessage string
	}

	tmpDir := filepath.Join(t.TempDir(), "/skupper")
	err := os.Setenv("SKUPPER_OUTPUT_PATH", tmpDir)
	assert.Check(t, err == nil)

	testTable := []test{
		{
			name:         "run function fails because the file does not exist",
			errorMessage: "no such file or directory",
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
				assert.Check(t, strings.HasSuffix(err.Error(), test.errorMessage))
			} else {
				assert.Check(t, err == nil)
			}
		})
	}
}

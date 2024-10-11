package nonkube

import (
	"testing"

	"github.com/skupperproject/skupper/internal/cmd/skupper/common"
	"github.com/skupperproject/skupper/internal/cmd/skupper/common/utils"
	fs2 "github.com/skupperproject/skupper/internal/nonkube/client/fs"
	"github.com/spf13/cobra"

	"gotest.tools/assert"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestNonKubeCmdConnectorCreate_ValidateInput(t *testing.T) {
	type test struct {
		name              string
		args              []string
		k8sObjects        []runtime.Object
		skupperObjects    []runtime.Object
		flags             *common.CommandConnectorCreateFlags
		cobraGenericFlags map[string]string
		expectedErrors    []string
	}

	testTable := []test{
		{
			name:           "Connector name and port are not specified",
			args:           []string{},
			flags:          &common.CommandConnectorCreateFlags{},
			expectedErrors: []string{"connector name and port must be configured"},
		},
		{
			name:           "Connector name is not valid",
			args:           []string{"my new Connector"},
			flags:          &common.CommandConnectorCreateFlags{},
			expectedErrors: []string{"connector name and port must be configured"},
		},
		{
			name:           "Connector name is empty",
			args:           []string{"", "1234"},
			flags:          &common.CommandConnectorCreateFlags{},
			expectedErrors: []string{"connector name must not be empty"},
		},
		{
			name:           "connector port empty",
			args:           []string{"my-name-port-empty", ""},
			flags:          &common.CommandConnectorCreateFlags{},
			expectedErrors: []string{"connector port must not be empty"},
		},
		{
			name:           "port is not valid",
			args:           []string{"my-connector-port", "abcd"},
			flags:          &common.CommandConnectorCreateFlags{},
			expectedErrors: []string{"connector port is not valid: strconv.Atoi: parsing \"abcd\": invalid syntax"},
		},
		{
			name:           "more than two arguments was specified",
			args:           []string{"my", "Connector", "test"},
			flags:          &common.CommandConnectorCreateFlags{},
			expectedErrors: []string{"only two arguments are allowed for this command"},
		},
		{
			name:           "type is not valid",
			args:           []string{"my-connector", "8080"},
			flags:          &common.CommandConnectorCreateFlags{ConnectorType: "not-valid"},
			expectedErrors: []string{"connector type is not valid: value not-valid not allowed. It should be one of this options: [tcp]"},
		},
		{
			name:           "routing key is not valid",
			args:           []string{"my-connector-rk", "8080"},
			flags:          &common.CommandConnectorCreateFlags{RoutingKey: "not-valid$"},
			expectedErrors: []string{"routing key is not valid: value does not match this regular expression: ^[a-z0-9]([-a-z0-9]*[a-z0-9])*(\\.[a-z0-9]([-a-z0-9]*[a-z0-9])*)*$"},
		},
		{
			name:           "output format is not valid",
			args:           []string{"my-connector", "8080"},
			flags:          &common.CommandConnectorCreateFlags{Output: "not-valid"},
			expectedErrors: []string{"output type is not valid: value not-valid not allowed. It should be one of this options: [json yaml]"},
		},
		{
			name:           "kubernetes flags are not valid on this platform",
			args:           []string{"my-connector", "8080"},
			flags:          &common.CommandConnectorCreateFlags{},
			expectedErrors: []string{},
			cobraGenericFlags: map[string]string{
				common.FlagNameContext:    "test",
				common.FlagNameKubeconfig: "test",
			},
		},
		{
			name: "flags all valid",
			args: []string{"my-connector-flags", "8080"},
			flags: &common.CommandConnectorCreateFlags{
				RoutingKey:    "routingkeyname",
				TlsSecret:     "secretname",
				ConnectorType: "tcp",
				Output:        "json",
				Host:          "1.2.3.4",
			},
			expectedErrors: []string{},
		},
	}

	for _, test := range testTable {
		t.Run(test.name, func(t *testing.T) {
			command := &CmdConnectorCreate{Flags: &common.CommandConnectorCreateFlags{}}
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

func TestNonKubeCmdConnectorCreate_InputToOptions(t *testing.T) {

	type test struct {
		name                  string
		args                  []string
		namespace             string
		flags                 common.CommandConnectorCreateFlags
		expectedNamespace     string
		Connectorname         string
		expectedTlsSecret     string
		expectedHost          string
		expectedRoutingKey    string
		expectedConnectorType string
		expectedOutput        string
	}

	testTable := []test{
		{
			name:                  "test1",
			flags:                 common.CommandConnectorCreateFlags{"backend", "", "", "secret", "tcp", false, "", 0, "json"},
			expectedTlsSecret:     "secret",
			expectedHost:          "",
			expectedRoutingKey:    "backend",
			expectedConnectorType: "tcp",
			expectedOutput:        "json",
			expectedNamespace:     "default",
		},
		{
			name:                  "test2",
			namespace:             "test",
			flags:                 common.CommandConnectorCreateFlags{"backend", "1.2.3.4", "", "secret", "tcp", false, "", 0, "json"},
			expectedTlsSecret:     "secret",
			expectedHost:          "1.2.3.4",
			expectedRoutingKey:    "backend",
			expectedConnectorType: "tcp",
			expectedOutput:        "json",
			expectedNamespace:     "test",
		},
		{
			name:                  "test3",
			namespace:             "test",
			flags:                 common.CommandConnectorCreateFlags{"", "", "", "secret", "tcp", false, "", 0, "yaml"},
			expectedTlsSecret:     "secret",
			expectedHost:          "",
			expectedRoutingKey:    "my-Connector",
			expectedConnectorType: "tcp",
			expectedOutput:        "yaml",
			expectedNamespace:     "test",
		},
	}

	for _, test := range testTable {
		t.Run(test.name, func(t *testing.T) {
			cmd := CmdConnectorCreate{}
			cmd.Flags = &test.flags
			cmd.connectorName = "my-Connector"
			cmd.namespace = test.namespace
			cmd.connectorHandler = fs2.NewConnectorHandler(cmd.namespace)

			cmd.InputToOptions()

			assert.Check(t, cmd.routingKey == test.expectedRoutingKey)
			assert.Check(t, cmd.tlsSecret == test.expectedTlsSecret)
			assert.Check(t, cmd.host == test.expectedHost)
			assert.Check(t, cmd.connectorType == test.expectedConnectorType)
			assert.Check(t, cmd.output == test.expectedOutput)
			assert.Check(t, cmd.namespace == test.expectedNamespace)
		})
	}
}

func TestNonKubeCmdConnectorCreate_Run(t *testing.T) {
	type test struct {
		name           string
		namespace      string
		k8sObjects     []runtime.Object
		skupperObjects []runtime.Object
		skupperError   string
		connectorName  string
		host           string
		output         string
		errorMessage   string
		routingKey     string
		tlsSecret      string
		connectorType  string
		connectorPort  int
	}

	testTable := []test{
		{
			name:           "runs ok",
			namespace:      "test",
			k8sObjects:     nil,
			skupperObjects: nil,
			connectorName:  "test1",
			connectorPort:  8080,
			connectorType:  "tcp",
			routingKey:     "keyname",
			tlsSecret:      "secretname",
			host:           "1.2.3.4",
		},
		{
			name:           "runs ok yaml",
			k8sObjects:     nil,
			skupperObjects: nil,
			connectorName:  "test2",
			connectorPort:  8080,
			connectorType:  "tcp",
			host:           "2.2.2.2",
			output:         "yaml",
		},
		{
			name:           "runs fails because the output format is not supported",
			namespace:      "default",
			k8sObjects:     nil,
			skupperObjects: nil,
			connectorName:  "test3",
			connectorPort:  8080,
			host:           "3.3.3.3",
			output:         "unsupported",
			errorMessage:   "format unsupported not supported",
		},
	}

	for _, test := range testTable {
		command := &CmdConnectorCreate{}

		command.connectorName = test.connectorName
		command.output = test.output
		command.port = test.connectorPort
		command.host = test.host
		command.connectorType = test.connectorType
		command.namespace = test.namespace
		command.connectorHandler = fs2.NewConnectorHandler(command.namespace)
		defer command.connectorHandler.Delete("test1")
		t.Run(test.name, func(t *testing.T) {

			err := command.Run()
			if err != nil {
				assert.Check(t, test.errorMessage == err.Error(), err.Error())
			} else {
				assert.Check(t, err == nil)
			}
		})
	}
}

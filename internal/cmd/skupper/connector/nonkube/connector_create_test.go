package nonkube

import (
	"testing"

	"github.com/skupperproject/skupper/internal/cmd/skupper/common"
	"github.com/skupperproject/skupper/internal/cmd/skupper/common/testutils"
	"github.com/skupperproject/skupper/internal/nonkube/client/fs"
	"github.com/spf13/cobra"

	"gotest.tools/v3/assert"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestNonKubeCmdConnectorCreate_ValidateInput(t *testing.T) {
	type test struct {
		name              string
		namespace         string
		args              []string
		k8sObjects        []runtime.Object
		skupperObjects    []runtime.Object
		flags             *common.CommandConnectorCreateFlags
		cobraGenericFlags map[string]string
		expectedError     string
	}

	testTable := []test{
		{
			name:          "Connector name and port are not specified",
			namespace:     "test",
			args:          []string{},
			flags:         &common.CommandConnectorCreateFlags{Host: "1.2.3.4"},
			expectedError: "connector name and port must be configured",
		},
		{
			name:          "Connector name is not valid",
			namespace:     "test",
			args:          []string{"my new Connector", "8080"},
			flags:         &common.CommandConnectorCreateFlags{Host: "1.2.3.4"},
			expectedError: "connector name is not valid: value does not match this regular expression: ^[a-z0-9]([-a-z0-9]*[a-z0-9])*(\\.[a-z0-9]([-a-z0-9]*[a-z0-9])*)*$",
		},
		{
			name:          "Connector name is empty",
			namespace:     "test",
			args:          []string{"", "1234"},
			flags:         &common.CommandConnectorCreateFlags{Host: "1.2.3.4"},
			expectedError: "connector name must not be empty",
		},
		{
			name:          "connector port empty",
			namespace:     "test",
			args:          []string{"my-name-port-empty", ""},
			flags:         &common.CommandConnectorCreateFlags{Host: "1.2.3.4"},
			expectedError: "connector port must not be empty",
		},
		{
			name:          "port is not valid",
			namespace:     "test",
			args:          []string{"my-connector-port", "abcd"},
			flags:         &common.CommandConnectorCreateFlags{Host: "1.2.3.4"},
			expectedError: "connector port is not valid: strconv.Atoi: parsing \"abcd\": invalid syntax",
		},
		{
			name:          "port not positive",
			namespace:     "test",
			args:          []string{"my-port-positive", "-45"},
			flags:         &common.CommandConnectorCreateFlags{Host: "1.2.3.4"},
			expectedError: "connector port is not valid: value is not positive",
		},
		{
			name:          "more than two arguments was specified",
			namespace:     "test",
			args:          []string{"my", "Connector", "test"},
			flags:         &common.CommandConnectorCreateFlags{Host: "1.2.3.4"},
			expectedError: "only two arguments are allowed for this command",
		},
		{
			name:          "type is not valid",
			namespace:     "test",
			args:          []string{"my-connector", "8080"},
			flags:         &common.CommandConnectorCreateFlags{ConnectorType: "not-valid", Host: "1.2.3.4"},
			expectedError: "connector type is not valid: value not-valid not allowed. It should be one of this options: [tcp]",
		},
		{
			name:          "routing key is not valid",
			namespace:     "test",
			args:          []string{"my-connector-rk", "8080"},
			flags:         &common.CommandConnectorCreateFlags{RoutingKey: "not-valid$", Host: "1.2.3.4"},
			expectedError: "routing key is not valid: value does not match this regular expression: ^[a-z0-9]([-a-z0-9]*[a-z0-9])*(\\.[a-z0-9]([-a-z0-9]*[a-z0-9])*)*$",
		},
		{
			name:          "TlsCredentials is not valid",
			args:          []string{"my-connector-tls", "8080"},
			flags:         &common.CommandConnectorCreateFlags{TlsCredentials: "not-valid$", Host: "1.2.3.4"},
			expectedError: "tlsCredentials value is not valid: value does not match this regular expression: ^[a-z0-9]([-a-z0-9]*[a-z0-9])*(\\.[a-z0-9]([-a-z0-9]*[a-z0-9])*)*$",
		},
		{
			name:          "host is not valid",
			args:          []string{"my-connector-host", "8080"},
			flags:         &common.CommandConnectorCreateFlags{Host: "not-valid$"},
			expectedError: "host is not valid: a valid IP address or hostname is expected",
		},
		{
			name:  "host is not configured default",
			args:  []string{"my-connector-host", "8080"},
			flags: &common.CommandConnectorCreateFlags{},
		},
		{
			name:  "kubernetes flags are not valid on this platform",
			args:  []string{"my-connector", "8080"},
			flags: &common.CommandConnectorCreateFlags{Host: "1.2.3.4"},
			cobraGenericFlags: map[string]string{
				common.FlagNameContext:    "test",
				common.FlagNameKubeconfig: "test",
			},
		},
		{
			name:          "invalid namespace",
			namespace:     "TestInvalid",
			args:          []string{"my-connector-invalid", "8080"},
			flags:         &common.CommandConnectorCreateFlags{},
			expectedError: "namespace is not valid: value does not match this regular expression: ^[a-z0-9]([-a-z0-9]*[a-z0-9])?$",
		},
		{
			name:      "flags all valid",
			namespace: "test",
			args:      []string{"my-connector-flags", "8080"},
			flags: &common.CommandConnectorCreateFlags{
				RoutingKey:     "routingkeyname",
				TlsCredentials: "secretname",
				ConnectorType:  "tcp",
				Host:           "1.2.3.4",
			},
			expectedError: "",
		},
	}

	for _, test := range testTable {
		t.Run(test.name, func(t *testing.T) {
			command := &CmdConnectorCreate{Flags: &common.CommandConnectorCreateFlags{}}
			command.CobraCmd = &cobra.Command{Use: "test"}

			if test.flags != nil {
				command.Flags = test.flags
			}
			command.namespace = test.namespace

			if test.cobraGenericFlags != nil && len(test.cobraGenericFlags) > 0 {
				for name, value := range test.cobraGenericFlags {
					command.CobraCmd.Flags().String(name, value, "")
				}
			}

			testutils.CheckValidateInput(t, command, test.expectedError, test.args)
		})
	}
}

func TestNonKubeCmdConnectorCreate_InputToOptions(t *testing.T) {

	type test struct {
		name                   string
		args                   []string
		namespace              string
		flags                  common.CommandConnectorCreateFlags
		expectedNamespace      string
		Connectorname          string
		expectedTlsCredentials string
		expectedHost           string
		expectedRoutingKey     string
		expectedConnectorType  string
	}

	testTable := []test{
		{
			name: "test1",
			flags: common.CommandConnectorCreateFlags{
				RoutingKey:          "backend",
				Host:                "",
				Selector:            "",
				TlsCredentials:      "secret",
				ConnectorType:       "tcp",
				IncludeNotReadyPods: false,
				Workload:            "",
				Timeout:             0,
				Wait:                "none",
			},
			expectedTlsCredentials: "secret",
			expectedHost:           "",
			expectedRoutingKey:     "backend",
			expectedConnectorType:  "tcp",
			expectedNamespace:      "default",
		},
		{
			name:      "test2",
			namespace: "test",
			flags: common.CommandConnectorCreateFlags{
				RoutingKey:          "backend",
				Host:                "1.2.3.4",
				Selector:            "",
				TlsCredentials:      "secret",
				ConnectorType:       "tcp",
				IncludeNotReadyPods: false,
				Workload:            "",
				Timeout:             0,
				Wait:                "configured",
			},
			expectedTlsCredentials: "secret",
			expectedHost:           "1.2.3.4",
			expectedRoutingKey:     "backend",
			expectedConnectorType:  "tcp",
			expectedNamespace:      "test",
		},
		{
			name:      "test3",
			namespace: "test",
			flags: common.CommandConnectorCreateFlags{
				RoutingKey:          "",
				Host:                "localhost",
				Selector:            "",
				TlsCredentials:      "secret",
				ConnectorType:       "tcp",
				IncludeNotReadyPods: false,
				Workload:            "",
				Timeout:             0,
				Wait:                "ready",
			},
			expectedTlsCredentials: "secret",
			expectedHost:           "localhost",
			expectedRoutingKey:     "my-Connector",
			expectedConnectorType:  "tcp",
			expectedNamespace:      "test",
		},
	}

	for _, test := range testTable {
		t.Run(test.name, func(t *testing.T) {
			cmd := CmdConnectorCreate{}
			cmd.Flags = &test.flags
			cmd.connectorName = "my-Connector"
			cmd.namespace = test.namespace
			cmd.connectorHandler = fs.NewConnectorHandler(cmd.namespace)

			cmd.InputToOptions()

			assert.Check(t, cmd.routingKey == test.expectedRoutingKey)
			assert.Check(t, cmd.tlsCredentials == test.expectedTlsCredentials)
			assert.Check(t, cmd.host == test.expectedHost)
			assert.Check(t, cmd.connectorType == test.expectedConnectorType)
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
		errorMessage   string
		routingKey     string
		tlsCredentials string
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
			tlsCredentials: "secretname",
			host:           "1.2.3.4",
		},
	}

	for _, test := range testTable {
		command := &CmdConnectorCreate{}

		command.connectorName = test.connectorName
		command.port = test.connectorPort
		command.host = test.host
		command.connectorType = test.connectorType
		command.namespace = test.namespace
		command.connectorHandler = fs.NewConnectorHandler(command.namespace)
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

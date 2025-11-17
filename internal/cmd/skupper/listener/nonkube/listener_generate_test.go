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

func TestNonKubeCmdListenerGenerate_ValidateInput(t *testing.T) {
	type test struct {
		name              string
		namespace         string
		args              []string
		k8sObjects        []runtime.Object
		skupperObjects    []runtime.Object
		flags             *common.CommandListenerGenerateFlags
		cobraGenericFlags map[string]string
		expectedError     string
	}

	testTable := []test{
		{
			name:          "listener name and port are not specified",
			namespace:     "test",
			args:          []string{},
			flags:         &common.CommandListenerGenerateFlags{Host: "1.2.3.4"},
			expectedError: "listener name and port must be configured",
		},
		{
			name:          "listener name is not valid",
			namespace:     "test",
			args:          []string{"my new Listener", "8080"},
			flags:         &common.CommandListenerGenerateFlags{Host: "1.2.3.4"},
			expectedError: "listener name is not valid: value does not match this regular expression: ^[a-z0-9]([-a-z0-9]*[a-z0-9])*(\\.[a-z0-9]([-a-z0-9]*[a-z0-9])*)*$",
		},
		{
			name:          "listener name is empty",
			namespace:     "test",
			args:          []string{"", "1234"},
			flags:         &common.CommandListenerGenerateFlags{Host: "1.2.3.4"},
			expectedError: "listener name must not be empty",
		},
		{
			name:          "listener port empty",
			args:          []string{"my-name-port-empty", ""},
			flags:         &common.CommandListenerGenerateFlags{Host: "1.2.3.4"},
			expectedError: "listener port must not be empty",
		},
		{
			name:          "port is not valid",
			args:          []string{"my-listener-port", "abcd"},
			flags:         &common.CommandListenerGenerateFlags{Host: "1.2.3.4"},
			expectedError: "listener port is not valid: strconv.Atoi: parsing \"abcd\": invalid syntax",
		},
		{
			name:          "listener port not positive",
			args:          []string{"my-port-positive", "-45"},
			flags:         &common.CommandListenerGenerateFlags{Host: "1.2.3.4"},
			expectedError: "listener port is not valid: value is not positive",
		},
		{
			name:          "more than two arguments was specified",
			args:          []string{"my", "listener", "test"},
			flags:         &common.CommandListenerGenerateFlags{Host: "1.2.3.4"},
			expectedError: "only two arguments are allowed for this command",
		},
		{
			name:          "type is not valid",
			args:          []string{"my-listener", "8080"},
			flags:         &common.CommandListenerGenerateFlags{ListenerType: "not-valid", Host: "1.2.3.4"},
			expectedError: "listener type is not valid: value not-valid not allowed. It should be one of this options: [tcp]",
		},
		{
			name:          "routing key is not valid",
			args:          []string{"my-listener-rk", "8080"},
			flags:         &common.CommandListenerGenerateFlags{RoutingKey: "not-valid$", Host: "1.2.3.4"},
			expectedError: "routing key is not valid: value does not match this regular expression: ^[a-z0-9]([-a-z0-9]*[a-z0-9])*(\\.[a-z0-9]([-a-z0-9]*[a-z0-9])*)*$",
		},
		{
			name:          "TlsCredentials key is not valid",
			args:          []string{"my-listener-tls", "8080"},
			flags:         &common.CommandListenerGenerateFlags{TlsCredentials: "not-valid$", Host: "1.2.3.4"},
			expectedError: "tlsCredentials is not valid: value does not match this regular expression: ^[a-z0-9]([-a-z0-9]*[a-z0-9])*(\\.[a-z0-9]([-a-z0-9]*[a-z0-9])*)*$",
		},
		{
			name:          "host is not valid",
			args:          []string{"my-listener-host", "8080"},
			flags:         &common.CommandListenerGenerateFlags{Host: "not-valid$"},
			expectedError: "host is not valid: a valid IP address or hostname is expected",
		},
		{
			name:          "output format is not valid",
			args:          []string{"my-listener", "8080"},
			flags:         &common.CommandListenerGenerateFlags{Output: "not-valid", Host: "1.2.3.4"},
			expectedError: "output type is not valid: value not-valid not allowed. It should be one of this options: [json yaml]",
		},
		{
			name:  "kubernetes flags are not valid on this platform",
			args:  []string{"my-listener", "8080"},
			flags: &common.CommandListenerGenerateFlags{Host: "1.2.3.4"},
			cobraGenericFlags: map[string]string{
				common.FlagNameContext:    "test",
				common.FlagNameKubeconfig: "test",
			},
		},
		{
			name:          "invalid namespace",
			namespace:     "TestInvalid",
			args:          []string{"my-listener", "8080"},
			flags:         &common.CommandListenerGenerateFlags{Host: "1.2.3.4"},
			expectedError: "namespace is not valid: value does not match this regular expression: ^[a-z0-9]([-a-z0-9]*[a-z0-9])?$",
		},
		{
			name: "flags all valid",
			args: []string{"my-listener-flags", "8080"},
			flags: &common.CommandListenerGenerateFlags{
				RoutingKey:     "routingkeyname",
				TlsCredentials: "secretname",
				ListenerType:   "tcp",
				Output:         "json",
				Host:           "1.2.3.4",
			},
		},
	}

	for _, test := range testTable {
		t.Run(test.name, func(t *testing.T) {
			command := &CmdListenerGenerate{Flags: &common.CommandListenerGenerateFlags{}}
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

func TestNonKubeCmdListenerGenerate_InputToOptions(t *testing.T) {

	type test struct {
		name                   string
		args                   []string
		namespace              string
		flags                  common.CommandListenerGenerateFlags
		expectedNamespace      string
		listenerName           string
		expectedTlsCredentials string
		expectedHost           string
		expectedRoutingKey     string
		expectedListenerType   string
		expectedOutput         string
	}

	testTable := []test{
		{
			name: "test1",
			flags: common.CommandListenerGenerateFlags{
				RoutingKey:     "backend",
				Host:           "",
				TlsCredentials: "secret",
				ListenerType:   "tcp",
				Output:         "json"},
			expectedTlsCredentials: "secret",
			expectedHost:           "0.0.0.0",
			expectedRoutingKey:     "backend",
			expectedListenerType:   "tcp",
			expectedOutput:         "json",
			expectedNamespace:      "default",
		},
		{
			name:      "test2",
			namespace: "test",
			flags: common.CommandListenerGenerateFlags{
				RoutingKey:     "backend",
				Host:           "1.2.3.4",
				TlsCredentials: "secret",
				ListenerType:   "tcp",
				Output:         "json",
			},
			expectedTlsCredentials: "secret",
			expectedHost:           "1.2.3.4",
			expectedRoutingKey:     "backend",
			expectedListenerType:   "tcp",
			expectedOutput:         "json",
			expectedNamespace:      "test",
		},
		{
			name:      "test3",
			namespace: "default",
			flags: common.CommandListenerGenerateFlags{
				RoutingKey:     "",
				Host:           "",
				TlsCredentials: "secret",
				ListenerType:   "tcp",
				Output:         "yaml"},
			expectedTlsCredentials: "secret",
			expectedHost:           "0.0.0.0",
			expectedRoutingKey:     "my-listener",
			expectedListenerType:   "tcp",
			expectedOutput:         "yaml",
			expectedNamespace:      "default",
		},
	}

	for _, test := range testTable {
		t.Run(test.name, func(t *testing.T) {
			cmd := CmdListenerGenerate{}
			cmd.Flags = &test.flags
			cmd.listenerName = "my-listener"
			cmd.namespace = test.namespace
			cmd.listenerHandler = fs.NewListenerHandler(cmd.namespace)

			cmd.InputToOptions()

			assert.Check(t, cmd.routingKey == test.expectedRoutingKey)
			assert.Check(t, cmd.tlsCredentials == test.expectedTlsCredentials)
			assert.Check(t, cmd.host == test.expectedHost)
			assert.Check(t, cmd.listenerType == test.expectedListenerType)
			assert.Check(t, cmd.output == test.expectedOutput)
			assert.Check(t, cmd.namespace == test.expectedNamespace)
		})
	}
}

func TestNonKubeCmdListenerGenerate_Run(t *testing.T) {
	type test struct {
		name           string
		k8sObjects     []runtime.Object
		skupperObjects []runtime.Object
		skupperError   string
		listenerName   string
		host           string
		output         string
		errorMessage   string
		routingKey     string
		tlsCredentials string
		listenerType   string
		listenerPort   int
	}

	testTable := []test{
		{
			name:           "runs ok",
			k8sObjects:     nil,
			skupperObjects: nil,
			listenerName:   "test1",
			listenerPort:   8080,
			listenerType:   "tcp",
			routingKey:     "keyname",
			tlsCredentials: "secretname",
			host:           "1.2.3.4",
			output:         "json",
		},
		{
			name:           "runs ok yaml",
			k8sObjects:     nil,
			skupperObjects: nil,
			listenerName:   "test2",
			listenerPort:   8080,
			listenerType:   "tcp",
			host:           "2.2.2.2",
			output:         "yaml",
		},
		{
			name:           "runs fails because the output format is not supported",
			k8sObjects:     nil,
			skupperObjects: nil,
			listenerName:   "test3",
			listenerPort:   8080,
			host:           "3.3.3.3",
			output:         "unsupported",
			errorMessage:   "format unsupported not supported",
		},
	}

	for _, test := range testTable {
		command := &CmdListenerGenerate{}

		command.listenerName = test.listenerName
		command.output = test.output
		command.port = test.listenerPort
		command.host = test.host
		command.listenerType = test.listenerType
		command.namespace = "test"
		command.listenerHandler = fs.NewListenerHandler(command.namespace)
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

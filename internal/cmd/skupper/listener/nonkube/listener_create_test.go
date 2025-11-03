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

func TestNonKubeCmdListenerCreate_ValidateInput(t *testing.T) {
	type test struct {
		name              string
		namespace         string
		args              []string
		k8sObjects        []runtime.Object
		skupperObjects    []runtime.Object
		flags             *common.CommandListenerCreateFlags
		cobraGenericFlags map[string]string
		expectedError     string
	}

	testTable := []test{
		{
			name:          "listener name and port are not specified",
			namespace:     "test",
			args:          []string{},
			flags:         &common.CommandListenerCreateFlags{Host: "1.2.3.4"},
			expectedError: "listener name and port must be configured",
		},
		{
			name:          "listener name is not valid",
			namespace:     "test",
			args:          []string{"my new Listener", "8080"},
			flags:         &common.CommandListenerCreateFlags{Host: "1.2.3.4"},
			expectedError: "listener name is not valid: value does not match this regular expression: ^[a-z0-9]([-a-z0-9]*[a-z0-9])*(\\.[a-z0-9]([-a-z0-9]*[a-z0-9])*)*$",
		},
		{
			name:          "listener name is empty",
			namespace:     "test",
			args:          []string{"", "1234"},
			flags:         &common.CommandListenerCreateFlags{Host: "1.2.3.4"},
			expectedError: "listener name must not be empty",
		},
		{
			name:          "listener port empty",
			args:          []string{"my-name-port-empty", ""},
			flags:         &common.CommandListenerCreateFlags{Host: "1.2.3.4"},
			expectedError: "listener port must not be empty",
		},
		{
			name:          "port is not valid",
			args:          []string{"my-listener-port", "abcd"},
			flags:         &common.CommandListenerCreateFlags{Host: "1.2.3.4"},
			expectedError: "listener port is not valid: strconv.Atoi: parsing \"abcd\": invalid syntax",
		},
		{
			name:          "listener port not positive",
			args:          []string{"my-port-positive", "-45"},
			flags:         &common.CommandListenerCreateFlags{Host: "1.2.3.4"},
			expectedError: "listener port is not valid: value is not positive",
		},
		{
			name:          "more than two arguments was specified",
			args:          []string{"my", "listener", "test"},
			flags:         &common.CommandListenerCreateFlags{Host: "1.2.3.4"},
			expectedError: "only two arguments are allowed for this command",
		},
		{
			name:          "type is not valid",
			args:          []string{"my-listener", "8080"},
			flags:         &common.CommandListenerCreateFlags{ListenerType: "not-valid", Host: "1.2.3.4"},
			expectedError: "listener type is not valid: value not-valid not allowed. It should be one of this options: [tcp]",
		},
		{
			name:          "routing key is not valid",
			args:          []string{"my-listener-rk", "8080"},
			flags:         &common.CommandListenerCreateFlags{RoutingKey: "not-valid$", Host: "1.2.3.4"},
			expectedError: "routing key is not valid: value does not match this regular expression: ^[a-z0-9]([-a-z0-9]*[a-z0-9])*(\\.[a-z0-9]([-a-z0-9]*[a-z0-9])*)*$",
		},
		{
			name:          "TlsCredentials key is not valid",
			args:          []string{"my-listener-tls", "8080"},
			flags:         &common.CommandListenerCreateFlags{TlsCredentials: "not-valid$", Host: "1.2.3.4"},
			expectedError: "tlsCredentials value is not valid: value does not match this regular expression: ^[a-z0-9]([-a-z0-9]*[a-z0-9])*(\\.[a-z0-9]([-a-z0-9]*[a-z0-9])*)*$",
		},
		{
			name:          "host is not valid",
			args:          []string{"my-listener-host", "8080"},
			flags:         &common.CommandListenerCreateFlags{Host: "not-valid$"},
			expectedError: "host is not valid: a valid IP address or hostname is expected",
		},
		{
			name:  "kubernetes flags are not valid on this platform",
			args:  []string{"my-listener", "8080"},
			flags: &common.CommandListenerCreateFlags{Host: "1.2.3.4"},
			cobraGenericFlags: map[string]string{
				common.FlagNameContext:    "test",
				common.FlagNameKubeconfig: "test",
			},
		},
		{
			name:          "invalid namespace",
			namespace:     "TestInvalid",
			args:          []string{"my-listener-invalid", "8080"},
			flags:         &common.CommandListenerCreateFlags{},
			expectedError: "namespace is not valid: value does not match this regular expression: ^[a-z0-9]([-a-z0-9]*[a-z0-9])?$",
		},
		{
			name: "flags all valid",
			args: []string{"my-listener-flags", "8080"},
			flags: &common.CommandListenerCreateFlags{
				RoutingKey:     "routingkeyname",
				TlsCredentials: "secretname",
				ListenerType:   "tcp",
				Host:           "1.2.3.4",
			},
		},
	}

	for _, test := range testTable {
		t.Run(test.name, func(t *testing.T) {
			command := &CmdListenerCreate{Flags: &common.CommandListenerCreateFlags{}}
			command.CobraCmd = &cobra.Command{Use: "test"}
			command.namespace = test.namespace

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

func TestNonKubeCmdListenerCreate_InputToOptions(t *testing.T) {

	type test struct {
		name                   string
		args                   []string
		namespace              string
		flags                  common.CommandListenerCreateFlags
		expectedNamespace      string
		listenerName           string
		expectedTlsCredentials string
		expectedHost           string
		expectedRoutingKey     string
		expectedListenerType   string
	}

	testTable := []test{
		{
			name: "test1",
			flags: common.CommandListenerCreateFlags{
				RoutingKey:     "backend",
				Host:           "",
				TlsCredentials: "secret",
				ListenerType:   "tcp",
				Timeout:        0,
				Wait:           "none",
			},
			expectedTlsCredentials: "secret",
			expectedHost:           "0.0.0.0",
			expectedRoutingKey:     "backend",
			expectedListenerType:   "tcp",
			expectedNamespace:      "default",
		},
		{
			name:      "test2",
			namespace: "test",
			flags: common.CommandListenerCreateFlags{
				RoutingKey:     "backend",
				Host:           "1.2.3.4",
				TlsCredentials: "secret",
				ListenerType:   "tcp",
				Timeout:        0,
				Wait:           "configured",
			},
			expectedTlsCredentials: "secret",
			expectedHost:           "1.2.3.4",
			expectedRoutingKey:     "backend",
			expectedListenerType:   "tcp",
			expectedNamespace:      "test",
		},
		{
			name:      "test3",
			namespace: "default",
			flags: common.CommandListenerCreateFlags{
				RoutingKey:     "",
				Host:           "",
				TlsCredentials: "secret",
				ListenerType:   "tcp",
				Timeout:        0,
				Wait:           "ready",
			},
			expectedTlsCredentials: "secret",
			expectedHost:           "0.0.0.0",
			expectedRoutingKey:     "my-listener",
			expectedListenerType:   "tcp",
			expectedNamespace:      "default",
		},
	}

	for _, test := range testTable {
		t.Run(test.name, func(t *testing.T) {
			cmd := CmdListenerCreate{}
			cmd.Flags = &test.flags
			cmd.listenerName = "my-listener"
			cmd.namespace = test.namespace
			cmd.listenerHandler = fs.NewListenerHandler(cmd.namespace)

			cmd.InputToOptions()

			assert.Check(t, cmd.routingKey == test.expectedRoutingKey)
			assert.Check(t, cmd.tlsCredentials == test.expectedTlsCredentials)
			assert.Check(t, cmd.host == test.expectedHost)
			assert.Check(t, cmd.listenerType == test.expectedListenerType)
			assert.Check(t, cmd.namespace == test.expectedNamespace)
		})
	}
}

func TestNonKubeCmdListenerCreate_Run(t *testing.T) {
	type test struct {
		name           string
		k8sObjects     []runtime.Object
		skupperObjects []runtime.Object
		skupperError   string
		listenerName   string
		host           string
		errorMessage   string
		routingKey     string
		tlsCredentials string
		listenerType   string
		listenerPort   int
	}

	testTable := []test{
		{
			name:           "test1",
			k8sObjects:     nil,
			skupperObjects: nil,
			listenerName:   "test1",
			listenerPort:   8080,
			listenerType:   "tcp",
			routingKey:     "keyname",
			tlsCredentials: "secretname",
			host:           "1.2.3.4",
		},
		{
			name:           "test2",
			k8sObjects:     nil,
			skupperObjects: nil,
			listenerName:   "test2",
			listenerPort:   8080,
			listenerType:   "tcp",
			host:           "2.2.2.2",
		},
		{
			name:           "test3",
			k8sObjects:     nil,
			skupperObjects: nil,
			listenerName:   "test3",
			listenerPort:   8080,
		},
	}

	for _, test := range testTable {
		command := &CmdListenerCreate{}

		command.listenerName = test.listenerName
		command.port = test.listenerPort
		command.host = test.host
		command.listenerType = test.listenerType
		command.namespace = "test"
		command.listenerHandler = fs.NewListenerHandler(command.namespace)
		defer command.listenerHandler.Delete("test1")
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

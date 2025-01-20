package kube

import (
	"testing"

	"github.com/skupperproject/skupper/internal/cmd/skupper/common"
	"github.com/skupperproject/skupper/internal/cmd/skupper/common/testutils"

	"github.com/skupperproject/skupper/pkg/apis/skupper/v2alpha1"
	"gotest.tools/v3/assert"
	v12 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestCmdListenerGenerate_ValidateInput(t *testing.T) {
	type test struct {
		name                string
		args                []string
		flags               common.CommandListenerGenerateFlags
		k8sObjects          []runtime.Object
		skupperObjects      []runtime.Object
		skupperErrorMessage string
		expectedError       string
	}

	testTable := []test{
		{
			name:          "listener name and port are not specified",
			args:          []string{},
			flags:         common.CommandListenerGenerateFlags{},
			expectedError: "listener name and port must be configured",
		},
		{
			name:          "listener name empty",
			args:          []string{"", "8090"},
			flags:         common.CommandListenerGenerateFlags{},
			expectedError: "listener name must not be empty",
		},
		{
			name:          "listener port empty",
			args:          []string{"my-name-port-empty", ""},
			flags:         common.CommandListenerGenerateFlags{},
			expectedError: "listener port must not be empty",
		},
		{
			name:          "listener port not positive",
			args:          []string{"my-port-positive", "-45"},
			flags:         common.CommandListenerGenerateFlags{},
			expectedError: "listener port is not valid: value is not positive",
		},
		{
			name:          "listener name and port are not specified",
			args:          []string{},
			flags:         common.CommandListenerGenerateFlags{},
			expectedError: "listener name and port must be configured",
		},
		{
			name:          "listener port is not specified",
			args:          []string{"my-name"},
			flags:         common.CommandListenerGenerateFlags{},
			expectedError: "listener name and port must be configured",
		},
		{
			name:          "more than two arguments are specified",
			args:          []string{"my", "listener", "8080"},
			flags:         common.CommandListenerGenerateFlags{},
			expectedError: "only two arguments are allowed for this command",
		},
		{
			name:          "listener name is not valid.",
			args:          []string{"my new listener", "8080"},
			flags:         common.CommandListenerGenerateFlags{},
			expectedError: "listener name is not valid: value does not match this regular expression: ^[a-z0-9]([-a-z0-9]*[a-z0-9])*(\\.[a-z0-9]([-a-z0-9]*[a-z0-9])*)*$",
		},
		{
			name:          "port is not valid.",
			args:          []string{"my-listener-port", "abcd"},
			flags:         common.CommandListenerGenerateFlags{},
			expectedError: "listener port is not valid: strconv.Atoi: parsing \"abcd\": invalid syntax",
		},
		{
			name:          "listener type is not valid",
			args:          []string{"my-listener-type", "8080"},
			flags:         common.CommandListenerGenerateFlags{ListenerType: "not-valid"},
			expectedError: "listener type is not valid: value not-valid not allowed. It should be one of this options: [tcp]",
		},
		{
			name:          "routing key is not valid",
			args:          []string{"my-listener-rk", "8080"},
			flags:         common.CommandListenerGenerateFlags{RoutingKey: "not-valid$"},
			expectedError: "routing key is not valid: value does not match this regular expression: ^[a-z0-9]([-a-z0-9]*[a-z0-9])*(\\.[a-z0-9]([-a-z0-9]*[a-z0-9])*)*$",
		},
		{
			name:          "tls-credentials does not exist",
			args:          []string{"my-listener-tls", "8080"},
			flags:         common.CommandListenerGenerateFlags{TlsCredentials: "not-&valid"},
			expectedError: "tlsCredentials is not valid: value does not match this regular expression: ^[a-z0-9]([-a-z0-9]*[a-z0-9])*(\\.[a-z0-9]([-a-z0-9]*[a-z0-9])*)*$",
		},
		{
			name:          "output is not valid",
			args:          []string{"bad-output", "1234"},
			flags:         common.CommandListenerGenerateFlags{Output: "not-supported"},
			expectedError: "output type is not valid: value not-supported not allowed. It should be one of this options: [json yaml]",
		},
		{
			name: "flags all valid",
			args: []string{"my-listener-flags", "8080"},
			flags: common.CommandListenerGenerateFlags{
				Host:           "hostname",
				RoutingKey:     "routingkeyname",
				TlsCredentials: "secretname",
				ListenerType:   "tcp",
				Output:         "json",
			},
			skupperObjects: []runtime.Object{
				&v2alpha1.Listener{
					ObjectMeta: v1.ObjectMeta{
						Name:      "my-listener",
						Namespace: "test",
					},
					Status: v2alpha1.ListenerStatus{
						Status: v2alpha1.Status{
							Conditions: []v1.Condition{
								{
									Type:   "Configured",
									Status: "True",
								},
							},
						},
					},
				},
			},
			k8sObjects: []runtime.Object{
				&v12.Secret{
					ObjectMeta: v1.ObjectMeta{
						Name:      "secretname",
						Namespace: "test",
					},
				},
			},
		},
	}

	for _, test := range testTable {
		t.Run(test.name, func(t *testing.T) {

			command, err := newCmdListenerGenerateWithMocks("test", test.k8sObjects, test.skupperObjects, "")
			assert.Assert(t, err)

			command.Flags = &test.flags

			testutils.CheckValidateInput(t, command, test.expectedError, test.args)

		})
	}
}

func TestCmdListenerGenerate_InputToOptions(t *testing.T) {

	type test struct {
		name                   string
		flags                  common.CommandListenerGenerateFlags
		Listenername           string
		expectedTlsCredentials string
		expectedHost           string
		expectedRoutingKey     string
		expectedListenerType   string
		expectedOutput         string
	}

	testTable := []test{
		{
			name:                   "test1",
			flags:                  common.CommandListenerGenerateFlags{"backend", "backend", "secret", "tcp", "json"},
			expectedTlsCredentials: "secret",
			expectedHost:           "backend",
			expectedRoutingKey:     "backend",
			expectedListenerType:   "tcp",
			expectedOutput:         "json",
		},
		{
			name:                   "test2",
			flags:                  common.CommandListenerGenerateFlags{"", "", "secret", "tcp", "yaml"},
			expectedTlsCredentials: "secret",
			expectedHost:           "test2",
			expectedRoutingKey:     "test2",
			expectedListenerType:   "tcp",
			expectedOutput:         "yaml",
		},
	}

	for _, test := range testTable {
		t.Run(test.name, func(t *testing.T) {

			cmd, err := newCmdListenerGenerateWithMocks("test", nil, nil, "")
			assert.Assert(t, err)

			cmd.Flags = &test.flags
			cmd.name = test.name

			cmd.InputToOptions()

			assert.Check(t, cmd.routingKey == test.expectedRoutingKey)
			assert.Check(t, cmd.output == test.expectedOutput)
			assert.Check(t, cmd.tlsCredentials == test.expectedTlsCredentials)
			assert.Check(t, cmd.host == test.expectedHost)
			assert.Check(t, cmd.output == test.expectedOutput)
			assert.Check(t, cmd.listenerType == test.expectedListenerType)
		})
	}
}

func TestCmdListenerGenerate_Run(t *testing.T) {
	type test struct {
		name                string
		listenerName        string
		listenerPort        int
		flags               common.CommandListenerGenerateFlags
		k8sObjects          []runtime.Object
		skupperObjects      []runtime.Object
		skupperErrorMessage string
		errorMessage        string
	}

	testTable := []test{
		{
			name:         "runs ok",
			listenerName: "run-listener",
			listenerPort: 8080,
			flags: common.CommandListenerGenerateFlags{
				ListenerType:   "tcp",
				Host:           "hostname",
				RoutingKey:     "keyname",
				TlsCredentials: "secretname",
				Output:         "json",
			},
		},
		{
			name:         "output yaml",
			listenerName: "run-listener",
			listenerPort: 8080,
			flags: common.CommandListenerGenerateFlags{
				ListenerType:   "tcp",
				Host:           "hostname",
				RoutingKey:     "keyname",
				TlsCredentials: "secretname",
				Output:         "yaml",
			},
		},
	}

	for _, test := range testTable {
		cmd, err := newCmdListenerGenerateWithMocks("test", test.k8sObjects, test.skupperObjects, test.skupperErrorMessage)
		assert.Assert(t, err)
		cmd.name = test.listenerName
		cmd.port = test.listenerPort
		cmd.Flags = &test.flags
		cmd.output = cmd.Flags.Output
		cmd.namespace = "test"

		t.Run(test.name, func(t *testing.T) {
			err := cmd.Run()
			if err != nil {
				assert.Check(t, test.errorMessage == err.Error())
			} else {
				assert.Check(t, err == nil)
			}
		})
	}
}

// --- helper methods

func newCmdListenerGenerateWithMocks(namespace string, k8sObjects []runtime.Object, skupperObjects []runtime.Object, fakeSkupperError string) (*CmdListenerGenerate, error) {

	cmdListenerGenerate := &CmdListenerGenerate{
		namespace: namespace,
	}
	return cmdListenerGenerate, nil
}

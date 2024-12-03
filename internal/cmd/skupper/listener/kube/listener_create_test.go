package kube

import (
	"testing"
	"time"

	"github.com/skupperproject/skupper/internal/cmd/skupper/common"
	"github.com/skupperproject/skupper/internal/cmd/skupper/common/utils"

	fakeclient "github.com/skupperproject/skupper/internal/kube/client/fake"
	"github.com/skupperproject/skupper/pkg/apis/skupper/v2alpha1"
	"gotest.tools/v3/assert"
	v12 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestCmdListenerCreate_ValidateInput(t *testing.T) {
	type test struct {
		name                string
		args                []string
		flags               common.CommandListenerCreateFlags
		k8sObjects          []runtime.Object
		skupperObjects      []runtime.Object
		skupperErrorMessage string
		expectedErrors      []string
	}

	testTable := []test{
		{
			name:  "listener is not created because there is already the same listener in the namespace",
			args:  []string{"my-listener", "8080"},
			flags: common.CommandListenerCreateFlags{Timeout: 1 * time.Minute},
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
			skupperErrorMessage: "AllReadyExists",
			expectedErrors: []string{
				"there is already a listener my-listener created for namespace test"},
		},
		{
			name:           "listener name and port are not specified",
			args:           []string{},
			flags:          common.CommandListenerCreateFlags{Timeout: 1 * time.Minute},
			expectedErrors: []string{"listener name and port must be configured"},
		},
		{
			name:           "listener name empty",
			args:           []string{"", "8090"},
			flags:          common.CommandListenerCreateFlags{Timeout: 1 * time.Minute},
			expectedErrors: []string{"listener name must not be empty"},
		},
		{
			name:           "listener port empty",
			args:           []string{"my-name-port-empty", ""},
			flags:          common.CommandListenerCreateFlags{Timeout: 1 * time.Minute},
			expectedErrors: []string{"listener port must not be empty"},
		},
		{
			name:           "listener port not positive",
			args:           []string{"my-port-positive", "-45"},
			flags:          common.CommandListenerCreateFlags{Timeout: 1 * time.Minute},
			expectedErrors: []string{"listener port is not valid: value is not positive"},
		},
		{
			name:           "listener name and port are not specified",
			args:           []string{},
			flags:          common.CommandListenerCreateFlags{Timeout: 1 * time.Minute},
			expectedErrors: []string{"listener name and port must be configured"},
		},
		{
			name:           "listener port is not specified",
			args:           []string{"my-name"},
			flags:          common.CommandListenerCreateFlags{Timeout: 1 * time.Minute},
			expectedErrors: []string{"listener name and port must be configured"},
		},
		{
			name:           "more than two arguments are specified",
			args:           []string{"my", "listener", "8080"},
			flags:          common.CommandListenerCreateFlags{Timeout: 1 * time.Minute},
			expectedErrors: []string{"only two arguments are allowed for this command"},
		},
		{
			name:  "listener name is not valid.",
			args:  []string{"my new listener", "8080"},
			flags: common.CommandListenerCreateFlags{Timeout: 1 * time.Minute},
			expectedErrors: []string{
				"listener name is not valid: value does not match this regular expression: ^[a-z0-9]([-a-z0-9]*[a-z0-9])*(\\.[a-z0-9]([-a-z0-9]*[a-z0-9])*)*$"},
		},
		{
			name:  "port is not valid.",
			args:  []string{"my-listener-port", "abcd"},
			flags: common.CommandListenerCreateFlags{Timeout: 1 * time.Minute},
			expectedErrors: []string{
				"listener port is not valid: strconv.Atoi: parsing \"abcd\": invalid syntax"},
		},
		{
			name: "listener type is not valid",
			args: []string{"my-listener-type", "8080"},
			flags: common.CommandListenerCreateFlags{
				Timeout:      1 * time.Minute,
				ListenerType: "not-valid",
			},
			expectedErrors: []string{
				"listener type is not valid: value not-valid not allowed. It should be one of this options: [tcp]"},
		},
		{
			name: "routing key is not valid",
			args: []string{"my-listener-rk", "8080"},
			flags: common.CommandListenerCreateFlags{
				Timeout:    60 * time.Second,
				RoutingKey: "not-valid$",
			},
			expectedErrors: []string{
				"routing key is not valid: value does not match this regular expression: ^[a-z0-9]([-a-z0-9]*[a-z0-9])*(\\.[a-z0-9]([-a-z0-9]*[a-z0-9])*)*$"},
		},
		{
			name: "tls-secret does not exist",
			args: []string{"my-listener-tls", "8080"},
			flags: common.CommandListenerCreateFlags{
				Timeout:        1 * time.Minute,
				TlsCredentials: "not-valid",
			},
			expectedErrors: []string{"tls-secret is not valid: does not exist"},
		},
		{
			name:           "timeout is not valid",
			args:           []string{"bad-timeout", "8080"},
			flags:          common.CommandListenerCreateFlags{Timeout: 0 * time.Second},
			expectedErrors: []string{"timeout is not valid: duration must not be less than 10s; got 0s"},
		},
		{
			name: "output is not valid",
			args: []string{"bad-output", "1234"},
			flags: common.CommandListenerCreateFlags{
				Timeout: 30 * time.Second,
				Output:  "not-supported",
			},
			expectedErrors: []string{
				"output type is not valid: value not-supported not allowed. It should be one of this options: [json yaml]"},
		},
		{
			name: "flags all valid",
			args: []string{"my-listener-flags", "8080"},
			flags: common.CommandListenerCreateFlags{
				Host:           "hostname",
				RoutingKey:     "routingkeyname",
				TlsCredentials: "secretname",
				ListenerType:   "tcp",
				Timeout:        1 * time.Minute,
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
			expectedErrors: []string{},
		},
	}

	for _, test := range testTable {
		t.Run(test.name, func(t *testing.T) {

			command, err := newCmdListenerCreateWithMocks("test", test.k8sObjects, test.skupperObjects, "")
			assert.Assert(t, err)

			command.Flags = &test.flags

			actualErrors := command.ValidateInput(test.args)

			actualErrorsMessages := utils.ErrorsToMessages(actualErrors)

			assert.DeepEqual(t, actualErrorsMessages, test.expectedErrors)

		})
	}
}

func TestCmdListenerCreate_InputToOptions(t *testing.T) {

	type test struct {
		name                   string
		flags                  common.CommandListenerCreateFlags
		Listenername           string
		expectedTlsCredentials string
		expectedHost           string
		expectedRoutingKey     string
		expectedListenerType   string
		expectedOutput         string
		expectedTimeout        time.Duration
	}

	testTable := []test{
		{
			name:                   "test1",
			flags:                  common.CommandListenerCreateFlags{"backend", "backend", "secret", "tcp", 20 * time.Second, "json"},
			expectedTlsCredentials: "secret",
			expectedHost:           "backend",
			expectedRoutingKey:     "backend",
			expectedTimeout:        20 * time.Second,
			expectedListenerType:   "tcp",
			expectedOutput:         "json",
		},
		{
			name:                   "test2",
			flags:                  common.CommandListenerCreateFlags{"", "", "secret", "tcp", 30 * time.Second, "yaml"},
			expectedTlsCredentials: "secret",
			expectedHost:           "test2",
			expectedRoutingKey:     "test2",
			expectedTimeout:        30 * time.Second,
			expectedListenerType:   "tcp",
			expectedOutput:         "yaml",
		},
	}

	for _, test := range testTable {
		t.Run(test.name, func(t *testing.T) {

			cmd, err := newCmdListenerCreateWithMocks("test", nil, nil, "")
			assert.Assert(t, err)

			cmd.Flags = &test.flags
			cmd.name = test.name

			cmd.InputToOptions()

			assert.Check(t, cmd.routingKey == test.expectedRoutingKey)
			assert.Check(t, cmd.output == test.expectedOutput)
			assert.Check(t, cmd.tlsCredentials == test.expectedTlsCredentials)
			assert.Check(t, cmd.host == test.expectedHost)
			assert.Check(t, cmd.timeout == test.expectedTimeout)
			assert.Check(t, cmd.output == test.expectedOutput)
			assert.Check(t, cmd.listenerType == test.expectedListenerType)
		})
	}
}

func TestCmdListenerCreate_Run(t *testing.T) {
	type test struct {
		name                string
		listenerName        string
		listenerPort        int
		flags               common.CommandListenerCreateFlags
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
			flags: common.CommandListenerCreateFlags{
				ListenerType:   "tcp",
				Host:           "hostname",
				RoutingKey:     "keyname",
				TlsCredentials: "secretname",
			},
		},
		{
			name:         "output yaml",
			listenerName: "run-listener",
			listenerPort: 8080,
			flags: common.CommandListenerCreateFlags{
				ListenerType:   "tcp",
				Host:           "hostname",
				RoutingKey:     "keyname",
				TlsCredentials: "secretname",
				Output:         "yaml",
			},
		},
	}

	for _, test := range testTable {
		cmd, err := newCmdListenerCreateWithMocks("test", test.k8sObjects, test.skupperObjects, test.skupperErrorMessage)
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

func TestCmdListenerCreate_WaitUntil(t *testing.T) {
	type test struct {
		name                string
		output              string
		k8sObjects          []runtime.Object
		skupperObjects      []runtime.Object
		skupperErrorMessage string
		expectError         bool
	}

	testTable := []test{
		{
			name: "listener is not ready",
			skupperObjects: []runtime.Object{
				&v2alpha1.Listener{
					ObjectMeta: v1.ObjectMeta{
						Name:      "my-listener",
						Namespace: "test",
					},
					Status: v2alpha1.ListenerStatus{
						Status: v2alpha1.Status{},
					},
				},
			},
			expectError: true,
		},
		{
			name:        "listener is not returned",
			expectError: true,
		},
		{
			name: "listener is ready",
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
			expectError: false,
		},
		{
			name:   "listener is ready yaml output",
			output: "yaml",
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
			expectError: false,
		},
	}

	for _, test := range testTable {
		cmd, err := newCmdListenerCreateWithMocks("test", test.k8sObjects, test.skupperObjects, test.skupperErrorMessage)
		assert.Assert(t, err)

		cmd.name = "my-listener"
		cmd.port = 8080
		cmd.output = test.output
		cmd.timeout = 1 * time.Second
		cmd.namespace = "test"

		t.Run(test.name, func(t *testing.T) {

			err := cmd.WaitUntil()
			if test.expectError {
				assert.Check(t, err != nil)
			} else {
				assert.Assert(t, err)
			}
		})
	}
}

// --- helper methods

func newCmdListenerCreateWithMocks(namespace string, k8sObjects []runtime.Object, skupperObjects []runtime.Object, fakeSkupperError string) (*CmdListenerCreate, error) {

	// We make sure the interval is appropriate
	utils.SetRetryProfile(utils.TestRetryProfile)
	client, err := fakeclient.NewFakeClient(namespace, k8sObjects, skupperObjects, fakeSkupperError)
	if err != nil {
		return nil, err
	}
	cmdListenerCreate := &CmdListenerCreate{
		client:     client.GetSkupperClient().SkupperV2alpha1(),
		KubeClient: client.GetKubeClient(),
		namespace:  namespace,
	}
	return cmdListenerCreate, nil
}

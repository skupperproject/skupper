package kube

import (
	"testing"
	"time"

	"github.com/skupperproject/skupper/internal/cmd/skupper/common"
	"github.com/skupperproject/skupper/internal/cmd/skupper/common/utils"

	fakeclient "github.com/skupperproject/skupper/internal/kube/client/fake"
	"github.com/skupperproject/skupper/pkg/apis/skupper/v1alpha1"
	"gotest.tools/assert"
	v12 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestCmdListenerUpdate_ValidateInput(t *testing.T) {
	type test struct {
		name           string
		args           []string
		flags          common.CommandListenerUpdateFlags
		k8sObjects     []runtime.Object
		skupperObjects []runtime.Object
		expectedErrors []string
	}

	testTable := []test{
		{
			name:           "listener is not updated because listener does not exist in the namespace",
			args:           []string{"my-listener"},
			flags:          common.CommandListenerUpdateFlags{Timeout: 1 * time.Minute},
			expectedErrors: []string{"listener my-listener must exist in namespace test to be updated"},
		},
		{
			name:           "listener name is not specified",
			args:           []string{},
			flags:          common.CommandListenerUpdateFlags{Timeout: 1 * time.Minute},
			expectedErrors: []string{"listener name must be configured"},
		},
		{
			name:           "listener name is nil",
			args:           []string{""},
			flags:          common.CommandListenerUpdateFlags{Timeout: 1 * time.Minute},
			expectedErrors: []string{"listener name must not be empty"},
		},
		{
			name:           "more than one argument is specified",
			args:           []string{"my", "listener"},
			flags:          common.CommandListenerUpdateFlags{Timeout: 1 * time.Minute},
			expectedErrors: []string{"only one argument is allowed for this command"},
		},
		{
			name:           "listener name is not valid.",
			args:           []string{"my new listener"},
			flags:          common.CommandListenerUpdateFlags{Timeout: 1 * time.Minute},
			expectedErrors: []string{"listener name is not valid: value does not match this regular expression: ^[a-z0-9]([-a-z0-9]*[a-z0-9])*(\\.[a-z0-9]([-a-z0-9]*[a-z0-9])*)*$"},
		},
		{
			name: "listener type is not valid",
			args: []string{"my-listener-type"},
			flags: common.CommandListenerUpdateFlags{
				ListenerType: "not-valid",
				Timeout:      60 * time.Second,
			},
			skupperObjects: []runtime.Object{
				&v1alpha1.Listener{
					ObjectMeta: v1.ObjectMeta{
						Name:      "my-listener-type",
						Namespace: "test",
					},
					Status: v1alpha1.ListenerStatus{
						Status: v1alpha1.Status{
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
			expectedErrors: []string{
				"listener type is not valid: value not-valid not allowed. It should be one of this options: [tcp]"},
		},
		{
			name: "routing key is not valid",
			args: []string{"my-listener-rk"},
			flags: common.CommandListenerUpdateFlags{
				RoutingKey: "not-valid$",
				Timeout:    30 * time.Second,
			},
			skupperObjects: []runtime.Object{
				&v1alpha1.Listener{
					ObjectMeta: v1.ObjectMeta{
						Name:      "my-listener-rk",
						Namespace: "test",
					}, Status: v1alpha1.ListenerStatus{
						Status: v1alpha1.Status{
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
			expectedErrors: []string{
				"routing key is not valid: value does not match this regular expression: ^[a-z0-9]([-a-z0-9]*[a-z0-9])*(\\.[a-z0-9]([-a-z0-9]*[a-z0-9])*)*$"},
		},
		{
			name: "tls-secret is not valid",
			args: []string{"my-listener-tls"},
			flags: common.CommandListenerUpdateFlags{
				TlsSecret: ":not-valid",
				Timeout:   50 * time.Minute,
			},
			skupperObjects: []runtime.Object{
				&v1alpha1.Listener{
					ObjectMeta: v1.ObjectMeta{
						Name:      "my-listener-tls",
						Namespace: "test",
					},
					Status: v1alpha1.ListenerStatus{
						Status: v1alpha1.Status{
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
			expectedErrors: []string{"tls-secret is not valid: does not exist"},
		},
		{
			name: "port is not valid",
			args: []string{"my-listener-port"},
			flags: common.CommandListenerUpdateFlags{
				Port:    -1,
				Timeout: 60 * time.Second,
			},
			skupperObjects: []runtime.Object{
				&v1alpha1.Listener{
					ObjectMeta: v1.ObjectMeta{
						Name:      "my-listener-port",
						Namespace: "test",
					},
					Status: v1alpha1.ListenerStatus{
						Status: v1alpha1.Status{
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
			expectedErrors: []string{"listener port is not valid: value is not positive"},
		},
		{
			name:  "timeout is not valid",
			args:  []string{"bad-timeout"},
			flags: common.CommandListenerUpdateFlags{Timeout: 5 * time.Second},
			skupperObjects: []runtime.Object{
				&v1alpha1.Listener{
					ObjectMeta: v1.ObjectMeta{
						Name:      "bad-timeout",
						Namespace: "test",
					},
					Status: v1alpha1.ListenerStatus{
						Status: v1alpha1.Status{
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
			expectedErrors: []string{"timeout is not valid: duration must not be less than 10s; got 5s"},
		},
		{
			name: "output is not valid",
			args: []string{"bad-output"},
			flags: common.CommandListenerUpdateFlags{
				Output:  "not-supported",
				Timeout: 10 * time.Second,
			},
			skupperObjects: []runtime.Object{
				&v1alpha1.Listener{
					ObjectMeta: v1.ObjectMeta{
						Name:      "bad-output",
						Namespace: "test",
					},
					Status: v1alpha1.ListenerStatus{
						Status: v1alpha1.Status{
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
			expectedErrors: []string{
				"output type is not valid: value not-supported not allowed. It should be one of this options: [json yaml]"},
		},
		{
			name: "flags all valid",
			args: []string{"my-listener-flags"},
			flags: common.CommandListenerUpdateFlags{
				Host:         "hostname",
				RoutingKey:   "routingkeyname",
				TlsSecret:    "secretname",
				Port:         1234,
				ListenerType: "tcp",
				Timeout:      10 * time.Second,
				Output:       "json",
			},
			skupperObjects: []runtime.Object{
				&v1alpha1.Listener{
					ObjectMeta: v1.ObjectMeta{
						Name:      "my-listener-flags",
						Namespace: "test",
					},
					Status: v1alpha1.ListenerStatus{
						Status: v1alpha1.Status{
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

			command, err := newCmdListenerUpdateWithMocks("test", test.k8sObjects, test.skupperObjects, "")
			assert.Assert(t, err)

			command.Flags = &test.flags

			actualErrors := command.ValidateInput(test.args)

			actualErrorsMessages := utils.ErrorsToMessages(actualErrors)

			assert.DeepEqual(t, actualErrorsMessages, test.expectedErrors)

		})
	}
}

func TestCmdListenerUpdate_Run(t *testing.T) {
	type test struct {
		name                string
		listenerName        string
		newOutput           string
		flags               common.CommandListenerUpdateFlags
		k8sObjects          []runtime.Object
		skupperObjects      []runtime.Object
		skupperErrorMessage string
		errorMessage        string
	}

	testTable := []test{
		{
			name:         "runs ok",
			listenerName: "run-listener",
			flags: common.CommandListenerUpdateFlags{
				ListenerType: "tcp",
				Host:         "hostname",
				RoutingKey:   "keyname",
				TlsSecret:    "secretname",
				Output:       "yaml",
				Timeout:      1 * time.Minute,
			},
			skupperObjects: []runtime.Object{
				&v1alpha1.Listener{
					ObjectMeta: v1.ObjectMeta{
						Name:      "run-listener",
						Namespace: "test",
					},
					Status: v1alpha1.ListenerStatus{
						Status: v1alpha1.Status{
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
		},
		{
			name:         "new output json",
			listenerName: "run-listener",
			flags: common.CommandListenerUpdateFlags{
				Timeout: 1 * time.Minute,
			},
			newOutput: "json",
			skupperObjects: []runtime.Object{
				&v1alpha1.Listener{
					ObjectMeta: v1.ObjectMeta{
						Name:      "run-listener",
						Namespace: "test",
					},
					Status: v1alpha1.ListenerStatus{
						Status: v1alpha1.Status{
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
		},
		{
			name:                "run fails",
			listenerName:        "run-listener",
			skupperErrorMessage: "error",
			errorMessage:        "error",
			flags:               common.CommandListenerUpdateFlags{Timeout: 1 * time.Minute},
		},
	}

	for _, test := range testTable {
		cmd, err := newCmdListenerUpdateWithMocks("test", test.k8sObjects, test.skupperObjects, test.skupperErrorMessage)
		assert.Assert(t, err)

		cmd.name = test.listenerName
		cmd.Flags = &test.flags
		cmd.namespace = "test"
		cmd.newSettings.output = test.newOutput

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

func TestCmdListenerUpdate_WaitUntil(t *testing.T) {
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
				&v1alpha1.Listener{
					ObjectMeta: v1.ObjectMeta{
						Name:      "my-listener",
						Namespace: "test",
					},
					Status: v1alpha1.ListenerStatus{
						Status: v1alpha1.Status{},
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
				&v1alpha1.Listener{
					ObjectMeta: v1.ObjectMeta{
						Name:      "my-listener",
						Namespace: "test",
					},
					Status: v1alpha1.ListenerStatus{
						Status: v1alpha1.Status{
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
			name:   "listener is ready json output",
			output: "json",
			skupperObjects: []runtime.Object{
				&v1alpha1.Listener{
					ObjectMeta: v1.ObjectMeta{
						Name:      "my-listener",
						Namespace: "test",
					},
					Status: v1alpha1.ListenerStatus{
						Status: v1alpha1.Status{
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
		cmd, err := newCmdListenerUpdateWithMocks("test", test.k8sObjects, test.skupperObjects, test.skupperErrorMessage)
		assert.Assert(t, err)

		cmd.name = "my-listener"
		cmd.Flags = &common.CommandListenerUpdateFlags{
			Timeout: 1 * time.Second,
			Output:  test.output,
		}
		cmd.namespace = "test"
		cmd.newSettings.output = cmd.Flags.Output

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

func newCmdListenerUpdateWithMocks(namespace string, k8sObjects []runtime.Object, skupperObjects []runtime.Object, fakeSkupperError string) (*CmdListenerUpdate, error) {

	// We make sure the interval is appropriate
	utils.SetRetryProfile(utils.TestRetryProfile)

	client, err := fakeclient.NewFakeClient(namespace, k8sObjects, skupperObjects, fakeSkupperError)
	if err != nil {
		return nil, err
	}
	cmdListenerUpdate := &CmdListenerUpdate{
		client:     client.GetSkupperClient().SkupperV1alpha1(),
		KubeClient: client.GetKubeClient(),
		namespace:  namespace,
	}
	return cmdListenerUpdate, nil
}

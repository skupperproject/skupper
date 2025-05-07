package kube

import (
	"testing"
	"time"

	"github.com/skupperproject/skupper/internal/cmd/skupper/common"
	"github.com/skupperproject/skupper/internal/cmd/skupper/common/testutils"
	"github.com/skupperproject/skupper/internal/cmd/skupper/common/utils"

	fakeclient "github.com/skupperproject/skupper/internal/kube/client/fake"
	"github.com/skupperproject/skupper/pkg/apis/skupper/v2alpha1"
	"gotest.tools/v3/assert"
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
		expectedError  string
		skupperError   string
	}

	testTable := []test{
		{
			name:          "missing CRD",
			args:          []string{"my-listener", "8080"},
			skupperError:  utils.CrdErr,
			expectedError: utils.CrdHelpErr,
		},
		{
			name:          "listener is not updated because listener does not exist in the namespace",
			args:          []string{"my-listener"},
			flags:         common.CommandListenerUpdateFlags{Timeout: 1 * time.Minute},
			expectedError: "listener my-listener must exist in namespace test to be updated",
		},
		{
			name:          "listener name is not specified",
			args:          []string{},
			flags:         common.CommandListenerUpdateFlags{Timeout: 1 * time.Minute},
			expectedError: "listener name must be configured",
		},
		{
			name:          "listener name is nil",
			args:          []string{""},
			flags:         common.CommandListenerUpdateFlags{Timeout: 1 * time.Minute},
			expectedError: "listener name must not be empty",
		},
		{
			name:          "more than one argument is specified",
			args:          []string{"my", "listener"},
			flags:         common.CommandListenerUpdateFlags{Timeout: 1 * time.Minute},
			expectedError: "only one argument is allowed for this command",
		},
		{
			name:          "listener name is not valid.",
			args:          []string{"my new listener"},
			flags:         common.CommandListenerUpdateFlags{Timeout: 1 * time.Minute},
			expectedError: "listener name is not valid: value does not match this regular expression: ^[a-z0-9]([-a-z0-9]*[a-z0-9])*(\\.[a-z0-9]([-a-z0-9]*[a-z0-9])*)*$",
		},
		{
			name: "listener type is not valid",
			args: []string{"my-listener-type"},
			flags: common.CommandListenerUpdateFlags{
				ListenerType: "not-valid",
				Timeout:      60 * time.Second,
			},
			skupperObjects: []runtime.Object{
				&v2alpha1.Listener{
					ObjectMeta: v1.ObjectMeta{
						Name:      "my-listener-type",
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
			expectedError: "listener type is not valid: value not-valid not allowed. It should be one of this options: [tcp]",
		},
		{
			name: "routing key is not valid",
			args: []string{"my-listener-rk"},
			flags: common.CommandListenerUpdateFlags{
				RoutingKey: "not-valid$",
				Timeout:    30 * time.Second,
			},
			skupperObjects: []runtime.Object{
				&v2alpha1.Listener{
					ObjectMeta: v1.ObjectMeta{
						Name:      "my-listener-rk",
						Namespace: "test",
					}, Status: v2alpha1.ListenerStatus{
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
			expectedError: "routing key is not valid: value does not match this regular expression: ^[a-z0-9]([-a-z0-9]*[a-z0-9])*(\\.[a-z0-9]([-a-z0-9]*[a-z0-9])*)*$",
		},
		{
			name: "tls-secret is not valid",
			args: []string{"my-listener-tls"},
			flags: common.CommandListenerUpdateFlags{
				TlsCredentials: ":not-valid",
				Timeout:        50 * time.Minute,
			},
			skupperObjects: []runtime.Object{
				&v2alpha1.Listener{
					ObjectMeta: v1.ObjectMeta{
						Name:      "my-listener-tls",
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
			expectedError: "tls-secret is not valid: does not exist",
		},
		{
			name: "port is not valid",
			args: []string{"my-listener-port"},
			flags: common.CommandListenerUpdateFlags{
				Port:    -1,
				Timeout: 60 * time.Second,
			},
			skupperObjects: []runtime.Object{
				&v2alpha1.Listener{
					ObjectMeta: v1.ObjectMeta{
						Name:      "my-listener-port",
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
			expectedError: "listener port is not valid: value is not positive",
		},
		{
			name:  "timeout is not valid",
			args:  []string{"bad-timeout"},
			flags: common.CommandListenerUpdateFlags{Timeout: 5 * time.Second},
			skupperObjects: []runtime.Object{
				&v2alpha1.Listener{
					ObjectMeta: v1.ObjectMeta{
						Name:      "bad-timeout",
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
			expectedError: "timeout is not valid: duration must not be less than 10s; got 5s",
		},
		{
			name: "flags all valid",
			args: []string{"my-listener-flags"},
			flags: common.CommandListenerUpdateFlags{
				Host:           "hostname",
				RoutingKey:     "routingkeyname",
				TlsCredentials: "secretname",
				Port:           1234,
				ListenerType:   "tcp",
				Timeout:        10 * time.Second,
			},
			skupperObjects: []runtime.Object{
				&v2alpha1.Listener{
					ObjectMeta: v1.ObjectMeta{
						Name:      "my-listener-flags",
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
			expectedError: "",
		},
		{
			name:       "wait status is not valid",
			args:       []string{"backend-listener"},
			flags:      common.CommandListenerUpdateFlags{Timeout: time.Second * 30, Wait: "created"},
			k8sObjects: nil,
			skupperObjects: []runtime.Object{
				&v2alpha1.Listener{
					ObjectMeta: v1.ObjectMeta{
						Name:      "backend-listener",
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
			expectedError: "status is not valid: value created not allowed. It should be one of this options: [ready configured none]",
		},
	}

	for _, test := range testTable {
		t.Run(test.name, func(t *testing.T) {

			command, err := newCmdListenerUpdateWithMocks("test", test.k8sObjects, test.skupperObjects, test.skupperError)
			assert.Assert(t, err)

			command.Flags = &test.flags

			testutils.CheckValidateInput(t, command, test.expectedError, test.args)
		})
	}
}

func TestCmdListenerUpdate_InputToOptions(t *testing.T) {

	type test struct {
		name           string
		args           []string
		flags          common.CommandListenerUpdateFlags
		expectedStatus string
	}

	testTable := []test{
		{
			name:           "options with waiting status",
			args:           []string{"backend-listener"},
			flags:          common.CommandListenerUpdateFlags{Wait: "configured"},
			expectedStatus: "configured",
		},
	}

	for _, test := range testTable {
		t.Run(test.name, func(t *testing.T) {
			command := &CmdListenerUpdate{}
			command.Flags = &test.flags

			command.InputToOptions()

			assert.Check(t, command.status == test.expectedStatus)
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
				ListenerType:   "tcp",
				Host:           "hostname",
				RoutingKey:     "keyname",
				TlsCredentials: "secretname",
				Timeout:        1 * time.Minute,
			},
			skupperObjects: []runtime.Object{
				&v2alpha1.Listener{
					ObjectMeta: v1.ObjectMeta{
						Name:      "run-listener",
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
		status              string
		k8sObjects          []runtime.Object
		skupperObjects      []runtime.Object
		skupperErrorMessage string
		expectError         bool
		errorMessage        string
	}

	testTable := []test{
		{
			name:   "listener is not ready",
			status: "ready",
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
			expectError:  true,
			errorMessage: "Listener \"my-listener\" is not yet ready, check the status for more information\n",
		},
		{
			name:         "listener is not returned",
			status:       "ready",
			expectError:  true,
			errorMessage: "Listener \"my-listener\" is not yet ready, check the status for more information\n",
		},
		{
			name:   "listener is ready",
			status: "ready",
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
									Type:   "Ready",
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
			name:       "listener is not ready yet, but user waits for configured",
			status:     "configured",
			k8sObjects: nil,
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
			name:       "user does not wait",
			status:     "none",
			k8sObjects: nil,
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
									Type:   "Ready",
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
			name:       "user waits for configured, but site had some errors while being configured",
			status:     "configured",
			k8sObjects: nil,
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
									Message:            "Error",
									ObservedGeneration: 1,
									Reason:             "Error",
									Status:             "False",
									Type:               "Configured",
								},
							},
						},
					},
				},
			},
			expectError:  true,
			errorMessage: "Listener \"my-listener\" is not yet configured: Error\n",
		},
	}

	for _, test := range testTable {
		cmd, err := newCmdListenerUpdateWithMocks("test", test.k8sObjects, test.skupperObjects, test.skupperErrorMessage)
		assert.Assert(t, err)

		cmd.name = "my-listener"
		cmd.Flags = &common.CommandListenerUpdateFlags{
			Timeout: 1 * time.Second,
		}
		cmd.namespace = "test"
		cmd.status = test.status

		t.Run(test.name, func(t *testing.T) {
			err := cmd.WaitUntil()
			if test.expectError {
				assert.Check(t, err != nil)
				assert.Equal(t, test.errorMessage, err.Error())
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
		client:     client.GetSkupperClient().SkupperV2alpha1(),
		KubeClient: client.GetKubeClient(),
		namespace:  namespace,
	}
	return cmdListenerUpdate, nil
}

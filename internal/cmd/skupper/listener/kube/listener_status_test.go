package kube

import (
	"testing"

	"github.com/skupperproject/skupper/internal/cmd/skupper/common"
	"github.com/skupperproject/skupper/internal/cmd/skupper/common/testutils"
	fakeclient "github.com/skupperproject/skupper/internal/kube/client/fake"
	"github.com/skupperproject/skupper/pkg/apis/skupper/v2alpha1"
	"gotest.tools/v3/assert"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestCmdListenerStatus_ValidateInput(t *testing.T) {
	type test struct {
		name           string
		args           []string
		flags          common.CommandListenerStatusFlags
		k8sObjects     []runtime.Object
		skupperObjects []runtime.Object
		expectedError  string
	}

	testTable := []test{
		{
			name:          "listener is not shown because listener does not exist in the namespace",
			args:          []string{"my-listener"},
			expectedError: "listener my-listener does not exist in namespace test",
		},
		{
			name:          "listener name is nil",
			args:          []string{""},
			expectedError: "listener name must not be empty",
		},
		{
			name:          "more than one argument is specified",
			args:          []string{"my", "listener"},
			expectedError: "only one argument is allowed for this command",
		},
		{
			name:          "listener name is not valid.",
			args:          []string{"my new listener"},
			expectedError: "listener name is not valid: value does not match this regular expression: ^[a-z0-9]([-a-z0-9]*[a-z0-9])*(\\.[a-z0-9]([-a-z0-9]*[a-z0-9])*)*$",
		},
		{
			name:          "no args",
			expectedError: "",
		},
		{
			name:  "bad output status",
			args:  []string{"out-listener"},
			flags: common.CommandListenerStatusFlags{Output: "not-supported"},
			skupperObjects: []runtime.Object{
				&v2alpha1.ListenerList{
					Items: []v2alpha1.Listener{
						{
							ObjectMeta: v1.ObjectMeta{
								Name:      "out-listener",
								Namespace: "test",
							},
							Spec: v2alpha1.ListenerSpec{
								Port: 8080,
								Type: "tcp",
								Host: "test",
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
			},
			expectedError: "output type is not valid: value not-supported not allowed. It should be one of this options: [json yaml]",
		},
		{
			name:          "good output status",
			flags:         common.CommandListenerStatusFlags{Output: "json"},
			expectedError: "",
		},
	}

	for _, test := range testTable {
		t.Run(test.name, func(t *testing.T) {

			command, err := newCmdListenerStatusWithMocks("test", test.k8sObjects, test.skupperObjects, "")
			assert.Assert(t, err)

			command.Flags = &test.flags

			testutils.CheckValidateInput(t, command, test.expectedError, test.args)
		})
	}
}

func TestCmdListenerStatus_Run(t *testing.T) {
	type test struct {
		name                string
		listenerName        string
		flags               common.CommandListenerStatusFlags
		k8sObjects          []runtime.Object
		skupperObjects      []runtime.Object
		skupperErrorMessage string
		errorMessage        string
	}

	testTable := []test{
		{
			name:                "1 listeners not found",
			listenerName:        "listener-fail",
			skupperErrorMessage: "not found",
			errorMessage:        "not found",
		},
		{
			name:                "no listeners found",
			skupperErrorMessage: "not found",
			errorMessage:        "not found",
		},
		{
			name:         "runs ok, returns 1 listener",
			listenerName: "listener1",
			skupperObjects: []runtime.Object{
				&v2alpha1.ListenerList{
					Items: []v2alpha1.Listener{
						{
							ObjectMeta: v1.ObjectMeta{
								Name:      "listener1",
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
			},
		},
		{
			name:         "returns 1 listener output yaml",
			listenerName: "listener-yaml",
			flags:        common.CommandListenerStatusFlags{Output: "yaml"},
			skupperObjects: []runtime.Object{
				&v2alpha1.ListenerList{
					Items: []v2alpha1.Listener{
						{
							ObjectMeta: v1.ObjectMeta{
								Name:      "listener-yaml",
								Namespace: "test",
							},
							Spec: v2alpha1.ListenerSpec{
								Port: 8080,
								Type: "tcp",
								Host: "test",
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
			},
		},
		{
			name: "returns all listeners",
			skupperObjects: []runtime.Object{
				&v2alpha1.ListenerList{
					Items: []v2alpha1.Listener{
						{
							ObjectMeta: v1.ObjectMeta{
								Name:      "my-listener1",
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
						{
							ObjectMeta: v1.ObjectMeta{
								Name:      "my-listener2",
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
			},
		},
		{
			name:  "returns all listeners output json",
			flags: common.CommandListenerStatusFlags{Output: "json"},
			skupperObjects: []runtime.Object{
				&v2alpha1.ListenerList{
					Items: []v2alpha1.Listener{
						{
							ObjectMeta: v1.ObjectMeta{
								Name:      "my-listener1",
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
						{
							ObjectMeta: v1.ObjectMeta{
								Name:      "my-listener2",
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
			},
		},
		{
			name:  "returns all listeners output bad",
			flags: common.CommandListenerStatusFlags{Output: "bad-value"},
			skupperObjects: []runtime.Object{
				&v2alpha1.ListenerList{
					Items: []v2alpha1.Listener{
						{
							ObjectMeta: v1.ObjectMeta{
								Name:      "my-listener1",
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
						{
							ObjectMeta: v1.ObjectMeta{
								Name:      "my-listener2",
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
			},
			errorMessage: "format bad-value not supported",
		},
		{
			name:         "returns 1 listeners output bad",
			listenerName: "my-listener",
			flags:        common.CommandListenerStatusFlags{Output: "bad-value"},
			skupperObjects: []runtime.Object{
				&v2alpha1.ListenerList{
					Items: []v2alpha1.Listener{
						{
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
				},
			},
			errorMessage: "format bad-value not supported",
		},
	}

	for _, test := range testTable {
		cmd, err := newCmdListenerStatusWithMocks("test", test.k8sObjects, test.skupperObjects, test.skupperErrorMessage)
		assert.Assert(t, err)
		cmd.name = test.listenerName
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

func newCmdListenerStatusWithMocks(namespace string, k8sObjects []runtime.Object, skupperObjects []runtime.Object, fakeSkupperError string) (*CmdListenerStatus, error) {

	client, err := fakeclient.NewFakeClient(namespace, k8sObjects, skupperObjects, fakeSkupperError)
	if err != nil {
		return nil, err
	}
	cmdListenerStatus := &CmdListenerStatus{
		client:    client.GetSkupperClient().SkupperV2alpha1(),
		namespace: namespace,
	}
	return cmdListenerStatus, nil
}

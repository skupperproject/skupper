package kube

import (
	"testing"

	"github.com/skupperproject/skupper/internal/cmd/skupper/utils"
	fakeclient "github.com/skupperproject/skupper/internal/kube/client/fake"
	"github.com/skupperproject/skupper/pkg/apis/skupper/v1alpha1"
	"gotest.tools/assert"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestCmdListenerStatus_NewCmdListenerStatus(t *testing.T) {

	t.Run("Status command", func(t *testing.T) {

		result := NewCmdListenerStatus()

		assert.Check(t, result.CobraCmd.Use != "")
		assert.Check(t, result.CobraCmd.Short != "")
		assert.Check(t, result.CobraCmd.Long != "")
		assert.Check(t, result.CobraCmd.Example != "")
		assert.Check(t, result.CobraCmd.PreRun != nil)
		assert.Check(t, result.CobraCmd.Run != nil)
	})

}

func TestCmdListenerStatus_ValidateInput(t *testing.T) {
	type test struct {
		name           string
		args           []string
		flags          ListenerStatus
		k8sObjects     []runtime.Object
		skupperObjects []runtime.Object
		expectedErrors []string
	}

	testTable := []test{
		{
			name:           "listener is not shown because listener does not exist in the namespace",
			args:           []string{"my-listener"},
			expectedErrors: []string{"listener my-listener does not exist in namespace test"},
		},
		{
			name:           "listener name is nil",
			args:           []string{""},
			expectedErrors: []string{"listener name must not be empty"},
		},
		{
			name:           "more than one argument is specified",
			args:           []string{"my", "listener"},
			expectedErrors: []string{"only one argument is allowed for this command"},
		},
		{
			name:           "listener name is not valid.",
			args:           []string{"my new listener"},
			expectedErrors: []string{"listener name is not valid: value does not match this regular expression: ^[a-z0-9]([-a-z0-9]*[a-z0-9])*(\\.[a-z0-9]([-a-z0-9]*[a-z0-9])*)*$"},
		},
		{
			name:           "no args",
			expectedErrors: []string{},
		},
		{
			name:  "bad output status",
			args:  []string{"out-listener"},
			flags: ListenerStatus{output: "not-supported"},
			skupperObjects: []runtime.Object{
				&v1alpha1.ListenerList{
					Items: []v1alpha1.Listener{
						{
							ObjectMeta: v1.ObjectMeta{
								Name:      "out-listener",
								Namespace: "test",
							},
							Spec: v1alpha1.ListenerSpec{
								Port: 8080,
								Type: "tcp",
								Host: "test",
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
			},
			expectedErrors: []string{"output type is not valid: value not-supported not allowed. It should be one of this options: [json yaml]"},
		},
		{
			name:           "good output status",
			flags:          ListenerStatus{output: "json"},
			expectedErrors: []string{},
		},
	}

	for _, test := range testTable {
		t.Run(test.name, func(t *testing.T) {

			command, err := newCmdListenerStatusWithMocks("test", test.k8sObjects, test.skupperObjects, "")
			assert.Assert(t, err)

			command.flags = test.flags

			actualErrors := command.ValidateInput(test.args)

			actualErrorsMessages := utils.ErrorsToMessages(actualErrors)

			assert.DeepEqual(t, actualErrorsMessages, test.expectedErrors)
		})
	}
}

func TestCmdListenerStatus_Run(t *testing.T) {
	type test struct {
		name                string
		listenerName        string
		flags               ListenerStatus
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
				&v1alpha1.ListenerList{
					Items: []v1alpha1.Listener{
						{
							ObjectMeta: v1.ObjectMeta{
								Name:      "listener1",
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
			},
		},
		{
			name:         "returns 1 listener output yaml",
			listenerName: "listener-yaml",
			flags:        ListenerStatus{output: "yaml"},
			skupperObjects: []runtime.Object{
				&v1alpha1.ListenerList{
					Items: []v1alpha1.Listener{
						{
							ObjectMeta: v1.ObjectMeta{
								Name:      "listener-yaml",
								Namespace: "test",
							},
							Spec: v1alpha1.ListenerSpec{
								Port: 8080,
								Type: "tcp",
								Host: "test",
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
			},
		},
		{
			name: "returns all listeners",
			skupperObjects: []runtime.Object{
				&v1alpha1.ListenerList{
					Items: []v1alpha1.Listener{
						{
							ObjectMeta: v1.ObjectMeta{
								Name:      "my-listener1",
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
						{
							ObjectMeta: v1.ObjectMeta{
								Name:      "my-listener2",
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
			},
		},
		{
			name:  "returns all listeners output json",
			flags: ListenerStatus{output: "json"},
			skupperObjects: []runtime.Object{
				&v1alpha1.ListenerList{
					Items: []v1alpha1.Listener{
						{
							ObjectMeta: v1.ObjectMeta{
								Name:      "my-listener1",
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
						{
							ObjectMeta: v1.ObjectMeta{
								Name:      "my-listener2",
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
			},
		},
		{
			name:  "returns all listeners output bad",
			flags: ListenerStatus{output: "bad-value"},
			skupperObjects: []runtime.Object{
				&v1alpha1.ListenerList{
					Items: []v1alpha1.Listener{
						{
							ObjectMeta: v1.ObjectMeta{
								Name:      "my-listener1",
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
						{
							ObjectMeta: v1.ObjectMeta{
								Name:      "my-listener2",
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
			},
			errorMessage: "format bad-value not supported",
		},
		{
			name:         "returns 1 listeners output bad",
			listenerName: "my-listener",
			flags:        ListenerStatus{output: "bad-value"},
			skupperObjects: []runtime.Object{
				&v1alpha1.ListenerList{
					Items: []v1alpha1.Listener{
						{
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
				},
			},
			errorMessage: "format bad-value not supported",
		},
	}

	for _, test := range testTable {
		cmd, err := newCmdListenerStatusWithMocks("test", test.k8sObjects, test.skupperObjects, test.skupperErrorMessage)
		assert.Assert(t, err)
		cmd.name = test.listenerName
		cmd.flags = test.flags
		cmd.output = cmd.flags.output
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
		client:    client.GetSkupperClient().SkupperV1alpha1(),
		namespace: namespace,
	}
	return cmdListenerStatus, nil
}

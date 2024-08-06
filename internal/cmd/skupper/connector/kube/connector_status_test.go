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

func TestCmdConnectorStatus_NewCmdConnectorStatus(t *testing.T) {

	t.Run("Status command", func(t *testing.T) {

		result := NewCmdConnectorStatus()

		assert.Check(t, result.CobraCmd.Use != "")
		assert.Check(t, result.CobraCmd.Short != "")
		assert.Check(t, result.CobraCmd.Long != "")
		assert.Check(t, result.CobraCmd.Example != "")
		assert.Check(t, result.CobraCmd.PreRun != nil)
		assert.Check(t, result.CobraCmd.Run != nil)
	})

}

func TestCmdConnectorStatus_ValidateInput(t *testing.T) {
	type test struct {
		name           string
		args           []string
		flags          ConnectorStatus
		k8sObjects     []runtime.Object
		skupperObjects []runtime.Object
		expectedErrors []string
	}

	testTable := []test{
		{
			name:           "connector is not shown because connector does not exist in the namespace",
			args:           []string{"my-connector"},
			expectedErrors: []string{"connector my-connector does not exist in namespace test"},
		},
		{
			name:           "connector name is nil",
			args:           []string{""},
			expectedErrors: []string{"connector name must not be empty"},
		},
		{
			name:           "more than one argument is specified",
			args:           []string{"my", "connector"},
			expectedErrors: []string{"only one argument is allowed for this command"},
		},
		{
			name:           "connector name is not valid.",
			args:           []string{"my new connector"},
			expectedErrors: []string{"connector name is not valid: value does not match this regular expression: ^[a-z0-9]([-a-z0-9]*[a-z0-9])*(\\.[a-z0-9]([-a-z0-9]*[a-z0-9])*)*$"},
		},
		{
			name:           "no args",
			expectedErrors: []string{},
		},
		{
			name:  "bad output status",
			args:  []string{"out-connector"},
			flags: ConnectorStatus{output: "not-supported"},
			skupperObjects: []runtime.Object{
				&v1alpha1.ConnectorList{
					Items: []v1alpha1.Connector{
						{
							ObjectMeta: v1.ObjectMeta{
								Name:      "out-connector",
								Namespace: "test",
							},
							Status: v1alpha1.ConnectorStatus{
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
			name:  "good output status",
			args:  []string{"out-connector"},
			flags: ConnectorStatus{output: "json"},
			skupperObjects: []runtime.Object{
				&v1alpha1.ConnectorList{
					Items: []v1alpha1.Connector{
						{
							ObjectMeta: v1.ObjectMeta{
								Name:      "out-connector",
								Namespace: "test",
							},
							Status: v1alpha1.ConnectorStatus{
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
			expectedErrors: []string{},
		},
	}

	for _, test := range testTable {
		t.Run(test.name, func(t *testing.T) {

			command, err := newCmdConnectorStatusWithMocks("test", test.k8sObjects, test.skupperObjects, "")
			assert.Assert(t, err)

			command.flags = test.flags

			actualErrors := command.ValidateInput(test.args)

			actualErrorsMessages := utils.ErrorsToMessages(actualErrors)

			assert.DeepEqual(t, actualErrorsMessages, test.expectedErrors)

		})
	}
}

func TestCmdConnectorStatus_Run(t *testing.T) {
	type test struct {
		name                string
		connectorName       string
		flags               ConnectorStatus
		k8sObjects          []runtime.Object
		skupperObjects      []runtime.Object
		skupperErrorMessage string
		errorMessage        string
	}

	testTable := []test{
		{
			name:         "run fails no connectors found",
			errorMessage: "NotFound",
		},
		{
			name:          "run fails connector doesn't exist",
			connectorName: "my-connector",
			errorMessage:  "connectors.skupper.io \"my-connector\" not found",
		},
		{
			name:          "runs ok, returns 1 connectors",
			connectorName: "my-connector",
			skupperObjects: []runtime.Object{
				&v1alpha1.ConnectorList{
					Items: []v1alpha1.Connector{
						{
							ObjectMeta: v1.ObjectMeta{
								Name:      "my-connector",
								Namespace: "test",
							},
							Status: v1alpha1.ConnectorStatus{
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
			name:          "runs ok, returns 1 connectors yaml",
			connectorName: "my-connector",
			flags:         ConnectorStatus{output: "yaml"},
			skupperObjects: []runtime.Object{
				&v1alpha1.ConnectorList{
					Items: []v1alpha1.Connector{
						{
							ObjectMeta: v1.ObjectMeta{
								Name:      "my-connector",
								Namespace: "test",
							},
							Status: v1alpha1.ConnectorStatus{
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
			name: "runs ok, returns all connectors",
			skupperObjects: []runtime.Object{
				&v1alpha1.ConnectorList{
					Items: []v1alpha1.Connector{
						{
							ObjectMeta: v1.ObjectMeta{
								Name:      "my-connector1",
								Namespace: "test",
							},
							Status: v1alpha1.ConnectorStatus{
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
								Name:      "my-connector2",
								Namespace: "test",
							},
							Status: v1alpha1.ConnectorStatus{
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
			name:  "runs ok, returns all connectors json",
			flags: ConnectorStatus{output: "json"},
			skupperObjects: []runtime.Object{
				&v1alpha1.ConnectorList{
					Items: []v1alpha1.Connector{
						{
							ObjectMeta: v1.ObjectMeta{
								Name:      "my-connector1",
								Namespace: "test",
							},
							Status: v1alpha1.ConnectorStatus{
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
								Name:      "my-connector2",
								Namespace: "test",
							},
							Status: v1alpha1.ConnectorStatus{
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
			name:  "runs ok, returns all connectors output bad",
			flags: ConnectorStatus{output: "bad-value"},
			skupperObjects: []runtime.Object{
				&v1alpha1.ConnectorList{
					Items: []v1alpha1.Connector{
						{
							ObjectMeta: v1.ObjectMeta{
								Name:      "my-connector1",
								Namespace: "test",
							},
							Status: v1alpha1.ConnectorStatus{
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
								Name:      "my-connector2",
								Namespace: "test",
							},
							Status: v1alpha1.ConnectorStatus{
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
			name:          "runs ok, returns 1 connectors bad output",
			connectorName: "my-connector",
			flags:         ConnectorStatus{output: "bad-value"},
			skupperObjects: []runtime.Object{
				&v1alpha1.ConnectorList{
					Items: []v1alpha1.Connector{
						{
							ObjectMeta: v1.ObjectMeta{
								Name:      "my-connector",
								Namespace: "test",
							},
							Status: v1alpha1.ConnectorStatus{
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
		cmd, err := newCmdConnectorStatusWithMocks("test", test.k8sObjects, test.skupperObjects, test.skupperErrorMessage)
		assert.Assert(t, err)

		cmd.name = test.connectorName
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

func newCmdConnectorStatusWithMocks(namespace string, k8sObjects []runtime.Object, skupperObjects []runtime.Object, fakeSkupperError string) (*CmdConnectorStatus, error) {

	client, err := fakeclient.NewFakeClient(namespace, k8sObjects, skupperObjects, fakeSkupperError)
	if err != nil {
		return nil, err
	}
	cmdConnectorStatus := &CmdConnectorStatus{
		client:    client.GetSkupperClient().SkupperV1alpha1(),
		namespace: namespace,
	}
	return cmdConnectorStatus, nil
}

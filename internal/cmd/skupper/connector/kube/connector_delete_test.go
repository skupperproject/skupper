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
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestCmdConnectorDelete_ValidateInput(t *testing.T) {
	type test struct {
		name           string
		args           []string
		flags          common.CommandConnectorDeleteFlags
		k8sObjects     []runtime.Object
		skupperObjects []runtime.Object
		expectedError  string
		skupperError   string
	}

	testTable := []test{
		{
			name:          "missing CRD",
			args:          []string{"my-connector"},
			flags:         common.CommandConnectorDeleteFlags{},
			skupperError:  utils.CrdErr,
			expectedError: utils.CrdHelpErr,
		},
		{
			name:          "connector is not Deleted because connector does not exist in the namespace",
			args:          []string{"my-connector"},
			flags:         common.CommandConnectorDeleteFlags{Timeout: 30 * time.Second},
			expectedError: "connector my-connector does not exist in namespace test",
		},
		{
			name:          "connector name is not specified",
			args:          []string{},
			flags:         common.CommandConnectorDeleteFlags{Timeout: 10 * time.Second},
			expectedError: "connector name must be specified",
		},
		{
			name:          "connector name is nil",
			args:          []string{""},
			flags:         common.CommandConnectorDeleteFlags{Timeout: 10 * time.Second},
			expectedError: "connector name must not be empty",
		},
		{
			name:          "connector name is not valid",
			args:          []string{"my name"},
			flags:         common.CommandConnectorDeleteFlags{Timeout: 10 * time.Second},
			expectedError: "connector name is not valid: value does not match this regular expression: ^[a-z0-9]([-a-z0-9]*[a-z0-9])*(\\.[a-z0-9]([-a-z0-9]*[a-z0-9])*)*$",
		},
		{
			name:          "more than one argument is specified",
			args:          []string{"my", "connector"},
			flags:         common.CommandConnectorDeleteFlags{Timeout: 10 * time.Second},
			expectedError: "only one argument is allowed for this command",
		},
		{
			name:  "timeout is not valid",
			args:  []string{"bad-timeout"},
			flags: common.CommandConnectorDeleteFlags{Timeout: 5 * time.Second},
			skupperObjects: []runtime.Object{
				&v2alpha1.Connector{
					ObjectMeta: v1.ObjectMeta{
						Name:      "bad-timeout",
						Namespace: "test",
					},
					Status: v2alpha1.ConnectorStatus{
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
	}

	for _, test := range testTable {
		t.Run(test.name, func(t *testing.T) {

			command, err := newCmdConnectorDeleteWithMocks("test", test.k8sObjects, test.skupperObjects, test.skupperError)
			assert.Assert(t, err)

			command.Flags = &test.flags

			testutils.CheckValidateInput(t, command, test.expectedError, test.args)
		})
	}
}

func TestCmdConnectorDelete_Run(t *testing.T) {
	type test struct {
		name                string
		deleteName          string
		k8sObjects          []runtime.Object
		skupperObjects      []runtime.Object
		skupperErrorMessage string
		errorMessage        string
	}

	testTable := []test{
		{
			name:                "run fails",
			deleteName:          "my-connector",
			skupperErrorMessage: "error",
			errorMessage:        "error",
		},
	}

	for _, test := range testTable {
		cmd, err := newCmdConnectorDeleteWithMocks("test", test.k8sObjects, test.skupperObjects, test.skupperErrorMessage)
		assert.Assert(t, err)

		t.Run(test.name, func(t *testing.T) {

			cmd.name = test.deleteName
			cmd.namespace = "test"

			err := cmd.Run()
			if err != nil {
				assert.Check(t, test.errorMessage == err.Error())
			} else {
				assert.Check(t, err == nil)
			}
		})
	}
}

func TestCmdConnectorDelete_WaitUntil(t *testing.T) {
	type test struct {
		name                string
		wait                bool
		k8sObjects          []runtime.Object
		skupperObjects      []runtime.Object
		skupperErrorMessage string
		expectError         bool
	}

	testTable := []test{
		{
			name: "error deleting connector",
			wait: true,
			skupperObjects: []runtime.Object{
				&v2alpha1.Connector{
					ObjectMeta: v1.ObjectMeta{
						Name:      "my-connector",
						Namespace: "test",
					},
					Status: v2alpha1.ConnectorStatus{
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
			expectError: true,
		},
		{
			name:        "connector is deleted",
			wait:        true,
			expectError: false,
		},
		{
			name: "connector is not deleted but user does not want to wait",
			wait: false,
			skupperObjects: []runtime.Object{
				&v2alpha1.Connector{
					ObjectMeta: v1.ObjectMeta{
						Name:      "my-connector",
						Namespace: "test",
					},
					Status: v2alpha1.ConnectorStatus{
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
		cmd, err := newCmdConnectorDeleteWithMocks("test", test.k8sObjects, test.skupperObjects, test.skupperErrorMessage)
		assert.Assert(t, err)

		cmd.name = "my-connector"
		cmd.Flags = &common.CommandConnectorDeleteFlags{Timeout: 1 * time.Second}
		cmd.namespace = "test"
		cmd.wait = test.wait

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

func TestCmdConnectorDelete_InputToOptions(t *testing.T) {

	type test struct {
		name         string
		flags        *common.CommandConnectorDeleteFlags
		expectedWait bool
	}

	testTable := []test{
		{
			name:         "wait for connector to be deleted",
			flags:        &common.CommandConnectorDeleteFlags{Wait: true},
			expectedWait: true,
		},
	}

	for _, test := range testTable {
		t.Run(test.name, func(t *testing.T) {
			command := &CmdConnectorDelete{}
			command.Flags = test.flags
			command.InputToOptions()

			assert.Check(t, command.wait == test.expectedWait)
		})
	}
}

// --- helper methods

func newCmdConnectorDeleteWithMocks(namespace string, k8sObjects []runtime.Object, skupperObjects []runtime.Object, fakeSkupperError string) (*CmdConnectorDelete, error) {

	// We make sure the interval is appropriate
	utils.SetRetryProfile(utils.TestRetryProfile)
	client, err := fakeclient.NewFakeClient(namespace, k8sObjects, skupperObjects, fakeSkupperError)
	if err != nil {
		return nil, err
	}
	cmdConnectorCreate := &CmdConnectorDelete{
		client:    client.GetSkupperClient().SkupperV2alpha1(),
		namespace: namespace,
	}
	return cmdConnectorCreate, nil
}

package kube

import (
	"fmt"
	"testing"
	"time"

	"github.com/skupperproject/skupper/internal/cmd/skupper/utils"
	"github.com/skupperproject/skupper/pkg/apis/skupper/v1alpha1"
	"github.com/skupperproject/skupper/pkg/generated/client/clientset/versioned/typed/skupper/v1alpha1/fake"
	"github.com/spf13/pflag"
	"gotest.tools/assert"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	testing2 "k8s.io/client-go/testing"
)

func TestCmdConnectorDelete_NewCmdConnectorDelete(t *testing.T) {

	t.Run("Delete command", func(t *testing.T) {

		result := NewCmdConnectorDelete()

		assert.Check(t, result.CobraCmd.Use != "")
		assert.Check(t, result.CobraCmd.Short != "")
		assert.Check(t, result.CobraCmd.Long != "")
		assert.Check(t, result.CobraCmd.Example != "")
		assert.Check(t, result.CobraCmd.PreRun != nil)
		assert.Check(t, result.CobraCmd.Run != nil)
		assert.Check(t, result.CobraCmd.PostRunE != nil)
		assert.Check(t, result.CobraCmd.Flags() != nil)
	})

}

func TestCmdConnectorDelete_AddFlags(t *testing.T) {

	expectedFlagsWithDefaultValue := map[string]interface{}{
		"timeout": "1m0s",
	}
	var flagList []string

	cmd := newCmdConnectorDeleteWithMocks()

	t.Run("add flags", func(t *testing.T) {

		cmd.CobraCmd.Flags().VisitAll(func(flag *pflag.Flag) {
			flagList = append(flagList, flag.Name)
		})

		assert.Check(t, len(flagList) == 0)

		cmd.AddFlags()

		cmd.CobraCmd.Flags().VisitAll(func(flag *pflag.Flag) {
			flagList = append(flagList, flag.Name)
			assert.Check(t, expectedFlagsWithDefaultValue[flag.Name] != nil)
			assert.Check(t, expectedFlagsWithDefaultValue[flag.Name] == flag.DefValue)
		})

		assert.Check(t, len(flagList) == len(expectedFlagsWithDefaultValue))
	})

}
func TestCmdConnectorDelete_ValidateInput(t *testing.T) {
	type test struct {
		name           string
		args           []string
		setUpMock      func(command *CmdConnectorDelete)
		expectedErrors []string
	}

	command := &CmdConnectorDelete{
		namespace: "test",
	}

	testTable := []test{
		{
			name: "connector is not Deleted because get connector returned error",
			args: []string{"my-connector"},
			setUpMock: func(command *CmdConnectorDelete) {
				fakeSkupperClient := &fake.FakeSkupperV1alpha1{Fake: &testing2.Fake{}}
				fakeSkupperClient.Fake.ClearActions()
				fakeSkupperClient.Fake.PrependReactor("get", "connectors", func(action testing2.Action) (handled bool, ret runtime.Object, err error) {
					return true, nil, fmt.Errorf("NotFound")
				})
				command.client = fakeSkupperClient
				command.flags = ConnectorDelete{timeout: 1 * time.Minute}
			},
			expectedErrors: []string{"NotFound"},
		},
		{
			name: "connector is not Deleted because connector does not exist in the namespace",
			args: []string{"my-connector"},
			setUpMock: func(command *CmdConnectorDelete) {
				fakeSkupperClient := &fake.FakeSkupperV1alpha1{Fake: &testing2.Fake{}}
				fakeSkupperClient.Fake.ClearActions()
				fakeSkupperClient.Fake.PrependReactor("get", "connectors", func(action testing2.Action) (handled bool, ret runtime.Object, err error) {
					return true, nil, nil
				})
				command.client = fakeSkupperClient
				command.flags = ConnectorDelete{timeout: 30 * time.Second}
			},
			expectedErrors: []string{"connector my-connector does not exist in namespace test"},
		},
		{
			name: "connector name is not specified",
			args: []string{},
			setUpMock: func(command *CmdConnectorDelete) {
				fakeSkupperClient := &fake.FakeSkupperV1alpha1{Fake: &testing2.Fake{}}
				fakeSkupperClient.Fake.ClearActions()
				command.client = fakeSkupperClient
				command.flags = ConnectorDelete{timeout: 1 * time.Second}
			},
			expectedErrors: []string{"connector name must be specified"},
		},
		{
			name: "connector name is nil",
			args: []string{""},
			setUpMock: func(command *CmdConnectorDelete) {
				fakeSkupperClient := &fake.FakeSkupperV1alpha1{Fake: &testing2.Fake{}}
				fakeSkupperClient.Fake.ClearActions()
				command.client = fakeSkupperClient
				command.flags = ConnectorDelete{timeout: 1 * time.Second}
			},
			expectedErrors: []string{"connector name must not be empty"},
		},
		{
			name: "connector name is not valid",
			args: []string{"my name"},
			setUpMock: func(command *CmdConnectorDelete) {
				fakeSkupperClient := &fake.FakeSkupperV1alpha1{Fake: &testing2.Fake{}}
				fakeSkupperClient.Fake.ClearActions()
				command.client = fakeSkupperClient
				command.flags = ConnectorDelete{timeout: 1 * time.Second}
			},
			expectedErrors: []string{"connector name is not valid: value does not match this regular expression: ^[a-z0-9]([-a-z0-9]*[a-z0-9])*(\\.[a-z0-9]([-a-z0-9]*[a-z0-9])*)*$"},
		},
		{
			name: "more than one argument is specified",
			args: []string{"my", "connector"},
			setUpMock: func(command *CmdConnectorDelete) {
				fakeSkupperClient := &fake.FakeSkupperV1alpha1{Fake: &testing2.Fake{}}
				fakeSkupperClient.Fake.ClearActions()
				command.client = fakeSkupperClient
				command.flags = ConnectorDelete{timeout: 1 * time.Second}
			},
			expectedErrors: []string{"only one argument is allowed for this command"},
		},
		{
			name: "timeout is not valid",
			args: []string{"bad-timeout"},
			setUpMock: func(command *CmdConnectorDelete) {
				fakeSkupperClient := &fake.FakeSkupperV1alpha1{Fake: &testing2.Fake{}}
				fakeSkupperClient.Fake.ClearActions()
				command.client = fakeSkupperClient
				command.flags = ConnectorDelete{timeout: 0 * time.Second}
			},
			expectedErrors: []string{
				"timeout is not valid"},
		},
	}

	for _, test := range testTable {
		t.Run(test.name, func(t *testing.T) {

			if test.setUpMock != nil {
				test.setUpMock(command)
			}

			actualErrors := command.ValidateInput(test.args)

			actualErrorsMessages := utils.ErrorsToMessages(actualErrors)

			assert.DeepEqual(t, actualErrorsMessages, test.expectedErrors)

		})
	}
}

func TestCmdConnectorDelete_Run(t *testing.T) {
	type test struct {
		name         string
		setUpMock    func(command *CmdConnectorDelete)
		errorMessage string
	}

	testTable := []test{
		{
			name: "runs ok",
			setUpMock: func(command *CmdConnectorDelete) {
				fakeSkupperClient := &fake.FakeSkupperV1alpha1{Fake: &testing2.Fake{}}
				fakeSkupperClient.Fake.ClearActions()
				fakeSkupperClient.Fake.PrependReactor("Delete", "connectors", func(action testing2.Action) (handled bool, ret runtime.Object, err error) {
					return true, nil, nil
				})
				command.client = fakeSkupperClient
				command.name = "my-connector"
			},
		},
		{
			name: "run fails",
			setUpMock: func(command *CmdConnectorDelete) {
				fakeSkupperClient := &fake.FakeSkupperV1alpha1{Fake: &testing2.Fake{}}
				fakeSkupperClient.Fake.ClearActions()
				fakeSkupperClient.Fake.PrependReactor("Delete", "connectors", func(action testing2.Action) (handled bool, ret runtime.Object, err error) {
					return true, nil, fmt.Errorf("error")
				})
				command.client = fakeSkupperClient
				command.name = "my-fail-connector"
			},
			errorMessage: "error",
		},
	}

	for _, test := range testTable {
		cmd := newCmdConnectorDeleteWithMocks()
		test.setUpMock(cmd)

		//create a connector

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

func TestCmdConnectorDelete_WaitUntilReady(t *testing.T) {
	type test struct {
		name        string
		setUpMock   func(command *CmdConnectorDelete)
		expectError bool
	}

	testTable := []test{
		{
			name: "error deleting connector",
			setUpMock: func(command *CmdConnectorDelete) {
				fakeSkupperClient := &fake.FakeSkupperV1alpha1{Fake: &testing2.Fake{}}
				fakeSkupperClient.Fake.ClearActions()
				fakeSkupperClient.Fake.PrependReactor("get", "connectors", func(action testing2.Action) (handled bool, ret runtime.Object, err error) {
					return true, &v1alpha1.Connector{
						ObjectMeta: v1.ObjectMeta{
							Name:      "my-connector",
							Namespace: "test",
						},
						Status: v1alpha1.Status{
							StatusMessage: "",
						},
					}, nil
				})
				command.client = fakeSkupperClient
			},
			expectError: true,
		},
		{
			name: "connector is not returned",
			setUpMock: func(command *CmdConnectorDelete) {
				fakeSkupperClient := &fake.FakeSkupperV1alpha1{Fake: &testing2.Fake{}}
				fakeSkupperClient.Fake.ClearActions()
				fakeSkupperClient.Fake.PrependReactor("get", "connectors", func(action testing2.Action) (handled bool, ret runtime.Object, err error) {
					return true, nil, fmt.Errorf("it failed")
				})
				command.client = fakeSkupperClient
			},
			expectError: true,
		},
		{
			name: "connector is deleted",
			setUpMock: func(command *CmdConnectorDelete) {
				fakeSkupperClient := &fake.FakeSkupperV1alpha1{Fake: &testing2.Fake{}}
				fakeSkupperClient.Fake.ClearActions()
				fakeSkupperClient.Fake.PrependReactor("get", "connectors", func(action testing2.Action) (handled bool, ret runtime.Object, err error) {
					return true, nil, nil
				})
				command.client = fakeSkupperClient
			},
			expectError: false,
		},
	}

	for _, test := range testTable {
		cmd := newCmdConnectorDeleteWithMocks()
		cmd.name = "my-connector"
		cmd.flags = ConnectorDelete{timeout: 5 * time.Second}

		test.setUpMock(cmd)
		t.Run(test.name, func(t *testing.T) {

			err := cmd.WaitUntilReady()
			if err != nil {
				assert.Check(t, test.expectError)
			}

		})
	}
}

// --- helper methods

func newCmdConnectorDeleteWithMocks() *CmdConnectorDelete {

	cmdConnectorDelete := &CmdConnectorDelete{
		client:    &fake.FakeSkupperV1alpha1{Fake: &testing2.Fake{}},
		namespace: "test",
	}

	return cmdConnectorDelete
}

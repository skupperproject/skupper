package kube

import (
	"fmt"
	"testing"

	"github.com/skupperproject/skupper/pkg/apis/skupper/v1alpha1"
	"github.com/skupperproject/skupper/pkg/generated/client/clientset/versioned/typed/skupper/v1alpha1/fake"
	"gotest.tools/assert"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	testing2 "k8s.io/client-go/testing"
)

func TestCmdListenerDelete_NewCmdListenerDelete(t *testing.T) {

	t.Run("Delete command", func(t *testing.T) {

		result := NewCmdListenerDelete()

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

func TestCmdListenerDelete_ValidateInput(t *testing.T) {
	type test struct {
		name           string
		args           []string
		setUpMock      func(command *CmdListenerDelete)
		expectedErrors []string
	}

	command := &CmdListenerDelete{
		namespace: "test",
	}

	testTable := []test{
		{
			name: "listener is not deleted because listener does not exist in the namespace",
			args: []string{"my-listener"},
			setUpMock: func(command *CmdListenerDelete) {
				fakeSkupperClient := &fake.FakeSkupperV1alpha1{Fake: &testing2.Fake{}}
				fakeSkupperClient.Fake.ClearActions()
				fakeSkupperClient.Fake.PrependReactor("get", "listeners", func(action testing2.Action) (handled bool, ret runtime.Object, err error) {
					return true, nil, fmt.Errorf("NotFound")
				})
				command.client = fakeSkupperClient
			},
			expectedErrors: []string{"NotFound"},
		},
		{
			name: "listener is not deleted because listener does not exist in the namespace",
			args: []string{"my-listener"},
			setUpMock: func(command *CmdListenerDelete) {
				fakeSkupperClient := &fake.FakeSkupperV1alpha1{Fake: &testing2.Fake{}}
				fakeSkupperClient.Fake.ClearActions()
				fakeSkupperClient.Fake.PrependReactor("get", "listeners", func(action testing2.Action) (handled bool, ret runtime.Object, err error) {
					return true, nil, nil
				})
				command.client = fakeSkupperClient
			},
			expectedErrors: []string{"listener my-listener does not exist in namespace test"},
		},
		{
			name: "listener name is not specified",
			args: []string{},
			setUpMock: func(command *CmdListenerDelete) {
				fakeSkupperClient := &fake.FakeSkupperV1alpha1{Fake: &testing2.Fake{}}
				fakeSkupperClient.Fake.ClearActions()
				command.client = fakeSkupperClient
			},
			expectedErrors: []string{"listener name must be specified"},
		},
		{
			name: "listener name is nil",
			args: []string{""},
			setUpMock: func(command *CmdListenerDelete) {
				fakeSkupperClient := &fake.FakeSkupperV1alpha1{Fake: &testing2.Fake{}}
				fakeSkupperClient.Fake.ClearActions()
				command.client = fakeSkupperClient
			},
			expectedErrors: []string{"listener name must not be empty"},
		},
		{
			name: "listener name is not valid",
			args: []string{"my name"},
			setUpMock: func(command *CmdListenerDelete) {
				fakeSkupperClient := &fake.FakeSkupperV1alpha1{Fake: &testing2.Fake{}}
				fakeSkupperClient.Fake.ClearActions()
				command.client = fakeSkupperClient
			},
			expectedErrors: []string{"listener name is not valid: value does not match this regular expression: ^[a-z0-9]([-a-z0-9]*[a-z0-9])*(\\.[a-z0-9]([-a-z0-9]*[a-z0-9])*)*$"},
		},
		{
			name: "more than one argument is specified",
			args: []string{"my", "listener"},
			setUpMock: func(command *CmdListenerDelete) {
				fakeSkupperClient := &fake.FakeSkupperV1alpha1{Fake: &testing2.Fake{}}
				fakeSkupperClient.Fake.ClearActions()
				command.client = fakeSkupperClient
			},
			expectedErrors: []string{"only one argument is allowed for this command"},
		},
	}

	for _, test := range testTable {
		t.Run(test.name, func(t *testing.T) {

			if test.setUpMock != nil {
				test.setUpMock(command)
			}

			actualErrors := command.ValidateInput(test.args)

			actualErrorsMessages := errorsToMessages(actualErrors)

			assert.DeepEqual(t, actualErrorsMessages, test.expectedErrors)

		})
	}
}

func TestCmdListenerDelete_Run(t *testing.T) {
	type test struct {
		name         string
		setUpMock    func(command *CmdListenerDelete)
		errorMessage string
	}

	testTable := []test{
		{
			name: "runs ok",
			setUpMock: func(command *CmdListenerDelete) {
				fakeSkupperClient := &fake.FakeSkupperV1alpha1{Fake: &testing2.Fake{}}
				fakeSkupperClient.Fake.ClearActions()
				fakeSkupperClient.Fake.PrependReactor("Delete", "listeners", func(action testing2.Action) (handled bool, ret runtime.Object, err error) {
					return true, nil, nil
				})
				command.client = fakeSkupperClient
				command.name = "my-listener"
			},
		},
		{
			name: "run fails",
			setUpMock: func(command *CmdListenerDelete) {
				fakeSkupperClient := &fake.FakeSkupperV1alpha1{Fake: &testing2.Fake{}}
				fakeSkupperClient.Fake.ClearActions()
				fakeSkupperClient.Fake.PrependReactor("Delete", "listeners", func(action testing2.Action) (handled bool, ret runtime.Object, err error) {
					return true, nil, fmt.Errorf("error")
				})
				command.client = fakeSkupperClient
				command.name = "my-fail-listener"
			},
			errorMessage: "error",
		},
	}

	for _, test := range testTable {
		cmd := newCmdListenerDeleteWithMocks()
		test.setUpMock(cmd)

		//create a listener

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

func TestCmdListenerDelete_WaitUntilReady(t *testing.T) {
	type test struct {
		name        string
		setUpMock   func(command *CmdListenerDelete)
		expectError bool
	}

	testTable := []test{
		{
			name: "error deleting listener",
			setUpMock: func(command *CmdListenerDelete) {
				fakeSkupperClient := &fake.FakeSkupperV1alpha1{Fake: &testing2.Fake{}}
				fakeSkupperClient.Fake.ClearActions()
				fakeSkupperClient.Fake.PrependReactor("get", "listeners", func(action testing2.Action) (handled bool, ret runtime.Object, err error) {

					return true, &v1alpha1.Listener{
						ObjectMeta: v1.ObjectMeta{
							Name:      "my-listener",
							Namespace: "test",
						},
						Status: v1alpha1.Status{
							StatusMessage: "",
						},
					}, nil
				})
				command.client = fakeSkupperClient
				command.name = "my-listener"
			},
			expectError: true,
		},
		{
			name: "listener is not returned",
			setUpMock: func(command *CmdListenerDelete) {
				fakeSkupperClient := &fake.FakeSkupperV1alpha1{Fake: &testing2.Fake{}}
				fakeSkupperClient.Fake.ClearActions()
				fakeSkupperClient.Fake.PrependReactor("get", "listeners", func(action testing2.Action) (handled bool, ret runtime.Object, err error) {
					return true, nil, fmt.Errorf("it failed")
				})
				command.client = fakeSkupperClient
				command.name = "my-listener"
			},
			expectError: true,
		},
		{
			name: "listener is deleted",
			setUpMock: func(command *CmdListenerDelete) {
				fakeSkupperClient := &fake.FakeSkupperV1alpha1{Fake: &testing2.Fake{}}
				fakeSkupperClient.Fake.ClearActions()
				fakeSkupperClient.Fake.PrependReactor("get", "listeners", func(action testing2.Action) (handled bool, ret runtime.Object, err error) {
					return true, nil, nil
				})
				command.client = fakeSkupperClient
				command.name = "my-listener"
			},
			expectError: false,
		},
	}

	for _, test := range testTable {
		cmd := newCmdListenerDeleteWithMocks()

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

func newCmdListenerDeleteWithMocks() *CmdListenerDelete {

	cmdListenerDelete := &CmdListenerDelete{
		client:    &fake.FakeSkupperV1alpha1{Fake: &testing2.Fake{}},
		namespace: "test",
	}

	return cmdListenerDelete
}

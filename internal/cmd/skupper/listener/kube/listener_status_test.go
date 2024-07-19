package kube

import (
	"fmt"
	"testing"

	"github.com/skupperproject/skupper/internal/cmd/skupper/utils"
	"github.com/skupperproject/skupper/pkg/apis/skupper/v1alpha1"
	"github.com/skupperproject/skupper/pkg/generated/client/clientset/versioned/typed/skupper/v1alpha1/fake"
	"gotest.tools/assert"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	testing2 "k8s.io/client-go/testing"
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
		setUpMock      func(command *CmdListenerStatus)
		expectedErrors []string
	}

	command := &CmdListenerStatus{
		namespace: "test",
	}

	testTable := []test{
		{
			name: "listener is not shown because listener does not exist in the namespace",
			args: []string{"my-listener"},
			setUpMock: func(command *CmdListenerStatus) {
				fakeSkupperClient := &fake.FakeSkupperV1alpha1{Fake: &testing2.Fake{}}
				fakeSkupperClient.Fake.ClearActions()
				fakeSkupperClient.Fake.PrependReactor("get", "listeners", func(action testing2.Action) (handled bool, ret runtime.Object, err error) {
					return true, nil, fmt.Errorf("NotFound")
				})
				command.client = fakeSkupperClient
			},
			expectedErrors: []string{"listener my-listener does not exist in namespace test"},
		},
		{
			name: "listener name is nil",
			args: []string{""},
			setUpMock: func(command *CmdListenerStatus) {
				fakeSkupperClient := &fake.FakeSkupperV1alpha1{Fake: &testing2.Fake{}}
				fakeSkupperClient.Fake.ClearActions()
				command.client = fakeSkupperClient
			},
			expectedErrors: []string{"listener name must not be empty"},
		},
		{
			name: "more than one argument is specified",
			args: []string{"my", "listener"},
			setUpMock: func(command *CmdListenerStatus) {
				fakeSkupperClient := &fake.FakeSkupperV1alpha1{Fake: &testing2.Fake{}}
				fakeSkupperClient.Fake.ClearActions()
				command.client = fakeSkupperClient
			},
			expectedErrors: []string{"only one argument is allowed for this command"},
		},
		{
			name: "listener name is not valid.",
			args: []string{"my new listener"},
			setUpMock: func(command *CmdListenerStatus) {
				fakeSkupperClient := &fake.FakeSkupperV1alpha1{Fake: &testing2.Fake{}}
				fakeSkupperClient.Fake.ClearActions()
				command.client = fakeSkupperClient
			},
			expectedErrors: []string{"listener name is not valid: value does not match this regular expression: ^[a-z0-9]([-a-z0-9]*[a-z0-9])*(\\.[a-z0-9]([-a-z0-9]*[a-z0-9])*)*$"},
		},
		{
			name: "no args",
			setUpMock: func(command *CmdListenerStatus) {
				fakeSkupperClient := &fake.FakeSkupperV1alpha1{Fake: &testing2.Fake{}}
				fakeSkupperClient.Fake.ClearActions()
				command.client = fakeSkupperClient
			},
			expectedErrors: []string{},
		},
		{
			name: "bad output status",
			args: []string{"out-listener"},
			setUpMock: func(command *CmdListenerStatus) {
				fakeSkupperClient := &fake.FakeSkupperV1alpha1{Fake: &testing2.Fake{}}
				fakeSkupperClient.Fake.ClearActions()
				command.client = fakeSkupperClient
				command.flags.output = "not-supported"
			},
			expectedErrors: []string{"output type is not valid: value not-supported not allowed. It should be one of this options: [json yaml]"},
		},
		{
			name: "good output status",
			args: []string{"out-listener"},
			setUpMock: func(command *CmdListenerStatus) {
				fakeSkupperClient := &fake.FakeSkupperV1alpha1{Fake: &testing2.Fake{}}
				fakeSkupperClient.Fake.ClearActions()
				command.client = fakeSkupperClient
				command.flags.output = "json"
			},
			expectedErrors: []string{},
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

func TestCmdListenerStatus_Run(t *testing.T) {
	type test struct {
		name         string
		setUpMock    func(command *CmdListenerStatus)
		errorMessage string
	}

	testTable := []test{
		{
			name: "runs ok",
			setUpMock: func(command *CmdListenerStatus) {
				fakeSkupperClient := &fake.FakeSkupperV1alpha1{Fake: &testing2.Fake{}}
				fakeSkupperClient.Fake.ClearActions()
				fakeSkupperClient.Fake.PrependReactor("get", "listeners", func(action testing2.Action) (handled bool, ret runtime.Object, err error) {
					return true, nil, nil
				})
				command.client = fakeSkupperClient
				command.name = "my-listener"
			},
		},
		{
			name: "run fails",
			setUpMock: func(command *CmdListenerStatus) {
				fakeSkupperClient := &fake.FakeSkupperV1alpha1{Fake: &testing2.Fake{}}
				fakeSkupperClient.Fake.ClearActions()
				fakeSkupperClient.Fake.PrependReactor("list", "listeners", func(action testing2.Action) (handled bool, ret runtime.Object, err error) {
					return true, nil, fmt.Errorf("NotFound")
				})
				command.client = fakeSkupperClient
			},
			errorMessage: "NotFound",
		},
		{
			name: "run fails listener doesn't exist",
			setUpMock: func(command *CmdListenerStatus) {
				fakeSkupperClient := &fake.FakeSkupperV1alpha1{Fake: &testing2.Fake{}}
				fakeSkupperClient.Fake.ClearActions()
				fakeSkupperClient.Fake.PrependReactor("get", "listeners", func(action testing2.Action) (handled bool, ret runtime.Object, err error) {
					return true, nil, fmt.Errorf("NotFound")
				})
				command.client = fakeSkupperClient
				command.name = "my-fail-listener"
			},
			errorMessage: "NotFound",
		},
		{
			name: "runs ok, returns 1 listener",
			setUpMock: func(command *CmdListenerStatus) {
				fakeSkupperClient := &fake.FakeSkupperV1alpha1{Fake: &testing2.Fake{}}
				fakeSkupperClient.Fake.ClearActions()
				fakeSkupperClient.Fake.PrependReactor("list", "listeners", func(action testing2.Action) (handled bool, ret runtime.Object, err error) {
					listener := v1alpha1.ListenerList{
						Items: []v1alpha1.Listener{
							{
								ObjectMeta: v1.ObjectMeta{
									Name:      "my-listener",
									Namespace: "test",
								},
								Spec: v1alpha1.ListenerSpec{
									Port: 8080,
									Type: "tcp",
									Host: "test",
								},
								Status: v1alpha1.Status{
									StatusMessage: "Ok",
								},
							},
						},
					}
					return true, &listener, nil
				})
				command.client = fakeSkupperClient
				command.name = "my-listener"
			},
		},
		{
			name: "runs ok, returns 1 listener output yaml",
			setUpMock: func(command *CmdListenerStatus) {
				fakeSkupperClient := &fake.FakeSkupperV1alpha1{Fake: &testing2.Fake{}}
				fakeSkupperClient.Fake.ClearActions()
				fakeSkupperClient.Fake.PrependReactor("list", "listeners", func(action testing2.Action) (handled bool, ret runtime.Object, err error) {
					listener := v1alpha1.ListenerList{
						Items: []v1alpha1.Listener{
							{
								ObjectMeta: v1.ObjectMeta{
									Name:      "my-listener",
									Namespace: "test",
								},
								Spec: v1alpha1.ListenerSpec{
									Port: 8080,
									Type: "tcp",
									Host: "test",
								},
								Status: v1alpha1.Status{
									StatusMessage: "Ok",
								},
							},
						},
					}
					return true, &listener, nil
				})
				command.client = fakeSkupperClient
				command.name = "my-listener"
				command.output = "yaml"
			},
		},
		{
			name: "runs ok, returns all listeners",
			setUpMock: func(command *CmdListenerStatus) {
				fakeSkupperClient := &fake.FakeSkupperV1alpha1{Fake: &testing2.Fake{}}
				fakeSkupperClient.Fake.ClearActions()
				fakeSkupperClient.Fake.PrependReactor("list", "listeners", func(action testing2.Action) (handled bool, ret runtime.Object, err error) {
					listener := v1alpha1.ListenerList{
						Items: []v1alpha1.Listener{
							{
								ObjectMeta: v1.ObjectMeta{
									Name:      "my-listener1",
									Namespace: "test",
								},
								Spec: v1alpha1.ListenerSpec{
									Port: 8080,
									Type: "tcp",
									Host: "test1",
								},
								Status: v1alpha1.Status{
									StatusMessage: "Ok",
								},
							},
							{
								ObjectMeta: v1.ObjectMeta{
									Name:      "my-listener2",
									Namespace: "test",
								},
								Spec: v1alpha1.ListenerSpec{
									Port:       8888,
									Type:       "tcp",
									Host:       "test2",
									RoutingKey: "key2",
								},
								Status: v1alpha1.Status{
									StatusMessage: "Ok",
								},
							},
						},
					}
					return true, &listener, nil
				})
				command.client = fakeSkupperClient
			},
		},
		{
			name: "runs ok, returns all listeners output json",
			setUpMock: func(command *CmdListenerStatus) {
				fakeSkupperClient := &fake.FakeSkupperV1alpha1{Fake: &testing2.Fake{}}
				fakeSkupperClient.Fake.ClearActions()
				fakeSkupperClient.Fake.PrependReactor("list", "listeners", func(action testing2.Action) (handled bool, ret runtime.Object, err error) {
					listener := v1alpha1.ListenerList{
						Items: []v1alpha1.Listener{
							{
								ObjectMeta: v1.ObjectMeta{
									Name:      "my-listener1",
									Namespace: "test",
								},
								Spec: v1alpha1.ListenerSpec{
									Port: 8080,
									Type: "tcp",
									Host: "test1",
								},
								Status: v1alpha1.Status{
									StatusMessage: "Ok",
								},
							},
							{
								ObjectMeta: v1.ObjectMeta{
									Name:      "my-listener2",
									Namespace: "test",
								},
								Spec: v1alpha1.ListenerSpec{
									Port:       8888,
									Type:       "tcp",
									Host:       "test2",
									RoutingKey: "key2",
								},
								Status: v1alpha1.Status{
									StatusMessage: "Ok",
								},
							},
						},
					}
					return true, &listener, nil
				})
				command.client = fakeSkupperClient
				command.output = "json"
			},
		},
		{
			name: "runs ok, returns all listeners output bad",
			setUpMock: func(command *CmdListenerStatus) {
				fakeSkupperClient := &fake.FakeSkupperV1alpha1{Fake: &testing2.Fake{}}
				fakeSkupperClient.Fake.ClearActions()
				fakeSkupperClient.Fake.PrependReactor("list", "listeners", func(action testing2.Action) (handled bool, ret runtime.Object, err error) {
					listener := v1alpha1.ListenerList{
						Items: []v1alpha1.Listener{
							{
								ObjectMeta: v1.ObjectMeta{
									Name:      "my-listener1",
									Namespace: "test",
								},
								Spec: v1alpha1.ListenerSpec{
									Port: 8080,
									Type: "tcp",
									Host: "test1",
								},
								Status: v1alpha1.Status{
									StatusMessage: "Ok",
								},
							},
							{
								ObjectMeta: v1.ObjectMeta{
									Name:      "my-listener2",
									Namespace: "test",
								},
								Spec: v1alpha1.ListenerSpec{
									Port:       8888,
									Type:       "tcp",
									Host:       "test2",
									RoutingKey: "key2",
								},
								Status: v1alpha1.Status{
									StatusMessage: "Ok",
								},
							},
						},
					}
					return true, &listener, nil
				})
				command.client = fakeSkupperClient
				command.output = "un-supported"
			},
			errorMessage: "format un-supported not supported",
		},
		{
			name: "runs ok, returns 1 listeners output bad",
			setUpMock: func(command *CmdListenerStatus) {
				fakeSkupperClient := &fake.FakeSkupperV1alpha1{Fake: &testing2.Fake{}}
				fakeSkupperClient.Fake.ClearActions()
				fakeSkupperClient.Fake.PrependReactor("list", "listeners", func(action testing2.Action) (handled bool, ret runtime.Object, err error) {
					listener := v1alpha1.ListenerList{
						Items: []v1alpha1.Listener{
							{
								ObjectMeta: v1.ObjectMeta{
									Name:      "my-listener",
									Namespace: "test",
								},
								Spec: v1alpha1.ListenerSpec{
									Port: 8080,
									Type: "tcp",
									Host: "test",
								},
								Status: v1alpha1.Status{
									StatusMessage: "Ok",
								},
							},
						},
					}
					return true, &listener, nil
				})
				command.client = fakeSkupperClient
				command.name = "my-listener"
				command.output = "bad"
			},
			errorMessage: "format bad not supported",
		},
	}

	for _, test := range testTable {
		cmd := newCmdListenerStatusWithMocks()
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

// --- helper methods

func newCmdListenerStatusWithMocks() *CmdListenerStatus {

	cmdListenerStatus := &CmdListenerStatus{
		client:    &fake.FakeSkupperV1alpha1{Fake: &testing2.Fake{}},
		namespace: "test",
	}

	return cmdListenerStatus
}

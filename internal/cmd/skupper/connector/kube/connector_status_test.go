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
		setUpMock      func(command *CmdConnectorStatus)
		expectedErrors []string
	}

	command := &CmdConnectorStatus{
		namespace: "test",
	}

	testTable := []test{
		{
			name: "connector is not shown because connector does not exist in the namespace",
			args: []string{"my-connector"},
			setUpMock: func(command *CmdConnectorStatus) {
				fakeSkupperClient := &fake.FakeSkupperV1alpha1{Fake: &testing2.Fake{}}
				fakeSkupperClient.Fake.ClearActions()
				fakeSkupperClient.Fake.PrependReactor("get", "connectors", func(action testing2.Action) (handled bool, ret runtime.Object, err error) {
					return true, nil, fmt.Errorf("NotFound")
				})
				command.client = fakeSkupperClient
			},
			expectedErrors: []string{"connector my-connector does not exist in namespace test"},
		},
		{
			name: "connector name is nil",
			args: []string{""},
			setUpMock: func(command *CmdConnectorStatus) {
				fakeSkupperClient := &fake.FakeSkupperV1alpha1{Fake: &testing2.Fake{}}
				fakeSkupperClient.Fake.ClearActions()
				command.client = fakeSkupperClient
			},
			expectedErrors: []string{"connector name must not be empty"},
		},
		{
			name: "more than one argument is specified",
			args: []string{"my", "connector"},
			setUpMock: func(command *CmdConnectorStatus) {
				fakeSkupperClient := &fake.FakeSkupperV1alpha1{Fake: &testing2.Fake{}}
				fakeSkupperClient.Fake.ClearActions()
				command.client = fakeSkupperClient
			},
			expectedErrors: []string{"only one argument is allowed for this command"},
		},
		{
			name: "connector name is not valid.",
			args: []string{"my new connector"},
			setUpMock: func(command *CmdConnectorStatus) {
				fakeSkupperClient := &fake.FakeSkupperV1alpha1{Fake: &testing2.Fake{}}
				fakeSkupperClient.Fake.ClearActions()
				command.client = fakeSkupperClient
			},
			expectedErrors: []string{"connector name is not valid: value does not match this regular expression: ^[a-z0-9]([-a-z0-9]*[a-z0-9])*(\\.[a-z0-9]([-a-z0-9]*[a-z0-9])*)*$"},
		},
		{
			name: "no args",
			setUpMock: func(command *CmdConnectorStatus) {
				fakeSkupperClient := &fake.FakeSkupperV1alpha1{Fake: &testing2.Fake{}}
				fakeSkupperClient.Fake.ClearActions()
				command.client = fakeSkupperClient
			},
			expectedErrors: []string{},
		},
		{
			name: "bad output status",
			args: []string{"out-connector"},
			setUpMock: func(command *CmdConnectorStatus) {
				fakeSkupperClient := &fake.FakeSkupperV1alpha1{Fake: &testing2.Fake{}}
				fakeSkupperClient.Fake.ClearActions()
				command.client = fakeSkupperClient
				command.flags.output = "not-supported"
			},
			expectedErrors: []string{"output type is not valid: value not-supported not allowed. It should be one of this options: [json yaml]"},
		},
		{
			name: "good output status",
			args: []string{"out-connector"},
			setUpMock: func(command *CmdConnectorStatus) {
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

func TestCmdConnectorStatus_Run(t *testing.T) {
	type test struct {
		name         string
		setUpMock    func(command *CmdConnectorStatus)
		errorMessage string
	}

	testTable := []test{
		{
			name: "runs ok",
			setUpMock: func(command *CmdConnectorStatus) {
				fakeSkupperClient := &fake.FakeSkupperV1alpha1{Fake: &testing2.Fake{}}
				fakeSkupperClient.Fake.ClearActions()
				fakeSkupperClient.Fake.PrependReactor("get", "connectors", func(action testing2.Action) (handled bool, ret runtime.Object, err error) {
					return true, nil, nil
				})
				command.client = fakeSkupperClient
				command.name = "my-connector"
			},
		},
		{
			name: "run fails no connectors found",
			setUpMock: func(command *CmdConnectorStatus) {
				fakeSkupperClient := &fake.FakeSkupperV1alpha1{Fake: &testing2.Fake{}}
				fakeSkupperClient.Fake.ClearActions()
				fakeSkupperClient.Fake.PrependReactor("list", "connectors", func(action testing2.Action) (handled bool, ret runtime.Object, err error) {
					return true, nil, fmt.Errorf("NotFound")
				})
				command.client = fakeSkupperClient
			},
			errorMessage: "NotFound",
		},
		{
			name: "run fails connector doesn't exist",
			setUpMock: func(command *CmdConnectorStatus) {
				fakeSkupperClient := &fake.FakeSkupperV1alpha1{Fake: &testing2.Fake{}}
				fakeSkupperClient.Fake.ClearActions()
				fakeSkupperClient.Fake.PrependReactor("get", "connectors", func(action testing2.Action) (handled bool, ret runtime.Object, err error) {
					return true, nil, fmt.Errorf("NotFound")
				})
				command.client = fakeSkupperClient
				command.name = "my-fail-connector"
			},
			errorMessage: "NotFound",
		},
		{
			name: "runs ok, returns 1 connectors",
			setUpMock: func(command *CmdConnectorStatus) {
				fakeSkupperClient := &fake.FakeSkupperV1alpha1{Fake: &testing2.Fake{}}
				fakeSkupperClient.Fake.ClearActions()
				fakeSkupperClient.Fake.PrependReactor("list", "connectors", func(action testing2.Action) (handled bool, ret runtime.Object, err error) {
					connector := v1alpha1.ConnectorList{
						Items: []v1alpha1.Connector{
							{
								ObjectMeta: v1.ObjectMeta{
									Name:      "my-connector",
									Namespace: "test",
								},
								Spec: v1alpha1.ConnectorSpec{
									Port:     8080,
									Type:     "tcp",
									Host:     "test",
									Selector: "backend",
								},
								Status: v1alpha1.Status{
									StatusMessage: "Ok",
								},
							},
						},
					}
					return true, &connector, nil
				})
				command.client = fakeSkupperClient
				command.name = "my-connector"
			},
		},
		{
			name: "runs ok, returns 1 connectors yaml",
			setUpMock: func(command *CmdConnectorStatus) {
				fakeSkupperClient := &fake.FakeSkupperV1alpha1{Fake: &testing2.Fake{}}
				fakeSkupperClient.Fake.ClearActions()
				fakeSkupperClient.Fake.PrependReactor("list", "connectors", func(action testing2.Action) (handled bool, ret runtime.Object, err error) {
					connector := v1alpha1.ConnectorList{
						Items: []v1alpha1.Connector{
							{
								ObjectMeta: v1.ObjectMeta{
									Name:      "my-connector",
									Namespace: "test",
								},
								Spec: v1alpha1.ConnectorSpec{
									Port:     8080,
									Type:     "tcp",
									Host:     "test",
									Selector: "backend",
								},
								Status: v1alpha1.Status{
									StatusMessage: "Ok",
								},
							},
						},
					}
					return true, &connector, nil
				})
				command.client = fakeSkupperClient
				command.name = "my-connector"
				command.output = "yaml"
			},
		},
		{
			name: "runs ok, returns all connectors",
			setUpMock: func(command *CmdConnectorStatus) {
				fakeSkupperClient := &fake.FakeSkupperV1alpha1{Fake: &testing2.Fake{}}
				fakeSkupperClient.Fake.ClearActions()
				fakeSkupperClient.Fake.PrependReactor("list", "connectors", func(action testing2.Action) (handled bool, ret runtime.Object, err error) {
					connector := v1alpha1.ConnectorList{
						Items: []v1alpha1.Connector{
							{
								ObjectMeta: v1.ObjectMeta{
									Name:      "my-connector1",
									Namespace: "test",
								},
								Spec: v1alpha1.ConnectorSpec{
									Port:       8080,
									Type:       "tcp",
									Host:       "test1",
									RoutingKey: "key1",
									Selector:   "backend1",
								},
								Status: v1alpha1.Status{
									StatusMessage: "Ok",
								},
							},
							{
								ObjectMeta: v1.ObjectMeta{
									Name:      "my-connector2",
									Namespace: "test",
								},
								Spec: v1alpha1.ConnectorSpec{
									Port:       8888,
									Type:       "tcp",
									Host:       "test2",
									RoutingKey: "key2",
									Selector:   "backend2",
								},
								Status: v1alpha1.Status{
									StatusMessage: "Ok",
								},
							},
						},
					}
					return true, &connector, nil
				})
				command.client = fakeSkupperClient
			},
		},
		{
			name: "runs ok, returns all connectors json",
			setUpMock: func(command *CmdConnectorStatus) {
				fakeSkupperClient := &fake.FakeSkupperV1alpha1{Fake: &testing2.Fake{}}
				fakeSkupperClient.Fake.ClearActions()
				fakeSkupperClient.Fake.PrependReactor("list", "connectors", func(action testing2.Action) (handled bool, ret runtime.Object, err error) {
					connector := v1alpha1.ConnectorList{
						Items: []v1alpha1.Connector{
							{
								ObjectMeta: v1.ObjectMeta{
									Name:      "my-connector1",
									Namespace: "test",
								},
								Spec: v1alpha1.ConnectorSpec{
									Port:     8080,
									Type:     "tcp",
									Host:     "test1",
									Selector: "backend1",
								},
								Status: v1alpha1.Status{
									StatusMessage: "Ok",
								},
							},
							{
								ObjectMeta: v1.ObjectMeta{
									Name:      "my-connector2",
									Namespace: "test",
								},
								Spec: v1alpha1.ConnectorSpec{
									Port:       8888,
									Type:       "tcp",
									Host:       "test2",
									RoutingKey: "key2",
									Selector:   "backend2",
								},
								Status: v1alpha1.Status{
									StatusMessage: "Ok",
								},
							},
						},
					}
					return true, &connector, nil
				})
				command.client = fakeSkupperClient
				command.output = "json"
			},
		},
		{
			name: "runs ok, returns all connectors output bad",
			setUpMock: func(command *CmdConnectorStatus) {
				fakeSkupperClient := &fake.FakeSkupperV1alpha1{Fake: &testing2.Fake{}}
				fakeSkupperClient.Fake.ClearActions()
				fakeSkupperClient.Fake.PrependReactor("list", "connectors", func(action testing2.Action) (handled bool, ret runtime.Object, err error) {
					connector := v1alpha1.ConnectorList{
						Items: []v1alpha1.Connector{
							{
								ObjectMeta: v1.ObjectMeta{
									Name:      "my-connector1",
									Namespace: "test",
								},
								Spec: v1alpha1.ConnectorSpec{
									Port:     8080,
									Type:     "tcp",
									Host:     "test1",
									Selector: "backend1",
								},
								Status: v1alpha1.Status{
									StatusMessage: "Ok",
								},
							},
							{
								ObjectMeta: v1.ObjectMeta{
									Name:      "my-connector2",
									Namespace: "test",
								},
								Spec: v1alpha1.ConnectorSpec{
									Port:       8888,
									Type:       "tcp",
									Host:       "test2",
									RoutingKey: "key2",
									Selector:   "backend2",
								},
								Status: v1alpha1.Status{
									StatusMessage: "Ok",
								},
							},
						},
					}
					return true, &connector, nil
				})
				command.client = fakeSkupperClient
				command.output = "bad"
			},
			errorMessage: "format bad not supported",
		},
		{
			name: "runs ok, returns 1 connectors bad output",
			setUpMock: func(command *CmdConnectorStatus) {
				fakeSkupperClient := &fake.FakeSkupperV1alpha1{Fake: &testing2.Fake{}}
				fakeSkupperClient.Fake.ClearActions()
				fakeSkupperClient.Fake.PrependReactor("list", "connectors", func(action testing2.Action) (handled bool, ret runtime.Object, err error) {
					connector := v1alpha1.ConnectorList{
						Items: []v1alpha1.Connector{
							{
								ObjectMeta: v1.ObjectMeta{
									Name:      "my-connector",
									Namespace: "test",
								},
								Spec: v1alpha1.ConnectorSpec{
									Port:     8080,
									Type:     "tcp",
									Host:     "test",
									Selector: "backend",
								},
								Status: v1alpha1.Status{
									StatusMessage: "Ok",
								},
							},
						},
					}
					return true, &connector, nil
				})
				command.client = fakeSkupperClient
				command.name = "my-connector"
				command.output = "bad"
			},
			errorMessage: "format bad not supported",
		},
	}

	for _, test := range testTable {
		cmd := newCmdConnectorStatusWithMocks()
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

// --- helper methods

func newCmdConnectorStatusWithMocks() *CmdConnectorStatus {

	cmdConnectorStatus := &CmdConnectorStatus{
		client:    &fake.FakeSkupperV1alpha1{Fake: &testing2.Fake{}},
		namespace: "test",
	}

	return cmdConnectorStatus
}

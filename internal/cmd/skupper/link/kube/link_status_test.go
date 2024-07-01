package kube

import (
	"fmt"
	"github.com/skupperproject/skupper/internal/cmd/skupper/utils"
	"github.com/skupperproject/skupper/pkg/apis/skupper/v1alpha1"
	"github.com/skupperproject/skupper/pkg/generated/client/clientset/versioned/typed/skupper/v1alpha1/fake"
	"gotest.tools/assert"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	testing2 "k8s.io/client-go/testing"
	"testing"
)

func TestCmdLinkStatus_NewCmdLinkStatus(t *testing.T) {

	t.Run("link status command", func(t *testing.T) {

		result := NewCmdLinkStatus()

		assert.Check(t, result.CobraCmd.Use != "")
		assert.Check(t, result.CobraCmd.Short != "")
		assert.Check(t, result.CobraCmd.Long != "")
		assert.Check(t, result.CobraCmd.PreRun != nil)
		assert.Check(t, result.CobraCmd.Run != nil)
		assert.Check(t, result.CobraCmd.Flags() != nil)

	})

}

func TestCmdLinkStatus_ValidateInput(t *testing.T) {
	type test struct {
		name           string
		args           []string
		setUpMock      func(command *CmdLinkStatus)
		expectedErrors []string
	}

	testTable := []test{
		{
			name: "more than one argument was specified",
			args: []string{"my-link", ""},
			setUpMock: func(command *CmdLinkStatus) {
				fakeSkupperClient := &fake.FakeSkupperV1alpha1{Fake: &testing2.Fake{}}
				fakeSkupperClient.Fake.ClearActions()
				fakeSkupperClient.Fake.PrependReactor("list", "sites", func(action testing2.Action) (handled bool, ret runtime.Object, err error) {
					return true, &v1alpha1.SiteList{
						Items: []v1alpha1.Site{
							{
								ObjectMeta: v1.ObjectMeta{
									Name:      "old-site",
									Namespace: "test",
								},
							},
						},
					}, nil
				})
				command.Client = fakeSkupperClient
			},
			expectedErrors: []string{"this command does not need any arguments"},
		},
		{
			name: "there are no sites",
			args: []string{},
			setUpMock: func(command *CmdLinkStatus) {
				fakeSkupperClient := &fake.FakeSkupperV1alpha1{Fake: &testing2.Fake{}}
				fakeSkupperClient.Fake.ClearActions()
				fakeSkupperClient.Fake.PrependReactor("list", "sites", func(action testing2.Action) (handled bool, ret runtime.Object, err error) {
					return true, &v1alpha1.SiteList{}, nil
				})
				command.Client = fakeSkupperClient
			},
			expectedErrors: []string{"there is no skupper site available"},
		},
	}

	for _, test := range testTable {
		t.Run(test.name, func(t *testing.T) {

			command := &CmdLinkStatus{
				Namespace: "test",
			}

			if test.setUpMock != nil {
				test.setUpMock(command)
			}

			actualErrors := command.ValidateInput(test.args)

			actualErrorsMessages := utils.ErrorsToMessages(actualErrors)

			assert.DeepEqual(t, actualErrorsMessages, test.expectedErrors)

		})
	}
}

func TestCmdLinkStatus_Run(t *testing.T) {
	type test struct {
		name         string
		setUpMock    func(command *CmdLinkStatus)
		errorMessage string
	}

	testTable := []test{
		{
			name: "runs ok",
			setUpMock: func(command *CmdLinkStatus) {
				fakeSkupperClient := &fake.FakeSkupperV1alpha1{Fake: &testing2.Fake{}}
				fakeSkupperClient.Fake.ClearActions()
				fakeSkupperClient.Fake.PrependReactor("list", "links", func(action testing2.Action) (handled bool, ret runtime.Object, err error) {
					return true, &v1alpha1.LinkList{
						Items: []v1alpha1.Link{
							{
								ObjectMeta: v1.ObjectMeta{
									Name:      "link",
									Namespace: "test",
								},
							},
						},
					}, nil
				})
				command.Client = fakeSkupperClient
			},
		},
		{
			name: "run fails",
			setUpMock: func(command *CmdLinkStatus) {
				fakeSkupperClient := &fake.FakeSkupperV1alpha1{Fake: &testing2.Fake{}}
				fakeSkupperClient.Fake.ClearActions()
				fakeSkupperClient.Fake.PrependReactor("list", "links", func(action testing2.Action) (handled bool, ret runtime.Object, err error) {
					return true, nil, fmt.Errorf("error")
				})
				command.Client = fakeSkupperClient
			},
			errorMessage: "error",
		},
		{
			name: "there are no links",
			setUpMock: func(command *CmdLinkStatus) {
				fakeSkupperClient := &fake.FakeSkupperV1alpha1{Fake: &testing2.Fake{}}
				fakeSkupperClient.Fake.ClearActions()
				fakeSkupperClient.Fake.PrependReactor("list", "links", func(action testing2.Action) (handled bool, ret runtime.Object, err error) {
					return true, &v1alpha1.LinkList{}, nil
				})
				command.Client = fakeSkupperClient
			},
		},
	}

	for _, test := range testTable {
		cmd := newCmdLinkStatusWithMocks()
		test.setUpMock(cmd)

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

func TestCmdLinkStatus_WaitUntilReady(t *testing.T) {

	t.Run("", func(t *testing.T) {

		cmd := newCmdLinkStatusWithMocks()

		result := cmd.WaitUntilReady()
		assert.Check(t, result == nil)

	})

}

// --- helper methods

func newCmdLinkStatusWithMocks() *CmdLinkStatus {

	CmdLinkStatus := &CmdLinkStatus{
		Client:    &fake.FakeSkupperV1alpha1{Fake: &testing2.Fake{}},
		Namespace: "test",
	}

	return CmdLinkStatus
}

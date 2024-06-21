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

func TestCmdSiteStatus_NewCmdSiteStatus(t *testing.T) {

	t.Run("status command", func(t *testing.T) {

		result := NewCmdSiteStatus()

		assert.Check(t, result.CobraCmd.Use != "")
		assert.Check(t, result.CobraCmd.Short != "")
		assert.Check(t, result.CobraCmd.Long != "")
		assert.Check(t, result.CobraCmd.PreRun != nil)
		assert.Check(t, result.CobraCmd.Run != nil)
		assert.Check(t, result.CobraCmd.Flags() != nil)

	})

}

func TestCmdSiteStatus_ValidateInput(t *testing.T) {
	type test struct {
		name           string
		args           []string
		setUpMock      func(command *CmdSiteStatus)
		expectedErrors []string
	}

	command := &CmdSiteStatus{
		Namespace: "test",
	}

	testTable := []test{
		{
			name: "more than one argument was specified",
			args: []string{"my-site", ""},
			setUpMock: func(command *CmdSiteStatus) {
				fakeSkupperClient := &fake.FakeSkupperV1alpha1{Fake: &testing2.Fake{}}
				fakeSkupperClient.Fake.ClearActions()
				command.Client = fakeSkupperClient
			},
			expectedErrors: []string{"this command does not need any arguments"},
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

func TestCmdSiteStatus_Run(t *testing.T) {
	type test struct {
		name         string
		setUpMock    func(command *CmdSiteStatus)
		errorMessage string
	}

	testTable := []test{
		{
			name: "runs ok",
			setUpMock: func(command *CmdSiteStatus) {
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
				command.Client = fakeSkupperClient
			},
		},
		{
			name: "run fails",
			setUpMock: func(command *CmdSiteStatus) {
				fakeSkupperClient := &fake.FakeSkupperV1alpha1{Fake: &testing2.Fake{}}
				fakeSkupperClient.Fake.ClearActions()
				fakeSkupperClient.Fake.PrependReactor("list", "sites", func(action testing2.Action) (handled bool, ret runtime.Object, err error) {
					return true, nil, fmt.Errorf("error")
				})
				command.Client = fakeSkupperClient
			},
			errorMessage: "error",
		},
		{
			name: "there is no existing skupper site",
			setUpMock: func(command *CmdSiteStatus) {
				fakeSkupperClient := &fake.FakeSkupperV1alpha1{Fake: &testing2.Fake{}}
				fakeSkupperClient.Fake.ClearActions()
				fakeSkupperClient.Fake.PrependReactor("list", "sites", func(action testing2.Action) (handled bool, ret runtime.Object, err error) {
					return true, &v1alpha1.SiteList{}, nil
				})
				command.Client = fakeSkupperClient
			},
		},
	}

	for _, test := range testTable {
		cmd := newCmdSiteStatusWithMocks()
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

func TestCmdSiteStatus_WaitUntilReady(t *testing.T) {

	t.Run("", func(t *testing.T) {

		cmd := newCmdSiteStatusWithMocks()

		result := cmd.WaitUntilReady()
		assert.Check(t, result == nil)

	})

}

// --- helper methods

func newCmdSiteStatusWithMocks() *CmdSiteStatus {

	CmdSiteStatus := &CmdSiteStatus{
		Client:    &fake.FakeSkupperV1alpha1{Fake: &testing2.Fake{}},
		Namespace: "test",
	}

	return CmdSiteStatus
}

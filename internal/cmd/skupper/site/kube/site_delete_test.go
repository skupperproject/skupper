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

func TestCmdSiteDelete_NewCmdSiteDelete(t *testing.T) {

	t.Run("delete command", func(t *testing.T) {

		result := NewCmdSiteDelete()

		assert.Check(t, result.CobraCmd.Use != "")
		assert.Check(t, result.CobraCmd.Short != "")
		assert.Check(t, result.CobraCmd.Long != "")
		assert.Check(t, result.CobraCmd.PreRun != nil)
		assert.Check(t, result.CobraCmd.Run != nil)
		assert.Check(t, result.CobraCmd.PostRunE != nil)
		assert.Check(t, result.CobraCmd.Flags() != nil)

	})

}

func TestCmdSiteDelete_ValidateInput(t *testing.T) {
	type test struct {
		name           string
		args           []string
		setUpMock      func(command *CmdSiteDelete)
		expectedErrors []string
	}

	command := &CmdSiteDelete{
		Namespace: "test",
	}

	testTable := []test{
		{
			name: "site is not deleted because it does not exist",
			args: []string{"my-site"},
			setUpMock: func(command *CmdSiteDelete) {
				fakeSkupperClient := &fake.FakeSkupperV1alpha1{Fake: &testing2.Fake{}}
				fakeSkupperClient.Fake.ClearActions()
				fakeSkupperClient.Fake.PrependReactor("get", "sites", func(action testing2.Action) (handled bool, ret runtime.Object, err error) {
					return true, nil, nil
				})
				command.Client = fakeSkupperClient
			},
			expectedErrors: []string{"there is no site with name \"my-site\""},
		},
		{
			name: "site is not deleted because there is an error trying to retrieve it",
			args: []string{"my-site"},
			setUpMock: func(command *CmdSiteDelete) {
				fakeSkupperClient := &fake.FakeSkupperV1alpha1{Fake: &testing2.Fake{}}
				fakeSkupperClient.Fake.ClearActions()
				fakeSkupperClient.Fake.PrependReactor("get", "sites", func(action testing2.Action) (handled bool, ret runtime.Object, err error) {
					return true, nil, fmt.Errorf("error getting the site")
				})
				command.Client = fakeSkupperClient
			},
			expectedErrors: []string{"error getting the site"},
		},
		{
			name: "site name is not specified.",
			args: []string{},
			setUpMock: func(command *CmdSiteDelete) {
				fakeSkupperClient := &fake.FakeSkupperV1alpha1{Fake: &testing2.Fake{}}
				fakeSkupperClient.Fake.ClearActions()
				command.Client = fakeSkupperClient
			},
			expectedErrors: []string{"site name must not be empty"},
		},
		{
			name: "more than one argument was specified",
			args: []string{"my", "site"},
			setUpMock: func(command *CmdSiteDelete) {
				fakeSkupperClient := &fake.FakeSkupperV1alpha1{Fake: &testing2.Fake{}}
				fakeSkupperClient.Fake.ClearActions()
				command.Client = fakeSkupperClient
			},
			expectedErrors: []string{"only one argument is allowed for this command."},
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

func TestCmdSiteDelete_Run(t *testing.T) {
	type test struct {
		name         string
		setUpMock    func(command *CmdSiteDelete)
		errorMessage string
	}

	testTable := []test{
		{
			name: "runs ok",
			setUpMock: func(command *CmdSiteDelete) {
				fakeSkupperClient := &fake.FakeSkupperV1alpha1{Fake: &testing2.Fake{}}
				fakeSkupperClient.Fake.ClearActions()
				fakeSkupperClient.Fake.PrependReactor("delete", "sites", func(action testing2.Action) (handled bool, ret runtime.Object, err error) {
					deleteAction, ok := action.(testing2.DeleteAction)
					if !ok {
						return
					}
					siteName := deleteAction.GetName()

					if siteName != "my-site" {
						return true, nil, fmt.Errorf("unexpected value")
					}

					return true, nil, nil
				})
				command.Client = fakeSkupperClient
				command.siteName = "my-site"
			},
		},
		{
			name: "run fails",
			setUpMock: func(command *CmdSiteDelete) {
				fakeSkupperClient := &fake.FakeSkupperV1alpha1{Fake: &testing2.Fake{}}
				fakeSkupperClient.Fake.ClearActions()
				fakeSkupperClient.Fake.PrependReactor("delete", "sites", func(action testing2.Action) (handled bool, ret runtime.Object, err error) {
					return true, nil, fmt.Errorf("error")
				})
				command.Client = fakeSkupperClient
				command.siteName = "my-site"
			},
			errorMessage: "error",
		},
	}

	for _, test := range testTable {
		cmd := newCmdSiteDeleteWithMocks()
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

func TestCmdSiteDelete_WaitUntilReady(t *testing.T) {
	type test struct {
		name        string
		setUpMock   func(command *CmdSiteDelete)
		expectError bool
	}

	testTable := []test{
		{
			name: "site is not deleted",
			setUpMock: func(command *CmdSiteDelete) {
				fakeSkupperClient := &fake.FakeSkupperV1alpha1{Fake: &testing2.Fake{}}
				fakeSkupperClient.Fake.ClearActions()
				fakeSkupperClient.Fake.PrependReactor("get", "sites", func(action testing2.Action) (handled bool, ret runtime.Object, err error) {

					return true, &v1alpha1.Site{
						ObjectMeta: v1.ObjectMeta{
							Name:      "my-site",
							Namespace: "test",
						},
						Status: v1alpha1.SiteStatus{
							Status: v1alpha1.Status{
								StatusMessage: "",
							},
						},
					}, nil
				})
				command.Client = fakeSkupperClient
			},
			expectError: true,
		},
		{
			name: "site is deleted",
			setUpMock: func(command *CmdSiteDelete) {
				fakeSkupperClient := &fake.FakeSkupperV1alpha1{Fake: &testing2.Fake{}}
				fakeSkupperClient.Fake.ClearActions()
				fakeSkupperClient.Fake.PrependReactor("get", "sites", func(action testing2.Action) (handled bool, ret runtime.Object, err error) {
					return true, nil, fmt.Errorf("no site")
				})
				command.Client = fakeSkupperClient
			},
			expectError: false,
		},
	}

	for _, test := range testTable {
		cmd := newCmdSiteDeleteWithMocks()
		cmd.siteName = "my-site"
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

func newCmdSiteDeleteWithMocks() *CmdSiteDelete {

	cmdSiteDelete := &CmdSiteDelete{
		Client:    &fake.FakeSkupperV1alpha1{Fake: &testing2.Fake{}},
		Namespace: "test",
	}

	return cmdSiteDelete
}

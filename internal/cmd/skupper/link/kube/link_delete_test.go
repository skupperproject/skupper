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

func TestCmdLinkDelete_NewCmdLinkDelete(t *testing.T) {

	t.Run("link delete command", func(t *testing.T) {

		result := NewCmdLinkDelete()

		assert.Check(t, result.CobraCmd.Use != "")
		assert.Check(t, result.CobraCmd.Short != "")
		assert.Check(t, result.CobraCmd.Long != "")
		assert.Check(t, result.CobraCmd.PreRun != nil)
		assert.Check(t, result.CobraCmd.Run != nil)
		assert.Check(t, result.CobraCmd.PostRunE != nil)
		assert.Check(t, result.CobraCmd.Flags() != nil)

	})

}

func TestCmdLinkDelete_ValidateInput(t *testing.T) {
	type test struct {
		name           string
		args           []string
		setUpMock      func(command *CmdLinkDelete)
		expectedErrors []string
	}

	testTable := []test{
		{
			name: "there is no site in the namespace.",
			args: []string{"my-link"},
			setUpMock: func(command *CmdLinkDelete) {
				fakeSkupperClient := &fake.FakeSkupperV1alpha1{Fake: &testing2.Fake{}}
				fakeSkupperClient.Fake.ClearActions()
				fakeSkupperClient.Fake.PrependReactor("list", "sites", func(action testing2.Action) (handled bool, ret runtime.Object, err error) {
					return true, &v1alpha1.SiteList{}, nil
				})
				command.Client = fakeSkupperClient
			},
			expectedErrors: []string{"there is no skupper site in this namespace"},
		},
		{
			name: "link is not deleted because it does not exist",
			args: []string{"my-link"},
			setUpMock: func(command *CmdLinkDelete) {
				fakeSkupperClient := &fake.FakeSkupperV1alpha1{Fake: &testing2.Fake{}}
				fakeSkupperClient.Fake.ClearActions()
				fakeSkupperClient.Fake.PrependReactor("list", "sites", func(action testing2.Action) (handled bool, ret runtime.Object, err error) {
					return true, &v1alpha1.SiteList{
						Items: []v1alpha1.Site{
							{
								ObjectMeta: v1.ObjectMeta{
									Name:      "the-site",
									Namespace: "test",
								},
							},
						},
					}, nil
				})
				fakeSkupperClient.Fake.PrependReactor("get", "links", func(action testing2.Action) (handled bool, ret runtime.Object, err error) {
					return true, nil, nil
				})
				command.Client = fakeSkupperClient
				command.Namespace = "test"
			},
			expectedErrors: []string{"the link \"my-link\" is not available in the namespace"},
		},
		{
			name: "more than one argument was specified",
			args: []string{"my", "link"},
			setUpMock: func(command *CmdLinkDelete) {
				fakeSkupperClient := &fake.FakeSkupperV1alpha1{Fake: &testing2.Fake{}}
				fakeSkupperClient.Fake.ClearActions()
				fakeSkupperClient.Fake.PrependReactor("list", "sites", func(action testing2.Action) (handled bool, ret runtime.Object, err error) {
					return true, &v1alpha1.SiteList{
						Items: []v1alpha1.Site{
							{
								ObjectMeta: v1.ObjectMeta{
									Name:      "my-site",
									Namespace: "test",
								},
							},
						},
					}, nil
				})
				command.Client = fakeSkupperClient
			},
			expectedErrors: []string{"only one argument is allowed for this command"},
		},
		{
			name: "trying to delete without specifying a name",
			args: []string{""},
			setUpMock: func(command *CmdLinkDelete) {
				fakeSkupperClient := &fake.FakeSkupperV1alpha1{Fake: &testing2.Fake{}}
				fakeSkupperClient.Fake.ClearActions()
				fakeSkupperClient.Fake.PrependReactor("list", "sites", func(action testing2.Action) (handled bool, ret runtime.Object, err error) {
					return true, &v1alpha1.SiteList{
						Items: []v1alpha1.Site{
							{
								ObjectMeta: v1.ObjectMeta{
									Name:      "my-site",
									Namespace: "test",
								},
							},
						},
					}, nil
				})
				command.Client = fakeSkupperClient
			},
			expectedErrors: []string{"link name must not be empty"},
		},
		{
			name: "deleting the link successfully",
			args: []string{"my-link"},
			setUpMock: func(command *CmdLinkDelete) {
				fakeSkupperClient := &fake.FakeSkupperV1alpha1{Fake: &testing2.Fake{}}
				fakeSkupperClient.Fake.ClearActions()
				fakeSkupperClient.Fake.PrependReactor("list", "sites", func(action testing2.Action) (handled bool, ret runtime.Object, err error) {
					return true, &v1alpha1.SiteList{
						Items: []v1alpha1.Site{
							{
								ObjectMeta: v1.ObjectMeta{
									Name:      "my-site",
									Namespace: "test",
								},
							},
						},
					}, nil
				})
				command.Client = fakeSkupperClient
			},
			expectedErrors: []string{},
		},
	}

	for _, test := range testTable {
		t.Run(test.name, func(t *testing.T) {

			command := &CmdLinkDelete{
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

func TestCmdLinkDelete_Run(t *testing.T) {
	type test struct {
		name         string
		setUpMock    func(command *CmdLinkDelete)
		errorMessage string
	}

	testTable := []test{
		{
			name: "runs ok",
			setUpMock: func(command *CmdLinkDelete) {
				fakeSkupperClient := &fake.FakeSkupperV1alpha1{Fake: &testing2.Fake{}}
				fakeSkupperClient.Fake.ClearActions()
				fakeSkupperClient.Fake.PrependReactor("delete", "sites", func(action testing2.Action) (handled bool, ret runtime.Object, err error) {
					deleteAction, ok := action.(testing2.DeleteAction)
					if !ok {
						return
					}
					linkName := deleteAction.GetName()

					if linkName != "my-site" {
						return true, nil, fmt.Errorf("unexpected value")
					}

					return true, nil, nil
				})
				command.Client = fakeSkupperClient
				command.linkName = "my-site"
			},
		},
		{
			name: "run fails",
			setUpMock: func(command *CmdLinkDelete) {
				fakeSkupperClient := &fake.FakeSkupperV1alpha1{Fake: &testing2.Fake{}}
				fakeSkupperClient.Fake.ClearActions()
				fakeSkupperClient.Fake.PrependReactor("delete", "sites", func(action testing2.Action) (handled bool, ret runtime.Object, err error) {
					return true, nil, fmt.Errorf("error")
				})
				command.Client = fakeSkupperClient
				command.linkName = "my-site"
			},
			errorMessage: "error",
		},
	}

	for _, test := range testTable {
		cmd := newCmdLinkDeleteWithMocks()
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

func TestCmdLinkDelete_WaitUntilReady(t *testing.T) {
	type test struct {
		name        string
		setUpMock   func(command *CmdLinkDelete)
		expectError bool
	}

	testTable := []test{
		{
			name: "link is not deleted",
			setUpMock: func(command *CmdLinkDelete) {
				fakeSkupperClient := &fake.FakeSkupperV1alpha1{Fake: &testing2.Fake{}}
				fakeSkupperClient.Fake.ClearActions()
				fakeSkupperClient.Fake.PrependReactor("get", "links", func(action testing2.Action) (handled bool, ret runtime.Object, err error) {

					return true, &v1alpha1.Link{
						ObjectMeta: v1.ObjectMeta{
							Name:      "my-link",
							Namespace: "test",
						},
						Status: v1alpha1.LinkStatus{
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
			name: "link is deleted",
			setUpMock: func(command *CmdLinkDelete) {
				fakeSkupperClient := &fake.FakeSkupperV1alpha1{Fake: &testing2.Fake{}}
				fakeSkupperClient.Fake.ClearActions()
				fakeSkupperClient.Fake.PrependReactor("get", "links", func(action testing2.Action) (handled bool, ret runtime.Object, err error) {
					return true, nil, fmt.Errorf("no site")
				})
				command.Client = fakeSkupperClient
			},
			expectError: false,
		},
	}

	for _, test := range testTable {
		cmd := newCmdLinkDeleteWithMocks()
		cmd.linkName = "my-site"
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

func newCmdLinkDeleteWithMocks() *CmdLinkDelete {

	CmdLinkDelete := &CmdLinkDelete{
		Client:    &fake.FakeSkupperV1alpha1{Fake: &testing2.Fake{}},
		Namespace: "test",
	}

	return CmdLinkDelete
}

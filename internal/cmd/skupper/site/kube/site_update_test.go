package kube

import (
	"fmt"
	"github.com/skupperproject/skupper/internal/cmd/skupper/utils"
	"github.com/skupperproject/skupper/pkg/apis/skupper/v1alpha1"
	"github.com/skupperproject/skupper/pkg/generated/client/clientset/versioned/typed/skupper/v1alpha1/fake"
	"github.com/spf13/pflag"
	"gotest.tools/assert"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	testing2 "k8s.io/client-go/testing"
	"testing"
)

func TestCmdSiteUpdate_NewCmdSiteUpdate(t *testing.T) {

	t.Run("update command", func(t *testing.T) {

		result := NewCmdSiteUpdate()

		assert.Check(t, result.CobraCmd.Use != "")
		assert.Check(t, result.CobraCmd.Short != "")
		assert.Check(t, result.CobraCmd.Long != "")
		assert.Check(t, result.CobraCmd.PreRun != nil)
		assert.Check(t, result.CobraCmd.Run != nil)
		assert.Check(t, result.CobraCmd.PostRunE != nil)
		assert.Check(t, result.CobraCmd.Flags() != nil)

	})

}

func TestCmdSiteUpdate_AddFlags(t *testing.T) {

	expectedFlagsWithDefaultValue := map[string]interface{}{
		"enable-link-access": "false",
		"link-access-type":   "",
		"service-account":    "skupper-controller",
		"output":             "",
	}
	var flagList []string

	cmd := newCmdSiteUpdateWithMocks()

	t.Run("add flags", func(t *testing.T) {

		cmd.CobraCmd.Flags().VisitAll(func(flag *pflag.Flag) {
			flagList = append(flagList, flag.Name)
		})

		assert.Check(t, len(flagList) == 0)

		cmd.AddFlags()

		cmd.CobraCmd.Flags().VisitAll(func(flag *pflag.Flag) {
			flagList = append(flagList, flag.Name)
			assert.Check(t, expectedFlagsWithDefaultValue[flag.Name] != nil, fmt.Sprintf("flag %q not expected", flag.Name))
			assert.Check(t, expectedFlagsWithDefaultValue[flag.Name] == flag.DefValue)
		})

		assert.Check(t, len(flagList) == len(expectedFlagsWithDefaultValue))

	})

}

func TestCmdSiteUpdate_ValidateInput(t *testing.T) {
	type test struct {
		name           string
		args           []string
		setUpMock      func(command *CmdSiteUpdate)
		expectedErrors []string
	}

	command := &CmdSiteUpdate{
		Namespace: "test",
	}

	testTable := []test{
		{
			name: "site is updated because there is already a site in the namespace.",
			args: []string{"my-site"},
			setUpMock: func(command *CmdSiteUpdate) {
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
		{
			name: "site name is not specified.",
			args: []string{},
			setUpMock: func(command *CmdSiteUpdate) {
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
		{
			name: "more than one argument was specified",
			args: []string{"my", "site"},
			setUpMock: func(command *CmdSiteUpdate) {
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
			name: "service account name is not valid.",
			args: []string{"my-site"},
			setUpMock: func(command *CmdSiteUpdate) {
				fakeSkupperClient := &fake.FakeSkupperV1alpha1{Fake: &testing2.Fake{}}
				fakeSkupperClient.Fake.ClearActions()
				command.Client = fakeSkupperClient
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
				command.flags = UpdateFlags{serviceAccount: "not valid service account name"}
			},
			expectedErrors: []string{"service account name is not valid: value does not match this regular expression: ^[a-z0-9]([-a-z0-9]*[a-z0-9])*(\\.[a-z0-9]([-a-z0-9]*[a-z0-9])*)*$"},
		},
		{
			name: "link access type is not valid",
			args: []string{"my-site"},
			setUpMock: func(command *CmdSiteUpdate) {
				fakeSkupperClient := &fake.FakeSkupperV1alpha1{Fake: &testing2.Fake{}}
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
				fakeSkupperClient.Fake.ClearActions()
				command.Client = fakeSkupperClient
				command.flags = UpdateFlags{linkAccessType: "not-valid"}
			},
			expectedErrors: []string{
				"link access type is not valid: value not-valid not allowed. It should be one of this options: [route loadbalancer default]",
				"for the site to work with this type of linkAccess, the --enable-link-access option must be set to true",
			},
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

func TestCmdSiteUpdate_InputToOptions(t *testing.T) {

	type test struct {
		name               string
		args               []string
		flags              UpdateFlags
		expectedSettings   map[string]string
		expectedLinkAccess string
		expectedOutput     string
	}

	testTable := []test{
		{
			name:  "options without link access enabled",
			args:  []string{"my-site"},
			flags: UpdateFlags{},
			expectedSettings: map[string]string{
				"name": "my-site",
			},
			expectedLinkAccess: "none",
			expectedOutput:     "",
		},
		{
			name:  "options with link access enabled but using a type by default and link access host specified",
			args:  []string{"my-site"},
			flags: UpdateFlags{enableLinkAccess: true},
			expectedSettings: map[string]string{
				"name": "my-site",
			},
			expectedLinkAccess: "loadbalancer",
			expectedOutput:     "",
		},
		{
			name:  "options with link access enabled using the nodeport type",
			args:  []string{"my-site"},
			flags: UpdateFlags{enableLinkAccess: true, linkAccessType: "nodeport"},
			expectedSettings: map[string]string{
				"name": "my-site",
			},
			expectedLinkAccess: "nodeport",
			expectedOutput:     "",
		},
		{
			name:  "options with link access options not well specified",
			args:  []string{"my-site"},
			flags: UpdateFlags{enableLinkAccess: false, linkAccessType: "nodeport"},
			expectedSettings: map[string]string{
				"name": "my-site",
			},
			expectedLinkAccess: "none",
			expectedOutput:     "",
		},
		{
			name:  "options output type",
			args:  []string{"my-site"},
			flags: UpdateFlags{enableLinkAccess: false, linkAccessType: "nodeport", output: "yaml"},
			expectedSettings: map[string]string{
				"name": "my-site",
			},
			expectedLinkAccess: "none",
			expectedOutput:     "yaml",
		},
	}

	for _, test := range testTable {
		t.Run(test.name, func(t *testing.T) {

			cmd := newCmdSiteUpdateWithMocks()
			cmd.flags = test.flags
			cmd.siteName = "my-site"

			cmd.InputToOptions()

			assert.DeepEqual(t, cmd.options, test.expectedSettings)

			assert.Check(t, cmd.output == test.expectedOutput)
		})
	}
}

func TestCmdSiteUpdate_Run(t *testing.T) {
	type test struct {
		name         string
		setUpMock    func(command *CmdSiteUpdate)
		errorMessage string
	}

	testTable := []test{
		{
			name: "runs ok",
			setUpMock: func(command *CmdSiteUpdate) {
				fakeSkupperClient := &fake.FakeSkupperV1alpha1{Fake: &testing2.Fake{}}
				fakeSkupperClient.Fake.ClearActions()
				fakeSkupperClient.Fake.PrependReactor("update", "sites", func(action testing2.Action) (handled bool, ret runtime.Object, err error) {
					createAction, ok := action.(testing2.CreateActionImpl)
					if !ok {
						return
					}
					createdObject := createAction.GetObject()

					site, ok := createdObject.(*v1alpha1.Site)
					if !ok {
						return
					}

					if site.Name != "my-site" {
						return true, nil, fmt.Errorf("unexpected value")
					}

					if site.Spec.Settings["name"] != "my-site" {
						return true, nil, fmt.Errorf("unexpected value")
					}

					if site.Spec.ServiceAccount != "my-service-account" {
						return true, nil, fmt.Errorf("unexpected value")
					}

					return true, nil, nil
				})
				command.Client = fakeSkupperClient
				command.siteName = "my-site"
				command.serviceAccountName = "my-service-account"
				command.options = map[string]string{"name": "my-site"}
			},
		},
		{
			name: "run fails",
			setUpMock: func(command *CmdSiteUpdate) {
				fakeSkupperClient := &fake.FakeSkupperV1alpha1{Fake: &testing2.Fake{}}
				fakeSkupperClient.Fake.ClearActions()
				fakeSkupperClient.Fake.PrependReactor("update", "sites", func(action testing2.Action) (handled bool, ret runtime.Object, err error) {
					return true, nil, fmt.Errorf("error")
				})
				command.Client = fakeSkupperClient
				command.siteName = "my-site"
			},
			errorMessage: "error",
		},
		{
			name: "runs ok without creating site",
			setUpMock: func(command *CmdSiteUpdate) {
				fakeSkupperClient := &fake.FakeSkupperV1alpha1{Fake: &testing2.Fake{}}
				fakeSkupperClient.Fake.ClearActions()
				command.Client = fakeSkupperClient
				command.siteName = "my-site"
				command.serviceAccountName = "my-service-account"
				command.options = map[string]string{"name": "my-site"}
				command.siteName = "test"
				command.output = "yaml"
			},
		},
		{
			name: "runs fails because the output format is not supported",
			setUpMock: func(command *CmdSiteUpdate) {
				fakeSkupperClient := &fake.FakeSkupperV1alpha1{Fake: &testing2.Fake{}}
				fakeSkupperClient.Fake.ClearActions()
				command.Client = fakeSkupperClient
				command.siteName = "my-site"
				command.serviceAccountName = "my-service-account"
				command.options = map[string]string{"name": "my-site"}
				command.siteName = "test"
				command.output = "unsupported"
			},
			errorMessage: "format unsupported not supported",
		},
	}

	for _, test := range testTable {
		cmd := newCmdSiteUpdateWithMocks()
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

func TestCmdSiteUpdate_WaitUntilReady(t *testing.T) {
	type test struct {
		name        string
		setUpMock   func(command *CmdSiteUpdate)
		expectError bool
	}

	testTable := []test{
		{
			name: "site is not ready",
			setUpMock: func(command *CmdSiteUpdate) {
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
			name: "site is not returned",
			setUpMock: func(command *CmdSiteUpdate) {
				fakeSkupperClient := &fake.FakeSkupperV1alpha1{Fake: &testing2.Fake{}}
				fakeSkupperClient.Fake.ClearActions()
				fakeSkupperClient.Fake.PrependReactor("get", "sites", func(action testing2.Action) (handled bool, ret runtime.Object, err error) {
					return true, nil, fmt.Errorf("it failed")
				})
				command.Client = fakeSkupperClient
			},
			expectError: true,
		},
		{
			name: "there is no need to wait for a site, the user just wanted the output",
			setUpMock: func(command *CmdSiteUpdate) {
				fakeSkupperClient := &fake.FakeSkupperV1alpha1{Fake: &testing2.Fake{}}
				fakeSkupperClient.Fake.ClearActions()
				command.Client = fakeSkupperClient
			},
			expectError: false,
		},
	}

	for _, test := range testTable {
		cmd := newCmdSiteUpdateWithMocks()
		cmd.siteName = "my-site"
		cmd.output = "json"
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

func newCmdSiteUpdateWithMocks() *CmdSiteUpdate {

	CmdSiteUpdate := &CmdSiteUpdate{
		Client:    &fake.FakeSkupperV1alpha1{Fake: &testing2.Fake{}},
		Namespace: "test",
	}

	return CmdSiteUpdate
}

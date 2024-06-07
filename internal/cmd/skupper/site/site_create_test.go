package site

import (
	"fmt"
	"github.com/skupperproject/skupper/pkg/apis/skupper/v1alpha1"
	"github.com/skupperproject/skupper/pkg/generated/client/clientset/versioned/typed/skupper/v1alpha1/fake"
	"github.com/spf13/pflag"
	"gotest.tools/assert"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	testing2 "k8s.io/client-go/testing"
	"testing"
)

func TestCmdSiteCreate_NewCmdSiteCreate(t *testing.T) {

	t.Run("create command", func(t *testing.T) {

		result := NewCmdSiteCreate()

		assert.Check(t, result.CobraCmd.Use != "")
		assert.Check(t, result.CobraCmd.Short != "")
		assert.Check(t, result.CobraCmd.Long != "")
		assert.Check(t, result.CobraCmd.PreRun != nil)
		assert.Check(t, result.CobraCmd.Run != nil)
		assert.Check(t, result.CobraCmd.PostRunE != nil)
		assert.Check(t, result.CobraCmd.Flags() != nil)

	})

}

func TestCmdSiteCreate_AddFlags(t *testing.T) {

	expectedFlagsWithDefaultValue := map[string]interface{}{
		"enable-link-access": "false",
		"link-access-type":   "",
		"link-access-host":   "",
		"service-account":    "skupper-controller",
	}
	var flagList []string

	cmd := newCmdSiteCreateWithMocks()

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

func TestCmdSiteCreate_ValidateInput(t *testing.T) {
	type test struct {
		name           string
		args           []string
		setUpMock      func(command *CmdSiteCreate)
		expectedErrors []string
	}

	command := &CmdSiteCreate{
		Namespace: "test",
	}

	testTable := []test{
		{
			name: "site is not created because there is already a site in the namespace.",
			args: []string{"my-new-site"},
			setUpMock: func(command *CmdSiteCreate) {
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
			expectedErrors: []string{"there is already a site created for this namespace"},
		},
		{
			name: "site name is not valid.",
			args: []string{"my new site"},
			setUpMock: func(command *CmdSiteCreate) {
				fakeSkupperClient := &fake.FakeSkupperV1alpha1{Fake: &testing2.Fake{}}
				fakeSkupperClient.Fake.ClearActions()
				command.Client = fakeSkupperClient
			},
			expectedErrors: []string{"site name is not valid: value contains spaces"},
		},
		{
			name: "site name is not specified.",
			args: []string{},
			setUpMock: func(command *CmdSiteCreate) {
				fakeSkupperClient := &fake.FakeSkupperV1alpha1{Fake: &testing2.Fake{}}
				fakeSkupperClient.Fake.ClearActions()
				command.Client = fakeSkupperClient
			},
			expectedErrors: []string{"site name must not be empty"},
		},
		{
			name: "more than one argument was specified",
			args: []string{"my", "site"},
			setUpMock: func(command *CmdSiteCreate) {
				fakeSkupperClient := &fake.FakeSkupperV1alpha1{Fake: &testing2.Fake{}}
				fakeSkupperClient.Fake.ClearActions()
				command.Client = fakeSkupperClient
			},
			expectedErrors: []string{"only one argument is allowed for this command."},
		},
		{
			name: "service account name is not valid.",
			args: []string{"my-site"},
			setUpMock: func(command *CmdSiteCreate) {
				fakeSkupperClient := &fake.FakeSkupperV1alpha1{Fake: &testing2.Fake{}}
				fakeSkupperClient.Fake.ClearActions()
				command.Client = fakeSkupperClient
				command.flags = CreateFlags{serviceAccount: "not valid service account name"}
			},
			expectedErrors: []string{"service account name is not valid: value contains spaces"},
		},
		{
			name: "link access type is not valid",
			args: []string{"my-site"},
			setUpMock: func(command *CmdSiteCreate) {
				fakeSkupperClient := &fake.FakeSkupperV1alpha1{Fake: &testing2.Fake{}}
				fakeSkupperClient.Fake.ClearActions()
				command.Client = fakeSkupperClient
				command.flags = CreateFlags{linkAccessType: "not-valid"}
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

			actualErrorsMessages := errorsToMessages(actualErrors)

			assert.DeepEqual(t, actualErrorsMessages, test.expectedErrors)

		})
	}
}

func TestCmdSiteCreate_InputToOptions(t *testing.T) {

	type test struct {
		name               string
		args               []string
		flags              CreateFlags
		expectedSettings   map[string]string
		expectedLinkAccess string
	}

	testTable := []test{
		{
			name:  "options without link access enabled",
			args:  []string{"my-site"},
			flags: CreateFlags{},
			expectedSettings: map[string]string{
				"name": "my-site",
			},
			expectedLinkAccess: "none",
		},
		{
			name:  "options with link access enabled but using a type by default and link access host specified",
			args:  []string{"my-site"},
			flags: CreateFlags{enableLinkAccess: true, linkAccessHost: "host"},
			expectedSettings: map[string]string{
				"name":         "my-site",
				"ingress-host": "host",
			},
			expectedLinkAccess: "loadbalancer",
		},
		{
			name:  "options with link access enabled using the nodeport type",
			args:  []string{"my-site"},
			flags: CreateFlags{enableLinkAccess: true, linkAccessType: "nodeport"},
			expectedSettings: map[string]string{
				"name": "my-site",
			},
			expectedLinkAccess: "nodeport",
		},
		{
			name:  "options with link access options not well specified",
			args:  []string{"my-site"},
			flags: CreateFlags{enableLinkAccess: false, linkAccessType: "nodeport"},
			expectedSettings: map[string]string{
				"name": "my-site",
			},
			expectedLinkAccess: "none",
		},
	}

	for _, test := range testTable {
		t.Run(test.name, func(t *testing.T) {

			cmd := newCmdSiteCreateWithMocks()
			cmd.flags = test.flags
			cmd.siteName = "my-site"

			cmd.InputToOptions()

			assert.DeepEqual(t, cmd.options, test.expectedSettings)
		})
	}
}

func TestCmdSiteCreate_Run(t *testing.T) {
	type test struct {
		name         string
		setUpMock    func(command *CmdSiteCreate)
		errorMessage string
	}

	testTable := []test{
		{
			name: "runs ok",
			setUpMock: func(command *CmdSiteCreate) {
				fakeSkupperClient := &fake.FakeSkupperV1alpha1{Fake: &testing2.Fake{}}
				fakeSkupperClient.Fake.ClearActions()
				fakeSkupperClient.Fake.PrependReactor("create", "sites", func(action testing2.Action) (handled bool, ret runtime.Object, err error) {
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

					if site.Spec.Settings["ingress"] != "none" {
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
				command.options = map[string]string{"ingress": "none"}
			},
		},
		{
			name: "run fails",
			setUpMock: func(command *CmdSiteCreate) {
				fakeSkupperClient := &fake.FakeSkupperV1alpha1{Fake: &testing2.Fake{}}
				fakeSkupperClient.Fake.ClearActions()
				fakeSkupperClient.Fake.PrependReactor("create", "sites", func(action testing2.Action) (handled bool, ret runtime.Object, err error) {
					return true, nil, fmt.Errorf("error")
				})
				command.Client = fakeSkupperClient
				command.siteName = "my-site"
				command.options = map[string]string{"ingress": "none"}
			},
			errorMessage: "error",
		},
	}

	for _, test := range testTable {
		cmd := newCmdSiteCreateWithMocks()
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

func TestCmdSiteCreate_WaitUntilReady(t *testing.T) {
	type test struct {
		name        string
		setUpMock   func(command *CmdSiteCreate)
		expectError bool
	}

	testTable := []test{
		{
			name: "site is not ready",
			setUpMock: func(command *CmdSiteCreate) {
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
			setUpMock: func(command *CmdSiteCreate) {
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
			name: "site is ready but network status configmap is empty",
			setUpMock: func(command *CmdSiteCreate) {
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
								StatusMessage: "OK",
							},
						},
					}, nil
				})
				command.Client = fakeSkupperClient

			},
			expectError: false,
		},
		{
			name: "site is ready but network status configmap is not returned",
			setUpMock: func(command *CmdSiteCreate) {
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
								StatusMessage: "OK",
							},
						},
					}, nil
				})
				command.Client = fakeSkupperClient
			},
			expectError: false,
		},
		{
			name: "site and network status configmap are ready",
			setUpMock: func(command *CmdSiteCreate) {
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
								StatusMessage: "OK",
							},
						},
					}, nil
				})
				command.Client = fakeSkupperClient
			},
			expectError: false,
		},
	}

	for _, test := range testTable {
		cmd := newCmdSiteCreateWithMocks()
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

func newCmdSiteCreateWithMocks() *CmdSiteCreate {

	cmdSiteCreate := &CmdSiteCreate{
		Client:    &fake.FakeSkupperV1alpha1{Fake: &testing2.Fake{}},
		Namespace: "test",
	}

	return cmdSiteCreate
}

func errorsToMessages(errs []error) []string {
	msgs := make([]string, len(errs))
	for i, err := range errs {
		msgs[i] = err.Error()
	}
	return msgs
}

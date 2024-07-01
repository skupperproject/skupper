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
	kubefake "k8s.io/client-go/kubernetes/fake"
	testing2 "k8s.io/client-go/testing"
	"testing"
)

func TestCmdLinkCreate_NewCmdLinkCreate(t *testing.T) {

	t.Run("create command", func(t *testing.T) {

		result := NewCmdLinkCreate()

		assert.Check(t, result.CobraCmd.Use != "")
		assert.Check(t, result.CobraCmd.Short != "")
		assert.Check(t, result.CobraCmd.Long != "")
		assert.Check(t, result.CobraCmd.PreRun != nil)
		assert.Check(t, result.CobraCmd.Run != nil)
		assert.Check(t, result.CobraCmd.PostRunE != nil)
		assert.Check(t, result.CobraCmd.Flags() != nil)

	})

}

func TestCmdLinkCreate_AddFlags(t *testing.T) {

	expectedFlagsWithDefaultValue := map[string]interface{}{
		"cost":       "1",
		"tls-secret": "",
		"output":     "",
	}
	var flagList []string

	cmd := newCmdLinkCreateWithMocks()

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

func TestCmdLinkCreate_ValidateInput(t *testing.T) {
	type test struct {
		name           string
		args           []string
		setUpMock      func(command *CmdLinkCreate)
		expectedErrors []string
	}

	testTable := []test{
		{
			name: "link is not created because there is no site in the namespace.",
			args: []string{"my-new-link"},
			setUpMock: func(command *CmdLinkCreate) {
				fakeSkupperClient := &fake.FakeSkupperV1alpha1{Fake: &testing2.Fake{}}
				fakeSkupperClient.Fake.ClearActions()
				fakeSkupperClient.Fake.PrependReactor("list", "sites", func(action testing2.Action) (handled bool, ret runtime.Object, err error) {
					return true, &v1alpha1.SiteList{}, nil
				})
				command.Client = fakeSkupperClient
				command.flags = CreateLinkFlags{cost: "1"}
			},
			expectedErrors: []string{"there is no skupper site in this namespace"},
		},
		{
			name: "link name is not valid.",
			args: []string{"my new site"},
			setUpMock: func(command *CmdLinkCreate) {
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
				command.Client = fakeSkupperClient
				command.flags = CreateLinkFlags{cost: "1"}
			},
			expectedErrors: []string{"link name is not valid: value does not match this regular expression: ^[a-z0-9]([-a-z0-9]*[a-z0-9])*(\\.[a-z0-9]([-a-z0-9]*[a-z0-9])*)*$"},
		},
		{
			name: "link name is not specified.",
			args: []string{},
			setUpMock: func(command *CmdLinkCreate) {
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
				command.Client = fakeSkupperClient
				command.flags = CreateLinkFlags{cost: "1"}
			},
			expectedErrors: []string{"link name must not be empty"},
		},
		{
			name: "more than one argument was specified",
			args: []string{"my", "link"},
			setUpMock: func(command *CmdLinkCreate) {
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
				command.Client = fakeSkupperClient
				command.flags = CreateLinkFlags{cost: "1"}
			},
			expectedErrors: []string{"only one argument is allowed for this command."},
		},
		{
			name: "cost is not valid.",
			args: []string{"link"},
			setUpMock: func(command *CmdLinkCreate) {
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
				command.Client = fakeSkupperClient
				command.flags = CreateLinkFlags{cost: "one"}
			},
			expectedErrors: []string{"link cost is not valid: strconv.Atoi: parsing \"one\": invalid syntax"},
		},
		{
			name: "cost is not positive",
			args: []string{"link"},
			setUpMock: func(command *CmdLinkCreate) {
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
				command.Client = fakeSkupperClient
				command.flags = CreateLinkFlags{cost: "-4"}
			},
			expectedErrors: []string{
				"link cost is not valid: value is not positive",
			},
		},
		{
			name: "output format is not valid",
			args: []string{"my-site"},
			setUpMock: func(command *CmdLinkCreate) {
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
				command.Client = fakeSkupperClient
				command.flags = CreateLinkFlags{cost: "1", output: "not-valid"}
			},
			expectedErrors: []string{
				"output type is not valid: value not-valid not allowed. It should be one of this options: [json yaml]",
			},
		},
		{
			name: "tls secret not available",
			args: []string{"my-site"},
			setUpMock: func(command *CmdLinkCreate) {
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
				command.Client = fakeSkupperClient
				fakeKubeClient := kubefake.NewSimpleClientset()
				fakeKubeClient.Fake.ClearActions()
				command.KubeClient = fakeKubeClient
				command.flags = CreateLinkFlags{cost: "1", tlsSecret: "secret"}
			},
			expectedErrors: []string{
				"the TLS secret \"secret\" is not available in the namespace: secrets \"secret\" not found",
			},
		},
	}

	for _, test := range testTable {
		t.Run(test.name, func(t *testing.T) {

			command := &CmdLinkCreate{
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

func TestCmdLinkCreate_InputToOptions(t *testing.T) {

	type test struct {
		name              string
		args              []string
		flags             CreateLinkFlags
		expectedTlsSecret string
		expectedCost      int
		expectedOutput    string
	}

	testTable := []test{
		{
			name:              "check options",
			args:              []string{"my-link"},
			flags:             CreateLinkFlags{"secret", "1", "json"},
			expectedCost:      1,
			expectedTlsSecret: "secret",
			expectedOutput:    "json",
		},
	}

	for _, test := range testTable {
		t.Run(test.name, func(t *testing.T) {

			cmd := newCmdLinkCreateWithMocks()
			cmd.flags = test.flags

			cmd.InputToOptions()

			assert.Check(t, cmd.output == test.expectedOutput)
			assert.Check(t, cmd.tlsSecret == test.expectedTlsSecret)
			assert.Check(t, cmd.cost == test.expectedCost)
		})
	}
}

func TestCmdLinkCreate_Run(t *testing.T) {
	type test struct {
		name         string
		setUpMock    func(command *CmdLinkCreate)
		errorMessage string
	}

	testTable := []test{
		{
			name: "runs ok",
			setUpMock: func(command *CmdLinkCreate) {
				fakeSkupperClient := &fake.FakeSkupperV1alpha1{Fake: &testing2.Fake{}}
				fakeSkupperClient.Fake.ClearActions()
				fakeSkupperClient.Fake.PrependReactor("create", "links", func(action testing2.Action) (handled bool, ret runtime.Object, err error) {
					createAction, ok := action.(testing2.CreateActionImpl)
					if !ok {
						return
					}
					createdObject := createAction.GetObject()

					link, ok := createdObject.(*v1alpha1.Link)
					if !ok {
						return
					}

					if link.Name != "my-link" {
						return true, nil, fmt.Errorf("unexpected value as link name")
					}

					if link.Spec.TlsCredentials != "secret" {
						return true, nil, fmt.Errorf("unexpected value as tls credentials")
					}

					if link.Spec.Cost != 1 {
						return true, nil, fmt.Errorf("unexpected value as cost")
					}

					return true, nil, nil
				})
				command.Client = fakeSkupperClient
				command.linkName = "my-link"
				command.cost = 1
				command.tlsSecret = "secret"
			},
		},
		{
			name: "run fails",
			setUpMock: func(command *CmdLinkCreate) {
				fakeSkupperClient := &fake.FakeSkupperV1alpha1{Fake: &testing2.Fake{}}
				fakeSkupperClient.Fake.ClearActions()
				fakeSkupperClient.Fake.PrependReactor("create", "links", func(action testing2.Action) (handled bool, ret runtime.Object, err error) {
					return true, nil, fmt.Errorf("error")
				})
				command.Client = fakeSkupperClient
				command.linkName = "my-site"
			},
			errorMessage: "error",
		},
		{
			name: "runs ok without creating site",
			setUpMock: func(command *CmdLinkCreate) {
				fakeSkupperClient := &fake.FakeSkupperV1alpha1{Fake: &testing2.Fake{}}
				fakeSkupperClient.Fake.ClearActions()
				command.Client = fakeSkupperClient
				command.linkName = "test"
				command.output = "yaml"
			},
		},
		{
			name: "runs fails because the output format is not supported",
			setUpMock: func(command *CmdLinkCreate) {
				fakeSkupperClient := &fake.FakeSkupperV1alpha1{Fake: &testing2.Fake{}}
				fakeSkupperClient.Fake.ClearActions()
				command.Client = fakeSkupperClient
				command.linkName = "my-site"
				command.output = "unsupported"
			},
			errorMessage: "format unsupported not supported",
		},
	}

	for _, test := range testTable {
		cmd := newCmdLinkCreateWithMocks()
		test.setUpMock(cmd)

		t.Run(test.name, func(t *testing.T) {

			err := cmd.Run()
			if err != nil {
				assert.Check(t, test.errorMessage == err.Error(), err.Error())
			} else {
				assert.Check(t, err == nil)
			}
		})
	}
}

func TestCmdLinkCreate_WaitUntilReady(t *testing.T) {
	type test struct {
		name        string
		setUpMock   func(command *CmdLinkCreate)
		expectError bool
	}

	testTable := []test{
		{
			name: "link is not ready",
			setUpMock: func(command *CmdLinkCreate) {
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
			name: "link is not returned",
			setUpMock: func(command *CmdLinkCreate) {
				fakeSkupperClient := &fake.FakeSkupperV1alpha1{Fake: &testing2.Fake{}}
				fakeSkupperClient.Fake.ClearActions()
				fakeSkupperClient.Fake.PrependReactor("get", "links", func(action testing2.Action) (handled bool, ret runtime.Object, err error) {
					return true, nil, fmt.Errorf("it failed")
				})
				command.Client = fakeSkupperClient
			},
			expectError: true,
		},
		{
			name: "there is no need to wait for a link, the user just wanted the output",
			setUpMock: func(command *CmdLinkCreate) {
				fakeSkupperClient := &fake.FakeSkupperV1alpha1{Fake: &testing2.Fake{}}
				fakeSkupperClient.Fake.ClearActions()
				command.Client = fakeSkupperClient
				command.output = "json"
			},
			expectError: false,
		},
		{
			name: "link is ready",
			setUpMock: func(command *CmdLinkCreate) {
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
		cmd := newCmdLinkCreateWithMocks()
		cmd.linkName = "my-link"
		test.setUpMock(cmd)
		t.Run(test.name, func(t *testing.T) {

			err := cmd.WaitUntilReady()

			if test.expectError {
				assert.Check(t, err != nil)
			} else {
				assert.Check(t, err == nil)
			}

		})
	}
}

// --- helper methods

func newCmdLinkCreateWithMocks() *CmdLinkCreate {

	CmdLinkCreate := &CmdLinkCreate{
		Client:     &fake.FakeSkupperV1alpha1{Fake: &testing2.Fake{}},
		KubeClient: kubefake.NewSimpleClientset(),
		Namespace:  "test",
	}

	return CmdLinkCreate
}

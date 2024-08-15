package kube

import (
	"fmt"

	"github.com/skupperproject/skupper/internal/cmd/skupper/utils"

	"testing"

	fakeclient "github.com/skupperproject/skupper/internal/kube/client/fake"

	"github.com/skupperproject/skupper/pkg/apis/skupper/v1alpha1"
	"github.com/spf13/pflag"
	"gotest.tools/assert"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
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
		"service-account":    "",
		"output":             "",
	}
	var flagList []string

	cmd, err := newCmdSiteCreateWithMocks("test")
	assert.Assert(t, err)

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

func TestCmdSiteCreate_ValidateInput(t *testing.T) {
	type test struct {
		name           string
		args           []string
		k8sObjects     []runtime.Object
		skupperObjects []runtime.Object
		flags          *CreateFlags
		expectedErrors []string
	}

	testTable := []test{
		{
			name:       "site is not created because there is already a site in the namespace.",
			args:       []string{"my-new-site"},
			k8sObjects: nil,
			skupperObjects: []runtime.Object{
				&v1alpha1.SiteList{
					Items: []v1alpha1.Site{
						{
							ObjectMeta: v1.ObjectMeta{
								Name:      "old-site",
								Namespace: "test",
							},
						},
					},
				},
			},
			expectedErrors: []string{"there is already a site created for this namespace"},
		},
		{
			name:           "site name is not valid.",
			args:           []string{"my new site"},
			expectedErrors: []string{"site name is not valid: value does not match this regular expression: ^[a-z0-9]([-a-z0-9]*[a-z0-9])*(\\.[a-z0-9]([-a-z0-9]*[a-z0-9])*)*$"},
		},
		{
			name:           "site name is not specified.",
			args:           []string{},
			expectedErrors: []string{"site name must not be empty"},
		},
		{
			name:           "more than one argument was specified",
			args:           []string{"my", "site"},
			expectedErrors: []string{"only one argument is allowed for this command."},
		},
		{
			name:           "service account name is not valid.",
			args:           []string{"my-site"},
			flags:          &CreateFlags{serviceAccount: "not valid service account name"},
			expectedErrors: []string{"service account name is not valid: serviceaccounts \"not valid service account name\" not found"},
		},
		{
			name:  "link access type is not valid",
			args:  []string{"my-site"},
			flags: &CreateFlags{linkAccessType: "not-valid"},
			expectedErrors: []string{
				"link access type is not valid: value not-valid not allowed. It should be one of this options: [route loadbalancer default]",
				"for the site to work with this type of linkAccess, the --enable-link-access option must be set to true",
			},
		},
		{
			name:  "output format is not valid",
			args:  []string{"my-site"},
			flags: &CreateFlags{output: "not-valid"},
			expectedErrors: []string{
				"output type is not valid: value not-valid not allowed. It should be one of this options: [json yaml]",
			},
		},
	}

	for _, test := range testTable {
		t.Run(test.name, func(t *testing.T) {
			command := &CmdSiteCreate{
				Namespace: "test",
			}

			fakeSkupperClient, err := fakeclient.NewFakeClient(command.Namespace, test.k8sObjects, test.skupperObjects, "")
			assert.Assert(t, err)
			command.Client = fakeSkupperClient.GetSkupperClient().SkupperV1alpha1()
			command.KubeClient = fakeSkupperClient.GetKubeClient()

			if test.flags != nil {
				command.flags = *test.flags
			}

			actualErrors := command.ValidateInput(test.args)

			actualErrorsMessages := utils.ErrorsToMessages(actualErrors)

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
		expectedOutput     string
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
			expectedOutput:     "",
		},
		{
			name:  "options with link access enabled but using a type by default and link access host specified",
			args:  []string{"my-site"},
			flags: CreateFlags{enableLinkAccess: true},
			expectedSettings: map[string]string{
				"name": "my-site",
			},
			expectedLinkAccess: "loadbalancer",
			expectedOutput:     "",
		},
		{
			name:  "options with link access enabled using the nodeport type",
			args:  []string{"my-site"},
			flags: CreateFlags{enableLinkAccess: true, linkAccessType: "nodeport"},
			expectedSettings: map[string]string{
				"name": "my-site",
			},
			expectedLinkAccess: "nodeport",
			expectedOutput:     "",
		},
		{
			name:  "options with link access options not well specified",
			args:  []string{"my-site"},
			flags: CreateFlags{enableLinkAccess: false, linkAccessType: "nodeport"},
			expectedSettings: map[string]string{
				"name": "my-site",
			},
			expectedLinkAccess: "none",
			expectedOutput:     "",
		},
		{
			name:  "options output type",
			args:  []string{"my-site"},
			flags: CreateFlags{enableLinkAccess: false, linkAccessType: "nodeport", output: "yaml"},
			expectedSettings: map[string]string{
				"name": "my-site",
			},
			expectedLinkAccess: "none",
			expectedOutput:     "yaml",
		},
	}

	for _, test := range testTable {
		t.Run(test.name, func(t *testing.T) {
			cmd, err := newCmdSiteCreateWithMocks("test")
			assert.Assert(t, err)
			cmd.flags = test.flags
			cmd.siteName = "my-site"

			cmd.InputToOptions()

			assert.DeepEqual(t, cmd.options, test.expectedSettings)

			assert.Check(t, cmd.output == test.expectedOutput)
		})
	}
}

func TestCmdSiteCreate_Run(t *testing.T) {
	type test struct {
		name               string
		k8sObjects         []runtime.Object
		skupperObjects     []runtime.Object
		skupperError       string
		siteName           string
		serviceAccountName string
		options            map[string]string
		output             string
		errorMessage       string
	}

	testTable := []test{
		{
			name:               "runs ok",
			k8sObjects:         nil,
			skupperObjects:     nil,
			siteName:           "my-site",
			serviceAccountName: "my-service-account",
			options:            map[string]string{"name": "my-site"},
			skupperError:       "",
		},
		{
			name:       "runs fails",
			k8sObjects: nil,
			skupperObjects: []runtime.Object{
				&v1alpha1.Site{
					ObjectMeta: v1.ObjectMeta{
						Name:      "my-site",
						Namespace: "test",
					},
					Status: v1alpha1.SiteStatus{
						Status: v1alpha1.Status{
							StatusMessage: "",
						},
					},
				},
			},
			siteName:           "my-site",
			serviceAccountName: "my-service-account",
			options:            map[string]string{"ingress": "none"},
			skupperError:       "",
			errorMessage:       "sites.skupper.io \"my-site\" already exists",
		},
		{
			name:               "runs ok without create site",
			k8sObjects:         nil,
			skupperObjects:     nil,
			siteName:           "test",
			serviceAccountName: "my-service-account",
			options:            map[string]string{"name": "my-site"},
			output:             "yaml",
			skupperError:       "",
		},
		{
			name:               "runs fails because the output format is not supported",
			k8sObjects:         nil,
			skupperObjects:     nil,
			siteName:           "test",
			serviceAccountName: "my-service-account",
			options:            map[string]string{"name": "my-site"},
			output:             "unsupported",
			skupperError:       "",
			errorMessage:       "format unsupported not supported",
		},
	}

	for _, test := range testTable {
		command := &CmdSiteCreate{
			Namespace: "test",
		}

		fakeSkupperClient, err := fakeclient.NewFakeClient(command.Namespace, test.k8sObjects, test.skupperObjects, test.skupperError)
		assert.Assert(t, err)
		command.Client = fakeSkupperClient.GetSkupperClient().SkupperV1alpha1()

		command.siteName = test.siteName
		command.serviceAccountName = test.serviceAccountName
		command.options = test.options
		command.output = test.output

		t.Run(test.name, func(t *testing.T) {

			err := command.Run()
			if err != nil {
				assert.Check(t, test.errorMessage == err.Error())
			} else {
				assert.Check(t, err == nil)
			}
		})
	}
}
func TestCmdSiteCreate_WaitUntil(t *testing.T) {
	type test struct {
		name           string
		k8sObjects     []runtime.Object
		skupperObjects []runtime.Object
		skupperError   string
		expectError    bool
		output         string
	}

	testTable := []test{
		{
			name:       "site is not ready",
			k8sObjects: nil,
			skupperObjects: []runtime.Object{
				&v1alpha1.Site{
					ObjectMeta: v1.ObjectMeta{
						Name:      "my-site",
						Namespace: "test",
					},
					Status: v1alpha1.SiteStatus{
						Status: v1alpha1.Status{
							StatusMessage: "",
						},
					},
				},
			},
			skupperError: "",
			expectError:  true,
		},
		{
			name:         "site is not returned",
			skupperError: "it failed",
			expectError:  true,
		},
		{
			name:       "there is no need to wait for a site, the user just wanted the output",
			k8sObjects: nil,
			skupperObjects: []runtime.Object{
				&v1alpha1.Site{
					ObjectMeta: v1.ObjectMeta{
						Name:      "my-site",
						Namespace: "test",
					},
					Status: v1alpha1.SiteStatus{
						Status: v1alpha1.Status{
							StatusMessage: "OK",
						},
					},
				},
			},
			output:       "yaml",
			skupperError: "",
			expectError:  false,
		},
		{
			name:       "site is ready",
			k8sObjects: nil,
			skupperObjects: []runtime.Object{
				&v1alpha1.Site{
					ObjectMeta: v1.ObjectMeta{
						Name:      "my-site",
						Namespace: "test",
					},
					Status: v1alpha1.SiteStatus{
						Status: v1alpha1.Status{
							Conditions: []v1.Condition{
								{
									Message:            "OK",
									ObservedGeneration: 1,
									Reason:             "OK",
									Status:             "True",
									Type:               "Configured",
								},
							},
						},
					},
				},
			},
			skupperError: "",
			expectError:  false,
		},
	}

	for _, test := range testTable {
		test := test
		command := &CmdSiteCreate{
			Namespace: "test",
		}

		fakeSkupperClient, err := fakeclient.NewFakeClient(command.Namespace, test.k8sObjects, test.skupperObjects, test.skupperError)
		assert.Assert(t, err)
		command.Client = fakeSkupperClient.GetSkupperClient().SkupperV1alpha1()
		command.siteName = "my-site"
		command.output = test.output

		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			err := command.WaitUntil()

			if test.expectError {
				assert.Check(t, err != nil)
			} else {
				assert.Check(t, err == nil, err)
			}

		})
	}
}

// --- helper methods

func newCmdSiteCreateWithMocks(namespace string) (*CmdSiteCreate, error) {

	client, err := fakeclient.NewFakeClient(namespace, nil, nil, "")
	if err != nil {
		return nil, err
	}
	cmdSiteCreate := &CmdSiteCreate{
		Client:     client.GetSkupperClient().SkupperV1alpha1(),
		KubeClient: client.GetKubeClient(),
		Namespace:  namespace,
	}

	return cmdSiteCreate, nil
}

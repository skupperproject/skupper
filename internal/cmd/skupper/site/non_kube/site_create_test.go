package non_kube

import (
	"fmt"

	"github.com/skupperproject/skupper/internal/cmd/skupper/utils"

	"testing"

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

	cmd := CmdSiteCreate{}

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
			name:  "link access type is not valid",
			args:  []string{"my-site"},
			flags: &CreateFlags{LinkAccessType: "not-valid"},
			expectedErrors: []string{
				"link access type is not valid: value not-valid not allowed. It should be one of this options: [route loadbalancer default]",
				"for the site to work with this type of linkAccess, the --enable-link-access option must be set to true",
			},
		},
		{
			name:  "output format is not valid",
			args:  []string{"my-site"},
			flags: &CreateFlags{Output: "not-valid"},
			expectedErrors: []string{
				"output type is not valid: value not-valid not allowed. It should be one of this options: [json yaml]",
			},
		},
	}

	for _, test := range testTable {
		t.Run(test.name, func(t *testing.T) {
			command := &CmdSiteCreate{}

			if test.flags != nil {
				command.Flags = *test.flags
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
			flags: CreateFlags{EnableLinkAccess: true},
			expectedSettings: map[string]string{
				"name": "my-site",
			},
			expectedLinkAccess: "loadbalancer",
			expectedOutput:     "",
		},
		{
			name:  "options with link access enabled using the nodeport type",
			args:  []string{"my-site"},
			flags: CreateFlags{EnableLinkAccess: true, LinkAccessType: "nodeport"},
			expectedSettings: map[string]string{
				"name": "my-site",
			},
			expectedLinkAccess: "nodeport",
			expectedOutput:     "",
		},
		{
			name:  "options with link access options not well specified",
			args:  []string{"my-site"},
			flags: CreateFlags{EnableLinkAccess: false, LinkAccessType: "nodeport"},
			expectedSettings: map[string]string{
				"name": "my-site",
			},
			expectedLinkAccess: "none",
			expectedOutput:     "",
		},
		{
			name:  "options output type",
			args:  []string{"my-site"},
			flags: CreateFlags{EnableLinkAccess: false, LinkAccessType: "nodeport", Output: "yaml"},
			expectedSettings: map[string]string{
				"name": "my-site",
			},
			expectedLinkAccess: "none",
			expectedOutput:     "yaml",
		},
	}

	for _, test := range testTable {
		t.Run(test.name, func(t *testing.T) {
			cmd := CmdSiteCreate{}
			cmd.Flags = test.flags
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
		command := &CmdSiteCreate{}

		command.siteName = test.siteName
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
func TestCmdSiteCreate_WaitUntilReady(t *testing.T) {
	type test struct {
		name           string
		k8sObjects     []runtime.Object
		skupperObjects []runtime.Object
		skupperError   string
		setUpMock      func(command *CmdSiteCreate)
		expectError    bool
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
							StatusMessage: "OK",
						},
					},
				},
			},
			skupperError: "",
			expectError:  false,
		},
	}

	for _, test := range testTable {
		command := &CmdSiteCreate{}

		command.siteName = "my-site"

		t.Run(test.name, func(t *testing.T) {

			err := command.WaitUntilReady()

			if test.expectError {
				assert.Check(t, err != nil)
			} else {
				assert.Check(t, err == nil)
			}

		})
	}
}

package nonkube

import (
	"testing"

	"github.com/skupperproject/skupper/internal/cmd/skupper/common"
	fs2 "github.com/skupperproject/skupper/internal/nonkube/client/fs"
	"github.com/spf13/cobra"

	"gotest.tools/assert"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestNonKubeCmdSiteCreate_ValidateInput(t *testing.T) {
	type test struct {
		name              string
		args              []string
		k8sObjects        []runtime.Object
		skupperObjects    []runtime.Object
		flags             *common.CommandSiteCreateFlags
		cobraGenericFlags map[string]string
		expectedError     string
	}

	testTable := []test{
		{
			name:          "site name is not valid.",
			args:          []string{"my new site"},
			flags:         &common.CommandSiteCreateFlags{BindHost: "bindHost"},
			expectedError: "site name is not valid: value does not match this regular expression: ^[a-z0-9]([-a-z0-9]*[a-z0-9])*(\\.[a-z0-9]([-a-z0-9]*[a-z0-9])*)*$",
		},
		{
			name:          "site name is not specified.",
			args:          []string{},
			flags:         &common.CommandSiteCreateFlags{BindHost: "bindHost"},
			expectedError: "site name must not be empty",
		},
		{
			name:          "more than one argument was specified",
			args:          []string{"my", "site"},
			flags:         &common.CommandSiteCreateFlags{BindHost: "bindHost"},
			expectedError: "only one argument is allowed for this command",
		},
		{
			name:  "link access type is not valid",
			args:  []string{"my-site"},
			flags: &common.CommandSiteCreateFlags{BindHost: "bindHost", LinkAccessType: "not-valid"},
			expectedError: "link access type is not valid: value not-valid not allowed. It should be one of this options: [route loadbalancer default]\n" +
				"for the site to work with this type of linkAccess, the --enable-link-access option must be set to true",
		},
		{
			name:          "output format is not valid",
			args:          []string{"my-site"},
			flags:         &common.CommandSiteCreateFlags{BindHost: "bindHost", Output: "not-valid"},
			expectedError: "output type is not valid: value not-valid not allowed. It should be one of this options: [json yaml]",
		},
		{
			name:          "bindHost was not specified",
			args:          []string{"my-site"},
			flags:         &common.CommandSiteCreateFlags{},
			expectedError: "bind host should not be empty",
		},
		{
			name:          "service-account is not valid on this platform",
			args:          []string{"my-site"},
			flags:         &common.CommandSiteCreateFlags{ServiceAccount: "service-account", BindHost: "bindHost"},
			expectedError: "",
		},
		{
			name:          "kubernetes flags are not valid on this platform",
			args:          []string{"my-site"},
			flags:         &common.CommandSiteCreateFlags{BindHost: "bindHost"},
			expectedError: "",
			cobraGenericFlags: map[string]string{
				common.FlagNameContext:    "test",
				common.FlagNameKubeconfig: "test",
			},
		},
	}

	for _, test := range testTable {
		t.Run(test.name, func(t *testing.T) {
			command := &CmdSiteCreate{Flags: &common.CommandSiteCreateFlags{}}
			command.CobraCmd = &cobra.Command{Use: "test"}

			if test.flags != nil {
				command.Flags = test.flags
			}

			if test.cobraGenericFlags != nil && len(test.cobraGenericFlags) > 0 {
				for name, value := range test.cobraGenericFlags {
					command.CobraCmd.Flags().String(name, value, "")
				}
			}

			actualError := command.ValidateInput(test.args)

			if test.expectedError == "" {
				assert.NilError(t, actualError)
			} else {
				assert.Error(t, actualError, test.expectedError)
			}

		})
	}
}

func TestNonKubeCmdSiteCreate_InputToOptions(t *testing.T) {

	type test struct {
		name                            string
		args                            []string
		namespace                       string
		flags                           common.CommandSiteCreateFlags
		expectedSettings                map[string]string
		expectedLinkAccess              string
		expectedOutput                  string
		expectedNamespace               string
		expectedSubjectAlternativeNames []string
	}

	testTable := []test{
		{
			name:  "options without link access enabled",
			args:  []string{"my-site"},
			flags: common.CommandSiteCreateFlags{},
			expectedSettings: map[string]string{
				"name": "my-site",
			},
			expectedLinkAccess: "none",
			expectedOutput:     "",
			expectedNamespace:  "default",
		},
		{
			name:  "options with link access enabled but using a type by default and link access bindHost specified",
			args:  []string{"my-site"},
			flags: common.CommandSiteCreateFlags{EnableLinkAccess: true},
			expectedSettings: map[string]string{
				"name": "my-site",
			},
			expectedLinkAccess: "loadbalancer",
			expectedOutput:     "",
			expectedNamespace:  "default",
		},
		{
			name:  "options with link access enabled using the nodeport type",
			args:  []string{"my-site"},
			flags: common.CommandSiteCreateFlags{EnableLinkAccess: true, LinkAccessType: "nodeport"},
			expectedSettings: map[string]string{
				"name": "my-site",
			},
			expectedLinkAccess: "nodeport",
			expectedOutput:     "",
			expectedNamespace:  "default",
		},
		{
			name:  "options with link access options not well specified",
			args:  []string{"my-site"},
			flags: common.CommandSiteCreateFlags{EnableLinkAccess: false, LinkAccessType: "nodeport"},
			expectedSettings: map[string]string{
				"name": "my-site",
			},
			expectedLinkAccess: "none",
			expectedOutput:     "",
			expectedNamespace:  "default",
		},
		{
			name:  "options output type",
			args:  []string{"my-site"},
			flags: common.CommandSiteCreateFlags{EnableLinkAccess: false, LinkAccessType: "nodeport", Output: "yaml"},
			expectedSettings: map[string]string{
				"name": "my-site",
			},
			expectedLinkAccess: "none",
			expectedOutput:     "yaml",
			expectedNamespace:  "default",
		},
		{
			name:      "options with specific namespace",
			args:      []string{"my-site"},
			namespace: "test",
			flags:     common.CommandSiteCreateFlags{},
			expectedSettings: map[string]string{
				"name": "my-site",
			},
			expectedNamespace: "test",
		},
		{
			name:      "options with subject alternative names",
			args:      []string{"my-site"},
			namespace: "test",
			flags:     common.CommandSiteCreateFlags{SubjectAlternativeNames: []string{"test", "test2"}},
			expectedSettings: map[string]string{
				"name": "my-site",
			},
			expectedNamespace:               "test",
			expectedSubjectAlternativeNames: []string{"test", "test2"},
		},
	}

	for _, test := range testTable {
		t.Run(test.name, func(t *testing.T) {
			cmd := CmdSiteCreate{}
			cmd.Flags = &test.flags
			cmd.siteName = "my-site"
			cmd.namespace = test.namespace
			cmd.siteHandler = fs2.NewSiteHandler(cmd.namespace)
			cmd.routerAccessHandler = fs2.NewRouterAccessHandler(cmd.namespace)

			cmd.InputToOptions()

			assert.DeepEqual(t, cmd.options, test.expectedSettings)

			assert.Check(t, cmd.output == test.expectedOutput)
			assert.Check(t, cmd.namespace == test.expectedNamespace)
			assert.DeepEqual(t, cmd.subjectAlternativeNames, test.expectedSubjectAlternativeNames)
		})
	}
}

func TestNonKubeCmdSiteCreate_Run(t *testing.T) {
	type test struct {
		name             string
		k8sObjects       []runtime.Object
		skupperObjects   []runtime.Object
		skupperError     string
		siteName         string
		host             string
		options          map[string]string
		output           string
		errorMessage     string
		routerAccessName string
	}

	testTable := []test{
		{
			name:             "runs ok",
			k8sObjects:       nil,
			skupperObjects:   nil,
			siteName:         "my-site",
			host:             "bindHost",
			routerAccessName: "ra-test",
			options:          map[string]string{"name": "my-site"},
		},
		{
			name:           "runs ok without create site",
			k8sObjects:     nil,
			skupperObjects: nil,
			siteName:       "test",
			host:           "bindHost",
			options:        map[string]string{"name": "my-site"},
			output:         "yaml",
			skupperError:   "",
		},
		{
			name:           "runs fails because the output format is not supported",
			k8sObjects:     nil,
			skupperObjects: nil,
			siteName:       "test",
			host:           "bindHost",
			options:        map[string]string{"name": "my-site"},
			output:         "unsupported",
			skupperError:   "",
			errorMessage:   "format unsupported not supported",
		},
	}

	for _, test := range testTable {
		command := &CmdSiteCreate{}

		command.siteName = test.siteName
		command.options = test.options
		command.output = test.output
		command.routerAccessName = test.routerAccessName
		command.siteHandler = fs2.NewSiteHandler(command.namespace)
		command.routerAccessHandler = fs2.NewRouterAccessHandler(command.namespace)

		t.Run(test.name, func(t *testing.T) {

			err := command.Run()
			if err != nil {
				assert.Check(t, test.errorMessage == err.Error(), err.Error())
			} else {
				assert.Check(t, err == nil)
			}
		})
	}
}

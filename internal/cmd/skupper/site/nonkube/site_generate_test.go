package nonkube

import (
	"testing"

	"github.com/skupperproject/skupper/internal/cmd/skupper/common"
	"github.com/skupperproject/skupper/internal/cmd/skupper/common/testutils"
	"github.com/skupperproject/skupper/internal/nonkube/client/fs"
	"github.com/spf13/cobra"

	"gotest.tools/v3/assert"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestNonKubeCmdSiteGenerate_ValidateInput(t *testing.T) {
	type test struct {
		name              string
		args              []string
		k8sObjects        []runtime.Object
		skupperObjects    []runtime.Object
		flags             *common.CommandSiteGenerateFlags
		cobraGenericFlags map[string]string
		expectedError     string
	}

	testTable := []test{
		{
			name:          "site name is not valid.",
			args:          []string{"my new site"},
			flags:         &common.CommandSiteGenerateFlags{BindHost: "bindhost"},
			expectedError: "site name is not valid: value does not match this regular expression: ^[a-z0-9]([-a-z0-9]*[a-z0-9])*(\\.[a-z0-9]([-a-z0-9]*[a-z0-9])*)*$",
		},
		{
			name:          "site name is not specified.",
			args:          []string{},
			flags:         &common.CommandSiteGenerateFlags{},
			expectedError: "site name must not be empty",
		},
		{
			name:          "more than one argument was specified",
			args:          []string{"my", "site"},
			flags:         &common.CommandSiteGenerateFlags{BindHost: "1.2.3.4"},
			expectedError: "only one argument is allowed for this command",
		},
		{
			name:  "bindHost was not specified ok",
			args:  []string{"my-site"},
			flags: &common.CommandSiteGenerateFlags{EnableLinkAccess: true},
		},
		{
			name:          "bindHost was not valid",
			args:          []string{"my-site"},
			flags:         &common.CommandSiteGenerateFlags{EnableLinkAccess: true, BindHost: "not-valid$"},
			expectedError: "bindhost is not valid: a valid IP address or hostname is expected",
		},
		{
			name:          "subjectAlternativeNames was not valid",
			args:          []string{"my-site"},
			flags:         &common.CommandSiteGenerateFlags{EnableLinkAccess: true, BindHost: "bindhost", SubjectAlternativeNames: []string{"not-valid$"}},
			expectedError: "SubjectAlternativeNames is not valid: a valid IP address or hostname is expected",
		},
		{
			name:          "output format is not valid",
			args:          []string{"my-site"},
			flags:         &common.CommandSiteGenerateFlags{BindHost: "bindhost", Output: "not-valid"},
			expectedError: "output type is not valid: value not-valid not allowed. It should be one of this options: [json yaml]",
		},
		{
			name:  "service-account is not valid on this platform",
			args:  []string{"my-site"},
			flags: &common.CommandSiteGenerateFlags{ServiceAccount: "service-account"},
		},
		{
			name:  "kubernetes flags are not valid on this platform",
			args:  []string{"my-site"},
			flags: &common.CommandSiteGenerateFlags{},
			cobraGenericFlags: map[string]string{
				common.FlagNameContext:    "test",
				common.FlagNameKubeconfig: "test",
			},
		},
		{
			name: "flags all valid",
			args: []string{"my-site"},
			flags: &common.CommandSiteGenerateFlags{
				BindHost:                "1.2.3.4",
				EnableLinkAccess:        true,
				SubjectAlternativeNames: []string{"3.3.3.3"},
			},
		},
	}

	for _, test := range testTable {
		t.Run(test.name, func(t *testing.T) {
			command := &CmdSiteGenerate{Flags: &common.CommandSiteGenerateFlags{}}
			command.CobraCmd = &cobra.Command{Use: "test"}

			if test.flags != nil {
				command.Flags = test.flags
			}

			if test.cobraGenericFlags != nil && len(test.cobraGenericFlags) > 0 {
				for name, value := range test.cobraGenericFlags {
					command.CobraCmd.Flags().String(name, value, "")
				}
			}

			testutils.CheckValidateInput(t, command, test.expectedError, test.args)

		})
	}
}

func TestNonKubeCmdSiteGenerate_InputToOptions(t *testing.T) {

	type test struct {
		name                            string
		args                            []string
		namespace                       string
		flags                           common.CommandSiteGenerateFlags
		expectedSettings                map[string]string
		expectedLinkAccess              bool
		expectedOutput                  string
		expectedNamespace               string
		expectedBindHost                string
		expectedSubjectAlternativeNames []string
		expectedRouterAccessName        string
	}

	testTable := []test{
		{
			name:  "options without link access enabled",
			args:  []string{"my-site"},
			flags: common.CommandSiteGenerateFlags{},
			expectedSettings: map[string]string{
				"name": "my-site",
			},
			expectedLinkAccess:       false,
			expectedOutput:           "",
			expectedBindHost:         "",
			expectedRouterAccessName: "",
			expectedNamespace:        "default",
		},
		{
			name:  "options with link access enabled",
			args:  []string{"my-site"},
			flags: common.CommandSiteGenerateFlags{EnableLinkAccess: true, BindHost: "test"},
			expectedSettings: map[string]string{
				"name": "my-site",
			},
			expectedLinkAccess:              true,
			expectedNamespace:               "default",
			expectedBindHost:                "test",
			expectedRouterAccessName:        "router-access-my-site",
			expectedSubjectAlternativeNames: nil,
		},
		{
			name:      "options with subject alternative names",
			args:      []string{"my-site"},
			namespace: "test",
			flags:     common.CommandSiteGenerateFlags{BindHost: "1.2.3.4", SubjectAlternativeNames: []string{"test"}},
			expectedSettings: map[string]string{
				"name": "my-site",
			},
			expectedLinkAccess:              false,
			expectedNamespace:               "test",
			expectedBindHost:                "",
			expectedSubjectAlternativeNames: nil,
			expectedRouterAccessName:        "",
		},
		{
			name:      "options with enable link access and subject alternative names",
			args:      []string{"my-site"},
			namespace: "test",
			flags:     common.CommandSiteGenerateFlags{EnableLinkAccess: true, BindHost: "1.2.3.4", SubjectAlternativeNames: []string{"test"}},
			expectedSettings: map[string]string{
				"name": "my-site",
			},
			expectedLinkAccess:              true,
			expectedNamespace:               "test",
			expectedSubjectAlternativeNames: []string{"test"},
			expectedBindHost:                "1.2.3.4",
			expectedRouterAccessName:        "router-access-my-site",
		},
		{
			name:  "options output type",
			args:  []string{"my-site"},
			flags: common.CommandSiteGenerateFlags{EnableLinkAccess: false, Output: "yaml"},
			expectedSettings: map[string]string{
				"name": "my-site",
			},
			expectedLinkAccess: false,
			expectedOutput:     "yaml",
			expectedNamespace:  "default",
		},
	}

	for _, test := range testTable {
		t.Run(test.name, func(t *testing.T) {
			cmd := CmdSiteGenerate{}
			cmd.Flags = &test.flags
			cmd.siteName = "my-site"
			cmd.namespace = test.namespace
			cmd.siteHandler = fs.NewSiteHandler(cmd.namespace)
			cmd.routerAccessHandler = fs.NewRouterAccessHandler(cmd.namespace)

			cmd.InputToOptions()

			assert.DeepEqual(t, cmd.options, test.expectedSettings)

			assert.Check(t, cmd.output == test.expectedOutput)
			assert.DeepEqual(t, cmd.options, test.expectedSettings)
			assert.Check(t, cmd.namespace == test.expectedNamespace)
			assert.Check(t, cmd.bindHost == test.expectedBindHost)
			assert.Check(t, cmd.linkAccessEnabled == test.expectedLinkAccess)
			assert.Check(t, cmd.routerAccessName == test.expectedRouterAccessName)
			assert.DeepEqual(t, cmd.subjectAlternativeNames, test.expectedSubjectAlternativeNames)
		})
	}
}

func TestNonKubeCmdSiteGenerate_Run(t *testing.T) {
	type test struct {
		name              string
		k8sObjects        []runtime.Object
		skupperObjects    []runtime.Object
		skupperError      string
		siteName          string
		host              string
		options           map[string]string
		output            string
		errorMessage      string
		routerAccessName  string
		linkAccessEnabled bool
	}

	testTable := []test{
		{
			name:              "runs ok",
			k8sObjects:        nil,
			skupperObjects:    nil,
			siteName:          "my-site",
			host:              "bindHost",
			routerAccessName:  "ra-test",
			linkAccessEnabled: true,
			options:           map[string]string{"name": "my-site"},
			output:            "json",
		},
		{
			name:           "runs ok with yaml output",
			k8sObjects:     nil,
			skupperObjects: nil,
			siteName:       "test",
			host:           "bindhost",
			options:        map[string]string{"name": "my-site"},
			output:         "yaml",
			skupperError:   "",
		},
		{
			name:           "runs fails because the output format is not supported",
			k8sObjects:     nil,
			skupperObjects: nil,
			siteName:       "test",
			host:           "bindhost",
			options:        map[string]string{"name": "my-site"},
			output:         "unsupported",
			skupperError:   "",
			errorMessage:   "format unsupported not supported",
		},
	}

	for _, test := range testTable {
		command := &CmdSiteGenerate{}

		command.siteName = test.siteName
		command.options = test.options
		command.output = test.output
		command.routerAccessName = test.routerAccessName
		command.linkAccessEnabled = test.linkAccessEnabled
		command.siteHandler = fs.NewSiteHandler(command.namespace)
		command.routerAccessHandler = fs.NewRouterAccessHandler(command.namespace)
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

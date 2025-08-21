package nonkube

import (
	"testing"

	"github.com/skupperproject/skupper/internal/cmd/skupper/common"
	"github.com/skupperproject/skupper/internal/cmd/skupper/common/testutils"
	"github.com/skupperproject/skupper/internal/nonkube/client/fs"
	"github.com/spf13/cobra"

	"gotest.tools/v3/assert"
)

func TestNonKubeCmdSiteGenerate_ValidateInput(t *testing.T) {
	type test struct {
		name              string
		namespace         string
		args              []string
		flags             *common.CommandSiteGenerateFlags
		cobraGenericFlags map[string]string
		expectedError     string
	}

	testTable := []test{
		{
			name:          "site name is not valid.",
			namespace:     "test",
			args:          []string{"my new site"},
			flags:         &common.CommandSiteGenerateFlags{},
			expectedError: "site name is not valid: value does not match this regular expression: ^[a-z0-9]([-a-z0-9]*[a-z0-9])*(\\.[a-z0-9]([-a-z0-9]*[a-z0-9])*)*$",
		},
		{
			name:          "site name is not specified.",
			namespace:     "test",
			args:          []string{},
			flags:         &common.CommandSiteGenerateFlags{},
			expectedError: "site name must not be empty",
		},
		{
			name:          "more than one argument was specified",
			namespace:     "test",
			args:          []string{"my", "site"},
			flags:         &common.CommandSiteGenerateFlags{},
			expectedError: "only one argument is allowed for this command",
		},
		{
			name:          "output format is not valid",
			namespace:     "test",
			args:          []string{"my-site"},
			flags:         &common.CommandSiteGenerateFlags{Output: "not-valid"},
			expectedError: "output type is not valid: value not-valid not allowed. It should be one of this options: [json yaml]",
		},
		{
			name:      "kubernetes flags are not valid on this platform",
			namespace: "test",
			args:      []string{"my-site"},
			flags:     &common.CommandSiteGenerateFlags{},
			cobraGenericFlags: map[string]string{
				common.FlagNameContext:    "test",
				common.FlagNameKubeconfig: "test",
			},
		},
		{
			name:          "invalid namespace",
			namespace:     "TestInvalid",
			args:          []string{"my-site"},
			flags:         &common.CommandSiteGenerateFlags{},
			expectedError: "namespace is not valid: value does not match this regular expression: ^[a-z0-9]([-a-z0-9]*[a-z0-9])?$",
		},
		{
			name:      "flags all valid",
			namespace: "test",
			args:      []string{"my-site"},
			flags: &common.CommandSiteGenerateFlags{
				EnableLinkAccess: true,
			},
		},
	}

	for _, test := range testTable {
		t.Run(test.name, func(t *testing.T) {
			command := &CmdSiteGenerate{Flags: &common.CommandSiteGenerateFlags{}}
			command.CobraCmd = &cobra.Command{Use: "test"}
			command.namespace = test.namespace
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
		name                     string
		args                     []string
		namespace                string
		flags                    common.CommandSiteGenerateFlags
		expectedLinkAccess       bool
		expectedOutput           string
		expectedNamespace        string
		expectedRouterAccessName string
		expectedSansByDefault    bool
	}

	testTable := []test{
		{
			name:                     "options without link access enabled",
			args:                     []string{"my-site"},
			flags:                    common.CommandSiteGenerateFlags{},
			expectedLinkAccess:       false,
			expectedOutput:           "",
			expectedRouterAccessName: "",
			expectedNamespace:        "default",
			expectedSansByDefault:    false,
		},
		{
			name:                     "options with link access enabled",
			args:                     []string{"my-site"},
			flags:                    common.CommandSiteGenerateFlags{EnableLinkAccess: true},
			expectedLinkAccess:       true,
			expectedNamespace:        "default",
			expectedRouterAccessName: "router-access-my-site",
			expectedSansByDefault:    true,
		},
		{
			name:                  "options output type",
			args:                  []string{"my-site"},
			flags:                 common.CommandSiteGenerateFlags{EnableLinkAccess: false, Output: "yaml"},
			expectedLinkAccess:    false,
			expectedOutput:        "yaml",
			expectedNamespace:     "default",
			expectedSansByDefault: false,
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

			assert.Check(t, cmd.output == test.expectedOutput)
			assert.Check(t, cmd.namespace == test.expectedNamespace)
			assert.Check(t, cmd.linkAccessEnabled == test.expectedLinkAccess)
			assert.Check(t, cmd.routerAccessName == test.expectedRouterAccessName)
			assert.Equal(t, len(cmd.subjectAlternativeNames) > 0, test.expectedSansByDefault)
		})
	}
}

func TestNonKubeCmdSiteGenerate_Run(t *testing.T) {
	type test struct {
		name              string
		siteName          string
		output            string
		errorMessage      string
		routerAccessName  string
		linkAccessEnabled bool
	}

	testTable := []test{
		{
			name:              "runs ok",
			siteName:          "my-site",
			routerAccessName:  "ra-test",
			linkAccessEnabled: true,
			output:            "json",
		},
		{
			name:     "runs ok with yaml output",
			siteName: "test",
			output:   "yaml",
		},
		{
			name:         "runs fails because the output format is not supported",
			siteName:     "test",
			output:       "unsupported",
			errorMessage: "format unsupported not supported",
		},
	}

	for _, test := range testTable {
		command := &CmdSiteGenerate{}

		command.siteName = test.siteName
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

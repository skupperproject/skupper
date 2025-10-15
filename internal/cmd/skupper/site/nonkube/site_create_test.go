package nonkube

import (
	"testing"

	"github.com/skupperproject/skupper/internal/cmd/skupper/common"
	"github.com/skupperproject/skupper/internal/cmd/skupper/common/testutils"
	"github.com/skupperproject/skupper/internal/nonkube/client/fs"
	"github.com/spf13/cobra"
	"gotest.tools/v3/assert"
)

func TestNonKubeCmdSiteCreate_ValidateInput(t *testing.T) {
	type test struct {
		name              string
		namespace         string
		args              []string
		flags             *common.CommandSiteCreateFlags
		cobraGenericFlags map[string]string
		expectedError     string
	}

	testTable := []test{
		{
			name:          "site name is not valid.",
			namespace:     "test1",
			args:          []string{"my new site"},
			flags:         &common.CommandSiteCreateFlags{EnableLinkAccess: true},
			expectedError: "site name is not valid: value does not match this regular expression: ^[a-z0-9]([-a-z0-9]*[a-z0-9])*(\\.[a-z0-9]([-a-z0-9]*[a-z0-9])*)*$",
		},
		{
			name:          "site name is not specified.",
			namespace:     "test2",
			args:          []string{},
			flags:         &common.CommandSiteCreateFlags{},
			expectedError: "site name must not be empty",
		},
		{
			name:          "more than one argument was specified",
			namespace:     "test3",
			args:          []string{"my", "site"},
			flags:         &common.CommandSiteCreateFlags{},
			expectedError: "only one argument is allowed for this command",
		},
		{
			name:          "kubernetes flags are not valid on this platform",
			namespace:     "test4",
			args:          []string{"my-site"},
			flags:         &common.CommandSiteCreateFlags{},
			expectedError: "",
			cobraGenericFlags: map[string]string{
				common.FlagNameContext:    "test",
				common.FlagNameKubeconfig: "test",
			},
		},
		{
			name:          "invalid namespace",
			namespace:     "Test5",
			args:          []string{"my-site"},
			flags:         &common.CommandSiteCreateFlags{},
			expectedError: "namespace is not valid: value does not match this regular expression: ^[a-z0-9]([-a-z0-9]*[a-z0-9])?$",
		},
		{
			name:      "flags all valid",
			namespace: "test6",
			args:      []string{"my-site"},
			flags: &common.CommandSiteCreateFlags{
				EnableLinkAccess: true,
			},
			expectedError: "",
		},
	}

	for _, test := range testTable {
		t.Run(test.name, func(t *testing.T) {
			command := &CmdSiteCreate{Flags: &common.CommandSiteCreateFlags{}}
			command.CobraCmd = &cobra.Command{Use: "test"}
			command.namespace = test.namespace
			command.siteHandler = fs.NewSiteHandler(command.namespace)

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

func TestNonKubeCmdSiteCreate_InputToOptions(t *testing.T) {

	type test struct {
		name                     string
		args                     []string
		namespace                string
		flags                    common.CommandSiteCreateFlags
		expectedLinkAccess       bool
		expectedNamespace        string
		expectedRouterAccessName string
		expectedSansByDefault    bool
	}

	testTable := []test{
		{
			name:                     "options without link access disabled",
			args:                     []string{"my-site"},
			flags:                    common.CommandSiteCreateFlags{},
			expectedLinkAccess:       false,
			expectedNamespace:        "default",
			expectedRouterAccessName: "",
			expectedSansByDefault:    false,
		},
		{
			name:                     "options with link access enabled",
			args:                     []string{"my-site"},
			flags:                    common.CommandSiteCreateFlags{EnableLinkAccess: true},
			expectedLinkAccess:       true,
			expectedNamespace:        "default",
			expectedRouterAccessName: "router-access-my-site",
			expectedSansByDefault:    true,
		},
	}

	for _, test := range testTable {
		t.Run(test.name, func(t *testing.T) {
			cmd := CmdSiteCreate{}
			cmd.Flags = &test.flags
			cmd.siteName = "my-site"
			cmd.namespace = test.namespace
			cmd.siteHandler = fs.NewSiteHandler(cmd.namespace)
			cmd.routerAccessHandler = fs.NewRouterAccessHandler(cmd.namespace)

			cmd.InputToOptions()

			assert.Check(t, cmd.namespace == test.expectedNamespace)
			assert.Check(t, cmd.linkAccessEnabled == test.expectedLinkAccess)
			assert.Check(t, cmd.routerAccessName == test.expectedRouterAccessName)
			assert.Equal(t, len(cmd.subjectAlternativeNames) > 0, test.expectedSansByDefault)
		})
	}
}

func TestNonKubeCmdSiteCreate_Run(t *testing.T) {
	type test struct {
		name              string
		siteName          string
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
		},
		{
			name:     "runs ok without create site",
			siteName: "test",
		},
	}

	for _, test := range testTable {
		command := &CmdSiteCreate{}

		command.siteName = test.siteName
		command.routerAccessName = test.routerAccessName
		command.linkAccessEnabled = test.linkAccessEnabled
		command.siteHandler = fs.NewSiteHandler(command.namespace)
		command.routerAccessHandler = fs.NewRouterAccessHandler(command.namespace)
		defer command.siteHandler.Delete("my-site")
		defer command.routerAccessHandler.Delete("my-site")
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

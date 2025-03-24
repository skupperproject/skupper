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
			flags:         &common.CommandSiteCreateFlags{BindHost: "bindhost", EnableLinkAccess: true},
			expectedError: "site name is not valid: value does not match this regular expression: ^[a-z0-9]([-a-z0-9]*[a-z0-9])*(\\.[a-z0-9]([-a-z0-9]*[a-z0-9])*)*$",
		},
		{
			name:          "site name is not specified.",
			args:          []string{},
			flags:         &common.CommandSiteCreateFlags{BindHost: "bindhost"},
			expectedError: "site name must not be empty",
		},
		{
			name:          "more than one argument was specified",
			args:          []string{"my", "site"},
			flags:         &common.CommandSiteCreateFlags{BindHost: "bindhost"},
			expectedError: "only one argument is allowed for this command",
		},
		{
			name:          "bindHost was not specified ok",
			args:          []string{"my-site"},
			flags:         &common.CommandSiteCreateFlags{EnableLinkAccess: true},
			expectedError: "",
		},
		{
			name:          "bindHost was not valid",
			args:          []string{"my-site"},
			flags:         &common.CommandSiteCreateFlags{EnableLinkAccess: true, BindHost: "not-valid$"},
			expectedError: "bindhost is not valid: a valid IP address or hostname is expected",
		},
		{
			name:          "subjectAlternativeNames was not valid",
			args:          []string{"my-site"},
			flags:         &common.CommandSiteCreateFlags{EnableLinkAccess: true, BindHost: "not-valid", SubjectAlternativeNames: []string{"not-valid$"}},
			expectedError: "SubjectAlternativeNames is not valid: a valid IP address or hostname is expected",
		},
		{
			name:          "kubernetes flags are not valid on this platform",
			args:          []string{"my-site"},
			flags:         &common.CommandSiteCreateFlags{BindHost: "bindhost"},
			expectedError: "",
			cobraGenericFlags: map[string]string{
				common.FlagNameContext:    "test",
				common.FlagNameKubeconfig: "test",
			},
		},
		{
			name: "flags all valid",
			args: []string{"my-site"},
			flags: &common.CommandSiteCreateFlags{
				BindHost:                "1.2.3.4",
				EnableLinkAccess:        true,
				SubjectAlternativeNames: []string{"3.3.3.3"},
			},
			expectedError: "",
		},
	}

	for _, test := range testTable {
		t.Run(test.name, func(t *testing.T) {
			command := &CmdSiteCreate{Flags: &common.CommandSiteCreateFlags{}}
			command.CobraCmd = &cobra.Command{Use: "test"}
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
		name                            string
		args                            []string
		namespace                       string
		flags                           common.CommandSiteCreateFlags
		expectedLinkAccess              bool
		expectedNamespace               string
		expectedSubjectAlternativeNames []string
		expectedBindHost                string
		expectedRouterAccessName        string
	}

	testTable := []test{
		{
			name:                            "options without link access disabled",
			args:                            []string{"my-site"},
			flags:                           common.CommandSiteCreateFlags{BindHost: "test"},
			expectedLinkAccess:              false,
			expectedNamespace:               "default",
			expectedBindHost:                "",
			expectedRouterAccessName:        "",
			expectedSubjectAlternativeNames: nil,
		},
		{
			name:                            "options with link access enabled",
			args:                            []string{"my-site"},
			flags:                           common.CommandSiteCreateFlags{EnableLinkAccess: true, BindHost: "test"},
			expectedLinkAccess:              true,
			expectedNamespace:               "default",
			expectedBindHost:                "test",
			expectedRouterAccessName:        "router-access-my-site",
			expectedSubjectAlternativeNames: nil,
		},
		{
			name:                            "options with subject alternative names",
			args:                            []string{"my-site"},
			namespace:                       "test",
			flags:                           common.CommandSiteCreateFlags{BindHost: "1.2.3.4", SubjectAlternativeNames: []string{"test"}},
			expectedLinkAccess:              false,
			expectedNamespace:               "test",
			expectedBindHost:                "",
			expectedSubjectAlternativeNames: nil,
			expectedRouterAccessName:        "",
		},
		{
			name:                            "options with enable link access and subject alternative names",
			args:                            []string{"my-site"},
			namespace:                       "test",
			flags:                           common.CommandSiteCreateFlags{EnableLinkAccess: true, BindHost: "1.2.3.4", SubjectAlternativeNames: []string{"test"}},
			expectedLinkAccess:              true,
			expectedNamespace:               "test",
			expectedSubjectAlternativeNames: []string{"test"},
			expectedBindHost:                "1.2.3.4",
			expectedRouterAccessName:        "router-access-my-site",
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
			assert.Check(t, cmd.bindHost == test.expectedBindHost)
			assert.Check(t, cmd.linkAccessEnabled == test.expectedLinkAccess)
			assert.Check(t, cmd.routerAccessName == test.expectedRouterAccessName)
			assert.DeepEqual(t, cmd.subjectAlternativeNames, test.expectedSubjectAlternativeNames)
		})
	}
}

func TestNonKubeCmdSiteCreate_Run(t *testing.T) {
	type test struct {
		name              string
		k8sObjects        []runtime.Object
		skupperObjects    []runtime.Object
		skupperError      string
		siteName          string
		host              string
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
			host:              "bindhost",
			routerAccessName:  "ra-test",
			linkAccessEnabled: true,
		},
		{
			name:           "runs ok without create site",
			k8sObjects:     nil,
			skupperObjects: nil,
			siteName:       "test",
			host:           "bindhost",
			skupperError:   "",
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

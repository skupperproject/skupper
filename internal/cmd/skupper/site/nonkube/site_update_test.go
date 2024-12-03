package nonkube

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/skupperproject/skupper/internal/cmd/skupper/common"
	"github.com/skupperproject/skupper/internal/cmd/skupper/common/utils"
	"github.com/skupperproject/skupper/internal/nonkube/client/fs"
	"github.com/skupperproject/skupper/pkg/apis/skupper/v2alpha1"
	"github.com/skupperproject/skupper/pkg/nonkube/api"
	"github.com/spf13/cobra"
	"gotest.tools/v3/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestCmdSiteUpdate_ValidateInput(t *testing.T) {
	type test struct {
		name              string
		args              []string
		flags             *common.CommandSiteUpdateFlags
		k8sObjects        []runtime.Object
		skupperObjects    []runtime.Object
		cobraGenericFlags map[string]string
		expectedErrors    []string
	}

	homeDir, err := os.UserHomeDir()
	assert.Check(t, err == nil)
	path := filepath.Join(homeDir, "/.local/share/skupper/namespaces/test/", string(api.InputSiteStatePath))

	testTable := []test{
		{
			name:           "site is not updated because get site returned error",
			args:           []string{"no-site"},
			flags:          &common.CommandSiteUpdateFlags{},
			expectedErrors: []string{"site no-site must exist to be updated"},
		},
		{
			name:           "site name is not specified",
			args:           []string{},
			flags:          &common.CommandSiteUpdateFlags{},
			expectedErrors: []string{"site name must be configured"},
		},
		{
			name:           "site name is nil",
			args:           []string{""},
			flags:          &common.CommandSiteUpdateFlags{},
			expectedErrors: []string{"site name must not be empty"},
		},
		{
			name:           "more than one argument is specified",
			args:           []string{"my", "site"},
			flags:          &common.CommandSiteUpdateFlags{},
			expectedErrors: []string{"only one argument is allowed for this command"},
		},
		{
			name:           "site name is not valid.",
			args:           []string{"my new site"},
			flags:          &common.CommandSiteUpdateFlags{},
			expectedErrors: []string{"site name is not valid: value does not match this regular expression: ^[a-z0-9]([-a-z0-9]*[a-z0-9])*(\\.[a-z0-9]([-a-z0-9]*[a-z0-9])*)*$"},
		},
		{
			name:           "bind-host is not valid",
			args:           []string{"my-site"},
			flags:          &common.CommandSiteUpdateFlags{BindHost: "not-valid$"},
			expectedErrors: []string{"bindhost is not valid: a valid IP address or hostname is expected"},
		},
		{
			name:           "subjectAlternativeNames are not valid",
			args:           []string{"my-site"},
			flags:          &common.CommandSiteUpdateFlags{SubjectAlternativeNames: []string{"not-valid$"}},
			expectedErrors: []string{"SubjectAlternativeNames are not valid: a valid IP address or hostname is expected"},
		},
		{
			name:           "output is not valid",
			args:           []string{"my-site"},
			flags:          &common.CommandSiteUpdateFlags{Output: "not-supported"},
			expectedErrors: []string{"output type is not valid: value not-supported not allowed. It should be one of this options: [json yaml]"},
		},
		{
			name:  "kubernetes flags are not valid on this platform",
			args:  []string{"my-site"},
			flags: &common.CommandSiteUpdateFlags{},
			cobraGenericFlags: map[string]string{
				common.FlagNameContext:    "test",
				common.FlagNameKubeconfig: "test",
			},
			expectedErrors: []string{},
		},
		{
			name: "flags all valid",
			args: []string{"my-site"},
			flags: &common.CommandSiteUpdateFlags{
				Output:                  "json",
				BindHost:                "1.2.3.4",
				EnableLinkAccess:        true,
				SubjectAlternativeNames: []string{"3.3.3.3"},
			},
			expectedErrors: []string{},
		},
	}

	// Add temp files so site exists for update tests
	siteResource := v2alpha1.Site{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "skupper.io/v2alpha1",
			Kind:       "Site",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-site",
			Namespace: "test",
		},
	}

	routerAccessResource := v2alpha1.RouterAccess{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "skupper.io/v2alpha1",
			Kind:       "RouterAccess",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "router-access-mysite",
			Namespace: "test",
		},
		Spec: v2alpha1.RouterAccessSpec{
			Roles: []v2alpha1.RouterAccessRole{
				{
					Name: "inter-router",
					Port: 55671,
				},
				{
					Name: "edge",
					Port: 45671,
				},
			},
			BindHost:                "1.2.3.4",
			SubjectAlternativeNames: []string{"test", "2.2.2.2"},
		},
	}
	command := &CmdSiteUpdate{Flags: &common.CommandSiteUpdateFlags{}}
	command.CobraCmd = &cobra.Command{Use: "test"}
	command.namespace = "test"
	command.siteHandler = fs.NewSiteHandler(command.namespace)
	command.routerAccessHandler = fs.NewRouterAccessHandler(command.namespace)

	defer command.siteHandler.Delete("my-site")
	defer command.routerAccessHandler.Delete("my-site")

	content, err := command.siteHandler.EncodeToYaml(siteResource)
	assert.Check(t, err == nil)
	err = command.siteHandler.WriteFile(path, "my-site.yaml", content, common.Sites)
	assert.Check(t, err == nil)
	content, err = command.routerAccessHandler.EncodeToYaml(routerAccessResource)
	assert.Check(t, err == nil)
	err = command.routerAccessHandler.WriteFile(path, "router-access-my-site.yaml", content, common.RouterAccesses)
	assert.Check(t, err == nil)

	for _, test := range testTable {
		t.Run(test.name, func(t *testing.T) {
			command.siteName = ""
			if test.flags != nil {
				command.Flags = test.flags
			}

			if test.cobraGenericFlags != nil && len(test.cobraGenericFlags) > 0 {
				for name, value := range test.cobraGenericFlags {
					command.CobraCmd.Flags().String(name, value, "")
				}
			}

			actualErrors := command.ValidateInput(test.args)

			actualErrorsMessages := utils.ErrorsToMessages(actualErrors)

			assert.DeepEqual(t, actualErrorsMessages, test.expectedErrors)

		})
	}
}

func TestNonKubeCmdSiteUpdate_InputToOptions(t *testing.T) {

	type test struct {
		name                            string
		args                            []string
		namespace                       string
		flags                           common.CommandSiteUpdateFlags
		expectedSettings                map[string]string
		expectedLinkAccess              bool
		expectedNamespace               string
		expectedSubjectAlternativeNames []string
		expectedBindHost                string
		expectedRouterAccessName        string
	}

	testTable := []test{
		{
			name:  "options without link access disabled",
			args:  []string{"my-site"},
			flags: common.CommandSiteUpdateFlags{BindHost: "test"},
			expectedSettings: map[string]string{
				"name": "my-site",
			},
			expectedLinkAccess:              false,
			expectedNamespace:               "default",
			expectedBindHost:                "",
			expectedRouterAccessName:        "router-access-my-site",
			expectedSubjectAlternativeNames: nil,
		},
		{
			name:  "options with link access enabled",
			args:  []string{"my-site"},
			flags: common.CommandSiteUpdateFlags{EnableLinkAccess: true, BindHost: "test"},
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
			name:      "options without enable link access and subject alternative names",
			args:      []string{"my-site"},
			namespace: "test",
			flags:     common.CommandSiteUpdateFlags{EnableLinkAccess: false, BindHost: "1.2.3.4", SubjectAlternativeNames: []string{"test"}},
			expectedSettings: map[string]string{
				"name": "my-site",
			},
			expectedLinkAccess:              false,
			expectedNamespace:               "test",
			expectedBindHost:                "",
			expectedSubjectAlternativeNames: nil,
			expectedRouterAccessName:        "router-access-my-site",
		},
		{
			name:      "options with enable link access and subject alternative names",
			args:      []string{"my-site"},
			namespace: "test",
			flags:     common.CommandSiteUpdateFlags{EnableLinkAccess: true, BindHost: "1.2.3.4", SubjectAlternativeNames: []string{"test"}},
			expectedSettings: map[string]string{
				"name": "my-site",
			},
			expectedLinkAccess:              true,
			expectedNamespace:               "test",
			expectedSubjectAlternativeNames: []string{"test"},
			expectedBindHost:                "1.2.3.4",
			expectedRouterAccessName:        "router-access-my-site",
		},
	}

	for _, test := range testTable {
		t.Run(test.name, func(t *testing.T) {
			cmd := &CmdSiteUpdate{Flags: &common.CommandSiteUpdateFlags{EnableLinkAccess: test.flags.EnableLinkAccess}}
			cmd.CobraCmd = &cobra.Command{Use: "test"}
			cmd.Flags = &test.flags
			cmd.newSettings.bindHost = test.flags.BindHost
			cmd.newSettings.subjectAlternativeNames = test.flags.SubjectAlternativeNames
			cmd.siteName = "my-site"
			cmd.namespace = test.namespace
			cmd.linkAccessEnabled = test.flags.EnableLinkAccess

			cmd.InputToOptions()

			assert.DeepEqual(t, cmd.options, test.expectedSettings)
			assert.Check(t, cmd.namespace == test.expectedNamespace)
			assert.Check(t, cmd.bindHost == test.expectedBindHost)
			assert.Check(t, cmd.linkAccessEnabled == test.expectedLinkAccess)
			assert.Check(t, cmd.routerAccessName == test.expectedRouterAccessName)
			assert.DeepEqual(t, cmd.subjectAlternativeNames, test.expectedSubjectAlternativeNames)
		})
	}
}

func TestCmdSiteUpdate_Run(t *testing.T) {
	type test struct {
		name                string
		namespace           string
		k8sObjects          []runtime.Object
		skupperObjects      []runtime.Object
		skupperErrorMessage string
		errorMessage        string
		siteName            string
		flags               common.CommandSiteUpdateFlags
		linkAccessEnabled   bool
	}

	testTable := []test{
		{
			name:      "runs ok link enable",
			namespace: "test",
			siteName:  "my-site",
			flags: common.CommandSiteUpdateFlags{
				BindHost:                "1.2.3.4",
				EnableLinkAccess:        true,
				SubjectAlternativeNames: []string{"2.2.2.2", "test"},
			},
			linkAccessEnabled: true,
		},
		{
			name:      "runs ok",
			namespace: "test",
			siteName:  "my-site",
			flags:     common.CommandSiteUpdateFlags{},
		},
		{
			name:     "run ok output json",
			siteName: "my-site",
			flags: common.CommandSiteUpdateFlags{
				BindHost:                "1.2.3.4",
				EnableLinkAccess:        true,
				SubjectAlternativeNames: []string{"2.2.2.2", "test", "5.6.7.8"},
				Output:                  "json",
			},
			linkAccessEnabled: true,
		},
		{
			name:     "run ok output yaml",
			siteName: "my-site",
			flags: common.CommandSiteUpdateFlags{
				EnableLinkAccess: false,
				Output:           "yaml",
			},
		},
	}

	for _, test := range testTable {
		command := &CmdSiteUpdate{}
		command.CobraCmd = &cobra.Command{Use: "test"}
		command.Flags = &test.flags
		command.siteName = test.siteName
		command.siteHandler = fs.NewSiteHandler(command.namespace)
		command.routerAccessHandler = fs.NewRouterAccessHandler(command.namespace)
		command.newSettings.bindHost = test.flags.BindHost
		command.newSettings.subjectAlternativeNames = test.flags.SubjectAlternativeNames
		command.linkAccessEnabled = test.linkAccessEnabled
		defer command.siteHandler.Delete("my-site")
		defer command.routerAccessHandler.Delete("my-site")
		t.Run(test.name, func(t *testing.T) {
			command.InputToOptions()
			err := command.Run()
			if err != nil {
				assert.Check(t, test.errorMessage == err.Error(), err.Error())
			} else {
				assert.Check(t, err == nil)
			}
		})
	}
}

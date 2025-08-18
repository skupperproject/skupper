package nonkube

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/skupperproject/skupper/internal/cmd/skupper/common"
	"github.com/skupperproject/skupper/internal/cmd/skupper/common/testutils"
	"github.com/skupperproject/skupper/internal/nonkube/client/fs"
	"github.com/skupperproject/skupper/pkg/apis/skupper/v2alpha1"
	"github.com/skupperproject/skupper/pkg/nonkube/api"
	"github.com/spf13/cobra"
	"gotest.tools/v3/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestCmdSiteUpdate_ValidateInput(t *testing.T) {
	type test struct {
		name              string
		namespace         string
		args              []string
		flags             *common.CommandSiteUpdateFlags
		cobraGenericFlags map[string]string
		expectedError     string
	}

	if os.Getuid() == 0 {
		api.DefaultRootDataHome = t.TempDir()
	} else {
		t.Setenv("XDG_DATA_HOME", t.TempDir())
	}
	tmpDir := api.GetDataHome()
	path := filepath.Join(tmpDir, "/namespaces/test4/", string(api.InputSiteStatePath))

	testTable := []test{
		{
			name:          "site is not updated because get site returned error",
			namespace:     "test4",
			args:          []string{"no-site"},
			flags:         &common.CommandSiteUpdateFlags{},
			expectedError: "site no-site must exist to be updated",
		},
		{
			name:          "site name is not specified",
			namespace:     "test4",
			args:          []string{},
			flags:         &common.CommandSiteUpdateFlags{},
			expectedError: "site name must be configured",
		},
		{
			name:          "site name is nil",
			namespace:     "test4",
			args:          []string{""},
			flags:         &common.CommandSiteUpdateFlags{},
			expectedError: "site name must not be empty",
		},
		{
			name:          "more than one argument is specified",
			namespace:     "test4",
			args:          []string{"my", "site"},
			flags:         &common.CommandSiteUpdateFlags{},
			expectedError: "only one argument is allowed for this command",
		},
		{
			name:          "site name is not valid.",
			args:          []string{"my new site"},
			flags:         &common.CommandSiteUpdateFlags{},
			expectedError: "site name is not valid: value does not match this regular expression: ^[a-z0-9]([-a-z0-9]*[a-z0-9])*(\\.[a-z0-9]([-a-z0-9]*[a-z0-9])*)*$",
		},
		{
			name:          "invalid namespace",
			namespace:     "TestInvalid",
			args:          []string{"my-site"},
			flags:         &common.CommandSiteUpdateFlags{},
			expectedError: "namespace is not valid: value does not match this regular expression: ^[a-z0-9]([-a-z0-9]*[a-z0-9])?$",
		},
		{
			name:      "flags all valid",
			namespace: "test4",
			args:      []string{"my-site"},
			flags: &common.CommandSiteUpdateFlags{
				EnableLinkAccess: true,
			},
			expectedError: "",
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
			Namespace: "test4",
		},
	}

	routerAccessResource := v2alpha1.RouterAccess{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "skupper.io/v2alpha1",
			Kind:       "RouterAccess",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "router-access-mysite",
			Namespace: "test4",
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
	command.namespace = "test4"
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

			command.namespace = test.namespace

			if test.cobraGenericFlags != nil && len(test.cobraGenericFlags) > 0 {
				for name, value := range test.cobraGenericFlags {
					command.CobraCmd.Flags().String(name, value, "")
				}
			}

			testutils.CheckValidateInput(t, command, test.expectedError, test.args)
		})
	}
}

func TestNonKubeCmdSiteUpdate_InputToOptions(t *testing.T) {

	type test struct {
		name                     string
		args                     []string
		namespace                string
		flags                    common.CommandSiteUpdateFlags
		expectedLinkAccess       bool
		expectedNamespace        string
		expectedRouterAccessName string
	}

	testTable := []test{
		{
			name:                     "options without link access disabled",
			args:                     []string{"my-site"},
			flags:                    common.CommandSiteUpdateFlags{},
			expectedLinkAccess:       false,
			expectedNamespace:        "default",
			expectedRouterAccessName: "router-access-my-site",
		},
		{
			name:                     "options with link access enabled",
			args:                     []string{"my-site"},
			flags:                    common.CommandSiteUpdateFlags{EnableLinkAccess: true},
			expectedLinkAccess:       true,
			expectedNamespace:        "default",
			expectedRouterAccessName: "router-access-my-site",
		},
		{
			name:                     "options with enable HA",
			args:                     []string{"my-site"},
			namespace:                "test4",
			flags:                    common.CommandSiteUpdateFlags{EnableHA: true},
			expectedNamespace:        "test4",
			expectedRouterAccessName: "router-access-my-site",
		},
	}

	for _, test := range testTable {
		t.Run(test.name, func(t *testing.T) {
			cmd := &CmdSiteUpdate{Flags: &common.CommandSiteUpdateFlags{EnableLinkAccess: test.flags.EnableLinkAccess}}
			cmd.CobraCmd = &cobra.Command{Use: "test"}
			cmd.Flags = &test.flags
			cmd.siteName = "my-site"
			cmd.namespace = test.namespace
			cmd.linkAccessEnabled = test.flags.EnableLinkAccess

			cmd.InputToOptions()

			assert.Check(t, cmd.namespace == test.expectedNamespace)
			assert.Check(t, cmd.linkAccessEnabled == test.expectedLinkAccess)
			assert.Check(t, cmd.routerAccessName == test.expectedRouterAccessName)
		})
	}
}

func TestCmdSiteUpdate_Run(t *testing.T) {
	type test struct {
		name              string
		namespace         string
		errorMessage      string
		siteName          string
		flags             common.CommandSiteUpdateFlags
		linkAccessEnabled bool
	}

	testTable := []test{
		{
			name:      "runs ok link enable",
			namespace: "test4",
			siteName:  "my-site",
			flags: common.CommandSiteUpdateFlags{
				EnableLinkAccess: true,
			},
			linkAccessEnabled: true,
		},
		{
			name:      "runs ok",
			namespace: "test4",
			siteName:  "my-site",
			flags:     common.CommandSiteUpdateFlags{},
		},
		{
			name:     "run ok output json",
			siteName: "my-site",
			flags: common.CommandSiteUpdateFlags{
				EnableLinkAccess: true,
			},
			linkAccessEnabled: true,
		},
		{
			name:     "run ok output yaml",
			siteName: "my-site",
			flags: common.CommandSiteUpdateFlags{
				EnableLinkAccess: false,
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

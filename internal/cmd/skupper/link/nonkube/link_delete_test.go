package nonkube

import (
	"os"
	"path/filepath"
	"strings"
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

func TestCmdLinkDelete_ValidateInput(t *testing.T) {
	type test struct {
		name              string
		namespace         string
		args              []string
		linkHandler       *fs.LinkHandler
		flags             *common.CommandLinkDeleteFlags
		cobraGenericFlags map[string]string
		expectedError     string
	}

	if os.Getuid() == 0 {
		api.DefaultRootDataHome = t.TempDir()
	} else {
		t.Setenv("XDG_DATA_HOME", t.TempDir())
	}
	tmpDir := api.GetDataHome()
	path := filepath.Join(tmpDir, "/namespaces/test/", string(api.InputSiteStatePath))

	testTable := []test{
		{
			name:          "Link name is not specified",
			namespace:     "test",
			args:          []string{},
			flags:         &common.CommandLinkDeleteFlags{},
			expectedError: "link name must be specified",
		},
		{
			name:          "Link name is nil",
			namespace:     "test",
			args:          []string{""},
			flags:         &common.CommandLinkDeleteFlags{},
			expectedError: "link name must not be empty",
		},
		{
			name:          "Link name is not valid",
			namespace:     "test",
			args:          []string{"my name"},
			flags:         &common.CommandLinkDeleteFlags{},
			expectedError: "link name is not valid: value does not match this regular expression: ^[a-z0-9]([-a-z0-9]*[a-z0-9])*(\\.[a-z0-9]([-a-z0-9]*[a-z0-9])*)*$",
		},
		{
			name:          "more than one argument is specified",
			args:          []string{"my", "Link"},
			flags:         &common.CommandLinkDeleteFlags{},
			expectedError: "only one argument is allowed for this command",
		},
		{
			name:          "Link doesn't exist",
			args:          []string{"no-exist"},
			flags:         &common.CommandLinkDeleteFlags{},
			expectedError: "There is no link resource in the namespace with the name \"no-exist\"",
		},
		{
			name:          "kubernetes flags are not valid on this platform",
			args:          []string{"mine"},
			flags:         &common.CommandLinkDeleteFlags{},
			expectedError: "",
			cobraGenericFlags: map[string]string{
				common.FlagNameContext:    "test",
				common.FlagNameKubeconfig: "test",
			},
		},
		{
			name:          "invalid namespace",
			namespace:     "TestInvalid",
			args:          []string{"link-mine"},
			flags:         &common.CommandLinkDeleteFlags{},
			expectedError: "namespace is not valid: value does not match this regular expression: ^[a-z0-9]([-a-z0-9]*[a-z0-9])?$\nThere is no link resource in the namespace with the name \"link-mine\"",
		},
	}

	// Add temp files so Link exists for update tests
	LinkResource := v2alpha1.Link{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "skupper.io/v2alpha1",
			Kind:       "Link",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "link-mine",
			Namespace: "test",
		},
	}
	command := &CmdLinkDelete{Flags: &common.CommandLinkDeleteFlags{}}
	command.CobraCmd = &cobra.Command{Use: "test"}
	command.namespace = "test"
	command.linkHandler = fs.NewLinkHandler(command.namespace)

	defer command.linkHandler.Delete("mine")
	content, err := command.linkHandler.EncodeToYaml(LinkResource)
	assert.Check(t, err == nil)
	err = command.linkHandler.WriteFile(path, "mine.yaml", content, common.Links)
	assert.Check(t, err == nil)

	for _, test := range testTable {
		t.Run(test.name, func(t *testing.T) {
			command.linkName = ""
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

func TestCmdLinkDelete_Run(t *testing.T) {
	type test struct {
		name              string
		namespace         string
		deleteName        string
		linkHandler       *fs.LinkHandler
		errorMessage      string
		expectedNamespace string
	}

	testTable := []test{
		{
			name:              "run default",
			deleteName:        "no-exist",
			errorMessage:      "no such file or directory",
			expectedNamespace: "default",
		},
		{
			name:              "run namespace",
			namespace:         "test2",
			deleteName:        "router-east",
			errorMessage:      "",
			expectedNamespace: "test2",
		},
	}

	// Add a temp file so Link exists for delete tests
	LinkResource := v2alpha1.Link{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "skupper.io/v2alpha1",
			Kind:       "Link",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "router-east",
			Namespace: "test2",
		},
	}

	if os.Getuid() == 0 {
		api.DefaultRootDataHome = t.TempDir()
	} else {
		t.Setenv("XDG_DATA_HOME", t.TempDir())
	}
	tmpDir := api.GetDataHome()
	path := filepath.Join(tmpDir, "/namespaces/test2/", string(api.InputSiteStatePath))

	command := &CmdLinkDelete{Flags: &common.CommandLinkDeleteFlags{}}
	command.namespace = "test2"
	command.linkHandler = fs.NewLinkHandler(command.namespace)

	content, err := command.linkHandler.EncodeToYaml(LinkResource)
	assert.Check(t, err == nil)
	err = command.linkHandler.WriteFile(path, "router-east.yaml", content, common.Links)
	assert.Check(t, err == nil)
	defer command.linkHandler.Delete("router-east.yaml")

	for _, test := range testTable {

		t.Run(test.name, func(t *testing.T) {
			command.namespace = test.namespace
			command.linkName = test.deleteName
			command.InputToOptions()

			err := command.Run()
			if err != nil {
				assert.Check(t, strings.HasSuffix(err.Error(), test.errorMessage))
			} else {
				assert.Check(t, err == nil)
				assert.Equal(t, command.namespace, test.expectedNamespace)
				// only deleting from input/resources directory
				// expect specified link is deleted
				opts := fs.GetOptions{RuntimeFirst: false, LogWarning: false}
				Link, _ := command.linkHandler.Get(command.linkName, opts)
				assert.Check(t, Link == nil)
			}
		})
	}
}

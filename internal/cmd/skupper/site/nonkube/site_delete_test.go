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

func TestCmdSiteDelete_ValidateInput(t *testing.T) {
	type test struct {
		name              string
		args              []string
		flags             *common.CommandSiteDeleteFlags
		cobraGenericFlags map[string]string
		k8sObjects        []runtime.Object
		skupperObjects    []runtime.Object
		expectedErrors    []string
	}

	homeDir, err := os.UserHomeDir()
	assert.Check(t, err == nil)
	path := filepath.Join(homeDir, "/.local/share/skupper/namespaces/test/", string(api.InputSiteStatePath))

	testTable := []test{
		{
			name:           "site name is not specified",
			args:           []string{},
			flags:          &common.CommandSiteDeleteFlags{},
			expectedErrors: []string{"site name must be specified"},
		},
		{
			name:           "site name is nil",
			args:           []string{""},
			flags:          &common.CommandSiteDeleteFlags{},
			expectedErrors: []string{"site name must not be empty"},
		},
		{
			name:           "site name is not valid",
			args:           []string{"my name"},
			flags:          &common.CommandSiteDeleteFlags{},
			expectedErrors: []string{"site name is not valid: value does not match this regular expression: ^[a-z0-9]([-a-z0-9]*[a-z0-9])*(\\.[a-z0-9]([-a-z0-9]*[a-z0-9])*)*$"},
		},
		{
			name:           "more than one argument is specified",
			args:           []string{"my", "site"},
			flags:          &common.CommandSiteDeleteFlags{},
			expectedErrors: []string{"only one argument is allowed for this command"},
		},
		{
			name:           "site doesn't exist",
			args:           []string{"no-site"},
			flags:          &common.CommandSiteDeleteFlags{},
			expectedErrors: []string{"site no-site does not exist"},
		},
		{
			name:           "kubernetes flags are not valid on this platform",
			args:           []string{"my-site"},
			flags:          &common.CommandSiteDeleteFlags{},
			expectedErrors: []string{},
			cobraGenericFlags: map[string]string{
				common.FlagNameContext:    "test",
				common.FlagNameKubeconfig: "test",
			},
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
	command := &CmdSiteDelete{Flags: &common.CommandSiteDeleteFlags{}}
	command.CobraCmd = &cobra.Command{Use: "test"}
	command.namespace = "test"
	command.siteHandler = fs.NewSiteHandler(command.namespace)
	command.routerAccessHandler = fs.NewRouterAccessHandler(command.namespace)

	defer command.siteHandler.Delete("my-site")
	content, err := command.siteHandler.EncodeToYaml(siteResource)
	assert.Check(t, err == nil)
	err = command.siteHandler.WriteFile(path, "my-site.yaml", content, common.Sites)
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

func TestCmdSiteDelete_Run(t *testing.T) {
	type test struct {
		name                string
		namespace           string
		deleteName          string
		k8sObjects          []runtime.Object
		skupperObjects      []runtime.Object
		skupperErrorMessage string
		errorMessage        string
	}

	testTable := []test{
		{
			name:         "run fails default",
			deleteName:   "my-site",
			errorMessage: "error",
		},
		{
			name:         "run fails",
			namespace:    "test",
			deleteName:   "my-site",
			errorMessage: "error",
		},
	}

	for _, test := range testTable {
		cmd := &CmdSiteDelete{}

		t.Run(test.name, func(t *testing.T) {

			cmd.siteName = test.deleteName
			cmd.namespace = test.namespace
			cmd.siteHandler = fs.NewSiteHandler(cmd.namespace)
			cmd.routerAccessHandler = fs.NewRouterAccessHandler(cmd.namespace)
			cmd.InputToOptions()

			err := cmd.Run()
			if err != nil {
				assert.Check(t, test.errorMessage == err.Error())
			} else {
				assert.Check(t, err == nil)
			}
		})
	}
}

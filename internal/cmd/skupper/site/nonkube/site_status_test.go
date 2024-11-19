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
	"gotest.tools/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestCmdSiteStatus_ValidateInput(t *testing.T) {
	type test struct {
		name              string
		args              []string
		flags             *common.CommandSiteStatusFlags
		cobraGenericFlags map[string]string
		k8sObjects        []runtime.Object
		skupperObjects    []runtime.Object
		expectedErrors    []string
	}

	homeDir, err := os.UserHomeDir()
	assert.Check(t, err == nil)
	path := filepath.Join(homeDir, "/.local/share/skupper/namespaces/test/", string(api.RuntimeSiteStatePath))

	testTable := []test{
		{
			name:           "site does not exist in the namespace",
			args:           []string{"no-site"},
			expectedErrors: []string{"site no-site does not exist"},
		},
		{
			name:           "site name is nil",
			args:           []string{""},
			expectedErrors: []string{"site name must not be empty"},
		},
		{
			name:           "more than one argument is specified",
			args:           []string{"my", "site"},
			expectedErrors: []string{"only one argument is allowed for this command"},
		},
		{
			name:           "site name is not valid.",
			args:           []string{"my new site"},
			expectedErrors: []string{"site name is not valid: value does not match this regular expression: ^[a-z0-9]([-a-z0-9]*[a-z0-9])*(\\.[a-z0-9]([-a-z0-9]*[a-z0-9])*)*$"},
		},
		{
			name:           "no args",
			expectedErrors: []string{},
		},
		{
			name:           "bad output",
			args:           []string{"my-site"},
			flags:          &common.CommandSiteStatusFlags{Output: "yaml$"},
			expectedErrors: []string{"output type is not valid: value yaml$ not allowed. It should be one of this options: [json yaml]"},
		},
		{
			name:           "good flags",
			args:           []string{"my-site"},
			flags:          &common.CommandSiteStatusFlags{Output: "yaml"},
			expectedErrors: []string{},
		},
	}

	//Add a temp file so site exists for status tests
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

	command := &CmdSiteStatus{}
	command.namespace = "test"
	command.siteHandler = fs.NewSiteHandler(command.namespace)

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

func TestCmdSiteStatus_Run(t *testing.T) {
	type test struct {
		name                string
		siteName            string
		k8sObjects          []runtime.Object
		skupperObjects      []runtime.Object
		skupperErrorMessage string
		errorMessage        string
		output              string
	}

	homeDir, err := os.UserHomeDir()
	assert.Check(t, err == nil)
	path := filepath.Join(homeDir, "/.local/share/skupper/namespaces/test/", string(api.InputSiteStatePath))
	testTable := []test{
		{
			name:         "run fails site doesn't exist",
			siteName:     "no-site",
			errorMessage: "failed to read file: open " + path + "/sites/no-site.yaml: no such file or directory",
		},
		{
			name:     "runs ok, returns 1 site",
			siteName: "my-site",
		},
		{
			name: "runs ok, returns 1 site",
		},
		{
			name:     "runs ok, returns 1 site yaml",
			siteName: "my-site",
			output:   "yaml",
		},
	}

	//Add a temp file so site exists for status tests
	siteResource1 := v2alpha1.Site{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "skupper.io/v2alpha1",
			Kind:       "Site",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-site",
			Namespace: "test",
		},
		Spec: v2alpha1.SiteSpec{
			LinkAccess: "route",
			Settings: map[string]string{
				"name": "my-site",
			},
		},
		Status: v2alpha1.SiteStatus{
			Status: v2alpha1.Status{
				Conditions: []metav1.Condition{
					{
						Type:   "Configured",
						Status: "True",
					},
				},
			},
		},
	}

	// add site in runtime directory
	command := &CmdSiteStatus{}
	command.namespace = "test"
	command.siteHandler = fs.NewSiteHandler(command.namespace)
	content, err := command.siteHandler.EncodeToYaml(siteResource1)
	assert.Check(t, err == nil)
	path = filepath.Join(homeDir, "/.local/share/skupper/namespaces/test/", string(api.RuntimeSiteStatePath))
	err = command.siteHandler.WriteFile(path, "my-site.yaml", content, common.Sites)
	assert.Check(t, err == nil)
	defer command.siteHandler.Delete("my-site")

	for _, test := range testTable {
		command.siteName = test.siteName
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

func TestCmdSiteStatus_RunNoDirectory(t *testing.T) {
	type test struct {
		name                string
		siteName            string
		k8sObjects          []runtime.Object
		skupperObjects      []runtime.Object
		skupperErrorMessage string
		errorMessage        string
	}

	homeDir, err := os.UserHomeDir()
	assert.Check(t, err == nil)
	path := filepath.Join(homeDir, "/.local/share/skupper/namespaces/test1/", string(api.InputSiteStatePath), "/sites")

	testTable := []test{
		{
			name:         "runs fails no directory",
			errorMessage: "failed to read directory: open " + path + ": no such file or directory",
		},
	}

	for _, test := range testTable {
		command := &CmdSiteStatus{}
		command.namespace = "test1"
		command.siteHandler = fs.NewSiteHandler(command.namespace)
		command.siteName = test.siteName
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

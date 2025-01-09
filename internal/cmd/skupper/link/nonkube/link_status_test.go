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
	"gotest.tools/v3/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestCmdLinkStatus_ValidateInput(t *testing.T) {
	type test struct {
		name              string
		args              []string
		flags             *common.CommandLinkStatusFlags
		cobraGenericFlags map[string]string
		k8sObjects        []runtime.Object
		skupperObjects    []runtime.Object
		expectedErrors    []string
		linkName          string
	}

	homeDir, err := os.UserHomeDir()
	assert.Check(t, err == nil)
	path := filepath.Join(homeDir, "/.local/share/skupper/namespaces/test/", string(api.RuntimeSiteStatePath))

	testTable := []test{
		{
			name:           "link is not shown because link does not exist in the namespace",
			args:           []string{"no-link"},
			flags:          &common.CommandLinkStatusFlags{},
			expectedErrors: []string{"link no-link does not exist in namespace test"},
		},
		{
			name:           "link name is nil",
			args:           []string{""},
			flags:          &common.CommandLinkStatusFlags{},
			expectedErrors: []string{"link name must not be empty"},
		},
		{
			name:           "more than one argument is specified",
			args:           []string{"my", "link"},
			flags:          &common.CommandLinkStatusFlags{},
			expectedErrors: []string{"only one argument is allowed for this command"},
		},
		{
			name:           "link name is not valid.",
			args:           []string{"my new link"},
			flags:          &common.CommandLinkStatusFlags{},
			expectedErrors: []string{"link name is not valid: value does not match this regular expression: ^[a-z0-9]([-a-z0-9]*[a-z0-9])*(\\.[a-z0-9]([-a-z0-9]*[a-z0-9])*)*$"},
		},
		{
			name:           "no args",
			flags:          &common.CommandLinkStatusFlags{},
			expectedErrors: []string{},
		},
	}

	//Add a temp file so link exists for status tests
	linkResource := v2alpha1.Link{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "skupper.io/v2alpha1",
			Kind:       "Link",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-link",
			Namespace: "test",
		},
	}

	command := &CmdLinkStatus{}
	command.namespace = "test"
	command.linkHandler = fs.NewLinkHandler(command.namespace)

	defer command.linkHandler.Delete("my-link")
	content, err := command.linkHandler.EncodeToYaml(linkResource)
	assert.Check(t, err == nil)
	err = command.linkHandler.WriteFile(path, "my-link.yaml", content, common.Links)
	assert.Check(t, err == nil)

	for _, test := range testTable {
		t.Run(test.name, func(t *testing.T) {
			command.linkName = ""

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

func TestCmdLinkStatus_Run(t *testing.T) {
	type test struct {
		name                string
		linkName            string
		flags               common.CommandLinkStatusFlags
		k8sObjects          []runtime.Object
		skupperObjects      []runtime.Object
		skupperErrorMessage string
		errorMessage        string
	}

	homeDir, err := os.UserHomeDir()
	assert.Check(t, err == nil)
	path := filepath.Join(homeDir, "/.local/share/skupper/namespaces/test/", string(api.RuntimeSiteStatePath))

	testTable := []test{
		{
			name:         "run fails link doesn't exist",
			linkName:     "no-link",
			errorMessage: "failed to read file: open " + path + "/links/no-link.yaml: no such file or directory",
		},
		{
			name:     "runs ok, returns 1 links",
			linkName: "my-link",
		},
		{
			name:     "runs ok, returns 1 links yaml",
			linkName: "my-link",
			flags:    common.CommandLinkStatusFlags{Output: "yaml"},
		},
		{
			name: "runs ok, returns all links",
		},
		{
			name:  "runs ok, returns all links json",
			flags: common.CommandLinkStatusFlags{Output: "json"},
		},
		{
			name:         "runs ok, returns all links output bad",
			flags:        common.CommandLinkStatusFlags{Output: "bad-value"},
			errorMessage: "format bad-value not supported",
		},
		{
			name:         "runs ok, returns 1 links bad output",
			linkName:     "my-link",
			flags:        common.CommandLinkStatusFlags{Output: "bad-value"},
			errorMessage: "format bad-value not supported",
		},
	}

	//Add a temp file so link exists for status tests
	linkResource1 := v2alpha1.Link{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "skupper.io/v2alpha1",
			Kind:       "Link",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-link",
			Namespace: "test",
		},
		Spec: v2alpha1.LinkSpec{
			TlsCredentials: "my-secret",
			Cost:           2,
		},
		Status: v2alpha1.LinkStatus{
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
	linkResource2 := v2alpha1.Link{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "skupper.io/v2alpha1",
			Kind:       "link",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-link2",
			Namespace: "test",
		},
		Spec: v2alpha1.LinkSpec{
			TlsCredentials: "my-secret",
			Cost:           1,
		},
		Status: v2alpha1.LinkStatus{
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

	// add two Links in runtime directory
	command := &CmdLinkStatus{}
	command.namespace = "test"
	command.linkHandler = fs.NewLinkHandler(command.namespace)

	defer command.linkHandler.Delete("my-link")
	defer command.linkHandler.Delete("my-link2")

	path = filepath.Join(homeDir, "/.local/share/skupper/namespaces/test/", string(api.RuntimeSiteStatePath))
	content, err := command.linkHandler.EncodeToYaml(linkResource1)
	assert.Check(t, err == nil)
	err = command.linkHandler.WriteFile(path, "my-link.yaml", content, common.Links)
	assert.Check(t, err == nil)

	content, err = command.linkHandler.EncodeToYaml(linkResource2)
	assert.Check(t, err == nil)
	err = command.linkHandler.WriteFile(path, "my-link2.yaml", content, common.Links)
	assert.Check(t, err == nil)

	for _, test := range testTable {
		command.linkName = test.linkName
		command.Flags = &test.flags

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

func TestCmdLinkStatus_RunNoDirectory(t *testing.T) {
	type test struct {
		name                string
		flags               common.CommandLinkStatusFlags
		k8sObjects          []runtime.Object
		skupperObjects      []runtime.Object
		skupperErrorMessage string
		errorMessage        string
		linkName            string
	}

	homeDir, err := os.UserHomeDir()
	assert.Check(t, err == nil)
	path := filepath.Join(homeDir, "/.local/share/skupper/namespaces/test1/", string(api.RuntimeSiteStatePath))

	testTable := []test{
		{
			name:         "runs fails no directory",
			errorMessage: "failed to read directory: open " + path + "/links: no such file or directory",
		},
		{
			name:         "runs fails no directory",
			linkName:     "my-link",
			errorMessage: "failed to read file: open " + path + "/links/my-link.yaml: no such file or directory",
		},
	}

	for _, test := range testTable {
		command := &CmdLinkStatus{}
		command.namespace = "test1"
		command.linkHandler = fs.NewLinkHandler(command.namespace)
		command.linkName = test.linkName
		command.Flags = &test.flags
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

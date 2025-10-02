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
		expectedError     string
		linkName          string
	}

	if os.Getuid() == 0 {
		api.DefaultRootDataHome = t.TempDir()
	} else {
		t.Setenv("XDG_DATA_HOME", t.TempDir())
	}
	tmpDir := api.GetDataHome()
	path := filepath.Join(tmpDir, "namespaces/test1/", string(api.RuntimeSiteStatePath))

	testTable := []test{
		{
			name:  "link is not shown because link does not exist in the namespace",
			args:  []string{"no-link"},
			flags: &common.CommandLinkStatusFlags{},
		},
		{
			name:          "link name is nil",
			args:          []string{""},
			flags:         &common.CommandLinkStatusFlags{},
			expectedError: "link name must not be empty",
		},
		{
			name:          "more than one argument is specified",
			args:          []string{"my", "link"},
			flags:         &common.CommandLinkStatusFlags{},
			expectedError: "only one argument is allowed for this command",
		},
		{
			name:          "link name is not valid.",
			args:          []string{"my new link"},
			flags:         &common.CommandLinkStatusFlags{},
			expectedError: "link name is not valid: value does not match this regular expression: ^[a-z0-9]([-a-z0-9]*[a-z0-9])*(\\.[a-z0-9]([-a-z0-9]*[a-z0-9])*)*$",
		},
		{
			name:  "no args",
			flags: &common.CommandLinkStatusFlags{},
		},
		{
			name:          "good output status",
			args:          []string{"my-link"},
			flags:         &common.CommandLinkStatusFlags{Output: "json"},
			expectedError: "",
		},
		{
			name:          "bad output status",
			args:          []string{"my-link"},
			flags:         &common.CommandLinkStatusFlags{Output: "invalid"},
			expectedError: "format bad-value not supported",
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

			testutils.CheckValidateInput(t, command, test.expectedError, test.args)

		})
	}
}

func TestCmdLinkStatus_Run(t *testing.T) {
	type test struct {
		name         string
		linkName     string
		flags        common.CommandLinkStatusFlags
		errorMessage string
	}

	if os.Getuid() == 0 {
		api.DefaultRootDataHome = t.TempDir()
	} else {
		t.Setenv("XDG_DATA_HOME", t.TempDir())
	}
	tmpDir := api.GetDataHome()
	path := filepath.Join(tmpDir, "namespaces/test1/", string(api.RuntimeSiteStatePath))

	testTable := []test{
		{
			name:         "runs ok, link doesn't exist",
			linkName:     "no-link",
			errorMessage: "There is no link resource in the namespace with the name \"no-link\"",
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
			Endpoints: []v2alpha1.Endpoint{
				{
					Name: "inter-router",
					Host: "127.0.0.1",
					Port: "55671",
				},
				{
					Name: "edge",
					Host: "127.0.0.1",
					Port: "45671",
				},
			},
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
		Spec: v2alpha1.SiteSpec{
			Edge: true,
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

	// add two Links in runtime directory
	command := &CmdLinkStatus{}
	command.namespace = "test"
	command.linkHandler = fs.NewLinkHandler(command.namespace)
	command.siteHandler = fs.NewSiteHandler(command.namespace)

	defer command.linkHandler.Delete("my-link")
	defer command.linkHandler.Delete("my-link2")
	defer command.siteHandler.Delete("my-site")

	path = filepath.Join(tmpDir, "/namespaces/test/", string(api.RuntimeSiteStatePath))
	content, err := command.linkHandler.EncodeToYaml(linkResource1)
	assert.Check(t, err == nil)
	err = command.linkHandler.WriteFile(path, "my-link.yaml", content, common.Links)
	assert.Check(t, err == nil)

	content, err = command.linkHandler.EncodeToYaml(linkResource2)
	assert.Check(t, err == nil)
	err = command.linkHandler.WriteFile(path, "my-link2.yaml", content, common.Links)
	assert.Check(t, err == nil)

	content, err = command.siteHandler.EncodeToYaml(siteResource)
	assert.Check(t, err == nil)
	err = command.siteHandler.WriteFile(path, "my-site.yaml", content, common.Sites)
	assert.Check(t, err == nil)

	for _, test := range testTable {
		command.linkName = test.linkName
		command.Flags = &test.flags
		command.output = command.Flags.Output

		t.Run(test.name, func(t *testing.T) {
			err := command.Run()
			if err != nil {
				assert.Check(t, test.errorMessage == err.Error(), test.errorMessage, err.Error())
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

	if os.Getuid() == 0 {
		api.DefaultRootDataHome = t.TempDir()
	} else {
		t.Setenv("XDG_DATA_HOME", t.TempDir())
	}
	tmpDir := api.GetDataHome()
	path := filepath.Join(tmpDir, "namespaces/test1/", string(api.RuntimeSiteStatePath))

	testTable := []test{
		{
			name:         "runs fails no directory",
			errorMessage: "failed to read directory: open " + path + ": no such file or directory",
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
				assert.Check(t, test.errorMessage == err.Error(), err.Error(), test.errorMessage)
			} else {
				assert.Check(t, err == nil)
			}
		})
	}
}

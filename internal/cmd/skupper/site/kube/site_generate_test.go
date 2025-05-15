package kube

import (
	"testing"

	"github.com/skupperproject/skupper/internal/cmd/skupper/common"
	"github.com/skupperproject/skupper/internal/cmd/skupper/common/testutils"

	fakeclient "github.com/skupperproject/skupper/internal/kube/client/fake"
	"gotest.tools/v3/assert"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestCmdSiteGenerate_ValidateInput(t *testing.T) {
	type test struct {
		name           string
		args           []string
		k8sObjects     []runtime.Object
		skupperObjects []runtime.Object
		flags          *common.CommandSiteGenerateFlags
		expectedError  string
	}

	testTable := []test{
		{
			name:          "site name is not valid.",
			args:          []string{"my new site"},
			expectedError: "site name is not valid: value does not match this regular expression: ^[a-z0-9]([-a-z0-9]*[a-z0-9])*(\\.[a-z0-9]([-a-z0-9]*[a-z0-9])*)*$",
		},
		{
			name:          "site name is not specified.",
			args:          []string{},
			expectedError: "site name must not be empty",
		},
		{
			name:          "more than one argument was specified",
			args:          []string{"my", "site"},
			expectedError: "only one argument is allowed for this command.",
		},
		{
			name:  "link access type is not valid",
			args:  []string{"my-site"},
			flags: &common.CommandSiteGenerateFlags{LinkAccessType: "not-valid"},
			expectedError: "link access type is not valid: value not-valid not allowed. It should be one of this options: [route loadbalancer default]\n" +
				"for the site to work with this type of linkAccess, the --enable-link-access option must be set to true",
		},
		{
			name:          "output format is not valid",
			args:          []string{"my-site"},
			flags:         &common.CommandSiteGenerateFlags{Output: "not-valid"},
			expectedError: "format value not-valid not allowed. It should be one of this options: [json yaml]",
		},
	}

	for _, test := range testTable {
		t.Run(test.name, func(t *testing.T) {
			command := &CmdSiteGenerate{
				Namespace: "test",
			}

			cmd := common.ConfigureCobraCommand(common.PlatformKubernetes, common.SkupperCmdDescription{}, command, nil)

			command.CobraCmd = cmd

			if test.flags != nil {
				command.Flags = test.flags
			}

			testutils.CheckValidateInput(t, command, test.expectedError, test.args)

		})
	}
}

func TestCmdSiteGenerate_InputToOptions(t *testing.T) {

	type test struct {
		name               string
		args               []string
		flags              common.CommandSiteGenerateFlags
		expectedLinkAccess string
		expectedHA         bool
		expectedOutput     string
	}

	testTable := []test{
		{
			name:               "options without link access enabled",
			args:               []string{"my-site"},
			flags:              common.CommandSiteGenerateFlags{},
			expectedLinkAccess: "",
			expectedOutput:     "yaml",
			expectedHA:         false,
		},
		{
			name:               "options with link access enabled but using a type by default",
			args:               []string{"my-site"},
			flags:              common.CommandSiteGenerateFlags{EnableLinkAccess: true},
			expectedLinkAccess: "default",
			expectedOutput:     "yaml",
			expectedHA:         false,
		},
		{
			name:               "options with link access enabled using the nodeport type",
			args:               []string{"my-site"},
			flags:              common.CommandSiteGenerateFlags{EnableLinkAccess: true, LinkAccessType: "nodeport"},
			expectedLinkAccess: "nodeport",
			expectedOutput:     "yaml",
			expectedHA:         false,
		},
		{
			name:               "options with link access options not well specified",
			args:               []string{"my-site"},
			flags:              common.CommandSiteGenerateFlags{EnableLinkAccess: false, LinkAccessType: "nodeport"},
			expectedLinkAccess: "",
			expectedOutput:     "yaml",
			expectedHA:         false,
		},
		{
			name:               "options output type",
			args:               []string{"my-site"},
			flags:              common.CommandSiteGenerateFlags{EnableLinkAccess: false, LinkAccessType: "nodeport", Output: "json"},
			expectedLinkAccess: "",
			expectedOutput:     "json",
			expectedHA:         false,
		},
		{
			name:               "options with HA enabled",
			args:               []string{"my-site"},
			flags:              common.CommandSiteGenerateFlags{EnableHA: true},
			expectedLinkAccess: "",
			expectedOutput:     "yaml",
			expectedHA:         true,
		},
	}

	for _, test := range testTable {
		t.Run(test.name, func(t *testing.T) {
			cmd, err := newCmdSiteGenerateWithMocks("test")
			assert.Assert(t, err)
			cmd.Flags = &test.flags
			cmd.siteName = "my-site"

			cmd.InputToOptions()

			assert.Check(t, cmd.output == test.expectedOutput)
			assert.Check(t, cmd.linkAccessType == test.expectedLinkAccess)
			assert.Check(t, cmd.HA == test.expectedHA)
		})
	}
}

func TestCmdSiteGenerate_Run(t *testing.T) {
	type test struct {
		name           string
		k8sObjects     []runtime.Object
		skupperObjects []runtime.Object
		skupperError   string
		siteName       string
		options        map[string]string
		output         string
		errorMessage   string
	}

	testTable := []test{
		{
			name:           "runs ok",
			k8sObjects:     nil,
			skupperObjects: nil,
			siteName:       "my-site",
			options:        map[string]string{"name": "my-site"},
			skupperError:   "",
			output:         "yaml",
		},
		{
			name:           "runs ok with yaml output",
			k8sObjects:     nil,
			skupperObjects: nil,
			siteName:       "test",
			options:        map[string]string{"name": "my-site"},
			output:         "json",
			skupperError:   "",
		},
		{
			name:           "runs fails because the output format is not supported",
			k8sObjects:     nil,
			skupperObjects: nil,
			siteName:       "test",
			options:        map[string]string{"name": "my-site"},
			output:         "unsupported",
			skupperError:   "",
			errorMessage:   "format unsupported not supported",
		},
	}

	for _, test := range testTable {
		command := &CmdSiteGenerate{
			Namespace: "test",
		}

		fakeSkupperClient, err := fakeclient.NewFakeClient(command.Namespace, test.k8sObjects, test.skupperObjects, test.skupperError)
		assert.Assert(t, err)
		command.Client = fakeSkupperClient.GetSkupperClient().SkupperV2alpha1()

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

// --- helper methods

func newCmdSiteGenerateWithMocks(namespace string) (*CmdSiteGenerate, error) {

	cmdSiteGenerate := &CmdSiteGenerate{
		Namespace: namespace,
	}

	return cmdSiteGenerate, nil
}

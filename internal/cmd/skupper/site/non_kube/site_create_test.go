package non_kube

import (
	"github.com/skupperproject/skupper/internal/cmd/skupper/common"
	"github.com/skupperproject/skupper/internal/cmd/skupper/common/utils"
	"github.com/spf13/cobra"
	"io/ioutil"
	"os"

	"testing"

	"gotest.tools/assert"
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
		expectedErrors    []string
	}

	testTable := []test{
		{
			name:           "site name is not valid.",
			args:           []string{"my new site"},
			flags:          &common.CommandSiteCreateFlags{Host: "host"},
			expectedErrors: []string{"site name is not valid: value does not match this regular expression: ^[a-z0-9]([-a-z0-9]*[a-z0-9])*(\\.[a-z0-9]([-a-z0-9]*[a-z0-9])*)*$"},
		},
		{
			name:           "site name is not specified.",
			args:           []string{},
			flags:          &common.CommandSiteCreateFlags{Host: "host"},
			expectedErrors: []string{"site name must not be empty"},
		},
		{
			name:           "more than one argument was specified",
			args:           []string{"my", "site"},
			flags:          &common.CommandSiteCreateFlags{Host: "host"},
			expectedErrors: []string{"only one argument is allowed for this command"},
		},
		{
			name:  "link access type is not valid",
			args:  []string{"my-site"},
			flags: &common.CommandSiteCreateFlags{Host: "host", LinkAccessType: "not-valid"},
			expectedErrors: []string{
				"link access type is not valid: value not-valid not allowed. It should be one of this options: [route loadbalancer default]",
				"for the site to work with this type of linkAccess, the --enable-link-access option must be set to true",
			},
		},
		{
			name:  "output format is not valid",
			args:  []string{"my-site"},
			flags: &common.CommandSiteCreateFlags{Host: "host", Output: "not-valid"},
			expectedErrors: []string{
				"output type is not valid: value not-valid not allowed. It should be one of this options: [json yaml]",
			},
		},
		{
			name:  "host was not specified",
			args:  []string{"my-site"},
			flags: &common.CommandSiteCreateFlags{},
			expectedErrors: []string{
				"host should not be empty",
			},
		},
		{
			name:  "service-account is not valid on this platform",
			args:  []string{"my-site"},
			flags: &common.CommandSiteCreateFlags{ServiceAccount: "service-account", Host: "host"},
			expectedErrors: []string{
				"--service-account flag is not supported on this platform",
			},
		},
		{
			name:  "kubernetes flags are not valid on this platform",
			args:  []string{"my-site"},
			flags: &common.CommandSiteCreateFlags{Host: "host"},
			expectedErrors: []string{
				"--context flag is not supported on this platform",
				"--kubeconfig flag is not supported on this platform",
			},
			cobraGenericFlags: map[string]string{
				common.FlagNameContext:    "test",
				common.FlagNameKubeconfig: "test",
			},
		},
	}

	for _, test := range testTable {
		t.Run(test.name, func(t *testing.T) {
			command := &CmdSiteCreate{Flags: &common.CommandSiteCreateFlags{}}
			command.CobraCmd = &cobra.Command{Use: "test"}

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

func TestNonKubeCmdSiteCreate_InputToOptions(t *testing.T) {

	type test struct {
		name               string
		args               []string
		namespace          string
		flags              common.CommandSiteCreateFlags
		expectedSettings   map[string]string
		expectedLinkAccess string
		expectedOutput     string
		expectedNamespace  string
	}

	testTable := []test{
		{
			name:  "options without link access enabled",
			args:  []string{"my-site"},
			flags: common.CommandSiteCreateFlags{},
			expectedSettings: map[string]string{
				"name": "my-site",
			},
			expectedLinkAccess: "none",
			expectedOutput:     "",
			expectedNamespace:  "default",
		},
		{
			name:  "options with link access enabled but using a type by default and link access host specified",
			args:  []string{"my-site"},
			flags: common.CommandSiteCreateFlags{EnableLinkAccess: true},
			expectedSettings: map[string]string{
				"name": "my-site",
			},
			expectedLinkAccess: "loadbalancer",
			expectedOutput:     "",
			expectedNamespace:  "default",
		},
		{
			name:  "options with link access enabled using the nodeport type",
			args:  []string{"my-site"},
			flags: common.CommandSiteCreateFlags{EnableLinkAccess: true, LinkAccessType: "nodeport"},
			expectedSettings: map[string]string{
				"name": "my-site",
			},
			expectedLinkAccess: "nodeport",
			expectedOutput:     "",
			expectedNamespace:  "default",
		},
		{
			name:  "options with link access options not well specified",
			args:  []string{"my-site"},
			flags: common.CommandSiteCreateFlags{EnableLinkAccess: false, LinkAccessType: "nodeport"},
			expectedSettings: map[string]string{
				"name": "my-site",
			},
			expectedLinkAccess: "none",
			expectedOutput:     "",
			expectedNamespace:  "default",
		},
		{
			name:  "options output type",
			args:  []string{"my-site"},
			flags: common.CommandSiteCreateFlags{EnableLinkAccess: false, LinkAccessType: "nodeport", Output: "yaml"},
			expectedSettings: map[string]string{
				"name": "my-site",
			},
			expectedLinkAccess: "none",
			expectedOutput:     "yaml",
			expectedNamespace:  "default",
		},
		{
			name:      "options with specific namespace",
			args:      []string{"my-site"},
			namespace: "test",
			flags:     common.CommandSiteCreateFlags{},
			expectedSettings: map[string]string{
				"name": "my-site",
			},
			expectedNamespace: "test",
		},
	}

	for _, test := range testTable {
		t.Run(test.name, func(t *testing.T) {
			cmd := CmdSiteCreate{}
			cmd.Flags = &test.flags
			cmd.siteName = "my-site"
			cmd.namespace = test.namespace

			cmd.InputToOptions()

			assert.DeepEqual(t, cmd.options, test.expectedSettings)

			assert.Check(t, cmd.output == test.expectedOutput)
			assert.Check(t, cmd.namespace == test.expectedNamespace)
		})
	}
}

func TestNonKubeCmdSiteCreate_Run(t *testing.T) {
	type test struct {
		name             string
		k8sObjects       []runtime.Object
		skupperObjects   []runtime.Object
		skupperError     string
		siteName         string
		host             string
		inputPath        string
		options          map[string]string
		output           string
		errorMessage     string
		routerAccessName string
	}

	testTable := []test{
		{
			name:             "runs ok",
			k8sObjects:       nil,
			skupperObjects:   nil,
			siteName:         "my-site",
			host:             "host",
			inputPath:        "default",
			routerAccessName: "ra-test",
			options:          map[string]string{"name": "my-site"},
		},
		{
			name:           "runs ok without create site",
			k8sObjects:     nil,
			skupperObjects: nil,
			siteName:       "test",
			host:           "host",
			options:        map[string]string{"name": "my-site"},
			output:         "yaml",
			skupperError:   "",
		},
		{
			name:           "runs fails because the output format is not supported",
			k8sObjects:     nil,
			skupperObjects: nil,
			siteName:       "test",
			host:           "host",
			options:        map[string]string{"name": "my-site"},
			output:         "unsupported",
			skupperError:   "",
			errorMessage:   "format unsupported not supported",
		},
	}

	for _, test := range testTable {
		command := &CmdSiteCreate{}

		command.siteName = test.siteName
		command.options = test.options
		command.output = test.output
		command.inputPath = test.inputPath
		command.routerAccessName = test.routerAccessName

		tmpDir, err := ioutil.TempDir("", test.inputPath)
		if err != nil {
			t.Fatalf("Failed to create temp directory: %s", err)
		}
		defer os.RemoveAll(tmpDir)

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

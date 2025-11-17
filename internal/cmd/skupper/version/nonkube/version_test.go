package nonkube

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/skupperproject/skupper/internal/cmd/skupper/common"
	"github.com/skupperproject/skupper/internal/cmd/skupper/common/testutils"
	"github.com/skupperproject/skupper/internal/utils/configs"
	"github.com/skupperproject/skupper/pkg/nonkube/api"
	"gotest.tools/v3/assert"
	"k8s.io/apimachinery/pkg/runtime"
)

var (
	nonkubeComponents = []string{"router"}
)

func TestCmdVersion_ValidateInput(t *testing.T) {
	type test struct {
		name           string
		args           []string
		flags          common.CommandVersionFlags
		namespace      string
		k8sObjects     []runtime.Object
		skupperObjects []runtime.Object
		expectedError  string
	}

	testTable := []test{
		{
			name:          "incorrect output type",
			args:          []string{""},
			namespace:     "test",
			flags:         common.CommandVersionFlags{Output: "not-supported"},
			expectedError: "output type is not valid: value not-supported not allowed. It should be one of this options: [json yaml]",
		},
		{
			name:          "valid output type",
			namespace:     "test",
			flags:         common.CommandVersionFlags{Output: "json"},
			expectedError: "",
		},
		{
			name:          "unspecified output type",
			namespace:     "test",
			flags:         common.CommandVersionFlags{},
			expectedError: "",
		},
		{
			name:          "unknown namespace",
			namespace:     "unknown",
			flags:         common.CommandVersionFlags{},
			expectedError: "there is no definition for namespace \"unknown\"",
		},
		{
			name:          "invalid-namespace",
			namespace:     "Invalid",
			expectedError: "namespace is not valid: value does not match this regular expression: ^[a-z0-9]([-a-z0-9]*[a-z0-9])?$\nthere is no definition for namespace \"Invalid\"",
		},
	}

	for _, test := range testTable {
		t.Run(test.name, func(t *testing.T) {

			if os.Getuid() == 0 {
				api.DefaultRootDataHome = t.TempDir()
			} else {
				t.Setenv("XDG_DATA_HOME", t.TempDir())
			}
			tmpDir := api.GetDataHome()

			nestedDir := filepath.Join(tmpDir, "namespaces", "test")
			err := os.MkdirAll(nestedDir, os.ModePerm)
			if err != nil {
				t.Fatalf("failed to create directories: %v", err)
			}

			cmd, err := newCmdVersionWithMocks(test.namespace)
			assert.Assert(t, err)

			cmd.Flags = &test.flags

			testutils.CheckValidateInput(t, cmd, test.expectedError, test.args)
		})
	}
}

func TestCmdVersion_InputToOptions(t *testing.T) {
	type test struct {
		name           string
		output         string
		k8sObjects     []runtime.Object
		skupperObjects []runtime.Object
		expected       bool
	}

	testTable := []test{
		{
			name:     "output type selected is json",
			output:   "json",
			expected: true,
		},
		{
			name:     "output type selected is the default one",
			expected: false,
		},
	}

	for _, test := range testTable {
		t.Run(test.name, func(t *testing.T) {

			cmd, err := newCmdVersionWithMocks("test")
			assert.Assert(t, err)

			cmd.output = test.output
			cmd.InputToOptions()

			assert.DeepEqual(t, test.expected, test.expected)
		})
	}
}

func TestCmdVersion_Run(t *testing.T) {
	type test struct {
		name                string
		VersionName         string
		flags               common.CommandVersionFlags
		k8sObjects          []runtime.Object
		skupperObjects      []runtime.Object
		skupperErrorMessage string
		errorMessage        string
	}

	testTable := []test{
		{
			name:  "default",
			flags: common.CommandVersionFlags{},
		},
		{
			name:  "json",
			flags: common.CommandVersionFlags{Output: "json"},
		},
		{
			name:         "no valid",
			flags:        common.CommandVersionFlags{Output: "not-valid"},
			errorMessage: "format not-valid not supported",
		},
	}

	for _, test := range testTable {
		cmd, err := newCmdVersionWithMocks("test")
		assert.Assert(t, err)
		cmd.Flags = &test.flags
		cmd.output = cmd.Flags.Output
		cmd.namespace = "test"
		cmd.manifest = configs.ManifestManager{Components: nonkubeComponents, EnableSHA: false}

		t.Run(test.name, func(t *testing.T) {

			err := cmd.Run()
			if err != nil {
				assert.Check(t, test.errorMessage == err.Error())
			} else {
				assert.Check(t, err == nil)
			}
		})
	}
}

// --- helper methods

func newCmdVersionWithMocks(namespace string) (*CmdVersion, error) {

	cmdVersion := &CmdVersion{
		namespace: namespace,
	}

	return cmdVersion, nil
}

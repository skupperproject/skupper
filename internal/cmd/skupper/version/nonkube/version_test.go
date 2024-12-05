package nonkube

import (
	"testing"

	"github.com/skupperproject/skupper/internal/cmd/skupper/common"
	"github.com/skupperproject/skupper/internal/cmd/skupper/common/utils"
	"github.com/skupperproject/skupper/pkg/utils/configs"
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
		k8sObjects     []runtime.Object
		skupperObjects []runtime.Object
		expectedErrors []string
	}

	testTable := []test{
		{
			name:           "bad output",
			args:           []string{""},
			flags:          common.CommandVersionFlags{Output: "not-supported"},
			expectedErrors: []string{"output type is not valid: value not-supported not allowed. It should be one of this options: [json yaml]"},
		},
		{
			name:           "good output",
			flags:          common.CommandVersionFlags{Output: "json"},
			expectedErrors: []string{},
		},
		{
			name:           "ok no output",
			flags:          common.CommandVersionFlags{},
			expectedErrors: []string{},
		},
	}

	for _, test := range testTable {
		t.Run(test.name, func(t *testing.T) {

			cmd, err := newCmdVersionWithMocks("test", test.k8sObjects, test.skupperObjects, "")
			assert.Assert(t, err)

			cmd.Flags = &test.flags

			actualErrors := cmd.ValidateInput(test.args)

			actualErrorsMessages := utils.ErrorsToMessages(actualErrors)

			assert.DeepEqual(t, actualErrorsMessages, test.expectedErrors)
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
			name:     "good json",
			output:   "json",
			expected: true,
		},
		{
			name:     "good defaul",
			expected: false,
		},
	}

	for _, test := range testTable {
		t.Run(test.name, func(t *testing.T) {

			cmd, err := newCmdVersionWithMocks("test", test.k8sObjects, test.skupperObjects, "")
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
		cmd, err := newCmdVersionWithMocks("test", test.k8sObjects, test.skupperObjects, test.skupperErrorMessage)
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

func newCmdVersionWithMocks(namespace string, k8sObjects []runtime.Object, skupperObjects []runtime.Object, fakeSkupperError string) (*CmdVersion, error) {

	cmdVersion := &CmdVersion{
		namespace: namespace,
	}

	return cmdVersion, nil
}

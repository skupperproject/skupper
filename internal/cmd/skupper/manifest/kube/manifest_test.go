package kube

import (
	"testing"

	"github.com/skupperproject/skupper/internal/cmd/skupper/common"
	"github.com/skupperproject/skupper/internal/cmd/skupper/common/testutils"
	fakeclient "github.com/skupperproject/skupper/internal/kube/client/fake"
	"github.com/skupperproject/skupper/internal/utils/configs"
	"gotest.tools/v3/assert"
	"k8s.io/apimachinery/pkg/runtime"
)

var (
	kubeComponents = []string{"router"}
)

func TestCmdManifest_ValidateInput(t *testing.T) {
	type test struct {
		name           string
		args           []string
		flags          common.CommandVersionFlags
		k8sObjects     []runtime.Object
		skupperObjects []runtime.Object
		expectedError  string
	}

	testTable := []test{
		{
			name:          "bad output",
			args:          []string{""},
			flags:         common.CommandVersionFlags{Output: "not-supported"},
			expectedError: "output type is not valid: value not-supported not allowed. It should be one of this options: [json yaml]",
		},
		{
			name:          "good output",
			flags:         common.CommandVersionFlags{Output: "json"},
			expectedError: "",
		},
		{
			name:          "ok no output",
			flags:         common.CommandVersionFlags{},
			expectedError: "",
		},
	}

	for _, test := range testTable {
		t.Run(test.name, func(t *testing.T) {

			cmd, err := newCmdManifestWithMocks("test", test.k8sObjects, test.skupperObjects, "")
			assert.Assert(t, err)

			cmd.Flags = &test.flags

			testutils.CheckValidateInput(t, cmd, test.expectedError, test.args)
		})
	}
}

func TestCmdManifest_InputToOptions(t *testing.T) {
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
			name:     "good default",
			expected: false,
		},
	}

	for _, test := range testTable {
		t.Run(test.name, func(t *testing.T) {

			cmd, err := newCmdManifestWithMocks("test", test.k8sObjects, test.skupperObjects, "")
			assert.Assert(t, err)

			cmd.output = test.output
			cmd.InputToOptions()

			assert.DeepEqual(t, test.expected, test.expected)
		})
	}
}

func TestCmdManifest_Run(t *testing.T) {
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
		{
			name:         "no cluster",
			flags:        common.CommandVersionFlags{},
			errorMessage: "Cluster is not accessible",
		},
	}

	for _, test := range testTable {
		cmd, err := newCmdManifestWithMocks("test", test.k8sObjects, test.skupperObjects, test.skupperErrorMessage)
		assert.Assert(t, err)
		cmd.Flags = &test.flags
		cmd.output = cmd.Flags.Output
		cmd.namespace = "test"
		cmd.manifest = configs.ManifestManager{Components: kubeComponents, EnableSHA: false}

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

func newCmdManifestWithMocks(namespace string, k8sObjects []runtime.Object, skupperObjects []runtime.Object, fakeSkupperError string) (*CmdManifest, error) {

	client, err := fakeclient.NewFakeClient(namespace, k8sObjects, skupperObjects, fakeSkupperError)
	if err != nil {
		return nil, err
	}
	cmdManifest := &CmdManifest{
		Client:     client.GetSkupperClient().SkupperV2alpha1(),
		namespace:  namespace,
		KubeClient: client.GetKubeClient(),
	}

	return cmdManifest, nil
}

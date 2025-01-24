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
	"github.com/spf13/cobra"
	"gotest.tools/v3/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestCmdListenerDelete_ValidateInput(t *testing.T) {
	type test struct {
		name              string
		args              []string
		flags             *common.CommandListenerDeleteFlags
		cobraGenericFlags map[string]string
		k8sObjects        []runtime.Object
		skupperObjects    []runtime.Object
		expectedError     string
	}

	homeDir, err := os.UserHomeDir()
	assert.Check(t, err == nil)
	path := filepath.Join(homeDir, "/.local/share/skupper/namespaces/test/", string(api.InputSiteStatePath))

	testTable := []test{
		{
			name:          "listener name is not specified",
			args:          []string{},
			flags:         &common.CommandListenerDeleteFlags{},
			expectedError: "listener name must be specified",
		},
		{
			name:          "listener name is nil",
			args:          []string{""},
			flags:         &common.CommandListenerDeleteFlags{},
			expectedError: "listener name must not be empty",
		},
		{
			name:          "listener name is not valid",
			args:          []string{"my name"},
			flags:         &common.CommandListenerDeleteFlags{},
			expectedError: "listener name is not valid: value does not match this regular expression: ^[a-z0-9]([-a-z0-9]*[a-z0-9])*(\\.[a-z0-9]([-a-z0-9]*[a-z0-9])*)*$",
		},
		{
			name:          "more than one argument is specified",
			args:          []string{"my", "listener"},
			flags:         &common.CommandListenerDeleteFlags{},
			expectedError: "only one argument is allowed for this command",
		},
		{
			name:          "listener doesn't exist",
			args:          []string{"no-listener"},
			flags:         &common.CommandListenerDeleteFlags{},
			expectedError: "listener no-listener does not exist",
		},
		{
			name:          "kubernetes flags are not valid on this platform",
			args:          []string{"my-listener"},
			flags:         &common.CommandListenerDeleteFlags{},
			expectedError: "",
			cobraGenericFlags: map[string]string{
				common.FlagNameContext:    "test",
				common.FlagNameKubeconfig: "test",
			},
		},
	}

	// Add a temp file so listener exists for delete tests to pass
	listenerResource := v2alpha1.Listener{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "skupper.io/v2alpha1",
			Kind:       "Listener",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-listener",
			Namespace: "test",
		},
	}

	command := &CmdListenerDelete{Flags: &common.CommandListenerDeleteFlags{}}
	command.CobraCmd = &cobra.Command{Use: "test"}
	command.namespace = "test"
	command.listenerHandler = fs.NewListenerHandler(command.namespace)

	defer command.listenerHandler.Delete("my-listener")
	content, err := command.listenerHandler.EncodeToYaml(listenerResource)
	assert.Check(t, err == nil)
	err = command.listenerHandler.WriteFile(path, "my-listener.yaml", content, common.Listeners)
	assert.Check(t, err == nil)

	for _, test := range testTable {
		t.Run(test.name, func(t *testing.T) {
			command.listenerName = ""
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

func TestCmdListenerDelete_Run(t *testing.T) {
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
			name:                "run fails default",
			deleteName:          "my-listener",
			skupperErrorMessage: "error",
			errorMessage:        "error",
		},
		{
			name:                "run fails",
			namespace:           "test",
			deleteName:          "my-listener",
			skupperErrorMessage: "error",
			errorMessage:        "error",
		},
	}

	for _, test := range testTable {
		cmd := &CmdListenerDelete{}

		t.Run(test.name, func(t *testing.T) {

			cmd.listenerName = test.deleteName
			cmd.namespace = test.namespace
			cmd.listenerHandler = fs.NewListenerHandler(cmd.namespace)
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

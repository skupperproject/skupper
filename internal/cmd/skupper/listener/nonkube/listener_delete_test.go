package nonkube

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/skupperproject/skupper/internal/cmd/skupper/common"
	"github.com/skupperproject/skupper/internal/cmd/skupper/common/testutils"
	"github.com/skupperproject/skupper/internal/nonkube/client/fs"
	"github.com/skupperproject/skupper/pkg/apis/skupper/v2alpha1"
	"github.com/skupperproject/skupper/pkg/nonkube/api"
	"github.com/spf13/cobra"
	"gotest.tools/v3/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestCmdListenerDelete_ValidateInput(t *testing.T) {
	type test struct {
		name              string
		namespace         string
		args              []string
		flags             *common.CommandListenerDeleteFlags
		cobraGenericFlags map[string]string
		expectedError     string
	}

	if os.Getuid() == 0 {
		api.DefaultRootDataHome = t.TempDir()
	} else {
		t.Setenv("XDG_DATA_HOME", t.TempDir())
	}
	tmpDir := api.GetDataHome()
	path := filepath.Join(tmpDir, "/namespaces/test/", string(api.InputSiteStatePath))

	testTable := []test{
		{
			name:          "listener name is not specified",
			namespace:     "test",
			args:          []string{},
			flags:         &common.CommandListenerDeleteFlags{},
			expectedError: "listener name must be specified",
		},
		{
			name:          "listener name is nil",
			namespace:     "test",
			args:          []string{""},
			flags:         &common.CommandListenerDeleteFlags{},
			expectedError: "listener name must not be empty",
		},
		{
			name:          "listener name is not valid",
			namespace:     "test",
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
		{
			name:          "invalid namespace",
			namespace:     "TestInvalid",
			args:          []string{"my-listener"},
			flags:         &common.CommandListenerDeleteFlags{},
			expectedError: "namespace is not valid: value does not match this regular expression: ^[a-z0-9]([-a-z0-9]*[a-z0-9])?$",
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
			command.namespace = test.namespace

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
		name         string
		namespace    string
		deleteName   string
		errorMessage string
	}

	if os.Getuid() == 0 {
		api.DefaultRootDataHome = t.TempDir()
	} else {
		t.Setenv("XDG_DATA_HOME", t.TempDir())
	}
	tmpDir := api.GetDataHome()
	path := filepath.Join(tmpDir, "/namespaces/test/", string(api.InputSiteStatePath))

	testTable := []test{
		{
			name:         "run fails default",
			deleteName:   "my-listener",
			errorMessage: "no such file or directory",
		},
		{
			name:       "run ok",
			namespace:  "test",
			deleteName: "my-listener",
		},
	}

	for _, test := range testTable {
		cmd := &CmdListenerDelete{}

		t.Run(test.name, func(t *testing.T) {

			createListenerResource(path, t)
			cmd.listenerName = test.deleteName
			cmd.namespace = test.namespace
			cmd.listenerHandler = fs.NewListenerHandler(cmd.namespace)
			cmd.InputToOptions()

			err := cmd.Run()
			if test.errorMessage != "" {
				assert.Check(t, strings.HasSuffix(err.Error(), test.errorMessage))
			} else {
				assert.Check(t, err == nil)
			}
		})
	}
}

func createListenerResource(path string, t *testing.T) {
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

	listenerHandler := fs.NewListenerHandler("test")

	contentConnector, err := listenerHandler.EncodeToYaml(listenerResource)
	assert.Check(t, err == nil)
	err = listenerHandler.WriteFile(path, "my-listener.yaml", contentConnector, common.Listeners)
	assert.Check(t, err == nil)
}

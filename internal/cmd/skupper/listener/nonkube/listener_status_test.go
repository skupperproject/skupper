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
	"gotest.tools/v3/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestCmdListenerStatus_ValidateInput(t *testing.T) {
	type test struct {
		name              string
		args              []string
		flags             *common.CommandListenerStatusFlags
		cobraGenericFlags map[string]string
		expectedError     string
	}

	tmpDir := filepath.Join(t.TempDir(), "/skupper")
	err := os.Setenv("SKUPPER_OUTPUT_PATH", tmpDir)
	assert.Check(t, err == nil)
	path := filepath.Join(tmpDir, "/namespaces/test/", string(api.RuntimeSiteStatePath))

	testTable := []test{
		{
			name:          "listener is not shown because listener does not exist in the namespace",
			args:          []string{"no-listener"},
			flags:         &common.CommandListenerStatusFlags{},
			expectedError: "listener no-listener does not exist in namespace test",
		},
		{
			name:          "listener name is nil",
			args:          []string{""},
			flags:         &common.CommandListenerStatusFlags{},
			expectedError: "listener name must not be empty",
		},
		{
			name:          "more than one argument is specified",
			args:          []string{"my", "listener"},
			flags:         &common.CommandListenerStatusFlags{},
			expectedError: "only one argument is allowed for this command",
		},
		{
			name:          "listener name is not valid.",
			args:          []string{"my new listener"},
			flags:         &common.CommandListenerStatusFlags{},
			expectedError: "listener name is not valid: value does not match this regular expression: ^[a-z0-9]([-a-z0-9]*[a-z0-9])*(\\.[a-z0-9]([-a-z0-9]*[a-z0-9])*)*$",
		},
		{
			name:          "no args",
			flags:         &common.CommandListenerStatusFlags{},
			expectedError: "",
		},
		{
			name:          "bad output status",
			args:          []string{"my-listener"},
			flags:         &common.CommandListenerStatusFlags{Output: "not-supported"},
			expectedError: "output type is not valid: value not-supported not allowed. It should be one of this options: [json yaml]",
		},
		{
			name:          "good output status",
			args:          []string{"my-listener"},
			flags:         &common.CommandListenerStatusFlags{Output: "json"},
			expectedError: "",
		},
	}

	//Add a temp file so listener exists for status tests
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

	command := &CmdListenerStatus{}
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

func TestCmdListenerStatus_Run(t *testing.T) {
	type test struct {
		name         string
		listenerName string
		flags        common.CommandListenerStatusFlags
		errorMessage string
	}

	tmpDir := filepath.Join(t.TempDir(), "/skupper")
	err := os.Setenv("SKUPPER_OUTPUT_PATH", tmpDir)
	assert.Check(t, err == nil)
	path := filepath.Join(tmpDir, "/namespaces/test/", string(api.RuntimeSiteStatePath))

	testTable := []test{
		{
			name:         "run fails listener doesn't exist",
			listenerName: "no-listener",
			errorMessage: "no such file or directory",
		},
		{
			name:         "runs ok, returns 1 listeners",
			listenerName: "my-listener",
		},
		{
			name:         "runs ok, returns 1 listeners yaml",
			listenerName: "my-listener",
			flags:        common.CommandListenerStatusFlags{Output: "yaml"},
		},
		{
			name: "runs ok, returns all listeners",
		},
		{
			name:  "runs ok, returns all listeners json",
			flags: common.CommandListenerStatusFlags{Output: "json"},
		},
		{
			name:         "runs ok, returns all listeners output bad",
			flags:        common.CommandListenerStatusFlags{Output: "bad-value"},
			errorMessage: "format bad-value not supported",
		},
		{
			name:         "runs ok, returns 1 listeners bad output",
			listenerName: "my-listener",
			flags:        common.CommandListenerStatusFlags{Output: "bad-value"},
			errorMessage: "format bad-value not supported",
		},
	}

	//Add a temp file so listener exists for status tests
	listenerResource1 := v2alpha1.Listener{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "skupper.io/v2alpha1",
			Kind:       "Listener",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-listener",
			Namespace: "test",
		},
		Spec: v2alpha1.ListenerSpec{
			Host:       "1.2.3.4",
			Port:       8080,
			RoutingKey: "backend-8080",
		},
		Status: v2alpha1.ListenerStatus{
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
	listenerResource2 := v2alpha1.Listener{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "skupper.io/v2alpha1",
			Kind:       "Listener",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-listener2",
			Namespace: "test",
		},
		Spec: v2alpha1.ListenerSpec{
			Host:       "1.1.1.1",
			Port:       9999,
			RoutingKey: "test-9999",
		},
		Status: v2alpha1.ListenerStatus{
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

	// add two listeners in runtime directory
	command := &CmdListenerStatus{}
	command.namespace = "test"
	command.listenerHandler = fs.NewListenerHandler(command.namespace)

	defer command.listenerHandler.Delete("my-listener")
	defer command.listenerHandler.Delete("my-listener2")

	content, err := command.listenerHandler.EncodeToYaml(listenerResource1)
	assert.Check(t, err == nil)
	err = command.listenerHandler.WriteFile(path, "my-listener.yaml", content, common.Listeners)
	assert.Check(t, err == nil)

	content, err = command.listenerHandler.EncodeToYaml(listenerResource2)
	assert.Check(t, err == nil)
	err = command.listenerHandler.WriteFile(path, "my-listener2.yaml", content, common.Listeners)
	assert.Check(t, err == nil)

	for _, test := range testTable {
		command.listenerName = test.listenerName
		command.Flags = &test.flags
		command.output = command.Flags.Output

		t.Run(test.name, func(t *testing.T) {
			err := command.Run()
			if err != nil {
				assert.Check(t, strings.HasSuffix(err.Error(), test.errorMessage))
			} else {
				assert.Check(t, err == nil)
			}
		})
	}
}

func TestCmdListenerStatus_RunNoDirectory(t *testing.T) {
	type test struct {
		name         string
		flags        common.CommandListenerStatusFlags
		errorMessage string
	}

	tmpDir := filepath.Join(t.TempDir(), "/skupper")
	err := os.Setenv("SKUPPER_OUTPUT_PATH", tmpDir)
	assert.Check(t, err == nil)

	testTable := []test{
		{
			name:         "runs fails no directory",
			errorMessage: "no such file or directory",
		},
	}

	for _, test := range testTable {
		command := &CmdListenerStatus{}
		command.namespace = "test1"
		command.listenerHandler = fs.NewListenerHandler(command.namespace)
		command.listenerName = "my-listener"
		command.Flags = &test.flags
		command.output = command.Flags.Output
		t.Run(test.name, func(t *testing.T) {

			err := command.Run()
			if err != nil {
				assert.Check(t, strings.HasSuffix(err.Error(), test.errorMessage))
			} else {
				assert.Check(t, err == nil)
			}
		})
	}
}

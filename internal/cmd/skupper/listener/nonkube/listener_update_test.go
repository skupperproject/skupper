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
	"github.com/spf13/cobra"
	"gotest.tools/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestCmdListenerUpdate_ValidateInput(t *testing.T) {
	type test struct {
		name              string
		args              []string
		flags             *common.CommandListenerUpdateFlags
		k8sObjects        []runtime.Object
		skupperObjects    []runtime.Object
		cobraGenericFlags map[string]string
		expectedErrors    []string
	}

	homeDir, err := os.UserHomeDir()
	assert.Check(t, err == nil)
	path := filepath.Join(homeDir, "/.local/share/skupper/namespaces/test/", string(api.InputSiteStatePath))

	testTable := []test{
		{
			name:           "Listener is not updated because get listener returned error",
			args:           []string{"no-listener"},
			flags:          &common.CommandListenerUpdateFlags{},
			expectedErrors: []string{"listener no-listener must exist in namespace test to be updated"},
		},
		{
			name:           "listener name is not specified",
			args:           []string{},
			flags:          &common.CommandListenerUpdateFlags{},
			expectedErrors: []string{"listener name must be configured"},
		},
		{
			name:           "listener name is nil",
			args:           []string{""},
			flags:          &common.CommandListenerUpdateFlags{},
			expectedErrors: []string{"listener name must not be empty"},
		},
		{
			name:           "more than one argument is specified",
			args:           []string{"my", "listener"},
			flags:          &common.CommandListenerUpdateFlags{},
			expectedErrors: []string{"only one argument is allowed for this command"},
		},
		{
			name:           "listener name is not valid.",
			args:           []string{"my new listener"},
			flags:          &common.CommandListenerUpdateFlags{},
			expectedErrors: []string{"listener name is not valid: value does not match this regular expression: ^[a-z0-9]([-a-z0-9]*[a-z0-9])*(\\.[a-z0-9]([-a-z0-9]*[a-z0-9])*)*$"},
		},
		{
			name:           "listener type is not valid",
			args:           []string{"my-listener"},
			flags:          &common.CommandListenerUpdateFlags{ListenerType: "not-valid"},
			expectedErrors: []string{"listener type is not valid: value not-valid not allowed. It should be one of this options: [tcp]"},
		},
		{
			name:           "routing key is not valid",
			args:           []string{"my-listener"},
			flags:          &common.CommandListenerUpdateFlags{RoutingKey: "not-valid$"},
			expectedErrors: []string{"routing key is not valid: value does not match this regular expression: ^[a-z0-9]([-a-z0-9]*[a-z0-9])*(\\.[a-z0-9]([-a-z0-9]*[a-z0-9])*)*$"},
		},
		{
			name:           "port is not valid",
			args:           []string{"my-listener"},
			flags:          &common.CommandListenerUpdateFlags{Port: -1},
			expectedErrors: []string{"listener port is not valid: value is not positive"},
		},
		{
			name:           "output is not valid",
			args:           []string{"my-listener"},
			flags:          &common.CommandListenerUpdateFlags{Output: "not-supported"},
			expectedErrors: []string{"output type is not valid: value not-supported not allowed. It should be one of this options: [json yaml]"},
		},
		{
			name:  "kubernetes flags are not valid on this platform",
			args:  []string{"my-listener"},
			flags: &common.CommandListenerUpdateFlags{},
			cobraGenericFlags: map[string]string{
				common.FlagNameContext:    "test",
				common.FlagNameKubeconfig: "test",
			},
			expectedErrors: []string{},
		},
		{
			name: "flags all valid",
			args: []string{"my-listener"},
			flags: &common.CommandListenerUpdateFlags{
				RoutingKey:   "routingkeyname",
				TlsSecret:    "secretname",
				Port:         1234,
				ListenerType: "tcp",
				Output:       "json",
				Host:         "1.2.3.4",
			},
			expectedErrors: []string{},
		},
	}

	//TBD add a temp file so listener exists for update tests will pass
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

	command := &CmdListenerUpdate{Flags: &common.CommandListenerUpdateFlags{}}
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

			actualErrors := command.ValidateInput(test.args)

			actualErrorsMessages := utils.ErrorsToMessages(actualErrors)

			assert.DeepEqual(t, actualErrorsMessages, test.expectedErrors)

		})
	}
}

func TestCmdListenerUpdate_Run(t *testing.T) {
	type test struct {
		name                string
		namespace           string
		k8sObjects          []runtime.Object
		skupperObjects      []runtime.Object
		skupperErrorMessage string
		errorMessage        string
		listenerName        string
		host                string
		output              string
		routingKey          string
		tlsSecret           string
		listenerType        string
		port                int
	}

	testTable := []test{
		{
			name:         "runs ok",
			namespace:    "test",
			listenerName: "my-listener",
			port:         8080,
			listenerType: "tcp",
			host:         "hostname",
			routingKey:   "keyname",
			tlsSecret:    "secretname",
		},
		{
			name:         "run output json",
			listenerName: "my-listener",
			port:         8181,
			listenerType: "tcp",
			host:         "hostname",
			routingKey:   "keyname",
			tlsSecret:    "secretname",
			output:       "json",
		},
	}

	for _, test := range testTable {
		command := &CmdListenerUpdate{}

		command.listenerName = test.listenerName
		command.newSettings.output = test.output
		command.newSettings.port = test.port
		command.newSettings.host = test.host
		command.newSettings.routingKey = test.routingKey
		command.newSettings.tlsSecret = test.tlsSecret
		command.namespace = test.namespace
		command.listenerHandler = fs.NewListenerHandler(command.namespace)
		defer command.listenerHandler.Delete("my-listener")
		t.Run(test.name, func(t *testing.T) {
			command.InputToOptions()
			err := command.Run()
			if err != nil {
				assert.Check(t, test.errorMessage == err.Error(), err.Error())
			} else {
				assert.Check(t, err == nil)
			}
		})
	}
}

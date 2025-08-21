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

func TestCmdListenerUpdate_ValidateInput(t *testing.T) {
	type test struct {
		name              string
		namespace         string
		args              []string
		flags             *common.CommandListenerUpdateFlags
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
			name:          "Listener is not updated because get listener returned error",
			namespace:     "test",
			args:          []string{"no-listener"},
			flags:         &common.CommandListenerUpdateFlags{},
			expectedError: "listener no-listener must exist in namespace test to be updated",
		},
		{
			name:          "listener name is not specified",
			namespace:     "test",
			args:          []string{},
			flags:         &common.CommandListenerUpdateFlags{},
			expectedError: "listener name must be configured",
		},
		{
			name:          "listener name is nil",
			namespace:     "test",
			args:          []string{""},
			flags:         &common.CommandListenerUpdateFlags{},
			expectedError: "listener name must not be empty",
		},
		{
			name:          "more than one argument is specified",
			args:          []string{"my", "listener"},
			flags:         &common.CommandListenerUpdateFlags{},
			expectedError: "only one argument is allowed for this command",
		},
		{
			name:          "listener name is not valid.",
			args:          []string{"my new listener"},
			flags:         &common.CommandListenerUpdateFlags{},
			expectedError: "listener name is not valid: value does not match this regular expression: ^[a-z0-9]([-a-z0-9]*[a-z0-9])*(\\.[a-z0-9]([-a-z0-9]*[a-z0-9])*)*$",
		},
		{
			name:          "listener type is not valid",
			args:          []string{"my-listener"},
			flags:         &common.CommandListenerUpdateFlags{ListenerType: "not-valid"},
			expectedError: "listener type is not valid: value not-valid not allowed. It should be one of this options: [tcp]",
		},
		{
			name:          "routing key is not valid",
			args:          []string{"my-listener"},
			flags:         &common.CommandListenerUpdateFlags{RoutingKey: "not-valid$"},
			expectedError: "routing key is not valid: value does not match this regular expression: ^[a-z0-9]([-a-z0-9]*[a-z0-9])*(\\.[a-z0-9]([-a-z0-9]*[a-z0-9])*)*$",
		},
		{
			name:          "TlsCredentials key is not valid",
			args:          []string{"my-listener"},
			flags:         &common.CommandListenerUpdateFlags{TlsCredentials: "not-valid$", Host: "1.2.3.4"},
			expectedError: "tlsCredentials value is not valid: value does not match this regular expression: ^[a-z0-9]([-a-z0-9]*[a-z0-9])*(\\.[a-z0-9]([-a-z0-9]*[a-z0-9])*)*$",
		},
		{
			name:          "port is not valid",
			args:          []string{"my-listener"},
			flags:         &common.CommandListenerUpdateFlags{Port: -1},
			expectedError: "listener port is not valid: value is not positive",
		},
		{
			name:          "host is not valid",
			args:          []string{"my-listener"},
			flags:         &common.CommandListenerUpdateFlags{Host: "not-valid$"},
			expectedError: "host is not valid: a valid IP address or hostname is expected",
		},
		{
			name:  "kubernetes flags are not valid on this platform",
			args:  []string{"my-listener"},
			flags: &common.CommandListenerUpdateFlags{},
			cobraGenericFlags: map[string]string{
				common.FlagNameContext:    "test",
				common.FlagNameKubeconfig: "test",
			},
			expectedError: "",
		},
		{
			name:          "invalid namespace",
			namespace:     "TestInvalid",
			args:          []string{"my-connector"},
			flags:         &common.CommandListenerUpdateFlags{},
			expectedError: "namespace is not valid: value does not match this regular expression: ^[a-z0-9]([-a-z0-9]*[a-z0-9])?$\nlistener my-connector must exist in namespace TestInvalid to be updated",
		},
		{
			name: "flags all valid",
			args: []string{"my-listener"},
			flags: &common.CommandListenerUpdateFlags{
				RoutingKey:     "routingkeyname",
				TlsCredentials: "secretname",
				Port:           1234,
				ListenerType:   "tcp",
				Host:           "1.2.3.4",
			},
			expectedError: "",
		},
	}

	// Add a temp file so listener exists for update tests to pass
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
		routingKey          string
		tlsCredentials      string
		listenerType        string
		port                int
	}

	testTable := []test{
		{
			name:           "runs ok",
			namespace:      "test",
			listenerName:   "my-listener",
			port:           8080,
			listenerType:   "tcp",
			host:           "hostname",
			routingKey:     "keyname",
			tlsCredentials: "secretname",
		},
		{
			name:         "run ok no secret",
			listenerName: "my-listener",
			port:         8181,
			listenerType: "tcp",
			host:         "hostname",
			routingKey:   "keyname",
		},
	}

	for _, test := range testTable {
		command := &CmdListenerUpdate{}
		command.CobraCmd = &cobra.Command{Use: "test"}
		command.listenerName = test.listenerName
		command.newSettings.port = test.port
		command.newSettings.host = test.host
		command.newSettings.routingKey = test.routingKey
		command.newSettings.tlsCredentials = test.tlsCredentials
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

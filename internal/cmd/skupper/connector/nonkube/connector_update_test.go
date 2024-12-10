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
	"gotest.tools/v3/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestCmdConnectorUpdate_ValidateInput(t *testing.T) {
	type test struct {
		name              string
		args              []string
		flags             *common.CommandConnectorUpdateFlags
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
			name:           "connector is not updated because get connector returned error",
			args:           []string{"no-connector"},
			flags:          &common.CommandConnectorUpdateFlags{},
			expectedErrors: []string{"connector no-connector must exist in namespace test to be updated"},
		},
		{
			name:           "connector name is not specified",
			args:           []string{},
			flags:          &common.CommandConnectorUpdateFlags{},
			expectedErrors: []string{"connector name must be configured"},
		},
		{
			name:           "connector name is nil",
			args:           []string{""},
			flags:          &common.CommandConnectorUpdateFlags{},
			expectedErrors: []string{"connector name must not be empty"},
		},
		{
			name:           "more than one argument is specified",
			args:           []string{"my", "connector"},
			flags:          &common.CommandConnectorUpdateFlags{},
			expectedErrors: []string{"only one argument is allowed for this command"},
		},
		{
			name:           "connector name is not valid.",
			args:           []string{"my new connector"},
			flags:          &common.CommandConnectorUpdateFlags{},
			expectedErrors: []string{"connector name is not valid: value does not match this regular expression: ^[a-z0-9]([-a-z0-9]*[a-z0-9])*(\\.[a-z0-9]([-a-z0-9]*[a-z0-9])*)*$"},
		},
		{
			name:           "connector type is not valid",
			args:           []string{"my-connector"},
			flags:          &common.CommandConnectorUpdateFlags{ConnectorType: "not-valid"},
			expectedErrors: []string{"connector type is not valid: value not-valid not allowed. It should be one of this options: [tcp]"},
		},
		{
			name:           "routing key is not valid",
			args:           []string{"my-connector"},
			flags:          &common.CommandConnectorUpdateFlags{RoutingKey: "not-valid$"},
			expectedErrors: []string{"routing key is not valid: value does not match this regular expression: ^[a-z0-9]([-a-z0-9]*[a-z0-9])*(\\.[a-z0-9]([-a-z0-9]*[a-z0-9])*)*$"},
		},
		{
			name:           "tlsCredentials is not valid",
			args:           []string{"my-connector"},
			flags:          &common.CommandConnectorUpdateFlags{TlsCredentials: "not-valid$", Host: "1.2.3.4"},
			expectedErrors: []string{"tlsCredentials value is not valid: value does not match this regular expression: ^[a-z0-9]([-a-z0-9]*[a-z0-9])*(\\.[a-z0-9]([-a-z0-9]*[a-z0-9])*)*$"},
		},
		{
			name:           "host is not valid",
			args:           []string{"my-connector"},
			flags:          &common.CommandConnectorUpdateFlags{Host: "not-valid$"},
			expectedErrors: []string{"host is not valid: a valid IP address or hostname is expected"},
		},
		{
			name:           "port is not valid",
			args:           []string{"my-connector"},
			flags:          &common.CommandConnectorUpdateFlags{Port: -1},
			expectedErrors: []string{"connector port is not valid: value is not positive"},
		},
		{
			name:           "output is not valid",
			args:           []string{"my-connector"},
			flags:          &common.CommandConnectorUpdateFlags{Output: "not-supported"},
			expectedErrors: []string{"output type is not valid: value not-supported not allowed. It should be one of this options: [json yaml]"},
		},
		{
			name:  "kubernetes flags are not valid on this platform",
			args:  []string{"my-connector"},
			flags: &common.CommandConnectorUpdateFlags{},
			cobraGenericFlags: map[string]string{
				common.FlagNameContext:    "test",
				common.FlagNameKubeconfig: "test",
			},
			expectedErrors: []string{},
		},
		{
			name: "flags all valid",
			args: []string{"my-connector"},
			flags: &common.CommandConnectorUpdateFlags{
				RoutingKey:     "routingkeyname",
				TlsCredentials: "secretname",
				Port:           1234,
				ConnectorType:  "tcp",
				Output:         "json",
				Host:           "1.2.3.4",
			},
			expectedErrors: []string{},
		},
	}

	//Add a temp file so connector exists for update tests will pass
	connectorResource := v2alpha1.Connector{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "skupper.io/v2alpha1",
			Kind:       "Connector",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-connector",
			Namespace: "test",
		},
	}

	command := &CmdConnectorUpdate{Flags: &common.CommandConnectorUpdateFlags{}}
	command.CobraCmd = &cobra.Command{Use: "test"}
	command.namespace = "test"
	command.connectorHandler = fs.NewConnectorHandler(command.namespace)

	defer command.connectorHandler.Delete("my-connector")
	content, err := command.connectorHandler.EncodeToYaml(connectorResource)
	assert.Check(t, err == nil)
	err = command.connectorHandler.WriteFile(path, "my-connector.yaml", content, common.Connectors)
	assert.Check(t, err == nil)

	for _, test := range testTable {
		t.Run(test.name, func(t *testing.T) {
			command.connectorName = ""
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

func TestCmdConnectorUpdate_Run(t *testing.T) {
	type test struct {
		name                string
		namespace           string
		k8sObjects          []runtime.Object
		skupperObjects      []runtime.Object
		skupperErrorMessage string
		errorMessage        string
		connectorName       string
		host                string
		output              string
		routingKey          string
		tlsCredentials      string
		connectorType       string
		port                int
	}

	testTable := []test{
		{
			name:           "runs ok",
			namespace:      "test",
			connectorName:  "my-connector",
			port:           8080,
			connectorType:  "tcp",
			host:           "hostname",
			routingKey:     "keyname",
			tlsCredentials: "secretname",
		},
		{
			name:           "run output json",
			connectorName:  "my-connector",
			port:           8181,
			connectorType:  "tcp",
			host:           "hostname",
			routingKey:     "keyname",
			tlsCredentials: "secretname",
			output:         "json",
		},
	}

	for _, test := range testTable {
		command := &CmdConnectorUpdate{}

		command.connectorName = test.connectorName
		command.output = test.output
		command.newSettings.port = test.port
		command.newSettings.host = test.host
		command.newSettings.routingKey = test.routingKey
		command.newSettings.tlsCredentials = test.tlsCredentials
		command.namespace = test.namespace
		command.connectorHandler = fs.NewConnectorHandler(command.namespace)
		defer command.connectorHandler.Delete("my-connector")
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

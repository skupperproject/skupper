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
)

func TestCmdConnectorUpdate_ValidateInput(t *testing.T) {
	type test struct {
		name              string
		namespace         string
		args              []string
		flags             *common.CommandConnectorUpdateFlags
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
			name:          "connector is not updated because get connector returned error",
			namespace:     "test",
			args:          []string{"no-connector"},
			flags:         &common.CommandConnectorUpdateFlags{Host: "1.2.3.4"},
			expectedError: "connector no-connector must exist in namespace test to be updated",
		},
		{
			name:          "connector name is not specified",
			namespace:     "test",
			args:          []string{},
			flags:         &common.CommandConnectorUpdateFlags{Host: "localhost"},
			expectedError: "connector name must be configured",
		},
		{
			name:          "connector name is nil",
			args:          []string{""},
			flags:         &common.CommandConnectorUpdateFlags{Host: "localhost"},
			expectedError: "connector name must not be empty",
		},
		{
			name:          "more than one argument is specified",
			args:          []string{"my", "connector"},
			flags:         &common.CommandConnectorUpdateFlags{Host: "localhost"},
			expectedError: "only one argument is allowed for this command",
		},
		{
			name:          "connector name is not valid.",
			args:          []string{"my new connector"},
			flags:         &common.CommandConnectorUpdateFlags{Host: "localhost"},
			expectedError: "connector name is not valid: value does not match this regular expression: ^[a-z0-9]([-a-z0-9]*[a-z0-9])*(\\.[a-z0-9]([-a-z0-9]*[a-z0-9])*)*$",
		},
		{
			name:          "connector type is not valid",
			args:          []string{"my-connector"},
			flags:         &common.CommandConnectorUpdateFlags{ConnectorType: "not-valid", Host: "localhost"},
			expectedError: "connector type is not valid: value not-valid not allowed. It should be one of this options: [tcp]",
		},
		{
			name:          "routing key is not valid",
			args:          []string{"my-connector"},
			flags:         &common.CommandConnectorUpdateFlags{RoutingKey: "not-valid$", Host: "localhost"},
			expectedError: "routing key is not valid: value does not match this regular expression: ^[a-z0-9]([-a-z0-9]*[a-z0-9])*(\\.[a-z0-9]([-a-z0-9]*[a-z0-9])*)*$",
		},
		{
			name:          "tlsCredentials is not valid",
			args:          []string{"my-connector"},
			flags:         &common.CommandConnectorUpdateFlags{TlsCredentials: "not-valid$", Host: "1.2.3.4"},
			expectedError: "tlsCredentials value is not valid: value does not match this regular expression: ^[a-z0-9]([-a-z0-9]*[a-z0-9])*(\\.[a-z0-9]([-a-z0-9]*[a-z0-9])*)*$",
		},
		{
			name:          "host is not valid",
			args:          []string{"my-connector"},
			flags:         &common.CommandConnectorUpdateFlags{Host: "not-valid$"},
			expectedError: "host is not valid: a valid IP address or hostname is expected",
		},
		{
			name:          "port is not valid",
			args:          []string{"my-connector"},
			flags:         &common.CommandConnectorUpdateFlags{Port: -1, Host: "localhost"},
			expectedError: "connector port is not valid: value is not positive",
		},
		{
			name:  "kubernetes flags are not valid on this platform",
			args:  []string{"my-connector"},
			flags: &common.CommandConnectorUpdateFlags{Host: "localhost"},
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
			flags:         &common.CommandConnectorUpdateFlags{Host: "localhost"},
			expectedError: "namespace is not valid: value does not match this regular expression: ^[a-z0-9]([-a-z0-9]*[a-z0-9])?$",
		},
		{
			name:      "flags all valid",
			namespace: "test",
			args:      []string{"my-connector"},
			flags: &common.CommandConnectorUpdateFlags{
				RoutingKey:     "routingkeyname",
				TlsCredentials: "secretname",
				Port:           1234,
				ConnectorType:  "tcp",
				Host:           "1.2.3.4",
			},
			expectedError: "",
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
		Spec: v2alpha1.ConnectorSpec{
			Host:       "localhost",
			Port:       8080,
			RoutingKey: "backend-8080",
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

func TestCmdConnectorUpdate_Run(t *testing.T) {
	type test struct {
		name           string
		namespace      string
		errorMessage   string
		connectorName  string
		host           string
		routingKey     string
		tlsCredentials string
		connectorType  string
		port           int
	}

	testTable := []test{
		{
			name:           "runs ok",
			namespace:      "test",
			connectorName:  "my-connector",
			port:           8080,
			connectorType:  "tcp",
			host:           "1.2.3.4",
			routingKey:     "keyname",
			tlsCredentials: "secretname",
		},
		{
			name:           "runs default namespace",
			connectorName:  "my-connector",
			port:           8080,
			connectorType:  "tcp",
			host:           "localhost",
			routingKey:     "keyname",
			tlsCredentials: "secretname",
		},
	}

	for _, test := range testTable {
		command := &CmdConnectorUpdate{}

		command.connectorName = test.connectorName
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

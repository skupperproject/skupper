package kube

import (
	"testing"

	"github.com/skupperproject/skupper/internal/cmd/skupper/common"
	"github.com/skupperproject/skupper/internal/cmd/skupper/common/testutils"

	"gotest.tools/v3/assert"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestCmdConnectorGenerate_ValidateInput(t *testing.T) {
	type test struct {
		name           string
		args           []string
		flags          common.CommandConnectorGenerateFlags
		k8sObjects     []runtime.Object
		skupperObjects []runtime.Object
		expectedError  string
	}

	testTable := []test{
		{
			name: "connector name and port are not specified",
			args: []string{},
			flags: common.CommandConnectorGenerateFlags{
				Selector: "backend",
			},
			expectedError: "connector name and port must be configured",
		},
		{
			name: "connector name empty",
			args: []string{"", "8090"},
			flags: common.CommandConnectorGenerateFlags{
				Selector: "backend",
			},
			expectedError: "connector name must not be empty",
		},
		{
			name: "connector port empty",
			args: []string{"my-name-port-empty", ""},
			flags: common.CommandConnectorGenerateFlags{
				Selector: "backend",
			},
			expectedError: "connector port must not be empty",
		},
		{
			name: "connector port not positive",
			args: []string{"my-port-positive", "-45"},
			flags: common.CommandConnectorGenerateFlags{
				Selector: "backend",
			},
			expectedError: "connector port is not valid: value is not positive",
		},
		{
			name: "connector name and port are not specified",
			args: []string{},
			flags: common.CommandConnectorGenerateFlags{
				Selector: "backend",
			},
			expectedError: "connector name and port must be configured",
		},
		{
			name: "connector port is not specified",
			args: []string{"my-name"},
			flags: common.CommandConnectorGenerateFlags{
				Selector: "backend",
			},
			expectedError: "connector name and port must be configured",
		},
		{
			name: "more than two arguments are specified",
			args: []string{"my", "connector", "8080"},
			flags: common.CommandConnectorGenerateFlags{
				Selector: "backend",
			},
			expectedError: "only two arguments are allowed for this command",
		},
		{
			name: "connector name is not valid.",
			args: []string{"my new connector", "8080"},
			flags: common.CommandConnectorGenerateFlags{
				Selector: "backend",
			},
			expectedError: "connector name is not valid: value does not match this regular expression: ^[a-z0-9]([-a-z0-9]*[a-z0-9])*(\\.[a-z0-9]([-a-z0-9]*[a-z0-9])*)*$",
		},
		{
			name: "port is not valid.",
			args: []string{"my-connector-port", "abcd"},
			flags: common.CommandConnectorGenerateFlags{
				Selector: "backend",
			},
			expectedError: "connector port is not valid: strconv.Atoi: parsing \"abcd\": invalid syntax",
		},
		{
			name: "connector type is not valid",
			args: []string{"my-connector-type", "8080"},
			flags: common.CommandConnectorGenerateFlags{
				ConnectorType: "not-valid",
				Selector:      "backend",
			},
			expectedError: "connector type is not valid: value not-valid not allowed. It should be one of this options: [tcp]",
		},
		{
			name: "routing key is not valid",
			args: []string{"my-connector-rk", "8080"},
			flags: common.CommandConnectorGenerateFlags{
				RoutingKey: "not-valid$",
				Selector:   "backend",
			},
			expectedError: "routing key is not valid: value does not match this regular expression: ^[a-z0-9]([-a-z0-9]*[a-z0-9])*(\\.[a-z0-9]([-a-z0-9]*[a-z0-9])*)*$",
		},
		{
			name: "tls-credentials does not exist",
			args: []string{"my-connector-tls", "8080"},
			flags: common.CommandConnectorGenerateFlags{
				TlsCredentials: "not-$valid",
				Selector:       "backend",
			},
			expectedError: "tlsCredentials is not valid: value does not match this regular expression: ^[a-z0-9]([-a-z0-9]*[a-z0-9])*(\\.[a-z0-9]([-a-z0-9]*[a-z0-9])*)*$",
		},
		{
			name: "workload is not valid",
			args: []string{"bad-workload", "1234"},
			flags: common.CommandConnectorGenerateFlags{
				Workload: "@345",
			},
			expectedError: "workload is not valid: workload must include <resource-type>/<resource-name>",
		},
		{
			name: "workload bad resourceType",
			args: []string{"bad-workload", "1234"},
			flags: common.CommandConnectorGenerateFlags{
				Workload: "bad/backend",
			},
			expectedError: "workload is not valid: resource-type does not match expected value: deployment/service/daemonset/statefulset",
		},
		{
			name: "selector is not valid",
			args: []string{"bad-selector", "1234"},
			flags: common.CommandConnectorGenerateFlags{
				Selector: "@#$%",
			},
			expectedError: "selector is not valid: value does not match this regular expression: ^[A-Za-z0-9=:./-]+$",
		},
		{
			name: "host is not valid",
			args: []string{"my-connector-host", "8080"},
			flags: common.CommandConnectorGenerateFlags{
				Host: "not-valid$"},
			expectedError: "host is not valid: a valid IP address or hostname is expected",
		},
		{
			name: "output is not valid",
			args: []string{"bad-output", "1234"},
			flags: common.CommandConnectorGenerateFlags{
				Host:   "host",
				Output: "not-supported",
			},
			expectedError: "output type is not valid: value not-supported not allowed. It should be one of this options: [json yaml]",
		},
		{
			name: "selector/host",
			args: []string{"selector", "1234"},
			flags: common.CommandConnectorGenerateFlags{
				Output:   "json",
				Selector: "app=test",
				Host:     "test",
			},
			expectedError: "If host is configured, cannot configure workload or selector\n" +
				"If selector is configured, cannot configure workload or host",
		},
		{
			name: "workload/host",
			args: []string{"workload", "1234"},
			flags: common.CommandConnectorGenerateFlags{
				Output:   "json",
				Workload: "deployment/test",
				Host:     "test",
			},
			expectedError: "If host is configured, cannot configure workload or selector\n" +
				"If workload is configured, cannot configure selector or host",
		},
	}

	for _, test := range testTable {
		t.Run(test.name, func(t *testing.T) {

			command, err := newCmdConnectorGenerateWithMocks("test", test.k8sObjects, test.skupperObjects, "")
			assert.Assert(t, err)

			command.Flags = &test.flags

			testutils.CheckValidateInput(t, command, test.expectedError, test.args)

		})
	}
}
func TestCmdConnectorGenerate_InputToOptions(t *testing.T) {

	type test struct {
		name                   string
		flags                  common.CommandConnectorGenerateFlags
		Connectorname          string
		expectedTlsCredentials string
		expectedHost           string
		expectedSelector       string
		expectedRoutingKey     string
		expectedConnectorType  string
		expectedOutput         string
	}

	testTable := []test{
		{
			name:                   "test1",
			flags:                  common.CommandConnectorGenerateFlags{"backend", "", "app=backend", "secret", "tcp", true, "", "json"},
			expectedTlsCredentials: "secret",
			expectedHost:           "",
			expectedRoutingKey:     "backend",
			expectedConnectorType:  "tcp",
			expectedOutput:         "json",
			expectedSelector:       "app=backend",
		},
		{
			name:                   "test2",
			flags:                  common.CommandConnectorGenerateFlags{"backend", "backend", "", "secret", "tcp", true, "", "json"},
			expectedTlsCredentials: "secret",
			expectedHost:           "backend",
			expectedRoutingKey:     "backend",
			expectedConnectorType:  "tcp",
			expectedOutput:         "json",
			expectedSelector:       "",
		},
		{
			name:                   "test3",
			flags:                  common.CommandConnectorGenerateFlags{"", "", "", "secret", "tcp", false, "", "yaml"},
			expectedTlsCredentials: "secret",
			expectedHost:           "",
			expectedRoutingKey:     "test3",
			expectedConnectorType:  "tcp",
			expectedOutput:         "yaml",
			expectedSelector:       "app=test3",
		},
	}

	for _, test := range testTable {
		t.Run(test.name, func(t *testing.T) {

			cmd, err := newCmdConnectorGenerateWithMocks("test", nil, nil, "")
			assert.Assert(t, err)

			cmd.Flags = &test.flags
			cmd.name = test.name

			cmd.InputToOptions()

			assert.Check(t, cmd.routingKey == test.expectedRoutingKey)
			assert.Check(t, cmd.output == test.expectedOutput)
			assert.Check(t, cmd.tlsCredentials == test.expectedTlsCredentials)
			assert.Check(t, cmd.host == test.expectedHost)
			assert.Check(t, cmd.selector == test.expectedSelector)
			assert.Check(t, cmd.connectorType == test.expectedConnectorType)
		})
	}
}

func TestCmdConnectorGenerate_Run(t *testing.T) {
	type test struct {
		name                string
		connectorName       string
		connectorPort       int
		flags               common.CommandConnectorGenerateFlags
		k8sObjects          []runtime.Object
		skupperObjects      []runtime.Object
		skupperErrorMessage string
		errorMessage        string
	}

	testTable := []test{
		{
			name:          "runs ok yaml",
			connectorName: "my-connector-ok",
			connectorPort: 8080,
			flags: common.CommandConnectorGenerateFlags{
				ConnectorType:       "tcp",
				RoutingKey:          "keyname",
				TlsCredentials:      "secretname",
				IncludeNotReadyPods: true,
				Selector:            "app=backend",
				Output:              "yaml",
			},
		},
		{
			name:          "run ok json",
			connectorName: "my-connector-json",
			connectorPort: 8080,
			flags: common.CommandConnectorGenerateFlags{
				ConnectorType:       "tcp",
				Host:                "hostname",
				RoutingKey:          "keyname",
				TlsCredentials:      "secretname",
				IncludeNotReadyPods: true,
				Output:              "json",
			},
		},
	}

	for _, test := range testTable {
		cmd, err := newCmdConnectorGenerateWithMocks("test", test.k8sObjects, test.skupperObjects, test.skupperErrorMessage)
		assert.Assert(t, err)

		t.Run(test.name, func(t *testing.T) {

			cmd.Flags = &common.CommandConnectorGenerateFlags{}
			cmd.name = test.connectorName
			cmd.port = test.connectorPort
			cmd.output = test.flags.Output
			cmd.namespace = "test"

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

func newCmdConnectorGenerateWithMocks(namespace string, k8sObjects []runtime.Object, skupperObjects []runtime.Object, fakeSkupperError string) (*CmdConnectorGenerate, error) {

	cmdConnectorGenerate := &CmdConnectorGenerate{
		namespace: namespace,
	}
	return cmdConnectorGenerate, nil
}

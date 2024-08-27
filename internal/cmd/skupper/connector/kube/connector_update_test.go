package kube

import (
	"testing"
	"time"

	"github.com/skupperproject/skupper/internal/cmd/skupper/utils"
	fakeclient "github.com/skupperproject/skupper/internal/kube/client/fake"
	"github.com/skupperproject/skupper/pkg/apis/skupper/v1alpha1"
	"github.com/spf13/pflag"
	"gotest.tools/assert"
	v12 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestCmdConnectorUpdate_NewCmdConnectorUpdate(t *testing.T) {

	t.Run("Update command", func(t *testing.T) {

		result := NewCmdConnectorUpdate()

		assert.Check(t, result.CobraCmd.Use != "")
		assert.Check(t, result.CobraCmd.Short != "")
		assert.Check(t, result.CobraCmd.Long != "")
		assert.Check(t, result.CobraCmd.Example != "")
		assert.Check(t, result.CobraCmd.PreRun != nil)
		assert.Check(t, result.CobraCmd.Run != nil)
		assert.Check(t, result.CobraCmd.PostRunE != nil)
		assert.Check(t, result.CobraCmd.Flags() != nil)
	})

}

func TestCmdConnectorUpdate_AddFlags(t *testing.T) {

	expectedFlagsWithDefaultValue := map[string]interface{}{
		"routing-key":       "",
		"host":              "",
		"tls-secret":        "",
		"type":              "tcp",
		"port":              "0",
		"workload":          "",
		"selector":          "",
		"include-not-ready": "false",
		"timeout":           "1m0s",
		"output":            "",
	}
	var flagList []string

	cmd, err := newCmdConnectorUpdateWithMocks("test", nil, nil, "")
	assert.Assert(t, err)

	t.Run("add flags", func(t *testing.T) {

		cmd.CobraCmd.Flags().VisitAll(func(flag *pflag.Flag) {
			flagList = append(flagList, flag.Name)
		})

		assert.Check(t, len(flagList) == 0)

		cmd.AddFlags()

		cmd.CobraCmd.Flags().VisitAll(func(flag *pflag.Flag) {
			flagList = append(flagList, flag.Name)
			assert.Check(t, expectedFlagsWithDefaultValue[flag.Name] != nil)
			assert.Check(t, expectedFlagsWithDefaultValue[flag.Name] == flag.DefValue)
		})

		assert.Check(t, len(flagList) == len(expectedFlagsWithDefaultValue))

	})
}

func TestCmdConnectorUpdate_ValidateInput(t *testing.T) {
	type test struct {
		name           string
		args           []string
		flags          ConnectorUpdates
		k8sObjects     []runtime.Object
		skupperObjects []runtime.Object
		expectedErrors []string
	}

	testTable := []test{
		{
			name:           "connector is not updated because get connector returned error",
			args:           []string{"my-connector"},
			flags:          ConnectorUpdates{timeout: 1 * time.Second},
			expectedErrors: []string{"connector my-connector must exist in namespace test to be updated"},
		},
		{
			name:           "connector name is not specified",
			args:           []string{},
			flags:          ConnectorUpdates{timeout: 1 * time.Second},
			expectedErrors: []string{"connector name must be configured"},
		},
		{
			name:           "connector name is nil",
			args:           []string{""},
			flags:          ConnectorUpdates{timeout: 1 * time.Minute},
			expectedErrors: []string{"connector name must not be empty"},
		},
		{
			name:           "more than one argument is specified",
			args:           []string{"my", "connector"},
			flags:          ConnectorUpdates{timeout: 1 * time.Minute},
			expectedErrors: []string{"only one argument is allowed for this command"},
		},
		{
			name:           "connector name is not valid.",
			args:           []string{"my new connector"},
			flags:          ConnectorUpdates{timeout: 1 * time.Minute},
			expectedErrors: []string{"connector name is not valid: value does not match this regular expression: ^[a-z0-9]([-a-z0-9]*[a-z0-9])*(\\.[a-z0-9]([-a-z0-9]*[a-z0-9])*)*$"},
		},
		{
			name: "connector type is not valid",
			args: []string{"my-connector-type"},
			flags: ConnectorUpdates{
				connectorType: "not-valid",
				timeout:       1 * time.Minute,
			},
			skupperObjects: []runtime.Object{
				&v1alpha1.Connector{
					ObjectMeta: v1.ObjectMeta{
						Name:      "my-connector-type",
						Namespace: "test",
					},
					Status: v1alpha1.ConnectorStatus{
						Status: v1alpha1.Status{
							Conditions: []v1.Condition{
								{
									Type:   "Configured",
									Status: "True",
								},
							},
						},
					},
				},
			},
			expectedErrors: []string{
				"connector type is not valid: value not-valid not allowed. It should be one of this options: [tcp]"},
		},
		{
			name: "routing key is not valid",
			args: []string{"my-connector-rk"},
			flags: ConnectorUpdates{
				routingKey: "not-valid$",
				timeout:    60 * time.Second,
			},
			skupperObjects: []runtime.Object{
				&v1alpha1.Connector{
					ObjectMeta: v1.ObjectMeta{
						Name:      "my-connector-rk",
						Namespace: "test",
					},
					Status: v1alpha1.ConnectorStatus{
						Status: v1alpha1.Status{
							Conditions: []v1.Condition{
								{
									Type:   "Configured",
									Status: "True",
								},
							},
						},
					},
				},
			},
			expectedErrors: []string{
				"routing key is not valid: value does not match this regular expression: ^[a-z0-9]([-a-z0-9]*[a-z0-9])*(\\.[a-z0-9]([-a-z0-9]*[a-z0-9])*)*$"},
		},
		{
			name: "tls-secret is not valid",
			args: []string{"my-connector-tls"},
			flags: ConnectorUpdates{
				tlsSecret: "test-tls",
				timeout:   5 * time.Second,
			},
			skupperObjects: []runtime.Object{
				&v1alpha1.Connector{
					ObjectMeta: v1.ObjectMeta{
						Name:      "my-connector-tls",
						Namespace: "test",
					},
					Status: v1alpha1.ConnectorStatus{
						Status: v1alpha1.Status{
							Conditions: []v1.Condition{
								{
									Type:   "Configured",
									Status: "True",
								},
							},
						},
					},
				},
			},
			expectedErrors: []string{"tls-secret is not valid: does not exist"},
		},
		{
			name: "port is not valid",
			args: []string{"my-connector-port"},
			flags: ConnectorUpdates{
				port:    -1,
				timeout: 40 * time.Second,
			},
			skupperObjects: []runtime.Object{
				&v1alpha1.Connector{
					ObjectMeta: v1.ObjectMeta{
						Name:      "my-connector-port",
						Namespace: "test",
					},
					Status: v1alpha1.ConnectorStatus{
						Status: v1alpha1.Status{
							Conditions: []v1.Condition{
								{
									Type:   "Configured",
									Status: "True",
								},
							},
						},
					},
				},
			},
			expectedErrors: []string{
				"connector port is not valid: value is not positive"},
		},
		{
			name: "workload is not valid",
			args: []string{"bad-workload"},
			flags: ConnectorUpdates{
				workload: "!workload",
				timeout:  70 * time.Second,
			},
			skupperObjects: []runtime.Object{
				&v1alpha1.Connector{
					ObjectMeta: v1.ObjectMeta{
						Name:      "bad-workload",
						Namespace: "test",
					},
					Status: v1alpha1.ConnectorStatus{
						Status: v1alpha1.Status{
							Conditions: []v1.Condition{
								{
									Type:   "Configured",
									Status: "True",
								},
							},
						},
					},
				},
			},
			expectedErrors: []string{
				"workload is not valid: value does not match this regular expression: ^[A-Za-z0-9=:./-]+$"},
		},
		{
			name: "selector is not valid",
			args: []string{"bad-selector"},
			flags: ConnectorUpdates{
				selector: "@#$%",
				timeout:  1 * time.Minute,
			},
			skupperObjects: []runtime.Object{
				&v1alpha1.Connector{
					ObjectMeta: v1.ObjectMeta{
						Name:      "bad-selector",
						Namespace: "test",
					},
					Status: v1alpha1.ConnectorStatus{
						Status: v1alpha1.Status{
							Conditions: []v1.Condition{
								{
									Type:   "Configured",
									Status: "True",
								},
							},
						},
					},
				},
			},
			expectedErrors: []string{
				"selector is not valid: value does not match this regular expression: ^[A-Za-z0-9=:./-]+$"},
		},
		{
			name: "timeout is not valid",
			args: []string{"bad-timeout"},
			flags: ConnectorUpdates{
				selector: "selector",
				timeout:  0 * time.Second,
			},
			skupperObjects: []runtime.Object{
				&v1alpha1.Connector{
					ObjectMeta: v1.ObjectMeta{
						Name:      "bad-timeout",
						Namespace: "test",
					},
					Status: v1alpha1.ConnectorStatus{
						Status: v1alpha1.Status{
							Conditions: []v1.Condition{
								{
									Type:   "Configured",
									Status: "True",
								},
							},
						},
					},
				},
			},
			expectedErrors: []string{"timeout is not valid"},
		},
		{
			name: "output is not valid",
			args: []string{"bad-output"},
			flags: ConnectorUpdates{
				output:  "not-supported",
				timeout: 1 * time.Minute,
			},
			skupperObjects: []runtime.Object{
				&v1alpha1.Connector{
					ObjectMeta: v1.ObjectMeta{
						Name:      "bad-output",
						Namespace: "test",
					},
					Status: v1alpha1.ConnectorStatus{
						Status: v1alpha1.Status{
							Conditions: []v1.Condition{
								{
									Type:   "Configured",
									Status: "True",
								},
							},
						},
					},
				},
			},
			expectedErrors: []string{
				"output type is not valid: value not-supported not allowed. It should be one of this options: [json yaml]"},
		},
		{
			name: "flags all valid",
			args: []string{"my-connector-flags"},
			flags: ConnectorUpdates{
				host:            "hostname",
				routingKey:      "routingkeyname",
				tlsSecret:       "secretname",
				port:            1234,
				connectorType:   "tcp",
				selector:        "backend",
				includeNotReady: false,
				timeout:         5 * time.Second,
				output:          "json",
			},
			skupperObjects: []runtime.Object{
				&v1alpha1.Connector{
					ObjectMeta: v1.ObjectMeta{
						Name:      "my-connector-flags",
						Namespace: "test",
					},
					Status: v1alpha1.ConnectorStatus{
						Status: v1alpha1.Status{
							Conditions: []v1.Condition{
								{
									Type:   "Configured",
									Status: "True",
								},
							},
						},
					},
				},
			},
			k8sObjects: []runtime.Object{
				&v12.Secret{
					ObjectMeta: v1.ObjectMeta{
						Name:      "secretname",
						Namespace: "test",
					},
				},
			},
			expectedErrors: []string{},
		},
	}

	for _, test := range testTable {
		t.Run(test.name, func(t *testing.T) {

			command, err := newCmdConnectorUpdateWithMocks("test", test.k8sObjects, test.skupperObjects, "")
			assert.Assert(t, err)

			command.flags = test.flags

			actualErrors := command.ValidateInput(test.args)

			actualErrorsMessages := utils.ErrorsToMessages(actualErrors)

			assert.DeepEqual(t, actualErrorsMessages, test.expectedErrors)

		})
	}
}

func TestCmdConnectorUpdate_Run(t *testing.T) {
	type test struct {
		name                string
		connectorName       string
		flags               ConnectorUpdates
		k8sObjects          []runtime.Object
		skupperObjects      []runtime.Object
		skupperErrorMessage string
		errorMessage        string
	}

	testTable := []test{
		{
			name:          "runs ok",
			connectorName: "my-connector-ok",
			flags: ConnectorUpdates{
				port:            8080,
				connectorType:   "tcp",
				host:            "hostname",
				routingKey:      "keyname",
				tlsSecret:       "secretname",
				includeNotReady: true,
				workload:        "deployment/backend",
				selector:        "backend",
				timeout:         1 * time.Second,
			},
			skupperObjects: []runtime.Object{
				&v1alpha1.Connector{
					ObjectMeta: v1.ObjectMeta{
						Name:      "my-connector-ok",
						Namespace: "test",
					},
					Status: v1alpha1.ConnectorStatus{
						Status: v1alpha1.Status{
							Conditions: []v1.Condition{
								{
									Type:   "Configured",
									Status: "True",
								},
							},
						},
					},
				},
			},
		},
		{
			name:          "run output json",
			connectorName: "my-connector-json",
			flags: ConnectorUpdates{
				port:            8181,
				connectorType:   "tcp",
				host:            "hostname",
				routingKey:      "keyname",
				tlsSecret:       "secretname",
				includeNotReady: true,
				workload:        "deployment/backend",
				selector:        "backend",
				timeout:         1 * time.Second,
				output:          "json",
			},
			skupperObjects: []runtime.Object{
				&v1alpha1.Connector{
					ObjectMeta: v1.ObjectMeta{
						Name:      "my-connector-json",
						Namespace: "test",
					},
					Status: v1alpha1.ConnectorStatus{
						Status: v1alpha1.Status{
							Conditions: []v1.Condition{
								{
									Type:   "Configured",
									Status: "True",
								},
							},
						},
					},
				},
			},
		},
	}

	for _, test := range testTable {
		cmd, err := newCmdConnectorUpdateWithMocks("test", test.k8sObjects, test.skupperObjects, test.skupperErrorMessage)
		assert.Assert(t, err)

		t.Run(test.name, func(t *testing.T) {

			cmd.name = test.connectorName
			cmd.newSettings = test.flags
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

func TestCmdConnectorUpdate_WaitUntil(t *testing.T) {
	type test struct {
		name                string
		output              string
		k8sObjects          []runtime.Object
		skupperObjects      []runtime.Object
		skupperErrorMessage string
		expectError         bool
	}

	testTable := []test{
		{
			name: "connector is not ready",
			skupperObjects: []runtime.Object{
				&v1alpha1.Connector{
					ObjectMeta: v1.ObjectMeta{
						Name:      "my-connector",
						Namespace: "test",
					},
					Status: v1alpha1.ConnectorStatus{},
				},
			},
			expectError: true,
		},
		{
			name:        "connector is not returned",
			expectError: true,
		},
		{
			name: "connector is ready",
			skupperObjects: []runtime.Object{
				&v1alpha1.Connector{
					ObjectMeta: v1.ObjectMeta{
						Name:      "my-connector",
						Namespace: "test",
					},
					Status: v1alpha1.ConnectorStatus{
						Status: v1alpha1.Status{
							Conditions: []v1.Condition{
								{
									Type:   "Configured",
									Status: "True",
								},
							},
						},
					},
				},
			},
			expectError: false,
		},
		{
			name:   "connector is ready json output",
			output: "json",
			skupperObjects: []runtime.Object{
				&v1alpha1.Connector{
					ObjectMeta: v1.ObjectMeta{
						Name:      "my-connector",
						Namespace: "test",
					},
					Status: v1alpha1.ConnectorStatus{
						Status: v1alpha1.Status{
							Conditions: []v1.Condition{
								{
									Type:   "Configured",
									Status: "True",
								},
							},
						},
					},
				},
			},
			expectError: false,
		},
	}

	for _, test := range testTable {
		cmd, err := newCmdConnectorUpdateWithMocks("test", test.k8sObjects, test.skupperObjects, test.skupperErrorMessage)
		assert.Assert(t, err)

		cmd.name = "my-connector"
		cmd.flags = ConnectorUpdates{
			timeout: 1 * time.Second,
			output:  test.output,
		}
		cmd.namespace = "test"
		cmd.newSettings.output = cmd.flags.output

		t.Run(test.name, func(t *testing.T) {

			err := cmd.WaitUntil()
			if test.expectError {
				assert.Check(t, err != nil)
			} else {
				assert.Assert(t, err)
			}
		})
	}
}

// --- helper methods

func newCmdConnectorUpdateWithMocks(namespace string, k8sObjects []runtime.Object, skupperObjects []runtime.Object, fakeSkupperError string) (*CmdConnectorUpdate, error) {

	// We make sure the interval is appropriate
	utils.SetRetryProfile(utils.TestRetryProfile)
	client, err := fakeclient.NewFakeClient(namespace, k8sObjects, skupperObjects, fakeSkupperError)
	if err != nil {
		return nil, err
	}
	cmdConnectorUpdate := &CmdConnectorUpdate{
		client:     client.GetSkupperClient().SkupperV1alpha1(),
		KubeClient: client.GetKubeClient(),
		namespace:  namespace,
	}
	return cmdConnectorUpdate, nil
}

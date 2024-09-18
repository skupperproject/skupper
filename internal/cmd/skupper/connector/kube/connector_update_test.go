package kube

import (
	"testing"
	"time"

	"github.com/skupperproject/skupper/internal/cmd/skupper/common"
	"github.com/skupperproject/skupper/internal/cmd/skupper/common/utils"

	fakeclient "github.com/skupperproject/skupper/internal/kube/client/fake"
	"github.com/skupperproject/skupper/pkg/apis/skupper/v1alpha1"
	"gotest.tools/assert"
	v12 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestCmdConnectorUpdate_ValidateInput(t *testing.T) {
	type test struct {
		name           string
		args           []string
		flags          common.CommandConnectorUpdateFlags
		k8sObjects     []runtime.Object
		skupperObjects []runtime.Object
		expectedErrors []string
	}

	testTable := []test{
		{
			name:           "connector is not updated because get connector returned error",
			args:           []string{"my-connector"},
			flags:          common.CommandConnectorUpdateFlags{Timeout: 1 * time.Second},
			expectedErrors: []string{"connector my-connector must exist in namespace test to be updated"},
		},
		{
			name:           "connector name is not specified",
			args:           []string{},
			flags:          common.CommandConnectorUpdateFlags{Timeout: 1 * time.Second},
			expectedErrors: []string{"connector name must be configured"},
		},
		{
			name:           "connector name is nil",
			args:           []string{""},
			flags:          common.CommandConnectorUpdateFlags{Timeout: 1 * time.Minute},
			expectedErrors: []string{"connector name must not be empty"},
		},
		{
			name:           "more than one argument is specified",
			args:           []string{"my", "connector"},
			flags:          common.CommandConnectorUpdateFlags{Timeout: 1 * time.Minute},
			expectedErrors: []string{"only one argument is allowed for this command"},
		},
		{
			name:           "connector name is not valid.",
			args:           []string{"my new connector"},
			flags:          common.CommandConnectorUpdateFlags{Timeout: 1 * time.Minute},
			expectedErrors: []string{"connector name is not valid: value does not match this regular expression: ^[a-z0-9]([-a-z0-9]*[a-z0-9])*(\\.[a-z0-9]([-a-z0-9]*[a-z0-9])*)*$"},
		},
		{
			name: "connector type is not valid",
			args: []string{"my-connector-type"},
			flags: common.CommandConnectorUpdateFlags{
				ConnectorType: "not-valid",
				Timeout:       1 * time.Minute,
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
			flags: common.CommandConnectorUpdateFlags{
				RoutingKey: "not-valid$",
				Timeout:    60 * time.Second,
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
			flags: common.CommandConnectorUpdateFlags{
				TlsSecret: "test-tls",
				Timeout:   5 * time.Second,
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
			flags: common.CommandConnectorUpdateFlags{
				Port:    -1,
				Timeout: 40 * time.Second,
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
			flags: common.CommandConnectorUpdateFlags{
				Workload: "!workload",
				Timeout:  70 * time.Second,
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
			flags: common.CommandConnectorUpdateFlags{
				Selector: "@#$%",
				Timeout:  1 * time.Minute,
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
			name: "selector/host",
			args: []string{"selector"},
			flags: common.CommandConnectorUpdateFlags{
				Timeout:  1 * time.Second,
				Output:   "json",
				Selector: "app=test",
				Host:     "test",
			},
			skupperObjects: []runtime.Object{
				&v1alpha1.Connector{
					ObjectMeta: v1.ObjectMeta{
						Name:      "selector",
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
				"If host is configured, cannot configure workload or selector",
				"If selector is configured, cannot configure workload or host"},
		},
		{
			name: "workload/host",
			args: []string{"workload"},
			flags: common.CommandConnectorUpdateFlags{
				Timeout:  1 * time.Second,
				Output:   "json",
				Workload: "deployment/test",
				Host:     "test",
			},
			skupperObjects: []runtime.Object{
				&v1alpha1.Connector{
					ObjectMeta: v1.ObjectMeta{
						Name:      "workload",
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
				"If host is configured, cannot configure workload or selector",
				"If workload is configured, cannot configure selector or host"},
		},
		{
			name: "timeout is not valid",
			args: []string{"bad-timeout"},
			flags: common.CommandConnectorUpdateFlags{
				Selector: "selector",
				Timeout:  0 * time.Second,
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
			flags: common.CommandConnectorUpdateFlags{
				Output:  "not-supported",
				Timeout: 1 * time.Minute,
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
			flags: common.CommandConnectorUpdateFlags{
				RoutingKey:      "routingkeyname",
				TlsSecret:       "secretname",
				Port:            1234,
				ConnectorType:   "tcp",
				IncludeNotReady: false,
				Timeout:         5 * time.Second,
				Output:          "json",
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

			command.Flags = &test.flags

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
		cmd.Flags = &common.CommandConnectorUpdateFlags{
			Timeout: 1 * time.Second,
			Output:  test.output,
		}
		cmd.namespace = "test"
		cmd.newSettings.output = cmd.Flags.Output

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

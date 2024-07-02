package kube

import (
	"fmt"
	"testing"

	"github.com/skupperproject/skupper/internal/cmd/skupper/utils"
	"github.com/skupperproject/skupper/pkg/apis/skupper/v1alpha1"
	"github.com/skupperproject/skupper/pkg/generated/client/clientset/versioned/typed/skupper/v1alpha1/fake"
	"github.com/spf13/pflag"
	"gotest.tools/assert"
	v12 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kubefake "k8s.io/client-go/kubernetes/fake"
	testing2 "k8s.io/client-go/testing"
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
		"output":            "",
	}
	var flagList []string

	cmd := newCmdConnectorUpdateWithMocks()

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
		setUpMock      func(command *CmdConnectorUpdate)
		expectedErrors []string
	}

	command := &CmdConnectorUpdate{
		namespace: "test",
	}

	testTable := []test{
		{
			name: "connector is not updated because get connector returned error",
			args: []string{"my-connector"},
			setUpMock: func(command *CmdConnectorUpdate) {
				fakeSkupperClient := &fake.FakeSkupperV1alpha1{Fake: &testing2.Fake{}}
				fakeSkupperClient.Fake.ClearActions()
				fakeSkupperClient.Fake.PrependReactor("get", "connectors", func(action testing2.Action) (handled bool, ret runtime.Object, err error) {
					return true, nil, fmt.Errorf("NotFound")
				})
				command.client = fakeSkupperClient
			},
			expectedErrors: []string{"connector my-connector must exist in namespace test to be updated"},
		},
		{
			name: "connector name is not specified",
			args: []string{},
			setUpMock: func(command *CmdConnectorUpdate) {
				fakeSkupperClient := &fake.FakeSkupperV1alpha1{Fake: &testing2.Fake{}}
				fakeSkupperClient.Fake.ClearActions()
				command.client = fakeSkupperClient
			},
			expectedErrors: []string{"connector name must be configured"},
		},
		{
			name: "connector name is nil",
			args: []string{""},
			setUpMock: func(command *CmdConnectorUpdate) {
				fakeSkupperClient := &fake.FakeSkupperV1alpha1{Fake: &testing2.Fake{}}
				fakeSkupperClient.Fake.ClearActions()
				command.client = fakeSkupperClient
			},
			expectedErrors: []string{"connector name must not be empty"},
		},
		{
			name: "more than one argument is specified",
			args: []string{"my", "connector"},
			setUpMock: func(command *CmdConnectorUpdate) {
				fakeSkupperClient := &fake.FakeSkupperV1alpha1{Fake: &testing2.Fake{}}
				fakeSkupperClient.Fake.ClearActions()
				command.client = fakeSkupperClient
			},
			expectedErrors: []string{"only one argument is allowed for this command"},
		},
		{
			name: "connector name is not valid.",
			args: []string{"my new connector"},
			setUpMock: func(command *CmdConnectorUpdate) {
				fakeSkupperClient := &fake.FakeSkupperV1alpha1{Fake: &testing2.Fake{}}
				fakeSkupperClient.Fake.ClearActions()
				command.client = fakeSkupperClient
			},
			expectedErrors: []string{"connector name is not valid: value does not match this regular expression: ^[a-z0-9]([-a-z0-9]*[a-z0-9])*(\\.[a-z0-9]([-a-z0-9]*[a-z0-9])*)*$"},
		},
		{
			name: "connector type is not valid",
			args: []string{"my-connector-type"},
			setUpMock: func(command *CmdConnectorUpdate) {
				fakeSkupperClient := &fake.FakeSkupperV1alpha1{Fake: &testing2.Fake{}}
				fakeSkupperClient.Fake.ClearActions()
				command.client = fakeSkupperClient
				command.flags = ConnectorUpdates{connectorType: "not-valid"}
			},
			expectedErrors: []string{
				"connector type is not valid: value not-valid not allowed. It should be one of this options: [tcp]"},
		},
		{
			name: "routing key is not valid",
			args: []string{"my-connector-rk"},
			setUpMock: func(command *CmdConnectorUpdate) {
				fakeSkupperClient := &fake.FakeSkupperV1alpha1{Fake: &testing2.Fake{}}
				fakeSkupperClient.Fake.ClearActions()
				command.client = fakeSkupperClient
				command.flags = ConnectorUpdates{routingKey: "not-valid$"}
			},
			expectedErrors: []string{
				"routing key is not valid: value does not match this regular expression: ^[a-z0-9]([-a-z0-9]*[a-z0-9])*(\\.[a-z0-9]([-a-z0-9]*[a-z0-9])*)*$"},
		},
		{
			name: "host is not valid",
			args: []string{"my-connector-host"},
			setUpMock: func(command *CmdConnectorUpdate) {
				fakeSkupperClient := &fake.FakeSkupperV1alpha1{Fake: &testing2.Fake{}}
				fakeSkupperClient.Fake.ClearActions()
				command.client = fakeSkupperClient
				command.flags = ConnectorUpdates{host: ":not-Valid"}
			},
			expectedErrors: []string{
				"host name is not valid: value does not match this regular expression: ^[a-z0-9]([-a-z0-9]*[a-z0-9])*(\\.[a-z0-9]([-a-z0-9]*[a-z0-9])*)*$"},
		},
		{
			name: "tls-secret is not valid",
			args: []string{"my-connector-tls"},
			setUpMock: func(command *CmdConnectorUpdate) {
				fakeSkupperClient := &fake.FakeSkupperV1alpha1{Fake: &testing2.Fake{}}
				fakeSkupperClient.Fake.ClearActions()
				command.client = fakeSkupperClient
				command.flags = ConnectorUpdates{tlsSecret: ":not-valid"}
				fakeKubeClient := kubefake.NewSimpleClientset()
				fakeKubeClient.Fake.ClearActions()
				fakeKubeClient.Fake.PrependReactor("get", "secrets", func(action testing2.Action) (handled bool, ret runtime.Object, err error) {
					return true, nil, fmt.Errorf("secret not found")
				})
				command.KubeClient = fakeKubeClient
			},
			expectedErrors: []string{
				"tls-secret is not valid: does not exist"},
		},
		{
			name: "port is not valid",
			args: []string{"my-connector-port"},
			setUpMock: func(command *CmdConnectorUpdate) {
				fakeSkupperClient := &fake.FakeSkupperV1alpha1{Fake: &testing2.Fake{}}
				fakeSkupperClient.Fake.ClearActions()
				command.client = fakeSkupperClient
				command.flags = ConnectorUpdates{port: -1}
			},
			expectedErrors: []string{
				"connector port is not valid: value is not positive"},
		},
		{
			name: "workload is not valid",
			args: []string{"bad-workload"},
			setUpMock: func(command *CmdConnectorUpdate) {
				fakeSkupperClient := &fake.FakeSkupperV1alpha1{Fake: &testing2.Fake{}}
				fakeSkupperClient.Fake.ClearActions()
				command.client = fakeSkupperClient
				command.flags = ConnectorUpdates{workload: "!workload"}
			},
			expectedErrors: []string{
				"workload is not valid: value does not match this regular expression: ^[A-Za-z0-9=:./-]+$"},
		},
		{
			name: "selector is not valid",
			args: []string{"bad-selector"},
			setUpMock: func(command *CmdConnectorUpdate) {
				fakeSkupperClient := &fake.FakeSkupperV1alpha1{Fake: &testing2.Fake{}}
				fakeSkupperClient.Fake.ClearActions()
				command.client = fakeSkupperClient
				command.flags = ConnectorUpdates{selector: "@#$%"}
			},
			expectedErrors: []string{
				"selector is not valid: value does not match this regular expression: ^[A-Za-z0-9=:./-]+$"},
		},
		{
			name: "output is not valid",
			args: []string{"bad-output"},
			setUpMock: func(command *CmdConnectorUpdate) {
				fakeSkupperClient := &fake.FakeSkupperV1alpha1{Fake: &testing2.Fake{}}
				fakeSkupperClient.Fake.ClearActions()
				command.client = fakeSkupperClient
				command.flags = ConnectorUpdates{output: "not-supported"}
			},
			expectedErrors: []string{
				"output type is not valid: value not-supported not allowed. It should be one of this options: [json yaml]"},
		},
		{
			name: "flags all valid",
			args: []string{"my-connector-flags"},
			setUpMock: func(command *CmdConnectorUpdate) {
				fakeSkupperClient := &fake.FakeSkupperV1alpha1{Fake: &testing2.Fake{}}
				fakeSkupperClient.Fake.ClearActions()
				command.client = fakeSkupperClient
				command.flags = ConnectorUpdates{
					host:            "hostname",
					routingKey:      "routingkeyname",
					tlsSecret:       "secretname",
					port:            1234,
					connectorType:   "tcp",
					selector:        "backend",
					includeNotReady: false,
					output:          "json",
				}
				fakeKubeClient := kubefake.NewSimpleClientset()
				fakeKubeClient.Fake.ClearActions()
				fakeKubeClient.Fake.PrependReactor("get", "secrets", func(action testing2.Action) (handled bool, ret runtime.Object, err error) {
					secret := v12.Secret{
						ObjectMeta: v1.ObjectMeta{
							Name: "secretname",
						},
					}
					return true, &secret, nil
				})
				command.KubeClient = fakeKubeClient
			},
			expectedErrors: []string{},
		},
	}

	for _, test := range testTable {
		t.Run(test.name, func(t *testing.T) {

			if test.setUpMock != nil {
				test.setUpMock(command)
			}

			actualErrors := command.ValidateInput(test.args)

			actualErrorsMessages := utils.ErrorsToMessages(actualErrors)

			assert.DeepEqual(t, actualErrorsMessages, test.expectedErrors)

		})
	}
}

func TestCmdConnectorUpdate_Run(t *testing.T) {
	type test struct {
		name         string
		setUpMock    func(command *CmdConnectorUpdate)
		errorMessage string
	}

	testTable := []test{
		{
			name: "runs ok",
			setUpMock: func(command *CmdConnectorUpdate) {
				fakeSkupperClient := &fake.FakeSkupperV1alpha1{Fake: &testing2.Fake{}}
				fakeSkupperClient.Fake.ClearActions()

				fakeSkupperClient.Fake.PrependReactor("update", "connectors", func(action testing2.Action) (handled bool, ret runtime.Object, err error) {
					UpdateAction, ok := action.(testing2.UpdateActionImpl)
					if !ok {
						return
					}
					UpdatedObject := UpdateAction.GetObject()

					connector, ok := UpdatedObject.(*v1alpha1.Connector)
					if !ok {
						return
					}
					if connector.Name != "my-connector" {
						return true, nil, fmt.Errorf("unexpected name value")
					}
					if connector.Spec.Type != "tcp" {
						return true, nil, fmt.Errorf("unexpected type value")
					}
					if connector.Spec.Port != 8080 {
						return true, nil, fmt.Errorf("unexpected port value")
					}
					if connector.Spec.Host != "hostname" {
						return true, nil, fmt.Errorf("unexpected host value")
					}
					if connector.Spec.RoutingKey != "keyname" {
						return true, nil, fmt.Errorf("unexpected routing key value")
					}
					if connector.Spec.TlsCredentials != "secretname" {
						return true, nil, fmt.Errorf("unexpected tls-secret value")
					}
					if connector.Spec.Selector != "backend" {
						return true, nil, fmt.Errorf("unexpected selector value")
					}
					return true, nil, nil
				})
				fakeKubeClient := kubefake.NewSimpleClientset()
				fakeKubeClient.Fake.ClearActions()
				fakeKubeClient.Fake.PrependReactor("get", "secrets", func(action testing2.Action) (handled bool, ret runtime.Object, err error) {
					secret := v12.Secret{
						ObjectMeta: v1.ObjectMeta{
							Name: "secretname",
						},
					}
					return true, &secret, nil
				})
				command.client = fakeSkupperClient
				command.name = "my-connector"
				command.newSettings.port = 8080
				command.newSettings.connectorType = "tcp"
				command.newSettings.host = "hostname"
				command.newSettings.routingKey = "keyname"
				command.newSettings.tlsSecret = "secretname"
				command.newSettings.selector = "backend"
			},
		},
		{
			name: "run fails",
			setUpMock: func(command *CmdConnectorUpdate) {
				fakeSkupperClient := &fake.FakeSkupperV1alpha1{Fake: &testing2.Fake{}}
				fakeSkupperClient.Fake.ClearActions()
				fakeSkupperClient.Fake.PrependReactor("update", "connectors", func(action testing2.Action) (handled bool, ret runtime.Object, err error) {
					return true, nil, fmt.Errorf("error")
				})
				fakeKubeClient := kubefake.NewSimpleClientset()
				fakeKubeClient.Fake.ClearActions()
				fakeKubeClient.Fake.PrependReactor("get", "secrets", func(action testing2.Action) (handled bool, ret runtime.Object, err error) {
					return true, nil, nil
				})
				command.KubeClient = fakeKubeClient
				command.client = fakeSkupperClient
				command.name = "my-fail-connector"
				command.newSettings.output = "json"
			},
			errorMessage: "error",
		},
	}

	for _, test := range testTable {
		cmd := newCmdConnectorUpdateWithMocks()
		test.setUpMock(cmd)

		//create a connector

		t.Run(test.name, func(t *testing.T) {

			err := cmd.Run()
			if err != nil {
				assert.Check(t, test.errorMessage == err.Error())
			} else {
				assert.Check(t, err == nil)
			}
		})
	}
}

func TestCmdConnectorUpdate_WaitUntilReady(t *testing.T) {
	type test struct {
		name        string
		setUpMock   func(command *CmdConnectorUpdate)
		expectError bool
	}

	testTable := []test{
		{
			name: "connector is not ready",
			setUpMock: func(command *CmdConnectorUpdate) {
				fakeSkupperClient := &fake.FakeSkupperV1alpha1{Fake: &testing2.Fake{}}
				fakeSkupperClient.Fake.ClearActions()
				fakeSkupperClient.Fake.PrependReactor("get", "connectors", func(action testing2.Action) (handled bool, ret runtime.Object, err error) {

					return true, &v1alpha1.Connector{
						ObjectMeta: v1.ObjectMeta{
							Name:      "my-connector",
							Namespace: "test",
						},
						Status: v1alpha1.Status{
							StatusMessage: "",
						},
					}, nil
				})
				command.client = fakeSkupperClient
				command.name = "my-connector"
			},
			expectError: true,
		},
		{
			name: "connector is not returned",
			setUpMock: func(command *CmdConnectorUpdate) {
				fakeSkupperClient := &fake.FakeSkupperV1alpha1{Fake: &testing2.Fake{}}
				fakeSkupperClient.Fake.ClearActions()
				fakeSkupperClient.Fake.PrependReactor("get", "connectors", func(action testing2.Action) (handled bool, ret runtime.Object, err error) {
					return true, nil, fmt.Errorf("it failed")
				})
				command.client = fakeSkupperClient
				command.name = "my-connector"
			},
			expectError: true,
		},
		{
			name: "connector is ready",
			setUpMock: func(command *CmdConnectorUpdate) {
				fakeSkupperClient := &fake.FakeSkupperV1alpha1{Fake: &testing2.Fake{}}
				fakeSkupperClient.Fake.ClearActions()
				fakeSkupperClient.Fake.PrependReactor("get", "connectors", func(action testing2.Action) (handled bool, ret runtime.Object, err error) {

					return true, &v1alpha1.Connector{
						ObjectMeta: v1.ObjectMeta{
							Name:      "my-connector",
							Namespace: "test",
						},
						Status: v1alpha1.Status{
							StatusMessage: "Ok",
						},
					}, nil
				})
				command.client = fakeSkupperClient
				command.name = "my-connector"
			},
			expectError: false,
		},
		{
			name: "connector is ready json output",
			setUpMock: func(command *CmdConnectorUpdate) {
				fakeSkupperClient := &fake.FakeSkupperV1alpha1{Fake: &testing2.Fake{}}
				fakeSkupperClient.Fake.ClearActions()
				fakeSkupperClient.Fake.PrependReactor("get", "connectors", func(action testing2.Action) (handled bool, ret runtime.Object, err error) {

					return true, &v1alpha1.Connector{
						ObjectMeta: v1.ObjectMeta{
							Name:      "my-connector",
							Namespace: "test",
						},
						Status: v1alpha1.Status{
							StatusMessage: "Ok",
						},
					}, nil
				})
				command.client = fakeSkupperClient
				command.newSettings.output = "json"
				command.name = "my-connector"
			},
			expectError: false,
		},
	}

	for _, test := range testTable {
		cmd := newCmdConnectorUpdateWithMocks()

		test.setUpMock(cmd)
		t.Run(test.name, func(t *testing.T) {

			err := cmd.WaitUntilReady()
			if err != nil {
				assert.Check(t, test.expectError)
			}

		})
	}
}

// --- helper methods

func newCmdConnectorUpdateWithMocks() *CmdConnectorUpdate {

	cmdConnectorUpdate := &CmdConnectorUpdate{
		client:     &fake.FakeSkupperV1alpha1{Fake: &testing2.Fake{}},
		namespace:  "test",
		KubeClient: kubefake.NewSimpleClientset(),
	}

	return cmdConnectorUpdate
}

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

func TestCmdConnectorCreate_NewCmdConnectorCreate(t *testing.T) {

	t.Run("connector command", func(t *testing.T) {

		result := NewCmdConnectorCreate()

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

func TestCmdConnectorCreate_AddFlags(t *testing.T) {

	expectedFlagsWithDefaultValue := map[string]interface{}{
		"routing-key":       "",
		"host":              "",
		"tls-secret":        "",
		"type":              "tcp",
		"workload":          "",
		"selector":          "",
		"include-not-ready": "false",
		"output":            "",
	}
	var flagList []string

	cmd := newCmdConnectorCreateWithMocks()

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

func TestCmdConnectorCreate_ValidateInput(t *testing.T) {
	type test struct {
		name           string
		args           []string
		setUpMock      func(command *CmdConnectorCreate)
		expectedErrors []string
	}

	command := &CmdConnectorCreate{
		namespace: "test",
	}

	testTable := []test{
		{
			name: "connector is not created because there is already the same connector in the namespace",
			args: []string{"my-connector", "8080"},
			setUpMock: func(command *CmdConnectorCreate) {
				fakeSkupperClient := &fake.FakeSkupperV1alpha1{Fake: &testing2.Fake{}}
				fakeSkupperClient.Fake.ClearActions()
				fakeSkupperClient.Fake.PrependReactor("get", "connectors", func(action testing2.Action) (handled bool, ret runtime.Object, err error) {
					connector := v1alpha1.Connector{
						ObjectMeta: v1.ObjectMeta{
							Name:      "my-connector",
							Namespace: "test",
						},
						Spec: v1alpha1.ConnectorSpec{
							Port:     8080,
							Type:     "tcp",
							Host:     "test",
							Selector: "mySelector",
						},
						Status: v1alpha1.Status{
							StatusMessage: "Ok",
						},
					}
					return true, &connector, nil
				})
				command.client = fakeSkupperClient
				command.flags = ConnectorCreate{selector: "backend"}
			},
			expectedErrors: []string{"there is already a connector my-connector created for namespace test"},
		},
		{
			name: "connector name and port are not specified",
			args: []string{},
			setUpMock: func(command *CmdConnectorCreate) {
				fakeSkupperClient := &fake.FakeSkupperV1alpha1{Fake: &testing2.Fake{}}
				fakeSkupperClient.Fake.ClearActions()
				command.client = fakeSkupperClient
				command.flags = ConnectorCreate{selector: "backend"}
			},
			expectedErrors: []string{"connector name and port must be configured"},
		},
		{
			name: "connector name empty",
			args: []string{"", "8090"},
			setUpMock: func(command *CmdConnectorCreate) {
				fakeSkupperClient := &fake.FakeSkupperV1alpha1{Fake: &testing2.Fake{}}
				fakeSkupperClient.Fake.ClearActions()
				command.client = fakeSkupperClient
				command.flags = ConnectorCreate{selector: "backend"}
			},
			expectedErrors: []string{"connector name must not be empty"},
		},
		{
			name: "connector port empty",
			args: []string{"my-name-port-empty", ""},
			setUpMock: func(command *CmdConnectorCreate) {
				fakeSkupperClient := &fake.FakeSkupperV1alpha1{Fake: &testing2.Fake{}}
				fakeSkupperClient.Fake.ClearActions()
				command.client = fakeSkupperClient
				command.flags = ConnectorCreate{selector: "backend"}
			},
			expectedErrors: []string{"connector port must not be empty"},
		},
		{
			name: "connector port not positive",
			args: []string{"my-port-positive", "-45"},
			setUpMock: func(command *CmdConnectorCreate) {
				fakeSkupperClient := &fake.FakeSkupperV1alpha1{Fake: &testing2.Fake{}}
				fakeSkupperClient.Fake.ClearActions()
				command.client = fakeSkupperClient
				command.flags = ConnectorCreate{selector: "backend"}
			},
			expectedErrors: []string{"connector port is not valid: value is not positive"},
		},
		{
			name: "connector name and port are not specified",
			args: []string{},
			setUpMock: func(command *CmdConnectorCreate) {
				fakeSkupperClient := &fake.FakeSkupperV1alpha1{Fake: &testing2.Fake{}}
				fakeSkupperClient.Fake.ClearActions()
				command.client = fakeSkupperClient
				command.flags = ConnectorCreate{selector: "backend"}
			},
			expectedErrors: []string{"connector name and port must be configured"},
		},
		{
			name: "connector port is not specified",
			args: []string{"my-name"},
			setUpMock: func(command *CmdConnectorCreate) {
				fakeSkupperClient := &fake.FakeSkupperV1alpha1{Fake: &testing2.Fake{}}
				fakeSkupperClient.Fake.ClearActions()
				command.client = fakeSkupperClient
				command.flags = ConnectorCreate{selector: "backend"}
			},
			expectedErrors: []string{"connector name and port must be configured"},
		},
		{
			name: "more than two arguments are specified",
			args: []string{"my", "connector", "8080"},
			setUpMock: func(command *CmdConnectorCreate) {
				fakeSkupperClient := &fake.FakeSkupperV1alpha1{Fake: &testing2.Fake{}}
				fakeSkupperClient.Fake.ClearActions()
				command.client = fakeSkupperClient
				command.flags = ConnectorCreate{selector: "backend"}
			},
			expectedErrors: []string{"only two arguments are allowed for this command"},
		},
		{
			name: "connector name is not valid.",
			args: []string{"my new connector", "8080"},
			setUpMock: func(command *CmdConnectorCreate) {
				fakeSkupperClient := &fake.FakeSkupperV1alpha1{Fake: &testing2.Fake{}}
				fakeSkupperClient.Fake.ClearActions()
				command.client = fakeSkupperClient
				command.flags = ConnectorCreate{selector: "backend"}
			},
			expectedErrors: []string{"connector name is not valid: value does not match this regular expression: ^[a-z0-9]([-a-z0-9]*[a-z0-9])*(\\.[a-z0-9]([-a-z0-9]*[a-z0-9])*)*$"},
		},
		{
			name: "port is not valid.",
			args: []string{"my-connector-port", "abcd"},
			setUpMock: func(command *CmdConnectorCreate) {
				fakeSkupperClient := &fake.FakeSkupperV1alpha1{Fake: &testing2.Fake{}}
				fakeSkupperClient.Fake.ClearActions()
				command.client = fakeSkupperClient
				command.flags = ConnectorCreate{selector: "backend"}
			},
			expectedErrors: []string{"connector port is not valid: strconv.Atoi: parsing \"abcd\": invalid syntax"},
		},
		{
			name: "connector type is not valid",
			args: []string{"my-connector-type", "8080"},
			setUpMock: func(command *CmdConnectorCreate) {
				fakeSkupperClient := &fake.FakeSkupperV1alpha1{Fake: &testing2.Fake{}}
				fakeSkupperClient.Fake.ClearActions()
				command.client = fakeSkupperClient
				command.flags = ConnectorCreate{
					connectorType: "not-valid",
					selector:      "backend",
				}
			},
			expectedErrors: []string{
				"connector type is not valid: value not-valid not allowed. It should be one of this options: [tcp]"},
		},
		{
			name: "routing key is not valid",
			args: []string{"my-connector-rk", "8080"},
			setUpMock: func(command *CmdConnectorCreate) {
				fakeSkupperClient := &fake.FakeSkupperV1alpha1{Fake: &testing2.Fake{}}
				fakeSkupperClient.Fake.ClearActions()
				command.client = fakeSkupperClient
				command.flags = ConnectorCreate{
					routingKey: "not-valid$",
					selector:   "backend",
				}
			},
			expectedErrors: []string{
				"routing key is not valid: value does not match this regular expression: ^[a-z0-9]([-a-z0-9]*[a-z0-9])*(\\.[a-z0-9]([-a-z0-9]*[a-z0-9])*)*$"},
		},
		{
			name: "host is not valid",
			args: []string{"my-connector-host", "8080"},
			setUpMock: func(command *CmdConnectorCreate) {
				fakeSkupperClient := &fake.FakeSkupperV1alpha1{Fake: &testing2.Fake{}}
				fakeSkupperClient.Fake.ClearActions()
				command.client = fakeSkupperClient
				command.flags = ConnectorCreate{
					host: ":not-Valid"}
			},
			expectedErrors: []string{
				"host name is not valid: value does not match this regular expression: ^[a-z0-9]([-a-z0-9]*[a-z0-9])*(\\.[a-z0-9]([-a-z0-9]*[a-z0-9])*)*$"},
		},
		{
			name: "tls-secret does not exist",
			args: []string{"my-connector-tls", "8080"},
			setUpMock: func(command *CmdConnectorCreate) {
				fakeSkupperClient := &fake.FakeSkupperV1alpha1{Fake: &testing2.Fake{}}
				fakeSkupperClient.Fake.ClearActions()
				command.client = fakeSkupperClient
				command.flags = ConnectorCreate{
					tlsSecret: "not-valid",
					selector:  "backend",
				}
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
			name: "workload is not valid",
			args: []string{"bad-workload", "1234"},
			setUpMock: func(command *CmdConnectorCreate) {
				fakeSkupperClient := &fake.FakeSkupperV1alpha1{Fake: &testing2.Fake{}}
				fakeSkupperClient.Fake.ClearActions()
				command.client = fakeSkupperClient
				command.flags = ConnectorCreate{workload: "@345"}
			},
			expectedErrors: []string{
				"workload is not valid: value does not match this regular expression: ^[A-Za-z0-9=:./-]+$"},
		},
		{
			name: "selector is not valid",
			args: []string{"bad-selector", "1234"},
			setUpMock: func(command *CmdConnectorCreate) {
				fakeSkupperClient := &fake.FakeSkupperV1alpha1{Fake: &testing2.Fake{}}
				fakeSkupperClient.Fake.ClearActions()
				command.client = fakeSkupperClient
				command.flags = ConnectorCreate{selector: "@#$%"}
			},
			expectedErrors: []string{
				"selector is not valid: value does not match this regular expression: ^[A-Za-z0-9=:./-]+$"},
		},
		{
			name: "output is not valid",
			args: []string{"bad-output", "1234"},
			setUpMock: func(command *CmdConnectorCreate) {
				fakeSkupperClient := &fake.FakeSkupperV1alpha1{Fake: &testing2.Fake{}}
				fakeSkupperClient.Fake.ClearActions()
				command.client = fakeSkupperClient
				command.flags = ConnectorCreate{
					output:   "not-supported",
					selector: "backend",
				}
			},
			expectedErrors: []string{
				"output type is not valid: value not-supported not allowed. It should be one of this options: [json yaml]"},
		},
		{
			name: "missing selector/host/workload",
			args: []string{"missing", "1234"},
			setUpMock: func(command *CmdConnectorCreate) {
				fakeSkupperClient := &fake.FakeSkupperV1alpha1{Fake: &testing2.Fake{}}
				fakeSkupperClient.Fake.ClearActions()
				command.client = fakeSkupperClient
				command.flags = ConnectorCreate{output: "json"}
			},
			expectedErrors: []string{
				"One of the following options must be set: workload, selector, host"},
		},
		{
			name: "flags all valid",
			args: []string{"my-connector-flags", "8080"},
			setUpMock: func(command *CmdConnectorCreate) {
				fakeSkupperClient := &fake.FakeSkupperV1alpha1{Fake: &testing2.Fake{}}
				fakeSkupperClient.Fake.ClearActions()
				command.client = fakeSkupperClient
				command.flags = ConnectorCreate{
					host:            "hostname",
					routingKey:      "routingkeyname",
					tlsSecret:       "secretname",
					connectorType:   "tcp",
					selector:        "backend",
					includeNotReady: true,
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

func TestCmdConnectorCreate_Run(t *testing.T) {
	type test struct {
		name         string
		setUpMock    func(command *CmdConnectorCreate)
		errorMessage string
	}

	testTable := []test{
		{
			name: "runs ok",
			setUpMock: func(command *CmdConnectorCreate) {
				fakeSkupperClient := &fake.FakeSkupperV1alpha1{Fake: &testing2.Fake{}}
				fakeSkupperClient.Fake.ClearActions()
				fakeSkupperClient.Fake.PrependReactor("create", "connectors", func(action testing2.Action) (handled bool, ret runtime.Object, err error) {
					createAction, ok := action.(testing2.CreateActionImpl)
					if !ok {
						return
					}
					createdObject := createAction.GetObject()

					connector, ok := createdObject.(*v1alpha1.Connector)
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
					// TBD workload not in connector CRD
					//if connector.Spec.Workload != "deployment/backend" {
					//	return true, nil, fmt.Errorf("unexpected workload value")
					//}
					if connector.Spec.Selector != "backend" {
						return true, nil, fmt.Errorf("unexpected selector value")
					}
					if connector.Spec.IncludeNotReady != true {
						return true, nil, fmt.Errorf("unexpected Include Not Ready value")
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
				command.KubeClient = fakeKubeClient
				command.client = fakeSkupperClient
				command.name = "my-connector"
				command.port = 8080
				command.flags.connectorType = "tcp"
				command.flags.host = "hostname"
				command.flags.routingKey = "keyname"
				command.flags.tlsSecret = "secretname"
				command.flags.includeNotReady = true
				command.flags.workload = "deployment/backend"
				command.flags.selector = "backend"
			},
		},
		{
			name: "run fails",
			setUpMock: func(command *CmdConnectorCreate) {
				fakeSkupperClient := &fake.FakeSkupperV1alpha1{Fake: &testing2.Fake{}}
				fakeSkupperClient.Fake.ClearActions()
				fakeSkupperClient.Fake.PrependReactor("create", "connectors", func(action testing2.Action) (handled bool, ret runtime.Object, err error) {
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
				command.output = "json"
			},
			errorMessage: "error",
		},
	}

	for _, test := range testTable {
		cmd := newCmdConnectorCreateWithMocks()
		test.setUpMock(cmd)

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

func TestCmdConnectorCreate_WaitUntilReady(t *testing.T) {
	type test struct {
		name        string
		setUpMock   func(command *CmdConnectorCreate)
		expectError bool
	}

	testTable := []test{
		{
			name: "connector is not ready",
			setUpMock: func(command *CmdConnectorCreate) {
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
			},
			expectError: true,
		},
		{
			name: "connector is not returned",
			setUpMock: func(command *CmdConnectorCreate) {
				fakeSkupperClient := &fake.FakeSkupperV1alpha1{Fake: &testing2.Fake{}}
				fakeSkupperClient.Fake.ClearActions()
				fakeSkupperClient.Fake.PrependReactor("get", "connectors", func(action testing2.Action) (handled bool, ret runtime.Object, err error) {
					return true, nil, fmt.Errorf("it failed")
				})
				command.client = fakeSkupperClient
			},
			expectError: true,
		},
		{
			name: "connector is ready",
			setUpMock: func(command *CmdConnectorCreate) {
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

			},
			expectError: false,
		},
		{
			name: "connector is ready yaml output",
			setUpMock: func(command *CmdConnectorCreate) {
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
				command.output = "yaml"
			},
			expectError: false,
		},
	}

	for _, test := range testTable {
		cmd := newCmdConnectorCreateWithMocks()
		cmd.name = "my-connector"
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

func newCmdConnectorCreateWithMocks() *CmdConnectorCreate {
	cmdConnectorCreate := &CmdConnectorCreate{
		client:     &fake.FakeSkupperV1alpha1{Fake: &testing2.Fake{}},
		namespace:  "test",
		KubeClient: kubefake.NewSimpleClientset(),
	}

	return cmdConnectorCreate
}

package kube

import (
	"fmt"
	"testing"

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

func TestCmdListenerCreate_NewCmdListenerCreate(t *testing.T) {

	t.Run("listener command", func(t *testing.T) {

		result := NewCmdListenerCreate()

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

func TestCmdListenerCreate_AddFlags(t *testing.T) {

	expectedFlagsWithDefaultValue := map[string]interface{}{
		"routing-key": "",
		"host":        "",
		"tls-secret":  "",
		"type":        "tcp",
		"output":      "",
	}
	var flagList []string

	cmd := newCmdListenerCreateWithMocks()

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

func TestCmdListenerCreate_ValidateInput(t *testing.T) {
	type test struct {
		name           string
		args           []string
		setUpMock      func(command *CmdListenerCreate)
		expectedErrors []string
	}

	command := &CmdListenerCreate{
		namespace: "test",
	}

	testTable := []test{
		{
			name: "listener is not created because there is already the same listener in the namespace",
			args: []string{"my-listener", "8080"},
			setUpMock: func(command *CmdListenerCreate) {
				fakeSkupperClient := &fake.FakeSkupperV1alpha1{Fake: &testing2.Fake{}}
				fakeSkupperClient.Fake.ClearActions()
				fakeSkupperClient.Fake.PrependReactor("get", "listeners", func(action testing2.Action) (handled bool, ret runtime.Object, err error) {
					listener := v1alpha1.Listener{
						ObjectMeta: v1.ObjectMeta{
							Name:      "my-listener",
							Namespace: "test",
						},
						Spec: v1alpha1.ListenerSpec{
							Port: 8080,
							Type: "tcp",
							Host: "test",
						},
						Status: v1alpha1.Status{
							StatusMessage: "Ok",
						},
					}
					return true, &listener, nil
				})
				command.client = fakeSkupperClient
			},
			expectedErrors: []string{"there is already a listener my-listener created for namespace test"},
		},
		{
			name: "listener name and port are not specified",
			args: []string{},
			setUpMock: func(command *CmdListenerCreate) {
				fakeSkupperClient := &fake.FakeSkupperV1alpha1{Fake: &testing2.Fake{}}
				fakeSkupperClient.Fake.ClearActions()
				command.client = fakeSkupperClient
			},
			expectedErrors: []string{"listener name and port must be configured"},
		},
		{
			name: "listener name empty",
			args: []string{"", "8090"},
			setUpMock: func(command *CmdListenerCreate) {
				fakeSkupperClient := &fake.FakeSkupperV1alpha1{Fake: &testing2.Fake{}}
				fakeSkupperClient.Fake.ClearActions()
				command.client = fakeSkupperClient
			},
			expectedErrors: []string{"listener name must not be empty"},
		},
		{
			name: "listener port empty",
			args: []string{"my-name-port-empty", ""},
			setUpMock: func(command *CmdListenerCreate) {
				fakeSkupperClient := &fake.FakeSkupperV1alpha1{Fake: &testing2.Fake{}}
				fakeSkupperClient.Fake.ClearActions()
				command.client = fakeSkupperClient
			},
			expectedErrors: []string{"listener port must not be empty"},
		},
		{
			name: "listener port not positive",
			args: []string{"my-port-positive", "-45"},
			setUpMock: func(command *CmdListenerCreate) {
				fakeSkupperClient := &fake.FakeSkupperV1alpha1{Fake: &testing2.Fake{}}
				fakeSkupperClient.Fake.ClearActions()
				command.client = fakeSkupperClient
			},
			expectedErrors: []string{"listener port is not valid: value is not positive"},
		},
		{
			name: "listener name and port are not specified",
			args: []string{},
			setUpMock: func(command *CmdListenerCreate) {
				fakeSkupperClient := &fake.FakeSkupperV1alpha1{Fake: &testing2.Fake{}}
				fakeSkupperClient.Fake.ClearActions()
				command.client = fakeSkupperClient
			},
			expectedErrors: []string{"listener name and port must be configured"},
		},
		{
			name: "listener port is not specified",
			args: []string{"my-name"},
			setUpMock: func(command *CmdListenerCreate) {
				fakeSkupperClient := &fake.FakeSkupperV1alpha1{Fake: &testing2.Fake{}}
				fakeSkupperClient.Fake.ClearActions()
				command.client = fakeSkupperClient
			},
			expectedErrors: []string{"listener name and port must be configured"},
		},
		{
			name: "more than two arguments are specified",
			args: []string{"my", "listener", "8080"},
			setUpMock: func(command *CmdListenerCreate) {
				fakeSkupperClient := &fake.FakeSkupperV1alpha1{Fake: &testing2.Fake{}}
				fakeSkupperClient.Fake.ClearActions()
				command.client = fakeSkupperClient
			},
			expectedErrors: []string{"only two arguments are allowed for this command"},
		},
		{
			name: "listener name is not valid.",
			args: []string{"my new listener", "8080"},
			setUpMock: func(command *CmdListenerCreate) {
				fakeSkupperClient := &fake.FakeSkupperV1alpha1{Fake: &testing2.Fake{}}
				fakeSkupperClient.Fake.ClearActions()
				command.client = fakeSkupperClient
			},
			expectedErrors: []string{"listener name is not valid: value does not match this regular expression: ^[a-z0-9]([-a-z0-9]*[a-z0-9])*(\\.[a-z0-9]([-a-z0-9]*[a-z0-9])*)*$"},
		},
		{
			name: "port is not valid.",
			args: []string{"my-listener-port", "abcd"},
			setUpMock: func(command *CmdListenerCreate) {
				fakeSkupperClient := &fake.FakeSkupperV1alpha1{Fake: &testing2.Fake{}}
				fakeSkupperClient.Fake.ClearActions()
				command.client = fakeSkupperClient
			},
			expectedErrors: []string{"listener port is not valid: strconv.Atoi: parsing \"abcd\": invalid syntax"},
		},
		{
			name: "listener type is not valid",
			args: []string{"my-listener-type", "8080"},
			setUpMock: func(command *CmdListenerCreate) {
				fakeSkupperClient := &fake.FakeSkupperV1alpha1{Fake: &testing2.Fake{}}
				fakeSkupperClient.Fake.ClearActions()
				command.client = fakeSkupperClient
				command.flags = ListenerCreate{listenerType: "not-valid"}
			},
			expectedErrors: []string{
				"listener type is not valid: value not-valid not allowed. It should be one of this options: [tcp]"},
		},
		{
			name: "routing key is not valid",
			args: []string{"my-listener-rk", "8080"},
			setUpMock: func(command *CmdListenerCreate) {
				fakeSkupperClient := &fake.FakeSkupperV1alpha1{Fake: &testing2.Fake{}}
				fakeSkupperClient.Fake.ClearActions()
				command.client = fakeSkupperClient
				command.flags = ListenerCreate{routingKey: "not-valid$"}
			},
			expectedErrors: []string{
				"routing key is not valid: value does not match this regular expression: ^[a-z0-9]([-a-z0-9]*[a-z0-9])*(\\.[a-z0-9]([-a-z0-9]*[a-z0-9])*)*$"},
		},
		{
			name: "host is not valid",
			args: []string{"my-listener-host", "8080"},
			setUpMock: func(command *CmdListenerCreate) {
				fakeSkupperClient := &fake.FakeSkupperV1alpha1{Fake: &testing2.Fake{}}
				fakeSkupperClient.Fake.ClearActions()
				command.client = fakeSkupperClient
				command.flags = ListenerCreate{host: ":not-Valid"}
			},
			expectedErrors: []string{
				"host name is not valid: value does not match this regular expression: ^[a-z0-9]([-a-z0-9]*[a-z0-9])*(\\.[a-z0-9]([-a-z0-9]*[a-z0-9])*)*$"},
		},
		{
			name: "tls-secret does not exist",
			args: []string{"my-listener-tls", "8080"},
			setUpMock: func(command *CmdListenerCreate) {
				fakeSkupperClient := &fake.FakeSkupperV1alpha1{Fake: &testing2.Fake{}}
				fakeSkupperClient.Fake.ClearActions()
				command.client = fakeSkupperClient
				command.flags = ListenerCreate{tlsSecret: "not-valid"}
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
			name: "output is not valid",
			args: []string{"bad-output", "1234"},
			setUpMock: func(command *CmdListenerCreate) {
				fakeSkupperClient := &fake.FakeSkupperV1alpha1{Fake: &testing2.Fake{}}
				fakeSkupperClient.Fake.ClearActions()
				command.client = fakeSkupperClient
				command.flags = ListenerCreate{output: "not-supported"}
			},
			expectedErrors: []string{
				"output type is not valid: value not-supported not allowed. It should be one of this options: [json yaml]"},
		},
		{
			name: "flags all valid",
			args: []string{"my-listener-flags", "8080"},
			setUpMock: func(command *CmdListenerCreate) {
				fakeSkupperClient := &fake.FakeSkupperV1alpha1{Fake: &testing2.Fake{}}
				fakeSkupperClient.Fake.ClearActions()
				command.client = fakeSkupperClient
				command.flags = ListenerCreate{
					host:         "hostname",
					routingKey:   "routingkeyname",
					tlsSecret:    "secretname",
					listenerType: "tcp",
					output:       "json",
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

			actualErrorsMessages := errorsToMessages(actualErrors)

			assert.DeepEqual(t, actualErrorsMessages, test.expectedErrors)

		})
	}
}

func TestCmdListenerCreate_Run(t *testing.T) {
	type test struct {
		name         string
		setUpMock    func(command *CmdListenerCreate)
		errorMessage string
	}

	testTable := []test{
		{
			name: "runs ok",
			setUpMock: func(command *CmdListenerCreate) {
				fakeSkupperClient := &fake.FakeSkupperV1alpha1{Fake: &testing2.Fake{}}
				fakeSkupperClient.Fake.ClearActions()
				fakeSkupperClient.Fake.PrependReactor("create", "listeners", func(action testing2.Action) (handled bool, ret runtime.Object, err error) {
					createAction, ok := action.(testing2.CreateActionImpl)
					if !ok {
						return
					}
					createdObject := createAction.GetObject()

					listener, ok := createdObject.(*v1alpha1.Listener)
					if !ok {
						return
					}

					if listener.Name != "my-listener" {
						return true, nil, fmt.Errorf("unexpected name value")
					}
					if listener.Spec.Type != "tcp" {
						return true, nil, fmt.Errorf("unexpected type value")
					}
					if listener.Spec.Port != 8080 {
						return true, nil, fmt.Errorf("unexpected port value")
					}
					if listener.Spec.Host != "hostname" {
						return true, nil, fmt.Errorf("unexpected host value")
					}
					if listener.Spec.RoutingKey != "keyname" {
						return true, nil, fmt.Errorf("unexpected routing key value")
					}
					if listener.Spec.TlsCredentials != "secretname" {
						return true, nil, fmt.Errorf("unexpected tls-secret value")
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
				command.name = "my-listener"
				command.port = 8080
				command.flags.listenerType = "tcp"
				command.flags.host = "hostname"
				command.flags.routingKey = "keyname"
				command.flags.tlsSecret = "secretname"
			},
		},
		{
			name: "run fails",
			setUpMock: func(command *CmdListenerCreate) {
				fakeSkupperClient := &fake.FakeSkupperV1alpha1{Fake: &testing2.Fake{}}
				fakeSkupperClient.Fake.ClearActions()
				fakeSkupperClient.Fake.PrependReactor("create", "listeners", func(action testing2.Action) (handled bool, ret runtime.Object, err error) {
					return true, nil, fmt.Errorf("error")
				})
				fakeKubeClient := kubefake.NewSimpleClientset()
				fakeKubeClient.Fake.ClearActions()
				fakeKubeClient.Fake.PrependReactor("get", "secrets", func(action testing2.Action) (handled bool, ret runtime.Object, err error) {
					return true, nil, nil
				})
				command.KubeClient = fakeKubeClient
				command.client = fakeSkupperClient
				command.name = "my-fail-listener"
				command.output = "json"
			},
			errorMessage: "error",
		},
	}

	for _, test := range testTable {
		cmd := newCmdListenerCreateWithMocks()
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

func TestCmdListenerCreate_WaitUntilReady(t *testing.T) {
	type test struct {
		name        string
		setUpMock   func(command *CmdListenerCreate)
		expectError bool
	}

	testTable := []test{
		{
			name: "listener is not ready",
			setUpMock: func(command *CmdListenerCreate) {
				fakeSkupperClient := &fake.FakeSkupperV1alpha1{Fake: &testing2.Fake{}}
				fakeSkupperClient.Fake.ClearActions()
				fakeSkupperClient.Fake.PrependReactor("get", "listeners", func(action testing2.Action) (handled bool, ret runtime.Object, err error) {

					return true, &v1alpha1.Listener{
						ObjectMeta: v1.ObjectMeta{
							Name:      "my-listener",
							Namespace: "test",
						},
						Status: v1alpha1.Status{
							StatusMessage: "",
						},
					}, nil
				})
				command.client = fakeSkupperClient
				command.name = "my-listener"
				command.port = 8080
			},
			expectError: true,
		},
		{
			name: "listener is not returned",
			setUpMock: func(command *CmdListenerCreate) {
				fakeSkupperClient := &fake.FakeSkupperV1alpha1{Fake: &testing2.Fake{}}
				fakeSkupperClient.Fake.ClearActions()
				fakeSkupperClient.Fake.PrependReactor("get", "listeners", func(action testing2.Action) (handled bool, ret runtime.Object, err error) {
					return true, nil, fmt.Errorf("it failed")
				})
				command.client = fakeSkupperClient
				command.name = "my-listener"
				command.port = 8080
			},
			expectError: true,
		},
		{
			name: "listener is ready",
			setUpMock: func(command *CmdListenerCreate) {
				fakeSkupperClient := &fake.FakeSkupperV1alpha1{Fake: &testing2.Fake{}}
				fakeSkupperClient.Fake.ClearActions()
				fakeSkupperClient.Fake.PrependReactor("get", "listeners", func(action testing2.Action) (handled bool, ret runtime.Object, err error) {

					return true, &v1alpha1.Listener{
						ObjectMeta: v1.ObjectMeta{
							Name:      "my-listener",
							Namespace: "test",
						},
						Status: v1alpha1.Status{
							StatusMessage: "Ok",
						},
					}, nil
				})
				command.client = fakeSkupperClient
				command.name = "my-listener"
				command.port = 8080
			},
			expectError: false,
		},
		{
			name: "listener is ready yaml output",
			setUpMock: func(command *CmdListenerCreate) {
				fakeSkupperClient := &fake.FakeSkupperV1alpha1{Fake: &testing2.Fake{}}
				fakeSkupperClient.Fake.ClearActions()
				fakeSkupperClient.Fake.PrependReactor("get", "listeners", func(action testing2.Action) (handled bool, ret runtime.Object, err error) {

					return true, &v1alpha1.Listener{
						ObjectMeta: v1.ObjectMeta{
							Name:      "my-listener",
							Namespace: "test",
						},
						Status: v1alpha1.Status{
							StatusMessage: "Ok",
						},
					}, nil
				})
				command.client = fakeSkupperClient
				command.output = "yaml"
				command.name = "my-listener"
				command.port = 8080
			},
			expectError: false,
		},
	}

	for _, test := range testTable {
		cmd := newCmdListenerCreateWithMocks()

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

func newCmdListenerCreateWithMocks() *CmdListenerCreate {
	cmdListenerCreate := &CmdListenerCreate{
		client:     &fake.FakeSkupperV1alpha1{Fake: &testing2.Fake{}},
		namespace:  "test",
		KubeClient: kubefake.NewSimpleClientset(),
	}

	return cmdListenerCreate
}

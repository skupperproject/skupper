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

func TestCmdListenerUpdate_NewCmdListenerUpdate(t *testing.T) {

	t.Run("Update command", func(t *testing.T) {

		result := NewCmdListenerUpdate()

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

func TestCmdListenerUpdate_AddFlags(t *testing.T) {

	expectedFlagsWithDefaultValue := map[string]interface{}{
		"routing-key": "",
		"host":        "",
		"tls-secret":  "",
		"type":        "tcp",
		"port":        "0",
		"output":      "",
	}
	var flagList []string

	cmd := newCmdListenerUpdateWithMocks()

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

func TestCmdListenerUpdate_ValidateInput(t *testing.T) {
	type test struct {
		name           string
		args           []string
		setUpMock      func(command *CmdListenerUpdate)
		expectedErrors []string
	}

	command := &CmdListenerUpdate{
		namespace: "test",
	}

	testTable := []test{
		{
			name: "listener is not updated because listener does not exist in the namespace",
			args: []string{"my-listener"},
			setUpMock: func(command *CmdListenerUpdate) {
				fakeSkupperClient := &fake.FakeSkupperV1alpha1{Fake: &testing2.Fake{}}
				fakeSkupperClient.Fake.ClearActions()
				fakeSkupperClient.Fake.PrependReactor("get", "listeners", func(action testing2.Action) (handled bool, ret runtime.Object, err error) {
					return true, nil, fmt.Errorf("NotFound")
				})
				command.client = fakeSkupperClient
			},
			expectedErrors: []string{"listener my-listener must exist in namespace test to be updated"},
		},
		{
			name: "listener name is not specified",
			args: []string{},
			setUpMock: func(command *CmdListenerUpdate) {
				fakeSkupperClient := &fake.FakeSkupperV1alpha1{Fake: &testing2.Fake{}}
				fakeSkupperClient.Fake.ClearActions()
				command.client = fakeSkupperClient
			},
			expectedErrors: []string{"listener name must be configured"},
		},
		{
			name: "listener name is nil",
			args: []string{""},
			setUpMock: func(command *CmdListenerUpdate) {
				fakeSkupperClient := &fake.FakeSkupperV1alpha1{Fake: &testing2.Fake{}}
				fakeSkupperClient.Fake.ClearActions()
				command.client = fakeSkupperClient
			},
			expectedErrors: []string{"listener name must not be empty"},
		},
		{
			name: "more than one argument is specified",
			args: []string{"my", "listener"},
			setUpMock: func(command *CmdListenerUpdate) {
				fakeSkupperClient := &fake.FakeSkupperV1alpha1{Fake: &testing2.Fake{}}
				fakeSkupperClient.Fake.ClearActions()
				command.client = fakeSkupperClient
			},
			expectedErrors: []string{"only one argument is allowed for this command"},
		},
		{
			name: "listener name is not valid.",
			args: []string{"my new listener"},
			setUpMock: func(command *CmdListenerUpdate) {
				fakeSkupperClient := &fake.FakeSkupperV1alpha1{Fake: &testing2.Fake{}}
				fakeSkupperClient.Fake.ClearActions()
				command.client = fakeSkupperClient
			},
			expectedErrors: []string{"listener name is not valid: value does not match this regular expression: ^[a-z0-9]([-a-z0-9]*[a-z0-9])*(\\.[a-z0-9]([-a-z0-9]*[a-z0-9])*)*$"},
		},
		{
			name: "listener type is not valid",
			args: []string{"my-listener-type"},
			setUpMock: func(command *CmdListenerUpdate) {
				fakeSkupperClient := &fake.FakeSkupperV1alpha1{Fake: &testing2.Fake{}}
				fakeSkupperClient.Fake.ClearActions()
				command.client = fakeSkupperClient
				command.flags = ListenerUpdates{listenerType: "not-valid"}
			},
			expectedErrors: []string{
				"listener type is not valid: value not-valid not allowed. It should be one of this options: [tcp]"},
		},
		{
			name: "routing key is not valid",
			args: []string{"my-listener-rk"},
			setUpMock: func(command *CmdListenerUpdate) {
				fakeSkupperClient := &fake.FakeSkupperV1alpha1{Fake: &testing2.Fake{}}
				fakeSkupperClient.Fake.ClearActions()
				command.client = fakeSkupperClient
				command.flags = ListenerUpdates{routingKey: "not-valid$"}
			},
			expectedErrors: []string{
				"routing key is not valid: value does not match this regular expression: ^[a-z0-9]([-a-z0-9]*[a-z0-9])*(\\.[a-z0-9]([-a-z0-9]*[a-z0-9])*)*$"},
		},
		{
			name: "host is not valid",
			args: []string{"my-listener-host"},
			setUpMock: func(command *CmdListenerUpdate) {
				fakeSkupperClient := &fake.FakeSkupperV1alpha1{Fake: &testing2.Fake{}}
				fakeSkupperClient.Fake.ClearActions()
				command.client = fakeSkupperClient
				command.flags = ListenerUpdates{host: ":not-Valid"}
			},
			expectedErrors: []string{
				"host name is not valid: value does not match this regular expression: ^[a-z0-9]([-a-z0-9]*[a-z0-9])*(\\.[a-z0-9]([-a-z0-9]*[a-z0-9])*)*$"},
		},
		{
			name: "tls-secret is not valid",
			args: []string{"my-listener-tls"},
			setUpMock: func(command *CmdListenerUpdate) {
				fakeSkupperClient := &fake.FakeSkupperV1alpha1{Fake: &testing2.Fake{}}
				fakeSkupperClient.Fake.ClearActions()
				command.client = fakeSkupperClient
				command.flags = ListenerUpdates{tlsSecret: ":not-valid"}
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
			args: []string{"my-listener-port"},
			setUpMock: func(command *CmdListenerUpdate) {
				fakeSkupperClient := &fake.FakeSkupperV1alpha1{Fake: &testing2.Fake{}}
				fakeSkupperClient.Fake.ClearActions()
				command.client = fakeSkupperClient
				command.flags = ListenerUpdates{port: -1}
			},
			expectedErrors: []string{
				"listener port is not valid: value is not positive"},
		},
		{
			name: "output is not valid",
			args: []string{"bad-output"},
			setUpMock: func(command *CmdListenerUpdate) {
				fakeSkupperClient := &fake.FakeSkupperV1alpha1{Fake: &testing2.Fake{}}
				fakeSkupperClient.Fake.ClearActions()
				command.client = fakeSkupperClient
				command.flags = ListenerUpdates{output: "not-supported"}
			},
			expectedErrors: []string{
				"output type is not valid: value not-supported not allowed. It should be one of this options: [json yaml]"},
		},
		{
			name: "flags all valid",
			args: []string{"my-listener-flags"},
			setUpMock: func(command *CmdListenerUpdate) {
				fakeSkupperClient := &fake.FakeSkupperV1alpha1{Fake: &testing2.Fake{}}
				fakeSkupperClient.Fake.ClearActions()
				command.client = fakeSkupperClient
				command.flags = ListenerUpdates{
					host:         "hostname",
					routingKey:   "routingkeyname",
					tlsSecret:    "secretname",
					port:         1234,
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

func TestCmdListenerUpdate_Run(t *testing.T) {
	type test struct {
		name         string
		setUpMock    func(command *CmdListenerUpdate)
		errorMessage string
	}

	testTable := []test{
		{
			name: "runs ok",
			setUpMock: func(command *CmdListenerUpdate) {
				fakeSkupperClient := &fake.FakeSkupperV1alpha1{Fake: &testing2.Fake{}}
				fakeSkupperClient.Fake.ClearActions()

				fakeSkupperClient.Fake.PrependReactor("update", "listeners", func(action testing2.Action) (handled bool, ret runtime.Object, err error) {
					UpdateAction, ok := action.(testing2.UpdateActionImpl)
					if !ok {
						t.Log("failed 1", UpdateAction, ok)
						return
					}
					UpdatedObject := UpdateAction.GetObject()

					listener, ok := UpdatedObject.(*v1alpha1.Listener)
					if !ok {
						t.Log("failed 2", listener, ok)
						return
					}
					if listener.Name != "my-listener" {
						t.Log("failed 3", listener)
						return true, nil, fmt.Errorf("unexpected name value")
					}
					if listener.Spec.Type != "tcp" {
						t.Log("failed 4", listener)
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
				command.client = fakeSkupperClient
				command.name = "my-listener"
				command.newSettings.port = 8080
				command.newSettings.listenerType = "tcp"
				command.newSettings.host = "hostname"
				command.newSettings.routingKey = "keyname"
				command.newSettings.tlsSecret = "secretname"
			},
		},
		{
			name: "run fails",
			setUpMock: func(command *CmdListenerUpdate) {
				fakeSkupperClient := &fake.FakeSkupperV1alpha1{Fake: &testing2.Fake{}}
				fakeSkupperClient.Fake.ClearActions()
				fakeSkupperClient.Fake.PrependReactor("update", "listeners", func(action testing2.Action) (handled bool, ret runtime.Object, err error) {
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
				command.newSettings.output = "json"
			},
			errorMessage: "error",
		},
	}

	for _, test := range testTable {
		cmd := newCmdListenerUpdateWithMocks()
		test.setUpMock(cmd)

		//create a listener

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

func TestCmdListenerUpdate_WaitUntilReady(t *testing.T) {
	type test struct {
		name        string
		setUpMock   func(command *CmdListenerUpdate)
		expectError bool
	}

	testTable := []test{
		{
			name: "listener is not ready",
			setUpMock: func(command *CmdListenerUpdate) {
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
			},
			expectError: true,
		},
		{
			name: "listener is not returned",
			setUpMock: func(command *CmdListenerUpdate) {
				fakeSkupperClient := &fake.FakeSkupperV1alpha1{Fake: &testing2.Fake{}}
				fakeSkupperClient.Fake.ClearActions()
				fakeSkupperClient.Fake.PrependReactor("get", "listeners", func(action testing2.Action) (handled bool, ret runtime.Object, err error) {
					return true, nil, fmt.Errorf("it failed")
				})
				command.client = fakeSkupperClient
				command.name = "my-listener"
			},
			expectError: true,
		},
		{
			name: "listener is ready",
			setUpMock: func(command *CmdListenerUpdate) {
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
			},
			expectError: false,
		},
		{
			name: "listener is ready json output",
			setUpMock: func(command *CmdListenerUpdate) {
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
				command.newSettings.output = "json"
			},
			expectError: false,
		},
	}

	for _, test := range testTable {
		cmd := newCmdListenerUpdateWithMocks()

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

func newCmdListenerUpdateWithMocks() *CmdListenerUpdate {

	cmdListenerUpdate := &CmdListenerUpdate{
		client:     &fake.FakeSkupperV1alpha1{Fake: &testing2.Fake{}},
		namespace:  "test",
		KubeClient: kubefake.NewSimpleClientset(),
	}

	return cmdListenerUpdate
}

func errorsToMessages(errs []error) []string {
	msgs := make([]string, len(errs))
	for i, err := range errs {
		msgs[i] = err.Error()
	}
	return msgs
}

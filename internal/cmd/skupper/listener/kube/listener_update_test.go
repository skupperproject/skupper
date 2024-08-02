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
		"timeout":     "1m0s",
		"output":      "",
	}
	var flagList []string

	cmd, err := newCmdListenerUpdateWithMocks("test", nil, nil, "")
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

func TestCmdListenerUpdate_ValidateInput(t *testing.T) {
	type test struct {
		name           string
		args           []string
		flags          ListenerUpdates
		k8sObjects     []runtime.Object
		skupperObjects []runtime.Object
		expectedErrors []string
	}

	testTable := []test{
		{
			name:           "listener is not updated because listener does not exist in the namespace",
			args:           []string{"my-listener"},
			flags:          ListenerUpdates{timeout: 1 * time.Minute},
			expectedErrors: []string{"listener my-listener must exist in namespace test to be updated"},
		},
		{
			name:           "listener name is not specified",
			args:           []string{},
			flags:          ListenerUpdates{timeout: 1 * time.Minute},
			expectedErrors: []string{"listener name must be configured"},
		},
		{
			name:           "listener name is nil",
			args:           []string{""},
			flags:          ListenerUpdates{timeout: 1 * time.Minute},
			expectedErrors: []string{"listener name must not be empty"},
		},
		{
			name:           "more than one argument is specified",
			args:           []string{"my", "listener"},
			flags:          ListenerUpdates{timeout: 1 * time.Minute},
			expectedErrors: []string{"only one argument is allowed for this command"},
		},
		{
			name:           "listener name is not valid.",
			args:           []string{"my new listener"},
			flags:          ListenerUpdates{timeout: 1 * time.Minute},
			expectedErrors: []string{"listener name is not valid: value does not match this regular expression: ^[a-z0-9]([-a-z0-9]*[a-z0-9])*(\\.[a-z0-9]([-a-z0-9]*[a-z0-9])*)*$"},
		},
		{
			name: "listener type is not valid",
			args: []string{"my-listener-type"},
			flags: ListenerUpdates{
				listenerType: "not-valid",
				timeout:      60 * time.Second,
			},
			skupperObjects: []runtime.Object{
				&v1alpha1.Listener{
					ObjectMeta: v1.ObjectMeta{
						Name:      "my-listener-type",
						Namespace: "test",
					},
					Status: v1alpha1.Status{
						StatusMessage: "Ok",
					},
				},
			},
			expectedErrors: []string{
				"listener type is not valid: value not-valid not allowed. It should be one of this options: [tcp]"},
		},
		{
			name: "routing key is not valid",
			args: []string{"my-listener-rk"},
			flags: ListenerUpdates{
				routingKey: "not-valid$",
				timeout:    30 * time.Second,
			},
			skupperObjects: []runtime.Object{
				&v1alpha1.Listener{
					ObjectMeta: v1.ObjectMeta{
						Name:      "my-listener-rk",
						Namespace: "test",
					},
					Status: v1alpha1.Status{
						StatusMessage: "Ok",
					},
				},
			},
			expectedErrors: []string{
				"routing key is not valid: value does not match this regular expression: ^[a-z0-9]([-a-z0-9]*[a-z0-9])*(\\.[a-z0-9]([-a-z0-9]*[a-z0-9])*)*$"},
		},
		{
			name: "tls-secret is not valid",
			args: []string{"my-listener-tls"},
			flags: ListenerUpdates{
				tlsSecret: ":not-valid",
				timeout:   5 * time.Minute,
			},
			skupperObjects: []runtime.Object{
				&v1alpha1.Listener{
					ObjectMeta: v1.ObjectMeta{
						Name:      "my-listener-tls",
						Namespace: "test",
					},
					Status: v1alpha1.Status{
						StatusMessage: "Ok",
					},
				},
			},
			expectedErrors: []string{"tls-secret is not valid: does not exist"},
		},
		{
			name: "port is not valid",
			args: []string{"my-listener-port"},
			flags: ListenerUpdates{
				port:    -1,
				timeout: 60 * time.Second,
			},
			skupperObjects: []runtime.Object{
				&v1alpha1.Listener{
					ObjectMeta: v1.ObjectMeta{
						Name:      "my-listener-port",
						Namespace: "test",
					},
					Status: v1alpha1.Status{
						StatusMessage: "Ok",
					},
				},
			},
			expectedErrors: []string{"listener port is not valid: value is not positive"},
		},
		{
			name:  "timeout is not valid",
			args:  []string{"bad-timeout"},
			flags: ListenerUpdates{timeout: 0 * time.Minute},
			skupperObjects: []runtime.Object{
				&v1alpha1.Listener{
					ObjectMeta: v1.ObjectMeta{
						Name:      "bad-timeout",
						Namespace: "test",
					},
					Status: v1alpha1.Status{
						StatusMessage: "Ok",
					},
				},
			},
			expectedErrors: []string{"timeout is not valid"},
		},
		{
			name: "output is not valid",
			args: []string{"bad-output"},
			flags: ListenerUpdates{
				output:  "not-supported",
				timeout: 1 * time.Second,
			},
			skupperObjects: []runtime.Object{
				&v1alpha1.Listener{
					ObjectMeta: v1.ObjectMeta{
						Name:      "bad-output",
						Namespace: "test",
					},
					Status: v1alpha1.Status{
						StatusMessage: "Ok",
					},
				},
			},
			expectedErrors: []string{
				"output type is not valid: value not-supported not allowed. It should be one of this options: [json yaml]"},
		},
		{
			name: "flags all valid",
			args: []string{"my-listener-flags"},
			flags: ListenerUpdates{
				host:         "hostname",
				routingKey:   "routingkeyname",
				tlsSecret:    "secretname",
				port:         1234,
				listenerType: "tcp",
				timeout:      1 * time.Second,
				output:       "json",
			},
			skupperObjects: []runtime.Object{
				&v1alpha1.Listener{
					ObjectMeta: v1.ObjectMeta{
						Name:      "my-listener-flags",
						Namespace: "test",
					},
					Status: v1alpha1.Status{
						StatusMessage: "Ok",
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

			command, err := newCmdListenerUpdateWithMocks("test", test.k8sObjects, test.skupperObjects, "")
			assert.Assert(t, err)

			command.flags = test.flags

			actualErrors := command.ValidateInput(test.args)

			actualErrorsMessages := utils.ErrorsToMessages(actualErrors)

			assert.DeepEqual(t, actualErrorsMessages, test.expectedErrors)

		})
	}
}

func TestCmdListenerUpdate_Run(t *testing.T) {
	type test struct {
		name                string
		listenerName        string
		newOutput           string
		flags               ListenerUpdates
		k8sObjects          []runtime.Object
		skupperObjects      []runtime.Object
		skupperErrorMessage string
		errorMessage        string
	}

	testTable := []test{
		{
			name:         "runs ok",
			listenerName: "run-listener",
			flags: ListenerUpdates{
				listenerType: "tcp",
				host:         "hostname",
				routingKey:   "keyname",
				tlsSecret:    "secretname",
				output:       "yaml",
				timeout:      1 * time.Minute,
			},
			skupperObjects: []runtime.Object{
				&v1alpha1.Listener{
					ObjectMeta: v1.ObjectMeta{
						Name:      "run-listener",
						Namespace: "test",
					},
					Status: v1alpha1.Status{
						StatusMessage: "Ok",
					},
				},
			},
		},
		{
			name:         "new output json",
			listenerName: "run-listener",
			flags: ListenerUpdates{
				timeout: 1 * time.Minute,
			},
			newOutput: "json",
			skupperObjects: []runtime.Object{
				&v1alpha1.Listener{
					ObjectMeta: v1.ObjectMeta{
						Name:      "run-listener",
						Namespace: "test",
					},
					Status: v1alpha1.Status{
						StatusMessage: "Ok",
					},
				},
			},
		},
		{
			name:                "run fails",
			listenerName:        "run-listener",
			skupperErrorMessage: "error",
			errorMessage:        "error",
			flags:               ListenerUpdates{timeout: 1 * time.Minute},
		},
	}

	for _, test := range testTable {
		cmd, err := newCmdListenerUpdateWithMocks("test", test.k8sObjects, test.skupperObjects, test.skupperErrorMessage)
		assert.Assert(t, err)

		cmd.name = test.listenerName
		cmd.flags = test.flags
		cmd.namespace = "test"
		cmd.newSettings.output = test.newOutput

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
		name                string
		output              string
		k8sObjects          []runtime.Object
		skupperObjects      []runtime.Object
		skupperErrorMessage string
		expectError         bool
	}

	testTable := []test{
		{
			name: "listener is not ready",
			skupperObjects: []runtime.Object{
				&v1alpha1.Listener{
					ObjectMeta: v1.ObjectMeta{
						Name:      "my-listener",
						Namespace: "test",
					},
					Status: v1alpha1.Status{
						StatusMessage: "",
					},
				},
			},
			expectError: true,
		},
		{
			name:        "listener is not returned",
			expectError: true,
		},
		{
			name: "listener is ready",
			skupperObjects: []runtime.Object{
				&v1alpha1.Listener{
					ObjectMeta: v1.ObjectMeta{
						Name:      "my-listener",
						Namespace: "test",
					},
					Status: v1alpha1.Status{
						StatusMessage: "Ok",
					},
				},
			},
			expectError: false,
		},
		{
			name:   "listener is ready json output",
			output: "json",
			skupperObjects: []runtime.Object{
				&v1alpha1.Listener{
					ObjectMeta: v1.ObjectMeta{
						Name:      "my-listener",
						Namespace: "test",
					},
					Status: v1alpha1.Status{
						StatusMessage: "Ok",
					},
				},
			},
			expectError: false,
		},
	}

	for _, test := range testTable {
		cmd, err := newCmdListenerUpdateWithMocks("test", test.k8sObjects, test.skupperObjects, test.skupperErrorMessage)
		assert.Assert(t, err)

		cmd.name = "my-listener"
		cmd.flags = ListenerUpdates{
			timeout: 1 * time.Second,
			output:  test.output,
		}
		cmd.namespace = "test"
		cmd.newSettings.output = cmd.flags.output

		t.Run(test.name, func(t *testing.T) {
			err := cmd.WaitUntilReady()
			if test.expectError {
				assert.Check(t, err != nil)
			} else {
				assert.Assert(t, err)
			}
		})
	}
}

// --- helper methods

func newCmdListenerUpdateWithMocks(namespace string, k8sObjects []runtime.Object, skupperObjects []runtime.Object, fakeSkupperError string) (*CmdListenerUpdate, error) {

	client, err := fakeclient.NewFakeClient(namespace, k8sObjects, skupperObjects, fakeSkupperError)
	if err != nil {
		return nil, err
	}
	cmdListenerUpdate := &CmdListenerUpdate{
		client:     client.GetSkupperClient().SkupperV1alpha1(),
		KubeClient: client.GetKubeClient(),
		namespace:  namespace,
	}
	return cmdListenerUpdate, nil
}

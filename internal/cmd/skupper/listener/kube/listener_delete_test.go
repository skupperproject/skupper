package kube

import (
	"testing"
	"time"

	"github.com/skupperproject/skupper/internal/cmd/skupper/utils"
	fakeclient "github.com/skupperproject/skupper/internal/kube/client/fake"
	"github.com/skupperproject/skupper/pkg/apis/skupper/v1alpha1"
	"github.com/spf13/pflag"
	"gotest.tools/assert"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestCmdListenerDelete_NewCmdListenerDelete(t *testing.T) {

	t.Run("Delete command", func(t *testing.T) {

		result := NewCmdListenerDelete()

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

func TestCmdListenerDelete_AddFlags(t *testing.T) {

	expectedFlagsWithDefaultValue := map[string]interface{}{
		"timeout": "1m0s",
	}
	var flagList []string

	cmd, err := newCmdListenerDeleteWithMocks("test", nil, nil, "")
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
func TestCmdListenerDelete_ValidateInput(t *testing.T) {
	type test struct {
		name                string
		args                []string
		flags               ListenerDelete
		k8sObjects          []runtime.Object
		skupperObjects      []runtime.Object
		skupperErrorMessage string
		expectedErrors      []string
	}

	testTable := []test{
		{
			name:           "listener is not deleted because listener does not exist in the namespace",
			args:           []string{"my-listener"},
			flags:          ListenerDelete{timeout: 1 * time.Minute},
			expectedErrors: []string{"listener my-listener does not exist in namespace test"},
		},
		{
			name:           "listener name is not specified",
			args:           []string{},
			flags:          ListenerDelete{timeout: 1 * time.Minute},
			expectedErrors: []string{"listener name must be specified"},
		},
		{
			name:           "listener name is nil",
			args:           []string{""},
			flags:          ListenerDelete{timeout: 1 * time.Minute},
			expectedErrors: []string{"listener name must not be empty"},
		},
		{
			name:           "listener name is not valid",
			args:           []string{"my name"},
			flags:          ListenerDelete{timeout: 1 * time.Minute},
			expectedErrors: []string{"listener name is not valid: value does not match this regular expression: ^[a-z0-9]([-a-z0-9]*[a-z0-9])*(\\.[a-z0-9]([-a-z0-9]*[a-z0-9])*)*$"},
		},
		{
			name:           "more than one argument is specified",
			args:           []string{"my", "listener"},
			flags:          ListenerDelete{timeout: 1 * time.Minute},
			expectedErrors: []string{"only one argument is allowed for this command"},
		},
		{
			name: "timeout is not valid",
			args: []string{"bad-timeout"},
			skupperObjects: []runtime.Object{
				&v1alpha1.Listener{
					ObjectMeta: v1.ObjectMeta{
						Name:      "bad-timeout",
						Namespace: "test",
					},
					Status: v1alpha1.ListenerStatus{
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
			flags:               ListenerDelete{timeout: 0 * time.Minute},
			skupperErrorMessage: "timeout is not valid",
			expectedErrors:      []string{"timeout is not valid"},
		},
	}

	for _, test := range testTable {
		t.Run(test.name, func(t *testing.T) {

			command, err := newCmdListenerDeleteWithMocks("test", test.k8sObjects, test.skupperObjects, "")
			assert.Assert(t, err)

			command.flags = test.flags

			actualErrors := command.ValidateInput(test.args)

			actualErrorsMessages := utils.ErrorsToMessages(actualErrors)

			assert.DeepEqual(t, actualErrorsMessages, test.expectedErrors)
		})
	}
}

func TestCmdListenerDelete_Run(t *testing.T) {
	type test struct {
		name                string
		listenerName        string
		k8sObjects          []runtime.Object
		skupperObjects      []runtime.Object
		skupperErrorMessage string
		errorMessage        string
	}

	testTable := []test{
		{
			name:         "runs ok",
			listenerName: "listener-delete",
			skupperObjects: []runtime.Object{
				&v1alpha1.Listener{
					ObjectMeta: v1.ObjectMeta{
						Name:      "listener-delete",
						Namespace: "test",
					},
					Status: v1alpha1.ListenerStatus{
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
			name:                "run fails",
			listenerName:        "my-fail-listener",
			skupperErrorMessage: "error",
			errorMessage:        "error",
		},
	}

	for _, test := range testTable {
		cmd, err := newCmdListenerDeleteWithMocks("test", test.k8sObjects, test.skupperObjects, test.skupperErrorMessage)
		assert.Assert(t, err)

		cmd.name = test.listenerName
		cmd.namespace = "test"

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

func TestCmdListenerDelete_WaitUntilReady(t *testing.T) {
	type test struct {
		name                string
		k8sObjects          []runtime.Object
		skupperObjects      []runtime.Object
		skupperErrorMessage string
		expectError         bool
	}

	testTable := []test{
		{
			name: "error deleting listener",
			skupperObjects: []runtime.Object{
				&v1alpha1.Listener{
					ObjectMeta: v1.ObjectMeta{
						Name:      "my-listener",
						Namespace: "test",
					},
					Status: v1alpha1.ListenerStatus{
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
			expectError: true,
		},
		{
			name:        "listener is deleted",
			expectError: false,
		},
	}

	for _, test := range testTable {
		cmd, err := newCmdListenerDeleteWithMocks("test", test.k8sObjects, test.skupperObjects, test.skupperErrorMessage)
		assert.Assert(t, err)

		cmd.name = "my-listener"
		cmd.flags = ListenerDelete{timeout: 1 * time.Second}
		cmd.namespace = "test"

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

func newCmdListenerDeleteWithMocks(namespace string, k8sObjects []runtime.Object, skupperObjects []runtime.Object, fakeSkupperError string) (*CmdListenerDelete, error) {

	client, err := fakeclient.NewFakeClient(namespace, k8sObjects, skupperObjects, fakeSkupperError)
	if err != nil {
		return nil, err
	}
	cmdListenerDelete := &CmdListenerDelete{
		client:    client.GetSkupperClient().SkupperV1alpha1(),
		namespace: namespace,
	}
	return cmdListenerDelete, nil
}

package kube

import (
	"github.com/skupperproject/skupper/internal/cmd/skupper/common"
	"github.com/skupperproject/skupper/internal/cmd/skupper/common/utils"
	"testing"
	"time"

	fakeclient "github.com/skupperproject/skupper/internal/kube/client/fake"
	"github.com/skupperproject/skupper/pkg/apis/skupper/v1alpha1"
	"gotest.tools/assert"
	v12 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestCmdListenerCreate_ValidateInput(t *testing.T) {
	type test struct {
		name                string
		args                []string
		flags               common.CommandListenerCreateFlags
		k8sObjects          []runtime.Object
		skupperObjects      []runtime.Object
		skupperErrorMessage string
		expectedErrors      []string
	}

	testTable := []test{
		{
			name:  "listener is not created because there is already the same listener in the namespace",
			args:  []string{"my-listener", "8080"},
			flags: common.CommandListenerCreateFlags{Timeout: 1 * time.Minute},
			skupperObjects: []runtime.Object{
				&v1alpha1.SiteList{
					Items: []v1alpha1.Site{
						{
							ObjectMeta: v1.ObjectMeta{
								Name:      "the-site",
								Namespace: "test",
							},
							Status: v1alpha1.SiteStatus{
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
			skupperErrorMessage: "AllReadyExists",
			expectedErrors: []string{
				"there is already a listener my-listener created for namespace test"},
		},
		{
			name:           "listener no site",
			args:           []string{"listener-site", "8090"},
			flags:          common.CommandListenerCreateFlags{Timeout: 1 * time.Minute},
			expectedErrors: []string{"A site must exist in namespace test before a listener can be created"},
		},
		{
			name:  "listener no site with ok status",
			args:  []string{"listener-site", "8090"},
			flags: common.CommandListenerCreateFlags{Timeout: 1 * time.Minute},
			skupperObjects: []runtime.Object{
				&v1alpha1.SiteList{
					Items: []v1alpha1.Site{
						{
							ObjectMeta: v1.ObjectMeta{
								Name:      "the-site",
								Namespace: "test",
							},
							Status: v1alpha1.SiteStatus{
								Status: v1alpha1.Status{
									StatusMessage: "",
								},
							},
						},
					},
				},
			},
			expectedErrors: []string{"there is no active skupper site in this namespace"},
		},
		{
			name:  "listener name and port are not specified",
			args:  []string{},
			flags: common.CommandListenerCreateFlags{Timeout: 1 * time.Minute},
			skupperObjects: []runtime.Object{
				&v1alpha1.SiteList{
					Items: []v1alpha1.Site{
						{
							ObjectMeta: v1.ObjectMeta{
								Name:      "the-site",
								Namespace: "test",
							},
							Status: v1alpha1.SiteStatus{
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
			},
			expectedErrors: []string{"listener name and port must be configured"},
		},
		{
			name:  "listener name empty",
			args:  []string{"", "8090"},
			flags: common.CommandListenerCreateFlags{Timeout: 1 * time.Minute},
			skupperObjects: []runtime.Object{
				&v1alpha1.SiteList{
					Items: []v1alpha1.Site{
						{
							ObjectMeta: v1.ObjectMeta{
								Name:      "the-site",
								Namespace: "test",
							},
							Status: v1alpha1.SiteStatus{
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
			},
			expectedErrors: []string{"listener name must not be empty"},
		},
		{
			name:  "listener port empty",
			args:  []string{"my-name-port-empty", ""},
			flags: common.CommandListenerCreateFlags{Timeout: 1 * time.Minute},
			skupperObjects: []runtime.Object{
				&v1alpha1.SiteList{
					Items: []v1alpha1.Site{
						{
							ObjectMeta: v1.ObjectMeta{
								Name:      "the-site",
								Namespace: "test",
							},
							Status: v1alpha1.SiteStatus{
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
			},
			expectedErrors: []string{"listener port must not be empty"},
		},
		{
			name:  "listener port not positive",
			args:  []string{"my-port-positive", "-45"},
			flags: common.CommandListenerCreateFlags{Timeout: 1 * time.Minute},
			skupperObjects: []runtime.Object{
				&v1alpha1.SiteList{
					Items: []v1alpha1.Site{
						{
							ObjectMeta: v1.ObjectMeta{
								Name:      "the-site",
								Namespace: "test",
							},
							Status: v1alpha1.SiteStatus{
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
			},
			expectedErrors: []string{"listener port is not valid: value is not positive"},
		},
		{
			name:  "listener name and port are not specified",
			args:  []string{},
			flags: common.CommandListenerCreateFlags{Timeout: 1 * time.Minute},
			skupperObjects: []runtime.Object{
				&v1alpha1.SiteList{
					Items: []v1alpha1.Site{
						{
							ObjectMeta: v1.ObjectMeta{
								Name:      "the-site",
								Namespace: "test",
							},
							Status: v1alpha1.SiteStatus{
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
			},
			expectedErrors: []string{"listener name and port must be configured"},
		},
		{
			name:  "listener port is not specified",
			args:  []string{"my-name"},
			flags: common.CommandListenerCreateFlags{Timeout: 1 * time.Minute},
			skupperObjects: []runtime.Object{
				&v1alpha1.SiteList{
					Items: []v1alpha1.Site{
						{
							ObjectMeta: v1.ObjectMeta{
								Name:      "the-site",
								Namespace: "test",
							},
							Status: v1alpha1.SiteStatus{
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
			},
			expectedErrors: []string{"listener name and port must be configured"},
		},
		{
			name:  "more than two arguments are specified",
			args:  []string{"my", "listener", "8080"},
			flags: common.CommandListenerCreateFlags{Timeout: 1 * time.Minute},
			skupperObjects: []runtime.Object{
				&v1alpha1.SiteList{
					Items: []v1alpha1.Site{
						{
							ObjectMeta: v1.ObjectMeta{
								Name:      "the-site",
								Namespace: "test",
							},
							Status: v1alpha1.SiteStatus{
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
			},
			expectedErrors: []string{"only two arguments are allowed for this command"},
		},
		{
			name:  "listener name is not valid.",
			args:  []string{"my new listener", "8080"},
			flags: common.CommandListenerCreateFlags{Timeout: 1 * time.Minute},
			skupperObjects: []runtime.Object{
				&v1alpha1.SiteList{
					Items: []v1alpha1.Site{
						{
							ObjectMeta: v1.ObjectMeta{
								Name:      "the-site",
								Namespace: "test",
							},
							Status: v1alpha1.SiteStatus{
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
			},
			expectedErrors: []string{
				"listener name is not valid: value does not match this regular expression: ^[a-z0-9]([-a-z0-9]*[a-z0-9])*(\\.[a-z0-9]([-a-z0-9]*[a-z0-9])*)*$"},
		},
		{
			name:  "port is not valid.",
			args:  []string{"my-listener-port", "abcd"},
			flags: common.CommandListenerCreateFlags{Timeout: 1 * time.Minute},
			skupperObjects: []runtime.Object{
				&v1alpha1.SiteList{
					Items: []v1alpha1.Site{
						{
							ObjectMeta: v1.ObjectMeta{
								Name:      "the-site",
								Namespace: "test",
							},
							Status: v1alpha1.SiteStatus{
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
			},
			expectedErrors: []string{
				"listener port is not valid: strconv.Atoi: parsing \"abcd\": invalid syntax"},
		},
		{
			name: "listener type is not valid",
			args: []string{"my-listener-type", "8080"},
			flags: common.CommandListenerCreateFlags{
				Timeout:      1 * time.Minute,
				ListenerType: "not-valid",
			},
			skupperObjects: []runtime.Object{
				&v1alpha1.SiteList{
					Items: []v1alpha1.Site{
						{
							ObjectMeta: v1.ObjectMeta{
								Name:      "the-site",
								Namespace: "test",
							},
							Status: v1alpha1.SiteStatus{
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
			},
			expectedErrors: []string{
				"listener type is not valid: value not-valid not allowed. It should be one of this options: [tcp]"},
		},
		{
			name: "routing key is not valid",
			args: []string{"my-listener-rk", "8080"},
			flags: common.CommandListenerCreateFlags{
				Timeout:    60 * time.Second,
				RoutingKey: "not-valid$",
			},
			skupperObjects: []runtime.Object{
				&v1alpha1.SiteList{
					Items: []v1alpha1.Site{
						{
							ObjectMeta: v1.ObjectMeta{
								Name:      "the-site",
								Namespace: "test",
							},
							Status: v1alpha1.SiteStatus{
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
			},
			expectedErrors: []string{
				"routing key is not valid: value does not match this regular expression: ^[a-z0-9]([-a-z0-9]*[a-z0-9])*(\\.[a-z0-9]([-a-z0-9]*[a-z0-9])*)*$"},
		},
		{
			name: "tls-secret does not exist",
			args: []string{"my-listener-tls", "8080"},
			flags: common.CommandListenerCreateFlags{
				Timeout:   1 * time.Minute,
				TlsSecret: "not-valid",
			},
			skupperObjects: []runtime.Object{
				&v1alpha1.SiteList{
					Items: []v1alpha1.Site{
						{
							ObjectMeta: v1.ObjectMeta{
								Name:      "the-site",
								Namespace: "test",
							},
							Status: v1alpha1.SiteStatus{
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
			},
			expectedErrors: []string{"tls-secret is not valid: does not exist"},
		},
		{
			name:  "timeout is not valid",
			args:  []string{"bad-timeout", "8080"},
			flags: common.CommandListenerCreateFlags{Timeout: 0 * time.Second},
			skupperObjects: []runtime.Object{
				&v1alpha1.SiteList{
					Items: []v1alpha1.Site{
						{
							ObjectMeta: v1.ObjectMeta{
								Name:      "the-site",
								Namespace: "test",
							},
							Status: v1alpha1.SiteStatus{
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
			},
			expectedErrors: []string{"timeout is not valid"},
		},
		{
			name: "output is not valid",
			args: []string{"bad-output", "1234"},
			flags: common.CommandListenerCreateFlags{
				Timeout: 30 * time.Second,
				Output:  "not-supported",
			},
			skupperObjects: []runtime.Object{
				&v1alpha1.SiteList{
					Items: []v1alpha1.Site{
						{
							ObjectMeta: v1.ObjectMeta{
								Name:      "the-site",
								Namespace: "test",
							},
							Status: v1alpha1.SiteStatus{
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
			},
			expectedErrors: []string{
				"output type is not valid: value not-supported not allowed. It should be one of this options: [json yaml]"},
		},
		{
			name: "flags all valid",
			args: []string{"my-listener-flags", "8080"},
			flags: common.CommandListenerCreateFlags{
				Host:         "hostname",
				RoutingKey:   "routingkeyname",
				TlsSecret:    "secretname",
				ListenerType: "tcp",
				Timeout:      1 * time.Minute,
				Output:       "json",
			},
			skupperObjects: []runtime.Object{
				&v1alpha1.SiteList{
					Items: []v1alpha1.Site{
						{
							ObjectMeta: v1.ObjectMeta{
								Name:      "the-site",
								Namespace: "test",
							},
							Status: v1alpha1.SiteStatus{
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

			command, err := newCmdListenerCreateWithMocks("test", test.k8sObjects, test.skupperObjects, "")
			assert.Assert(t, err)

			command.Flags = &test.flags

			actualErrors := command.ValidateInput(test.args)

			actualErrorsMessages := utils.ErrorsToMessages(actualErrors)

			assert.DeepEqual(t, actualErrorsMessages, test.expectedErrors)

		})
	}
}

func TestCmdListenerCreate_Run(t *testing.T) {
	type test struct {
		name                string
		listenerName        string
		listenerPort        int
		flags               common.CommandListenerCreateFlags
		k8sObjects          []runtime.Object
		skupperObjects      []runtime.Object
		skupperErrorMessage string
		errorMessage        string
	}

	testTable := []test{
		{
			name:         "runs ok",
			listenerName: "run-listener",
			listenerPort: 8080,
			flags: common.CommandListenerCreateFlags{
				ListenerType: "tcp",
				Host:         "hostname",
				RoutingKey:   "keyname",
				TlsSecret:    "secretname",
			},
		},
		{
			name:         "output yaml",
			listenerName: "run-listener",
			listenerPort: 8080,
			flags: common.CommandListenerCreateFlags{
				ListenerType: "tcp",
				Host:         "hostname",
				RoutingKey:   "keyname",
				TlsSecret:    "secretname",
				Output:       "yaml",
			},
		},
	}

	for _, test := range testTable {
		cmd, err := newCmdListenerCreateWithMocks("test", test.k8sObjects, test.skupperObjects, test.skupperErrorMessage)
		assert.Assert(t, err)
		cmd.name = test.listenerName
		cmd.port = test.listenerPort
		cmd.Flags = &test.flags
		cmd.output = cmd.Flags.Output
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

func TestCmdListenerCreate_WaitUntil(t *testing.T) {
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
					Status: v1alpha1.ListenerStatus{
						Status: v1alpha1.Status{},
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
			expectError: false,
		},
		{
			name:   "listener is ready yaml output",
			output: "yaml",
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
			expectError: false,
		},
	}

	for _, test := range testTable {
		cmd, err := newCmdListenerCreateWithMocks("test", test.k8sObjects, test.skupperObjects, test.skupperErrorMessage)
		assert.Assert(t, err)

		cmd.name = "my-listener"
		cmd.port = 8080
		cmd.Flags = &common.CommandListenerCreateFlags{
			Timeout: 1 * time.Second,
			Output:  test.output,
		}
		cmd.output = cmd.Flags.Output
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

func newCmdListenerCreateWithMocks(namespace string, k8sObjects []runtime.Object, skupperObjects []runtime.Object, fakeSkupperError string) (*CmdListenerCreate, error) {

	// We make sure the interval is appropriate
	utils.SetRetryProfile(utils.TestRetryProfile)
	client, err := fakeclient.NewFakeClient(namespace, k8sObjects, skupperObjects, fakeSkupperError)
	if err != nil {
		return nil, err
	}
	cmdListenerCreate := &CmdListenerCreate{
		client:     client.GetSkupperClient().SkupperV1alpha1(),
		KubeClient: client.GetKubeClient(),
		namespace:  namespace,
	}
	return cmdListenerCreate, nil
}

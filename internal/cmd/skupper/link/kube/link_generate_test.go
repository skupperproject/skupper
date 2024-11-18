package kube

import (
	"testing"
	"time"

	"github.com/skupperproject/skupper/internal/cmd/skupper/common"
	"github.com/skupperproject/skupper/internal/cmd/skupper/common/utils"
	fakeclient "github.com/skupperproject/skupper/internal/kube/client/fake"
	"github.com/skupperproject/skupper/pkg/apis/skupper/v2alpha1"
	"gotest.tools/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestCmdLinkGenerate_ValidateInput(t *testing.T) {
	type test struct {
		name           string
		args           []string
		flags          common.CommandLinkGenerateFlags
		k8sObjects     []runtime.Object
		skupperObjects []runtime.Object
		expectedErrors []string
	}

	testTable := []test{
		{
			name:           "link yaml is not generated because there is no site in the namespace.",
			expectedErrors: []string{"there is no skupper site in this namespace", "there is no active skupper site in this namespace"},
			flags:          common.CommandLinkGenerateFlags{Cost: "1", TlsCredentials: "secret", Timeout: time.Minute},
		},
		{
			name: "arguments were specified and they are not needed",
			args: []string{"something"},
			skupperObjects: []runtime.Object{
				&v2alpha1.SiteList{
					Items: []v2alpha1.Site{
						{
							ObjectMeta: v1.ObjectMeta{
								Name:      "the-site",
								Namespace: "test",
							},
							Status: v2alpha1.SiteStatus{
								Status: v2alpha1.Status{
									Message: "OK",
									Conditions: []v1.Condition{
										{
											Message:            "OK",
											ObservedGeneration: 1,
											Reason:             "OK",
											Status:             "True",
											Type:               "Running",
										},
										{
											Message:            "OK",
											ObservedGeneration: 1,
											Reason:             "OK",
											Status:             "True",
											Type:               "Resolved",
										},
										{
											Message:            "OK",
											ObservedGeneration: 1,
											Reason:             "OK",
											Status:             "True",
											Type:               "Configured",
										},
										{
											Message:            "OK",
											ObservedGeneration: 1,
											Reason:             "OK",
											Status:             "True",
											Type:               "Ready",
										},
									},
								},
							},
						},
					},
				},
			},
			flags:          common.CommandLinkGenerateFlags{Cost: "1", TlsCredentials: "secret", Timeout: time.Minute},
			expectedErrors: []string{"arguments are not allowed in this command"},
		},
		{
			name: "tls secret was not specified",
			skupperObjects: []runtime.Object{
				&v2alpha1.SiteList{
					Items: []v2alpha1.Site{
						{
							ObjectMeta: v1.ObjectMeta{
								Name:      "the-site",
								Namespace: "test",
							},
							Status: v2alpha1.SiteStatus{
								Status: v2alpha1.Status{
									Message: "OK",
									Conditions: []v1.Condition{
										{
											Message:            "OK",
											ObservedGeneration: 1,
											Reason:             "OK",
											Status:             "True",
											Type:               "Running",
										},
										{
											Message:            "OK",
											ObservedGeneration: 1,
											Reason:             "OK",
											Status:             "True",
											Type:               "Resolved",
										},
										{
											Message:            "OK",
											ObservedGeneration: 1,
											Reason:             "OK",
											Status:             "True",
											Type:               "Configured",
										},
										{
											Message:            "OK",
											ObservedGeneration: 1,
											Reason:             "OK",
											Status:             "True",
											Type:               "Ready",
										},
									},
								},
							},
						},
					},
				},
			},
			flags:          common.CommandLinkGenerateFlags{Cost: "1", Timeout: time.Minute},
			expectedErrors: []string{"the TLS secret name was not specified"},
		},
		{
			name: "cost is not valid.",
			skupperObjects: []runtime.Object{
				&v2alpha1.SiteList{
					Items: []v2alpha1.Site{
						{
							ObjectMeta: v1.ObjectMeta{
								Name:      "the-site",
								Namespace: "test",
							},
							Status: v2alpha1.SiteStatus{
								Status: v2alpha1.Status{
									Message: "OK",
									Conditions: []v1.Condition{
										{
											Message:            "OK",
											ObservedGeneration: 1,
											Reason:             "OK",
											Status:             "True",
											Type:               "Running",
										},
										{
											Message:            "OK",
											ObservedGeneration: 1,
											Reason:             "OK",
											Status:             "True",
											Type:               "Resolved",
										},
										{
											Message:            "OK",
											ObservedGeneration: 1,
											Reason:             "OK",
											Status:             "True",
											Type:               "Configured",
										},
										{
											Message:            "OK",
											ObservedGeneration: 1,
											Reason:             "OK",
											Status:             "True",
											Type:               "Ready",
										},
									},
								},
							},
						},
					},
				},
			},
			flags:          common.CommandLinkGenerateFlags{Cost: "one", TlsCredentials: "secret", Timeout: time.Minute},
			expectedErrors: []string{"link cost is not valid: strconv.Atoi: parsing \"one\": invalid syntax"},
		},
		{
			name: "cost is not positive",
			skupperObjects: []runtime.Object{
				&v2alpha1.SiteList{
					Items: []v2alpha1.Site{
						{
							ObjectMeta: v1.ObjectMeta{
								Name:      "the-site",
								Namespace: "test",
							},
							Status: v2alpha1.SiteStatus{
								Status: v2alpha1.Status{
									Message: "OK",
									Conditions: []v1.Condition{
										{
											Message:            "OK",
											ObservedGeneration: 1,
											Reason:             "OK",
											Status:             "True",
											Type:               "Running",
										},
										{
											Message:            "OK",
											ObservedGeneration: 1,
											Reason:             "OK",
											Status:             "True",
											Type:               "Resolved",
										},
										{
											Message:            "OK",
											ObservedGeneration: 1,
											Reason:             "OK",
											Status:             "True",
											Type:               "Configured",
										},
										{
											Message:            "OK",
											ObservedGeneration: 1,
											Reason:             "OK",
											Status:             "True",
											Type:               "Ready",
										},
									},
								},
							},
						},
					},
				},
			},
			flags: common.CommandLinkGenerateFlags{Cost: "-4", TlsCredentials: "secret", Timeout: time.Minute},
			expectedErrors: []string{
				"link cost is not valid: value is not positive",
			},
		},
		{
			name: "output format is not valid",
			skupperObjects: []runtime.Object{
				&v2alpha1.SiteList{
					Items: []v2alpha1.Site{
						{
							ObjectMeta: v1.ObjectMeta{
								Name:      "the-site",
								Namespace: "test",
							},
							Status: v2alpha1.SiteStatus{
								Status: v2alpha1.Status{
									Message: "OK",
									Conditions: []v1.Condition{
										{
											Message:            "OK",
											ObservedGeneration: 1,
											Reason:             "OK",
											Status:             "True",
											Type:               "Running",
										},
										{
											Message:            "OK",
											ObservedGeneration: 1,
											Reason:             "OK",
											Status:             "True",
											Type:               "Resolved",
										},
										{
											Message:            "OK",
											ObservedGeneration: 1,
											Reason:             "OK",
											Status:             "True",
											Type:               "Configured",
										},
										{
											Message:            "OK",
											ObservedGeneration: 1,
											Reason:             "OK",
											Status:             "True",
											Type:               "Ready",
										},
									},
								},
							},
						},
					},
				},
			},
			flags: common.CommandLinkGenerateFlags{Cost: "1", TlsCredentials: "secret", Output: "not-valid", Timeout: time.Minute},
			expectedErrors: []string{
				"output type is not valid: value not-valid not allowed. It should be one of this options: [json yaml]",
			},
		},
		{
			name: "tls secret name is not valid",
			skupperObjects: []runtime.Object{
				&v2alpha1.SiteList{
					Items: []v2alpha1.Site{
						{
							ObjectMeta: v1.ObjectMeta{
								Name:      "the-site",
								Namespace: "test",
							},
							Status: v2alpha1.SiteStatus{
								Status: v2alpha1.Status{
									Message: "OK",
									Conditions: []v1.Condition{
										{
											Message:            "OK",
											ObservedGeneration: 1,
											Reason:             "OK",
											Status:             "True",
											Type:               "Running",
										},
										{
											Message:            "OK",
											ObservedGeneration: 1,
											Reason:             "OK",
											Status:             "True",
											Type:               "Resolved",
										},
										{
											Message:            "OK",
											ObservedGeneration: 1,
											Reason:             "OK",
											Status:             "True",
											Type:               "Configured",
										},
										{
											Message:            "OK",
											ObservedGeneration: 1,
											Reason:             "OK",
											Status:             "True",
											Type:               "Ready",
										},
									},
								},
							},
						},
					},
				},
			},
			flags: common.CommandLinkGenerateFlags{Cost: "1", TlsCredentials: "tls secret", Timeout: time.Minute},
			expectedErrors: []string{
				"the name of the tls secret is not valid: value does not match this regular expression: ^[a-z0-9]([-a-z0-9]*[a-z0-9])*(\\.[a-z0-9]([-a-z0-9]*[a-z0-9])*)*$",
			},
		},
		{
			name: "timeout is not valid",
			skupperObjects: []runtime.Object{
				&v2alpha1.SiteList{
					Items: []v2alpha1.Site{
						{
							ObjectMeta: v1.ObjectMeta{
								Name:      "the-site",
								Namespace: "test",
							},
							Status: v2alpha1.SiteStatus{
								Status: v2alpha1.Status{
									Message: "OK",
									Conditions: []v1.Condition{
										{
											Message:            "OK",
											ObservedGeneration: 1,
											Reason:             "OK",
											Status:             "True",
											Type:               "Running",
										},
										{
											Message:            "OK",
											ObservedGeneration: 1,
											Reason:             "OK",
											Status:             "True",
											Type:               "Resolved",
										},
										{
											Message:            "OK",
											ObservedGeneration: 1,
											Reason:             "OK",
											Status:             "True",
											Type:               "Configured",
										},
										{
											Message:            "OK",
											ObservedGeneration: 1,
											Reason:             "OK",
											Status:             "True",
											Type:               "Ready",
										},
									},
								},
							},
						},
					},
				},
			},
			flags: common.CommandLinkGenerateFlags{Cost: "1", TlsCredentials: "secret", Timeout: time.Second * 0},
			expectedErrors: []string{
				"timeout is not valid: duration must not be less than 10s; got 0s",
			},
		},
	}

	for _, test := range testTable {
		t.Run(test.name, func(t *testing.T) {

			command, err := newCmdLinkGenerateWithMocks("test", nil, test.skupperObjects, "")
			assert.Assert(t, err)
			command.Flags = &test.flags

			actualErrors := command.ValidateInput(test.args)

			actualErrorsMessages := utils.ErrorsToMessages(actualErrors)

			assert.DeepEqual(t, actualErrorsMessages, test.expectedErrors)

		})
	}
}

func TestCmdLinkGenerate_InputToOptions(t *testing.T) {

	type test struct {
		name                        string
		args                        []string
		flags                       common.CommandLinkGenerateFlags
		activeSite                  *v2alpha1.Site
		expectedLinkname            string
		expectedTlsCredentials      string
		expectedCost                int
		expectedOutput              string
		expectedGenerateCredentials bool
	}

	testTable := []test{
		{
			name:                        "check options",
			flags:                       common.CommandLinkGenerateFlags{"secret", "1", "json", false, time.Minute},
			expectedCost:                1,
			expectedTlsCredentials:      "secret",
			expectedOutput:              "json",
			expectedGenerateCredentials: false,
		},
		{
			name:  "credentials are not needed",
			flags: common.CommandLinkGenerateFlags{"", "1", "json", true, time.Minute},
			activeSite: &v2alpha1.Site{

				ObjectMeta: v1.ObjectMeta{
					Name:      "the-site",
					Namespace: "test",
				},
				Status: v2alpha1.SiteStatus{
					Status: v2alpha1.Status{
						Message: "OK",
						Conditions: []v1.Condition{
							{
								Message:            "OK",
								ObservedGeneration: 1,
								Reason:             "OK",
								Status:             "True",
								Type:               "Configured",
							},
						},
					},
				},
			},
			expectedLinkname:            "link-the-site",
			expectedCost:                1,
			expectedTlsCredentials:      "link-the-site",
			expectedOutput:              "json",
			expectedGenerateCredentials: true,
		},
	}

	for _, test := range testTable {
		t.Run(test.name, func(t *testing.T) {

			cmd, err := newCmdLinkGenerateWithMocks("test", nil, nil, "")
			assert.Assert(t, err)

			cmd.Flags = &test.flags
			cmd.activeSite = test.activeSite

			cmd.InputToOptions()

			assert.Check(t, cmd.linkName == test.expectedLinkname)
			assert.Check(t, cmd.output == test.expectedOutput)
			assert.Check(t, cmd.tlsCredentials == test.expectedTlsCredentials)
			assert.Check(t, cmd.cost == test.expectedCost)
			assert.Check(t, cmd.generateCredential == test.expectedGenerateCredentials)
		})
	}
}

func TestCmdLinkGenerate_Run(t *testing.T) {
	type test struct {
		name              string
		setUpMock         func(command *CmdLinkGenerate)
		errorMessage      string
		skCliErrorMessage string
	}

	testTable := []test{
		{
			name: "runs ok without generating credentials",
			setUpMock: func(command *CmdLinkGenerate) {
				command.linkName = "my-link"
				command.cost = 1
				command.tlsCredentials = "secret"
				command.output = "yaml"
				command.generateCredential = true
				command.activeSite = &v2alpha1.Site{

					ObjectMeta: v1.ObjectMeta{
						Name:      "the-site",
						Namespace: "test",
					},
					Status: v2alpha1.SiteStatus{
						Status: v2alpha1.Status{
							Message: "OK",
							Conditions: []v1.Condition{
								{
									Message:            "OK",
									ObservedGeneration: 1,
									Reason:             "OK",
									Status:             "True",
									Type:               "Configured",
								},
							},
						},
						Endpoints: []v2alpha1.Endpoint{
							{
								Name:  "inter-router",
								Host:  "127.0.0.1",
								Port:  "8080",
								Group: "skupper-router-1",
							},
							{
								Name:  "edge",
								Host:  "127.0.0.1",
								Port:  "8080",
								Group: "skupper-router-1",
							},
						},
					},
				}
			},
		},
		{
			name: "runs ok generating credentials",
			setUpMock: func(command *CmdLinkGenerate) {
				command.linkName = "my-link"
				command.cost = 1
				command.tlsCredentials = "secret"
				command.output = "yaml"
				command.generateCredential = false
				command.activeSite = &v2alpha1.Site{

					ObjectMeta: v1.ObjectMeta{
						Name:      "the-site",
						Namespace: "test",
					},
					Status: v2alpha1.SiteStatus{
						Status: v2alpha1.Status{
							Message: "OK",
							Conditions: []v1.Condition{
								{
									Message:            "OK",
									ObservedGeneration: 1,
									Reason:             "OK",
									Status:             "True",
									Type:               "Configured",
								},
							},
						},
						Endpoints: []v2alpha1.Endpoint{
							{
								Name:  "inter-router",
								Host:  "127.0.0.1",
								Port:  "8080",
								Group: "skupper-router-1",
							},
							{
								Name:  "edge",
								Host:  "127.0.0.1",
								Port:  "8080",
								Group: "skupper-router-1",
							},
						},
					},
				}
			},
		},
		{
			name: "runs fails because the output format is not supported",
			setUpMock: func(command *CmdLinkGenerate) {
				command.linkName = "my-link"
				command.cost = 1
				command.tlsCredentials = "secret"
				command.output = ""
				command.generateCredential = true
				command.activeSite = &v2alpha1.Site{

					ObjectMeta: v1.ObjectMeta{
						Name:      "the-site",
						Namespace: "test",
					},
					Status: v2alpha1.SiteStatus{
						Status: v2alpha1.Status{
							Message: "OK",
							Conditions: []v1.Condition{
								{
									Message:            "OK",
									ObservedGeneration: 1,
									Reason:             "OK",
									Status:             "True",
									Type:               "Configured",
								},
							},
						},
						Endpoints: []v2alpha1.Endpoint{
							{
								Name:  "inter-router",
								Host:  "127.0.0.1",
								Port:  "8080",
								Group: "skupper-router-1",
							},
							{
								Name:  "edge",
								Host:  "127.0.0.1",
								Port:  "8080",
								Group: "skupper-router-1",
							},
						},
					},
				}
			},
			errorMessage: "output format has not been specified",
		},
		{
			name: "runs fails because the output format is not supported",
			setUpMock: func(command *CmdLinkGenerate) {
				command.linkName = "my-link"
				command.cost = 1
				command.tlsCredentials = "secret"
				command.output = "unsupported"
				command.generateCredential = true
				command.activeSite = &v2alpha1.Site{

					ObjectMeta: v1.ObjectMeta{
						Name:      "the-site",
						Namespace: "test",
					},
					Status: v2alpha1.SiteStatus{
						Status: v2alpha1.Status{
							Message: "OK",
							Conditions: []v1.Condition{
								{
									Message:            "OK",
									ObservedGeneration: 1,
									Reason:             "OK",
									Status:             "True",
									Type:               "Configured",
								},
							},
						},
						Endpoints: []v2alpha1.Endpoint{
							{
								Name:  "inter-router",
								Host:  "127.0.0.1",
								Port:  "8080",
								Group: "skupper-router-1",
							},
							{
								Name:  "edge",
								Host:  "127.0.0.1",
								Port:  "8080",
								Group: "skupper-router-1",
							},
						},
					},
				}
			},
			errorMessage: "format unsupported not supported",
		},
		{
			name: "runs fails because active site has not endpoints configured",
			setUpMock: func(command *CmdLinkGenerate) {
				command.linkName = "my-link"
				command.cost = 1
				command.tlsCredentials = "secret"
				command.output = "yaml"
				command.generateCredential = false
				command.activeSite = &v2alpha1.Site{

					ObjectMeta: v1.ObjectMeta{
						Name:      "the-site",
						Namespace: "test",
					},
					Status: v2alpha1.SiteStatus{
						Status: v2alpha1.Status{
							Message: "OK",
							Conditions: []v1.Condition{
								{
									Message:            "OK",
									ObservedGeneration: 1,
									Reason:             "OK",
									Status:             "True",
									Type:               "Configured",
								},
							},
						},
					},
				}
			},
			errorMessage: "the active site has not configured endpoints yet",
		},
		{
			name: "runs fails because there are no active site",
			setUpMock: func(command *CmdLinkGenerate) {
				command.linkName = "my-link"
				command.cost = 1
				command.tlsCredentials = "secret"
				command.output = "yaml"
				command.generateCredential = false
				command.activeSite = nil
			},
			errorMessage: "there is no active site to generate the link resource file",
		},
		{
			name: "runs fails because certificate could not be created",
			setUpMock: func(command *CmdLinkGenerate) {
				command.linkName = "my-link"
				command.cost = 1
				command.tlsCredentials = "secret"
				command.output = "yaml"
				command.generateCredential = true
				command.activeSite = &v2alpha1.Site{

					ObjectMeta: v1.ObjectMeta{
						Name:      "the-site",
						Namespace: "test",
					},
					Status: v2alpha1.SiteStatus{
						Status: v2alpha1.Status{
							Message: "OK",
							Conditions: []v1.Condition{
								{
									Message:            "OK",
									ObservedGeneration: 1,
									Reason:             "OK",
									Status:             "True",
									Type:               "Configured",
								},
							},
						},
						Endpoints: []v2alpha1.Endpoint{
							{
								Name:  "inter-router",
								Host:  "127.0.0.1",
								Port:  "8080",
								Group: "skupper-router-1",
							},
							{
								Name:  "edge",
								Host:  "127.0.0.1",
								Port:  "8080",
								Group: "skupper-router-1",
							},
						},
					},
				}
			},
			skCliErrorMessage: "error creating certificate",
			errorMessage:      "error creating certificate",
		},
	}

	for _, test := range testTable {
		cmd, err := newCmdLinkGenerateWithMocks("test", nil, nil, test.skCliErrorMessage)
		assert.Assert(t, err)

		test.setUpMock(cmd)

		t.Run(test.name, func(t *testing.T) {

			err := cmd.Run()
			if err != nil {
				assert.Check(t, test.errorMessage == err.Error(), err.Error())
			} else {
				assert.Check(t, err == nil)
			}
		})
	}
}

func TestCmdLinkGenerate_WaitUntil(t *testing.T) {
	type test struct {
		name               string
		generateCredential bool
		generatedLink      v2alpha1.Link
		outputType         string
		tlsCredentials     string
		timeout            time.Duration
		k8sObjects         []runtime.Object
		skupperObjects     []runtime.Object
		skupperError       string
		expectError        bool
	}

	testTable := []test{
		{
			name:               "secret is not returned",
			outputType:         "yaml",
			timeout:            time.Second,
			tlsCredentials:     "linkSecret",
			generateCredential: true,
			expectError:        true,
		},
		{
			name:               "the output only contains the link",
			generateCredential: false,
			outputType:         "yaml",
			timeout:            time.Second,
			expectError:        false,
		},
		{
			name:               "bad format for the output",
			generateCredential: false,
			outputType:         "not supported",
			timeout:            time.Second,
			expectError:        true,
		},
		{
			name:               "the output contains the link and the secret",
			generateCredential: true,
			outputType:         "yaml",
			timeout:            time.Second,
			tlsCredentials:     "link-test",
			k8sObjects: []runtime.Object{
				&corev1.Secret{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "v1",
						Kind:       "Secret",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "link-test",
						Namespace: "test",
					},
					Type: "kubernetes.io/tls",
					Data: map[string][]byte{
						"tls.crt": []byte("tls cert"),
						"tls.key": []byte("tls key"),
						"ca.crt":  []byte("ca"),
					},
				},
			},
			skupperObjects: []runtime.Object{
				&v2alpha1.Certificate{
					ObjectMeta: v1.ObjectMeta{
						Name:      "link-test",
						Namespace: "test",
					},
				},
			},
			expectError: false,
		},
		{
			name:               "the output contains the link and the secret, but it failed while deleting the certificate",
			generateCredential: true,
			outputType:         "yaml",
			timeout:            time.Second,
			tlsCredentials:     "link-test",
			k8sObjects: []runtime.Object{
				&corev1.Secret{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "v1",
						Kind:       "Secret",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "link-test",
						Namespace: "test",
					},
					Type: "kubernetes.io/tls",
					Data: map[string][]byte{
						"tls.crt": []byte("tls cert"),
						"tls.key": []byte("tls key"),
						"ca.crt":  []byte("ca"),
					},
				},
			},
			expectError: true,
		},
	}

	for _, test := range testTable {
		command, err := newCmdLinkGenerateWithMocks("test", test.k8sObjects, test.skupperObjects, test.skupperError)
		assert.Assert(t, err)

		t.Run(test.name, func(t *testing.T) {

			command.output = test.outputType
			command.generateCredential = test.generateCredential
			command.tlsCredentials = test.tlsCredentials
			command.timeout = test.timeout

			err := command.WaitUntil()

			if test.expectError {
				assert.Check(t, err != nil)
			} else {
				assert.Check(t, err == nil)
			}

		})
	}
}

// --- helper methods

func newCmdLinkGenerateWithMocks(namespace string, k8sObjects []runtime.Object, skupperObjects []runtime.Object, fakeSkupperError string) (*CmdLinkGenerate, error) {

	// We make sure the interval is appropriate
	utils.SetRetryProfile(utils.TestRetryProfile)

	client, err := fakeclient.NewFakeClient(namespace, k8sObjects, skupperObjects, fakeSkupperError)
	if err != nil {
		return nil, err
	}
	CmdLinkGenerate := &CmdLinkGenerate{
		Client:     client.GetSkupperClient().SkupperV2alpha1(),
		KubeClient: client.GetKubeClient(),
		Namespace:  namespace,
	}

	return CmdLinkGenerate, nil
}

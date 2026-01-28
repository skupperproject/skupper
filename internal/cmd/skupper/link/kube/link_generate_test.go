package kube

import (
	"testing"
	"time"

	"github.com/skupperproject/skupper/internal/cmd/skupper/common"
	"github.com/skupperproject/skupper/internal/cmd/skupper/common/testutils"
	"github.com/skupperproject/skupper/internal/cmd/skupper/common/utils"
	fakeclient "github.com/skupperproject/skupper/internal/kube/client/fake"
	"github.com/skupperproject/skupper/pkg/apis/skupper/v2alpha1"
	fakeskupperv2alpha1 "github.com/skupperproject/skupper/pkg/generated/client/clientset/versioned/typed/skupper/v2alpha1/fake"
	"gotest.tools/v3/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	k8stesting "k8s.io/client-go/testing"
)

func TestCmdLinkGenerate_ValidateInput(t *testing.T) {
	type test struct {
		name           string
		args           []string
		flags          common.CommandLinkGenerateFlags
		k8sObjects     []runtime.Object
		skupperObjects []runtime.Object
		expectedError  string
		skupperError   string
	}

	testTable := []test{
		{
			name:          "missing CRD",
			args:          []string{"my-connector", "8080"},
			flags:         common.CommandLinkGenerateFlags{},
			skupperError:  utils.CrdErr,
			expectedError: utils.CrdHelpErr,
		},
		{
			name: "link yaml is not generated because there is no site in the namespace.",
			expectedError: "there is no skupper site in this namespace\n" +
				"there is no active skupper site in this namespace",
			flags: common.CommandLinkGenerateFlags{Cost: "1", TlsCredentials: "secret", Timeout: time.Minute},
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
			flags:         common.CommandLinkGenerateFlags{Cost: "1", TlsCredentials: "secret", Timeout: time.Minute},
			expectedError: "arguments are not allowed in this command",
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
			flags:         common.CommandLinkGenerateFlags{Cost: "1", Timeout: time.Minute},
			expectedError: "the TLS secret name was not specified",
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
			flags:         common.CommandLinkGenerateFlags{Cost: "one", TlsCredentials: "secret", Timeout: time.Minute},
			expectedError: "link cost is not valid: strconv.Atoi: parsing \"one\": invalid syntax",
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
			flags:         common.CommandLinkGenerateFlags{Cost: "-4", TlsCredentials: "secret", Timeout: time.Minute},
			expectedError: "link cost is not valid: value is not positive",
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
			flags:         common.CommandLinkGenerateFlags{Cost: "1", TlsCredentials: "secret", Output: "not-valid", Timeout: time.Minute},
			expectedError: "output type is not valid: value not-valid not allowed. It should be one of this options: [json yaml]",
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
			flags:         common.CommandLinkGenerateFlags{Cost: "1", TlsCredentials: "tls secret", Timeout: time.Minute},
			expectedError: "the name of the tls secret is not valid: value does not match this regular expression: ^[a-z0-9]([-a-z0-9]*[a-z0-9])*(\\.[a-z0-9]([-a-z0-9]*[a-z0-9])*)*$",
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
			flags:         common.CommandLinkGenerateFlags{Cost: "1", TlsCredentials: "secret", Timeout: time.Second * 0},
			expectedError: "timeout is not valid: duration must not be less than 10s; got 0s",
		},
	}

	for _, test := range testTable {
		t.Run(test.name, func(t *testing.T) {

			command, err := newCmdLinkGenerateWithMocks("test", nil, test.skupperObjects, test.skupperError)
			assert.Assert(t, err)
			command.Flags = &test.flags

			testutils.CheckValidateInput(t, command, test.expectedError, test.args)
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
			flags:                       common.CommandLinkGenerateFlags{TlsCredentials: "secret", Cost: "1", Output: "json", GenerateCredential: false, Timeout: time.Minute},
			expectedCost:                1,
			expectedTlsCredentials:      "secret",
			expectedOutput:              "json",
			expectedGenerateCredentials: false,
		},
		{
			name:  "credentials are not needed",
			flags: common.CommandLinkGenerateFlags{TlsCredentials: "", Cost: "1", Output: "json", GenerateCredential: true, Timeout: time.Minute},
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
		skupperObjects    []runtime.Object
		skCliErrorMessage string
	}

	testTable := []test{
		{
			name: "The function runs correctly without generating credentials",
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
			name: "The function runs correctly generating credentials",
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
			name: "The function runs correctly without generating credentials because they already exist.",
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
			skupperObjects: []runtime.Object{
				&v2alpha1.Certificate{
					TypeMeta: v1.TypeMeta{
						APIVersion: "skupper.io/v2alpha1",
						Kind:       "Certificate",
					},
					ObjectMeta: v1.ObjectMeta{
						Name:      "secret",
						Namespace: "test",
					},
				},
			},
		},
		{
			name: "The function fails because the output format is not supported",
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
			errorMessage: "Output format is not specified",
		},
		{
			name: "The function fails because the output format is not supported",
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
			name: "The function fails because active site has not endpoints configured",
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
			errorMessage: "A link cannot be generated because link access is not enabled. \n Use \"skupper site update --enable-link-access\" to enable it.",
		},
		{
			name: "The function fails because there are no active site",
			setUpMock: func(command *CmdLinkGenerate) {
				command.linkName = "my-link"
				command.cost = 1
				command.tlsCredentials = "secret"
				command.output = "yaml"
				command.generateCredential = false
				command.activeSite = nil
			},
			errorMessage: "There is no active site to generate the link resource file",
		},
		{
			name: "The function fails because certificate could not be created",
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
		cmd, err := newCmdLinkGenerateWithMocks("test", nil, test.skupperObjects, test.skCliErrorMessage)
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
		generatedLinks     []v2alpha1.Link
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
			generatedLinks: []v2alpha1.Link{
				{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "skupper.io/v2alpha1",
						Kind:       "Link",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "link",
					},
					Spec: v2alpha1.LinkSpec{
						TlsCredentials: "credentials",
						Cost:           1,
					},
				},
			},
			outputType:  "not supported",
			timeout:     time.Second,
			expectError: true,
		},
		{
			name:               "the output contains the link and the secret",
			generateCredential: true,
			outputType:         "yaml",
			timeout:            time.Second,
			tlsCredentials:     "link-test",
			k8sObjects: []runtime.Object{
				&corev1.Secret{
					TypeMeta: v1.TypeMeta{
						APIVersion: "v1",
						Kind:       "Secret",
					},
					ObjectMeta: v1.ObjectMeta{
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
					TypeMeta: v1.TypeMeta{
						APIVersion: "v1",
						Kind:       "Secret",
					},
					ObjectMeta: v1.ObjectMeta{
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
			command.generatedLinks = test.generatedLinks

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
	defaultIssuer := &v2alpha1.Certificate{
		TypeMeta: v1.TypeMeta{
			APIVersion: "skupper.io/v2alpha1",
			Kind:       "Certificate",
		},
		ObjectMeta: v1.ObjectMeta{
			Name:      "skupper-site-ca",
			Namespace: namespace,
		},
	}
	fakeSkupperCli := client.Skupper.SkupperV2alpha1().(*fakeskupperv2alpha1.FakeSkupperV2alpha1)
	fakeSkupperCli.PrependReactor("get", "certificates", func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
		getAction := action.(k8stesting.GetAction)
		if getAction.GetName() == "skupper-site-ca" {
			return true, defaultIssuer, nil
		}
		return false, nil, nil
	})
	CmdLinkGenerate := &CmdLinkGenerate{
		Client:     client.GetSkupperClient().SkupperV2alpha1(),
		KubeClient: client.GetKubeClient(),
		Namespace:  namespace,
	}

	return CmdLinkGenerate, nil
}

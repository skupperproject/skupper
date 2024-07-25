package kube

import (
	"fmt"
	"github.com/skupperproject/skupper/internal/cmd/skupper/utils"
	fakeclient "github.com/skupperproject/skupper/internal/kube/client/fake"
	"github.com/skupperproject/skupper/pkg/apis/skupper/v1alpha1"
	"github.com/spf13/pflag"
	"gotest.tools/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"os"
	"path/filepath"
	"testing"
)

func TestCmdLinkExport_NewCmdLinkExport(t *testing.T) {

	t.Run("export command", func(t *testing.T) {

		result := NewCmdLinkExport()

		assert.Check(t, result.CobraCmd.Use != "")
		assert.Check(t, result.CobraCmd.Short != "")
		assert.Check(t, result.CobraCmd.Long != "")
		assert.Check(t, result.CobraCmd.PreRun != nil)
		assert.Check(t, result.CobraCmd.Run != nil)
		assert.Check(t, result.CobraCmd.Flags() != nil)
		assert.Check(t, result.CobraCmd.PostRunE != nil)

	})

}

func TestCmdLinkExport_AddFlags(t *testing.T) {

	expectedFlagsWithDefaultValue := map[string]interface{}{
		"cost":                "1",
		"tls-secret":          "",
		"output":              "yaml",
		"generate-credential": "true",
		"timeout":             "60",
	}
	var flagList []string

	cmd, err := newCmdLinkExportWithMocks("test", nil, nil, "")
	assert.Assert(t, err)

	t.Run("add flags", func(t *testing.T) {

		cmd.CobraCmd.Flags().VisitAll(func(flag *pflag.Flag) {
			flagList = append(flagList, flag.Name)
		})

		assert.Check(t, len(flagList) == 0)

		cmd.AddFlags()

		cmd.CobraCmd.Flags().VisitAll(func(flag *pflag.Flag) {
			flagList = append(flagList, flag.Name)
			assert.Check(t, expectedFlagsWithDefaultValue[flag.Name] != nil, fmt.Sprintf("flag %q not expected", flag.Name))
			assert.Check(t, expectedFlagsWithDefaultValue[flag.Name] == flag.DefValue, fmt.Sprintf("flag %q witn not expected default value %q", flag.Name, flag.DefValue))
		})

		assert.Check(t, len(flagList) == len(expectedFlagsWithDefaultValue))

	})

}

func TestCmdLinkExport_ValidateInput(t *testing.T) {
	type test struct {
		name           string
		args           []string
		flags          ExportLinkFlags
		k8sObjects     []runtime.Object
		skupperObjects []runtime.Object
		expectedErrors []string
	}

	testTable := []test{
		{
			name:           "link yaml is not generated because there is no site in the namespace.",
			args:           []string{"my-new-link", "~/link.yaml"},
			expectedErrors: []string{"there is no skupper site in this namespace", "there is no active skupper site in this namespace"},
			flags:          ExportLinkFlags{cost: "1", tlsSecret: "secret", timeout: "60"},
		},
		{
			name: "link name is not valid.",
			args: []string{"my new site", "~/link.yaml"},
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
									Active:        true,
									StatusMessage: "OK",
								},
							},
						},
					},
				},
			},
			flags:          ExportLinkFlags{cost: "1", tlsSecret: "secret", timeout: "60"},
			expectedErrors: []string{"link name is not valid: value does not match this regular expression: ^[a-z0-9]([-a-z0-9]*[a-z0-9])*(\\.[a-z0-9]([-a-z0-9]*[a-z0-9])*)*$"},
		},
		{
			name: "link name is not specified.",
			args: []string{},
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
									Active:        true,
									StatusMessage: "OK",
								},
							},
						},
					},
				},
			},
			flags:          ExportLinkFlags{cost: "1", tlsSecret: "secret", timeout: "60"},
			expectedErrors: []string{"link name and output file must not be empty"},
		},
		{
			name: "more than two arguments were specified",
			args: []string{"my", "link", "path"},
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
									Active:        true,
									StatusMessage: "OK",
								},
							},
						},
					},
				},
			},
			flags:          ExportLinkFlags{cost: "1", tlsSecret: "secret", timeout: "60"},
			expectedErrors: []string{"only two arguments are allowed for this command."},
		},
		{
			name: "tls secret was not specified",
			args: []string{"link", "file.yaml"},
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
									Active:        true,
									StatusMessage: "OK",
								},
							},
						},
					},
				},
			},
			flags:          ExportLinkFlags{cost: "1", timeout: "60"},
			expectedErrors: []string{"the TLS secret name was not specified"},
		},
		{
			name: "cost is not valid.",
			args: []string{"link", "file.yaml"},
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
									Active:        true,
									StatusMessage: "OK",
								},
							},
						},
					},
				},
			},
			flags:          ExportLinkFlags{cost: "one", tlsSecret: "secret", timeout: "60"},
			expectedErrors: []string{"link cost is not valid: strconv.Atoi: parsing \"one\": invalid syntax"},
		},
		{
			name: "cost is not positive",
			args: []string{"link", "file.yaml"},
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
									Active:        true,
									StatusMessage: "OK",
								},
							},
						},
					},
				},
			},
			flags: ExportLinkFlags{cost: "-4", tlsSecret: "secret", timeout: "60"},
			expectedErrors: []string{
				"link cost is not valid: value is not positive",
			},
		},
		{
			name: "output format is not valid",
			args: []string{"my-site", "file.yaml"},
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
									Active:        true,
									StatusMessage: "OK",
								},
							},
						},
					},
				},
			},
			flags: ExportLinkFlags{cost: "1", tlsSecret: "secret", output: "not-valid", timeout: "60"},
			expectedErrors: []string{
				"output type is not valid: value not-valid not allowed. It should be one of this options: [json yaml]",
			},
		},
		{
			name: "tls secret name is not valid",
			args: []string{"my-site", "file.yaml"},
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
									Active:        true,
									StatusMessage: "OK",
								},
							},
						},
					},
				},
			},
			flags: ExportLinkFlags{cost: "1", tlsSecret: "tls secret", timeout: "60"},
			expectedErrors: []string{
				"the name of the tls secret is not valid: value does not match this regular expression: ^[a-z0-9]([-a-z0-9]*[a-z0-9])*(\\.[a-z0-9]([-a-z0-9]*[a-z0-9])*)*$",
			},
		},
		{
			name: "timeout is not valid",
			args: []string{"my-site", "file.yaml"},
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
									Active:        true,
									StatusMessage: "OK",
								},
							},
						},
					},
				},
			},
			flags: ExportLinkFlags{cost: "1", tlsSecret: "secret", timeout: "0"},
			expectedErrors: []string{
				"timeout is not valid: value 0 is not allowed",
			},
		},
		{
			name: "timeout is not valid because it is not a number",
			args: []string{"my-site", "file.yaml"},
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
									Active:        true,
									StatusMessage: "OK",
								},
							},
						},
					},
				},
			},
			flags: ExportLinkFlags{cost: "1", tlsSecret: "secret", timeout: "two"},
			expectedErrors: []string{
				"timeout is not valid: strconv.Atoi: parsing \"two\": invalid syntax",
			},
		},
	}

	for _, test := range testTable {
		t.Run(test.name, func(t *testing.T) {

			command, err := newCmdLinkExportWithMocks("test", nil, test.skupperObjects, "")
			assert.Assert(t, err)
			command.flags = test.flags

			actualErrors := command.ValidateInput(test.args)

			actualErrorsMessages := utils.ErrorsToMessages(actualErrors)

			assert.DeepEqual(t, actualErrorsMessages, test.expectedErrors)

		})
	}
}

func TestCmdLinkExport_InputToOptions(t *testing.T) {

	type test struct {
		name                        string
		args                        []string
		flags                       ExportLinkFlags
		expectedTlsSecret           string
		expectedCost                int
		expectedOutput              string
		expectedGenerateCredentials bool
	}

	testTable := []test{
		{
			name:                        "check options",
			args:                        []string{"my-link"},
			flags:                       ExportLinkFlags{"secret", "1", "json", false, "60"},
			expectedCost:                1,
			expectedTlsSecret:           "secret",
			expectedOutput:              "json",
			expectedGenerateCredentials: false,
		},
	}

	for _, test := range testTable {
		t.Run(test.name, func(t *testing.T) {

			cmd, err := newCmdLinkExportWithMocks("test", nil, nil, "")
			assert.Assert(t, err)

			cmd.flags = test.flags

			cmd.InputToOptions()

			assert.Check(t, cmd.output == test.expectedOutput)
			assert.Check(t, cmd.tlsSecret == test.expectedTlsSecret)
			assert.Check(t, cmd.cost == test.expectedCost)
			assert.Check(t, cmd.generateCredential == test.expectedGenerateCredentials)
		})
	}
}

func TestCmdLinkExport_Run(t *testing.T) {
	type test struct {
		name              string
		setUpMock         func(command *CmdLinkExport)
		errorMessage      string
		skCliErrorMessage string
	}

	testTable := []test{
		{
			name: "runs ok without generating credentials",
			setUpMock: func(command *CmdLinkExport) {
				command.linkName = "my-link"
				command.cost = 1
				command.tlsSecret = "secret"
				command.output = "yaml"
				command.outputFile = "created-link.yaml"
				command.generateCredential = true
				command.activeSite = &v1alpha1.Site{

					ObjectMeta: v1.ObjectMeta{
						Name:      "the-site",
						Namespace: "test",
					},
					Status: v1alpha1.SiteStatus{
						Status: v1alpha1.Status{
							Active:        true,
							StatusMessage: "OK",
						},
						Endpoints: []v1alpha1.Endpoint{
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
			setUpMock: func(command *CmdLinkExport) {
				command.linkName = "my-link"
				command.cost = 1
				command.tlsSecret = "secret"
				command.output = "yaml"
				command.outputFile = "created-link.yaml"
				command.generateCredential = false
				command.activeSite = &v1alpha1.Site{

					ObjectMeta: v1.ObjectMeta{
						Name:      "the-site",
						Namespace: "test",
					},
					Status: v1alpha1.SiteStatus{
						Status: v1alpha1.Status{
							Active:        true,
							StatusMessage: "OK",
						},
						Endpoints: []v1alpha1.Endpoint{
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
			setUpMock: func(command *CmdLinkExport) {
				command.linkName = "my-link"
				command.cost = 1
				command.tlsSecret = "secret"
				command.output = ""
				command.generateCredential = true
				command.activeSite = &v1alpha1.Site{

					ObjectMeta: v1.ObjectMeta{
						Name:      "the-site",
						Namespace: "test",
					},
					Status: v1alpha1.SiteStatus{
						Status: v1alpha1.Status{
							Active:        true,
							StatusMessage: "OK",
						},
						Endpoints: []v1alpha1.Endpoint{
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
			setUpMock: func(command *CmdLinkExport) {
				command.linkName = "my-link"
				command.cost = 1
				command.tlsSecret = "secret"
				command.output = "unsupported"
				command.generateCredential = true
				command.activeSite = &v1alpha1.Site{

					ObjectMeta: v1.ObjectMeta{
						Name:      "the-site",
						Namespace: "test",
					},
					Status: v1alpha1.SiteStatus{
						Status: v1alpha1.Status{
							Active:        true,
							StatusMessage: "OK",
						},
						Endpoints: []v1alpha1.Endpoint{
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
			setUpMock: func(command *CmdLinkExport) {
				command.linkName = "my-link"
				command.cost = 1
				command.tlsSecret = "secret"
				command.output = "yaml"
				command.outputFile = "created-link.yaml"
				command.generateCredential = false
				command.activeSite = &v1alpha1.Site{

					ObjectMeta: v1.ObjectMeta{
						Name:      "the-site",
						Namespace: "test",
					},
					Status: v1alpha1.SiteStatus{
						Status: v1alpha1.Status{
							Active:        true,
							StatusMessage: "OK",
						},
					},
				}
			},
			errorMessage: "the active site has not configured endpoints yet",
		},
		{
			name: "runs fails because there are no active site",
			setUpMock: func(command *CmdLinkExport) {
				command.linkName = "my-link"
				command.cost = 1
				command.tlsSecret = "secret"
				command.output = "yaml"
				command.outputFile = "created-link.yaml"
				command.generateCredential = false
				command.activeSite = nil
			},
			errorMessage: "there is no active site to generate the link resource file",
		},
		{
			name: "runs fails because certificate could not be created",
			setUpMock: func(command *CmdLinkExport) {
				command.linkName = "my-link"
				command.cost = 1
				command.tlsSecret = "secret"
				command.output = "yaml"
				command.outputFile = "created-link.yaml"
				command.generateCredential = true
				command.activeSite = &v1alpha1.Site{

					ObjectMeta: v1.ObjectMeta{
						Name:      "the-site",
						Namespace: "test",
					},
					Status: v1alpha1.SiteStatus{
						Status: v1alpha1.Status{
							Active:        true,
							StatusMessage: "OK",
						},
						Endpoints: []v1alpha1.Endpoint{
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
		cmd, err := newCmdLinkExportWithMocks("test", nil, nil, test.skCliErrorMessage)
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

func TestCmdLinkExport_WaitUntilReady(t *testing.T) {
	type test struct {
		name               string
		generateCredential bool
		generatedLink      v1alpha1.Link
		outputFile         string
		outputType         string
		tlsSecret          string
		timeout            int
		k8sObjects         []runtime.Object
		skupperObjects     []runtime.Object
		skupperError       string
		expectError        bool
	}

	testTable := []test{
		{
			name:               "secret is not returned",
			outputFile:         "link.yaml",
			outputType:         "yaml",
			timeout:            3,
			tlsSecret:          "linkSecret",
			generateCredential: true,
			expectError:        true,
		},
		{
			name:               "the file only contains the link",
			generateCredential: false,
			outputFile:         "link.yaml",
			outputType:         "yaml",
			timeout:            3,
			expectError:        false,
		},
		{
			name:               "bad format for the file",
			generateCredential: false,
			outputFile:         "link.yaml",
			outputType:         "not supported",
			timeout:            3,
			expectError:        true,
		},
		{
			name:               "the file contains the link and the secret",
			generateCredential: true,
			outputFile:         "link.yaml",
			outputType:         "yaml",
			timeout:            3,
			tlsSecret:          "linkSecret",
			k8sObjects: []runtime.Object{
				&corev1.Secret{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "v1",
						Kind:       "Secret",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "linkSecret",
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
			expectError: false,
		},
	}

	for _, test := range testTable {
		command, err := newCmdLinkExportWithMocks("test", test.k8sObjects, test.skupperObjects, test.skupperError)
		assert.Assert(t, err)

		t.Run(test.name, func(t *testing.T) {

			tempDir := os.TempDir()
			testFilePath := filepath.Join(tempDir, command.outputFile)
			command.outputFile = testFilePath
			command.output = test.outputType
			command.generateCredential = test.generateCredential
			command.tlsSecret = test.tlsSecret
			command.timeout = test.timeout

			err := command.WaitUntilReady()

			if test.expectError {
				assert.Check(t, err != nil)
			} else {
				assert.Check(t, err == nil)
				_, err := os.Stat(command.outputFile)
				assert.Check(t, err == nil, err)
			}

		})
	}
}

// --- helper methods

func newCmdLinkExportWithMocks(namespace string, k8sObjects []runtime.Object, skupperObjects []runtime.Object, fakeSkupperError string) (*CmdLinkExport, error) {

	client, err := fakeclient.NewFakeClient(namespace, k8sObjects, skupperObjects, fakeSkupperError)
	if err != nil {
		return nil, err
	}
	cmdLinkExport := &CmdLinkExport{
		Client:     client.GetSkupperClient().SkupperV1alpha1(),
		KubeClient: client.GetKubeClient(),
		Namespace:  namespace,
	}

	return cmdLinkExport, nil
}

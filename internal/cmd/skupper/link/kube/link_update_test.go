package kube

import (
	"fmt"
	"testing"

	"github.com/skupperproject/skupper/internal/cmd/skupper/utils"
	fakeclient "github.com/skupperproject/skupper/internal/kube/client/fake"
	"github.com/skupperproject/skupper/pkg/apis/skupper/v1alpha1"
	"github.com/spf13/pflag"
	"gotest.tools/assert"
	v12 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestCmdLinkUpdate_NewCmdLinkUpdate(t *testing.T) {

	t.Run("update command", func(t *testing.T) {

		result := NewCmdLinkUpdate()

		assert.Check(t, result.CobraCmd.Use != "")
		assert.Check(t, result.CobraCmd.Short != "")
		assert.Check(t, result.CobraCmd.Long != "")
		assert.Check(t, result.CobraCmd.PreRun != nil)
		assert.Check(t, result.CobraCmd.Run != nil)
		assert.Check(t, result.CobraCmd.PostRunE != nil)
		assert.Check(t, result.CobraCmd.Flags() != nil)

	})

}

func TestCmdLinkUpdate_AddFlags(t *testing.T) {

	expectedFlagsWithDefaultValue := map[string]interface{}{
		"cost":       "1",
		"tls-secret": "",
		"output":     "",
		"timeout":    "60",
	}
	var flagList []string

	cmd, err := newCmdLinkUpdateWithMocks("test", nil, nil, "")
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
			assert.Check(t, expectedFlagsWithDefaultValue[flag.Name] == flag.DefValue)
		})

		assert.Check(t, len(flagList) == len(expectedFlagsWithDefaultValue))

	})

}

func TestCmdLinkUpdate_ValidateInput(t *testing.T) {
	type test struct {
		name           string
		args           []string
		flags          UpdateLinkFlags
		k8sObjects     []runtime.Object
		skupperObjects []runtime.Object
		expectedErrors []string
	}

	testTable := []test{
		{
			name:  "link is not updated because there is no site in the namespace.",
			args:  []string{"my-link"},
			flags: UpdateLinkFlags{cost: "1", timeout: "60"},
			skupperObjects: []runtime.Object{
				&v1alpha1.Link{
					ObjectMeta: v1.ObjectMeta{
						Name:      "my-link",
						Namespace: "test",
					},
				},
			},
			expectedErrors: []string{"there is no skupper site in this namespace"},
		},
		{
			name:  "link is not available",
			args:  []string{"my-link"},
			flags: UpdateLinkFlags{cost: "1", timeout: "60"},
			skupperObjects: []runtime.Object{
				&v1alpha1.SiteList{
					Items: []v1alpha1.Site{
						{
							ObjectMeta: v1.ObjectMeta{
								Name:      "the-site",
								Namespace: "test",
							},
						},
					},
				},
			},
			expectedErrors: []string{"the link \"my-link\" is not available in the namespace: links.skupper.io \"my-link\" not found"},
		},
		{
			name:  "selected link does not exist",
			args:  []string{"my"},
			flags: UpdateLinkFlags{cost: "1", timeout: "60"},
			skupperObjects: []runtime.Object{
				&v1alpha1.SiteList{
					Items: []v1alpha1.Site{
						{
							ObjectMeta: v1.ObjectMeta{
								Name:      "the-site",
								Namespace: "test",
							},
						},
					},
				},
				&v1alpha1.Link{
					ObjectMeta: v1.ObjectMeta{
						Name:      "my-link",
						Namespace: "test",
					},
				},
			},
			expectedErrors: []string{"the link \"my\" is not available in the namespace: links.skupper.io \"my\" not found"},
		},
		{
			name:  "link name is not specified.",
			args:  []string{},
			flags: UpdateLinkFlags{cost: "1", timeout: "60"},
			skupperObjects: []runtime.Object{
				&v1alpha1.SiteList{
					Items: []v1alpha1.Site{
						{
							ObjectMeta: v1.ObjectMeta{
								Name:      "the-site",
								Namespace: "test",
							},
						},
					},
				},
				&v1alpha1.Link{
					ObjectMeta: v1.ObjectMeta{
						Name:      "my-link",
						Namespace: "test",
					},
				},
			},
			expectedErrors: []string{"link name must not be empty"},
		},
		{
			name:  "more than one argument was specified",
			args:  []string{"my", "link"},
			flags: UpdateLinkFlags{cost: "1", timeout: "60"},
			skupperObjects: []runtime.Object{
				&v1alpha1.SiteList{
					Items: []v1alpha1.Site{
						{
							ObjectMeta: v1.ObjectMeta{
								Name:      "the-site",
								Namespace: "test",
							},
						},
					},
				},
				&v1alpha1.Link{
					ObjectMeta: v1.ObjectMeta{
						Name:      "my-link",
						Namespace: "test",
					},
				},
			},
			expectedErrors: []string{"only one argument is allowed for this command"},
		},
		{
			name:  "cost is not valid.",
			args:  []string{"my-link"},
			flags: UpdateLinkFlags{cost: "one", timeout: "60"},
			skupperObjects: []runtime.Object{
				&v1alpha1.SiteList{
					Items: []v1alpha1.Site{
						{
							ObjectMeta: v1.ObjectMeta{
								Name:      "the-site",
								Namespace: "test",
							},
						},
					},
				},
				&v1alpha1.Link{
					ObjectMeta: v1.ObjectMeta{
						Name:      "my-link",
						Namespace: "test",
					},
				},
			},
			expectedErrors: []string{"link cost is not valid: strconv.Atoi: parsing \"one\": invalid syntax"},
		},
		{
			name:  "cost is not positive",
			args:  []string{"my-link"},
			flags: UpdateLinkFlags{cost: "-4", timeout: "60"},
			skupperObjects: []runtime.Object{
				&v1alpha1.SiteList{
					Items: []v1alpha1.Site{
						{
							ObjectMeta: v1.ObjectMeta{
								Name:      "the-site",
								Namespace: "test",
							},
						},
					},
				},
				&v1alpha1.Link{
					ObjectMeta: v1.ObjectMeta{
						Name:      "my-link",
						Namespace: "test",
					},
				},
			},
			expectedErrors: []string{
				"link cost is not valid: value is not positive",
			},
		},
		{
			name:  "output format is not valid",
			args:  []string{"my-link"},
			flags: UpdateLinkFlags{cost: "1", output: "not-valid", timeout: "60"},
			skupperObjects: []runtime.Object{
				&v1alpha1.SiteList{
					Items: []v1alpha1.Site{
						{
							ObjectMeta: v1.ObjectMeta{
								Name:      "the-site",
								Namespace: "test",
							},
						},
					},
				},
				&v1alpha1.Link{
					ObjectMeta: v1.ObjectMeta{
						Name:      "my-link",
						Namespace: "test",
					},
				},
			},
			expectedErrors: []string{
				"output type is not valid: value not-valid not allowed. It should be one of this options: [json yaml]",
			},
		},
		{
			name:  "tls secret not available",
			args:  []string{"my-link"},
			flags: UpdateLinkFlags{cost: "1", tlsSecret: "secret", timeout: "60"},
			skupperObjects: []runtime.Object{
				&v1alpha1.SiteList{
					Items: []v1alpha1.Site{
						{
							ObjectMeta: v1.ObjectMeta{
								Name:      "the-site",
								Namespace: "test",
							},
						},
					},
				},
				&v1alpha1.Link{
					ObjectMeta: v1.ObjectMeta{
						Name:      "my-link",
						Namespace: "test",
					},
				},
			},
			expectedErrors: []string{
				"the TLS secret \"secret\" is not available in the namespace: secrets \"secret\" not found",
			},
		},
		{
			name:  "timeout value is 0",
			args:  []string{"my-link"},
			flags: UpdateLinkFlags{cost: "1", tlsSecret: "secret", timeout: "0"},
			k8sObjects: []runtime.Object{
				&v12.Secret{
					ObjectMeta: v1.ObjectMeta{
						Name:      "secret",
						Namespace: "test",
					},
				},
			},
			skupperObjects: []runtime.Object{
				&v1alpha1.SiteList{
					Items: []v1alpha1.Site{
						{
							ObjectMeta: v1.ObjectMeta{
								Name:      "the-site",
								Namespace: "test",
							},
						},
					},
				},
				&v1alpha1.Link{
					ObjectMeta: v1.ObjectMeta{
						Name:      "my-link",
						Namespace: "test",
					},
				},
			},
			expectedErrors: []string{
				"timeout is not valid: value 0 is not allowed",
			},
		},
		{
			name:  "timeout value is negative",
			args:  []string{"my-link"},
			flags: UpdateLinkFlags{cost: "1", tlsSecret: "secret", timeout: "-4"},
			k8sObjects: []runtime.Object{
				&v12.Secret{
					ObjectMeta: v1.ObjectMeta{
						Name:      "secret",
						Namespace: "test",
					},
				},
			},
			skupperObjects: []runtime.Object{
				&v1alpha1.SiteList{
					Items: []v1alpha1.Site{
						{
							ObjectMeta: v1.ObjectMeta{
								Name:      "the-site",
								Namespace: "test",
							},
						},
					},
				},
				&v1alpha1.Link{
					ObjectMeta: v1.ObjectMeta{
						Name:      "my-link",
						Namespace: "test",
					},
				},
			},
			expectedErrors: []string{
				"timeout is not valid: value is not positive",
			},
		},
		{
			name:  "timeout value is not a number",
			args:  []string{"my-link"},
			flags: UpdateLinkFlags{cost: "1", tlsSecret: "secret", timeout: "four"},
			k8sObjects: []runtime.Object{
				&v12.Secret{
					ObjectMeta: v1.ObjectMeta{
						Name:      "secret",
						Namespace: "test",
					},
				},
			},
			skupperObjects: []runtime.Object{
				&v1alpha1.SiteList{
					Items: []v1alpha1.Site{
						{
							ObjectMeta: v1.ObjectMeta{
								Name:      "the-site",
								Namespace: "test",
							},
						},
					},
				},
				&v1alpha1.Link{
					ObjectMeta: v1.ObjectMeta{
						Name:      "my-link",
						Namespace: "test",
					},
				},
			},
			expectedErrors: []string{
				"timeout is not valid: strconv.Atoi: parsing \"four\": invalid syntax",
			},
		},
	}

	for _, test := range testTable {
		t.Run(test.name, func(t *testing.T) {

			command, err := newCmdLinkUpdateWithMocks("test", test.k8sObjects, test.skupperObjects, "")
			assert.Assert(t, err)

			command.flags = test.flags

			actualErrors := command.ValidateInput(test.args)

			actualErrorsMessages := utils.ErrorsToMessages(actualErrors)

			assert.DeepEqual(t, actualErrorsMessages, test.expectedErrors)

		})
	}
}

func TestCmdLinkUpdate_InputToOptions(t *testing.T) {

	type test struct {
		name              string
		args              []string
		flags             UpdateLinkFlags
		expectedTlsSecret string
		expectedCost      int
		expectedOutput    string
		expectedTimeout   int
	}

	testTable := []test{
		{
			name:              "check options",
			args:              []string{"my-link"},
			flags:             UpdateLinkFlags{"secret", "1", "json", "60"},
			expectedCost:      1,
			expectedTlsSecret: "secret",
			expectedOutput:    "json",
			expectedTimeout:   60,
		},
	}

	for _, test := range testTable {
		t.Run(test.name, func(t *testing.T) {

			cmd, err := newCmdLinkUpdateWithMocks("test", nil, nil, "")
			assert.Assert(t, err)
			cmd.flags = test.flags

			cmd.InputToOptions()

			assert.Check(t, cmd.output == test.expectedOutput)
			assert.Check(t, cmd.tlsSecret == test.expectedTlsSecret)
			assert.Check(t, cmd.cost == test.expectedCost)
			assert.Check(t, cmd.timeout == test.expectedTimeout)
		})
	}
}

func TestCmdLinkUpdate_Run(t *testing.T) {
	type test struct {
		name                string
		k8sObjects          []runtime.Object
		skupperObjects      []runtime.Object
		linkName            string
		cost                int
		output              string
		tlsSecret           string
		errorMessage        string
		skupperErrorMessage string
	}

	testTable := []test{
		{
			name:      "runs ok",
			linkName:  "my-link",
			cost:      1,
			tlsSecret: "secret",
			skupperObjects: []runtime.Object{
				&v1alpha1.Link{
					ObjectMeta: v1.ObjectMeta{
						Name:      "my-link",
						Namespace: "test",
					},
				},
			},
		},
		{
			name:                "run fails",
			linkName:            "my-link",
			skupperErrorMessage: "error",
			errorMessage:        "error",
		},
		{
			name:         "run fails because link does not exist",
			linkName:     "my-link",
			errorMessage: "links.skupper.io \"my-link\" not found",
		},
		{
			name:     "runs ok without updating link",
			linkName: "my-link",
			output:   "yaml",
			skupperObjects: []runtime.Object{
				&v1alpha1.Link{
					ObjectMeta: v1.ObjectMeta{
						Name:      "my-link",
						Namespace: "test",
					},
				},
			},
		},
		{
			name:         "runs fails because the output format is not supported",
			linkName:     "my-link",
			output:       "unsupported",
			errorMessage: "format unsupported not supported",
			skupperObjects: []runtime.Object{
				&v1alpha1.Link{
					ObjectMeta: v1.ObjectMeta{
						Name:      "my-link",
						Namespace: "test",
					},
				},
			},
		},
	}

	for _, test := range testTable {
		cmd, err := newCmdLinkUpdateWithMocks("test", test.k8sObjects, test.skupperObjects, test.skupperErrorMessage)
		assert.Assert(t, err)

		cmd.linkName = test.linkName
		cmd.output = test.output
		cmd.tlsSecret = test.tlsSecret
		cmd.cost = test.cost

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

func TestCmdLinkUpdate_WaitUntil(t *testing.T) {
	type test struct {
		name                string
		k8sObjects          []runtime.Object
		skupperObjects      []runtime.Object
		skupperErrorMessage string
		linkName            string
		output              string
		timeout             int
		expectError         bool
	}

	testTable := []test{
		{
			name: "link is not configured",
			skupperObjects: []runtime.Object{
				&v1alpha1.Link{
					ObjectMeta: v1.ObjectMeta{
						Name:      "my-link",
						Namespace: "test",
					},
					Status: v1alpha1.LinkStatus{},
				},
			},
			linkName:    "my-link",
			timeout:     3,
			expectError: true,
		},
		{
			name:        "link is not returned",
			linkName:    "my-link",
			timeout:     3,
			expectError: true,
		},
		{
			name:        "there is no need to wait for a link, the user just wanted the output",
			linkName:    "my-link",
			output:      "json",
			timeout:     3,
			expectError: false,
		},
		{
			name: "link is configured",
			skupperObjects: []runtime.Object{
				&v1alpha1.Link{
					ObjectMeta: v1.ObjectMeta{
						Name:      "my-link",
						Namespace: "test",
					},
					Status: v1alpha1.LinkStatus{
						Status: v1alpha1.Status{
							StatusMessage: "OK",
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
			},
			linkName:    "my-link",
			timeout:     3,
			expectError: false,
		},
	}

	for _, test := range testTable {
		test := test
		cmd, err := newCmdLinkUpdateWithMocks("test", test.k8sObjects, test.skupperObjects, test.skupperErrorMessage)
		assert.Assert(t, err)
		cmd.linkName = test.linkName
		cmd.output = test.output
		cmd.timeout = test.timeout

		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
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

func newCmdLinkUpdateWithMocks(namespace string, k8sObjects []runtime.Object, skupperObjects []runtime.Object, fakeSkupperError string) (*CmdLinkUpdate, error) {

	client, err := fakeclient.NewFakeClient(namespace, k8sObjects, skupperObjects, fakeSkupperError)
	if err != nil {
		return nil, err
	}
	cmdLinkUpdate := &CmdLinkUpdate{
		Client:     client.GetSkupperClient().SkupperV1alpha1(),
		KubeClient: client.GetKubeClient(),
		Namespace:  namespace,
	}

	return cmdLinkUpdate, nil
}

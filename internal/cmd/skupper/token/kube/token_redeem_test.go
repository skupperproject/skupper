package kube

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/skupperproject/skupper/internal/cmd/skupper/utils"
	fakeclient "github.com/skupperproject/skupper/internal/kube/client/fake"
	"github.com/skupperproject/skupper/pkg/apis/skupper/v1alpha1"
	"github.com/skupperproject/skupper/pkg/generated/client/clientset/versioned/scheme"
	"github.com/spf13/pflag"
	"gotest.tools/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
)

func TestCmdTokenRedeem_NewCmdTokenRedeem(t *testing.T) {

	t.Run("connector command", func(t *testing.T) {

		result := NewCmdTokenRedeem()

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

func TestCmdTokenRedeem_AddFlags(t *testing.T) {

	expectedFlagsWithDefaultValue := map[string]interface{}{
		"timeout": "1m0s",
	}
	var flagList []string

	cmd, err := newCmdTokenRedeemWithMocks("test", nil, nil, "")
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

func TestCmdTokenRedeem_ValidateInput(t *testing.T) {
	type test struct {
		name           string
		args           []string
		flags          TokenRedeem
		k8sObjects     []runtime.Object
		skupperObjects []runtime.Object
		expectedErrors []string
	}

	// create temp token file for tests
	_, err := os.Create("/tmp/token-redeem.yaml")
	assert.Check(t, err == nil)

	defer os.Remove("/tmp/token-redeem.yaml") // clean up

	testTable := []test{
		{
			name:           "token no site",
			args:           []string{"/tmp/token-redeem.yaml"},
			flags:          TokenRedeem{timeout: 60 * time.Second},
			expectedErrors: []string{"A site must exist in namespace test before a token can be redeemed"},
		},
		{
			name:  "token not site ok",
			args:  []string{"/tmp/token-redeem.yaml"},
			flags: TokenRedeem{timeout: 60 * time.Second},
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
			name:  "file name is not specified",
			args:  []string{},
			flags: TokenRedeem{timeout: 60 * time.Second},
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
			expectedErrors: []string{"token file name must be configured"},
		},
		{
			name:  "file name empty",
			args:  []string{""},
			flags: TokenRedeem{timeout: 60 * time.Second},
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
			expectedErrors: []string{"file name must not be empty"},
		},
		{
			name:  "more than one argument is specified",
			args:  []string{"my-grant", "/home/user/my-grant.yaml"},
			flags: TokenRedeem{timeout: 60 * time.Second},
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
			expectedErrors: []string{"only one argument is allowed for this command"},
		},
		{
			name:  "token file name is not valid.",
			args:  []string{"my new file"},
			flags: TokenRedeem{timeout: 60 * time.Second},
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
			expectedErrors: []string{"token file name is not valid: value does not match this regular expression: ^[A-Za-z0-9./~-]+$"},
		},
		{
			name:  "timeout is not valid",
			args:  []string{"~/token.yaml"},
			flags: TokenRedeem{timeout: 0 * time.Second},
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
				"token file does not exist: stat ~/token.yaml: no such file or directory",
				"timeout is not valid"},
		},
		{
			name:  "flags all valid",
			args:  []string{"/tmp/token-redeem.yaml"},
			flags: TokenRedeem{timeout: 50 * time.Second},
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
			expectedErrors: []string{},
		},
	}

	for _, test := range testTable {
		t.Run(test.name, func(t *testing.T) {

			command, err := newCmdTokenRedeemWithMocks("test", test.k8sObjects, test.skupperObjects, "")
			assert.Assert(t, err)

			command.flags = test.flags

			actualErrors := command.ValidateInput(test.args)

			actualErrorsMessages := utils.ErrorsToMessages(actualErrors)

			assert.DeepEqual(t, actualErrorsMessages, test.expectedErrors)

		})
	}
}

func TestCmdTokenRedeem_Run(t *testing.T) {
	type test struct {
		name                string
		k8sObjects          []runtime.Object
		skupperObjects      []runtime.Object
		errorMessage        string
		skupperErrorMessage string
	}

	defer os.Remove("/tmp/tokenR.yaml") // clean up

	err := newCmdCreateAccessGrantFile("/tmp/tokenR.yaml")
	assert.Check(t, err == nil)

	testTable := []test{
		{
			name: "runs ok",
			skupperObjects: []runtime.Object{
				&v1alpha1.AccessToken{
					ObjectMeta: v1.ObjectMeta{
						Name:      "my-token",
						Namespace: "test",
					},
					Status: v1alpha1.Status{
						Conditions: []v1.Condition{
							{
								Type:   "Redeemed",
								Status: "True",
							},
						},
					},
				},
			},
			skupperErrorMessage: "",
			errorMessage:        "",
		},
		{
			name:                "run fails",
			skupperErrorMessage: "error",
			errorMessage:        "error",
		},
	}

	for _, test := range testTable {
		cmd, err := newCmdTokenRedeemWithMocks("test", test.k8sObjects, test.skupperObjects, test.skupperErrorMessage)
		assert.Assert(t, err)

		cmd.fileName = "/tmp/tokenR.yaml"

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

func TestCmdTokenRedeem_WaitUntilReady(t *testing.T) {
	type test struct {
		name                string
		k8sObjects          []runtime.Object
		skupperObjects      []runtime.Object
		skupperErrorMessage string
		expectError         bool
	}

	testTable := []test{
		{
			name: "token cannot be redeemed",
			skupperObjects: []runtime.Object{
				&v1alpha1.AccessToken{
					ObjectMeta: v1.ObjectMeta{
						Name:      "my-grant",
						Namespace: "test",
					},
					Status: v1alpha1.Status{},
				},
			},
			expectError: true,
		},
		{
			name:                "failure redeeming token",
			skupperErrorMessage: "it failed",
			expectError:         true,
		},
		{
			name: "token is redeemed",
			skupperObjects: []runtime.Object{
				&v1alpha1.AccessToken{
					ObjectMeta: v1.ObjectMeta{
						Name:      "my-grant",
						Namespace: "test",
					},
					Status: v1alpha1.Status{
						Conditions: []v1.Condition{
							{
								Type:   "Redeemed",
								Status: "True",
							},
						},
					},
				},
			},
			expectError: false,
		},
	}

	for _, test := range testTable {
		cmd, err := newCmdTokenRedeemWithMocks("test", test.k8sObjects, test.skupperObjects, test.skupperErrorMessage)
		assert.Assert(t, err)

		cmd.name = "my-grant"
		cmd.fileName = "/tmp/token.yaml"
		cmd.flags = TokenRedeem{timeout: 1 * time.Second}
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

func newCmdCreateAccessGrantFile(fileName string) error {

	resource := v1alpha1.AccessGrant{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "skupper.io/v1alpha1",
			Kind:       "AccessGrant",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "grant",
		},
		Status: v1alpha1.AccessGrantStatus{
			Url:  "AAA",
			Ca:   "BBB",
			Code: "CCC",
		},
	}

	s := json.NewYAMLSerializer(json.DefaultMetaFactory, scheme.Scheme, scheme.Scheme)
	out, err := os.Create(fileName)
	if err != nil {
		return fmt.Errorf("Could not write to file " + fileName + ": " + err.Error())
	}
	err = s.Encode(&resource, out)
	if err != nil {
		return fmt.Errorf("Could not write out generated token: " + err.Error())
	}

	return nil

}

func newCmdTokenRedeemWithMocks(namespace string, k8sObjects []runtime.Object, skupperObjects []runtime.Object, fakeSkupperError string) (*CmdTokenRedeem, error) {

	client, err := fakeclient.NewFakeClient(namespace, k8sObjects, skupperObjects, fakeSkupperError)
	if err != nil {
		return nil, err
	}
	cmdTokenRedeem := &CmdTokenRedeem{
		client:    client.GetSkupperClient().SkupperV1alpha1(),
		namespace: namespace,
	}

	return cmdTokenRedeem, nil
}

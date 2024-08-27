package kube

import (
	"os"
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

func TestCmdTokenIssue_NewCmdTokenIssue(t *testing.T) {

	t.Run("connector command", func(t *testing.T) {

		result := NewCmdTokenIssue()

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

func TestCmdTokenIssue_AddFlags(t *testing.T) {

	expectedFlagsWithDefaultValue := map[string]interface{}{
		"redemptions-allowed": "1",
		"expiration-window":   "15m0s",
		"timeout":             "1m0s",
	}
	var flagList []string

	cmd, err := newCmdTokenIssueWithMocks("test", nil, nil, "")
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

func TestCmdTokenIssue_ValidateInput(t *testing.T) {
	type test struct {
		name           string
		args           []string
		flags          TokenIssue
		k8sObjects     []runtime.Object
		skupperObjects []runtime.Object
		expectedErrors []string
	}

	testTable := []test{
		{
			name: "token is not issued because there is already the same token in the namespace",
			args: []string{"my-token", "~/token.yaml"},
			flags: TokenIssue{
				expiration:  15 * time.Minute,
				redemptions: 1,
				timeout:     60 * time.Second,
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
				&v1alpha1.AccessGrant{
					ObjectMeta: v1.ObjectMeta{
						Name:      "my-token",
						Namespace: "test",
					},
					Spec: v1alpha1.AccessGrantSpec{
						RedemptionsAllowed: 1,
						ExpirationWindow:   "15m0s",
					},
					Status: v1alpha1.AccessGrantStatus{
						Status: v1alpha1.Status{
							Conditions: []v1.Condition{
								{
									Type:   "Ready",
									Status: "True",
								},
							},
						},
					},
				},
			},
			expectedErrors: []string{"there is already a token my-token created in namespace test"},
		},
		{
			name: "token no site",
			args: []string{"token", "filename"},
			flags: TokenIssue{
				expiration:  15 * time.Minute,
				redemptions: 1,
				timeout:     60 * time.Second,
			},
			skupperObjects: []runtime.Object{
				&v1alpha1.AccessGrant{
					ObjectMeta: v1.ObjectMeta{
						Name:      "my-token",
						Namespace: "test",
					},
					Status: v1alpha1.AccessGrantStatus{
						Status: v1alpha1.Status{
							Conditions: []v1.Condition{
								{
									Type:   "Ready",
									Status: "True",
								},
							},
						},
					},
				},
			},
			expectedErrors: []string{"A site must exist in namespace test before a token can be created"},
		},
		{
			name: "token no site with OK status",
			args: []string{"token", "filename"},
			flags: TokenIssue{
				expiration:  15 * time.Minute,
				redemptions: 1,
				timeout:     60 * time.Second,
			},
			skupperObjects: []runtime.Object{
				&v1alpha1.SiteList{
					Items: []v1alpha1.Site{
						{
							ObjectMeta: v1.ObjectMeta{
								Name:      "site1",
								Namespace: "test",
							},
							Status: v1alpha1.SiteStatus{
								Status: v1alpha1.Status{},
							},
						},
					},
				},
				&v1alpha1.AccessGrant{
					ObjectMeta: v1.ObjectMeta{
						Name:      "my-token",
						Namespace: "test",
					},
				},
			},
			expectedErrors: []string{"there is no active skupper site in this namespace"},
		},
		{
			name: "token name and file name are not specified",
			args: []string{},
			flags: TokenIssue{
				expiration:  15 * time.Minute,
				redemptions: 1,
				timeout:     60 * time.Second,
			},
			skupperObjects: []runtime.Object{
				&v1alpha1.SiteList{
					Items: []v1alpha1.Site{
						{
							ObjectMeta: v1.ObjectMeta{
								Name:      "site1",
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
			expectedErrors: []string{"token name and file name must be configured"},
		},
		{
			name: "token name empty",
			args: []string{"", "~/token.yaml"},
			flags: TokenIssue{
				expiration:  15 * time.Minute,
				redemptions: 1,
				timeout:     60 * time.Second,
			},
			skupperObjects: []runtime.Object{
				&v1alpha1.SiteList{
					Items: []v1alpha1.Site{
						{
							ObjectMeta: v1.ObjectMeta{
								Name:      "site1",
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
			expectedErrors: []string{"token name must not be empty"},
		},
		{
			name: "token file name empty",
			args: []string{"my-grant-file-empty", ""},
			flags: TokenIssue{
				expiration:  15 * time.Minute,
				redemptions: 1,
				timeout:     60 * time.Second,
			},
			skupperObjects: []runtime.Object{
				&v1alpha1.SiteList{
					Items: []v1alpha1.Site{
						{
							ObjectMeta: v1.ObjectMeta{
								Name:      "site1",
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
			name: "token filename is not specified",
			args: []string{"my-name"},
			flags: TokenIssue{
				expiration:  15 * time.Minute,
				redemptions: 1,
				timeout:     60 * time.Second,
			},
			skupperObjects: []runtime.Object{
				&v1alpha1.SiteList{
					Items: []v1alpha1.Site{
						{
							ObjectMeta: v1.ObjectMeta{
								Name:      "site1",
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
			expectedErrors: []string{"token name and file name must be configured"},
		},
		{
			name: "more than two arguments are specified",
			args: []string{"my-grant", "/home/user/my-grant.yaml", "test"},
			flags: TokenIssue{
				expiration:  15 * time.Minute,
				redemptions: 1,
				timeout:     60 * time.Second,
			},
			skupperObjects: []runtime.Object{
				&v1alpha1.SiteList{
					Items: []v1alpha1.Site{
						{
							ObjectMeta: v1.ObjectMeta{
								Name:      "site1",
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
			name: "token name is not valid.",
			args: []string{"my new token", "~/token.yaml"},
			flags: TokenIssue{
				expiration:  15 * time.Minute,
				redemptions: 1,
				timeout:     60 * time.Second,
			},
			skupperObjects: []runtime.Object{
				&v1alpha1.SiteList{
					Items: []v1alpha1.Site{
						{
							ObjectMeta: v1.ObjectMeta{
								Name:      "site1",
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
			expectedErrors: []string{"token name is not valid: value does not match this regular expression: ^[a-z0-9]([-a-z0-9]*[a-z0-9])*(\\.[a-z0-9]([-a-z0-9]*[a-z0-9])*)*$"},
		},
		{
			name: "token file name is not valid.",
			args: []string{"my-grant", "ab cd"},
			flags: TokenIssue{
				expiration:  15 * time.Minute,
				redemptions: 1,
				timeout:     60 * time.Second,
			},
			skupperObjects: []runtime.Object{
				&v1alpha1.SiteList{
					Items: []v1alpha1.Site{
						{
							ObjectMeta: v1.ObjectMeta{
								Name:      "site1",
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
			name: "redemptions is not valid",
			args: []string{"my-token-redemptions", "~/token.yaml"},
			flags: TokenIssue{
				expiration:  15 * time.Minute,
				redemptions: 0,
				timeout:     60 * time.Second,
			},
			skupperObjects: []runtime.Object{
				&v1alpha1.SiteList{
					Items: []v1alpha1.Site{
						{
							ObjectMeta: v1.ObjectMeta{
								Name:      "site1",
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
				"number of redemptions is not valid"},
		},
		{
			name: "expiration is not valid",
			args: []string{"my-token-expiration", "~/token.yaml"},
			flags: TokenIssue{
				expiration:  0 * time.Minute,
				redemptions: 1,
				timeout:     60 * time.Second,
			},
			skupperObjects: []runtime.Object{
				&v1alpha1.SiteList{
					Items: []v1alpha1.Site{
						{
							ObjectMeta: v1.ObjectMeta{
								Name:      "site1",
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
			expectedErrors: []string{"expiration time is not valid"},
		},
		{
			name: "timeout is not valid",
			args: []string{"token-timeout", "~/token.yaml"},
			flags: TokenIssue{
				expiration:  15 * time.Minute,
				redemptions: 1,
				timeout:     0 * time.Second,
			},
			skupperObjects: []runtime.Object{
				&v1alpha1.SiteList{
					Items: []v1alpha1.Site{
						{
							ObjectMeta: v1.ObjectMeta{
								Name:      "site1",
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
			name: "flags all valid",
			args: []string{"my-token-flags", "~/token.yaml"},
			flags: TokenIssue{
				expiration:  15 * time.Minute,
				redemptions: 1,
				timeout:     60 * time.Second,
			},
			skupperObjects: []runtime.Object{
				&v1alpha1.SiteList{
					Items: []v1alpha1.Site{
						{
							ObjectMeta: v1.ObjectMeta{
								Name:      "site1",
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
				&v1alpha1.AccessGrant{
					ObjectMeta: v1.ObjectMeta{
						Name:      "my-token",
						Namespace: "test",
					},
					Spec: v1alpha1.AccessGrantSpec{
						RedemptionsAllowed: 1,
						ExpirationWindow:   "15m0s",
					},
					Status: v1alpha1.AccessGrantStatus{
						Status: v1alpha1.Status{
							Conditions: []v1.Condition{
								{
									Type:   "Ready",
									Status: "True",
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

			command, err := newCmdTokenIssueWithMocks("test", test.k8sObjects, test.skupperObjects, "")
			assert.Assert(t, err)

			command.flags = test.flags

			actualErrors := command.ValidateInput(test.args)

			actualErrorsMessages := utils.ErrorsToMessages(actualErrors)

			assert.DeepEqual(t, actualErrorsMessages, test.expectedErrors)

		})
	}
}

func TestCmdTokenIssue_Run(t *testing.T) {
	type test struct {
		name                string
		k8sObjects          []runtime.Object
		skupperObjects      []runtime.Object
		grantName           string
		fileName            string
		errorMessage        string
		skupperErrorMessage string
		flags               TokenIssue
	}

	testTable := []test{
		{
			name:      "runs ok",
			grantName: "run-token",
			fileName:  "/tmp/token.yaml",
			flags: TokenIssue{
				expiration:  20 * time.Minute,
				redemptions: 2,
				timeout:     60 * time.Second,
			},
			skupperObjects: []runtime.Object{
				&v1alpha1.AccessGrant{
					ObjectMeta: v1.ObjectMeta{
						Name:      "my-token",
						Namespace: "test",
					},
					Status: v1alpha1.AccessGrantStatus{
						Status: v1alpha1.Status{
							Conditions: []v1.Condition{
								{
									Type:   "Ready",
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
			grantName:           "fail-token",
			skupperErrorMessage: "error",
			errorMessage:        "error",
		},
	}

	for _, test := range testTable {
		cmd, err := newCmdTokenIssueWithMocks("test", test.k8sObjects, test.skupperObjects, test.skupperErrorMessage)
		assert.Assert(t, err)

		cmd.grantName = test.grantName
		cmd.fileName = test.fileName
		cmd.flags.expiration = test.flags.expiration
		cmd.flags.redemptions = test.flags.redemptions

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

func TestCmdTokenIssue_WaitUntil(t *testing.T) {
	type test struct {
		name                string
		k8sObjects          []runtime.Object
		skupperObjects      []runtime.Object
		skupperErrorMessage string
		expectError         bool
	}
	defer os.Remove("/tmp/token.yaml") // clean up

	testTable := []test{
		{
			name: "token is not ready",
			skupperObjects: []runtime.Object{
				&v1alpha1.AccessGrant{
					ObjectMeta: v1.ObjectMeta{
						Name:      "my-token",
						Namespace: "test",
					},
					Status: v1alpha1.AccessGrantStatus{},
				},
			},
			skupperErrorMessage: "",
			expectError:         true,
		},
		{
			name:                "token is not returned",
			expectError:         true,
			skupperErrorMessage: "not found",
		},
		{
			name: "token is ready",
			skupperObjects: []runtime.Object{
				&v1alpha1.AccessGrant{
					ObjectMeta: v1.ObjectMeta{
						Name:      "my-token",
						Namespace: "test",
					},
					Spec: v1alpha1.AccessGrantSpec{
						RedemptionsAllowed: 1,
						ExpirationWindow:   "15m0s",
					},
					Status: v1alpha1.AccessGrantStatus{
						Status: v1alpha1.Status{
							Conditions: []v1.Condition{
								{
									Type:   "Ready",
									Status: "True",
								},
							},
						},
					},
				},
			},
			skupperErrorMessage: "",
			expectError:         false,
		},
	}

	for _, test := range testTable {
		cmd, err := newCmdTokenIssueWithMocks("test", test.k8sObjects, test.skupperObjects, test.skupperErrorMessage)
		assert.Assert(t, err)

		cmd.grantName = "my-token"
		cmd.fileName = "/tmp/token.yaml"
		cmd.flags = TokenIssue{
			expiration:  20 * time.Minute,
			redemptions: 2,
			timeout:     1 * time.Second,
		}
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

func newCmdTokenIssueWithMocks(namespace string, k8sObjects []runtime.Object, skupperObjects []runtime.Object, fakeSkupperError string) (*CmdTokenIssue, error) {

	// We make sure the interval is appropriate
	utils.SetRetryProfile(utils.TestRetryProfile)

	client, err := fakeclient.NewFakeClient(namespace, k8sObjects, skupperObjects, fakeSkupperError)
	if err != nil {
		return nil, err
	}
	cmdTokenIssue := &CmdTokenIssue{
		client:    client.GetSkupperClient().SkupperV1alpha1(),
		namespace: namespace,
	}

	return cmdTokenIssue, nil
}

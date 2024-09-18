package kube

import (
	"os"
	"testing"
	"time"

	"github.com/skupperproject/skupper/internal/cmd/skupper/common"
	"github.com/skupperproject/skupper/internal/cmd/skupper/common/utils"

	fakeclient "github.com/skupperproject/skupper/internal/kube/client/fake"
	"github.com/skupperproject/skupper/pkg/apis/skupper/v1alpha1"
	"gotest.tools/assert"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestCmdTokenIssue_ValidateInput(t *testing.T) {
	type test struct {
		name           string
		args           []string
		flags          common.CommandTokenIssueFlags
		k8sObjects     []runtime.Object
		skupperObjects []runtime.Object
		expectedErrors []string
	}

	testTable := []test{
		{
			name: "token is not issued because there is already the same token in the namespace",
			args: []string{"~/token.yaml"},
			flags: common.CommandTokenIssueFlags{
				ExpirationWindow:   15 * time.Minute,
				RedemptionsAllowed: 1,
				Timeout:            60 * time.Second,
				Name:               "my-token",
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
										{
											Type:   "Running",
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
			args: []string{"filename"},
			flags: common.CommandTokenIssueFlags{
				ExpirationWindow:   15 * time.Minute,
				RedemptionsAllowed: 1,
				Timeout:            60 * time.Second,
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
			args: []string{"filename"},
			flags: common.CommandTokenIssueFlags{
				ExpirationWindow:   15 * time.Minute,
				RedemptionsAllowed: 1,
				Timeout:            60 * time.Second,
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
			name: "file name is not specified",
			args: []string{},
			flags: common.CommandTokenIssueFlags{
				ExpirationWindow:   15 * time.Minute,
				RedemptionsAllowed: 1,
				Timeout:            60 * time.Second,
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
										{
											Type:   "Running",
											Status: "True",
										},
									},
								},
							},
						},
					},
				},
			},
			expectedErrors: []string{"file name must be configured"},
		},
		{
			name: "token file name empty",
			args: []string{""},
			flags: common.CommandTokenIssueFlags{
				ExpirationWindow:   15 * time.Minute,
				RedemptionsAllowed: 1,
				Timeout:            60 * time.Second,
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
										{
											Type:   "Running",
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
			name: "more than one arguments is specified",
			args: []string{"/home/user/my-grant.yaml", "test"},
			flags: common.CommandTokenIssueFlags{
				ExpirationWindow:   15 * time.Minute,
				RedemptionsAllowed: 1,
				Timeout:            60 * time.Second,
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
										{
											Type:   "Running",
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
			name: "token name is not valid.",
			args: []string{"~/token.yaml"},
			flags: common.CommandTokenIssueFlags{
				ExpirationWindow:   15 * time.Minute,
				RedemptionsAllowed: 1,
				Timeout:            60 * time.Second,
				Name:               "my new token",
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
										{
											Type:   "Running",
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
			args: []string{"ab cd"},
			flags: common.CommandTokenIssueFlags{
				ExpirationWindow:   15 * time.Minute,
				RedemptionsAllowed: 1,
				Timeout:            60 * time.Second,
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
										{
											Type:   "Running",
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
			args: []string{"~/token.yaml"},
			flags: common.CommandTokenIssueFlags{
				ExpirationWindow:   15 * time.Minute,
				RedemptionsAllowed: 0,
				Timeout:            60 * time.Second,
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
										{
											Type:   "Running",
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
			args: []string{"~/token.yaml"},
			flags: common.CommandTokenIssueFlags{
				ExpirationWindow:   0 * time.Minute,
				RedemptionsAllowed: 1,
				Timeout:            60 * time.Second,
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
										{
											Type:   "Running",
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
			args: []string{"~/token.yaml"},
			flags: common.CommandTokenIssueFlags{
				ExpirationWindow:   15 * time.Minute,
				RedemptionsAllowed: 1,
				Timeout:            0 * time.Second,
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
										{
											Type:   "Running",
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
			args: []string{"~/token.yaml"},
			flags: common.CommandTokenIssueFlags{
				ExpirationWindow:   15 * time.Minute,
				RedemptionsAllowed: 1,
				Timeout:            60 * time.Second,
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
										{
											Type:   "Running",
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

			command.Flags = &test.flags

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
		flags               common.CommandTokenIssueFlags
	}

	testTable := []test{
		{
			name:      "runs ok",
			grantName: "run-token",
			fileName:  "/tmp/token.yaml",
			flags: common.CommandTokenIssueFlags{
				ExpirationWindow:   20 * time.Minute,
				RedemptionsAllowed: 2,
				Timeout:            60 * time.Second,
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
		cmd.Flags = &common.CommandTokenIssueFlags{}
		cmd.Flags.ExpirationWindow = test.flags.ExpirationWindow
		cmd.Flags.RedemptionsAllowed = test.flags.RedemptionsAllowed

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
		cmd.Flags = &common.CommandTokenIssueFlags{
			ExpirationWindow:   20 * time.Minute,
			RedemptionsAllowed: 2,
			Timeout:            1 * time.Second,
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

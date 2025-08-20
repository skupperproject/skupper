package kube

import (
	"os"
	"testing"
	"time"

	"github.com/skupperproject/skupper/internal/cmd/skupper/common"
	"github.com/skupperproject/skupper/internal/cmd/skupper/common/testutils"
	"github.com/skupperproject/skupper/internal/cmd/skupper/common/utils"

	fakeclient "github.com/skupperproject/skupper/internal/kube/client/fake"
	"github.com/skupperproject/skupper/pkg/apis/skupper/v2alpha1"
	"gotest.tools/v3/assert"
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
		expectedError  string
		skupperError   string
	}

	testTable := []test{
		{
			name:          "missing CRD",
			args:          []string{"filename"},
			flags:         common.CommandTokenIssueFlags{},
			skupperError:  utils.CrdErr,
			expectedError: utils.CrdHelpErr,
		},
		{
			name: "token no site",
			args: []string{"filename"},
			flags: common.CommandTokenIssueFlags{
				ExpirationWindow:   15 * time.Minute,
				RedemptionsAllowed: 1,
				Timeout:            60 * time.Second,
				Cost:               "1",
			},
			skupperObjects: []runtime.Object{
				&v2alpha1.AccessGrant{
					ObjectMeta: v1.ObjectMeta{
						Name:      "my-token",
						Namespace: "test",
					},
					Status: v2alpha1.AccessGrantStatus{
						Status: v2alpha1.Status{
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
			expectedError: "A site must exist in namespace test before a token can be created",
		},
		{
			name: "token no site with OK status",
			args: []string{"filename"},
			flags: common.CommandTokenIssueFlags{
				ExpirationWindow:   15 * time.Minute,
				RedemptionsAllowed: 1,
				Timeout:            60 * time.Second,
				Cost:               "1",
			},
			skupperObjects: []runtime.Object{
				&v2alpha1.SiteList{
					Items: []v2alpha1.Site{
						{
							ObjectMeta: v1.ObjectMeta{
								Name:      "site1",
								Namespace: "test",
							},
							Status: v2alpha1.SiteStatus{
								Status: v2alpha1.Status{},
							},
						},
					},
				},
				&v2alpha1.AccessGrant{
					ObjectMeta: v1.ObjectMeta{
						Name:      "my-token",
						Namespace: "test",
					},
				},
			},
			expectedError: "there is no active skupper site in this namespace",
		},
		{
			name: "file name is not specified",
			args: []string{},
			flags: common.CommandTokenIssueFlags{
				ExpirationWindow:   15 * time.Minute,
				RedemptionsAllowed: 1,
				Timeout:            60 * time.Second,
				Cost:               "1",
			},
			skupperObjects: []runtime.Object{
				&v2alpha1.SiteList{
					Items: []v2alpha1.Site{
						{
							ObjectMeta: v1.ObjectMeta{
								Name:      "site1",
								Namespace: "test",
							},
							Spec: v2alpha1.SiteSpec{
								LinkAccess: "default",
							},
							Status: v2alpha1.SiteStatus{
								Status: v2alpha1.Status{
									Conditions: []v1.Condition{
										{
											Type:   "Configured",
											Status: "True",
										},
										{
											Type:   "Running",
											Status: "True",
										},
										{
											Type:   "Ready",
											Status: "True",
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
						},
					},
				},
			},
			expectedError: "file name must be configured",
		},
		{
			name: "token file name empty",
			args: []string{""},
			flags: common.CommandTokenIssueFlags{
				ExpirationWindow:   15 * time.Minute,
				RedemptionsAllowed: 1,
				Timeout:            60 * time.Second,
				Cost:               "1",
			},
			skupperObjects: []runtime.Object{
				&v2alpha1.SiteList{
					Items: []v2alpha1.Site{
						{
							ObjectMeta: v1.ObjectMeta{
								Name:      "site1",
								Namespace: "test",
							},
							Spec: v2alpha1.SiteSpec{
								LinkAccess: "loadbalancer",
							},
							Status: v2alpha1.SiteStatus{
								Status: v2alpha1.Status{
									Conditions: []v1.Condition{
										{
											Type:   "Configured",
											Status: "True",
										},
										{
											Type:   "Running",
											Status: "True",
										},
										{
											Type:   "Ready",
											Status: "True",
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
						},
					},
				},
			},
			expectedError: "file name must not be empty",
		},
		{
			name: "more than one arguments is specified",
			args: []string{"/home/user/my-grant.yaml", "test"},
			flags: common.CommandTokenIssueFlags{
				ExpirationWindow:   15 * time.Minute,
				RedemptionsAllowed: 1,
				Timeout:            60 * time.Second,
				Cost:               "1",
			},
			skupperObjects: []runtime.Object{
				&v2alpha1.SiteList{
					Items: []v2alpha1.Site{
						{
							ObjectMeta: v1.ObjectMeta{
								Name:      "site1",
								Namespace: "test",
							},
							Spec: v2alpha1.SiteSpec{
								LinkAccess: "default",
							},
							Status: v2alpha1.SiteStatus{
								Status: v2alpha1.Status{
									Conditions: []v1.Condition{
										{
											Type:   "Configured",
											Status: "True",
										},
										{
											Type:   "Running",
											Status: "True",
										},
										{
											Type:   "Ready",
											Status: "True",
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
						},
					},
				},
			},
			expectedError: "only one argument is allowed for this command",
		},
		{
			name: "token file name is not valid.",
			args: []string{"ab cd"},
			flags: common.CommandTokenIssueFlags{
				ExpirationWindow:   15 * time.Minute,
				RedemptionsAllowed: 1,
				Timeout:            60 * time.Second,
				Cost:               "1",
			},
			skupperObjects: []runtime.Object{
				&v2alpha1.SiteList{
					Items: []v2alpha1.Site{
						{
							ObjectMeta: v1.ObjectMeta{
								Name:      "site1",
								Namespace: "test",
							},
							Spec: v2alpha1.SiteSpec{
								LinkAccess: "default",
							},
							Status: v2alpha1.SiteStatus{
								Status: v2alpha1.Status{
									Conditions: []v1.Condition{
										{
											Type:   "Configured",
											Status: "True",
										},
										{
											Type:   "Running",
											Status: "True",
										},
										{
											Type:   "Ready",
											Status: "True",
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
						},
					},
				},
			},
			expectedError: "token file name is not valid: value does not match this regular expression: ^[A-Za-z0-9./~-]+$",
		},
		{
			name: "token name is a directory.",
			args: []string{"/tmp"},
			flags: common.CommandTokenIssueFlags{
				ExpirationWindow:   15 * time.Minute,
				RedemptionsAllowed: 1,
				Timeout:            60 * time.Second,
				Cost:               "1",
			},
			skupperObjects: []runtime.Object{
				&v2alpha1.SiteList{
					Items: []v2alpha1.Site{
						{
							ObjectMeta: v1.ObjectMeta{
								Name:      "site1",
								Namespace: "test",
							},
							Spec: v2alpha1.SiteSpec{
								LinkAccess: "route",
							},
							Status: v2alpha1.SiteStatus{
								Status: v2alpha1.Status{
									Conditions: []v1.Condition{
										{
											Type:   "Configured",
											Status: "True",
										},
										{
											Type:   "Running",
											Status: "True",
										},
										{
											Type:   "Ready",
											Status: "True",
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
						},
					},
				},
			},
			expectedError: "token file name is a directory",
		},
		{
			name: "directory doesn't exist.",
			args: []string{"/invalid/token.yaml"},
			flags: common.CommandTokenIssueFlags{
				ExpirationWindow:   15 * time.Minute,
				RedemptionsAllowed: 1,
				Timeout:            60 * time.Second,
				Cost:               "1",
			},
			skupperObjects: []runtime.Object{
				&v2alpha1.SiteList{
					Items: []v2alpha1.Site{
						{
							ObjectMeta: v1.ObjectMeta{
								Name:      "site1",
								Namespace: "test",
							},
							Spec: v2alpha1.SiteSpec{
								LinkAccess: "route",
							},
							Status: v2alpha1.SiteStatus{
								Status: v2alpha1.Status{
									Conditions: []v1.Condition{
										{
											Type:   "Configured",
											Status: "True",
										},
										{
											Type:   "Running",
											Status: "True",
										},
										{
											Type:   "Ready",
											Status: "True",
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
						},
					},
				},
			},
			expectedError: "invalid token file name: no such file or directory",
		},
		{
			name: "redemptions is not valid",
			args: []string{"token.yaml"},
			flags: common.CommandTokenIssueFlags{
				ExpirationWindow:   15 * time.Minute,
				RedemptionsAllowed: 0,
				Timeout:            60 * time.Second,
				Cost:               "1",
			},
			skupperObjects: []runtime.Object{
				&v2alpha1.SiteList{
					Items: []v2alpha1.Site{
						{
							ObjectMeta: v1.ObjectMeta{
								Name:      "site1",
								Namespace: "test",
							},
							Spec: v2alpha1.SiteSpec{
								LinkAccess: "default",
							},
							Status: v2alpha1.SiteStatus{
								Status: v2alpha1.Status{
									Conditions: []v1.Condition{
										{
											Type:   "Configured",
											Status: "True",
										},
										{
											Type:   "Running",
											Status: "True",
										},
										{
											Type:   "Ready",
											Status: "True",
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
						},
					},
				},
			},
			expectedError: "number of redemptions is not valid",
		},
		{
			name: "expiration is not valid",
			args: []string{"token.yaml"},
			flags: common.CommandTokenIssueFlags{
				ExpirationWindow:   10 * time.Second,
				RedemptionsAllowed: 1,
				Timeout:            10 * time.Second,
				Cost:               "1",
			},
			skupperObjects: []runtime.Object{
				&v2alpha1.SiteList{
					Items: []v2alpha1.Site{
						{
							ObjectMeta: v1.ObjectMeta{
								Name:      "site1",
								Namespace: "test",
							},
							Spec: v2alpha1.SiteSpec{
								LinkAccess: "default",
							},
							Status: v2alpha1.SiteStatus{
								Status: v2alpha1.Status{
									Conditions: []v1.Condition{
										{
											Type:   "Configured",
											Status: "True",
										},
										{
											Type:   "Running",
											Status: "True",
										},
										{
											Type:   "Ready",
											Status: "True",
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
						},
					},
				},
			},
			expectedError: "expiration time is not valid: duration must not be less than 1m0s; got 10s",
		},
		{
			name: "timeout is not valid",
			args: []string{"token.yaml"},
			flags: common.CommandTokenIssueFlags{
				ExpirationWindow:   15 * time.Minute,
				RedemptionsAllowed: 1,
				Timeout:            0 * time.Second,
				Cost:               "1",
			},
			skupperObjects: []runtime.Object{
				&v2alpha1.SiteList{
					Items: []v2alpha1.Site{
						{
							ObjectMeta: v1.ObjectMeta{
								Name:      "site1",
								Namespace: "test",
							},
							Spec: v2alpha1.SiteSpec{
								LinkAccess: "default",
							},
							Status: v2alpha1.SiteStatus{
								Status: v2alpha1.Status{
									Conditions: []v1.Condition{
										{
											Type:   "Configured",
											Status: "True",
										},
										{
											Type:   "Running",
											Status: "True",
										},
										{
											Type:   "Ready",
											Status: "True",
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
						},
					},
				},
			},
			expectedError: "timeout is not valid: duration must not be less than 10s; got 0s",
		},
		{
			name: "cost is not valid",
			args: []string{"token.yaml"},
			flags: common.CommandTokenIssueFlags{
				ExpirationWindow:   15 * time.Minute,
				RedemptionsAllowed: 1,
				Timeout:            60 * time.Second,
				Cost:               "Not-valid",
			},
			skupperObjects: []runtime.Object{
				&v2alpha1.SiteList{
					Items: []v2alpha1.Site{
						{
							ObjectMeta: v1.ObjectMeta{
								Name:      "site1",
								Namespace: "test",
							},
							Spec: v2alpha1.SiteSpec{
								LinkAccess: "default",
							},
							Status: v2alpha1.SiteStatus{
								Status: v2alpha1.Status{
									Conditions: []v1.Condition{
										{
											Type:   "Configured",
											Status: "True",
										},
										{
											Type:   "Running",
											Status: "True",
										},
										{
											Type:   "Ready",
											Status: "True",
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
						},
					},
				},
			},
			expectedError: `link cost is not valid: strconv.Atoi: parsing "Not-valid": invalid syntax`,
		},
		{
			name: "link access is not valid",
			args: []string{"token.yaml"},
			flags: common.CommandTokenIssueFlags{
				ExpirationWindow:   15 * time.Minute,
				RedemptionsAllowed: 1,
				Timeout:            60 * time.Second,
				Cost:               "1",
			},
			skupperObjects: []runtime.Object{
				&v2alpha1.SiteList{
					Items: []v2alpha1.Site{
						{
							ObjectMeta: v1.ObjectMeta{
								Name:      "site1",
								Namespace: "test",
							},
							Spec: v2alpha1.SiteSpec{
								LinkAccess: "local",
							},
							Status: v2alpha1.SiteStatus{
								Status: v2alpha1.Status{
									Conditions: []v1.Condition{
										{
											Type:   "Configured",
											Status: "True",
										},
										{
											Type:   "Running",
											Status: "True",
										},
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
			},
			expectedError: `You must enable link access for this site before you can create a token.`,
		},
		{
			name: "flags all valid",
			args: []string{"/tmp/token.yaml"},
			flags: common.CommandTokenIssueFlags{
				ExpirationWindow:   15 * time.Minute,
				RedemptionsAllowed: 1,
				Timeout:            60 * time.Second,
				Cost:               "5",
			},
			skupperObjects: []runtime.Object{
				&v2alpha1.SiteList{
					Items: []v2alpha1.Site{
						{
							ObjectMeta: v1.ObjectMeta{
								Name:      "site1",
								Namespace: "test",
							},
							Spec: v2alpha1.SiteSpec{
								LinkAccess: "default",
							},
							Status: v2alpha1.SiteStatus{
								Status: v2alpha1.Status{
									Conditions: []v1.Condition{
										{
											Type:   "Configured",
											Status: "True",
										},
										{
											Type:   "Running",
											Status: "True",
										},
										{
											Type:   "Ready",
											Status: "True",
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
						},
					},
				},
				&v2alpha1.AccessGrant{
					ObjectMeta: v1.ObjectMeta{
						Name:      "my-token",
						Namespace: "test",
					},
					Spec: v2alpha1.AccessGrantSpec{
						RedemptionsAllowed: 1,
						ExpirationWindow:   "15m0s",
					},
					Status: v2alpha1.AccessGrantStatus{
						Status: v2alpha1.Status{
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
			expectedError: "",
		},
	}

	for _, test := range testTable {
		t.Run(test.name, func(t *testing.T) {

			command, err := newCmdTokenIssueWithMocks("test", test.k8sObjects, test.skupperObjects, test.skupperError)
			assert.Assert(t, err)

			command.Flags = &test.flags

			testutils.CheckValidateInput(t, command, test.expectedError, test.args)
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
				Cost:               "5",
			},
			skupperObjects: []runtime.Object{
				&v2alpha1.AccessGrant{
					ObjectMeta: v1.ObjectMeta{
						Name:      "my-token",
						Namespace: "test",
					},
					Status: v2alpha1.AccessGrantStatus{
						Status: v2alpha1.Status{
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
				&v2alpha1.AccessGrant{
					ObjectMeta: v1.ObjectMeta{
						Name:      "my-token",
						Namespace: "test",
					},
					Status: v2alpha1.AccessGrantStatus{},
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
				&v2alpha1.AccessGrant{
					ObjectMeta: v1.ObjectMeta{
						Name:      "my-token",
						Namespace: "test",
					},
					Spec: v2alpha1.AccessGrantSpec{
						RedemptionsAllowed: 1,
						ExpirationWindow:   "15m0s",
					},
					Status: v2alpha1.AccessGrantStatus{
						Status: v2alpha1.Status{
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
		client:    client.GetSkupperClient().SkupperV2alpha1(),
		namespace: namespace,
	}

	return cmdTokenIssue, nil
}

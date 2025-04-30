package kube

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/skupperproject/skupper/internal/cmd/skupper/common"
	"github.com/skupperproject/skupper/internal/cmd/skupper/common/testutils"
	"github.com/skupperproject/skupper/internal/cmd/skupper/common/utils"

	fakeclient "github.com/skupperproject/skupper/internal/kube/client/fake"
	"github.com/skupperproject/skupper/pkg/apis/skupper/v2alpha1"
	"github.com/skupperproject/skupper/pkg/generated/client/clientset/versioned/scheme"
	"gotest.tools/v3/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
)

func TestCmdTokenRedeem_ValidateInput(t *testing.T) {
	type test struct {
		name           string
		args           []string
		flags          common.CommandTokenRedeemFlags
		k8sObjects     []runtime.Object
		skupperObjects []runtime.Object
		expectedError  string
	}

	// create temp token file for tests
	_, err := os.Create("/tmp/token-redeem.yaml")
	assert.Check(t, err == nil)

	defer os.Remove("/tmp/token-redeem.yaml") // clean up

	testTable := []test{
		{
			name:          "token no site",
			args:          []string{"/tmp/token-redeem.yaml"},
			flags:         common.CommandTokenRedeemFlags{Timeout: 60 * time.Second},
			expectedError: "A site must exist in namespace test before a token can be redeemed",
		},
		{
			name:  "token not site ok",
			args:  []string{"/tmp/token-redeem.yaml"},
			flags: common.CommandTokenRedeemFlags{Timeout: 60 * time.Second},
			skupperObjects: []runtime.Object{
				&v2alpha1.SiteList{
					Items: []v2alpha1.Site{
						{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "the-site",
								Namespace: "test",
							},
						},
					},
				},
			},
			expectedError: "there is no active skupper site in this namespace",
		},
		{
			name:  "file name is not specified",
			args:  []string{},
			flags: common.CommandTokenRedeemFlags{Timeout: 60 * time.Second},
			skupperObjects: []runtime.Object{
				&v2alpha1.SiteList{
					Items: []v2alpha1.Site{
						{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "the-site",
								Namespace: "test",
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
			expectedError: "token file name must be configured",
		},
		{
			name:  "file name empty",
			args:  []string{""},
			flags: common.CommandTokenRedeemFlags{Timeout: 60 * time.Second},
			skupperObjects: []runtime.Object{
				&v2alpha1.SiteList{
					Items: []v2alpha1.Site{
						{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "the-site",
								Namespace: "test",
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
			expectedError: "file name must not be empty",
		},
		{
			name:  "more than one argument is specified",
			args:  []string{"my-grant", "/home/user/my-grant.yaml"},
			flags: common.CommandTokenRedeemFlags{Timeout: 60 * time.Second},
			skupperObjects: []runtime.Object{
				&v2alpha1.SiteList{
					Items: []v2alpha1.Site{
						{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "the-site",
								Namespace: "test",
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
			expectedError: "only one argument is allowed for this command",
		},
		{
			name:  "token file name is not valid.",
			args:  []string{"my new file"},
			flags: common.CommandTokenRedeemFlags{Timeout: 60 * time.Second},
			skupperObjects: []runtime.Object{
				&v2alpha1.SiteList{
					Items: []v2alpha1.Site{
						{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "the-site",
								Namespace: "test",
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
			expectedError: "token file name is not valid: value does not match this regular expression: ^[A-Za-z0-9./~-]+$",
		},
		{
			name:  "timeout is not valid",
			args:  []string{"~/token.yaml"},
			flags: common.CommandTokenRedeemFlags{Timeout: 0 * time.Second},
			skupperObjects: []runtime.Object{
				&v2alpha1.SiteList{
					Items: []v2alpha1.Site{
						{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "the-site",
								Namespace: "test",
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
			expectedError: "token file does not exist: stat ~/token.yaml: no such file or directory\n" +
				"timeout is not valid: duration must not be less than 10s; got 0s"},
		{
			name:  "flags all valid",
			args:  []string{"/tmp/token-redeem.yaml"},
			flags: common.CommandTokenRedeemFlags{Timeout: 50 * time.Second},
			skupperObjects: []runtime.Object{
				&v2alpha1.SiteList{
					Items: []v2alpha1.Site{
						{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "the-site",
								Namespace: "test",
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
			expectedError: "",
		},
	}

	for _, test := range testTable {
		t.Run(test.name, func(t *testing.T) {

			command, err := newCmdTokenRedeemWithMocks("test", test.k8sObjects, test.skupperObjects, "")
			assert.Assert(t, err)

			command.Flags = &test.flags

			testutils.CheckValidateInput(t, command, test.expectedError, test.args)
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

	err := newCmdCreateAccessTokenFile("/tmp/tokenR.yaml")
	assert.Check(t, err == nil)

	testTable := []test{
		{
			name: "runs ok",
			skupperObjects: []runtime.Object{
				&v2alpha1.AccessToken{
					ObjectMeta: v1.ObjectMeta{
						Name:      "my-token",
						Namespace: "test",
					},
					Status: v2alpha1.AccessTokenStatus{
						Status: v2alpha1.Status{
							Conditions: []v1.Condition{
								{
									Type:   "Redeemed",
									Status: "True",
								},
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

func TestCmdTokenRedeem_WaitUntil(t *testing.T) {
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
				&v2alpha1.AccessToken{
					ObjectMeta: v1.ObjectMeta{
						Name:      "my-grant",
						Namespace: "test",
					},
					Status: v2alpha1.AccessTokenStatus{},
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
				&v2alpha1.AccessToken{
					ObjectMeta: v1.ObjectMeta{
						Name:      "my-grant",
						Namespace: "test",
					},
					Status: v2alpha1.AccessTokenStatus{
						Status: v2alpha1.Status{
							Conditions: []v1.Condition{
								{
									Type:   "Redeemed",
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
		cmd, err := newCmdTokenRedeemWithMocks("test", test.k8sObjects, test.skupperObjects, test.skupperErrorMessage)
		assert.Assert(t, err)

		cmd.name = "my-grant"
		cmd.fileName = "/tmp/token.yaml"
		cmd.Flags = &common.CommandTokenRedeemFlags{Timeout: 1 * time.Second}
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

func newCmdCreateAccessTokenFile(fileName string) error {

	resource := v2alpha1.AccessToken{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "skupper.io/v2alpha1",
			Kind:       "AccessToken",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "token",
		},
		Spec: v2alpha1.AccessTokenSpec{
			Url:  "AAA",
			Ca:   "BBB",
			Code: "CCC",
		},
	}

	s := json.NewYAMLSerializer(json.DefaultMetaFactory, scheme.Scheme, scheme.Scheme)
	out, err := os.Create(fileName)
	if err != nil {
		return fmt.Errorf("could not write to file %s:%s ", fileName, err.Error())
	}
	err = s.Encode(&resource, out)
	if err != nil {
		return fmt.Errorf("could not write out generated token: %s", err.Error())
	}

	return nil

}

func newCmdTokenRedeemWithMocks(namespace string, k8sObjects []runtime.Object, skupperObjects []runtime.Object, fakeSkupperError string) (*CmdTokenRedeem, error) {

	// We make sure the interval is appropriate
	utils.SetRetryProfile(utils.TestRetryProfile)
	client, err := fakeclient.NewFakeClient(namespace, k8sObjects, skupperObjects, fakeSkupperError)
	if err != nil {
		return nil, err
	}
	cmdTokenRedeem := &CmdTokenRedeem{
		client:    client.GetSkupperClient().SkupperV2alpha1(),
		namespace: namespace,
	}

	return cmdTokenRedeem, nil
}

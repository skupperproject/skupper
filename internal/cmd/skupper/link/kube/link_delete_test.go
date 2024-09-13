package kube

import (
	"github.com/skupperproject/skupper/internal/cmd/skupper/common"
	"github.com/skupperproject/skupper/internal/cmd/skupper/common/utils"
	fakeclient "github.com/skupperproject/skupper/internal/kube/client/fake"
	"github.com/skupperproject/skupper/pkg/apis/skupper/v1alpha1"
	"gotest.tools/assert"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"testing"
)

func TestCmdLinkDelete_ValidateInput(t *testing.T) {
	type test struct {
		name           string
		args           []string
		flags          common.CommandLinkDeleteFlags
		k8sObjects     []runtime.Object
		skupperObjects []runtime.Object
		expectedErrors []string
	}

	testTable := []test{
		{
			name:           "there is no active skupper site in this namespace",
			args:           []string{"my-link"},
			flags:          common.CommandLinkDeleteFlags{Timeout: "60"},
			expectedErrors: []string{"there is no skupper site in this namespace", "the link \"my-link\" is not available in the namespace"},
		},
		{
			name:  "link is not deleted because it does not exist",
			args:  []string{"my-link"},
			flags: common.CommandLinkDeleteFlags{Timeout: "60"},
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
			expectedErrors: []string{"the link \"my-link\" is not available in the namespace"},
		},
		{
			name:  "more than one argument was specified",
			args:  []string{"my", "link"},
			flags: common.CommandLinkDeleteFlags{Timeout: "60"},
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
			expectedErrors: []string{"only one argument is allowed for this command"},
		},
		{
			name:  "trying to delete without specifying a name",
			args:  []string{""},
			flags: common.CommandLinkDeleteFlags{Timeout: "60"},
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
			expectedErrors: []string{"link name must not be empty"},
		},
		{
			name:  "link deleted successfully",
			args:  []string{"my-link"},
			flags: common.CommandLinkDeleteFlags{Timeout: "60"},
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
			expectedErrors: []string{},
		},
		{
			name:  "timeout is not valid because it is negative",
			args:  []string{"my-link"},
			flags: common.CommandLinkDeleteFlags{Timeout: "-1"},
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
			expectedErrors: []string{"timeout is not valid: value is not positive"},
		},
		{
			name:  "timeout is not valid because it is zero",
			args:  []string{"my-link"},
			flags: common.CommandLinkDeleteFlags{Timeout: "0"},
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
			expectedErrors: []string{"timeout is not valid: value 0 is not allowed"},
		},
		{
			name:  "timeout is not valid because it is not a number",
			args:  []string{"my-link"},
			flags: common.CommandLinkDeleteFlags{Timeout: "one"},
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
			expectedErrors: []string{"timeout is not valid: strconv.Atoi: parsing \"one\": invalid syntax"},
		},
	}

	for _, test := range testTable {
		t.Run(test.name, func(t *testing.T) {

			command, err := newCmdLinkDeleteWithMocks("test", test.k8sObjects, test.skupperObjects, "")
			assert.Assert(t, err)
			command.Flags = &test.flags

			actualErrors := command.ValidateInput(test.args)

			actualErrorsMessages := utils.ErrorsToMessages(actualErrors)

			assert.DeepEqual(t, actualErrorsMessages, test.expectedErrors)

		})
	}
}

func TestCmdLinkDelete_InputToOptions(t *testing.T) {

	type test struct {
		name            string
		flags           common.CommandLinkDeleteFlags
		expectedTimeout int
	}

	testTable := []test{
		{
			name:            "check options",
			flags:           common.CommandLinkDeleteFlags{"60"},
			expectedTimeout: 60,
		},
	}

	for _, test := range testTable {
		t.Run(test.name, func(t *testing.T) {

			cmd, err := newCmdLinkDeleteWithMocks("test", nil, nil, "")
			assert.Assert(t, err)
			cmd.Flags = &test.flags

			cmd.InputToOptions()

			assert.Check(t, cmd.timeout == test.expectedTimeout)
		})
	}
}

func TestCmdLinkDelete_Run(t *testing.T) {
	type test struct {
		name           string
		k8sObjects     []runtime.Object
		skupperObjects []runtime.Object
		linkName       string
		errorMessage   string
	}

	testTable := []test{
		{
			name:     "runs ok",
			linkName: "my-link",
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
			name:         "run fails",
			errorMessage: "error",
			linkName:     "my-link",
		},
	}

	for _, test := range testTable {
		cmd, err := newCmdLinkDeleteWithMocks("test", nil, test.skupperObjects, test.errorMessage)
		assert.Assert(t, err)
		cmd.linkName = test.linkName

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

func TestCmdLinkDelete_WaitUntil(t *testing.T) {
	type test struct {
		name           string
		timeout        int
		skupperObjects []runtime.Object
		expectError    bool
	}

	testTable := []test{
		{
			name: "link is not deleted",
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
			timeout:     1,
			expectError: true,
		},
		{
			name:        "link is deleted",
			timeout:     1,
			expectError: false,
		},
	}

	for _, test := range testTable {
		cmd, err := newCmdLinkDeleteWithMocks("test", nil, test.skupperObjects, "")
		assert.Assert(t, err)
		cmd.linkName = "my-link"
		cmd.timeout = test.timeout
		t.Run(test.name, func(t *testing.T) {

			err := cmd.WaitUntil()
			if err != nil {
				assert.Check(t, test.expectError)
			}

		})
	}
}

// --- helper methods

func newCmdLinkDeleteWithMocks(namespace string, k8sObjects []runtime.Object, skupperObjects []runtime.Object, fakeSkupperError string) (*CmdLinkDelete, error) {

	// We make sure the interval is appropriate
	utils.SetRetryProfile(utils.TestRetryProfile)
	client, err := fakeclient.NewFakeClient(namespace, k8sObjects, skupperObjects, fakeSkupperError)
	if err != nil {
		return nil, err
	}
	cmdLinkDelete := &CmdLinkDelete{
		Client:    client.GetSkupperClient().SkupperV1alpha1(),
		Namespace: namespace,
	}

	return cmdLinkDelete, nil
}

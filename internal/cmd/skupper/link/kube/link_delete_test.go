package kube

import (
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

func TestCmdLinkDelete_ValidateInput(t *testing.T) {
	type test struct {
		name           string
		args           []string
		flags          common.CommandLinkDeleteFlags
		k8sObjects     []runtime.Object
		skupperObjects []runtime.Object
		expectedError  string
	}

	testTable := []test{
		{
			name:  "there is no active skupper site in this namespace",
			args:  []string{"my-link"},
			flags: common.CommandLinkDeleteFlags{Timeout: time.Minute},
			expectedError: "there is no skupper site in this namespace\n" +
				"the link \"my-link\" is not available in the namespace",
		},
		{
			name:  "link is not deleted because it does not exist",
			args:  []string{"my-link"},
			flags: common.CommandLinkDeleteFlags{Timeout: time.Minute},
			skupperObjects: []runtime.Object{
				&v2alpha1.SiteList{
					Items: []v2alpha1.Site{
						{
							ObjectMeta: v1.ObjectMeta{
								Name:      "the-site",
								Namespace: "test",
							},
						},
					},
				},
			},
			expectedError: "the link \"my-link\" is not available in the namespace",
		},
		{
			name:  "more than one argument was specified",
			args:  []string{"my", "link"},
			flags: common.CommandLinkDeleteFlags{Timeout: time.Minute},
			skupperObjects: []runtime.Object{
				&v2alpha1.SiteList{
					Items: []v2alpha1.Site{
						{
							ObjectMeta: v1.ObjectMeta{
								Name:      "the-site",
								Namespace: "test",
							},
						},
					},
				},
			},
			expectedError: "only one argument is allowed for this command",
		},
		{
			name:  "trying to delete without specifying a name",
			args:  []string{""},
			flags: common.CommandLinkDeleteFlags{Timeout: time.Minute},
			skupperObjects: []runtime.Object{
				&v2alpha1.SiteList{
					Items: []v2alpha1.Site{
						{
							ObjectMeta: v1.ObjectMeta{
								Name:      "the-site",
								Namespace: "test",
							},
						},
					},
				},
			},
			expectedError: "link name must not be empty",
		},
		{
			name:  "link deleted successfully",
			args:  []string{"my-link"},
			flags: common.CommandLinkDeleteFlags{Timeout: time.Minute},
			skupperObjects: []runtime.Object{
				&v2alpha1.SiteList{
					Items: []v2alpha1.Site{
						{
							ObjectMeta: v1.ObjectMeta{
								Name:      "the-site",
								Namespace: "test",
							},
						},
					},
				},
				&v2alpha1.Link{
					ObjectMeta: v1.ObjectMeta{
						Name:      "my-link",
						Namespace: "test",
					},
				},
			},
			expectedError: "",
		},
		{
			name:  "timeout is not valid because it is zero",
			args:  []string{"my-link"},
			flags: common.CommandLinkDeleteFlags{Timeout: time.Second * 0},
			skupperObjects: []runtime.Object{
				&v2alpha1.SiteList{
					Items: []v2alpha1.Site{
						{
							ObjectMeta: v1.ObjectMeta{
								Name:      "the-site",
								Namespace: "test",
							},
						},
					},
				},
				&v2alpha1.Link{
					ObjectMeta: v1.ObjectMeta{
						Name:      "my-link",
						Namespace: "test",
					},
				},
			},
			expectedError: "timeout is not valid: duration must not be less than 10s; got 0s",
		},
	}

	for _, test := range testTable {
		t.Run(test.name, func(t *testing.T) {

			command, err := newCmdLinkDeleteWithMocks("test", test.k8sObjects, test.skupperObjects, "")
			assert.Assert(t, err)
			command.Flags = &test.flags

			testutils.CheckValidateInput(t, command, test.expectedError, test.args)
		})
	}
}

func TestCmdLinkDelete_InputToOptions(t *testing.T) {

	type test struct {
		name            string
		flags           common.CommandLinkDeleteFlags
		expectedTimeout time.Duration
		expectedWait    bool
	}

	testTable := []test{
		{
			name:            "check options",
			flags:           common.CommandLinkDeleteFlags{Timeout: time.Minute, Wait: false},
			expectedTimeout: time.Minute,
			expectedWait:    false,
		},
	}

	for _, test := range testTable {
		t.Run(test.name, func(t *testing.T) {

			cmd, err := newCmdLinkDeleteWithMocks("test", nil, nil, "")
			assert.Assert(t, err)
			cmd.Flags = &test.flags

			cmd.InputToOptions()

			assert.Check(t, cmd.timeout == test.expectedTimeout)
			assert.Check(t, cmd.wait == test.expectedWait)
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
				&v2alpha1.Link{
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
		timeout        time.Duration
		wait           bool
		skupperObjects []runtime.Object
		expectError    bool
	}

	testTable := []test{
		{
			name: "link is not deleted",
			skupperObjects: []runtime.Object{
				&v2alpha1.Link{
					ObjectMeta: v1.ObjectMeta{
						Name:      "my-link",
						Namespace: "test",
					},
					Status: v2alpha1.LinkStatus{
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
			},
			timeout:     time.Second,
			expectError: true,
		},
		{
			name:        "link is deleted",
			timeout:     time.Second,
			expectError: false,
		},
		{
			name: "link is not deleted but user does not want to wait",
			wait: false,
			skupperObjects: []runtime.Object{
				&v2alpha1.Link{
					ObjectMeta: v1.ObjectMeta{
						Name:      "my-link",
						Namespace: "test",
					},
					Status: v2alpha1.LinkStatus{
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
			},
			timeout:     time.Second,
			expectError: false,
		},
	}

	for _, test := range testTable {
		cmd, err := newCmdLinkDeleteWithMocks("test", nil, test.skupperObjects, "")
		assert.Assert(t, err)
		cmd.linkName = "my-link"
		cmd.timeout = test.timeout
		cmd.wait = test.wait
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
		Client:    client.GetSkupperClient().SkupperV2alpha1(),
		Namespace: namespace,
	}

	return cmdLinkDelete, nil
}

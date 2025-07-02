package kube

import (
	"testing"

	"github.com/skupperproject/skupper/internal/cmd/skupper/common"
	"github.com/skupperproject/skupper/internal/cmd/skupper/common/testutils"
	"github.com/skupperproject/skupper/internal/cmd/skupper/common/utils"
	fakeclient "github.com/skupperproject/skupper/internal/kube/client/fake"
	"github.com/skupperproject/skupper/pkg/apis/skupper/v2alpha1"
	"gotest.tools/v3/assert"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestCmdSiteStatus_ValidateInput(t *testing.T) {
	type test struct {
		name           string
		args           []string
		flags          common.CommandSiteStatusFlags
		k8sObjects     []runtime.Object
		skupperObjects []runtime.Object
		skupperError   string
		expectedError  string
	}

	testTable := []test{
		{
			name:          "more than one argument was specified",
			args:          []string{"my-site", ""},
			flags:         common.CommandSiteStatusFlags{},
			expectedError: "this command does not need any arguments",
		},
		{
			name:          "bad output flag",
			flags:         common.CommandSiteStatusFlags{Output: "not-supported"},
			expectedError: "output type is not valid: value not-supported not allowed. It should be one of this options: [json yaml]",
		},
		{
			name:  "good output flag",
			flags: common.CommandSiteStatusFlags{Output: "yaml"},
		},
	}

	for _, test := range testTable {
		t.Run(test.name, func(t *testing.T) {
			command := &CmdSiteStatus{
				Namespace: "test",
			}

			fakeSkupperClient, err := fakeclient.NewFakeClient(command.Namespace, test.k8sObjects, test.skupperObjects, test.skupperError)
			assert.Assert(t, err)
			command.Client = fakeSkupperClient.GetSkupperClient().SkupperV2alpha1()
			command.Flags = &test.flags
			testutils.CheckValidateInput(t, command, test.expectedError, test.args)
		})
	}
}

func TestCmdSiteStatus_Run(t *testing.T) {
	type test struct {
		name           string
		k8sObjects     []runtime.Object
		skupperObjects []runtime.Object
		skupperError   string
		errorMessage   string
		output         string
	}

	testTable := []test{
		{
			name:           "missing CRD",
			k8sObjects:     nil,
			skupperObjects: nil,
			skupperError:   utils.CrdErr,
			errorMessage:   utils.CrdHelpErr,
		},
		{
			name:       "runs ok",
			k8sObjects: nil,
			skupperObjects: []runtime.Object{
				&v2alpha1.Site{
					ObjectMeta: v1.ObjectMeta{
						Name:      "old-site",
						Namespace: "test",
					},
					Status: v2alpha1.SiteStatus{
						Status: v2alpha1.Status{
							Message: "OK",
						},
					},
				},
			},
			skupperError: "",
			errorMessage: "",
		},
		{
			name:           "run fails",
			k8sObjects:     nil,
			skupperObjects: nil,
			skupperError:   "error",
			errorMessage:   "error",
		},
		{
			name:           "there is no existing skupper site",
			k8sObjects:     nil,
			skupperObjects: nil,
			skupperError:   "",
			errorMessage:   "",
		},
		{
			name:       "runs ok, output yaml",
			k8sObjects: nil,
			skupperObjects: []runtime.Object{
				&v2alpha1.Site{
					ObjectMeta: v1.ObjectMeta{
						Name:      "old-site",
						Namespace: "test",
					},
					Status: v2alpha1.SiteStatus{
						Status: v2alpha1.Status{
							Message: "OK",
						},
					},
				},
			},
			skupperError: "",
			errorMessage: "",
			output:       "yaml",
		},
	}

	for _, test := range testTable {
		command := &CmdSiteStatus{
			Namespace: "test",
			output:    test.output,
		}

		fakeSkupperClient, err := fakeclient.NewFakeClient(command.Namespace, test.k8sObjects, test.skupperObjects, test.skupperError)
		assert.Assert(t, err)
		command.Client = fakeSkupperClient.GetSkupperClient().SkupperV2alpha1()

		t.Run(test.name, func(t *testing.T) {

			err := command.Run()
			if err != nil {
				assert.Check(t, test.errorMessage == err.Error())
			} else {
				assert.Check(t, err == nil)
			}
		})
	}
}

func TestCmdSiteStatus_WaitUntil(t *testing.T) {

	t.Run("", func(t *testing.T) {

		command := &CmdSiteStatus{
			Namespace: "test",
		}

		fakeSkupperClient, err := fakeclient.NewFakeClient(command.Namespace, nil, nil, "")
		assert.Assert(t, err)
		command.Client = fakeSkupperClient.GetSkupperClient().SkupperV2alpha1()

		result := command.WaitUntil()
		assert.Check(t, result == nil)

	})

}

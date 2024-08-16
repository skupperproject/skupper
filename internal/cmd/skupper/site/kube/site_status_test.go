package kube

import (
	"github.com/skupperproject/skupper/internal/cmd/skupper/common/utils"
	"testing"

	fakeclient "github.com/skupperproject/skupper/internal/kube/client/fake"
	"github.com/skupperproject/skupper/pkg/apis/skupper/v1alpha1"
	"gotest.tools/assert"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestCmdSiteStatus_ValidateInput(t *testing.T) {
	type test struct {
		name           string
		args           []string
		k8sObjects     []runtime.Object
		skupperObjects []runtime.Object
		skupperError   string
		expectedErrors []string
	}

	testTable := []test{
		{
			name:           "more than one argument was specified",
			args:           []string{"my-site", ""},
			k8sObjects:     nil,
			skupperObjects: nil,
			skupperError:   "",
			expectedErrors: []string{"this command does not need any arguments"},
		},
	}

	for _, test := range testTable {
		t.Run(test.name, func(t *testing.T) {
			command := &CmdSiteStatus{
				Namespace: "test",
			}

			fakeSkupperClient, err := fakeclient.NewFakeClient(command.Namespace, test.k8sObjects, test.skupperObjects, test.skupperError)
			assert.Assert(t, err)
			command.Client = fakeSkupperClient.GetSkupperClient().SkupperV1alpha1()

			actualErrors := command.ValidateInput(test.args)

			actualErrorsMessages := utils.ErrorsToMessages(actualErrors)

			assert.DeepEqual(t, actualErrorsMessages, test.expectedErrors)

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
	}

	testTable := []test{
		{
			name:       "runs ok",
			k8sObjects: nil,
			skupperObjects: []runtime.Object{
				&v1alpha1.Site{
					ObjectMeta: v1.ObjectMeta{
						Name:      "old-site",
						Namespace: "test",
					},
					Status: v1alpha1.SiteStatus{
						Status: v1alpha1.Status{
							StatusMessage: "OK",
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
	}

	for _, test := range testTable {
		command := &CmdSiteStatus{
			Namespace: "test",
		}

		fakeSkupperClient, err := fakeclient.NewFakeClient(command.Namespace, test.k8sObjects, test.skupperObjects, test.skupperError)
		assert.Assert(t, err)
		command.Client = fakeSkupperClient.GetSkupperClient().SkupperV1alpha1()

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
		command.Client = fakeSkupperClient.GetSkupperClient().SkupperV1alpha1()

		result := command.WaitUntil()
		assert.Check(t, result == nil)

	})

}

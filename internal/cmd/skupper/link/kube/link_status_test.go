package kube

import (
	"github.com/skupperproject/skupper/internal/cmd/skupper/utils"
	fakeclient "github.com/skupperproject/skupper/internal/kube/client/fake"
	"github.com/skupperproject/skupper/pkg/apis/skupper/v1alpha1"
	"gotest.tools/assert"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"testing"
)

func TestCmdLinkStatus_NewCmdLinkStatus(t *testing.T) {

	t.Run("link status command", func(t *testing.T) {

		result := NewCmdLinkStatus()

		assert.Check(t, result.CobraCmd.Use != "")
		assert.Check(t, result.CobraCmd.Short != "")
		assert.Check(t, result.CobraCmd.Long != "")
		assert.Check(t, result.CobraCmd.PreRun != nil)
		assert.Check(t, result.CobraCmd.Run != nil)
		assert.Check(t, result.CobraCmd.Flags() != nil)

	})

}

func TestCmdLinkStatus_ValidateInput(t *testing.T) {
	type test struct {
		name                string
		args                []string
		flags               CmdLinkStatusFlags
		k8sObjects          []runtime.Object
		skupperObjects      []runtime.Object
		skupperErrorMessage string
		expectedErrors      []string
	}

	testTable := []test{
		{
			name: "more than one argument was specified",
			args: []string{"my-link", ""},
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
			expectedErrors: []string{"this command only accepts one argument"},
		},
		{
			name:           "there are no sites",
			args:           []string{},
			expectedErrors: []string{"there is no skupper site available"},
		},
		{
			name:  "output format is not valid",
			args:  []string{"my-link"},
			flags: CmdLinkStatusFlags{output: "not-valid"},
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
	}

	for _, test := range testTable {
		t.Run(test.name, func(t *testing.T) {

			command, err := newCmdLinkStatusWithMocks("test", test.k8sObjects, test.skupperObjects, test.skupperErrorMessage)
			assert.Assert(t, err)

			command.flags = test.flags

			actualErrors := command.ValidateInput(test.args)

			actualErrorsMessages := utils.ErrorsToMessages(actualErrors)

			assert.DeepEqual(t, actualErrorsMessages, test.expectedErrors)

		})
	}
}

func TestCmdLinkStatus_InputToOptions(t *testing.T) {

	t.Run("input to options", func(t *testing.T) {

		cmd, err := newCmdLinkStatusWithMocks("test", nil, nil, "")
		assert.Assert(t, err)

		cmd.flags.output = "json"

		cmd.InputToOptions()

		assert.Check(t, cmd.output == "json")

	})

}

func TestCmdLinkStatus_Run(t *testing.T) {
	type test struct {
		name                string
		skupperObjects      []runtime.Object
		skupperErrorMessage string
		errorMessage        string
		linkName            string
		output              string
	}

	testTable := []test{
		{
			name: "runs ok showing all the links",
			skupperObjects: []runtime.Object{
				&v1alpha1.Link{
					ObjectMeta: v1.ObjectMeta{
						Name:      "my-link",
						Namespace: "test",
					},
				},
				&v1alpha1.Link{
					ObjectMeta: v1.ObjectMeta{
						Name:      "link2",
						Namespace: "test",
					},
				},
			},
		},
		{
			name: "runs ok showing all the links in yaml format",
			skupperObjects: []runtime.Object{
				&v1alpha1.Link{
					ObjectMeta: v1.ObjectMeta{
						Name:      "my-link",
						Namespace: "test",
					},
				},
				&v1alpha1.Link{
					ObjectMeta: v1.ObjectMeta{
						Name:      "link2",
						Namespace: "test",
					},
				},
			},
			output: "yaml",
		},
		{
			name: "runs ok showing one of the links",
			skupperObjects: []runtime.Object{
				&v1alpha1.Link{
					ObjectMeta: v1.ObjectMeta{
						Name:      "my-link",
						Namespace: "test",
					},
				},
				&v1alpha1.Link{
					ObjectMeta: v1.ObjectMeta{
						Name:      "link2",
						Namespace: "test",
					},
				},
			},
			linkName: "link2",
		},
		{
			name: "runs ok showing one of the links in json format",
			skupperObjects: []runtime.Object{
				&v1alpha1.Link{
					ObjectMeta: v1.ObjectMeta{
						Name:      "my-link",
						Namespace: "test",
					},
				},
				&v1alpha1.Link{
					ObjectMeta: v1.ObjectMeta{
						Name:      "link2",
						Namespace: "test",
					},
				},
			},
			linkName: "link2",
			output:   "json",
		},
		{
			name:                "run fails",
			skupperErrorMessage: "error",
			errorMessage:        "error",
		},
		{
			name: "runs ok but there are no links",
		},
		{
			name: "there is no link with the name specified as an argument",
			skupperObjects: []runtime.Object{
				&v1alpha1.Link{
					ObjectMeta: v1.ObjectMeta{
						Name:      "my-link",
						Namespace: "test",
					},
				},
				&v1alpha1.Link{
					ObjectMeta: v1.ObjectMeta{
						Name:      "link2",
						Namespace: "test",
					},
				},
			},
			linkName:     "link3",
			errorMessage: "links.skupper.io \"link3\" not found",
		},
		{
			name: "fails showing all the links in yaml format",
			skupperObjects: []runtime.Object{
				&v1alpha1.Link{
					ObjectMeta: v1.ObjectMeta{
						Name:      "my-link",
						Namespace: "test",
					},
				},
				&v1alpha1.Link{
					ObjectMeta: v1.ObjectMeta{
						Name:      "link2",
						Namespace: "test",
					},
				},
			},
			output:       "unsupported",
			errorMessage: "format unsupported not supported",
		},
	}

	for _, test := range testTable {
		cmd, err := newCmdLinkStatusWithMocks("test", nil, test.skupperObjects, test.skupperErrorMessage)
		assert.Assert(t, err)
		cmd.linkName = test.linkName
		cmd.output = test.output

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

func TestCmdLinkStatus_WaitUntilReady(t *testing.T) {

	t.Run("wait until ready", func(t *testing.T) {

		cmd, err := newCmdLinkStatusWithMocks("test", nil, nil, "")
		assert.Assert(t, err)

		result := cmd.WaitUntil()
		assert.Check(t, result == nil)

	})

}

// --- helper methods

func newCmdLinkStatusWithMocks(namespace string, k8sObjects []runtime.Object, skupperObjects []runtime.Object, fakeSkupperError string) (*CmdLinkStatus, error) {

	client, err := fakeclient.NewFakeClient(namespace, k8sObjects, skupperObjects, fakeSkupperError)
	if err != nil {
		return nil, err
	}
	cmdLinkStatus := &CmdLinkStatus{
		Client:    client.GetSkupperClient().SkupperV1alpha1(),
		Namespace: namespace,
	}

	return cmdLinkStatus, nil
}

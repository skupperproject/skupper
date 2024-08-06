package kube

import (
	"fmt"
	"github.com/skupperproject/skupper/internal/cmd/skupper/utils"
	fakeclient "github.com/skupperproject/skupper/internal/kube/client/fake"
	"github.com/skupperproject/skupper/pkg/apis/skupper/v1alpha1"
	"github.com/spf13/pflag"
	"gotest.tools/assert"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"testing"
)

func TestCmdLinkDelete_NewCmdLinkDelete(t *testing.T) {

	t.Run("link delete command", func(t *testing.T) {

		result := NewCmdLinkDelete()

		assert.Check(t, result.CobraCmd.Use != "")
		assert.Check(t, result.CobraCmd.Short != "")
		assert.Check(t, result.CobraCmd.Long != "")
		assert.Check(t, result.CobraCmd.PreRun != nil)
		assert.Check(t, result.CobraCmd.Run != nil)
		assert.Check(t, result.CobraCmd.PostRunE != nil)
		assert.Check(t, result.CobraCmd.Flags() != nil)

	})

}

func TestCmdLinkDelete_AddFlags(t *testing.T) {

	expectedFlagsWithDefaultValue := map[string]interface{}{
		"timeout": "60",
	}
	var flagList []string

	cmd, err := newCmdLinkDeleteWithMocks("test", nil, nil, "")
	assert.Assert(t, err)

	t.Run("add flags", func(t *testing.T) {

		cmd.CobraCmd.Flags().VisitAll(func(flag *pflag.Flag) {
			flagList = append(flagList, flag.Name)
		})

		assert.Check(t, len(flagList) == 0)

		cmd.AddFlags()

		cmd.CobraCmd.Flags().VisitAll(func(flag *pflag.Flag) {
			flagList = append(flagList, flag.Name)
			assert.Check(t, expectedFlagsWithDefaultValue[flag.Name] != nil, fmt.Sprintf("flag %q not expected", flag.Name))
			assert.Check(t, expectedFlagsWithDefaultValue[flag.Name] == flag.DefValue, fmt.Sprintf("flag %q witn not expected default value %q", flag.Name, flag.DefValue))
		})

		assert.Check(t, len(flagList) == len(expectedFlagsWithDefaultValue))

	})

}

func TestCmdLinkDelete_ValidateInput(t *testing.T) {
	type test struct {
		name           string
		args           []string
		flags          DeleteLinkFlags
		k8sObjects     []runtime.Object
		skupperObjects []runtime.Object
		expectedErrors []string
	}

	testTable := []test{
		{
			name:           "there is no active skupper site in this namespace",
			args:           []string{"my-link"},
			flags:          DeleteLinkFlags{timeout: "60"},
			expectedErrors: []string{"there is no skupper site in this namespace", "the link \"my-link\" is not available in the namespace"},
		},
		{
			name:  "link is not deleted because it does not exist",
			args:  []string{"my-link"},
			flags: DeleteLinkFlags{timeout: "60"},
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
			flags: DeleteLinkFlags{timeout: "60"},
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
			flags: DeleteLinkFlags{timeout: "60"},
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
			flags: DeleteLinkFlags{timeout: "60"},
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
			flags: DeleteLinkFlags{timeout: "-1"},
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
			flags: DeleteLinkFlags{timeout: "0"},
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
			flags: DeleteLinkFlags{timeout: "one"},
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
			command.flags = test.flags

			actualErrors := command.ValidateInput(test.args)

			actualErrorsMessages := utils.ErrorsToMessages(actualErrors)

			assert.DeepEqual(t, actualErrorsMessages, test.expectedErrors)

		})
	}
}

func TestCmdLinkDelete_InputToOptions(t *testing.T) {

	type test struct {
		name            string
		flags           DeleteLinkFlags
		expectedTimeout int
	}

	testTable := []test{
		{
			name:            "check options",
			flags:           DeleteLinkFlags{"60"},
			expectedTimeout: 60,
		},
	}

	for _, test := range testTable {
		t.Run(test.name, func(t *testing.T) {

			cmd, err := newCmdLinkDeleteWithMocks("test", nil, nil, "")
			assert.Assert(t, err)
			cmd.flags = test.flags

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
			timeout:     3,
			expectError: true,
		},
		{
			name:        "link is deleted",
			timeout:     3,
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

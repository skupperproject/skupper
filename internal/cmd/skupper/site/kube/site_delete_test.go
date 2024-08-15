package kube

import (
	"testing"

	fakeclient "github.com/skupperproject/skupper/internal/kube/client/fake"

	"github.com/skupperproject/skupper/internal/cmd/skupper/utils"
	"github.com/skupperproject/skupper/pkg/apis/skupper/v1alpha1"
	"gotest.tools/assert"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestCmdSiteDelete_NewCmdSiteDelete(t *testing.T) {

	t.Run("delete command", func(t *testing.T) {

		result := NewCmdSiteDelete()

		assert.Check(t, result.CobraCmd.Use != "")
		assert.Check(t, result.CobraCmd.Short != "")
		assert.Check(t, result.CobraCmd.Long != "")
		assert.Check(t, result.CobraCmd.PreRun != nil)
		assert.Check(t, result.CobraCmd.Run != nil)
		assert.Check(t, result.CobraCmd.PostRunE != nil)
		assert.Check(t, result.CobraCmd.Flags() != nil)

	})

}

func TestCmdSiteDelete_ValidateInput(t *testing.T) {
	type test struct {
		name           string
		args           []string
		k8sObjects     []runtime.Object
		skupperObjects []runtime.Object
		flags          *CreateFlags
		expectedErrors []string
		skupperError   string
	}

	testTable := []test{
		{
			name:           "site is not deleted because it does not exist",
			args:           []string{"my-site"},
			k8sObjects:     nil,
			skupperObjects: nil,
			expectedErrors: []string{"there is no existing Skupper site resource to update"},
			skupperError:   "",
		},
		{
			name:           "site is not deleted because there is an error trying to retrieve it",
			args:           []string{"my-site"},
			k8sObjects:     nil,
			skupperObjects: nil,
			expectedErrors: []string{"error getting the site"},
			skupperError:   "error getting the site",
		},
		{
			name:       "more than one argument was specified",
			args:       []string{"my", "site"},
			k8sObjects: nil,
			skupperObjects: []runtime.Object{
				&v1alpha1.Site{
					ObjectMeta: v1.ObjectMeta{
						Name:      "my-site",
						Namespace: "test",
					},
					Status: v1alpha1.SiteStatus{
						Status: v1alpha1.Status{
							StatusMessage: "OK",
						},
					},
				},
			},
			expectedErrors: []string{"only one argument is allowed for this command"},
			skupperError:   "",
		},
		{
			name:       "there are several skupper sites and no site name was specified",
			args:       []string{},
			k8sObjects: nil,
			skupperObjects: []runtime.Object{
				&v1alpha1.Site{
					ObjectMeta: v1.ObjectMeta{
						Name:      "my-site",
						Namespace: "test",
					},
					Status: v1alpha1.SiteStatus{
						Status: v1alpha1.Status{
							StatusMessage: "OK",
						},
					},
				},
				&v1alpha1.Site{
					ObjectMeta: v1.ObjectMeta{
						Name:      "another-site",
						Namespace: "test",
					},
					Status: v1alpha1.SiteStatus{
						Status: v1alpha1.Status{
							StatusMessage: "OK",
						},
					},
				},
			},
			expectedErrors: []string{"site name is required because there are several sites in this namespace"},
			skupperError:   "",
		},
		{
			name:       "there are several skupper sites but not the one specified by the user",
			args:       []string{"special-site"},
			k8sObjects: nil,
			skupperObjects: []runtime.Object{
				&v1alpha1.Site{
					ObjectMeta: v1.ObjectMeta{
						Name:      "my-site",
						Namespace: "test",
					},
					Status: v1alpha1.SiteStatus{
						Status: v1alpha1.Status{
							StatusMessage: "OK",
						},
					},
				},
				&v1alpha1.Site{
					ObjectMeta: v1.ObjectMeta{
						Name:      "another-site",
						Namespace: "test",
					},
					Status: v1alpha1.SiteStatus{
						Status: v1alpha1.Status{
							StatusMessage: "OK",
						},
					},
				},
			},
			expectedErrors: []string{"site with name \"special-site\" is not available"},
			skupperError:   "",
		},
		{
			name:       "there are several skupper sites and the user specifies one of them",
			args:       []string{"my-site"},
			k8sObjects: nil,
			skupperObjects: []runtime.Object{
				&v1alpha1.Site{
					ObjectMeta: v1.ObjectMeta{
						Name:      "my-site",
						Namespace: "test",
					},
					Status: v1alpha1.SiteStatus{
						Status: v1alpha1.Status{
							StatusMessage: "OK",
						},
					},
				},
				&v1alpha1.Site{
					ObjectMeta: v1.ObjectMeta{
						Name:      "another-site",
						Namespace: "test",
					},
					Status: v1alpha1.SiteStatus{
						Status: v1alpha1.Status{
							StatusMessage: "OK",
						},
					},
				},
			},
			expectedErrors: []string{},
			skupperError:   "",
		},
		{
			name:       "trying to delete a site that does not exist",
			args:       []string{"siteb"},
			k8sObjects: nil,
			skupperObjects: []runtime.Object{
				&v1alpha1.Site{
					ObjectMeta: v1.ObjectMeta{
						Name:      "my-site",
						Namespace: "test",
					},
					Status: v1alpha1.SiteStatus{
						Status: v1alpha1.Status{
							StatusMessage: "OK",
						},
					},
				},
			},
			expectedErrors: []string{"site with name \"siteb\" is not available"},
			skupperError:   "",
		},
		{
			name:       "deleting the site successfully",
			args:       []string{},
			k8sObjects: nil,
			skupperObjects: []runtime.Object{
				&v1alpha1.Site{
					ObjectMeta: v1.ObjectMeta{
						Name:      "my-site",
						Namespace: "test",
					},
					Status: v1alpha1.SiteStatus{
						Status: v1alpha1.Status{
							StatusMessage: "OK",
						},
					},
				},
			},
			expectedErrors: []string{},
			skupperError:   "",
		},
	}

	for _, test := range testTable {
		t.Run(test.name, func(t *testing.T) {

			command := &CmdSiteDelete{
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

func TestCmdSiteDelete_Run(t *testing.T) {
	type test struct {
		name           string
		k8sObjects     []runtime.Object
		skupperObjects []runtime.Object
		skupperError   string
		siteName       string
		errorMessage   string
	}

	testTable := []test{
		{
			name:       "runs ok",
			k8sObjects: nil,
			skupperObjects: []runtime.Object{
				&v1alpha1.Site{
					ObjectMeta: v1.ObjectMeta{
						Name:      "my-site",
						Namespace: "test",
					},
					Status: v1alpha1.SiteStatus{
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
			skupperError: "",
			siteName:     "my-site",
		},
		{
			name:           "runs fails",
			k8sObjects:     nil,
			skupperObjects: nil,
			skupperError:   "",
			siteName:       "my-site",
			errorMessage:   "sites.skupper.io \"my-site\" not found",
		},
	}

	for _, test := range testTable {
		command := &CmdSiteDelete{
			Namespace: "test",
		}

		fakeSkupperClient, err := fakeclient.NewFakeClient(command.Namespace, test.k8sObjects, test.skupperObjects, test.skupperError)
		assert.Assert(t, err)
		command.Client = fakeSkupperClient.GetSkupperClient().SkupperV1alpha1()

		command.siteName = test.siteName

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

func TestCmdSiteDelete_WaitUntil(t *testing.T) {
	type test struct {
		name           string
		k8sObjects     []runtime.Object
		skupperObjects []runtime.Object
		skupperError   string
		expectError    bool
	}

	testTable := []test{
		{
			name:       "site is not deleted",
			k8sObjects: nil,
			skupperObjects: []runtime.Object{
				&v1alpha1.Site{
					ObjectMeta: v1.ObjectMeta{
						Name:      "my-site",
						Namespace: "test",
					},
					Status: v1alpha1.SiteStatus{
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
			skupperError: "",
			expectError:  true,
		},
		{
			name:           "site is deleted",
			k8sObjects:     nil,
			skupperObjects: nil,
			skupperError:   "no site",
			expectError:    false,
		},
	}

	for _, test := range testTable {
		test := test
		command := &CmdSiteDelete{
			Namespace: "test",
		}

		fakeSkupperClient, err := fakeclient.NewFakeClient(command.Namespace, test.k8sObjects, test.skupperObjects, test.skupperError)
		assert.Assert(t, err)
		command.Client = fakeSkupperClient.GetSkupperClient().SkupperV1alpha1()
		command.siteName = "my-site"
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			err := command.WaitUntil()
			if err != nil {
				assert.Check(t, test.expectError)
			}

		})
	}
}

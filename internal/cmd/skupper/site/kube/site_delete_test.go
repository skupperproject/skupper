package kube

import (
	"github.com/skupperproject/skupper/internal/cmd/skupper/common"
	"github.com/skupperproject/skupper/internal/cmd/skupper/common/utils"
	"testing"
	"time"

	fakeclient "github.com/skupperproject/skupper/internal/kube/client/fake"

	"github.com/skupperproject/skupper/pkg/apis/skupper/v2alpha1"
	"gotest.tools/assert"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestCmdSiteDelete_ValidateInput(t *testing.T) {
	type test struct {
		name           string
		args           []string
		k8sObjects     []runtime.Object
		skupperObjects []runtime.Object
		expectedErrors []string
		skupperError   string
		flags          *common.CommandSiteDeleteFlags
	}

	testTable := []test{
		{
			name:           "site is not deleted because it does not exist",
			args:           []string{"my-site"},
			k8sObjects:     nil,
			skupperObjects: nil,
			expectedErrors: []string{"there is no existing Skupper site resource to delete"},
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
				&v2alpha1.Site{
					ObjectMeta: v1.ObjectMeta{
						Name:      "my-site",
						Namespace: "test",
					},
					Status: v2alpha1.SiteStatus{
						Status: v2alpha1.Status{
							Message: "OK",
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
				&v2alpha1.Site{
					ObjectMeta: v1.ObjectMeta{
						Name:      "my-site",
						Namespace: "test",
					},
					Status: v2alpha1.SiteStatus{
						Status: v2alpha1.Status{
							Message: "OK",
						},
					},
				},
				&v2alpha1.Site{
					ObjectMeta: v1.ObjectMeta{
						Name:      "another-site",
						Namespace: "test",
					},
					Status: v2alpha1.SiteStatus{
						Status: v2alpha1.Status{
							Message: "OK",
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
				&v2alpha1.Site{
					ObjectMeta: v1.ObjectMeta{
						Name:      "my-site",
						Namespace: "test",
					},
					Status: v2alpha1.SiteStatus{
						Status: v2alpha1.Status{
							Message: "OK",
						},
					},
				},
				&v2alpha1.Site{
					ObjectMeta: v1.ObjectMeta{
						Name:      "another-site",
						Namespace: "test",
					},
					Status: v2alpha1.SiteStatus{
						Status: v2alpha1.Status{
							Message: "OK",
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
				&v2alpha1.Site{
					ObjectMeta: v1.ObjectMeta{
						Name:      "my-site",
						Namespace: "test",
					},
					Status: v2alpha1.SiteStatus{
						Status: v2alpha1.Status{
							Message: "OK",
						},
					},
				},
				&v2alpha1.Site{
					ObjectMeta: v1.ObjectMeta{
						Name:      "another-site",
						Namespace: "test",
					},
					Status: v2alpha1.SiteStatus{
						Status: v2alpha1.Status{
							Message: "OK",
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
				&v2alpha1.Site{
					ObjectMeta: v1.ObjectMeta{
						Name:      "my-site",
						Namespace: "test",
					},
					Status: v2alpha1.SiteStatus{
						Status: v2alpha1.Status{
							Message: "OK",
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
				&v2alpha1.Site{
					ObjectMeta: v1.ObjectMeta{
						Name:      "my-site",
						Namespace: "test",
					},
					Status: v2alpha1.SiteStatus{
						Status: v2alpha1.Status{
							Message: "OK",
						},
					},
				},
			},
			expectedErrors: []string{},
			skupperError:   "",
		},
		{
			name:       "timeout is not valid",
			args:       []string{"my-site"},
			flags:      &common.CommandSiteDeleteFlags{Timeout: time.Second * 0},
			k8sObjects: nil,
			skupperObjects: []runtime.Object{
				&v2alpha1.Site{
					ObjectMeta: v1.ObjectMeta{
						Name:      "my-site",
						Namespace: "test",
					},
					Status: v2alpha1.SiteStatus{
						Status: v2alpha1.Status{
							Message: "OK",
						},
					},
				},
			},
			expectedErrors: []string{"timeout is not valid: duration must not be less than 10s; got 0s"},
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

			command.Client = fakeSkupperClient.GetSkupperClient().SkupperV2alpha1()

			if test.flags != nil {
				command.Flags = test.flags
			}

			actualErrors := command.ValidateInput(test.args)

			actualErrorsMessages := utils.ErrorsToMessages(actualErrors)

			assert.DeepEqual(t, actualErrorsMessages, test.expectedErrors)

		})
	}
}

func TestCmdSiteDelete_InputToOptions(t *testing.T) {

	type test struct {
		name            string
		flags           *common.CommandSiteDeleteFlags
		expectedTimeout time.Duration
		expectedWait    bool
	}

	testTable := []test{
		{
			name:            "options with timeout",
			flags:           &common.CommandSiteDeleteFlags{Timeout: time.Second * 30},
			expectedTimeout: time.Second * 30,
		},
		{
			name:         "wait for site enabled",
			flags:        &common.CommandSiteDeleteFlags{Wait: true},
			expectedWait: true,
		},
	}

	for _, test := range testTable {
		t.Run(test.name, func(t *testing.T) {
			command := &CmdSiteDelete{
				Namespace: "test",
			}
			command.Flags = test.flags
			command.InputToOptions()

			assert.Check(t, command.timeout == test.expectedTimeout)
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
				&v2alpha1.Site{
					ObjectMeta: v1.ObjectMeta{
						Name:      "my-site",
						Namespace: "test",
					},
					Status: v2alpha1.SiteStatus{
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
		command.Client = fakeSkupperClient.GetSkupperClient().SkupperV2alpha1()

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
		wait           bool
		k8sObjects     []runtime.Object
		skupperObjects []runtime.Object
		skupperError   string
		expectError    bool
	}

	testTable := []test{
		{
			name:       "site is not deleted",
			k8sObjects: nil,
			wait:       true,
			skupperObjects: []runtime.Object{
				&v2alpha1.Site{
					ObjectMeta: v1.ObjectMeta{
						Name:      "my-site",
						Namespace: "test",
					},
					Status: v2alpha1.SiteStatus{
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
			skupperError: "",
			expectError:  true,
		},
		{
			name:           "site is deleted",
			wait:           true,
			k8sObjects:     nil,
			skupperObjects: nil,
			skupperError:   "no site",
			expectError:    false,
		},
		{
			name:       "site is not deleted but user does not want to wait",
			k8sObjects: nil,
			wait:       false,
			skupperObjects: []runtime.Object{
				&v2alpha1.Site{
					ObjectMeta: v1.ObjectMeta{
						Name:      "my-site",
						Namespace: "test",
					},
					Status: v2alpha1.SiteStatus{
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
			skupperError: "",
			expectError:  false,
		},
	}

	for _, test := range testTable {
		command := &CmdSiteDelete{
			Namespace: "test",
		}

		utils.SetRetryProfile(utils.TestRetryProfile)
		fakeSkupperClient, err := fakeclient.NewFakeClient(command.Namespace, test.k8sObjects, test.skupperObjects, test.skupperError)
		assert.Assert(t, err)
		command.Client = fakeSkupperClient.GetSkupperClient().SkupperV2alpha1()
		command.siteName = "my-site"
		command.timeout = time.Second
		command.wait = test.wait

		t.Run(test.name, func(t *testing.T) {

			err := command.WaitUntil()
			if err != nil {
				assert.Check(t, test.expectError)
			}

		})
	}
}

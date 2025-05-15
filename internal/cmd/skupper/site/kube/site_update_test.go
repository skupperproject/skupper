package kube

import (
	"fmt"
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

func TestCmdSiteUpdate_ValidateInput(t *testing.T) {
	type test struct {
		name           string
		args           []string
		flags          *common.CommandSiteUpdateFlags
		k8sObjects     []runtime.Object
		skupperObjects []runtime.Object
		skupperError   string
		expectedError  string
	}

	testTable := []test{
		{
			name:          "missing CRD",
			args:          []string{"my-site"},
			skupperError:  utils.CrdErr,
			expectedError: utils.CrdHelpErr,
		},
		{
			name:       "site is updated because there is already a site in the namespace.",
			args:       []string{"my-site"},
			flags:      &common.CommandSiteUpdateFlags{Timeout: time.Minute},
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
			skupperError:  "",
			expectedError: "",
		},
		{
			name:       "site name is not specified.",
			args:       []string{},
			flags:      &common.CommandSiteUpdateFlags{Timeout: time.Minute},
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
			skupperError:  "",
			expectedError: "",
		},
		{
			name:       "more than one argument was specified",
			args:       []string{"my", "site"},
			flags:      &common.CommandSiteUpdateFlags{Timeout: time.Minute},
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
			skupperError:  "",
			expectedError: "only one argument is allowed for this command",
		},
		{
			name:       "link access type is not valid",
			args:       []string{"my-site"},
			flags:      &common.CommandSiteUpdateFlags{LinkAccessType: "not-valid", Timeout: time.Minute},
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
			skupperError: "",
			expectedError: "link access type is not valid: value not-valid not allowed. It should be one of this options: [route loadbalancer default]\n" +
				"for the site to work with this type of linkAccess, the --enable-link-access option must be set to true",
		},
		{
			name:           "there is no skupper site",
			args:           []string{"my-site"},
			flags:          &common.CommandSiteUpdateFlags{Timeout: time.Minute},
			k8sObjects:     nil,
			skupperObjects: nil,
			skupperError:   "",
			expectedError:  "there is no existing Skupper site resource to update",
		},
		{
			name:       "there are several skupper sites and no site name was specified",
			args:       []string{},
			flags:      &common.CommandSiteUpdateFlags{Timeout: time.Minute},
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
			skupperError:  "",
			expectedError: "site name is required because there are several sites in this namespace",
		},
		{
			name:       "there are several skupper sites but not the one specified by the user",
			args:       []string{"special-site"},
			flags:      &common.CommandSiteUpdateFlags{Timeout: time.Minute},
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
			skupperError:  "",
			expectedError: "site with name \"special-site\" is not available",
		},
		{
			name:       "there are several skupper sites and the user specifies one of them",
			args:       []string{"my-site"},
			flags:      &common.CommandSiteUpdateFlags{Timeout: time.Minute},
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
			skupperError:  "",
			expectedError: "",
		},
		{
			name:       "the name specified in the arguments does not match with the current site",
			args:       []string{"a-site"},
			flags:      &common.CommandSiteUpdateFlags{Timeout: time.Minute},
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
			skupperError:  "",
			expectedError: "site with name \"a-site\" is not available",
		},
		{
			name:       "timeout format is not valid",
			args:       []string{"my-site"},
			flags:      &common.CommandSiteUpdateFlags{Timeout: time.Second * 0},
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
			expectedError: "timeout is not valid: duration must not be less than 10s; got 0s",
		},
		{
			name:       "wait status is not valid",
			args:       []string{"my-site"},
			flags:      &common.CommandSiteUpdateFlags{Timeout: time.Second * 30, Wait: "created"},
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
			expectedError: "status is not valid: value created not allowed. It should be one of this options: [ready configured none]",
		},
	}

	for _, test := range testTable {
		command := &CmdSiteUpdate{
			Namespace: "test",
		}

		t.Run(test.name, func(t *testing.T) {

			fakeSkupperClient, err := fakeclient.NewFakeClient(command.Namespace, test.k8sObjects, test.skupperObjects, test.skupperError)
			assert.Assert(t, err)
			command.Client = fakeSkupperClient.GetSkupperClient().SkupperV2alpha1()
			command.KubeClient = fakeSkupperClient.GetKubeClient()

			if test.flags != nil {
				command.Flags = test.flags
			}

			testutils.CheckValidateInput(t, command, test.expectedError, test.args)
		})
	}
}

func TestCmdSiteUpdate_InputToOptions(t *testing.T) {

	type test struct {
		name               string
		args               []string
		flags              common.CommandSiteUpdateFlags
		expectedLinkAccess string
		expectedHA         bool
		expectedTimeout    time.Duration
		expectedStatus     string
	}

	testTable := []test{
		{
			name:               "options without link access enabled",
			args:               []string{"my-site"},
			flags:              common.CommandSiteUpdateFlags{},
			expectedLinkAccess: "",
		},
		{
			name:               "options with link access enabled but using a type by default and link access host specified",
			args:               []string{"my-site"},
			flags:              common.CommandSiteUpdateFlags{EnableLinkAccess: true},
			expectedLinkAccess: "default",
		},
		{
			name:               "options with link access enabled using the nodeport type",
			args:               []string{"my-site"},
			flags:              common.CommandSiteUpdateFlags{EnableLinkAccess: true, LinkAccessType: "nodeport"},
			expectedLinkAccess: "nodeport",
		},
		{
			name:               "options with link access options not well specified",
			args:               []string{"my-site"},
			flags:              common.CommandSiteUpdateFlags{EnableLinkAccess: false, LinkAccessType: "nodeport"},
			expectedLinkAccess: "",
		},
		{
			name:               "options with loadbalancer and timeout",
			args:               []string{"my-site"},
			flags:              common.CommandSiteUpdateFlags{EnableLinkAccess: true, LinkAccessType: "loadbalancer", Timeout: time.Minute},
			expectedLinkAccess: "loadbalancer",
			expectedTimeout:    time.Minute,
		},
		{
			name:           "options with waiting status",
			args:           []string{"my-site"},
			flags:          common.CommandSiteUpdateFlags{Wait: "configured"},
			expectedStatus: "configured",
		},
		{
			name:       "options with EnableHA enabled",
			args:       []string{"my-site"},
			flags:      common.CommandSiteUpdateFlags{EnableHA: true},
			expectedHA: true,
		},
	}

	for _, test := range testTable {
		t.Run(test.name, func(t *testing.T) {
			command := &CmdSiteUpdate{
				Namespace: "test",
			}

			fakeSkupperClient, err := fakeclient.NewFakeClient(command.Namespace, nil, nil, "")
			assert.Assert(t, err)
			command.Client = fakeSkupperClient.GetSkupperClient().SkupperV2alpha1()
			command.Flags = &test.flags
			command.siteName = "my-site"

			command.InputToOptions()

			assert.Check(t, command.status == test.expectedStatus)
			assert.Check(t, command.linkAccessType == test.expectedLinkAccess)
			assert.Check(t, command.timeout == test.expectedTimeout)
			assert.Check(t, command.HA == test.expectedHA)
		})
	}
}

func TestCmdSiteUpdate_Run(t *testing.T) {
	type test struct {
		name           string
		k8sObjects     []runtime.Object
		skupperObjects []runtime.Object
		skupperError   string
		siteName       string
		linkAccessType string
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
				},
			},
			siteName:       "my-site",
			linkAccessType: "default",
			skupperError:   "",
			errorMessage:   "",
		},
		{
			name:           "run fails",
			k8sObjects:     nil,
			skupperObjects: nil,
			siteName:       "my-site",
			linkAccessType: "default",
			skupperError:   "error",
			errorMessage:   "error",
		},
		{
			name:           "runs ok without creating site",
			k8sObjects:     nil,
			skupperObjects: nil,
			siteName:       "my-site",
			linkAccessType: "default",
			skupperError:   "",
			errorMessage:   "sites.skupper.io \"my-site\" not found",
		},
	}

	for _, test := range testTable {
		command := &CmdSiteUpdate{
			Namespace: "test",
		}

		fakeSkupperClient, err := fakeclient.NewFakeClient(command.Namespace, test.k8sObjects, test.skupperObjects, test.skupperError)
		assert.Assert(t, err)
		command.Client = fakeSkupperClient.GetSkupperClient().SkupperV2alpha1()
		command.siteName = test.siteName
		command.linkAccessType = test.linkAccessType

		t.Run(test.name, func(t *testing.T) {

			err := command.Run()
			if err != nil {
				fmt.Println("error", err.Error())
				assert.Check(t, test.errorMessage == err.Error())
			} else {
				assert.Check(t, err == nil)
			}
		})
	}
}

func TestCmdSiteUpdate_WaitUntil(t *testing.T) {
	type test struct {
		name           string
		status         string
		k8sObjects     []runtime.Object
		skupperObjects []runtime.Object
		siteName       string
		skupperError   string
		errorMessage   string
		expectError    bool
	}

	testTable := []test{
		{
			name:       "site is not ready",
			status:     "ready",
			k8sObjects: nil,
			skupperObjects: []runtime.Object{
				&v2alpha1.Site{
					ObjectMeta: v1.ObjectMeta{
						Name:      "my-site",
						Namespace: "test",
					},
				},
			},
			siteName:     "my-site",
			skupperError: "",
			errorMessage: "Site \"my-site\" is not yet ready, check the status for more information\n",
			expectError:  true,
		},
		{
			name:       "site is not ready yet, but user waits for configured",
			status:     "configured",
			k8sObjects: nil,
			skupperObjects: []runtime.Object{
				&v2alpha1.Site{
					ObjectMeta: v1.ObjectMeta{
						Name:      "my-site",
						Namespace: "test",
					},
					Status: v2alpha1.SiteStatus{
						Status: v2alpha1.Status{
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
			siteName:     "my-site",
			skupperError: "",
			expectError:  false,
		},
		{
			name:       "user does not wait",
			status:     "none",
			k8sObjects: nil,
			skupperObjects: []runtime.Object{
				&v2alpha1.Site{
					ObjectMeta: v1.ObjectMeta{
						Name:      "my-site",
						Namespace: "test",
					},
					Status: v2alpha1.SiteStatus{
						Status: v2alpha1.Status{
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
		{
			name:       "user waits for configured, but site had some errors while being configured",
			status:     "configured",
			k8sObjects: nil,
			skupperObjects: []runtime.Object{
				&v2alpha1.Site{
					ObjectMeta: v1.ObjectMeta{
						Name:      "my-site",
						Namespace: "test",
					},
					Status: v2alpha1.SiteStatus{
						Status: v2alpha1.Status{
							Conditions: []v1.Condition{
								{
									Message:            "Error",
									ObservedGeneration: 1,
									Reason:             "Error",
									Status:             "False",
									Type:               "Configured",
								},
							},
						},
					},
				},
			},
			siteName:     "my-site",
			skupperError: "",
			expectError:  true,
			errorMessage: "Site \"my-site\" is not yet configured: Error\n",
		},
	}

	for _, test := range testTable {
		command := &CmdSiteUpdate{
			Namespace: "test",
		}

		utils.SetRetryProfile(utils.TestRetryProfile)
		fakeSkupperClient, err := fakeclient.NewFakeClient(command.Namespace, test.k8sObjects, test.skupperObjects, test.skupperError)
		assert.Assert(t, err)
		command.Client = fakeSkupperClient.GetSkupperClient().SkupperV2alpha1()
		command.siteName = test.siteName
		command.timeout = time.Second
		command.status = test.status

		t.Run(test.name, func(t *testing.T) {

			err := command.WaitUntil()
			if err != nil {
				assert.Check(t, test.expectError)
				assert.Equal(t, test.errorMessage, err.Error())
			}

		})
	}
}

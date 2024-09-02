package kube

import (
	"fmt"
	"github.com/skupperproject/skupper/internal/cmd/skupper/common"
	"github.com/skupperproject/skupper/internal/cmd/skupper/common/utils"
	"testing"

	fakeclient "github.com/skupperproject/skupper/internal/kube/client/fake"
	"github.com/skupperproject/skupper/pkg/apis/skupper/v1alpha1"
	"gotest.tools/assert"
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
		expectedErrors []string
	}

	testTable := []test{
		{
			name:       "site is updated because there is already a site in the namespace.",
			args:       []string{"my-site"},
			flags:      &common.CommandSiteUpdateFlags{},
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
			skupperError:   "",
			expectedErrors: []string{},
		},
		{
			name:       "site name is not specified.",
			args:       []string{},
			flags:      &common.CommandSiteUpdateFlags{},
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
			skupperError:   "",
			expectedErrors: []string{},
		},
		{
			name:       "more than one argument was specified",
			args:       []string{"my", "site"},
			flags:      &common.CommandSiteUpdateFlags{},
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
			skupperError:   "",
			expectedErrors: []string{"only one argument is allowed for this command"},
		},
		{
			name:       "service account name is not valid.",
			args:       []string{"my-site"},
			flags:      &common.CommandSiteUpdateFlags{ServiceAccount: "not valid service account name"},
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
			skupperError:   "",
			expectedErrors: []string{"service account name is not valid: serviceaccounts \"not valid service account name\" not found"},
		},
		{
			name:  "host name was specified, but this flag does not work on kube platforms",
			args:  []string{"my-site"},
			flags: &common.CommandSiteUpdateFlags{BindHost: "host"},
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
			expectedErrors: []string{"--host flag is not supported on this platform"},
		},
		{
			name:       "link access type is not valid",
			args:       []string{"my-site"},
			flags:      &common.CommandSiteUpdateFlags{LinkAccessType: "not-valid"},
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
			skupperError: "",
			expectedErrors: []string{
				"link access type is not valid: value not-valid not allowed. It should be one of this options: [route loadbalancer default]",
				"for the site to work with this type of linkAccess, the --enable-link-access option must be set to true",
			},
		},
		{
			name:       "output format is not valid",
			args:       []string{"my-site"},
			flags:      &common.CommandSiteUpdateFlags{Output: "not-valid"},
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
			expectedErrors: []string{
				"output type is not valid: value not-valid not allowed. It should be one of this options: [json yaml]",
			},
		},
		{
			name:           "there is no skupper site",
			args:           []string{"my-site"},
			flags:          &common.CommandSiteUpdateFlags{},
			k8sObjects:     nil,
			skupperObjects: nil,
			skupperError:   "",
			expectedErrors: []string{
				"there is no existing Skupper site resource to update",
			},
		},
		{
			name:       "there are several skupper sites and no site name was specified",
			args:       []string{},
			flags:      &common.CommandSiteUpdateFlags{},
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
			skupperError:   "",
			expectedErrors: []string{"site name is required because there are several sites in this namespace"},
		},
		{
			name:       "there are several skupper sites but not the one specified by the user",
			args:       []string{"special-site"},
			flags:      &common.CommandSiteUpdateFlags{},
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
			skupperError:   "",
			expectedErrors: []string{"site with name \"special-site\" is not available"},
		},
		{
			name:       "there are several skupper sites and the user specifies one of them",
			args:       []string{"my-site"},
			flags:      &common.CommandSiteUpdateFlags{},
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
			skupperError:   "",
			expectedErrors: []string{},
		},
		{
			name:       "the name specified in the arguments does not match with the current site",
			args:       []string{"a-site"},
			flags:      &common.CommandSiteUpdateFlags{},
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
			skupperError: "",
			expectedErrors: []string{
				"site with name \"a-site\" is not available",
			},
		},
		{
			name:       "timeout format is not valid",
			args:       []string{"my-site"},
			flags:      &common.CommandSiteUpdateFlags{Timeout: "2seconds"},
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
			expectedErrors: []string{
				"timeout is not valid: value is not an integer",
			},
		},
	}

	for _, test := range testTable {
		command := &CmdSiteUpdate{
			Namespace: "test",
		}

		t.Run(test.name, func(t *testing.T) {

			fakeSkupperClient, err := fakeclient.NewFakeClient(command.Namespace, test.k8sObjects, test.skupperObjects, test.skupperError)
			assert.Assert(t, err)
			command.Client = fakeSkupperClient.GetSkupperClient().SkupperV1alpha1()
			command.KubeClient = fakeSkupperClient.GetKubeClient()

			if test.flags != nil {
				command.Flags = test.flags
			}

			actualErrors := command.ValidateInput(test.args)

			actualErrorsMessages := utils.ErrorsToMessages(actualErrors)

			assert.DeepEqual(t, actualErrorsMessages, test.expectedErrors)

		})
	}
}

func TestCmdSiteUpdate_InputToOptions(t *testing.T) {

	type test struct {
		name               string
		args               []string
		flags              common.CommandSiteUpdateFlags
		expectedLinkAccess string
		expectedOutput     string
		expectedTimeout    int
	}

	testTable := []test{
		{
			name:               "options without link access enabled",
			args:               []string{"my-site"},
			flags:              common.CommandSiteUpdateFlags{},
			expectedLinkAccess: "none",
			expectedOutput:     "",
		},
		{
			name:               "options with link access enabled but using a type by default and link access host specified",
			args:               []string{"my-site"},
			flags:              common.CommandSiteUpdateFlags{EnableLinkAccess: true},
			expectedLinkAccess: "loadbalancer",
			expectedOutput:     "",
		},
		{
			name:               "options with link access enabled using the nodeport type",
			args:               []string{"my-site"},
			flags:              common.CommandSiteUpdateFlags{EnableLinkAccess: true, LinkAccessType: "nodeport"},
			expectedLinkAccess: "nodeport",
			expectedOutput:     "",
		},
		{
			name:               "options with link access options not well specified",
			args:               []string{"my-site"},
			flags:              common.CommandSiteUpdateFlags{EnableLinkAccess: false, LinkAccessType: "nodeport"},
			expectedLinkAccess: "none",
			expectedOutput:     "",
		},
		{
			name:               "options with output type and timeout",
			args:               []string{"my-site"},
			flags:              common.CommandSiteUpdateFlags{EnableLinkAccess: false, LinkAccessType: "nodeport", Output: "yaml", Timeout: "60"},
			expectedLinkAccess: "none",
			expectedOutput:     "yaml",
			expectedTimeout:    60,
		},
	}

	for _, test := range testTable {
		t.Run(test.name, func(t *testing.T) {
			command := &CmdSiteUpdate{
				Namespace: "test",
			}

			fakeSkupperClient, err := fakeclient.NewFakeClient(command.Namespace, nil, nil, "")
			assert.Assert(t, err)
			command.Client = fakeSkupperClient.GetSkupperClient().SkupperV1alpha1()
			command.Flags = &test.flags
			command.siteName = "my-site"

			command.InputToOptions()

			assert.Check(t, command.output == test.expectedOutput)
		})
	}
}

func TestCmdSiteUpdate_Run(t *testing.T) {
	type test struct {
		name               string
		k8sObjects         []runtime.Object
		skupperObjects     []runtime.Object
		skupperError       string
		siteName           string
		serviceAccountName string
		linkAccessType     string
		output             string
		errorMessage       string
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
							StatusMessage: "",
						},
					},
				},
			},
			siteName:           "my-site",
			serviceAccountName: "my-service-account",
			linkAccessType:     "default",
			output:             "",
			skupperError:       "",
			errorMessage:       "",
		},
		{
			name:               "run fails",
			k8sObjects:         nil,
			skupperObjects:     nil,
			siteName:           "my-site",
			serviceAccountName: "my-service-account",
			linkAccessType:     "default",
			output:             "",
			skupperError:       "error",
			errorMessage:       "error",
		},
		{
			name:               "runs ok without creating site",
			k8sObjects:         nil,
			skupperObjects:     nil,
			siteName:           "my-site",
			serviceAccountName: "my-service-account",
			linkAccessType:     "default",
			output:             "yaml",
			skupperError:       "",
			errorMessage:       "sites.skupper.io \"my-site\" not found",
		},
		{
			name:               "runs fails because the output format is not supported",
			k8sObjects:         nil,
			skupperObjects:     nil,
			siteName:           "my-site",
			serviceAccountName: "my-service-account",
			linkAccessType:     "default",
			output:             "unsupported",
			skupperError:       "",
			errorMessage:       "sites.skupper.io \"my-site\" not found",
		},
	}

	for _, test := range testTable {
		command := &CmdSiteUpdate{
			Namespace: "test",
		}

		fakeSkupperClient, err := fakeclient.NewFakeClient(command.Namespace, test.k8sObjects, test.skupperObjects, test.skupperError)
		assert.Assert(t, err)
		command.Client = fakeSkupperClient.GetSkupperClient().SkupperV1alpha1()
		command.siteName = test.siteName
		command.serviceAccountName = test.serviceAccountName
		command.linkAccessType = test.linkAccessType
		command.output = test.output

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
			k8sObjects: nil,
			skupperObjects: []runtime.Object{
				&v1alpha1.Site{
					ObjectMeta: v1.ObjectMeta{
						Name:      "my-site",
						Namespace: "test",
					},
					Status: v1alpha1.SiteStatus{
						Status: v1alpha1.Status{
							StatusMessage: "",
						},
					},
				},
			},
			siteName:     "my-site",
			skupperError: "",
			errorMessage: "Site \"my-site\" not ready yet, check the status for more information\n",
			expectError:  true,
		},
	}

	for _, test := range testTable {
		command := &CmdSiteUpdate{
			Namespace: "test",
		}

		utils.SetRetryProfile(utils.TestRetryProfile)
		fakeSkupperClient, err := fakeclient.NewFakeClient(command.Namespace, test.k8sObjects, test.skupperObjects, test.skupperError)
		assert.Assert(t, err)
		command.Client = fakeSkupperClient.GetSkupperClient().SkupperV1alpha1()
		command.siteName = test.siteName
		command.timeout = 1

		t.Run(test.name, func(t *testing.T) {

			err := command.WaitUntil()
			if err != nil {
				assert.Check(t, test.expectError)
				assert.Check(t, test.errorMessage == err.Error())
			}

		})
	}
}

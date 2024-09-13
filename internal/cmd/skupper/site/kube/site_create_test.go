package kube

import (
	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/internal/cmd/skupper/common"
	"github.com/skupperproject/skupper/internal/cmd/skupper/common/utils"
	fakeclient "github.com/skupperproject/skupper/internal/kube/client/fake"
	"testing"
	"time"

	"github.com/skupperproject/skupper/pkg/apis/skupper/v1alpha1"
	"gotest.tools/assert"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestCmdSiteCreate_ValidateInput(t *testing.T) {
	type test struct {
		name           string
		args           []string
		k8sObjects     []runtime.Object
		skupperObjects []runtime.Object
		flags          *common.CommandSiteCreateFlags
		expectedErrors []string
	}

	testTable := []test{
		{
			name:       "site is not created because there is already a site in the namespace.",
			args:       []string{"my-new-site"},
			k8sObjects: nil,
			skupperObjects: []runtime.Object{
				&v1alpha1.SiteList{
					Items: []v1alpha1.Site{
						{
							ObjectMeta: v1.ObjectMeta{
								Name:      "old-site",
								Namespace: "test",
							},
						},
					},
				},
			},
			expectedErrors: []string{"there is already a site created for this namespace"},
		},
		{
			name:           "site name is not valid.",
			args:           []string{"my new site"},
			expectedErrors: []string{"site name is not valid: value does not match this regular expression: ^[a-z0-9]([-a-z0-9]*[a-z0-9])*(\\.[a-z0-9]([-a-z0-9]*[a-z0-9])*)*$"},
		},
		{
			name:           "site name is not specified.",
			args:           []string{},
			expectedErrors: []string{"site name must not be empty"},
		},
		{
			name:           "more than one argument was specified",
			args:           []string{"my", "site"},
			expectedErrors: []string{"only one argument is allowed for this command."},
		},
		{
			name:           "service account name is not valid.",
			args:           []string{"my-site"},
			flags:          &common.CommandSiteCreateFlags{ServiceAccount: "not valid service account name", Timeout: time.Minute},
			expectedErrors: []string{"service account name is not valid: serviceaccounts \"not valid service account name\" not found"},
		},
		{
			name:           "host name was specified, but this flag does not work on kube platforms",
			args:           []string{"my-site"},
			flags:          &common.CommandSiteCreateFlags{Host: "host", Timeout: time.Minute},
			expectedErrors: []string{},
		},
		{
			name:  "link access type is not valid",
			args:  []string{"my-site"},
			flags: &common.CommandSiteCreateFlags{LinkAccessType: "not-valid", Timeout: time.Minute},
			expectedErrors: []string{
				"link access type is not valid: value not-valid not allowed. It should be one of this options: [route loadbalancer default]",
				"for the site to work with this type of linkAccess, the --enable-link-access option must be set to true",
			},
		},
		{
			name:  "output format is not valid",
			args:  []string{"my-site"},
			flags: &common.CommandSiteCreateFlags{Output: "not-valid", Timeout: time.Minute},
			expectedErrors: []string{
				"output type is not valid: value not-valid not allowed. It should be one of this options: [json yaml]",
			},
		},
		{
			name:           "host flag is not valid for this platform",
			args:           []string{"my-site"},
			flags:          &common.CommandSiteCreateFlags{Host: "host", Timeout: time.Minute},
			expectedErrors: []string{},
		},
		{
			name:           "subject alternative names flag is not valid for this platform",
			args:           []string{"my-site"},
			flags:          &common.CommandSiteCreateFlags{SubjectAlternativeNames: []string{"test"}, Timeout: time.Minute},
			expectedErrors: []string{},
		},
		{
			name:  "timeout is not valid",
			args:  []string{"my-site"},
			flags: &common.CommandSiteCreateFlags{Timeout: time.Second * 0},
			expectedErrors: []string{
				"timeout is not valid: duration must not be less than 10s; got 0s",
			},
		},
	}

	for _, test := range testTable {
		t.Run(test.name, func(t *testing.T) {
			command := &CmdSiteCreate{
				Namespace: "test",
			}

			cmd := common.ConfigureCobraCommand(types.PlatformKubernetes, common.SkupperCmdDescription{}, command, nil)

			command.CobraCmd = cmd

			fakeSkupperClient, err := fakeclient.NewFakeClient(command.Namespace, test.k8sObjects, test.skupperObjects, "")
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

func TestCmdSiteCreate_InputToOptions(t *testing.T) {

	type test struct {
		name               string
		args               []string
		flags              common.CommandSiteCreateFlags
		expectedLinkAccess string
		expectedOutput     string
		expectedTimeout    time.Duration
	}

	testTable := []test{
		{
			name:               "options without link access enabled",
			args:               []string{"my-site"},
			flags:              common.CommandSiteCreateFlags{},
			expectedLinkAccess: "",
			expectedOutput:     "",
		},
		{
			name:               "options with link access enabled but using a type by default",
			args:               []string{"my-site"},
			flags:              common.CommandSiteCreateFlags{EnableLinkAccess: true},
			expectedLinkAccess: "default",
			expectedOutput:     "",
		},
		{
			name:               "options with link access enabled using the nodeport type",
			args:               []string{"my-site"},
			flags:              common.CommandSiteCreateFlags{EnableLinkAccess: true, LinkAccessType: "nodeport"},
			expectedLinkAccess: "nodeport",
			expectedOutput:     "",
		},
		{
			name:               "options with link access options not well specified",
			args:               []string{"my-site"},
			flags:              common.CommandSiteCreateFlags{EnableLinkAccess: false, LinkAccessType: "nodeport"},
			expectedLinkAccess: "",
			expectedOutput:     "",
		},
		{
			name:               "options output type",
			args:               []string{"my-site"},
			flags:              common.CommandSiteCreateFlags{EnableLinkAccess: false, LinkAccessType: "nodeport", Output: "yaml"},
			expectedLinkAccess: "",
			expectedOutput:     "yaml",
		},
		{
			name:               "options with timeout",
			args:               []string{"my-site"},
			flags:              common.CommandSiteCreateFlags{Timeout: time.Second * 60},
			expectedLinkAccess: "",
			expectedOutput:     "",
			expectedTimeout:    time.Minute,
		},
	}

	for _, test := range testTable {
		t.Run(test.name, func(t *testing.T) {
			cmd, err := newCmdSiteCreateWithMocks("test")
			assert.Assert(t, err)
			cmd.Flags = &test.flags
			cmd.siteName = "my-site"

			cmd.InputToOptions()

			assert.Check(t, cmd.output == test.expectedOutput)
			assert.Check(t, cmd.linkAccessType == test.expectedLinkAccess)
			assert.Check(t, cmd.timeout == test.expectedTimeout)
		})
	}
}

func TestCmdSiteCreate_Run(t *testing.T) {
	type test struct {
		name               string
		k8sObjects         []runtime.Object
		skupperObjects     []runtime.Object
		skupperError       string
		siteName           string
		serviceAccountName string
		options            map[string]string
		output             string
		errorMessage       string
	}

	testTable := []test{
		{
			name:               "runs ok",
			k8sObjects:         nil,
			skupperObjects:     nil,
			siteName:           "my-site",
			serviceAccountName: "my-service-account",
			options:            map[string]string{"name": "my-site"},
			skupperError:       "",
		},
		{
			name:       "runs fails",
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
			options:            map[string]string{"ingress": "none"},
			skupperError:       "",
			errorMessage:       "sites.skupper.io \"my-site\" already exists",
		},
		{
			name:               "runs ok without create site",
			k8sObjects:         nil,
			skupperObjects:     nil,
			siteName:           "test",
			serviceAccountName: "my-service-account",
			options:            map[string]string{"name": "my-site"},
			output:             "yaml",
			skupperError:       "",
		},
		{
			name:               "runs fails because the output format is not supported",
			k8sObjects:         nil,
			skupperObjects:     nil,
			siteName:           "test",
			serviceAccountName: "my-service-account",
			options:            map[string]string{"name": "my-site"},
			output:             "unsupported",
			skupperError:       "",
			errorMessage:       "format unsupported not supported",
		},
	}

	for _, test := range testTable {
		command := &CmdSiteCreate{
			Namespace: "test",
		}

		fakeSkupperClient, err := fakeclient.NewFakeClient(command.Namespace, test.k8sObjects, test.skupperObjects, test.skupperError)
		assert.Assert(t, err)
		command.Client = fakeSkupperClient.GetSkupperClient().SkupperV1alpha1()

		command.siteName = test.siteName
		command.serviceAccountName = test.serviceAccountName
		command.output = test.output

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
func TestCmdSiteCreate_WaitUntil(t *testing.T) {
	type test struct {
		name           string
		k8sObjects     []runtime.Object
		skupperObjects []runtime.Object
		skupperError   string
		expectError    bool
		output         string
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
			skupperError: "",
			expectError:  true,
		},
		{
			name:         "site is not returned",
			skupperError: "it failed",
			expectError:  true,
		},
		{
			name:       "there is no need to wait for a site, the user just wanted the output",
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
			output:       "yaml",
			skupperError: "",
			expectError:  false,
		},
		{
			name:       "site is ready",
			k8sObjects: nil,
			skupperObjects: []runtime.Object{
				&v1alpha1.Site{
					ObjectMeta: v1.ObjectMeta{
						Name:      "my-site",
						Namespace: "test",
					},
					Status: v1alpha1.SiteStatus{
						Status: v1alpha1.Status{
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
		command := &CmdSiteCreate{
			Namespace: "test",
		}

		utils.SetRetryProfile(utils.TestRetryProfile)
		fakeSkupperClient, err := fakeclient.NewFakeClient(command.Namespace, test.k8sObjects, test.skupperObjects, test.skupperError)
		assert.Assert(t, err)
		command.Client = fakeSkupperClient.GetSkupperClient().SkupperV1alpha1()
		command.siteName = "my-site"
		command.output = test.output
		command.timeout = time.Second

		t.Run(test.name, func(t *testing.T) {

			err := command.WaitUntil()

			if test.expectError {
				assert.Check(t, err != nil)
			} else {
				assert.Check(t, err == nil, err)
			}

		})
	}
}

// --- helper methods

func newCmdSiteCreateWithMocks(namespace string) (*CmdSiteCreate, error) {

	client, err := fakeclient.NewFakeClient(namespace, nil, nil, "")
	if err != nil {
		return nil, err
	}
	cmdSiteCreate := &CmdSiteCreate{
		Client:     client.GetSkupperClient().SkupperV1alpha1(),
		KubeClient: client.GetKubeClient(),
		Namespace:  namespace,
	}

	return cmdSiteCreate, nil
}

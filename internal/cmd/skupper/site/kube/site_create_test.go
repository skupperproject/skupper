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

func TestCmdSiteCreate_ValidateInput(t *testing.T) {
	type test struct {
		name           string
		args           []string
		k8sObjects     []runtime.Object
		skupperObjects []runtime.Object
		flags          *common.CommandSiteCreateFlags
		expectedError  string
	}

	testTable := []test{
		{
			name:       "site is not created because there is already a site in the namespace.",
			args:       []string{"my-new-site"},
			k8sObjects: nil,
			skupperObjects: []runtime.Object{
				&v2alpha1.SiteList{
					Items: []v2alpha1.Site{
						{
							ObjectMeta: v1.ObjectMeta{
								Name:      "old-site",
								Namespace: "test",
							},
						},
					},
				},
			},
			expectedError: "There is already a site created for this namespace",
		},
		{
			name:          "site name is not valid.",
			args:          []string{"my new site"},
			expectedError: "site name is not valid: value does not match this regular expression: ^[a-z0-9]([-a-z0-9]*[a-z0-9])*(\\.[a-z0-9]([-a-z0-9]*[a-z0-9])*)*$",
		},
		{
			name:          "site name is not specified.",
			args:          []string{},
			expectedError: "site name must not be empty",
		},
		{
			name:          "more than one argument was specified",
			args:          []string{"my", "site"},
			expectedError: "only one argument is allowed for this command.",
		},
		{
			name:          "service account name is not valid.",
			args:          []string{"my-site"},
			flags:         &common.CommandSiteCreateFlags{ServiceAccount: "not valid service account name", Timeout: time.Minute},
			expectedError: "service account name is not valid: serviceaccounts \"not valid service account name\" not found",
		},
		{
			name:  "link access type is not valid",
			args:  []string{"my-site"},
			flags: &common.CommandSiteCreateFlags{LinkAccessType: "not-valid", Timeout: time.Minute},
			expectedError: "link access type is not valid: value not-valid not allowed. It should be one of this options: [route loadbalancer default]\n" +
				"for the site to work with this type of linkAccess, the --enable-link-access option must be set to true",
		},
		{
			name:  "bind-host flag is not valid for this platform",
			args:  []string{"my-site"},
			flags: &common.CommandSiteCreateFlags{BindHost: "host", Timeout: time.Minute},
		},
		{
			name:          "subject alternative names flag is not valid for this platform",
			args:          []string{"my-site"},
			flags:         &common.CommandSiteCreateFlags{SubjectAlternativeNames: []string{"test"}, Timeout: time.Minute},
			expectedError: "",
		},
		{
			name:          "timeout is not valid",
			args:          []string{"my-site"},
			flags:         &common.CommandSiteCreateFlags{Timeout: time.Second * 0},
			expectedError: "timeout is not valid: duration must not be less than 10s; got 0s",
		},
		{
			name:          "wait status is not valid",
			args:          []string{"my-site"},
			flags:         &common.CommandSiteCreateFlags{Timeout: time.Minute, Wait: "created"},
			expectedError: "status is not valid: value created not allowed. It should be one of this options: [ready configured none]",
		},
	}

	for _, test := range testTable {
		t.Run(test.name, func(t *testing.T) {
			command := &CmdSiteCreate{
				Namespace: "test",
			}

			cmd := common.ConfigureCobraCommand(common.PlatformKubernetes, common.SkupperCmdDescription{}, command, nil)

			command.CobraCmd = cmd

			fakeSkupperClient, err := fakeclient.NewFakeClient(command.Namespace, test.k8sObjects, test.skupperObjects, "")
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

func TestCmdSiteCreate_InputToOptions(t *testing.T) {

	type test struct {
		name               string
		args               []string
		flags              common.CommandSiteCreateFlags
		expectedLinkAccess string
		expectedTimeout    time.Duration
		expectedStatus     string
	}

	testTable := []test{
		{
			name:               "options without link access enabled",
			args:               []string{"my-site"},
			flags:              common.CommandSiteCreateFlags{},
			expectedLinkAccess: "",
		},
		{
			name:               "options with link access enabled but using a type by default",
			args:               []string{"my-site"},
			flags:              common.CommandSiteCreateFlags{EnableLinkAccess: true},
			expectedLinkAccess: "default",
		},
		{
			name:               "options with link access enabled using the nodeport type",
			args:               []string{"my-site"},
			flags:              common.CommandSiteCreateFlags{EnableLinkAccess: true, LinkAccessType: "nodeport"},
			expectedLinkAccess: "nodeport",
		},
		{
			name:               "options with link access options not well specified",
			args:               []string{"my-site"},
			flags:              common.CommandSiteCreateFlags{EnableLinkAccess: false, LinkAccessType: "nodeport"},
			expectedLinkAccess: "",
		},
		{
			name:               "options with timeout",
			args:               []string{"my-site"},
			flags:              common.CommandSiteCreateFlags{Timeout: time.Second * 60},
			expectedLinkAccess: "",
			expectedTimeout:    time.Minute,
		},
		{
			name:           "options with waiting status",
			args:           []string{"my-site"},
			flags:          common.CommandSiteCreateFlags{Wait: "configured"},
			expectedStatus: "configured",
		},
	}

	for _, test := range testTable {
		t.Run(test.name, func(t *testing.T) {
			cmd, err := newCmdSiteCreateWithMocks("test")
			assert.Assert(t, err)
			cmd.Flags = &test.flags
			cmd.siteName = "my-site"

			cmd.InputToOptions()

			assert.Check(t, cmd.linkAccessType == test.expectedLinkAccess)
			assert.Check(t, cmd.timeout == test.expectedTimeout)
			assert.Check(t, cmd.status == test.expectedStatus)
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
			name:       "run fails",
			k8sObjects: nil,
			skupperObjects: []runtime.Object{
				&v2alpha1.Site{
					ObjectMeta: v1.ObjectMeta{
						Name:      "my-site",
						Namespace: "test",
					},
					Status: v2alpha1.SiteStatus{
						Status: v2alpha1.Status{},
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
			name:               "runs ok but it does not create the site",
			k8sObjects:         nil,
			skupperObjects:     nil,
			siteName:           "test",
			serviceAccountName: "my-service-account",
			options:            map[string]string{"name": "my-site"},
			skupperError:       "",
		},
	}

	for _, test := range testTable {
		command := &CmdSiteCreate{
			Namespace: "test",
		}

		fakeSkupperClient, err := fakeclient.NewFakeClient(command.Namespace, test.k8sObjects, test.skupperObjects, test.skupperError)
		assert.Assert(t, err)
		command.Client = fakeSkupperClient.GetSkupperClient().SkupperV2alpha1()

		command.siteName = test.siteName
		command.serviceAccountName = test.serviceAccountName

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
		status         string
		k8sObjects     []runtime.Object
		skupperObjects []runtime.Object
		skupperError   string
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
					Status: v2alpha1.SiteStatus{
						Status: v2alpha1.Status{
							Conditions: []v1.Condition{
								{
									Message:            "OK",
									ObservedGeneration: 1,
									Reason:             "OK",
									Status:             "True",
									Type:               "Pending",
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
			name:         "site is not returned",
			skupperError: "it failed",
			expectError:  true,
		},
		{
			name:       "site is ready",
			status:     "ready",
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
									Type:               "Ready",
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
			skupperError: "",
			expectError:  true,
		},
	}

	for _, test := range testTable {
		command := &CmdSiteCreate{
			Namespace: "test",
		}

		utils.SetRetryProfile(utils.TestRetryProfile)
		fakeSkupperClient, err := fakeclient.NewFakeClient(command.Namespace, test.k8sObjects, test.skupperObjects, test.skupperError)
		assert.Assert(t, err)
		command.Client = fakeSkupperClient.GetSkupperClient().SkupperV2alpha1()
		command.siteName = "my-site"
		command.timeout = time.Second
		command.status = test.status

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
		Client:     client.GetSkupperClient().SkupperV2alpha1(),
		KubeClient: client.GetKubeClient(),
		Namespace:  namespace,
	}

	return cmdSiteCreate, nil
}

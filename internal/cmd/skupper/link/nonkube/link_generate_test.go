package nonkube

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/skupperproject/skupper/internal/cmd/skupper/common"
	"github.com/skupperproject/skupper/internal/cmd/skupper/common/testutils"
	"github.com/skupperproject/skupper/internal/nonkube/client/fs"
	nonkubecommon "github.com/skupperproject/skupper/internal/nonkube/common"
	"github.com/skupperproject/skupper/pkg/apis/skupper/v2alpha1"
	"github.com/skupperproject/skupper/pkg/nonkube/api"
	"gotest.tools/v3/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestCmdLinkGenerate_ValidateInput(t *testing.T) {
	type test struct {
		name               string
		namespace          string
		args               []string
		createSite         bool
		createRouterAccess bool
		expectedError      string
	}

	testTable := []test{
		{
			name:               "an argument was specified",
			namespace:          "test",
			args:               []string{"test"},
			createSite:         true,
			createRouterAccess: true,
			expectedError:      "arguments are not allowed in this command",
		},
		{
			name:          "site is not active",
			createSite:    false,
			expectedError: "there is no active site in this namespace",
		},
		{
			name:               "site was not enabled for link access",
			namespace:          "test",
			createSite:         true,
			createRouterAccess: false,
			expectedError:      "this site is not enabled for link access, there are no links created",
		},
		{
			name:               "invalid namespace",
			namespace:          "TestInvalid",
			createSite:         true,
			createRouterAccess: true,
			expectedError:      "namespace is not valid: value does not match this regular expression: ^[a-z0-9]([-a-z0-9]*[a-z0-9])?$",
		},
	}

	command := &CmdLinkGenerate{}

	for _, test := range testTable {
		t.Run(test.name, func(t *testing.T) {
			command.Namespace = test.namespace
			if os.Getuid() == 0 {
				api.DefaultRootDataHome = t.TempDir()
			} else {
				t.Setenv("XDG_DATA_HOME", t.TempDir())
			}
			tmpDir := api.GetDataHome()
			path := filepath.Join(tmpDir, "/namespaces/", command.Namespace, "/", string(api.RuntimeSiteStatePath))
			if test.createSite {
				createSiteResource(path, t)
			}
			if test.createRouterAccess {
				createRouterAccessResource(path, t)
			}

			testutils.CheckValidateInput(t, command, test.expectedError, test.args)

		})
	}
}

func TestCmdLinkGenerate_Run(t *testing.T) {
	type test struct {
		name         string
		createLink   bool
		errorMessage string
	}

	testTable := []test{
		{
			name:         "it fails because link doesn't exist",
			errorMessage: "Error searching for tokens: there are no links created",
		},
		{
			name:       "runs ok",
			createLink: true,
		},
	}

	// add two Links in runtime directory
	command := &CmdLinkGenerate{}
	command.Namespace = "test"
	command.tokenHandler = fs.NewTokenHandler(command.Namespace)

	for _, test := range testTable {

		t.Run(test.name, func(t *testing.T) {

			if os.Getuid() == 0 {
				api.DefaultRootDataHome = t.TempDir()
			} else {
				t.Setenv("XDG_DATA_HOME", t.TempDir())
			}
			tmpDir := api.GetDataHome()
			sitePath := filepath.Join(tmpDir, "/namespaces/test/", string(api.RuntimeSiteStatePath))
			linkPath := filepath.Join(tmpDir, "/namespaces/test", string(api.RuntimeTokenPath))

			createSiteResource(sitePath, t)

			pathProvider := fs.PathProvider{Namespace: command.Namespace}
			siteStateLoader := &nonkubecommon.FileSystemSiteStateLoader{
				Path: pathProvider.GetRuntimeNamespace(),
			}

			siteState, err := siteStateLoader.Load()

			command.siteState = siteState

			if test.createLink {
				createLinkResource(linkPath, t)
			}

			err = command.Run()
			if err != nil {
				assert.Check(t, test.errorMessage == err.Error())
			} else {
				assert.Check(t, err == nil)
			}
		})
	}
}

func createSiteResource(path string, t *testing.T) {
	siteResource := v2alpha1.Site{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "skupper.io/v2alpha1",
			Kind:       "Site",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-site",
			Namespace: "test",
		},
	}

	siteHandler := fs.NewSiteHandler("test")

	defer siteHandler.Delete("my-site")

	contentSite, err := siteHandler.EncodeToYaml(siteResource)
	assert.Check(t, err == nil)
	err = siteHandler.WriteFile(path, "my-site.yaml", contentSite, common.Sites)
	assert.Check(t, err == nil)
}

func createRouterAccessResource(path string, t *testing.T) {
	routerAccessResource := v2alpha1.RouterAccess{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "skupper.io/v2alpha1",
			Kind:       "RouterAccess",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "router-access-test",
			Namespace: "test",
		},
		Spec: v2alpha1.RouterAccessSpec{
			BindHost: "0.0.0.0",
			Roles: []v2alpha1.RouterAccessRole{
				{
					Name: "inter-router",
					Port: 55671,
				},
				{
					Name: "edge",
					Port: 45671,
				},
			},
		},
	}

	routerAccessHandler := fs.NewRouterAccessHandler("test")

	defer routerAccessHandler.Delete("my-router-access")

	contentRouterAccess, err := routerAccessHandler.EncodeToYaml(routerAccessResource)
	assert.Check(t, err == nil)
	err = routerAccessHandler.WriteFile(path, "my-router-access.yaml", contentRouterAccess, common.RouterAccesses)
	assert.Check(t, err == nil)
}

func createLinkResource(path string, t *testing.T) {
	linkResource := v2alpha1.Link{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "skupper.io/v2alpha1",
			Kind:       "Link",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-link",
			Namespace: "test",
		},
		Spec: v2alpha1.LinkSpec{
			TlsCredentials: "my-secret",
			Cost:           2,
			Endpoints: []v2alpha1.Endpoint{
				{
					Name: "inter-router",
					Host: "127.0.0.1",
					Port: "55671",
				},
				{
					Name: "edge",
					Host: "127.0.0.1",
					Port: "45671",
				},
			},
		},
		Status: v2alpha1.LinkStatus{
			Status: v2alpha1.Status{
				Conditions: []metav1.Condition{
					{
						Type:   "Configured",
						Status: "True",
					},
				},
			},
		},
	}

	linkHandler := fs.NewLinkHandler("test")

	defer linkHandler.Delete("my-link")

	content, err := linkHandler.EncodeToYaml(linkResource)
	assert.Check(t, err == nil)
	err = linkHandler.WriteFile(path, "my-link.yaml", content, common.Links)
	assert.Check(t, err == nil)
}

package nonkube

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/skupperproject/skupper/internal/cmd/skupper/common"
	"github.com/skupperproject/skupper/internal/cmd/skupper/common/testutils"
	"github.com/skupperproject/skupper/internal/nonkube/client/fs"
	"github.com/skupperproject/skupper/pkg/apis/skupper/v2alpha1"
	"github.com/skupperproject/skupper/pkg/nonkube/api"
	"github.com/spf13/cobra"
	"gotest.tools/v3/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestCmdSiteDelete_ValidateInput(t *testing.T) {
	type test struct {
		name              string
		namespace         string
		args              []string
		flags             *common.CommandSiteDeleteFlags
		cobraGenericFlags map[string]string
		expectedError     string
	}

	if os.Getuid() == 0 {
		api.DefaultRootDataHome = t.TempDir()
	} else {
		t.Setenv("XDG_DATA_HOME", t.TempDir())
	}
	tmpDir := api.GetDataHome()
	path := filepath.Join(tmpDir, "/namespaces/test/", string(api.InputSiteStatePath))

	testTable := []test{
		{
			name:          "site name is not specified",
			namespace:     "test",
			args:          []string{},
			flags:         &common.CommandSiteDeleteFlags{},
			expectedError: "site name must be specified",
		},
		{
			name:      "site name is not specified, all",
			namespace: "test",
			args:      []string{},
			flags:     &common.CommandSiteDeleteFlags{All: true},
		},
		{
			name:          "site name is nil",
			namespace:     "test",
			args:          []string{""},
			flags:         &common.CommandSiteDeleteFlags{All: false},
			expectedError: "site name must not be empty",
		},
		{
			name:          "site name is not valid",
			namespace:     "test",
			args:          []string{"my name"},
			flags:         &common.CommandSiteDeleteFlags{},
			expectedError: "site name is not valid: value does not match this regular expression: ^[a-z0-9]([-a-z0-9]*[a-z0-9])*(\\.[a-z0-9]([-a-z0-9]*[a-z0-9])*)*$",
		},
		{
			name:          "more than one argument is specified",
			namespace:     "test",
			args:          []string{"my", "site"},
			flags:         &common.CommandSiteDeleteFlags{},
			expectedError: "only one argument is allowed for this command",
		},
		{
			name:          "site doesn't exist",
			namespace:     "test",
			args:          []string{"no-site"},
			flags:         &common.CommandSiteDeleteFlags{},
			expectedError: "site no-site does not exist",
		},
		{
			name:          "kubernetes flags are not valid on this platform",
			namespace:     "test",
			args:          []string{"my-site"},
			flags:         &common.CommandSiteDeleteFlags{},
			expectedError: "",
			cobraGenericFlags: map[string]string{
				common.FlagNameContext:    "test",
				common.FlagNameKubeconfig: "test",
			},
		},
		{
			name:          "invalid namespace",
			namespace:     "TestInvalid",
			args:          []string{"my-site"},
			flags:         &common.CommandSiteDeleteFlags{},
			expectedError: "namespace is not valid: value does not match this regular expression: ^[a-z0-9]([-a-z0-9]*[a-z0-9])?$",
		},
	}

	// Add temp files so site exists for update tests
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
	command := &CmdSiteDelete{Flags: &common.CommandSiteDeleteFlags{}}
	command.CobraCmd = &cobra.Command{Use: "test"}
	command.namespace = "test"
	command.siteHandler = fs.NewSiteHandler(command.namespace)
	command.routerAccessHandler = fs.NewRouterAccessHandler(command.namespace)

	defer command.siteHandler.Delete("my-site")
	content, err := command.siteHandler.EncodeToYaml(siteResource)
	assert.Check(t, err == nil)
	err = command.siteHandler.WriteFile(path, "my-site.yaml", content, common.Sites)
	assert.Check(t, err == nil)

	for _, test := range testTable {
		t.Run(test.name, func(t *testing.T) {
			command.siteName = ""
			if test.flags != nil {
				command.Flags = test.flags
			}
			command.namespace = test.namespace

			if test.cobraGenericFlags != nil && len(test.cobraGenericFlags) > 0 {
				for name, value := range test.cobraGenericFlags {
					command.CobraCmd.Flags().String(name, value, "")
				}
			}

			testutils.CheckValidateInput(t, command, test.expectedError, test.args)
		})
	}
}

func TestCmdSiteDelete_Run(t *testing.T) {
	type test struct {
		name              string
		namespace         string
		deleteName        string
		errorMessage      string
		expectedNamespace string
		all               bool
	}

	testTable := []test{
		{
			name:              "run default",
			deleteName:        "no-site",
			errorMessage:      "no such file or directory",
			expectedNamespace: "default",
			all:               false,
		},
		{
			name:              "run delete all",
			namespace:         "test2",
			deleteName:        "my-site",
			errorMessage:      "error",
			expectedNamespace: "test2",
			all:               true,
		},
		{
			name:              "run delete all",
			namespace:         "test2",
			errorMessage:      "error",
			expectedNamespace: "test2",
			all:               true,
		},
	}

	// Add a temp file so listener/connector/site exists for delete tests
	listenerResource := v2alpha1.Listener{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "skupper.io/v2alpha1",
			Kind:       "Listener",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-listener",
			Namespace: "test2",
		},
	}
	connectorResource := v2alpha1.Connector{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "skupper.io/v2alpha1",
			Kind:       "Connector",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-connector",
			Namespace: "test2",
		},
	}
	siteResource := v2alpha1.Site{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "skupper.io/v2alpha1",
			Kind:       "Site",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-site",
			Namespace: "test2",
		},
	}

	if os.Getuid() == 0 {
		api.DefaultRootDataHome = t.TempDir()
	} else {
		t.Setenv("XDG_DATA_HOME", t.TempDir())
	}
	tmpDir := api.GetDataHome()
	path := filepath.Join(tmpDir, "/namespaces/test2/", string(api.InputSiteStatePath))

	command := &CmdSiteDelete{Flags: &common.CommandSiteDeleteFlags{}}
	command.namespace = "test2"
	command.siteHandler = fs.NewSiteHandler(command.namespace)
	command.routerAccessHandler = fs.NewRouterAccessHandler(command.namespace)
	listenerHandler := fs.NewListenerHandler(command.namespace)
	connectorHandler := fs.NewConnectorHandler(command.namespace)

	content, err := command.siteHandler.EncodeToYaml(siteResource)
	assert.Check(t, err == nil)
	err = command.siteHandler.WriteFile(path, "my-site.yaml", content, common.Sites)
	assert.Check(t, err == nil)
	defer command.siteHandler.Delete("my-site")

	content, err = listenerHandler.EncodeToYaml(listenerResource)
	assert.Check(t, err == nil)
	err = listenerHandler.WriteFile(path, "my-listener.yaml", content, common.Listeners)
	assert.Check(t, err == nil)
	defer listenerHandler.Delete("my-listener")

	content, err = connectorHandler.EncodeToYaml(connectorResource)
	assert.Check(t, err == nil)
	err = connectorHandler.WriteFile(path, "my-connector.yaml", content, common.Connectors)
	assert.Check(t, err == nil)
	defer connectorHandler.Delete("my-connector")

	for _, test := range testTable {

		t.Run(test.name, func(t *testing.T) {
			command.namespace = test.namespace
			command.siteName = test.deleteName
			command.Flags.All = test.all
			command.InputToOptions()

			err := command.Run()
			if err != nil {
				assert.Check(t, strings.HasSuffix(err.Error(), test.errorMessage))
			} else {
				assert.Check(t, err == nil)
				assert.Equal(t, command.namespace, test.expectedNamespace)
				if test.all {
					// only deleting from input/resources directory
					// expect all resources are deleted
					opts := fs.GetOptions{RuntimeFirst: false, LogWarning: false}
					site, _ := command.siteHandler.Get(command.siteName, opts)
					assert.Check(t, site == nil)
					listeners, _ := listenerHandler.List()
					for _, listener := range listeners {
						resource, _ := listenerHandler.Get(listener.Name, opts)
						assert.Check(t, resource == nil)
					}
					connectors, _ := connectorHandler.List()
					for _, connector := range connectors {
						resource, _ := connectorHandler.Get(connector.Name, opts)
						assert.Check(t, resource == nil)
					}
				}
			}
		})
	}
}

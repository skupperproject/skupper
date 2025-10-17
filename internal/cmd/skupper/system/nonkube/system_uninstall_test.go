package nonkube

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/skupperproject/skupper/internal/cmd/skupper/common"
	"github.com/skupperproject/skupper/internal/cmd/skupper/common/testutils"
	"github.com/skupperproject/skupper/internal/config"
	"github.com/skupperproject/skupper/internal/nonkube/client/fs"
	"github.com/skupperproject/skupper/pkg/apis/skupper/v2alpha1"
	"github.com/skupperproject/skupper/pkg/nonkube/api"
	"gotest.tools/v3/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type CmdSiteStatus struct {
	siteHandler *fs.SiteHandler
	Flags       *common.CommandSiteStatusFlags
	namespace   string
}

func TestCmdSystemUnInstall_ValidateInput(t *testing.T) {
	type test struct {
		name          string
		args          []string
		flags         *common.CommandSystemUninstallFlags
		platform      string
		mock          func() (bool, error)
		expectedError string
	}

	testTable := []test{
		{
			name:          "args are not accepted",
			args:          []string{"something"},
			platform:      "podman",
			expectedError: "this command does not accept arguments",
		},
		{
			name:     "force flag is provided",
			platform: "podman",
			flags:    &common.CommandSystemUninstallFlags{Force: true},
		},
		{
			name:          "force flag is not provided and there are active sites",
			flags:         &common.CommandSystemUninstallFlags{Force: false},
			platform:      "podman",
			mock:          mockCmdSystemUninstallThereAreStillSites,
			expectedError: "Uninstallation halted: Active sites detected. Use --force flag to stop and remove active sites",
		},
		{
			name:     "force flag is not provided but there are not any active site",
			flags:    &common.CommandSystemUninstallFlags{Force: false},
			platform: "podman",
			mock:     mockCmdSystemUninstallNoActiveSites,
		},
		{
			name:          "force flag is not provided but checking sites fails",
			flags:         &common.CommandSystemUninstallFlags{Force: false},
			platform:      "podman",
			mock:          mockCmdSystemUninstallCheckActiveSitesFails,
			expectedError: "error",
		},
		{
			name:          "platform not supported",
			platform:      "linux",
			expectedError: "the selected platform is not supported by this command. There is nothing to uninstall",
		},
	}

	for _, test := range testTable {
		t.Run(test.name, func(t *testing.T) {

			config.ClearPlatform()
			err := os.Setenv("SKUPPER_PLATFORM", test.platform)
			assert.Check(t, err == nil)

			command := newCmdSystemUninstallWithMocks(false)
			command.CheckActiveSites = test.mock
			command.Flags = test.flags

			testutils.CheckValidateInput(t, command, test.expectedError, test.args)
		})
	}

}

func TestCmdSystemUninstall_InputToOptions(t *testing.T) {

	type test struct {
		name          string
		flags         *common.CommandSystemUninstallFlags
		expectedForce bool
	}

	testTable := []test{
		{
			name:          "options-by-default",
			flags:         &common.CommandSystemUninstallFlags{Force: false},
			expectedForce: false,
		},
		{
			name:          "forced to uninstall",
			flags:         &common.CommandSystemUninstallFlags{Force: true},
			expectedForce: true,
		},
	}

	for _, test := range testTable {
		t.Run(test.name, func(t *testing.T) {

			cmd := newCmdSystemUninstallWithMocks(false)
			cmd.Flags = test.flags
			cmd.InputToOptions()

			assert.Check(t, cmd.forceUninstall == test.expectedForce)
		})
	}
}

func TestCmdSystemUninstall_Run(t *testing.T) {
	type test struct {
		name               string
		flags              *common.CommandSystemUninstallFlags
		disableSocketFails bool
		errorMessage       string
	}

	testTable := []test{
		{
			name:               "runs ok",
			disableSocketFails: false,
			errorMessage:       "",
			flags:              &common.CommandSystemUninstallFlags{Force: true},
		},
		{
			name:               "disable socket fails",
			disableSocketFails: true,
			errorMessage:       "Unable to uninstall.\nError: disable socket fails",
			flags:              &common.CommandSystemUninstallFlags{Force: false},
		},
	}

	//Add a temp file so site exists for uninstall tests
	siteResource1 := v2alpha1.Site{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "skupper.io/v2alpha1",
			Kind:       "Site",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-site",
			Namespace: "test",
		},
		Spec: v2alpha1.SiteSpec{
			LinkAccess: "route",
			Settings: map[string]string{
				"name": "my-site",
			},
		},
		Status: v2alpha1.SiteStatus{
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

	// add site in runtime directory
	if os.Getuid() == 0 {
		api.DefaultRootDataHome = t.TempDir()
	} else {
		t.Setenv("XDG_DATA_HOME", t.TempDir())
	}
	tmpDir := api.GetDataHome()
	cmd := &CmdSiteStatus{}
	cmd.namespace = "test"
	cmd.siteHandler = fs.NewSiteHandler(cmd.namespace)
	content, err := cmd.siteHandler.EncodeToYaml(siteResource1)
	assert.Check(t, err == nil)
	path := filepath.Join(tmpDir, "/namespaces/test/", string(api.RuntimeSiteStatePath))
	path2 := filepath.Join(tmpDir, "/namespaces/test2/", string(api.InputSiteStatePath))
	err = cmd.siteHandler.WriteFile(path, "my-site.yaml", content, common.Sites)
	assert.Check(t, err == nil)
	err = cmd.siteHandler.WriteFile(path2, "my-site.yaml", content, common.Sites)
	assert.Check(t, err == nil)
	defer cleanup()

	for _, test := range testTable {
		command := newCmdSystemUninstallWithMocks(test.disableSocketFails)
		command.forceUninstall = test.flags.Force

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

// --- helper methods

func cleanup() {
	tmpDir := api.GetDataHome()
	path := filepath.Join(tmpDir, "/namespaces/test/")
	os.RemoveAll(path)
	path = filepath.Join(tmpDir, "/namespaces/test2/")
	os.RemoveAll(path)
}

func newCmdSystemUninstallWithMocks(disableSocketFails bool) *CmdSystemUninstall {

	cmdMock := &CmdSystemUninstall{
		SystemUninstall:  mockCmdSystemUninstall,
		CheckActiveSites: mockCmdSystemUninstallNoActiveSites,
		TearDown:         mockCmdSystemTearDown,
	}

	if disableSocketFails {
		cmdMock.SystemUninstall = mockCmdSystemUninstallDisableSocketFails
	}

	return cmdMock
}

func mockCmdSystemUninstall(platform string) error { return nil }
func mockCmdSystemUninstallDisableSocketFails(platform string) error {
	return fmt.Errorf("disable socket fails")
}

func mockCmdSystemUninstallThereAreStillSites() (bool, error)    { return true, nil }
func mockCmdSystemUninstallCheckActiveSitesFails() (bool, error) { return false, fmt.Errorf("error") }
func mockCmdSystemUninstallNoActiveSites() (bool, error)         { return false, nil }
func mockCmdSystemTearDown(string) error                         { return nil }

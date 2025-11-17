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

func TestCmdConnectorDelete_ValidateInput(t *testing.T) {
	type test struct {
		name              string
		namespace         string
		args              []string
		flags             *common.CommandConnectorDeleteFlags
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
			name:          "connector name is not specified",
			namespace:     "test",
			args:          []string{},
			flags:         &common.CommandConnectorDeleteFlags{},
			expectedError: "connector name must be configured",
		},
		{
			name:          "connector name is nil",
			namespace:     "test",
			args:          []string{""},
			flags:         &common.CommandConnectorDeleteFlags{},
			expectedError: "connector name must not be empty",
		},
		{
			name:          "connector name is not valid",
			args:          []string{"my name"},
			flags:         &common.CommandConnectorDeleteFlags{},
			expectedError: "connector name is not valid: value does not match this regular expression: ^[a-z0-9]([-a-z0-9]*[a-z0-9])*(\\.[a-z0-9]([-a-z0-9]*[a-z0-9])*)*$",
		},
		{
			name:          "more than one argument is specified",
			args:          []string{"my", "connector"},
			flags:         &common.CommandConnectorDeleteFlags{},
			expectedError: "only one argument is allowed for this command",
		},
		{
			name:          "connector doesn't exist ",
			args:          []string{"no-connector"},
			flags:         &common.CommandConnectorDeleteFlags{},
			expectedError: "connector no-connector does not exist",
		},
		{
			name:          "kubernetes flags are not valid on this platform",
			args:          []string{"my-connector"},
			flags:         &common.CommandConnectorDeleteFlags{},
			expectedError: "",
			cobraGenericFlags: map[string]string{
				common.FlagNameContext:    "test",
				common.FlagNameKubeconfig: "test",
			},
		},
		{
			name:          "invalid namespace",
			namespace:     "Test5",
			args:          []string{"my-connector"},
			flags:         &common.CommandConnectorDeleteFlags{},
			expectedError: "namespace is not valid: value does not match this regular expression: ^[a-z0-9]([-a-z0-9]*[a-z0-9])?$",
		},
	}

	//Add a temp file so connector exists for update tests will pass
	connectorResource := v2alpha1.Connector{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "skupper.io/v2alpha1",
			Kind:       "Connector",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-connector",
			Namespace: "test",
		},
	}

	command := &CmdConnectorDelete{Flags: &common.CommandConnectorDeleteFlags{}}
	command.CobraCmd = &cobra.Command{Use: "test"}
	command.namespace = "test"
	command.connectorHandler = fs.NewConnectorHandler(command.namespace)

	defer command.connectorHandler.Delete("my-connector")
	content, err := command.connectorHandler.EncodeToYaml(connectorResource)
	assert.Check(t, err == nil)
	err = command.connectorHandler.WriteFile(path, "my-connector.yaml", content, common.Connectors)
	assert.Check(t, err == nil)

	for _, test := range testTable {
		t.Run(test.name, func(t *testing.T) {
			command.connectorName = ""
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

func TestCmdConnectorDelete_Run(t *testing.T) {
	type test struct {
		name         string
		namespace    string
		deleteName   string
		errorMessage string
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
			name:         "run fails default",
			deleteName:   "my-connector",
			errorMessage: "no such file or directory",
		},
		{
			name:       "runs ok",
			namespace:  "test",
			deleteName: "my-connector",
		},
	}

	for _, test := range testTable {
		cmd := &CmdConnectorDelete{}

		t.Run(test.name, func(t *testing.T) {

			createConnectorResource(path, t)
			cmd.connectorName = test.deleteName
			cmd.namespace = test.namespace
			cmd.connectorHandler = fs.NewConnectorHandler(cmd.namespace)
			cmd.InputToOptions()

			err := cmd.Run()

			if test.errorMessage != "" {
				assert.Check(t, strings.HasSuffix(err.Error(), test.errorMessage), err.Error())
			} else {
				assert.Check(t, err == nil)
			}
		})
	}
}

func createConnectorResource(path string, t *testing.T) {
	connectorResource := v2alpha1.Connector{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "skupper.io/v2alpha1",
			Kind:       "Connector",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-connector",
			Namespace: "test",
		},
	}

	connectorHandler := fs.NewConnectorHandler("test")

	contentConnector, err := connectorHandler.EncodeToYaml(connectorResource)
	assert.Check(t, err == nil)
	err = connectorHandler.WriteFile(path, "my-connector.yaml", contentConnector, common.Connectors)
	assert.Check(t, err == nil)
}
